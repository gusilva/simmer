package device

import (
	"testing"
)

func TestParseSimctlOutput(t *testing.T) {
	jsonInput := `{
		"devices": {
			"com.apple.CoreSimulator.SimRuntime.iOS-17-0": [
				{
					"state": "Booted",
					"isAvailable": true,
					"name": "iPhone 15",
					"udid": "123-456"
				},
				{
					"state": "Shutdown",
					"isAvailable": true,
					"name": "iPhone 14",
					"udid": "789-012"
				}
			]
		}
	}`

	devices, err := parseSimctlOutput([]byte(jsonInput))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(devices))
	}

	foundRunning := false
	for _, d := range devices {
		if d.Name == "iPhone 15" && d.Status == StatusRunning {
			foundRunning = true
		}
	}

	if !foundRunning {
		t.Error("expected to find running iPhone 15")
	}
}
