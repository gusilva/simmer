package app

import (
	"simmer/internal/device"
	"simmer/internal/session"
)

type buildStartedMsg struct {
	device device.Device
	stream *device.BuildStream
}

type buildEventMsg struct {
	deviceID string
	text     string
}

type buildDoneMsg struct {
	deviceID string
	err      error
}

type clearStatusMsg int

type autoRefreshMsg struct{}

type discoveryMsg device.DiscoveryResult

type deviceTypesMsg struct {
	types []device.DeviceType
	err   error
}

type runtimesMsg struct {
	runtimes []device.Runtime
	err      error
}

type createSimulatorResultMsg struct {
	name string
	udid string
	err  error
}

type deleteSimulatorResultMsg struct {
	name string
	err  error
}

type deleteAppResultMsg struct {
	deviceID string
	bundleID string
	appLabel string
	err      error
}

type launchAppResultMsg struct {
	deviceID string
	bundleID string
	appLabel string
	err      error
}

type closeAppResultMsg struct {
	deviceID string
	bundleID string
	appLabel string
	err      error
}

type xcodeSchemesMsg struct {
	schemes []string
	err     error
}

type rebuildIOSResolvedMsg struct {
	device  device.Device
	app     device.App
	path    string
	schemes []string
	err     error
}

type androidSystemImagesMsg struct {
	images []device.Runtime
	err    error
}

type androidDeviceProfilesMsg struct {
	profiles []device.DeviceType
	err      error
}

type createAndroidEmulatorResultMsg struct {
	name string
	err  error
}

type bootResultMsg struct {
	device device.Device
	err    error
}

type shutdownResultMsg struct {
	device device.Device
	err    error
}

type fileTreeMsg struct {
	device device.Device
	root   device.FileNode
	err    error
}

// appsListMsg carries the bare app list for a device, plus the channel the
// Session streams later refinements on (nil when the platform needs none).
// fileSearchMsg carries the result of a device file search.
type fileSearchMsg struct {
	query   string
	matches []device.FileMatch
	err     error
}

// subtreeMsg carries the entries of one expanded directory.
type subtreeMsg struct {
	device device.Device
	path   string
	node   device.FileNode
	err    error
}

type appsListMsg struct {
	device  device.Device
	apps    []device.App
	updates <-chan session.AppUpdate
	// stop abandons the enrichment pool behind updates. Without it a
	// device switch leaves the previous pool running against the same USB
	// connection, which is the contention the worker limit exists to avoid.
	stop func()
	err  error
}

// appUpdateMsg wraps one refinement from the Session's enrichment pool (a
// version name, or React Native status) along with the channel it came from,
// so Update can re-arm the listener and keep draining until the pool
// finishes and closes ch.
type appUpdateMsg struct {
	update session.AppUpdate
	ch     <-chan session.AppUpdate
}

type infoMsg struct {
	device device.Device
	info   device.DeviceInfo
	err    error
}

type bootPollMsg struct {
	device    device.Device
	remaining int
}

type logBatchMsg struct {
	bundleID string
	lines    []string
}

type logEndedMsg struct {
	bundleID string
	err      error
}

// bundlerConfigResultMsg reports the outcome of setting an RN app's Metro
// bundler location (see model.setBundlerLocationCmd).
type bundlerConfigResultMsg struct {
	appLabel string
	err      error
}

// devMenuTriggerResultMsg reports the outcome of triggering the RN dev menu
// (see model.triggerDevMenuCmd).
type devMenuTriggerResultMsg struct {
	err error
}

// rnLogRelaunchedMsg reports the outcome of the terminate+relaunch step that
// precedes starting an RN "log to file" session (see
// model.rnStartLoggingCmd), carrying the resolved log path through to the
// handler that opens the file and starts streaming.
type rnLogRelaunchedMsg struct {
	device device.Device
	app    device.App
	path   string
	err    error
}

// consoleLogStartedMsg reports the outcome of starting a devicectl --console
// log stream (physical iOS only) for the RN debugging menu's "log to file"
// (see model.startConsoleLogCmd).
type consoleLogStartedMsg struct {
	device device.Device
	app    device.App
	path   string
	stream *device.LogStream
	err    error
}
