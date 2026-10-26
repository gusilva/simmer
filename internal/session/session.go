// Package session is Simmer's transport-agnostic interface over device
// tooling. It owns operation deadlines, the discovery roster, device
// reference resolution and user configuration, so the TUI (internal/app)
// and the headless commands (internal/cli) are two adapters over one
// module rather than two implementations of the same orchestration.
//
// Nothing in this package may import Bubble Tea or any rendering library.
package session

import (
	"context"
	"sync"
	"time"

	"simmer/internal/config"
	"simmer/internal/device"
	"simmer/internal/logging"
)

// Operation deadlines. Callers pass a context for cancellation; the deadline
// for each operation belongs to this module so every adapter inherits the
// same tuned values.
const (
	discoveryTimeout   = 15 * time.Second
	appsTimeout        = 10 * time.Second
	appVersionTimeout  = 10 * time.Second
	reactNativeTimeout = 20 * time.Second
	localTreeTimeout   = 10 * time.Second
	physicalTimeout    = 30 * time.Second
	queryTimeout       = 30 * time.Second
	schemaTimeout      = 15 * time.Second
)

// Session holds everything a Simmer run needs to talk to devices: the
// manager coordinator, the command log, user configuration, and the roster
// produced by the most recent discovery.
type Session struct {
	coord  *device.Coordinator
	logger *logging.Logger
	cfg    config.Config

	// mu guards everything that changes after construction: the roster is
	// replaced by discovery on one goroutine and read by resolution on
	// another, and cfg is reloaded by the settings form while queries read
	// it.
	mu     sync.RWMutex
	roster []device.Device
}

// New returns a Session over the given coordinator. logger may be nil.
func New(coord *device.Coordinator, logger *logging.Logger, cfg config.Config) *Session {
	return &Session{coord: coord, logger: logger, cfg: cfg}
}

// NewDefault returns a Session wired to the standard managers (iOS
// simulators, physical iOS, Android emulators, physical Android) with
// configuration loaded from disk. A config read failure is returned
// alongside a usable Session carrying defaults.
func NewDefault(logger *logging.Logger) (*Session, error) {
	cfg, err := config.Load()
	coord := device.NewCoordinator(
		device.NewIOSManager(logger),
		device.NewPhysicalIOSManager(logger),
		device.NewAndroidManager(logger),
		device.NewPhysicalAndroidManager(logger),
	)
	return New(coord, logger, cfg), err
}

// Coordinator exposes the underlying coordinator for operations that have
// not been migrated behind this module yet.
func (s *Session) Coordinator() *device.Coordinator { return s.coord }

// Config returns the loaded user configuration.
func (s *Session) Config() config.Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// ListDevices discovers devices on every registered platform concurrently
// and records the result as the session's roster, which Resolve matches
// references against.
func (s *Session) ListDevices(ctx context.Context) device.DiscoveryResult {
	ctx, cancel := context.WithTimeout(ctx, discoveryTimeout)
	defer cancel()

	res := s.coord.Discover(ctx)

	s.mu.Lock()
	s.roster = res.Devices
	s.mu.Unlock()

	return res
}

// devices returns the roster from the most recent ListDevices call.
func (s *Session) devices() []device.Device {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]device.Device, len(s.roster))
	copy(out, s.roster)
	return out
}

// deviceTimeout returns the deadline for a per-device round-trip. Physical
// hardware talks over USB and is consistently slower than a simulator or
// emulator on local loopback.
func deviceTimeout(dev device.Device) time.Duration {
	if dev.Kind == device.KindPhysical {
		return physicalTimeout
	}
	return localTreeTimeout
}

// FileTree walks dev's filesystem at the given scope. bundleID is required
// for device.ScopeAppSandbox and ignored otherwise.
func (s *Session) FileTree(ctx context.Context, dev device.Device, scope device.FileScope, bundleID string) (device.FileNode, error) {
	ctx, cancel := context.WithTimeout(ctx, deviceTimeout(dev))
	defer cancel()
	return s.coord.FileTree(ctx, dev, scope, bundleID)
}

// Info returns the detailed facts about a device.
func (s *Session) Info(ctx context.Context, dev device.Device) (device.DeviceInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, schemaTimeout)
	defer cancel()
	return s.coord.Info(ctx, dev)
}

// ReloadConfig re-reads user configuration from disk. The settings form
// persists its changes as a file, so the Session has to pick them up again
// for subsequent queries to use them.
func (s *Session) ReloadConfig() error {
	cfg, err := config.Load()

	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()

	return err
}
