package device

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type androidManager struct{}

// NewAndroidManager returns a Manager implementation for Android emulators.
func NewAndroidManager() Manager {
	return &androidManager{}
}

// Platform reports the platform this manager handles.
func (m *androidManager) Platform() Platform { return PlatformAndroid }

// Boot launches the emulator for the given AVD name. The emulator process is
// started detached (we don't wait for it) because it runs until explicitly
// closed; the context is intentionally not forwarded to the child process so
// the 30-second boot timeout doesn't kill the emulator window.
func (m *androidManager) Boot(_ context.Context, id string) error {
	cmd := exec.Command("emulator", "-avd", id) //nolint:gosec
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("emulator -avd %s: %w", id, err)
	}

	go cmd.Wait()
	return nil
}

// Shutdown sends a kill command to the running emulator whose AVD name matches
// id. It maps the AVD name back to an adb serial via `adb -s <serial> emu avd
// name`, then issues `adb -s <serial> emu kill`.
func (m *androidManager) Shutdown(ctx context.Context, id string) error {
	serial, err := m.findSerial(ctx, id)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "adb", "-s", serial, "emu", "kill")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("adb -s %s emu kill: %w", serial, err)
		}

		return fmt.Errorf("adb -s %s emu kill: %w: %s", serial, err, msg)
	}

	return nil
}

// findSerial returns the adb serial (e.g. "emulator-5554") for the running
// emulator whose AVD name matches avdName.
func (m *androidManager) findSerial(ctx context.Context, avdName string) (string, error) {
	out, err := exec.CommandContext(ctx, "adb", "devices").Output()
	if err != nil {
		return "", fmt.Errorf("adb devices: %w", err)
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if !strings.Contains(line, "\tdevice") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		serial := parts[0]
		nameOut, err := exec.CommandContext(ctx, "adb", "-s", serial, "emu", "avd", "name").Output()
		if err != nil {
			continue
		}

		// Output is "<name>\nOK\n" — take first line only.
		firstLine := strings.TrimSpace(strings.SplitN(string(nameOut), "\n", 2)[0])
		if firstLine == avdName {
			return serial, nil
		}
	}

	return "", fmt.Errorf("no running emulator found for AVD %q", avdName)
}

// ToolVersion returns the adb version string (e.g. "1.0.41").
// First line of `adb version` is "Android Debug Bridge version 1.0.41".
func (m *androidManager) ToolVersion(ctx context.Context) (Platform, string) {
	out, err := exec.CommandContext(ctx, "adb", "version").Output()
	if err != nil {
		return PlatformAndroid, "n/a"
	}

	line := strings.SplitN(string(out), "\n", 2)[0]
	const prefix = "Android Debug Bridge version "

	return PlatformAndroid, strings.TrimPrefix(strings.TrimSpace(line), prefix)
}

func (m *androidManager) ListDevices(ctx context.Context) ([]Device, error) {
	// 1. Get defined emulators
	avds, err := m.getAVDs(ctx)
	if err != nil {
		return nil, err
	}

	// 2. Get running devices via adb
	running, err := m.getRunningDevices(ctx)
	if err != nil {
		// If adb fails, we still return the AVDs with "Shutdown" status
		running = make(map[string]bool)
	}

	var devices []Device
	for _, name := range avds {
		status := StatusOff
		// Note: matching AVD name to adb device name can be tricky
		// Usually emulator-5554 style, but we'll do a simple check
		if running[name] {
			status = StatusRunning
		}

		devices = append(devices, Device{
			ID:       name,
			Name:     name,
			Platform: PlatformAndroid,
			Version:  "Unknown", // AVD list doesn't easily give version without more commands
			Status:   status,
		})
	}

	return devices, nil
}

func (m *androidManager) getAVDs(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "emulator", "-list-avds")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run emulator -list-avds: %w", err)
	}

	return parseAVDs(output), nil
}

func parseAVDs(output []byte) []string {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var avds []string
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			avds = append(avds, trimmed)
		}
	}
	return avds
}

func (m *androidManager) getRunningDevices(ctx context.Context) (map[string]bool, error) {
	cmd := exec.CommandContext(ctx, "adb", "devices")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run adb devices: %w", err)
	}

	running := make(map[string]bool)
	lines := strings.SplitSeq(string(output), "\n")
	for line := range lines {
		if strings.Contains(line, "\tdevice") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				// This is the serial, e.g., emulator-5554
				// Mapping this back to AVD name requires 'adb -s <serial> emu avd name'
				// For now, we'll just mark it as potentially running if we find it
				serial := parts[0]

				// Try to get the actual AVD name for this serial.
				// Output is "<name>\nOK\n" — take first line only.
				nameCmd := exec.CommandContext(ctx, "adb", "-s", serial, "emu", "avd", "name")
				nameOut, err := nameCmd.Output()
				if err == nil {
					firstLine := strings.TrimSpace(strings.SplitN(string(nameOut), "\n", 2)[0])
					if firstLine != "" {
						running[firstLine] = true
						continue
					}
				}
				running[serial] = true
			}
		}
	}
	return running, nil
}

// Info implements InfoLister for Android emulators. It reads static metadata
// from the AVD config.ini on disk and, when the device is running, queries
// live properties via `adb shell getprop`.
func (m *androidManager) Info(ctx context.Context, dev Device) (DeviceInfo, error) {
	info := DeviceInfo{}
	add := func(k, v string) {
		if v == "" {
			v = "—"
		}
		info.Fields = append(info.Fields, InfoField{Key: k, Value: v})
	}

	add("Name", dev.Name)
	add("AVD", dev.ID)
	add("Platform", string(dev.Platform))
	add("Status", string(dev.Status))

	// Parse config.ini for static AVD metadata.
	cfg := avdConfig(dev.ID)

	if api := apiFromSysdir(cfg["image.sysdir.1"]); api != "" {
		add("API Level", api)
	}
	if v := cfg["tag.display"]; v != "" {
		add("Type", v)
	}
	if v := cfg["abi.type"]; v != "" {
		add("Architecture", v)
	}
	if v := cfg["hw.cpu.ncore"]; v != "" {
		add("CPU Cores", v)
	}
	if ram := cfg["hw.ramSize"]; ram != "" {
		add("RAM", ram+" MB")
	}
	if w, h := cfg["hw.lcd.width"], cfg["hw.lcd.height"]; w != "" && h != "" {
		add("Resolution", w+"×"+h)
	}
	if d := cfg["hw.lcd.density"]; d != "" {
		add("Density", d+" dpi")
	}
	if sz := cfg["disk.dataPartition.size"]; sz != "" {
		add("Storage", formatPartitionSize(sz))
	}
	if dev := cfg["hw.device.name"]; dev != "" {
		add("Device", dev)
	}

	// Live properties — only when the emulator is running.
	if dev.Status == StatusRunning {
		if serial, err := m.findSerial(ctx, dev.ID); err == nil {
			props := adbProps(ctx, serial, []string{
				"ro.build.version.release",
				"ro.build.version.sdk",
				"ro.build.display.id",
				"ro.product.model",
			})
			if v := props["ro.build.version.release"]; v != "" {
				add("Android", v)
			}
			if v := props["ro.build.version.sdk"]; v != "" {
				add("SDK", v)
			}
			if v := props["ro.build.display.id"]; v != "" {
				add("Build", v)
			}
			if v := props["ro.product.model"]; v != "" {
				add("Model", v)
			}
			add("ADB Serial", serial)
		}
	}

	return info, nil
}

// avdConfig reads ~/.android/avd/<name>.avd/config.ini and returns a key→value
// map. Missing file or parse errors produce an empty map (best-effort).
func avdConfig(avdName string) map[string]string {
	home, err := os.UserHomeDir()
	if err != nil {
		return map[string]string{}
	}
	path := filepath.Join(home, ".android", "avd", avdName+".avd", "config.ini")
	f, err := os.Open(path)
	if err != nil {
		return map[string]string{}
	}
	defer f.Close()

	cfg := make(map[string]string)
	s := bufio.NewScanner(f)
	for s.Scan() {
		k, v, ok := strings.Cut(s.Text(), "=")
		if !ok {
			continue
		}
		cfg[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return cfg
}

// apiFromSysdir extracts the API level from a sysdir path like
// "system-images/android-33/google_apis_playstore/x86_64/".
func apiFromSysdir(sysdir string) string {
	for _, part := range strings.Split(sysdir, "/") {
		if after, ok := strings.CutPrefix(part, "android-"); ok && after != "" {
			return after
		}
	}
	return ""
}

// adbProps fetches the given getprop keys from a running emulator in a single
// shell invocation and returns a key→value map.
func adbProps(ctx context.Context, serial string, keys []string) map[string]string {
	// Build a one-liner: getprop k1; getprop k2; ...
	cmds := make([]string, len(keys))
	for i, k := range keys {
		cmds[i] = "getprop " + k
	}
	out, err := exec.CommandContext(ctx, "adb", "-s", serial, "shell",
		strings.Join(cmds, "; ")).Output()
	if err != nil {
		return map[string]string{}
	}
	result := make(map[string]string, len(keys))
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i, k := range keys {
		if i < len(lines) {
			result[k] = strings.TrimSpace(lines[i])
		}
	}
	return result
}

// formatPartitionSize converts a raw size string (bytes or with K/M/G suffix)
// from config.ini into a human-readable form.
func formatPartitionSize(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return s
	}
	suffix := s[len(s)-1]
	switch suffix {
	case 'K', 'k':
		return s[:len(s)-1] + " KB"
	case 'M', 'm':
		return s[:len(s)-1] + " MB"
	case 'G', 'g':
		return s[:len(s)-1] + " GB"
	}
	// Raw bytes
	var b int64
	fmt.Sscanf(s, "%d", &b)
	if b == 0 {
		return s
	}
	return formatBytes(b)
}
