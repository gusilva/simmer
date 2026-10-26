package session

import (
	"strings"
	"testing"

	"simmer/internal/device"
)

func dev(id, name string, p device.Platform, st device.Status) device.Device {
	return device.Device{ID: id, Name: name, Platform: p, Status: st}
}

// emulatorDev is a running emulator: identified by AVD name, reachable by
// adb serial too.
func emulatorDev(avd, serial string) device.Device {
	return device.Device{
		ID:       avd,
		Name:     avd,
		Platform: device.PlatformAndroid,
		Status:   device.StatusRunning,
		Serial:   serial,
	}
}

var roster = []device.Device{
	dev("A1B2-UDID", "iPhone 16 Pro", device.PlatformIOS, device.StatusRunning),
	dev("C3D4-UDID", "iPhone 16 Pro Max", device.PlatformIOS, device.StatusOff),
	dev("emulator-5554", "Pixel_8_API_35", device.PlatformAndroid, device.StatusOff),
}

func TestResolveDevice(t *testing.T) {
	tests := []struct {
		name   string
		devs   []device.Device
		ref    string
		wantID string
		errHas string
	}{
		{name: "exact udid", devs: roster, ref: "A1B2-UDID", wantID: "A1B2-UDID"},
		{name: "udid is case-insensitive", devs: roster, ref: "a1b2-udid", wantID: "A1B2-UDID"},
		{name: "adb serial of a physical device", devs: roster, ref: "emulator-5554", wantID: "emulator-5554"},
		{name: "exact name beats fragment", devs: roster, ref: "iPhone 16 Pro", wantID: "A1B2-UDID"},
		{name: "unique fragment", devs: roster, ref: "Max", wantID: "C3D4-UDID"},
		{name: "fragment is case-insensitive", devs: roster, ref: "pixel", wantID: "emulator-5554"},
		{name: "empty ref means the only booted device", devs: roster, ref: "", wantID: "A1B2-UDID"},
		{name: "booted keyword", devs: roster, ref: "booted", wantID: "A1B2-UDID"},

		{name: "ambiguous fragment names candidates", devs: roster, ref: "iPhone", errHas: "matches 2 devices"},
		{name: "no match", devs: roster, ref: "Galaxy", errHas: "no device matches"},
		{name: "empty roster", devs: nil, ref: "", errHas: "no devices found"},
		{
			name:   "no booted device",
			devs:   []device.Device{dev("X", "iPhone SE", device.PlatformIOS, device.StatusOff)},
			ref:    "",
			errHas: "no running device",
		},
		{
			name: "several booted devices must be disambiguated",
			devs: []device.Device{
				dev("X", "iPhone SE", device.PlatformIOS, device.StatusRunning),
				dev("Y", "Pixel_8", device.PlatformAndroid, device.StatusRunning),
			},
			ref:    "booted",
			errHas: "2 devices are running",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveDevice(tc.devs, tc.ref)

			if tc.errHas != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got device %q", tc.errHas, got.ID)
				}
				if !strings.Contains(err.Error(), tc.errHas) {
					t.Fatalf("error %q does not contain %q", err, tc.errHas)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ID != tc.wantID {
				t.Errorf("resolved %q, want %q", got.ID, tc.wantID)
			}
		})
	}
}

func TestResolveEmulatorByEitherIdentifier(t *testing.T) {
	// `adb devices` reports "emulator-5554" while Simmer lists the AVD name,
	// so a script written against adb carries the serial and must resolve.
	devs := []device.Device{emulatorDev("pixel_8", "emulator-5554")}

	for _, ref := range []string{"pixel_8", "emulator-5554", "EMULATOR-5554", "pixel"} {
		got, err := resolveDevice(devs, ref)
		if err != nil {
			t.Errorf("resolve %q: %v", ref, err)
			continue
		}
		if got.ID != "pixel_8" {
			t.Errorf("resolve %q = %q, want pixel_8", ref, got.ID)
		}
	}
}

func TestResolveSerialBeatsNameFragment(t *testing.T) {
	// An identifier is exact; a fragment is a guess. A serial must not be
	// outranked by some other device whose name happens to contain it.
	devs := []device.Device{
		emulatorDev("pixel_8", "emulator-5554"),
		{ID: "udid-1", Name: "emulator-5554 lookalike", Platform: device.PlatformIOS, Status: device.StatusOff},
	}

	got, err := resolveDevice(devs, "emulator-5554")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "pixel_8" {
		t.Errorf("resolved %q, want the emulator matched by serial", got.ID)
	}
}

func TestResolveAmbiguityListsCandidates(t *testing.T) {
	_, err := resolveDevice(roster, "iPhone")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	for _, want := range []string{"iPhone 16 Pro (A1B2-UDID)", "iPhone 16 Pro Max (C3D4-UDID)"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q omits candidate %q", err, want)
		}
	}
}

func TestSessionResolveUsesRosterFromDiscovery(t *testing.T) {
	s := New(device.NewCoordinator(), nil, testConfig())

	if _, err := s.Resolve("iPhone"); err == nil {
		t.Fatal("expected resolution to fail before discovery")
	}

	s.roster = roster
	got, err := s.Resolve("Max")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "C3D4-UDID" {
		t.Errorf("resolved %q, want C3D4-UDID", got.ID)
	}
}
