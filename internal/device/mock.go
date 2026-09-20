package device

import (
	"context"
)

// MockManager is a test-friendly implementation of the Manager interface
// and related optional interfaces (ToolVersioner, Booter, Shutdowner).
type MockManager struct {
	PlatformVal    Platform
	DevicesVal     []Device
	ErrorVal       error
	ToolVersionVal string
	BootError      error
	ShutdownError  error

	BootCalled     bool
	BootID         string
	ShutdownCalled bool
	ShutdownID     string

	TerminateAppError error
	TerminateCalled   bool
	TerminateBundleID string

	LaunchAppError error
	LaunchCalled   bool
	LaunchBundleID string
}

// NewMockManager creates a mock manager for the given platform.
func NewMockManager(p Platform) *MockManager {
	return &MockManager{PlatformVal: p}
}

func (m *MockManager) ListDevices(ctx context.Context) ([]Device, error) {
	if m.ErrorVal != nil {
		return nil, m.ErrorVal
	}
	return m.DevicesVal, nil
}

func (m *MockManager) Platform() Platform {
	return m.PlatformVal
}

func (m *MockManager) ToolVersion(ctx context.Context) (Platform, string) {
	return m.PlatformVal, m.ToolVersionVal
}

func (m *MockManager) Boot(ctx context.Context, id string) error {
	m.BootCalled = true
	m.BootID = id
	return m.BootError
}

func (m *MockManager) Shutdown(ctx context.Context, id string) error {
	m.ShutdownCalled = true
	m.ShutdownID = id
	return m.ShutdownError
}

func (m *MockManager) TerminateApp(ctx context.Context, deviceID, bundleID string) error {
	m.TerminateCalled = true
	m.TerminateBundleID = bundleID
	return m.TerminateAppError
}

func (m *MockManager) LaunchApp(ctx context.Context, deviceID, bundleID string) error {
	m.LaunchCalled = true
	m.LaunchBundleID = bundleID
	return m.LaunchAppError
}

// Ensure MockManager implements all desired interfaces
var (
	_ Manager       = (*MockManager)(nil)
	_ ToolVersioner = (*MockManager)(nil)
	_ Booter        = (*MockManager)(nil)
	_ Shutdowner    = (*MockManager)(nil)
	_ AppTerminator = (*MockManager)(nil)
	_ AppLauncher   = (*MockManager)(nil)
)
