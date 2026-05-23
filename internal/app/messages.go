package app

import "simmer/internal/device"

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


type xcodeSchemesMsg struct {
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

type appsListMsg struct {
	device device.Device
	apps   []device.App
	err    error
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
