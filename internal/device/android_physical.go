package device

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
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
				Problem:  "unauthorized: accept the USB debugging prompt on the device",
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

	// Version names and React Native status are fetched separately
	// (FetchAppVersions, DetectReactNative) — both need an extra device
	// round-trip, so this list returns the moment `pm list packages` itself
	// is parsed instead of blocking on them too.
	apps := make([]App, 0, len(byID))
	for bundleID, e := range byID {
		apps = append(apps, App{
			BundleID: bundleID,
			Path:     e.path,
			Type:     "User",
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

// FetchAppVersions implements AppVersionFetcher for physical Android devices.
func (m *physicalAndroidManager) FetchAppVersions(_ context.Context, id string, bundleIDs []string) map[string]string {
	d, err := gadbDevice(id)
	if err != nil {
		return nil
	}
	return androidFetchVersions(m.logger, d, bundleIDs)
}

// DetectReactNative implements ReactNativeDetector for physical Android devices.
func (m *physicalAndroidManager) DetectReactNative(ctx context.Context, id string, app App) bool {
	d, err := gadbDevice(id)
	if err != nil {
		return false
	}
	return detectReactNativeApp(ctx, m.logger, d, app.BundleID, app.Path)
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

// TerminateApp stops a running app via `adb shell am force-stop`.
func (m *physicalAndroidManager) TerminateApp(_ context.Context, deviceID, bundleID string) error {
	d, err := gadbDevice(deviceID)
	if err != nil {
		return err
	}
	out, err := d.RunShellCommand("am", "force-stop", bundleID)
	m.logger.LogExec("adb shell", []string{"am", "force-stop", bundleID}, out, err)
	return err
}

// LaunchApp starts an already-installed app, resolving its launcher activity
// first (device.App only stores the package name).
func (m *physicalAndroidManager) LaunchApp(_ context.Context, deviceID, bundleID string) error {
	d, err := gadbDevice(deviceID)
	if err != nil {
		return err
	}
	activity, err := androidResolveLauncherActivity(d, bundleID)
	if err != nil {
		return fmt.Errorf("resolve launcher activity: %w", err)
	}
	component := bundleID + "/" + activity
	out, err := d.RunShellCommand("am", "start", "-n", component)
	m.logger.LogExec("adb shell", []string{"am", "start", "-n", component}, out, err)
	return err
}

// SetBundlerLocation patches the app's default SharedPreferences with the
// debug_http_host key React Native reads for the Metro bundler address, then
// force-stops and relaunches the app so it picks up the change.
func (m *physicalAndroidManager) SetBundlerLocation(ctx context.Context, deviceID, bundleID, hostPort string) error {
	d, err := gadbDevice(deviceID)
	if err != nil {
		return err
	}
	if err := androidWriteDebugHost(m.logger, d, bundleID, hostPort); err != nil {
		return err
	}
	out, err := d.RunShellCommand("am", "force-stop", bundleID)
	m.logger.LogExec("adb shell", []string{"am", "force-stop", bundleID}, out, err)
	return m.LaunchApp(ctx, deviceID, bundleID)
}

// TriggerDevMenu opens the React Native dev menu. It broadcasts a "devMenu"
// message over Metro's dev-server WebSocket (see triggerDevMenuViaMetro),
// because many OEM ROMs (e.g. Xiaomi MIUI) remap KEYCODE_MENU (82) to the
// recent-apps screen, so the key event never reaches the app.
//
// The broadcast only reaches the app if it is connected to that Metro, so the
// port is the one the foreground app is configured for (debug_http_host,
// readable via run-as on debuggable apps). Failing that: an explicit
// metroPort, the device's `adb reverse` tunnels, then 8081. If the app's
// own Metro port has nothing listening, an error says so rather than
// reporting a broadcast nobody receives. If Metro is unreachable it falls
// back to the hardware Menu key, except on OEMs known to remap it.
func (m *physicalAndroidManager) TriggerDevMenu(ctx context.Context, deviceID, metroPort string) error {
	d, err := gadbDevice(deviceID)
	if err != nil {
		return err
	}

	appPort := ""
	if pkg := foregroundPackage(d); pkg != "" {
		appPort = appDebugHostPort(d, pkg)
		m.logger.LogExec("adb shell", []string{"run-as", pkg, "cat", "<debug_http_host>"}, appPort, nil)
	}

	var ports []string
	switch {
	case appPort != "":
		ports = []string{appPort}
	case metroPort != "":
		ports = []string{metroPort}
	default:
		ports = append(reversedTCPPorts(ctx, deviceID), defaultMetroPort)
	}

	var metroErr error
	for _, port := range ports {
		err := triggerDevMenuViaMetro(ctx, port)
		m.logger.LogExec("metro-ws", []string{"devMenu", "localhost:" + port}, "", err)
		if err == nil {
			return nil
		}
		if metroErr == nil {
			metroErr = err
		}
	}
	if appPort != "" {
		return fmt.Errorf("app expects Metro on :%s but nothing is listening (start Metro there or set Bundler Location): %w", appPort, metroErr)
	}
	if mfr, _ := d.RunShellCommand("getprop", "ro.product.manufacturer"); remapsMenuKey(mfr) {
		return fmt.Errorf("metro not reachable (start Metro or set Bundler Location): %w", metroErr)
	}
	out, kerr := d.RunShellCommand("input", "keyevent", "82")
	m.logger.LogExec("adb shell", []string{"input", "keyevent", "82"}, out, kerr)
	return kerr
}

var (
	resumedActivityRE = regexp.MustCompile(`(?:mResumedActivity|topResumedActivity)[^\n]*? ([A-Za-z0-9_.]+)/`)
	debugHostValueRE  = regexp.MustCompile(`<string name="debug_http_host">([^<]*)</string>`)
)

// foregroundPackage returns the package of the resumed activity, or "".
func foregroundPackage(d gadb.Device) string {
	out, err := d.RunShellCommand("dumpsys", "activity", "activities")
	if err != nil {
		return ""
	}
	if mm := resumedActivityRE.FindStringSubmatch(out); mm != nil {
		return mm[1]
	}
	return ""
}

// appDebugHostPort returns the port of pkg's saved debug_http_host, or ""
// when the app isn't debuggable or has none set.
func appDebugHostPort(d gadb.Device, pkg string) string {
	prefs := fmt.Sprintf("shared_prefs/%s_preferences.xml", pkg)
	out, err := d.RunShellCommand("run-as", pkg, "cat", prefs)
	if err != nil {
		return ""
	}
	return parseDebugHostPort(out)
}

// parseDebugHostPort extracts the port from a prefs XML's debug_http_host.
func parseDebugHostPort(xml string) string {
	mm := debugHostValueRE.FindStringSubmatch(xml)
	if mm == nil {
		return ""
	}
	host := strings.TrimSpace(mm[1])
	i := strings.LastIndex(host, ":")
	if i < 0 || i == len(host)-1 {
		return ""
	}
	return host[i+1:]
}

// remapsMenuKey reports whether the manufacturer's ROM is known to map
// KEYCODE_MENU to recent apps instead of delivering it to the foreground app.
func remapsMenuKey(manufacturer string) bool {
	switch strings.ToLower(strings.TrimSpace(manufacturer)) {
	case "xiaomi", "redmi", "poco":
		return true
	}
	return false
}

// reversedTCPPorts returns the host-side ports of the device's
// `adb reverse` tunnels, which is how a device normally reaches Metro.
func reversedTCPPorts(ctx context.Context, serial string) []string {
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "adb", "-s", serial, "reverse", "--list").Output()
	if err != nil {
		return nil
	}
	return parseReversePorts(string(out))
}

// parseReversePorts extracts host ports from `adb reverse --list` lines of
// the form "<serial> tcp:<devicePort> tcp:<hostPort>".
func parseReversePorts(out string) []string {
	var ports []string
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 || !strings.HasPrefix(f[2], "tcp:") {
			continue
		}
		ports = append(ports, strings.TrimPrefix(f[2], "tcp:"))
	}
	return ports
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
