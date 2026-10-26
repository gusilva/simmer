package session

import (
	"context"
	"errors"
	"testing"

	"simmer/internal/device"
)

func androidDevice(kind device.DeviceKind) device.Device {
	return device.Device{ID: "X", Name: "Pixel_8", Platform: device.PlatformAndroid, Status: device.StatusRunning, Kind: kind}
}

func androidSession(f *fakeManager) *Session {
	f.platform = device.PlatformAndroid
	return New(device.NewCoordinator(f), nil, testConfig())
}

func TestAppsSettledMergesEnrichment(t *testing.T) {
	s := androidSession(&fakeManager{
		kind: device.KindVirtual,
		apps: []device.App{
			{BundleID: "com.acme.one", Name: "One", Type: "User"},
			{BundleID: "com.acme.two", Name: "Two", Type: "User"},
		},
		versions: map[string]string{"com.acme.one": "1.4.2", "com.acme.two": "9.0"},
		rn:       map[string]bool{"com.acme.two": true},
	})

	cat, err := s.Apps(context.Background(), androidDevice(device.KindVirtual), EnrichOptions{})
	if err != nil {
		t.Fatalf("Apps: %v", err)
	}

	apps := cat.Settled(context.Background())
	if len(apps) != 2 {
		t.Fatalf("got %d apps, want 2", len(apps))
	}

	byID := map[string]device.App{}
	for _, a := range apps {
		byID[a.BundleID] = a
	}

	if got := byID["com.acme.one"].Version; got != "1.4.2" {
		t.Errorf("one version = %q, want 1.4.2", got)
	}
	if byID["com.acme.one"].IsReactNative {
		t.Error("one should not be marked React Native")
	}
	if got := byID["com.acme.two"].Version; got != "9.0" {
		t.Errorf("two version = %q, want 9.0", got)
	}
	if !byID["com.acme.two"].IsReactNative {
		t.Error("two should be marked React Native")
	}
}

func TestAppsNoEnrichmentForIOSSimulator(t *testing.T) {
	// The simulator's single listapps call already returns complete data, so
	// there is nothing to stream and Updates must be nil.
	f := &fakeManager{platform: device.PlatformIOS, kind: device.KindVirtual, apps: []device.App{{BundleID: "com.acme.one"}}}
	s := New(device.NewCoordinator(f), nil, testConfig())

	cat, err := s.Apps(context.Background(), device.Device{ID: "U", Platform: device.PlatformIOS, Kind: device.KindVirtual}, EnrichOptions{})
	if err != nil {
		t.Fatalf("Apps: %v", err)
	}
	if cat.Updates != nil {
		t.Error("expected no enrichment channel for an iOS simulator")
	}
	if got := cat.Settled(context.Background()); len(got) != 1 {
		t.Errorf("got %d apps, want 1", len(got))
	}
}

func TestAppsEmptyListSkipsEnrichment(t *testing.T) {
	s := androidSession(&fakeManager{kind: device.KindVirtual})

	cat, err := s.Apps(context.Background(), androidDevice(device.KindVirtual), EnrichOptions{})
	if err != nil {
		t.Fatalf("Apps: %v", err)
	}
	if cat.Updates != nil {
		t.Error("expected no enrichment channel for an empty app list")
	}
}

func TestAppsListFailurePropagates(t *testing.T) {
	s := androidSession(&fakeManager{kind: device.KindVirtual, listAppsErr: errors.New("adb offline")})

	if _, err := s.Apps(context.Background(), androidDevice(device.KindVirtual), EnrichOptions{}); err == nil {
		t.Fatal("expected the list failure to propagate")
	}
}

func TestSettledStopsAtCancelledContext(t *testing.T) {
	s := androidSession(&fakeManager{
		kind:     device.KindVirtual,
		apps:     []device.App{{BundleID: "com.acme.one"}},
		versions: map[string]string{"com.acme.one": "1.0"},
	})

	cat, err := s.Apps(context.Background(), androidDevice(device.KindVirtual), EnrichOptions{})
	if err != nil {
		t.Fatalf("Apps: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Returns the list it has rather than blocking; enrichment is abandoned.
	if got := cat.Settled(ctx); len(got) != 1 {
		t.Errorf("got %d apps, want 1", len(got))
	}
}

func TestApplyFoldsUpdates(t *testing.T) {
	app := device.App{BundleID: "com.acme.one"}

	Apply(&app, AppUpdate{BundleID: "com.acme.one", Version: "2.1"})
	Apply(&app, AppUpdate{BundleID: "com.acme.one", ReactNative: true})

	if app.Version != "2.1" || !app.IsReactNative {
		t.Errorf("got version %q rn %v, want 2.1 true", app.Version, app.IsReactNative)
	}

	// An empty update must not erase what is already known.
	Apply(&app, AppUpdate{BundleID: "com.acme.one"})
	if app.Version != "2.1" || !app.IsReactNative {
		t.Errorf("empty update clobbered data: %+v", app)
	}
}

func TestEnrichConcurrency(t *testing.T) {
	tests := []struct {
		name string
		dev  device.Device
		want int
	}{
		{"ios simulator", device.Device{Platform: device.PlatformIOS, Kind: device.KindVirtual}, 4},
		{"android emulator", device.Device{Platform: device.PlatformAndroid, Kind: device.KindVirtual}, 4},
		{"physical android", device.Device{Platform: device.PlatformAndroid, Kind: device.KindPhysical}, 4},
		{"physical ios stays serial", device.Device{Platform: device.PlatformIOS, Kind: device.KindPhysical}, 1},
	}
	for _, tc := range tests {
		if got := enrichConcurrency(tc.dev); got != tc.want {
			t.Errorf("%s: got %d workers, want %d", tc.name, got, tc.want)
		}
	}
}

func TestAppsSkipReactNative(t *testing.T) {
	// Android needs versions regardless, so the catalogue still streams —
	// but the React Native round-trip must not be made.
	f := &fakeManager{
		kind:     device.KindVirtual,
		apps:     []device.App{{BundleID: "com.acme.one"}},
		versions: map[string]string{"com.acme.one": "3.0"},
		rn:       map[string]bool{"com.acme.one": true},
	}
	s := androidSession(f)

	cat, err := s.Apps(context.Background(), androidDevice(device.KindVirtual), EnrichOptions{SkipReactNative: true})
	if err != nil {
		t.Fatalf("AppsWith: %v", err)
	}

	apps := cat.Settled(context.Background())
	if apps[0].Version != "3.0" {
		t.Errorf("version = %q, want 3.0", apps[0].Version)
	}
	if apps[0].IsReactNative {
		t.Error("React Native was detected despite SkipReactNative")
	}
}

func TestAppsSkipReactNativeOnIOSPhysicalNeedsNothing(t *testing.T) {
	// Physical iOS needs only the React Native check; skipping it leaves no
	// enrichment to do at all.
	f := &fakeManager{platform: device.PlatformIOS, kind: device.KindPhysical, apps: []device.App{{BundleID: "com.acme.one"}}}
	s := New(device.NewCoordinator(f), nil, testConfig())

	cat, err := s.Apps(context.Background(), device.Device{ID: "P", Platform: device.PlatformIOS, Kind: device.KindPhysical}, EnrichOptions{SkipReactNative: true})
	if err != nil {
		t.Fatalf("AppsWith: %v", err)
	}
	if cat.Updates != nil {
		t.Error("expected no enrichment channel when the only refinement is skipped")
	}
}
