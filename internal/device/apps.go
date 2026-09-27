package device

import (
	"context"
	"fmt"
)

// App describes one application installed on a device.
type App struct {
	BundleID      string
	DisplayName   string // CFBundleDisplayName (preferred)
	Name          string // CFBundleName (fallback)
	Version       string // CFBundleVersion
	ShortVersion  string // CFBundleShortVersionString
	Type          string // ApplicationType: "User", "System", ...
	Path          string // on-disk bundle path
	IsReactNative bool   // detected React Native app (main.jsbundle / hermes)
}

// Label returns the user-facing display label for the app, falling back from
// CFBundleDisplayName → CFBundleName → bundle identifier.
func (a App) Label() string {
	switch {
	case a.DisplayName != "":
		return a.DisplayName
	case a.Name != "":
		return a.Name
	default:
		return a.BundleID
	}
}

// IsLocal reports whether this app is a developer build rather than a
// system app. On the simulator/emulator, ApplicationType "User" implies a
// local build since there is no App Store to install from. See CONTEXT.md
// "Local app".
func (a App) IsLocal() bool {
	return a.Type == "User"
}

// AppLister is an optional interface a Manager may implement to enumerate the
// applications installed on a device.
type AppLister interface {
	Platform() Platform
	ListApps(ctx context.Context, id string) ([]App, error)
}

// AppDeleter is an optional interface a Manager may implement to uninstall an
// application from a device.
type AppDeleter interface {
	Platform() Platform
	DeleteApp(ctx context.Context, deviceID, bundleID string) error
}

// AppVersionFetcher is an optional interface a Manager may implement when
// version names come from a separate device round-trip, so ListApps can
// return the bare app list first and the caller fetches versions after.
// Android needs this (dumpsys is a second adb call); iOS does not — listapps
// / BrowseUserApps already include version info in the one call ListApps
// makes.
type AppVersionFetcher interface {
	Platform() Platform
	FetchAppVersions(ctx context.Context, deviceID string, bundleIDs []string) map[string]string
}

// FetchAppVersions routes to the Manager that implements AppVersionFetcher
// for the device's Platform and Kind. Returns nil if none matches.
func (c *Coordinator) FetchAppVersions(ctx context.Context, dev Device, bundleIDs []string) map[string]string {
	for _, m := range c.Managers {
		f, ok := m.(AppVersionFetcher)
		if !ok || f.Platform() != dev.Platform {
			continue
		}
		k, isKinded := m.(KindedManager)
		if !isKinded || k.Kind() != dev.Kind {
			continue
		}
		return f.FetchAppVersions(ctx, dev.ID, bundleIDs)
	}
	return nil
}

// ReactNativeDetector is an optional interface a Manager may implement when
// its React Native check needs a device round-trip (APK pull, AFC lookup)
// too slow to run inline in ListApps. Callers run it asynchronously per app
// after the app list is already showing. Simulator ListApps checks the
// local bundle directly and never needs this.
type ReactNativeDetector interface {
	Platform() Platform
	DetectReactNative(ctx context.Context, deviceID string, app App) bool
}

// DetectReactNative routes to the Manager that implements ReactNativeDetector
// for the device's Platform and Kind. Returns false if none matches.
func (c *Coordinator) DetectReactNative(ctx context.Context, dev Device, app App) bool {
	for _, m := range c.Managers {
		rd, ok := m.(ReactNativeDetector)
		if !ok || rd.Platform() != dev.Platform {
			continue
		}
		k, isKinded := m.(KindedManager)
		if !isKinded || k.Kind() != dev.Kind {
			continue
		}
		return rd.DetectReactNative(ctx, dev.ID, app)
	}
	return false
}

// AppInstaller is an optional interface a Manager may implement to install an
// application onto a device. path is platform-specific (see iosManager and
// androidManager). scheme is the Xcode build scheme; Android ignores it.
type AppInstaller interface {
	Platform() Platform
	InstallApp(ctx context.Context, deviceID, path, scheme string) error
}

// ListApps routes to the Manager that implements AppLister for the device's
// Platform. Prefers a KindedManager whose Kind matches dev.Kind; falls back to
// any AppLister that does not implement KindedManager.
func (c *Coordinator) ListApps(ctx context.Context, dev Device) ([]App, error) {
	for _, m := range c.Managers {
		a, ok := m.(AppLister)
		if !ok || a.Platform() != dev.Platform {
			continue
		}
		k, isKinded := m.(KindedManager)
		if !isKinded || k.Kind() != dev.Kind {
			continue
		}
		return a.ListApps(ctx, dev.ID)
	}
	for _, m := range c.Managers {
		a, ok := m.(AppLister)
		if !ok || a.Platform() != dev.Platform {
			continue
		}
		if _, isKinded := m.(KindedManager); isKinded {
			continue
		}
		return a.ListApps(ctx, dev.ID)
	}
	return nil, fmt.Errorf("no app lister registered for platform %s", dev.Platform)
}

// InstallApp installs an app onto a device, routing to the Manager that
// implements AppInstaller for the device's Platform.
func (c *Coordinator) InstallApp(ctx context.Context, dev Device, path, scheme string) error {
	for _, m := range c.Managers {
		i, ok := m.(AppInstaller)
		if !ok || i.Platform() != dev.Platform {
			continue
		}
		return i.InstallApp(ctx, dev.ID, path, scheme)
	}
	return fmt.Errorf("no app installer registered for platform %s", dev.Platform)
}

// AppTerminator is an optional interface a Manager may implement to stop a
// running application on a device without uninstalling it.
type AppTerminator interface {
	Platform() Platform
	TerminateApp(ctx context.Context, deviceID, bundleID string) error
}

// AppLauncher is an optional interface a Manager may implement to start an
// already-installed application on a device.
type AppLauncher interface {
	Platform() Platform
	LaunchApp(ctx context.Context, deviceID, bundleID string) error
}

// LaunchApp starts an app, routing to the Manager that implements AppLauncher
// for the device's Platform, preferring a KindedManager match.
func (c *Coordinator) LaunchApp(ctx context.Context, dev Device, bundleID string) error {
	for _, m := range c.Managers {
		l, ok := m.(AppLauncher)
		if !ok || l.Platform() != dev.Platform {
			continue
		}
		k, isKinded := m.(KindedManager)
		if !isKinded || k.Kind() != dev.Kind {
			continue
		}
		return l.LaunchApp(ctx, dev.ID, bundleID)
	}
	for _, m := range c.Managers {
		l, ok := m.(AppLauncher)
		if !ok || l.Platform() != dev.Platform {
			continue
		}
		if _, isKinded := m.(KindedManager); isKinded {
			continue
		}
		return l.LaunchApp(ctx, dev.ID, bundleID)
	}
	return fmt.Errorf("no app launcher registered for platform %s", dev.Platform)
}

// TerminateApp stops a running app, routing to the Manager that implements
// AppTerminator for the device's Platform, preferring a KindedManager match.
// Callers that treat termination as best-effort (the app may simply not be
// running) may ignore the returned error.
func (c *Coordinator) TerminateApp(ctx context.Context, dev Device, bundleID string) error {
	for _, m := range c.Managers {
		t, ok := m.(AppTerminator)
		if !ok || t.Platform() != dev.Platform {
			continue
		}
		k, isKinded := m.(KindedManager)
		if !isKinded || k.Kind() != dev.Kind {
			continue
		}
		return t.TerminateApp(ctx, dev.ID, bundleID)
	}
	for _, m := range c.Managers {
		t, ok := m.(AppTerminator)
		if !ok || t.Platform() != dev.Platform {
			continue
		}
		if _, isKinded := m.(KindedManager); isKinded {
			continue
		}
		return t.TerminateApp(ctx, dev.ID, bundleID)
	}
	return fmt.Errorf("no app terminator registered for platform %s", dev.Platform)
}

// BundlerConfigurer is an optional interface a Manager may implement to point
// a React Native app's JS bundler (Metro) at a given host:port and relaunch
// the app so it picks up the change.
type BundlerConfigurer interface {
	Platform() Platform
	SetBundlerLocation(ctx context.Context, deviceID, bundleID, hostPort string) error
}

// SetBundlerLocation routes to the Manager that implements BundlerConfigurer
// for the device's Platform, preferring a KindedManager match whose Kind
// equals dev.Kind; falls back to any BundlerConfigurer that does not
// implement KindedManager (e.g. iosManager, which serves only simulators and
// never needs to disambiguate Kind).
func (c *Coordinator) SetBundlerLocation(ctx context.Context, dev Device, bundleID, hostPort string) error {
	for _, m := range c.Managers {
		b, ok := m.(BundlerConfigurer)
		if !ok || b.Platform() != dev.Platform {
			continue
		}
		k, isKinded := m.(KindedManager)
		if !isKinded || k.Kind() != dev.Kind {
			continue
		}
		return b.SetBundlerLocation(ctx, dev.ID, bundleID, hostPort)
	}
	for _, m := range c.Managers {
		b, ok := m.(BundlerConfigurer)
		if !ok || b.Platform() != dev.Platform {
			continue
		}
		if _, isKinded := m.(KindedManager); isKinded {
			continue
		}
		return b.SetBundlerLocation(ctx, dev.ID, bundleID, hostPort)
	}
	return fmt.Errorf("no bundler configurer registered for platform %s", dev.Platform)
}

// DevMenuTrigger is an optional interface a Manager may implement to open the
// React Native dev menu on a device. It is device-wide, not app-scoped — the
// dev menu key/gesture always targets whatever app is foreground. metroPort
// is only meaningful to a manager whose trigger routes through Metro (only
// physicalIOSManager, currently) — pass "" to use that manager's default.
type DevMenuTrigger interface {
	Platform() Platform
	TriggerDevMenu(ctx context.Context, deviceID, metroPort string) error
}

// TriggerDevMenu routes to the Manager that implements DevMenuTrigger for the
// device's Platform, preferring a KindedManager match whose Kind equals
// dev.Kind; falls back to any DevMenuTrigger that does not implement
// KindedManager (e.g. iosManager, which serves only simulators and never
// needs to disambiguate Kind).
func (c *Coordinator) TriggerDevMenu(ctx context.Context, dev Device, metroPort string) error {
	for _, m := range c.Managers {
		t, ok := m.(DevMenuTrigger)
		if !ok || t.Platform() != dev.Platform {
			continue
		}
		k, isKinded := m.(KindedManager)
		if !isKinded || k.Kind() != dev.Kind {
			continue
		}
		return t.TriggerDevMenu(ctx, dev.ID, metroPort)
	}
	for _, m := range c.Managers {
		t, ok := m.(DevMenuTrigger)
		if !ok || t.Platform() != dev.Platform {
			continue
		}
		if _, isKinded := m.(KindedManager); isKinded {
			continue
		}
		return t.TriggerDevMenu(ctx, dev.ID, metroPort)
	}
	return fmt.Errorf("no dev menu trigger registered for platform %s", dev.Platform)
}

// DeleteApp uninstalls the given app from a device, routing to the Manager that
// implements AppDeleter for the device's Platform, preferring a KindedManager match.
func (c *Coordinator) DeleteApp(ctx context.Context, dev Device, bundleID string) error {
	for _, m := range c.Managers {
		d, ok := m.(AppDeleter)
		if !ok || d.Platform() != dev.Platform {
			continue
		}
		k, isKinded := m.(KindedManager)
		if !isKinded || k.Kind() != dev.Kind {
			continue
		}
		return d.DeleteApp(ctx, dev.ID, bundleID)
	}
	for _, m := range c.Managers {
		d, ok := m.(AppDeleter)
		if !ok || d.Platform() != dev.Platform {
			continue
		}
		if _, isKinded := m.(KindedManager); isKinded {
			continue
		}
		return d.DeleteApp(ctx, dev.ID, bundleID)
	}
	return fmt.Errorf("no app deleter registered for platform %s", dev.Platform)
}
