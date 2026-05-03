package device

import (
	"context"
	"reflect"
	"testing"
)

func TestCoordinator_Discover(t *testing.T) {
	iosMock := &MockManager{
		PlatformVal:    PlatformIOS,
		DevicesVal:     []Device{{ID: "ios-1", Name: "iPhone 15", Platform: PlatformIOS}},
		ToolVersionVal: "15.3",
	}
	androidMock := &MockManager{
		PlatformVal:    PlatformAndroid,
		DevicesVal:     []Device{{ID: "and-1", Name: "Pixel 8", Platform: PlatformAndroid}},
		ToolVersionVal: "1.0.41",
	}

	coord := NewCoordinator(iosMock, androidMock)
	res := coord.Discover(context.Background())

	if len(res.Devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(res.Devices))
	}

	expectedVersions := map[Platform]string{
		PlatformIOS:     "15.3",
		PlatformAndroid: "1.0.41",
	}

	if !reflect.DeepEqual(res.ToolVersions, expectedVersions) {
		t.Errorf("expected tool versions %v, got %v", expectedVersions, res.ToolVersions)
	}
}

func TestCoordinator_Boot(t *testing.T) {
	iosMock := &MockManager{PlatformVal: PlatformIOS}
	coord := NewCoordinator(iosMock)

	dev := Device{ID: "test-id", Platform: PlatformIOS}
	err := coord.Boot(context.Background(), dev)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if !iosMock.BootCalled || iosMock.BootID != "test-id" {
		t.Errorf("boot was not called correctly on mock")
	}
}
