package ui

import "simmer/internal/device"

// ── Messages ────────────────────────────────────────────────────────────────

// ShowPlatformPickerMsg is emitted by Sidebar when 'a' is pressed in Available pane.
type ShowPlatformPickerMsg struct{}

// ConfirmPlatformPickerMsg is emitted when the user picks a platform.
type ConfirmPlatformPickerMsg struct{ Platform device.Platform }

// ShowDeleteSimulatorMsg is emitted by Sidebar when 'd' is pressed on a selected device.
type ShowDeleteSimulatorMsg struct{ Device device.Device }

// ConfirmCreateSimulatorMsg is emitted when the iOS create form is submitted.
type ConfirmCreateSimulatorMsg struct {
	Name         string
	DeviceTypeID string
	RuntimeID    string
}

// ConfirmCreateAndroidEmulatorMsg is emitted when the Android create form is submitted.
type ConfirmCreateAndroidEmulatorMsg struct {
	Name            string
	SystemImagePkg  string
	DeviceProfileID string // may be empty
}

// ConfirmDeleteSimulatorMsg is emitted when the user confirms deletion.
type ConfirmDeleteSimulatorMsg struct{ Device device.Device }

// ShowInstallAppMsg is emitted by MainPane when 'i' is pressed in the Apps tab.
type ShowInstallAppMsg struct {
	Device device.Device
}

// ConfirmInstallAppMsg is emitted when the user submits the install form.
type ConfirmInstallAppMsg struct {
	Device device.Device
	Path   string
	Scheme string // Xcode scheme; empty for Android or direct .app/.ipa installs
}

// RequestXcodeSchemesMsg is emitted by InstallAppModal when it needs schemes for a project path.
type RequestXcodeSchemesMsg struct {
	Path string
}

// ShowDeleteAppMsg is emitted by MainPane when 'd' is pressed on an app row.
type ShowDeleteAppMsg struct {
	Device device.Device
	App    device.App
}

// ConfirmDeleteAppMsg is emitted when the user confirms app deletion.
type ConfirmDeleteAppMsg struct {
	Device device.Device
	App    device.App
}

// LaunchAppMsg is emitted by MainPane when enter is pressed on an app row.
type LaunchAppMsg struct {
	Device device.Device
	App    device.App
}

// ShowCloseAppMsg is emitted by MainPane when 'c' is pressed on an app row.
type ShowCloseAppMsg struct {
	Device device.Device
	App    device.App
}

// ConfirmCloseAppMsg is emitted when the user confirms closing the app.
type ConfirmCloseAppMsg struct {
	Device device.Device
	App    device.App
}

// RequestRebuildMsg is emitted by MainPane when 'r' is pressed on an app row.
type RequestRebuildMsg struct {
	Device device.Device
	App    device.App
}

// ConfirmRebuildMsg is emitted when a rebuild's project path (and, for iOS,
// scheme) has been picked, either automatically or via the picker overlay.
type ConfirmRebuildMsg struct {
	Device device.Device
	App    device.App
	Path   string
	Scheme string // Xcode scheme; empty for Android
}

// CancelOverlayMsg is emitted when the user dismisses any overlay.
type CancelOverlayMsg struct{}

// StartBootMsg is emitted when the user requests booting a device, either via
// the sidebar's 'b' key or the double-click action menu.
type StartBootMsg struct{ Device device.Device }

// PinAppMsg is emitted when the user requests pinning/unpinning an app for
// Files-panel sandbox browsing from the double-click action menu (the
// "space" key calls MainPane.TogglePinnedApp directly, since it already owns
// the pane).
type PinAppMsg struct{ App device.App }

// ActivateTreeRowMsg is emitted by the double-click action menu to run the
// same effect as pressing "enter" on the selected Files-panel row.
type ActivateTreeRowMsg struct{}

// LoadDeviceMsg is emitted when the user requests loading a running device
// into the main pane, either via the sidebar's "space" key or the
// double-click action menu.
type LoadDeviceMsg struct{ Device device.Device }

// RefreshDevicesMsg is emitted when the user requests a device list refresh,
// either via the sidebar's "r" key or the double-click action menu.
type RefreshDevicesMsg struct{}
