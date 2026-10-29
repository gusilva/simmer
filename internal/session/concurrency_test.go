package session

import (
	"context"
	"sync"
	"testing"

	"simmer/internal/device"
)

// The session is shared by goroutines by construction: Bubble Tea runs every
// command on its own goroutine while the Update loop handles keys. These
// tests exist so `go test -race` has something to catch — the tree passed
// -race for months while Session.cfg was written and read without a lock,
// because nothing exercised both at once.

func TestConfigReloadRacesWithQueries(t *testing.T) {
	// The settings form saves config on the Update goroutine while a
	// tables-query command reads it on another.
	s := New(device.NewCoordinator(), nil, testConfig())

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.ReloadConfig()
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.Config().ForDB("rooms.db").EffectiveTablesQuery()
		}()
	}
	wg.Wait()
}

func TestDiscoveryRacesWithResolution(t *testing.T) {
	// Auto-refresh replaces the roster every 30 seconds while a key press
	// resolves a device against it.
	f := &fakeManager{
		platform: device.PlatformAndroid,
		kind:     device.KindVirtual,
		devices:  []device.Device{dev("X", "Pixel_8", device.PlatformAndroid, device.StatusRunning)},
	}
	s := New(device.NewCoordinator(f), nil, testConfig())

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.ListDevices(context.Background())
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.Resolve("Pixel")
		}()
	}
	wg.Wait()
}

func TestEnrichmentStopsCleanlyUnderConcurrentUse(t *testing.T) {
	// A device switch abandons the pool mid-flight: Stop runs while workers
	// are still sending.
	s := androidSession(&fakeManager{
		kind:     device.KindVirtual,
		apps:     []device.App{{BundleID: "a"}, {BundleID: "b"}, {BundleID: "c"}},
		versions: map[string]string{"a": "1", "b": "2", "c": "3"},
		rn:       map[string]bool{"b": true},
	})

	for range 4 {
		cat, err := s.Apps(context.Background(), androidDevice(device.KindVirtual), EnrichOptions{})
		if err != nil {
			t.Fatalf("Apps: %v", err)
		}

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range cat.Updates { //nolint:revive // draining is the point
			}
		}()

		cat.Stop()
		wg.Wait()
	}
}
