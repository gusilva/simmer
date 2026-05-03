package ui

import (
	"strings"
	"testing"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
)

func TestPlatformPickerModal(t *testing.T) {
	m := NewPlatformPickerModal()

	// Initial selection (iOS)
	if m.idx != 0 {
		t.Errorf("expected idx 0, got %d", m.idx)
	}

	// Move down to Android
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.idx != 1 {
		t.Errorf("expected idx 1, got %d", m.idx)
	}

	// Confirm
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	confirm, ok := msg.(ConfirmPlatformPickerMsg)
	if !ok || confirm.Platform != device.PlatformAndroid {
		t.Errorf("expected ConfirmPlatformPickerMsg with Android, got %v", msg)
	}
}

func TestDeleteSimulatorAlert(t *testing.T) {
	dev := device.Device{Name: "iPhone 15", Platform: device.PlatformIOS}
	m := NewDeleteSimulatorAlert(dev)

	// Initial focus is Cancel
	if m.confirmFocus {
		t.Error("expected confirmFocus to be false (Cancel)")
	}

	// Toggle focus
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if !m.confirmFocus {
		t.Error("expected confirmFocus to be true (Delete)")
	}

	// Confirm delete
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	confirm, ok := msg.(ConfirmDeleteSimulatorMsg)
	if !ok || confirm.Device.Name != "iPhone 15" {
		t.Errorf("expected ConfirmDeleteSimulatorMsg, got %v", msg)
	}
}

func TestCreateSimulatorModal(t *testing.T) {
	m, _ := NewCreateSimulatorModal()

	// 1. Loading state
	if !m.loading {
		t.Error("expected loading to be true initially")
	}
	if !strings.Contains(m.View(), "Loading") {
		t.Error("View should show loading message")
	}

	// 2. Data population
	m.SetDeviceTypes([]device.DeviceType{{Name: "iPhone 16", Identifier: "i16"}})
	m.SetRuntimes([]device.Runtime{{Name: "iOS 18", Identifier: "ios18", IsAvailable: true}})

	if m.loading {
		t.Error("expected loading to be false after data population")
	}

	// 3. Input handling
	m, _ = m.Update(tea.KeyPressMsg{Text: "M"})
	m, _ = m.Update(tea.KeyPressMsg{Text: "y"})
	if m.nameInput.Value() != "My" {
		t.Errorf("expected name 'My', got %q", m.nameInput.Value())
	}

	// 4. Submission
	// Focus is on Name. Enter moves to Device Type.
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.focused != fieldDeviceType {
		t.Errorf("expected focus on DeviceType, got %v", m.focused)
	}

	// Enter again moves to Runtime
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.focused != fieldRuntime {
		t.Errorf("expected focus on Runtime, got %v", m.focused)
	}

	// Enter again submits if valid
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected submission command, got nil")
	}
	msg := cmd()
	confirm, ok := msg.(ConfirmCreateSimulatorMsg)
	if !ok || confirm.Name != "My" || confirm.DeviceTypeID != "i16" || confirm.RuntimeID != "ios18" {
		t.Errorf("invalid ConfirmCreateSimulatorMsg: %v", msg)
	}
}
