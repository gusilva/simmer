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

// CancelOverlayMsg is emitted when the user dismisses any overlay.
type CancelOverlayMsg struct{}
