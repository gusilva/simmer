package device

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type androidManager struct{}

// NewAndroidManager returns a Manager implementation for Android emulators.
func NewAndroidManager() Manager {
	return &androidManager{}
}

// Platform reports the platform this manager handles.
func (m *androidManager) Platform() Platform { return PlatformAndroid }

// Boot launches the emulator for the given AVD name. The emulator process is
// started detached (we don't wait for it) because it runs until explicitly
// closed; the context is intentionally not forwarded to the child process so
// the 30-second boot timeout doesn't kill the emulator window.
func (m *androidManager) Boot(_ context.Context, id string) error {
	cmd := exec.Command("emulator", "-avd", id) //nolint:gosec
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("emulator -avd %s: %w", id, err)
	}

	go cmd.Wait()
	return nil
}

// Shutdown sends a kill command to the running emulator whose AVD name matches
// id. It maps the AVD name back to an adb serial via `adb -s <serial> emu avd
// name`, then issues `adb -s <serial> emu kill`.
func (m *androidManager) Shutdown(ctx context.Context, id string) error {
	serial, err := m.findSerial(ctx, id)
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "adb", "-s", serial, "emu", "kill")
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("adb -s %s emu kill: %w", serial, err)
		}

		return fmt.Errorf("adb -s %s emu kill: %w: %s", serial, err, msg)
	}

	return nil
}

// findSerial returns the adb serial (e.g. "emulator-5554") for the running
// emulator whose AVD name matches avdName.
func (m *androidManager) findSerial(ctx context.Context, avdName string) (string, error) {
	out, err := exec.CommandContext(ctx, "adb", "devices").Output()
	if err != nil {
		return "", fmt.Errorf("adb devices: %w", err)
	}
	for line := range strings.SplitSeq(string(out), "\n") {
		if !strings.Contains(line, "\tdevice") {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		serial := parts[0]
		nameOut, err := exec.CommandContext(ctx, "adb", "-s", serial, "emu", "avd", "name").Output()
		if err != nil {
			continue
		}

		// Output is "<name>\nOK\n" — take first line only.
		firstLine := strings.TrimSpace(strings.SplitN(string(nameOut), "\n", 2)[0])
		if firstLine == avdName {
			return serial, nil
		}
	}

	return "", fmt.Errorf("no running emulator found for AVD %q", avdName)
}

// ToolVersion returns the adb version string (e.g. "1.0.41").
// First line of `adb version` is "Android Debug Bridge version 1.0.41".
func (m *androidManager) ToolVersion(ctx context.Context) (Platform, string) {
	out, err := exec.CommandContext(ctx, "adb", "version").Output()
	if err != nil {
		return PlatformAndroid, "n/a"
	}

	line := strings.SplitN(string(out), "\n", 2)[0]
	const prefix = "Android Debug Bridge version "

	return PlatformAndroid, strings.TrimPrefix(strings.TrimSpace(line), prefix)
}

func (m *androidManager) ListDevices(ctx context.Context) ([]Device, error) {
	// 1. Get defined emulators
	avds, err := m.getAVDs(ctx)
	if err != nil {
		return nil, err
	}

	// 2. Get running devices via adb
	running, err := m.getRunningDevices(ctx)
	if err != nil {
		// If adb fails, we still return the AVDs with "Shutdown" status
		running = make(map[string]bool)
	}

	var devices []Device
	for _, name := range avds {
		status := StatusOff
		// Note: matching AVD name to adb device name can be tricky
		// Usually emulator-5554 style, but we'll do a simple check
		if running[name] {
			status = StatusRunning
		}

		devices = append(devices, Device{
			ID:       name,
			Name:     name,
			Platform: PlatformAndroid,
			Version:  "Unknown", // AVD list doesn't easily give version without more commands
			Status:   status,
		})
	}

	return devices, nil
}

func (m *androidManager) getAVDs(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "emulator", "-list-avds")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run emulator -list-avds: %w", err)
	}

	return parseAVDs(output), nil
}

func parseAVDs(output []byte) []string {
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	var avds []string
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			avds = append(avds, trimmed)
		}
	}
	return avds
}

func (m *androidManager) getRunningDevices(ctx context.Context) (map[string]bool, error) {
	cmd := exec.CommandContext(ctx, "adb", "devices")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run adb devices: %w", err)
	}

	running := make(map[string]bool)
	lines := strings.SplitSeq(string(output), "\n")
	for line := range lines {
		if strings.Contains(line, "\tdevice") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				// This is the serial, e.g., emulator-5554
				// Mapping this back to AVD name requires 'adb -s <serial> emu avd name'
				// For now, we'll just mark it as potentially running if we find it
				serial := parts[0]

				// Try to get the actual AVD name for this serial.
				// Output is "<name>\nOK\n" — take first line only.
				nameCmd := exec.CommandContext(ctx, "adb", "-s", serial, "emu", "avd", "name")
				nameOut, err := nameCmd.Output()
				if err == nil {
					firstLine := strings.TrimSpace(strings.SplitN(string(nameOut), "\n", 2)[0])
					if firstLine != "" {
						running[firstLine] = true
						continue
					}
				}
				running[serial] = true
			}
		}
	}
	return running, nil
}
