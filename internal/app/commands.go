package app

import (
	"context"
	"time"

	"simmer/internal/device"
	"simmer/internal/ui"

	tea "charm.land/bubbletea/v2"
)

func (m model) fetchDevicesCmd() tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return discoveryMsg(coord.Discover(ctx))
	}
}

func (m model) fetchDeviceTypesCmd() tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		types, err := coord.ListDeviceTypes(ctx, device.PlatformIOS)
		return deviceTypesMsg{types: types, err: err}
	}
}

func (m model) fetchRuntimesCmd() tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		runtimes, err := coord.ListRuntimes(ctx, device.PlatformIOS)
		return runtimesMsg{runtimes: runtimes, err: err}
	}
}

func (m model) createIOSSimulatorCmd(name, deviceTypeID, runtimeID string) tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		udid, err := coord.Create(ctx, device.PlatformIOS, name, deviceTypeID, runtimeID)
		return createSimulatorResultMsg{name: name, udid: udid, err: err}
	}
}

func (m model) deleteDeviceCmd(dev device.Device) tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := coord.Delete(ctx, dev)
		return deleteSimulatorResultMsg{name: dev.Name, err: err}
	}
}

func (m model) fetchAndroidSystemImagesCmd() tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		imgs, err := coord.ListRuntimes(ctx, device.PlatformAndroid)
		return androidSystemImagesMsg{images: imgs, err: err}
	}
}

func (m model) fetchAndroidDeviceProfilesCmd() tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		profiles, err := coord.ListDeviceTypes(ctx, device.PlatformAndroid)
		return androidDeviceProfilesMsg{profiles: profiles, err: err}
	}
}

func (m model) createAndroidEmulatorCmd(name, systemImagePkg, deviceProfileID string) tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		_, err := coord.Create(ctx, device.PlatformAndroid, name, deviceProfileID, systemImagePkg)
		return createAndroidEmulatorResultMsg{name: name, err: err}
	}
}

func (m model) bootDeviceCmd(dev device.Device) tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return bootResultMsg{device: dev, err: coord.Boot(ctx, dev)}
	}
}

func (m model) shutdownDeviceCmd(dev device.Device) tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return shutdownResultMsg{device: dev, err: coord.Shutdown(ctx, dev)}
	}
}

func (m model) loadIOSAppFileTreeCmd(dev device.Device, app device.App) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		fs := device.NewIOSAppFileSystem(app.BundleID)
		root, err := fs.Tree(ctx, dev)
		return fileTreeMsg{device: dev, root: root, err: err}
	}
}

func (m model) loadIOSRootFileTreeCmd(dev device.Device) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		fs := device.NewIOSRootFileSystem()
		root, err := fs.Tree(ctx, dev)
		return fileTreeMsg{device: dev, root: root, err: err}
	}
}

func (m model) loadAndroidFileTreeCmd(dev device.Device, app device.App) tea.Cmd {
	logger := m.logger
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		fs := device.NewAndroidFileSystem(app.BundleID, logger)
		root, err := fs.Tree(ctx, dev)
		return fileTreeMsg{device: dev, root: root, err: err}
	}
}

func (m model) loadAndroidRootFileTreeCmd(dev device.Device) tea.Cmd {
	logger := m.logger
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		fs := device.NewAndroidRootFileSystem(logger)
		root, err := fs.Tree(ctx, dev)
		return fileTreeMsg{device: dev, root: root, err: err}
	}
}

func (m model) loadIOSPhysicalFileTreeCmd(dev device.Device) tea.Cmd {
	logger := m.logger
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		fs := device.NewIOSPhysicalFileSystem(logger)
		root, err := fs.Tree(ctx, dev)
		return fileTreeMsg{device: dev, root: root, err: err}
	}
}

func (m model) loadIOSPhysicalAppFileTreeCmd(dev device.Device, app device.App) tea.Cmd {
	logger := m.logger
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		fs := device.NewIOSPhysicalAppFileSystem(app.BundleID, logger)
		root, err := fs.Tree(ctx, dev)
		return fileTreeMsg{device: dev, root: root, err: err}
	}
}

func (m model) loadAndroidPhysicalFileTreeCmd(dev device.Device) tea.Cmd {
	logger := m.logger
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		fs := device.NewAndroidPhysicalFileSystem(logger)
		root, err := fs.Tree(ctx, dev)
		return fileTreeMsg{device: dev, root: root, err: err}
	}
}

func (m model) startInstallCmd(dev device.Device, path, scheme string) tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		stream, err := coord.StartInstall(dev, path, scheme)
		if err != nil {
			return buildDoneMsg{deviceID: dev.ID, err: err}
		}
		return buildStartedMsg{device: dev, stream: stream}
	}
}

func nextBuildEventCmd(stream *device.BuildStream, deviceID string) tea.Cmd {
	return func() tea.Msg {
		text, ok := <-stream.Events
		if !ok {
			err := <-stream.Done
			return buildDoneMsg{deviceID: deviceID, err: err}
		}
		return buildEventMsg{deviceID: deviceID, text: text}
	}
}

// resolveIOSRebuildCmd finds the Xcode project for a locally-built app by
// matching its display name against DerivedData, then loads its schemes.
func (m model) resolveIOSRebuildCmd(dev device.Device, app device.App) tea.Cmd {
	logger := m.logger
	return func() tea.Msg {
		path, err := device.ResolveIOSProjectPath(app.Label())
		if err != nil {
			logger.LogError("resolve ios project for "+app.Label(), err)
			return rebuildIOSResolvedMsg{device: dev, app: app, err: err}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		schemes, err := device.ListXcodeSchemes(ctx, path)
		if err != nil {
			logger.LogError("list xcode schemes for "+path, err)
			return rebuildIOSResolvedMsg{device: dev, app: app, err: err}
		}
		return rebuildIOSResolvedMsg{device: dev, app: app, path: path, schemes: schemes}
	}
}

// startRebuildCmd stops any in-flight build stream and starts a new
// rebuild-and-reinstall pipeline for app, updating status/spinner state.
func (m *model) startRebuildCmd(dev device.Device, app device.App, path, scheme string) tea.Cmd {
	if m.buildStream != nil {
		m.buildStream.Stop()
		m.buildStream = nil
	}
	m.installing = true
	return tea.Batch(
		m.terminateThenInstallCmd(dev, app.BundleID, path, scheme),
		m.setStatus("Rebuilding "+app.Label()+"…", ui.StatusInfo),
		tea.Cmd(m.installSpinner.Tick),
	)
}

// terminateThenInstallCmd stops the currently running instance of an app
// (best-effort — it may not be running) and then starts the normal
// build→install→launch pipeline, matching xcodebuild/simctl rebuild flow.
func (m model) terminateThenInstallCmd(dev device.Device, bundleID, path, scheme string) tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = coord.TerminateApp(ctx, dev, bundleID)
		cancel()

		stream, err := coord.StartInstall(dev, path, scheme)
		if err != nil {
			return buildDoneMsg{deviceID: dev.ID, err: err}
		}
		return buildStartedMsg{device: dev, stream: stream}
	}
}

func (m model) fetchXcodeSchemesCmd(path string) tea.Cmd {
	logger := m.logger
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		schemes, err := device.ListXcodeSchemes(ctx, path)
		if err != nil {
			logger.LogError("list xcode schemes for "+path, err)
		}
		return xcodeSchemesMsg{schemes: schemes, err: err}
	}
}

func (m model) deleteAppCmd(dev device.Device, app device.App) tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := coord.DeleteApp(ctx, dev, app.BundleID)
		return deleteAppResultMsg{deviceID: dev.ID, bundleID: app.BundleID, appLabel: app.Label(), err: err}
	}
}

func (m model) launchAppCmd(dev device.Device, app device.App) tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := coord.LaunchApp(ctx, dev, app.BundleID)
		return launchAppResultMsg{deviceID: dev.ID, bundleID: app.BundleID, appLabel: app.Label(), err: err}
	}
}

func (m model) closeAppCmd(dev device.Device, app device.App) tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := coord.TerminateApp(ctx, dev, app.BundleID)
		return closeAppResultMsg{deviceID: dev.ID, bundleID: app.BundleID, appLabel: app.Label(), err: err}
	}
}

func (m model) loadAppsCmd(dev device.Device) tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		apps, err := coord.ListApps(ctx, dev)
		return appsListMsg{device: dev, apps: apps, err: err}
	}
}

func (m model) loadInfoCmd(dev device.Device) tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		info, err := coord.Info(ctx, dev)
		return infoMsg{device: dev, info: info, err: err}
	}
}

// scheduleBootPoll returns a Cmd that fires bootPollMsg after 4 seconds.
// Used to keep refreshing after an Android emulator boot until adb sees it.
func scheduleBootPoll(dev device.Device, remaining int) tea.Cmd {
	if remaining <= 0 {
		return nil
	}
	return tea.Tick(4*time.Second, func(_ time.Time) tea.Msg {
		return bootPollMsg{device: dev, remaining: remaining}
	})
}

// nextLogBatchCmd blocks until at least one log line is available, then drains
// up to 100 additional lines that are immediately available. This batches
// rapid log output into a single Update+View cycle instead of one per line.
func nextLogBatchCmd(stream *device.LogStream, bundleID string) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-stream.Lines
		if !ok {
			err := <-stream.Done
			return logEndedMsg{bundleID: bundleID, err: err}
		}
		lines := []string{line}
		for len(lines) < 100 {
			select {
			case l, ok := <-stream.Lines:
				if !ok {
					return logEndedMsg{bundleID: bundleID, err: <-stream.Done}
				}
				lines = append(lines, l)
			default:
				return logBatchMsg{bundleID: bundleID, lines: lines}
			}
		}
		return logBatchMsg{bundleID: bundleID, lines: lines}
	}
}
