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
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "\tdevice") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				// This is the serial, e.g., emulator-5554
				// Mapping this back to AVD name requires 'adb -s <serial> emu avd name'
				// For now, we'll just mark it as potentially running if we find it
				serial := parts[0]
				
				// Try to get the actual AVD name for this serial
				nameCmd := exec.CommandContext(ctx, "adb", "-s", serial, "emu", "avd", "name")
				nameOut, err := nameCmd.Output()
				if err == nil {
					avdName := strings.TrimSpace(string(nameOut))
					if avdName != "" {
						running[avdName] = true
						continue
					}
				}
				running[serial] = true
			}
		}
	}
	return running, nil
}
