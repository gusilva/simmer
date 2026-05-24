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
	"github.com/danielpaulus/go-ios/ios/installationproxy"
	"github.com/danielpaulus/go-ios/ios/syslog"
	"github.com/danielpaulus/go-ios/ios/zipconduit"

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
