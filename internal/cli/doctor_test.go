package cli

import (
	"bytes"
	"strings"
	"testing"

	"simmer/internal/device"
)

func TestWarnDiscovery_NamesStoppedAdbServer(t *testing.T) {
	tests := []struct {
		state device.AdbServerState
		want  bool
	}{
		{device.AdbServerStopped, true},
		{device.AdbServerRunning, false},
		{device.AdbNotInstalled, false},
		{device.AdbServerUnknown, false},
	}
	for _, tt := range tests {
		var errBuf, outBuf bytes.Buffer
		c := &command{io: Streams{Out: &outBuf, Err: &errBuf}}
		c.warnDiscovery(device.DiscoveryResult{AdbServer: tt.state})

		got := strings.Contains(errBuf.String(), "adb server not running")
		if got != tt.want {
			t.Errorf("%v: warned=%v, want %v (stderr %q)", tt.state, got, tt.want, errBuf.String())
		}
		if outBuf.Len() != 0 {
			t.Errorf("%v: warning leaked to stdout: %q", tt.state, outBuf.String())
		}
	}
}

func TestSeverityMark(t *testing.T) {
	for sev, want := range map[device.Severity]string{
		device.SeverityOK:   "✓",
		device.SeverityWarn: "!",
		device.SeverityFail: "✗",
	} {
		if got := severityMark(sev); got != want {
			t.Errorf("severityMark(%s) = %q, want %q", sev, got, want)
		}
	}
}
