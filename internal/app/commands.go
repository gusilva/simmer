package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"simmer/internal/device"
	"simmer/internal/session"
	"simmer/internal/ui"

	tea "charm.land/bubbletea/v2"
)

// defaultBundlerHostPort returns a Metro host:port that is actually
// reachable from dev, so the prompt isn't prefilled with a value that looks
// plausible but silently never connects:
//   - iOS simulator: "localhost" works — simctl processes share this Mac's
//     network namespace.
//   - Android emulator: "localhost" resolves to the emulator itself, not
//     this Mac; the AVD's host-loopback alias is "10.0.2.2" [TODO - ip improvement].
//   - Any physical device: "localhost" resolves to the phone itself. It has
//     to be pointed at this Mac's actual LAN address instead.
func defaultBundlerHostPort(dev device.Device) string {
	const port = "8081"
	if dev.Kind == device.KindPhysical {
		if ip, ok := device.LocalLANIP(); ok {
			return ip + ":" + port
		}
		return "localhost:" + port
	}
	if dev.Platform == device.PlatformAndroid {
		return "10.0.2.2:" + port
	}
	return "localhost:" + port
}

// validateHostPort checks basic well-formedness of a "host:port" string
// entered in the bundler-location prompt: non-empty, contains ":", and the
// port segment is numeric. It does not validate the host portion — Metro
// itself surfaces a connection failure for a bad host.
func validateHostPort(s string) error {
	if s == "" {
		return fmt.Errorf("required")
	}
	i := strings.LastIndex(s, ":")
	if i < 0 || i == len(s)-1 {
		return fmt.Errorf("expected host:port")
	}
	if _, err := strconv.Atoi(s[i+1:]); err != nil {
		return fmt.Errorf("port must be numeric")
	}
	return nil
}

func (m model) fetchDevicesCmd() tea.Cmd {
	sess := m.sess
	return func() tea.Msg {
		return discoveryMsg(sess.ListDevices(context.Background()))
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

// searchFilesCmd searches the device under the tree's roots. The tree only
// knows the directories that have been expanded, so this is what finds a
// file nobody has navigated to.
func (m model) searchFilesCmd(dev device.Device, roots []string, query string) tea.Cmd {
	sess := m.sess
	return func() tea.Msg {
		matches, err := sess.SearchFiles(context.Background(), dev, roots, query)
		return fileSearchMsg{query: query, matches: matches, err: err}
	}
}

// loadTreeAtCmd re-roots the Files tree at a directory, which is how a
// search result turns back into something browsable.
func (m model) loadTreeAtCmd(dev device.Device, path string) tea.Cmd {
	sess := m.sess
	return func() tea.Msg {
		root, err := sess.FileTree(context.Background(), dev, device.ScopeDirectory, path)
		return fileTreeMsg{device: dev, root: root, err: err}
	}
}

// loadSubtreeCmd lists one directory's entries, for a tree node the user
// expanded. Listing one level keeps an expansion cheap no matter how large
// the subtree below it is.
func (m model) loadSubtreeCmd(dev device.Device, path string) tea.Cmd {
	sess := m.sess
	return func() tea.Msg {
		node, err := sess.FileTree(context.Background(), dev, device.ScopeDirectory, path)
		return subtreeMsg{device: dev, path: path, node: node, err: err}
	}
}

// loadFileTreeCmd walks a device's filesystem at the given scope. The
// Session picks the adapter for the device's platform and kind, so this
// command no longer needs to know that physical iOS goes over AFC or that a
// package's private data dir needs run-as.
func (m model) loadFileTreeCmd(dev device.Device, scope device.FileScope, bundleID string) tea.Cmd {
	sess := m.sess
	return func() tea.Msg {
		root, err := sess.FileTree(context.Background(), dev, scope, bundleID)
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
	sess := m.sess
	return func() tea.Msg {
		cat, err := sess.Apps(context.Background(), dev, session.EnrichOptions{})
		if err != nil {
			return appsListMsg{device: dev, err: err}
		}
		return appsListMsg{device: dev, apps: cat.Apps, updates: cat.Updates, stop: cat.Stop}
	}
}

// waitForAppUpdateCmd blocks for the next refinement the Session's
// enrichment pool produces (a version name, or React Native status) and
// re-arms itself, so the Apps panel fills in progressively. Returns no
// message once the channel is drained and closed.
func waitForAppUpdateCmd(ch <-chan session.AppUpdate) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		update, ok := <-ch
		if !ok {
			return nil
		}
		return appUpdateMsg{update: update, ch: ch}
	}
}

func (m model) loadInfoCmd(dev device.Device) tea.Cmd {
	sess := m.sess
	return func() tea.Msg {
		info, err := sess.Info(context.Background(), dev)
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

// setBundlerLocationCmd points app's JS bundler (Metro) at hostPort and
// relaunches it, per the RN debugging menu's "Configure Bundler Location".
func (m model) setBundlerLocationCmd(dev device.Device, app device.App, hostPort string) tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := coord.SetBundlerLocation(ctx, dev, app.BundleID, hostPort)
		return bundlerConfigResultMsg{appLabel: app.Label(), err: err}
	}
}

// triggerDevMenuCmd opens the RN dev menu on dev, per the RN debugging
// menu's "Dev Menu (Trigger)". metroPort only matters to the physical-iOS
// trigger, which routes through Metro's WebSocket and needs to know which
// port it's on; pass "" to use that trigger's default.
func (m model) triggerDevMenuCmd(dev device.Device, metroPort string) tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		err := coord.TriggerDevMenu(ctx, dev, metroPort)
		return devMenuTriggerResultMsg{err: err}
	}
}

// portFromHostPort returns the port segment of a validated "host:port"
// string, or "" if none is found.
func portFromHostPort(hostPort string) string {
	i := strings.LastIndex(hostPort, ":")
	if i < 0 || i == len(hostPort)-1 {
		return ""
	}
	return hostPort[i+1:]
}

// rnStartLoggingCmd terminates and relaunches app, per the RN debugging
// menu's "Log to File", which always starts from a clean launch. The actual
// file-open + StreamLogs setup happens in Update's rnLogRelaunchedMsg
// handler, sharing the existing "l"-key app-logging state/pipeline.
func (m model) rnStartLoggingCmd(dev device.Device, app device.App, path string) tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = coord.TerminateApp(ctx, dev, app.BundleID)
		cancel()

		ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		err := coord.LaunchApp(ctx, dev, app.BundleID)
		return rnLogRelaunchedMsg{device: dev, app: app, path: path, err: err}
	}
}

// startConsoleLogCmd launches app via `devicectl device process launch
// --console --terminate-existing`, atomically relaunching it and starting a
// console-output stream in one step — see
// physicalIOSManager.StreamConsoleLog. Used by the RN debugging menu's "log
// to file" only for physical iOS; every other device kind goes through
// rnStartLoggingCmd (explicit terminate+launch) then StreamLogs instead.
//
// hostPort re-applies a previously configured bundler location to this
// relaunch — see the ConsoleLogStreamer doc comment for why that's required
// on physical iOS, not just a nice-to-have.
func (m model) startConsoleLogCmd(dev device.Device, app device.App, path, hostPort string) tea.Cmd {
	coord := m.coordinator
	return func() tea.Msg {
		stream, err := coord.StreamConsoleLog(context.Background(), dev, app.BundleID, hostPort)
		return consoleLogStartedMsg{device: dev, app: app, path: path, stream: stream, err: err}
	}
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
