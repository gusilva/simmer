package device

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"

	ios "github.com/danielpaulus/go-ios/ios"
	"github.com/danielpaulus/go-ios/ios/afc"
	"github.com/danielpaulus/go-ios/ios/house_arrest"
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/syslog"
	"github.com/danielpaulus/go-ios/ios/zipconduit"
	"howett.net/plist"

	"simmer/internal/logging"
)

type physicalIOSManager struct {
	logger *logging.Logger
}

// NewPhysicalIOSManager returns a Manager for physical iOS devices via go-ios.
func NewPhysicalIOSManager(logger *logging.Logger) Manager {
	return &physicalIOSManager{logger: logger}
}

func (m *physicalIOSManager) Platform() Platform { return PlatformIOS }
func (m *physicalIOSManager) Kind() DeviceKind   { return KindPhysical }

// ListDevices returns all physical iOS devices visible to usbmuxd.
// Returns nil, nil when usbmuxd is unavailable so the coordinator skips
// gracefully without surfacing an error.
func (m *physicalIOSManager) ListDevices(_ context.Context) ([]Device, error) {
	list, err := ios.ListDevices()
	if err != nil {
		return nil, nil
	}

	var out []Device
	for _, entry := range list.DeviceList {
		udid := entry.Properties.SerialNumber
		if udid == "" {
			continue
		}
		vals, err := ios.GetValues(entry)
		if err != nil {
			// Device locked or not yet trusted.
			out = append(out, Device{
				ID:       udid,
				Name:     "iPhone (locked — unlock and trust this computer)",
				Platform: PlatformIOS,
				Status:   StatusRunning,
				Kind:     KindPhysical,
			})
			continue
		}
		name := vals.Value.DeviceName
		if name == "" {
			name = udid
		}
		out = append(out, Device{
			ID:       udid,
			Name:     name,
			Platform: PlatformIOS,
			Version:  vals.Value.ProductVersion,
			Status:   StatusRunning,
			Kind:     KindPhysical,
		})
	}
	return out, nil
}

// ListApps returns user-installed apps on a physical iOS device via installationproxy.
func (m *physicalIOSManager) ListApps(_ context.Context, id string) ([]App, error) {
	entry, err := goIOSDevice(id)
	if err != nil {
		return nil, err
	}
	conn, err := installationproxy.New(entry)
	if err != nil {
		return nil, fmt.Errorf("installation proxy: %w", err)
	}
	defer conn.Close()

	appInfos, err := conn.BrowseUserApps()
	m.logger.LogExec("go-ios", []string{"installationproxy", "BrowseUserApps"}, "", err)
	if err != nil {
		return nil, fmt.Errorf("browse user apps: %w", err)
	}

	out := make([]App, 0, len(appInfos))
	for _, info := range appInfos {
		bundleID := info.CFBundleIdentifier()
		if bundleID == "" {
			continue
		}
		displayName := ""
		if v, ok := info[installationproxy.CFBundleDisplayName].(string); ok {
			displayName = v
		}
		// React Native detection needs an AFC round-trip (slow) — run it
		// lazily via DetectReactNative once the list below is on screen.
		out = append(out, App{
			BundleID:     bundleID,
			DisplayName:  displayName,
			Name:         info.CFBundleName(),
			ShortVersion: info.CFBundleShortVersionString(),
			Path:         info.Path(),
			Type:         "User",
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i].Label()), strings.ToLower(out[j].Label())
		if a == b {
			return out[i].BundleID < out[j].BundleID
		}
		return a < b
	})
	return out, nil
}

// DetectReactNative implements ReactNativeDetector for physical iOS devices.
func (m *physicalIOSManager) DetectReactNative(_ context.Context, id string, app App) bool {
	entry, err := goIOSDevice(id)
	if err != nil {
		return false
	}
	return isReactNativeBundlePhysical(entry, app.BundleID)
}

// isReactNativeBundlePhysical is a React Native check for physical iOS
// devices via the house_arrest AFC service. house_arrest only vends the
// app's data container (Documents/Library), never its bundle container —
// so main.jsbundle/hermes.framework, which live in the bundle container,
// are never visible this way (verified against a real device: AFC error
// code 8, object not found, for both). Checking those directly always
// returns false and was silently dead code.
//
// Instead this reads the app's own NSUserDefaults plist
// (Library/Preferences/<bundleID>.plist), which *is* in the data container.
// React Native's native iOS modules write several "RCT"-prefixed keys there
// — RCTI18nUtil's RTL-flip flag is set on every launch regardless of build
// config, RCTDevMenu once the app has run in dev mode — and no non-RN app
// plausibly has keys under that namespace, so their presence is a reliable
// signal. (Also verified: house_arrest's VendContainer only succeeds for
// dev-signed apps at all — every App Store-signed app on the test device
// got InstallationLookupFailed — which happens to match this tool's actual
// audience: apps someone is actively developing.)
func isReactNativeBundlePhysical(entry ios.DeviceEntry, bundleID string) bool {
	client, err := house_arrest.New(entry, bundleID)
	if err != nil {
		return false
	}
	defer client.Close()

	f, err := client.Open("/Library/Preferences/"+bundleID+".plist", afc.READ_ONLY)
	if err != nil {
		return false
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return false
	}

	var prefs map[string]any
	if _, err := plist.Unmarshal(data, &prefs); err != nil {
		return false
	}
	for key := range prefs {
		if strings.HasPrefix(key, "RCT") {
			return true
		}
	}
	return false
}

// DeleteApp uninstalls an app from a physical iOS device via installationproxy.
func (m *physicalIOSManager) DeleteApp(_ context.Context, deviceID, bundleID string) error {
	entry, err := goIOSDevice(deviceID)
	if err != nil {
		return err
	}
	conn, err := installationproxy.New(entry)
	if err != nil {
		return fmt.Errorf("installation proxy: %w", err)
	}
	defer conn.Close()
	return conn.Uninstall(bundleID)
}

// LaunchApp starts an already-installed app via `xcrun devicectl device
// process launch --terminate-existing`, which also covers relaunching an
// already-running instance in one call.
func (m *physicalIOSManager) LaunchApp(ctx context.Context, deviceID, bundleID string) error {
	args := []string{"devicectl", "device", "process", "launch", "--device", deviceID, "--terminate-existing", bundleID}
	cmd := exec.CommandContext(ctx, "xcrun", args...)
	out, err := cmd.CombinedOutput()
	m.logger.LogExec("xcrun", args, string(out), err)
	if err != nil {
		return fmt.Errorf("launch app: %w", err)
	}
	return nil
}

// TriggerDevMenu opens the React Native dev menu via a Metro dev-server
// WebSocket broadcast (see triggerDevMenuViaMetro) — the only mechanism that
// reaches a physical device, since the shake gesture RN listens for is an
// in-process notification and devicectl has no UI-automation equivalent for
// real hardware.
func (m *physicalIOSManager) TriggerDevMenu(ctx context.Context, _, metroPort string) error {
	port := metroPort
	if port == "" {
		port = defaultMetroPort
	}
	err := triggerDevMenuViaMetro(ctx, port)
	m.logger.LogExec("metro-ws", []string{"devMenu", "localhost:" + port}, "", err)
	if err != nil {
		return fmt.Errorf("trigger dev menu: %w", err)
	}
	return nil
}

// StreamConsoleLog launches app via `xcrun devicectl device process launch
// --console --terminate-existing`, which atomically relaunches it and
// streams its combined stdout/stderr back over the same devicectl process —
// the same command a developer would pipe through `tee` by hand. This is
// the RN debugging menu's physical-iOS "log to file" capture: it reaches the
// app's actual console output (where console.log/Hermes writes) instead of
// filtering the unified-logging syslog stream, which StreamLogs uses and
// which often misses or drowns out RN's own output.
//
// hostPort re-applies a previously configured bundler location (see
// devicectlLaunchArgs) — without it, this relaunch would silently drop back
// to the app's compiled-in default and never reach Metro.
func (m *physicalIOSManager) StreamConsoleLog(parent context.Context, deviceID, bundleID, hostPort string) (*LogStream, error) {
	ctx, cancel := context.WithCancel(parent)

	args := devicectlLaunchArgs(deviceID, bundleID, hostPort, "--console")
	cmd := exec.CommandContext(ctx, "xcrun", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout // combine, like the `2>&1` in the equivalent shell command

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("devicectl launch --console: %w", err)
	}

	lines := make(chan string, 256)
	done := make(chan error, 1)

	go func() {
		defer close(lines)
		s := bufio.NewScanner(stdout)
		s.Buffer(make([]byte, 64*1024), 1024*1024)
	scanLoop:
		for s.Scan() {
			select {
			case lines <- s.Text():
			case <-ctx.Done():
				break scanLoop
			}
		}
		var waitErr error
		if ctx.Err() == nil {
			waitErr = cmd.Wait()
		}
		m.logger.LogExec("xcrun", args, "", waitErr)
		if ctx.Err() != nil {
			done <- nil
		} else {
			done <- waitErr
		}
		close(done)
	}()

	return &LogStream{
		Lines: lines,
		Done:  done,
		Stop:  cancel,
	}, nil
}

// devicectlLaunchArgs builds `devicectl device process launch
// --terminate-existing` args for deviceID/bundleID. extra is inserted right
// after "launch" (e.g. "--console"). When hostPort is non-empty, it's
// appended as a `-RCT_jsLocation host:port` command-line argument:
// Foundation's NSUserDefaults argument domain treats a leading `-key value`
// launch argument as a (session-only, non-persistent) defaults override,
// which is the only scriptable way to reach an app's preferences on
// physical hardware — there is no `defaults write` equivalent for real
// devices in devicectl. Every caller that relaunches the app (bundler
// config, and the "log to file" console capture) must pass the same
// hostPort or the override is lost on that relaunch.
func devicectlLaunchArgs(deviceID, bundleID, hostPort string, extra ...string) []string {
	args := append([]string{"devicectl", "device", "process", "launch"}, extra...)
	args = append(args, "--device", deviceID, "--terminate-existing", bundleID)
	if hostPort != "" {
		args = append(args, "--", "-RCT_jsLocation", hostPort)
	}
	return args
}

// SetBundlerLocation relaunches the app with hostPort applied via
// devicectlLaunchArgs.
func (m *physicalIOSManager) SetBundlerLocation(ctx context.Context, deviceID, bundleID, hostPort string) error {
	args := devicectlLaunchArgs(deviceID, bundleID, hostPort)
	cmd := exec.CommandContext(ctx, "xcrun", args...)
	out, err := cmd.CombinedOutput()
	m.logger.LogExec("xcrun", args, string(out), err)
	if err != nil {
		return fmt.Errorf("set bundler location: %w", err)
	}
	return nil
}

// StreamLogs streams syslog output from a physical iOS device, filtered to
// lines containing the app's bundle ID or name. conn.Close is called when Stop
// is invoked to unblock the blocking ReadLogMessage call.
func (m *physicalIOSManager) StreamLogs(parent context.Context, dev Device, app App) (*LogStream, error) {
	ctx, cancel := context.WithCancel(parent)

	entry, err := goIOSDevice(dev.ID)
	if err != nil {
		cancel()
		return nil, err
	}

	conn, err := syslog.New(entry)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("syslog: %w", err)
	}

	filterTerms := []string{app.BundleID}
	if app.Name != "" {
		filterTerms = append(filterTerms, app.Name)
	}

	lines := make(chan string, 256)
	done := make(chan error, 1)

	stop := func() {
		cancel()
		conn.Close() // unblocks the blocking ReadLogMessage call
	}

	go func() {
		defer close(lines)
		defer close(done)
		for {
			msg, err := conn.ReadLogMessage()
			if err != nil {
				if ctx.Err() != nil {
					done <- nil
				} else if err == io.EOF {
					done <- nil
				} else {
					done <- err
				}
				return
			}
			msg = strings.TrimRight(msg, "\x00")
			if msg == "" {
				continue
			}
			relevant := false
			for _, term := range filterTerms {
				if strings.Contains(msg, term) {
					relevant = true
					break
				}
			}
			if !relevant {
				continue
			}
			select {
			case lines <- msg:
			case <-ctx.Done():
				done <- nil
				return
			}
		}
	}()

	return &LogStream{Lines: lines, Done: done, Stop: stop}, nil
}

// StartInstall builds the project for the physical device with xcodebuild, then
// installs the signed .app bundle via go-ios zipconduit over USB.
func (m *physicalIOSManager) StartInstall(dev Device, path, scheme string) (*BuildStream, error) {
	events := make(chan string, 64)
	done := make(chan error, 1)
	ctx, cancel := newBuildContext()

	go func() {
		defer close(events)
		defer cancel()

		p := expandPath(path)
		lower := strings.ToLower(p)

		var appPath string
		if strings.HasSuffix(lower, ".app") || strings.HasSuffix(lower, ".ipa") {
			appPath = p
		} else {
			derivedData, err := os.MkdirTemp("", "simmer-build-*")
			if err != nil {
				done <- fmt.Errorf("create build dir: %w", err)
				return
			}
			defer os.RemoveAll(derivedData)

			sendEvent(ctx, events, "Building "+scheme+"…")
			built, err := xcodeBuildPhysicalStream(ctx, m.logger, events, p, scheme, dev.ID, derivedData)
			if err != nil {
				done <- err
				return
			}
			appPath = built
		}

		sendEvent(ctx, events, "Installing…")
		entry, err := goIOSDevice(dev.ID)
		if err != nil {
			done <- err
			return
		}
		conn, err := zipconduit.New(entry)
		m.logger.LogExec("go-ios", []string{"zipconduit", "connect"}, "", err)
		if err != nil {
			done <- fmt.Errorf("zipconduit: %w", err)
			return
		}
		err = conn.SendFile(appPath)
		m.logger.LogExec("go-ios", []string{"zipconduit", "SendFile", appPath}, "", err)
		if err != nil {
			done <- fmt.Errorf("install: %w", err)
			return
		}
		done <- nil
	}()

	return &BuildStream{Events: events, Done: done, Stop: cancel}, nil
}

// xcodeBuildPhysicalStream runs xcodebuild targeting a physical iOS device by
// UDID, streams filtered output to events, and returns the path to the built
// .app bundle on success.
func xcodeBuildPhysicalStream(ctx context.Context, logger *logging.Logger, events chan<- string, projectPath, scheme, deviceID, derivedData string) (string, error) {
	lower := strings.ToLower(projectPath)
	projectFlag := "-project"
	if strings.HasSuffix(lower, ".xcworkspace") {
		projectFlag = "-workspace"
	}
	args := []string{
		projectFlag, projectPath,
		"-scheme", scheme,
		"-configuration", "Debug",
		"-destination", "platform=iOS,id=" + deviceID,
		"-allowProvisioningUpdates",
		"-derivedDataPath", derivedData,
		"build",
	}
	cmd := exec.CommandContext(ctx, "xcodebuild", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("xcodebuild stdout pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		logger.LogStart("xcodebuild", args, err)
		return "", fmt.Errorf("xcodebuild start: %w", err)
	}
	logger.LogStart("xcodebuild", args, nil)

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if line := filterBuildLine(scanner.Text()); line != "" {
			sendEvent(ctx, events, line)
		}
	}

	if err := cmd.Wait(); err != nil {
		logger.LogExec("xcodebuild", args, stderrBuf.String(), err)
		msg := trimBuildOutput(stderrBuf.String())
		if msg == "" {
			return "", fmt.Errorf("xcodebuild: %w", err)
		}
		return "", fmt.Errorf("xcodebuild: %w: %s", err, msg)
	}
	return findBuiltApp(derivedData)
}

// Info returns device metadata for a physical iOS device via lockdown GetValues.
func (m *physicalIOSManager) Info(_ context.Context, dev Device) (DeviceInfo, error) {
	entry, err := goIOSDevice(dev.ID)
	if err != nil {
		return DeviceInfo{}, err
	}
	vals, err := ios.GetValues(entry)
	if err != nil {
		return DeviceInfo{}, fmt.Errorf("lockdown get values: %w", err)
	}

	info := DeviceInfo{}
	add := func(k, v string) {
		if v == "" {
			v = "—"
		}
		info.Fields = append(info.Fields, InfoField{Key: k, Value: v})
	}

	v := vals.Value
	add("Name", dev.Name)
	add("UDID", dev.ID)
	add("Platform", string(dev.Platform))
	add("Status", string(dev.Status))
	add("Device Name", v.DeviceName)
	add("iOS Version", v.ProductVersion)
	add("Build", v.BuildVersion)
	add("Product Type", v.ProductType)
	add("Model Number", v.ModelNumber)
	add("Serial Number", v.SerialNumber)
	add("WiFi MAC", v.WiFiAddress)
	add("Architecture", v.CPUArchitecture)
	add("Device Class", v.DeviceClass)
	return info, nil
}

// goIOSDevice returns the DeviceEntry for the given UDID by querying usbmuxd.
func goIOSDevice(udid string) (ios.DeviceEntry, error) {
	list, err := ios.ListDevices()
	if err != nil {
		return ios.DeviceEntry{}, fmt.Errorf("usbmuxd unavailable: %w", err)
	}
	for _, entry := range list.DeviceList {
		if entry.Properties.SerialNumber == udid {
			return entry, nil
		}
	}
	return ios.DeviceEntry{}, fmt.Errorf("iOS device %s not connected", udid)
}
