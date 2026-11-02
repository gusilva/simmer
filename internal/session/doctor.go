package session

import (
	"context"
	"os"
	"time"

	"simmer/internal/config"
	"simmer/internal/device"
)

const (
	doctorTimeout    = 20 * time.Second
	startAdbTimeout  = 15 * time.Second
	doctorConfigName = "config"
)

// DoctorReport is every health check, asked the same way by the TUI and by
// `simmer doctor`. It reports and prescribes; it never fixes anything.
type DoctorReport struct {
	Checks []device.HealthCheck
}

// Failed reports whether any check makes a whole platform unusable.
func (r DoctorReport) Failed() bool {
	for _, c := range r.Checks {
		if c.Severity == device.SeverityFail {
			return true
		}
	}
	return false
}

// hostHealth is replaceable so the report's composition can be tested
// without the machine it runs on.
var hostHealth = device.HostHealth

// Doctor checks the host tools and services, the configuration, and every
// connected device that needs the user's attention. It runs a discovery,
// so the roster is current afterwards.
func (s *Session) Doctor(ctx context.Context) DoctorReport {
	ctx, cancel := context.WithTimeout(ctx, doctorTimeout)
	defer cancel()

	checks := hostHealth(ctx)
	checks = append(checks, s.configCheck())

	res := s.ListDevices(ctx)
	for _, err := range res.Errors {
		checks = append(checks, device.HealthCheck{
			Section:  device.SectionDevices,
			Name:     "discovery",
			Severity: device.SeverityWarn,
			Detail:   err.Error(),
		})
	}
	for _, d := range res.Devices {
		if d.Problem == "" {
			continue
		}
		checks = append(checks, device.HealthCheck{
			Section:  device.SectionDevices,
			Name:     d.ID,
			Severity: device.SeverityWarn,
			Detail:   string(d.Platform) + " device " + d.Problem,
		})
	}

	return DoctorReport{Checks: checks}
}

// configCheck reports whether the user's configuration loaded. A config
// that does not load never blocks anything — defaults apply — so it is a
// warning at worst.
func (s *Session) configCheck() device.HealthCheck {
	s.mu.RLock()
	err := s.cfgErr
	s.mu.RUnlock()

	c := device.HealthCheck{Section: device.SectionEnvironment, Name: doctorConfigName}
	path, perr := config.Path()
	switch {
	case err != nil:
		c.Severity, c.Detail, c.Fix = device.SeverityWarn, err.Error()+": using defaults", "fix or remove "+path
	case perr != nil:
		c.Severity, c.Detail = device.SeverityOK, "defaults"
	default:
		c.Severity, c.Detail = device.SeverityOK, path
		if _, statErr := os.Stat(path); statErr != nil {
			c.Detail = "defaults (no " + path + ")"
		}
	}
	return c
}

// StartAdbServer starts the adb server at the user's request.
func (s *Session) StartAdbServer(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, startAdbTimeout)
	defer cancel()
	return device.StartAdbServer(ctx)
}
