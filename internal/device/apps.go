package device

import (
	"context"
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

// FetchAppVersions returns version names for the given bundle ids, or nil
// when this device's kind has no fetcher: an emulator's manager must not
// answer for a phone.
func (c *Coordinator) FetchAppVersions(ctx context.Context, dev Device, bundleIDs []string) map[string]string {
	f, err := pickKinded[AppVersionFetcher](c.Managers, dev.Platform, dev.Kind, "app version fetcher")
	if err != nil {
		return nil
	}
	return f.FetchAppVersions(ctx, dev.ID, bundleIDs)
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

// DetectReactNative reports whether an app is a React Native build, or
// false when this device's kind has no detector.
func (c *Coordinator) DetectReactNative(ctx context.Context, dev Device, app App) bool {
	d, err := pickKinded[ReactNativeDetector](c.Managers, dev.Platform, dev.Kind, "react native detector")
	if err != nil {
		return false
	}
	return d.DetectReactNative(ctx, dev.ID, app)
}

// AppInstaller is an optional interface a Manager may implement to install an
// application onto a device. path is platform-specific (see iosManager and
// androidManager). scheme is the Xcode build scheme; Android ignores it.
type AppInstaller interface {
	Platform() Platform
	InstallApp(ctx context.Context, deviceID, path, scheme string) error
}

// ListApps returns the apps installed on a device.
func (c *Coordinator) ListApps(ctx context.Context, dev Device) ([]App, error) {
	a, err := pickCapability[AppLister](c.Managers, dev.Platform, dev.Kind, "app lister")
	if err != nil {
		return nil, err
	}
	return a.ListApps(ctx, dev.ID)
}

// InstallApp installs an app onto a device.
func (c *Coordinator) InstallApp(ctx context.Context, dev Device, path, scheme string) error {
	i, err := pickCapability[AppInstaller](c.Managers, dev.Platform, dev.Kind, "app installer")
	if err != nil {
		return err
	}
	return i.InstallApp(ctx, dev.ID, path, scheme)
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

// LaunchApp starts an already-installed app.
func (c *Coordinator) LaunchApp(ctx context.Context, dev Device, bundleID string) error {
	l, err := pickCapability[AppLauncher](c.Managers, dev.Platform, dev.Kind, "app launcher")
	if err != nil {
		return err
	}
	return l.LaunchApp(ctx, dev.ID, bundleID)
}

// TerminateApp stops a running app. Callers that treat termination as
// best-effort (the app may simply not be running) may ignore the error.
func (c *Coordinator) TerminateApp(ctx context.Context, dev Device, bundleID string) error {
	t, err := pickCapability[AppTerminator](c.Managers, dev.Platform, dev.Kind, "app terminator")
	if err != nil {
		return err
	}
	return t.TerminateApp(ctx, dev.ID, bundleID)
}

// BundlerConfigurer is an optional interface a Manager may implement to point
// a React Native app's JS bundler (Metro) at a given host:port and relaunch
// the app so it picks up the change.
type BundlerConfigurer interface {
	Platform() Platform
	SetBundlerLocation(ctx context.Context, deviceID, bundleID, hostPort string) error
}

// SetBundlerLocation points an app's JS bundler at hostPort.
func (c *Coordinator) SetBundlerLocation(ctx context.Context, dev Device, bundleID, hostPort string) error {
	b, err := pickCapability[BundlerConfigurer](c.Managers, dev.Platform, dev.Kind, "bundler configurer")
	if err != nil {
		return err
	}
	return b.SetBundlerLocation(ctx, dev.ID, bundleID, hostPort)
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

// TriggerDevMenu opens the React Native dev menu on a device.
func (c *Coordinator) TriggerDevMenu(ctx context.Context, dev Device, metroPort string) error {
	t, err := pickCapability[DevMenuTrigger](c.Managers, dev.Platform, dev.Kind, "dev menu trigger")
	if err != nil {
		return err
	}
	return t.TriggerDevMenu(ctx, dev.ID, metroPort)
}

// DeleteApp uninstalls an app from a device.
func (c *Coordinator) DeleteApp(ctx context.Context, dev Device, bundleID string) error {
	d, err := pickCapability[AppDeleter](c.Managers, dev.Platform, dev.Kind, "app deleter")
	if err != nil {
		return err
	}
	return d.DeleteApp(ctx, dev.ID, bundleID)
}
