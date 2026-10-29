package device

import (
	"context"
	"fmt"
)

// InfoField is one labelled fact about a device.
type InfoField struct {
	Key   string
	Value string
}

// DeviceInfo collects platform-specific facts about a device for display.
// Field order is meaningful — UI renders fields in the order the lister
// returned them.
type DeviceInfo struct {
	Fields []InfoField
}

// InfoLister is an optional interface a Manager may implement to report
// detailed information about a single device.
type InfoLister interface {
	Platform() Platform
	Info(ctx context.Context, dev Device) (DeviceInfo, error)
}

// Info reports the detailed facts about a device.
func (c *Coordinator) Info(ctx context.Context, dev Device) (DeviceInfo, error) {
	l, err := pickCapability[InfoLister](c.Managers, dev.Platform, dev.Kind, "info lister")
	if err != nil {
		return DeviceInfo{}, err
	}
	return l.Info(ctx, dev)
}

// formatBytes renders a byte count with a 1024-based unit suffix.
func formatBytes(b int64) string {
	const k = 1 << 10
	switch {
	case b >= 1<<40:
		return fmt.Sprintf("%.1f TB", float64(b)/(1<<40))
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/(1<<20))
	case b >= k:
		return fmt.Sprintf("%.1f KB", float64(b)/k)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
