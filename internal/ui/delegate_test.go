package ui

import (
	"bytes"
	"testing"

	"simmer/internal/device"

	"charm.land/bubbles/v2/list"
)

func TestItem_Methods(t *testing.T) {
	dev := device.Device{Name: "iPhone 15", Platform: device.PlatformIOS, Version: "17.0"}
	item := Item{Dev: dev}

	if item.FilterValue() != "iPhone 15" {
		t.Errorf("FilterValue: got %q", item.FilterValue())
	}
	if item.Title() != "iPhone 15" {
		t.Errorf("Title: got %q", item.Title())
	}
	if item.Description() != "iOS 17.0" {
		t.Errorf("Description: got %q", item.Description())
	}
}

func TestDeviceDelegate_HeightSpacingUpdate(t *testing.T) {
	d := DeviceDelegate{}
	if d.Height() != 1 {
		t.Errorf("Height: want 1, got %d", d.Height())
	}
	if d.Spacing() != 0 {
		t.Errorf("Spacing: want 0, got %d", d.Spacing())
	}
	cmd := d.Update(nil, nil)
	if cmd != nil {
		t.Error("Update must return nil cmd")
	}
}

func TestDeviceDelegate_Render_NotItem(t *testing.T) {
	d := DeviceDelegate{}
	m := list.New(nil, list.NewDefaultDelegate(), 80, 10)
	var buf bytes.Buffer
	// Passing a non-Item — must not panic.
	d.Render(&buf, m, 0, struct{ list.Item }{})
	if buf.Len() != 0 {
		t.Error("expected empty output for non-Item")
	}
}

func TestDeviceDelegate_Render_Unselected(t *testing.T) {
	d := DeviceDelegate{}
	items := []list.Item{
		Item{Dev: device.Device{Name: "iPhone 15", Platform: device.PlatformIOS, Version: "17.0", Status: device.StatusRunning}},
		Item{Dev: device.Device{Name: "Pixel 8", Platform: device.PlatformAndroid, Version: "14", Status: device.StatusOff}},
	}
	m := list.New(items, list.NewDefaultDelegate(), 80, 10)
	var buf bytes.Buffer
	// index 1 = Pixel 8, not selected (cursor is at 0).
	d.Render(&buf, m, 1, items[1])
	if buf.Len() == 0 {
		t.Error("expected non-empty output for unselected item")
	}
}

func TestDeviceDelegate_Render_Selected(t *testing.T) {
	d := DeviceDelegate{}
	items := []list.Item{
		Item{Dev: device.Device{Name: "iPhone 15", Platform: device.PlatformIOS, Version: "17.0", Status: device.StatusRunning}},
	}
	m := list.New(items, list.NewDefaultDelegate(), 80, 10)
	var buf bytes.Buffer
	// index 0 = selected (default cursor position).
	d.Render(&buf, m, 0, items[0])
	if buf.Len() == 0 {
		t.Error("expected non-empty output for selected item")
	}
}

func TestNewListStyles_Smoke(t *testing.T) {
	s := NewListStyles()
	// Just verify it doesn't panic and returns a non-zero struct.
	_ = s.NoItems
	_ = s.PaginationStyle
}
