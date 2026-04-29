package device

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type iosManager struct{}

// NewIOSManager returns a Manager implementation for Apple iOS simulators.
func NewIOSManager() Manager {
	return &iosManager{}
}

type simctlList struct {
	Devices map[string][]simctlDevice `json:"devices"`
}

type simctlDevice struct {
	State        string `json:"state"`
	IsAvailable  bool   `json:"isAvailable"`
	Name         string `json:"name"`
	UDID         string `json:"udid"`
	DeviceTypeID string `json:"deviceTypeIdentifier"`
	DataPath     string `json:"dataPath"`
	LogPath      string `json:"logPath"`
}

// ToolVersion returns the xcrun version string (e.g. "64").
// Output of `xcrun --version` is "xcrun version 64.\n".
func (m *iosManager) ToolVersion(ctx context.Context) (Platform, string) {
	out, err := exec.CommandContext(ctx, "xcrun", "--version").Output()
	if err != nil {
		return PlatformIOS, "n/a"
	}
	s := strings.TrimSuffix(strings.TrimSpace(string(out)), ".")
	parts := strings.Fields(s)
	if len(parts) > 0 {
		return PlatformIOS, parts[len(parts)-1]
	}
	return PlatformIOS, s
}

// Platform reports the platform this manager handles.
func (m *iosManager) Platform() Platform { return PlatformIOS }

// Boot starts the simulator with the given UDID via `xcrun simctl boot`.
// simctl returns immediately after the boot is initiated; the caller can
// re-list devices to observe the resulting state transition.
func (m *iosManager) Boot(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "boot", id)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("xcrun simctl boot %s: %w", id, err)
		}
		return fmt.Errorf("xcrun simctl boot %s: %w: %s", id, err, msg)
	}
	return nil
}

// Shutdown stops the simulator with the given UDID via `xcrun simctl shutdown`.
func (m *iosManager) Shutdown(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "shutdown", id)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("xcrun simctl shutdown %s: %w", id, err)
		}
		return fmt.Errorf("xcrun simctl shutdown %s: %w: %s", id, err, msg)
	}
	return nil
}

// ListApps runs `xcrun simctl listapps <UDID>` and converts the resulting
// plist to JSON via `plutil` so it can be parsed without an extra dep.
// Returns apps sorted alphabetically by display label.
func (m *iosManager) ListApps(ctx context.Context, id string) ([]App, error) {
	listCmd := exec.CommandContext(ctx, "xcrun", "simctl", "listapps", id)
	listOut, err := listCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("xcrun simctl listapps %s: %w", id, err)
	}

	plutilCmd := exec.CommandContext(ctx, "plutil", "-convert", "json", "-o", "-", "-")
	plutilCmd.Stdin = bytes.NewReader(listOut)
	jsonOut, err := plutilCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("plutil convert json: %w", err)
	}

	var raw map[string]map[string]any
	if err := json.Unmarshal(jsonOut, &raw); err != nil {
		return nil, fmt.Errorf("parse listapps json: %w", err)
	}

	apps := make([]App, 0, len(raw))
	for bundleID, info := range raw {
		app := App{BundleID: bundleID}
		if v, ok := info["CFBundleDisplayName"].(string); ok {
			app.DisplayName = v
		}
		if v, ok := info["CFBundleName"].(string); ok {
			app.Name = v
		}
		if v, ok := info["CFBundleVersion"].(string); ok {
			app.Version = v
		}
		if v, ok := info["CFBundleShortVersionString"].(string); ok {
			app.ShortVersion = v
		}
		if v, ok := info["ApplicationType"].(string); ok {
			app.Type = v
		}
		if v, ok := info["Path"].(string); ok {
			app.Path = v
		}
		apps = append(apps, app)
	}

	sort.SliceStable(apps, func(i, j int) bool {
		a, b := apps[i].Label(), apps[j].Label()
		if a == b {
			return apps[i].BundleID < apps[j].BundleID
		}
		return strings.ToLower(a) < strings.ToLower(b)
	})
	return apps, nil
}

// Info gathers details about a single iOS simulator from the local
// CoreSimulator data directory plus, for booted devices, sw_vers via
// `simctl spawn`. Best-effort: missing data is reported as "—".
func (m *iosManager) Info(ctx context.Context, dev Device) (DeviceInfo, error) {
	info := DeviceInfo{}
	add := func(k, v string) {
		if v == "" {
			v = "—"
		}
		info.Fields = append(info.Fields, InfoField{Key: k, Value: v})
	}

	add("Name", dev.Name)
	add("UDID", dev.ID)
	add("Platform", string(dev.Platform))
	add("Status", string(dev.Status))
	add("Runtime", dev.Version)

	home, err := os.UserHomeDir()
	if err != nil {
		return info, nil
	}
	deviceDir := filepath.Join(home, "Library", "Developer", "CoreSimulator", "Devices", dev.ID)

	if pl, err := readDevicePlist(ctx, filepath.Join(deviceDir, "device.plist")); err == nil {
		if v, ok := pl["deviceType"].(string); ok {
			add("Device Type", trimDevicePrefix(v))
		}
		if v, ok := pl["runtime"].(string); ok {
			add("Runtime ID", trimDevicePrefix(v))
		}
		if v, ok := pl["state"].(float64); ok {
			add("State", simctlStateName(int(v)))
		}
		if v, ok := pl["lastBootedAt"].(string); ok {
			add("Last Booted", v)
		}
	}

	add("Data Path", filepath.Join(deviceDir, "data"))
	if size, err := dirSizeKB(ctx, filepath.Join(deviceDir, "data")); err == nil {
		add("Data Size", formatBytes(size*1024))
	}

	if dev.Status == StatusRunning {
		if out, err := exec.CommandContext(ctx, "xcrun", "simctl", "spawn", dev.ID, "sw_vers").Output(); err == nil {
			for line := range strings.SplitSeq(string(out), "\n") {
				k, v, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				k = strings.TrimSpace(k)
				v = strings.TrimSpace(v)
				switch k {
				case "ProductName":
					add("OS Name", v)
				case "ProductVersion":
					add("OS Version", v)
				case "BuildVersion":
					add("OS Build", v)
				}
			}
		}
		if out, err := exec.CommandContext(ctx, "xcrun", "simctl", "spawn", dev.ID, "uname", "-m").Output(); err == nil {
			add("Architecture", strings.TrimSpace(string(out)))
		}
	}

	return info, nil
}

// readDevicePlist converts a binary/XML plist on disk to JSON via plutil and
// returns it as a generic map.
func readDevicePlist(ctx context.Context, path string) (map[string]any, error) {
	out, err := exec.CommandContext(ctx, "plutil", "-convert", "json", "-o", "-", path).Output()
	if err != nil {
		return nil, fmt.Errorf("plutil %s: %w", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return m, nil
}

// dirSizeKB invokes `du -sk` and returns the directory's apparent size in KB.
func dirSizeKB(ctx context.Context, path string) (int64, error) {
	out, err := exec.CommandContext(ctx, "du", "-sk", path).Output()
	if err != nil {
		return 0, fmt.Errorf("du -sk %s: %w", path, err)
	}
	parts := strings.Fields(string(out))
	if len(parts) < 1 {
		return 0, fmt.Errorf("unexpected du output: %q", string(out))
	}
	kb, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse du size %q: %w", parts[0], err)
	}
	return kb, nil
}

// trimDevicePrefix strips the well-known SimDeviceType / SimRuntime DNS-style
// prefix from a CoreSimulator identifier.
func trimDevicePrefix(s string) string {
	for _, p := range []string{
		"com.apple.CoreSimulator.SimDeviceType.",
		"com.apple.CoreSimulator.SimRuntime.",
	} {
		if strings.HasPrefix(s, p) {
			return s[len(p):]
		}
	}
	return s
}

// simctlStateName maps the integer state field in device.plist to a label.
func simctlStateName(s int) string {
	switch s {
	case 0:
		return "Creating"
	case 1:
		return "Shutdown"
	case 2:
		return "Booting"
	case 3:
		return "Booted"
	case 4:
		return "Shutting Down"
	default:
		return fmt.Sprintf("State(%d)", s)
	}
}

func (m *iosManager) ListDevices(ctx context.Context) ([]Device, error) {
	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "list", "devices", "available", "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run xcrun simctl: %w", err)
	}

	return parseSimctlOutput(output)
}

func parseSimctlOutput(output []byte) ([]Device, error) {
	var list simctlList
	if err := json.Unmarshal(output, &list); err != nil {
		return nil, fmt.Errorf("failed to parse simctl output: %w", err)
	}

	var devices []Device
	for runtime, runtimeDevices := range list.Devices {
		version := parseRuntimeVersion(runtime)
		for _, d := range runtimeDevices {
			status := StatusOff
			if d.State == "Booted" {
				status = StatusRunning
			}

			devices = append(devices, Device{
				ID:       d.UDID,
				Name:     d.Name,
				Platform: PlatformIOS,
				Version:  version,
				Status:   status,
			})
		}
	}

	return devices, nil
}

// parseRuntimeVersion extracts a human-friendly version string from a simctl
// runtime identifier. Examples:
//
//	"com.apple.CoreSimulator.SimRuntime.iOS-18-5"     -> "18.5"
//	"com.apple.CoreSimulator.SimRuntime.watchOS-12-0" -> "12.0"
//	"iOS 17.0"                                        -> "iOS 17.0" (passthrough)
func parseRuntimeVersion(id string) string {
	last := id
	if i := strings.LastIndex(id, "."); i >= 0 {
		last = id[i+1:]
	}
	if _, after, ok := strings.Cut(last, "-"); ok {
		return strings.ReplaceAll(after, "-", ".")
	}
	return last
}
