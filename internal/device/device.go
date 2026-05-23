// Package device provides functionality to discover and manage mobile emulators and simulators.
package device

import (
	"context"
	"fmt"
)

// DeviceKind distinguishes physical hardware from virtual devices.
type DeviceKind int

const (
	// KindVirtual is a simulator or emulator.
	KindVirtual DeviceKind = iota
	// KindPhysical is a real device connected via USB or WiFi ADB.
	KindPhysical
)

// Platform represents the operating system of the device (e.g., iOS, Android).
type Platform string

const (
	// PlatformIOS represents Apple's iOS platform.
	PlatformIOS Platform = "iOS"
	// PlatformAndroid represents Google's Android platform.
	PlatformAndroid Platform = "Android"
)

// Status represents the current operational state of a device.
type Status string

const (
	// StatusRunning indicates the device is currently booted and active.
	StatusRunning Status = "Running"
	// StatusOff indicates the device is shutdown or unavailable.
	StatusOff Status = "Shutdown"
)

// Device represents a specific simulator or emulator instance.
type Device struct {
	ID       string     // Unique identifier (UDID for iOS, Serial/Name for Android)
	Name     string     // Human-readable name
	Platform Platform   // iOS or Android
	Version  string     // OS Version
	Status   Status     // Running or Shutdown
	Kind     DeviceKind // Physical or Virtual (zero value = KindVirtual)
}

// Manager defines the contract for discovering devices on a specific platform.
type Manager interface {
	// ListDevices returns all available devices for the platform.
	// It must respect context cancellation.
	ListDevices(ctx context.Context) ([]Device, error)
}

// KindedManager is optionally implemented by managers that serve exactly one
// DeviceKind. Coordinator uses this to route physical vs. virtual operations.
type KindedManager interface {
	Kind() DeviceKind
}

// ToolVersioner is an optional interface a Manager may implement to report
// the version of its underlying CLI tool.
type ToolVersioner interface {
	ToolVersion(ctx context.Context) (Platform, string)
}

// Booter is an optional interface a Manager may implement to start a device.
// Platform reports which Platform this booter handles so a Coordinator can
// route boot requests to the right manager.
type Booter interface {
	Platform() Platform
	Boot(ctx context.Context, id string) error
}

// Shutdowner is an optional interface a Manager may implement to stop a device.
// Platform reports which Platform this shutdowner handles so a Coordinator can
// route shutdown requests to the right manager.
type Shutdowner interface {
	Platform() Platform
	Shutdown(ctx context.Context, id string) error
}

// DeviceType represents a simulator device type template (e.g. "iPhone 16 Pro").
type DeviceType struct {
	Name       string
	Identifier string
}

// Runtime represents a simulator runtime (e.g. "iOS 18.5").
type Runtime struct {
	Name        string
	Identifier  string
	Version     string
	IsAvailable bool
}

// Creator is an optional interface for creating new simulators/emulators.
type Creator interface {
	Platform() Platform
	Create(ctx context.Context, name, deviceTypeID, runtimeID string) (string, error)
}

// Deleter is an optional interface for deleting simulators/emulators.
type Deleter interface {
	Platform() Platform
	Delete(ctx context.Context, id string) error
}

// DeviceTypeLister enumerates available device type templates.
type DeviceTypeLister interface {
	Platform() Platform
	ListDeviceTypes(ctx context.Context) ([]DeviceType, error)
}

// RuntimeLister enumerates available runtimes.
type RuntimeLister interface {
	Platform() Platform
	ListRuntimes(ctx context.Context) ([]Runtime, error)
}

// DiscoveryResult holds the results of a multi-platform discovery operation.
type DiscoveryResult struct {
	Devices      []Device
	Errors       []error
	ToolVersions map[Platform]string
}

// Coordinator manages multiple Managers to perform concurrent device discovery.
type Coordinator struct {
	Managers []Manager
}

// NewCoordinator creates a coordinator with the provided managers.
func NewCoordinator(managers ...Manager) *Coordinator {
	return &Coordinator{Managers: managers}
}

// Boot starts the given device by routing to a Manager that implements
// Booter for the device's Platform. Returns an error if no booter is
// registered for the platform or if the underlying boot command fails.
func (c *Coordinator) Boot(ctx context.Context, dev Device) error {
	for _, m := range c.Managers {
		b, ok := m.(Booter)
		if !ok {
			continue
		}
		if b.Platform() != dev.Platform {
			continue
		}
		return b.Boot(ctx, dev.ID)
	}
	return fmt.Errorf("no booter registered for platform %s", dev.Platform)
}

// Shutdown stops the given device by routing to a Manager that implements
// Shutdowner for the device's Platform. Returns an error if no shutdowner is
// registered for the platform or if the underlying shutdown command fails.
func (c *Coordinator) Shutdown(ctx context.Context, dev Device) error {
	for _, m := range c.Managers {
		s, ok := m.(Shutdowner)
		if !ok {
			continue
		}
		if s.Platform() != dev.Platform {
			continue
		}
		return s.Shutdown(ctx, dev.ID)
	}
	return fmt.Errorf("no shutdowner registered for platform %s", dev.Platform)
}

// Discover fetches devices from all registered managers concurrently.
// It returns a DiscoveryResult containing all found devices and any errors encountered.
func (c *Coordinator) Discover(ctx context.Context) DiscoveryResult {
	type managerResult struct {
		devices     []Device
		err         error
		platform    Platform
		toolVersion string
		hasVersion  bool
	}

	ch := make(chan managerResult, len(c.Managers))

	for _, mgr := range c.Managers {
		go func(m Manager) {
			r := managerResult{}
			r.devices, r.err = m.ListDevices(ctx)
			if tv, ok := m.(ToolVersioner); ok {
				r.platform, r.toolVersion = tv.ToolVersion(ctx)
				r.hasVersion = true
			}
			ch <- r
		}(mgr)
	}

	res := DiscoveryResult{ToolVersions: make(map[Platform]string)}
	for range len(c.Managers) {
		r := <-ch
		res.Devices = append(res.Devices, r.devices...)
		if r.err != nil {
			res.Errors = append(res.Errors, r.err)
		}
		if r.hasVersion {
			res.ToolVersions[r.platform] = r.toolVersion
		}
	}
	return res
}

// Create creates a new simulator for the given platform, routing to a Manager
// that implements Creator. Returns the new simulator's UDID.
func (c *Coordinator) Create(ctx context.Context, platform Platform, name, deviceTypeID, runtimeID string) (string, error) {
	for _, m := range c.Managers {
		cr, ok := m.(Creator)
		if !ok || cr.Platform() != platform {
			continue
		}
		return cr.Create(ctx, name, deviceTypeID, runtimeID)
	}
	return "", fmt.Errorf("no creator registered for platform %s", platform)
}

// Delete removes the given simulator, routing to a Manager that implements Deleter.
func (c *Coordinator) Delete(ctx context.Context, dev Device) error {
	for _, m := range c.Managers {
		d, ok := m.(Deleter)
		if !ok || d.Platform() != dev.Platform {
			continue
		}
		return d.Delete(ctx, dev.ID)
	}
	return fmt.Errorf("no deleter registered for platform %s", dev.Platform)
}

// ListDeviceTypes returns available device type templates for the given platform.
func (c *Coordinator) ListDeviceTypes(ctx context.Context, platform Platform) ([]DeviceType, error) {
	for _, m := range c.Managers {
		l, ok := m.(DeviceTypeLister)
		if !ok || l.Platform() != platform {
			continue
		}
		return l.ListDeviceTypes(ctx)
	}
	return nil, fmt.Errorf("no device type lister for platform %s", platform)
}

// ListRuntimes returns available runtimes for the given platform.
func (c *Coordinator) ListRuntimes(ctx context.Context, platform Platform) ([]Runtime, error) {
	for _, m := range c.Managers {
		l, ok := m.(RuntimeLister)
		if !ok || l.Platform() != platform {
			continue
		}
		return l.ListRuntimes(ctx)
	}
	return nil, fmt.Errorf("no runtime lister for platform %s", platform)
}
