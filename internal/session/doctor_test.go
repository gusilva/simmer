package session

import (
	"context"
	"errors"
	"testing"

	"simmer/internal/config"
	"simmer/internal/device"
)

func TestDoctor_ReportsHostConfigAndDevices(t *testing.T) {
	orig := hostHealth
	t.Cleanup(func() { hostHealth = orig })
	hostHealth = func(context.Context) []device.HealthCheck {
		return []device.HealthCheck{{Section: device.SectionServices, Name: "adb server", Severity: device.SeverityFail, Fix: "adb start-server"}}
	}

	android := &device.MockManager{
		PlatformVal: device.PlatformAndroid,
		DevicesVal: []device.Device{
			{ID: "ok", Platform: device.PlatformAndroid},
			{ID: "R5C", Platform: device.PlatformAndroid, Problem: "unauthorized: accept the prompt"},
		},
	}
	broken := &device.MockManager{PlatformVal: device.PlatformIOS, ErrorVal: errors.New("simctl exploded")}
	s := New(device.NewCoordinator(android, broken), nil, config.Default())

	r := s.Doctor(context.Background())

	if !r.Failed() {
		t.Error("report with a failing check should be Failed")
	}
	want := map[string]device.Severity{
		"adb server": device.SeverityFail,
		"config":     device.SeverityOK,
		"R5C":        device.SeverityWarn,
		"discovery":  device.SeverityWarn,
	}
	got := map[string]device.Severity{}
	for _, c := range r.Checks {
		got[c.Name] = c.Severity
	}
	for name, sev := range want {
		if got[name] != sev {
			t.Errorf("%s = %q, want %q (checks: %+v)", name, got[name], sev, r.Checks)
		}
	}
	if _, ok := got["ok"]; ok {
		t.Error("a device with no problem should not be reported")
	}
}

func TestDoctor_ConfigErrorWarns(t *testing.T) {
	orig := hostHealth
	t.Cleanup(func() { hostHealth = orig })
	hostHealth = func(context.Context) []device.HealthCheck { return nil }

	s := New(device.NewCoordinator(), nil, config.Default())
	s.cfgErr = errors.New("read config: permission denied")

	r := s.Doctor(context.Background())
	if r.Failed() {
		t.Error("a bad config never blocks anything, so it must not fail the report")
	}
	if len(r.Checks) != 1 || r.Checks[0].Severity != device.SeverityWarn || r.Checks[0].Fix == "" {
		t.Errorf("want one config warning with a fix, got %+v", r.Checks)
	}
}
