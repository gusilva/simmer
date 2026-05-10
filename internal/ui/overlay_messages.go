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

// CancelOverlayMsg is emitted when the user dismisses any overlay.
type CancelOverlayMsg struct{}
