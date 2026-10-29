package session

import (
	"context"
	"sync"

	"simmer/internal/device"
)

// AppUpdate is one late-arriving fact about an installed app. Version is set
// when the update carries a version name; ReactNative is set when the app
// was detected as a React Native build.
type AppUpdate struct {
	DeviceID    string
	BundleID    string
	Version     string
	ReactNative bool
}

// AppCatalog is the apps installed on one device: the list as it can be read
// in a single round-trip, plus a stream of refinements that need further
// round-trips.
//
// Consumers receive refinements from Updates until that channel is closed.
// Updates is nil when the platform needs none (the iOS simulator's one
// listapps call already returns complete data). Stop abandons any in-flight
// enrichment and is safe to call more than once.
type AppCatalog struct {
	Device  device.Device
	Apps    []device.App
	Updates <-chan AppUpdate
	Stop    func()
}

// EnrichOptions selects which refinements Apps should fetch. The zero value
// fetches everything the platform needs.
type EnrichOptions struct {
	// SkipReactNative leaves React Native marks undetected. Detection reads
	// each APK's entry list off the device, so a caller that does not show
	// the mark can skip a round-trip per app.
	SkipReactNative bool
}

// Apps lists the apps installed on dev and starts enrichment for the facts
// this platform cannot report in one call.
//
// Android reports version names only via a second dumpsys call, and both
// Android and physical iOS need a device round-trip (APK pull / AFC lookup)
// to recognise a React Native build. Those run on a bounded worker pool
// behind Updates: physical hardware gets a single worker because concurrent
// pulls over one USB transport starve each other and fail silently rather
// than erroring, while an emulator on local loopback tolerates four.
func (s *Session) Apps(ctx context.Context, dev device.Device, opts EnrichOptions) (*AppCatalog, error) {
	listCtx, cancel := context.WithTimeout(ctx, appsTimeout)
	apps, err := s.coord.ListApps(listCtx, dev)
	cancel()
	if err != nil {
		return nil, err
	}

	cat := &AppCatalog{Device: dev, Apps: apps, Stop: func() {}}

	needVersions := dev.Platform == device.PlatformAndroid
	needRN := !opts.SkipReactNative &&
		(dev.Platform == device.PlatformAndroid ||
			(dev.Platform == device.PlatformIOS && dev.Kind == device.KindPhysical))
	if len(apps) == 0 || (!needVersions && !needRN) {
		return cat, nil
	}

	enrichCtx, stop := context.WithCancel(ctx)
	updates := make(chan AppUpdate, len(apps))
	cat.Updates = updates
	cat.Stop = stop

	go s.enrich(enrichCtx, dev, apps, needVersions, needRN, updates)

	return cat, nil
}

// enrichConcurrency returns how many apps may be enriched at once.
//
// Physical Android used to be limited to one: React Native detection pulled
// each whole APK, and concurrent multi-hundred-megabyte transfers over a
// single USB connection starved each other and failed silently rather than
// erroring. Detection now reads only an APK's central directory (a few
// hundred kilobytes), and four workers measured clean and repeatable against
// a 56-app phone, so the limit no longer applies there.
//
// Physical iOS keeps it. Its detection opens a house_arrest AFC session per
// app over the same USB transport, and that has not been measured under
// concurrency — the conservative value stays until it has been.
func enrichConcurrency(dev device.Device) int {
	if dev.Kind == device.KindPhysical && dev.Platform == device.PlatformIOS {
		return 1
	}
	return 4
}

// enrich runs the per-app round-trips and closes updates when done.
func (s *Session) enrich(ctx context.Context, dev device.Device, apps []device.App, needVersions, needRN bool, updates chan<- AppUpdate) {
	defer close(updates)

	workers := enrichConcurrency(dev)

	jobs := make(chan device.App)
	var wg sync.WaitGroup

	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for app := range jobs {
				// The version name and the React Native check are
				// independent round-trips, so one app's two questions are
				// asked at once rather than in series.
				var inner sync.WaitGroup

				if needVersions {
					inner.Add(1)
					go func() {
						defer inner.Done()
						vctx, cancel := context.WithTimeout(ctx, appVersionTimeout)
						versions := s.coord.FetchAppVersions(vctx, dev, []string{app.BundleID})
						cancel()
						if v := versions[app.BundleID]; v != "" {
							send(ctx, updates, AppUpdate{DeviceID: dev.ID, BundleID: app.BundleID, Version: v})
						}
					}()
				}

				if needRN {
					inner.Add(1)
					go func() {
						defer inner.Done()
						rctx, cancel := context.WithTimeout(ctx, reactNativeTimeout)
						isRN := s.coord.DetectReactNative(rctx, dev, app)
						cancel()
						if isRN {
							send(ctx, updates, AppUpdate{DeviceID: dev.ID, BundleID: app.BundleID, ReactNative: true})
						}
					}()
				}

				inner.Wait()
			}
		}()
	}

	for _, a := range apps {
		select {
		case jobs <- a:
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return
		}
	}
	close(jobs)
	wg.Wait()
}

func send(ctx context.Context, ch chan<- AppUpdate, u AppUpdate) {
	select {
	case ch <- u:
	case <-ctx.Done():
	}
}

// Settled drains every pending refinement and returns the fully enriched app
// list. Callers that cannot display partial data (scripted output) use this;
// callers that render progressively consume Updates directly. Returns the
// list as-is when ctx is cancelled first.
func (c *AppCatalog) Settled(ctx context.Context) []device.App {
	defer c.Stop()

	apps := make([]device.App, len(c.Apps))
	copy(apps, c.Apps)

	if c.Updates == nil {
		return apps
	}

	index := make(map[string]int, len(apps))
	for i, a := range apps {
		index[a.BundleID] = i
	}

	for {
		select {
		case u, ok := <-c.Updates:
			if !ok {
				return apps
			}
			i, found := index[u.BundleID]
			if !found {
				continue
			}
			Apply(&apps[i], u)
		case <-ctx.Done():
			return apps
		}
	}
}

// Apply folds a refinement into an app record. Shared by progressive and
// drained consumers so both end up with identical data.
func Apply(app *device.App, u AppUpdate) {
	if u.Version != "" {
		app.Version = u.Version
	}
	if u.ReactNative {
		app.IsReactNative = true
	}
}
