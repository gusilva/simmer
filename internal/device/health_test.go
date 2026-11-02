package device

import (
	"context"
	"errors"
	"testing"
)

func fakeProbe() hostProbe {
	return hostProbe{
		lookPath:   func(name string) (string, error) { return "/bin/" + name, nil },
		version:    func(context.Context, string) string { return "" },
		adbServer:  func() AdbServerState { return AdbServerRunning },
		xcodeDir:   func(context.Context) (string, error) { return "/Applications/Xcode.app", nil },
		androidSDK: func() (string, bool) { return "/sdk", true },
	}
}

func findCheck(checks []HealthCheck, name string) (HealthCheck, bool) {
	for _, c := range checks {
		if c.Name == name {
			return c, true
		}
	}
	return HealthCheck{}, false
}

func TestHostHealth(t *testing.T) {
	missing := func(miss string) func(string) (string, error) {
		return func(name string) (string, error) {
			if name == miss {
				return "", errors.New("not found")
			}
			return "/bin/" + name, nil
		}
	}

	tests := []struct {
		name     string
		mutate   func(*hostProbe)
		check    string
		want     Severity
		wantFix  bool
		absent   string // a check that must not be reported
		noneFail bool
	}{
		{name: "healthy host", mutate: func(*hostProbe) {}, check: "adb server", want: SeverityOK, noneFail: true},
		{
			name:   "adb server stopped is a failure with a fix",
			mutate: func(p *hostProbe) { p.adbServer = func() AdbServerState { return AdbServerStopped } },
			check:  "adb server", want: SeverityFail, wantFix: true,
		},
		{
			name:   "adb missing fails and skips the server check",
			mutate: func(p *hostProbe) { p.lookPath = missing("adb") },
			check:  "adb", want: SeverityFail, wantFix: true, absent: "adb server",
		},
		{
			name:   "missing sqlite3 only degrades",
			mutate: func(p *hostProbe) { p.lookPath = missing("sqlite3") },
			check:  "sqlite3", want: SeverityWarn, wantFix: true,
		},
		{
			name:   "no xcode developer dir fails",
			mutate: func(p *hostProbe) { p.xcodeDir = func(context.Context) (string, error) { return "", errors.New("x") } },
			check:  "xcode developer dir", want: SeverityFail, wantFix: true,
		},
		{
			name:   "no android sdk warns",
			mutate: func(p *hostProbe) { p.androidSDK = func() (string, bool) { return "/nope", false } },
			check:  "android sdk", want: SeverityWarn, wantFix: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := fakeProbe()
			tt.mutate(&p)
			checks := hostHealth(context.Background(), p)

			c, ok := findCheck(checks, tt.check)
			if !ok {
				t.Fatalf("check %q not reported in %+v", tt.check, checks)
			}
			if c.Severity != tt.want {
				t.Errorf("%s severity = %s, want %s", tt.check, c.Severity, tt.want)
			}
			if tt.wantFix && c.Fix == "" {
				t.Errorf("%s has no fix", tt.check)
			}
			if tt.absent != "" {
				if _, ok := findCheck(checks, tt.absent); ok {
					t.Errorf("check %q should not be reported", tt.absent)
				}
			}
			if tt.noneFail {
				for _, c := range checks {
					if c.Severity != SeverityOK {
						t.Errorf("unexpected %s: %+v", c.Severity, c)
					}
				}
			}
		})
	}
}

func TestCoordinator_Discover_ReportsAdbServer(t *testing.T) {
	ios := &MockManager{PlatformVal: PlatformIOS} // cannot tell: AdbServerUnknown
	android := &MockManager{PlatformVal: PlatformAndroid, AdbServerVal: AdbServerStopped}

	res := NewCoordinator(ios, android).Discover(context.Background())
	if res.AdbServer != AdbServerStopped {
		t.Errorf("AdbServer = %v, want stopped", res.AdbServer)
	}
}
