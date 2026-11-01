package device

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"simmer/internal/logging"
)

// PortReverser is an optional interface a Manager may implement to forward a
// device port to the host (`adb reverse`).
type PortReverser interface {
	Platform() Platform
	ReversePort(ctx context.Context, deviceID, port string) error
}

// ValidatePort checks that s is a TCP port number in 1-65535.
func ValidatePort(s string) error {
	if s == "" {
		return fmt.Errorf("required")
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("port must be 1-65535")
	}
	return nil
}

// ReversePort forwards port on the device to the same port on the host.
func (c *Coordinator) ReversePort(ctx context.Context, dev Device, port string) error {
	r, err := pickCapability[PortReverser](c.Managers, dev.Platform, dev.Kind, "port reverser")
	if err != nil {
		return err
	}
	return r.ReversePort(ctx, dev.ID, port)
}

func (m *androidManager) ReversePort(ctx context.Context, deviceID, port string) error {
	serial, err := resolveAndroidSerial(ctx, deviceID)
	if err != nil {
		return fmt.Errorf("find android serial: %w", err)
	}
	return adbReverse(ctx, m.logger, serial, port)
}

func (m *physicalAndroidManager) ReversePort(ctx context.Context, deviceID, port string) error {
	return adbReverse(ctx, m.logger, deviceID, port)
}

func adbReverse(ctx context.Context, logger *logging.Logger, serial, port string) error {
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	args := androidReverseArgs(serial, port)
	out, err := exec.CommandContext(cctx, "adb", args...).CombinedOutput()
	logger.LogExec("adb", args, string(out), err)
	if err != nil {
		return fmt.Errorf("adb reverse tcp:%s: %w: %s", port, err, strings.TrimSpace(string(out)))
	}
	return nil
}
