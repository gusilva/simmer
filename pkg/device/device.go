// Package device provides functionality to discover and manage mobile emulators and simulators.
package device

import (
	"context"
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
	ID       string   // Unique identifier (UDID for iOS, Serial/Name for Android)
	Name     string   // Human-readable name
	Platform Platform // iOS or Android
	Version  string   // OS Version
	Status   Status   // Running or Shutdown
}

// Manager defines the contract for discovering devices on a specific platform.
type Manager interface {
	// ListDevices returns all available devices for the platform.
	// It must respect context cancellation.
	ListDevices(ctx context.Context) ([]Device, error)
}

// ToolVersioner is an optional interface a Manager may implement to report
// the version of its underlying CLI tool.
type ToolVersioner interface {
	ToolVersion(ctx context.Context) (Platform, string)
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
