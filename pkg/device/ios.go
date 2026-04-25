package device

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

type iosManager struct{}

// NewIOSManager returns a Manager implementation for Apple iOS simulators.
func NewIOSManager() Manager {
	return &iosManager{}
}

type simctlList struct {
	Devices map[string][]simctlDevice `json:"devices"`
}

type simctlDevice struct {
	State           string `json:"state"`
	IsAvailable     bool   `json:"isAvailable"`
	Name            string `json:"name"`
	UDID            string `json:"udid"`
	DeviceTypeID    string `json:"deviceTypeIdentifier"`
	DataPath        string `json:"dataPath"`
	LogPath         string `json:"logPath"`
}

func (m *iosManager) ListDevices(ctx context.Context) ([]Device, error) {
	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "list", "devices", "available", "--json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to run xcrun simctl: %w", err)
	}

	return parseSimctlOutput(output)
}

func parseSimctlOutput(output []byte) ([]Device, error) {
	var list simctlList
	if err := json.Unmarshal(output, &list); err != nil {
		return nil, fmt.Errorf("failed to parse simctl output: %w", err)
	}

	var devices []Device
	for runtime, runtimeDevices := range list.Devices {
		for _, d := range runtimeDevices {
			status := StatusOff
			if d.State == "Booted" {
				status = StatusRunning
			}

			devices = append(devices, Device{
				ID:       d.UDID,
				Name:     d.Name,
				Platform: PlatformIOS,
				Version:  runtime,
				Status:   status,
			})
		}
	}

	return devices, nil
}

