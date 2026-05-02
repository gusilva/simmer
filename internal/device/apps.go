package device

import (
	"context"
	"fmt"
)

// App describes one application installed on a device.
type App struct {
	BundleID     string
	DisplayName  string // CFBundleDisplayName (preferred)
	Name         string // CFBundleName (fallback)
	Version      string // CFBundleVersion
	ShortVersion string // CFBundleShortVersionString
	Type         string // ApplicationType: "User", "System", ...
	Path         string // on-disk bundle path
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

// AppLister is an optional interface a Manager may implement to enumerate the
// applications installed on a device.
type AppLister interface {
	Platform() Platform
	ListApps(ctx context.Context, id string) ([]App, error)
}

// ListApps routes to the Manager that implements AppLister for the device's
// Platform. Returns an error if no AppLister is registered for the platform.
func (c *Coordinator) ListApps(ctx context.Context, dev Device) ([]App, error) {
	for _, m := range c.Managers {
		a, ok := m.(AppLister)
		if !ok {
			continue
		}
		if a.Platform() != dev.Platform {
			continue
		}
		return a.ListApps(ctx, dev.ID)
	}
	return nil, fmt.Errorf("no app lister registered for platform %s", dev.Platform)
}
