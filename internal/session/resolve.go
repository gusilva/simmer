package session

import (
	"fmt"
	"strings"

	"simmer/internal/device"
)

// BootedRef selects the single running device. It is also what an empty
// reference means.
const BootedRef = "booted"

// Resolve turns a device reference into a device from the session roster.
//
// A reference is a UDID or adb serial, a device name, a unique fragment of a
// name, or the literal "booted". An empty reference means "booted". Matching
// is case-insensitive and stops at the first precedence level that yields
// candidates; more than one candidate at that level is an error naming them,
// never an arbitrary pick.
//
// A running emulator answers to two identifiers — its AVD name and its adb
// serial — and both are matched at the identifier level, because `adb
// devices` shows the serial and a script written against adb will carry it.
//
// Resolve matches against the roster recorded by the most recent
// ListDevices call, so callers must discover before resolving.
func (s *Session) Resolve(ref string) (device.Device, error) {
	return resolveDevice(s.devices(), ref)
}

// resolveDevice is Resolve's matching rule, pure for testing.
func resolveDevice(devs []device.Device, ref string) (device.Device, error) {
	if len(devs) == 0 {
		return device.Device{}, fmt.Errorf("no devices found: run discovery first, and check that Xcode or the Android SDK is installed")
	}

	ref = strings.TrimSpace(ref)
	if ref == "" || strings.EqualFold(ref, BootedRef) {
		return onlyBooted(devs)
	}

	// Precedence: exact identifier → exact name → unique name fragment.
	if hits := matching(devs, func(d device.Device) bool {
		return strings.EqualFold(d.ID, ref) ||
			(d.Serial != "" && strings.EqualFold(d.Serial, ref))
	}); len(hits) > 0 {
		return single(hits, ref, "id")
	}
	if hits := matching(devs, func(d device.Device) bool {
		return strings.EqualFold(d.Name, ref)
	}); len(hits) > 0 {
		return single(hits, ref, "name")
	}
	if hits := matching(devs, func(d device.Device) bool {
		return strings.Contains(strings.ToLower(d.Name), strings.ToLower(ref))
	}); len(hits) > 0 {
		return single(hits, ref, "name fragment")
	}

	return device.Device{}, fmt.Errorf("no device matches %q", ref)
}

// onlyBooted returns the single running device, or an error explaining why
// the choice cannot be made implicitly.
func onlyBooted(devs []device.Device) (device.Device, error) {
	booted := matching(devs, func(d device.Device) bool {
		return d.Status == device.StatusRunning
	})
	switch len(booted) {
	case 0:
		return device.Device{}, fmt.Errorf("no running device: boot one, or name an offline device explicitly")
	case 1:
		return booted[0], nil
	default:
		return device.Device{}, fmt.Errorf("%d devices are running, name one explicitly: %s", len(booted), describe(booted))
	}
}

func single(hits []device.Device, ref, by string) (device.Device, error) {
	if len(hits) == 1 {
		return hits[0], nil
	}
	return device.Device{}, fmt.Errorf("%q matches %d devices by %s: %s", ref, len(hits), by, describe(hits))
}

func matching(devs []device.Device, pred func(device.Device) bool) []device.Device {
	var out []device.Device
	for _, d := range devs {
		if pred(d) {
			out = append(out, d)
		}
	}
	return out
}

// describe renders candidates as "Name (id)" for disambiguation messages.
func describe(devs []device.Device) string {
	parts := make([]string, len(devs))
	for i, d := range devs {
		parts[i] = fmt.Sprintf("%s (%s)", d.Name, d.ID)
	}
	return strings.Join(parts, ", ")
}
