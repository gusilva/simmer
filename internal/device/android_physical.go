package device

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/electricbubble/gadb"

	"simmer/internal/logging"
)

type physicalAndroidManager struct {
	logger *logging.Logger
}

// NewPhysicalAndroidManager returns a Manager for physical Android devices via goadb.
func NewPhysicalAndroidManager(logger *logging.Logger) Manager {
	return &physicalAndroidManager{logger: logger}
}

func (m *physicalAndroidManager) Platform() Platform { return PlatformAndroid }
func (m *physicalAndroidManager) Kind() DeviceKind   { return KindPhysical }

func (m *physicalAndroidManager) ToolVersion(_ context.Context) (Platform, string) {
	client, err := gadb.NewClient()
	if err != nil {
		return PlatformAndroid, "gadb (adb unavailable)"
	}
	v, err := client.ServerVersion()
	if err != nil {
		return PlatformAndroid, "gadb"
	}
	return PlatformAndroid, fmt.Sprintf("adb-server %d (gadb)", v)
}

// ListDevices returns all physical Android devices seen by the ADB server.
// Emulator serials (emulator-NNNN) are excluded — those belong to androidManager.
// Returns nil, nil when the ADB server is unreachable so the coordinator skips
// gracefully without surfacing an error.
func (m *physicalAndroidManager) ListDevices(_ context.Context) ([]Device, error) {
	client, err := gadb.NewClient()
	if err != nil {
		return nil, nil
	}

	// DeviceSerialList covers all devices including unauthorized ones (requires
	// only 2 fields in the adb output). DeviceList (host:devices-l) requires 4+
	// fields and silently skips unauthorized devices.
	allSerials, err := client.DeviceSerialList()
	if err != nil {
		return nil, nil
	}

	deviceList, err := client.DeviceList()
	if err != nil {
		return nil, nil
	}

	bySerial := make(map[string]gadb.Device, len(deviceList))
	for _, d := range deviceList {
		bySerial[d.Serial()] = d
	}

	var out []Device
	for _, serial := range allSerials {
		if strings.HasPrefix(serial, "emulator-") {
			continue
		}

		d, ok := bySerial[serial]
		if !ok {
			// Serial visible but no attrs — typically unauthorized.
			out = append(out, Device{
				ID:       serial,
				Name:     "(unauthorized — accept USB prompt on device)",
				Platform: PlatformAndroid,
				Status:   StatusRunning,
				Kind:     KindPhysical,
			})
			continue
		}

		state, err := d.State()
		if err != nil || state == gadb.StateOffline || state == gadb.StateDisconnected || state == gadb.StateUnknown {
			continue
		}

		model, _ := d.RunShellCommand("getprop", "ro.product.model")
		version, _ := d.RunShellCommand("getprop", "ro.build.version.release")

		name := strings.TrimSpace(model)
		if name == "" {
			name = serial
		}

		out = append(out, Device{
			ID:       serial,
			Name:     name,
			Platform: PlatformAndroid,
			Version:  strings.TrimSpace(version),
			Status:   StatusRunning,
			Kind:     KindPhysical,
		})
	}

	return out, nil
}

// ListApps returns user-installed apps on a physical Android device via gadb.
func (m *physicalAndroidManager) ListApps(_ context.Context, id string) ([]App, error) {
	d, err := gadbDevice(id)
	if err != nil {
		return nil, err
	}

	pkgOut, err := d.RunShellCommand("pm", "list", "packages", "-3", "-f")
	m.logger.LogExec("adb shell", []string{"pm", "list", "packages", "-3", "-f"}, pkgOut, err)
	if err != nil {
		return nil, fmt.Errorf("pm list packages -3 -f: %w", err)
	}

	type entry struct{ path string }
	byID := map[string]entry{}

	for line := range strings.SplitSeq(strings.TrimSpace(pkgOut), "\n") {
		line = strings.TrimSpace(line)
		after, ok := strings.CutPrefix(line, "package:")
		if !ok {
			continue
		}
		eq := strings.LastIndex(after, "=")
		if eq < 0 {
			continue
		}
		bundleID := strings.Clone(strings.TrimSpace(after[eq+1:]))
		path := strings.Clone(after[:eq])
		if bundleID != "" {
			byID[bundleID] = entry{path: path}
		}
	}

	if len(byID) == 0 {
		return nil, nil
	}

	versions := androidFetchVersions(m.logger, d, byID)

	apps := make([]App, 0, len(byID))
	for bundleID, e := range byID {
		apps = append(apps, App{
			BundleID:     bundleID,
			Path:         e.path,
			Type:         "User",
			ShortVersion: versions[bundleID],
		})
	}

	sort.SliceStable(apps, func(i, j int) bool {
		a, b := strings.ToLower(apps[i].Label()), strings.ToLower(apps[j].Label())
		if a == b {
			return apps[i].BundleID < apps[j].BundleID
		}
		return a < b
	})
	return apps, nil
}

// DeleteApp uninstalls an app from a physical Android device via gadb.
func (m *physicalAndroidManager) DeleteApp(_ context.Context, deviceID, bundleID string) error {
	d, err := gadbDevice(deviceID)
	if err != nil {
		return err
	}
	out, err := d.RunShellCommand("pm", "uninstall", bundleID)
	if err != nil {
		return fmt.Errorf("pm uninstall %s: %w", bundleID, err)
	}
	if strings.Contains(out, "Failure") {
		return fmt.Errorf("pm uninstall %s: %s", bundleID, strings.TrimSpace(out))
	}
	return nil
}

// StreamLogs streams logcat output from a physical Android device.
// Uses the adb subprocess (adb logcat) since gadb lacks streaming shell support.
// The physical device serial (dev.ID) is used directly as the adb serial.
func (m *physicalAndroidManager) StreamLogs(parent context.Context, dev Device, app App) (*LogStream, error) {
	ctx, cancel := context.WithCancel(parent)

	serial := dev.ID

	launchCtx, launchCancel := context.WithTimeout(ctx, 5*time.Second)
	_ = exec.CommandContext(launchCtx, "adb", "-s", serial, "shell",
		"monkey", "-p", app.BundleID,
		"-c", "android.intent.category.LAUNCHER", "1").Run()
	launchCancel()

	logcatArgs := []string{"-s", serial, "logcat", "-v", "time"}
	if pidOut, err := exec.CommandContext(ctx, "adb", "-s", serial,
		"shell", "pidof", "-s", app.BundleID).Output(); err == nil {
		if pid := strings.TrimSpace(string(pidOut)); pid != "" {
			logcatArgs = append(logcatArgs, "--pid="+pid)
		}
	}

	cmd := exec.CommandContext(ctx, "adb", logcatArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("adb logcat: %w", err)
	}

	lines := make(chan string, 256)
	done := make(chan error, 1)

	scan := func(r io.Reader, wg *sync.WaitGroup) {
		defer wg.Done()
		s := bufio.NewScanner(r)
		s.Buffer(make([]byte, 64*1024), 1024*1024)
		for s.Scan() {
			select {
			case lines <- s.Text():
			case <-ctx.Done():
				return
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go scan(stdout, &wg)
	go scan(stderr, &wg)

	go func() {
		wg.Wait()
		close(lines)
		done <- cmd.Wait()
		close(done)
	}()

	return &LogStream{Lines: lines, Done: done, Stop: cancel}, nil
}

// Info returns device metadata for a physical Android device via gadb.
func (m *physicalAndroidManager) Info(_ context.Context, dev Device) (DeviceInfo, error) {
	d, err := gadbDevice(dev.ID)
	if err != nil {
		return DeviceInfo{}, err
	}

	info := DeviceInfo{}
	add := func(k, v string) {
		if v == "" {
			v = "—"
		}
		info.Fields = append(info.Fields, InfoField{Key: k, Value: v})
	}

	add("Name", dev.Name)
	add("Serial", dev.ID)
	add("Platform", string(dev.Platform))
	add("Status", string(dev.Status))

	for _, kv := range []struct{ prop, label string }{
		{"ro.product.model", "Model"},
		{"ro.product.manufacturer", "Manufacturer"},
		{"ro.build.version.release", "Android"},
		{"ro.build.version.sdk", "SDK"},
		{"ro.build.display.id", "Build"},
		{"ro.product.cpu.abi", "ABI"},
	} {
		if val, err := d.RunShellCommand("getprop", kv.prop); err == nil {
			if v := strings.TrimSpace(val); v != "" {
				add(kv.label, v)
			}
		}
	}

	return info, nil
}
