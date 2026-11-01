package device

import (
	"context"
	"testing"
)

// capabilityFake is a Manager whose enrichment capabilities can be chosen,
// so the registry questions Session asks can be exercised directly.
type capabilityFake struct {
	platform Platform
	kind     DeviceKind
}

func (f *capabilityFake) ListDevices(context.Context) ([]Device, error) { return nil, nil }
func (f *capabilityFake) Platform() Platform                            { return f.platform }
func (f *capabilityFake) Kind() DeviceKind                              { return f.kind }

type versionFake struct{ capabilityFake }

func (f *versionFake) FetchAppVersions(context.Context, string, []string) map[string]string {
	return nil
}

type rnFake struct{ capabilityFake }

func (f *rnFake) DetectReactNative(context.Context, string, App) bool { return false }

func TestCapabilitySupportFollowsTheRegistry(t *testing.T) {
	// Session used to hand-copy these rules as "Android, or physical iOS",
	// which goes stale the moment a manager is added. Asking the registry
	// cannot.
	android := Device{Platform: PlatformAndroid, Kind: KindVirtual}
	iosPhysical := Device{Platform: PlatformIOS, Kind: KindPhysical}

	c := NewCoordinator(
		&versionFake{capabilityFake{PlatformAndroid, KindVirtual}},
		&rnFake{capabilityFake{PlatformIOS, KindPhysical}},
	)

	if !c.SupportsAppVersions(android) {
		t.Error("android: version fetcher is registered")
	}
	if c.SupportsReactNativeDetection(android) {
		t.Error("android: no detector is registered in this coordinator")
	}
	if c.SupportsAppVersions(iosPhysical) {
		t.Error("physical ios: no version fetcher is registered")
	}
	if !c.SupportsReactNativeDetection(iosPhysical) {
		t.Error("physical ios: detector is registered")
	}

	// A kind nobody serves gets nothing, rather than an emulator's manager
	// answering for a phone.
	if c.SupportsAppVersions(Device{Platform: PlatformAndroid, Kind: KindPhysical}) {
		t.Error("physical android: must not fall back to the emulator manager")
	}
}

type limitedFake struct {
	capabilityFake
	limit int
}

func (f *limitedFake) AppEnrichmentConcurrency() int { return f.limit }

func TestEnrichmentConcurrencyComesFromTheTransportOwner(t *testing.T) {
	physicalIOS := Device{Platform: PlatformIOS, Kind: KindPhysical}

	// A manager that declares a limit sets it: detection there opens one
	// AFC session per app over a single USB transport.
	c := NewCoordinator(&limitedFake{capabilityFake{PlatformIOS, KindPhysical}, 1})
	if got := c.EnrichmentConcurrency(physicalIOS); got != 1 {
		t.Errorf("got %d workers, want the manager's limit of 1", got)
	}

	// A manager that declares none, and a platform with no manager at all,
	// both get the default.
	c = NewCoordinator(&capabilityFake{PlatformIOS, KindPhysical})
	if got := c.EnrichmentConcurrency(physicalIOS); got != DefaultEnrichmentConcurrency {
		t.Errorf("got %d workers, want the default %d", got, DefaultEnrichmentConcurrency)
	}
	if got := c.EnrichmentConcurrency(Device{Platform: PlatformAndroid}); got != DefaultEnrichmentConcurrency {
		t.Errorf("unrouted platform got %d workers, want the default", got)
	}

	// A non-positive declaration is a bug in the manager, not an
	// instruction to stop enriching.
	c = NewCoordinator(&limitedFake{capabilityFake{PlatformIOS, KindPhysical}, 0})
	if got := c.EnrichmentConcurrency(physicalIOS); got != DefaultEnrichmentConcurrency {
		t.Errorf("got %d workers for a zero limit, want the default", got)
	}
}

func TestRealManagersDeclareTheExpectedCapabilities(t *testing.T) {
	// Pins the registry the session asks about, so adding or removing a
	// capability shows up here rather than as a behaviour change.
	c := NewCoordinator(
		NewIOSManager(nil), NewPhysicalIOSManager(nil),
		NewAndroidManager(nil), NewPhysicalAndroidManager(nil),
	)

	tests := []struct {
		name        string
		dev         Device
		wantVersion bool
		wantRN      bool
		wantWorkers int
	}{
		{"ios simulator", Device{Platform: PlatformIOS, Kind: KindVirtual}, false, false, DefaultEnrichmentConcurrency},
		{"physical ios", Device{Platform: PlatformIOS, Kind: KindPhysical}, false, true, 1},
		{"android emulator", Device{Platform: PlatformAndroid, Kind: KindVirtual}, true, true, DefaultEnrichmentConcurrency},
		{"physical android", Device{Platform: PlatformAndroid, Kind: KindPhysical}, true, true, DefaultEnrichmentConcurrency},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := c.SupportsAppVersions(tc.dev); got != tc.wantVersion {
				t.Errorf("SupportsAppVersions = %v, want %v", got, tc.wantVersion)
			}
			if got := c.SupportsReactNativeDetection(tc.dev); got != tc.wantRN {
				t.Errorf("SupportsReactNativeDetection = %v, want %v", got, tc.wantRN)
			}
			if got := c.EnrichmentConcurrency(tc.dev); got != tc.wantWorkers {
				t.Errorf("EnrichmentConcurrency = %d, want %d", got, tc.wantWorkers)
			}
		})
	}
}
