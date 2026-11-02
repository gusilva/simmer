package device

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/electricbubble/gadb"
)

// AdbServerState is what Simmer knows about the adb server, the host process
// every Android device is reached through.
type AdbServerState int

const (
	// AdbServerUnknown means no discovery has asked yet.
	AdbServerUnknown AdbServerState = iota
	// AdbServerRunning means the server accepted a connection.
	AdbServerRunning
	// AdbServerStopped means adb is installed but nothing is listening, so
	// every Android device is unreachable.
	AdbServerStopped
	// AdbNotInstalled means there is no adb binary on PATH.
	AdbNotInstalled
)

func (s AdbServerState) String() string {
	switch s {
	case AdbServerRunning:
		return "running"
	case AdbServerStopped:
		return "stopped"
	case AdbNotInstalled:
		return "not installed"
	default:
		return "unknown"
	}
}

// AdbServerReporter is implemented by the manager that owns the adb server's
// lifecycle questions. Discovery asks it alongside listing devices, so the
// answer is never older than the device list it explains.
type AdbServerReporter interface {
	AdbServer(ctx context.Context) AdbServerState
}

// ProbeAdbServer reports whether adb is installed and its server is
// listening. It never starts the server: `adb version` would not, but any
// other adb subcommand would, so the server is probed over its socket.
func ProbeAdbServer() AdbServerState {
	if _, err := exec.LookPath("adb"); err != nil {
		return AdbNotInstalled
	}
	if _, err := gadb.NewClient(); err != nil {
		return AdbServerStopped
	}
	return AdbServerRunning
}

// StartAdbServer starts the adb server. Simmer only does this when the user
// asks; a server stopped on purpose is never restarted behind their back.
func StartAdbServer(ctx context.Context) error {
	out, err := exec.CommandContext(ctx, "adb", "start-server").CombinedOutput()
	if err != nil {
		return fmt.Errorf("adb start-server: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Severity grades a health check.
type Severity string

const (
	// SeverityOK means nothing is wrong.
	SeverityOK Severity = "ok"
	// SeverityWarn means one capability is degraded.
	SeverityWarn Severity = "warn"
	// SeverityFail means a whole platform is unusable.
	SeverityFail Severity = "fail"
)

// Health check sections, in the order a report lists them.
const (
	SectionTools       = "tools"
	SectionServices    = "services"
	SectionEnvironment = "environment"
	SectionDevices     = "devices"
)

// HealthCheck is one fact about whether Simmer can do its job on this
// machine. Detail says what was found or, when not ok, what it breaks; Fix
// is the command or action that remedies it.
type HealthCheck struct {
	Section  string
	Name     string
	Severity Severity
	Detail   string
	Fix      string
}

// hostProbe is everything HostHealth asks of the machine, so the grading
// can be tested without the machine.
type hostProbe struct {
	lookPath   func(string) (string, error)
	version    func(ctx context.Context, tool string) string
	adbServer  func() AdbServerState
	xcodeDir   func(ctx context.Context) (string, error)
	androidSDK func() (string, bool)
}

func realHostProbe() hostProbe {
	return hostProbe{
		lookPath:  exec.LookPath,
		version:   toolVersion,
		adbServer: ProbeAdbServer,
		xcodeDir: func(ctx context.Context) (string, error) {
			out, err := exec.CommandContext(ctx, "xcode-select", "-p").Output()
			return strings.TrimSpace(string(out)), err
		},
		androidSDK: func() (string, bool) {
			p := androidSDKPath()
			info, err := os.Stat(p)
			return p, err == nil && info.IsDir()
		},
	}
}

// HostHealth checks the tools and services Simmer drives on this machine.
func HostHealth(ctx context.Context) []HealthCheck {
	return hostHealth(ctx, realHostProbe())
}

// toolSpec describes one host binary: how bad its absence is and why.
type toolSpec struct {
	name     string
	missing  Severity
	breaks   string
	fix      string
	versions bool
}

var hostTools = []toolSpec{
	{"xcrun", SeverityFail, "iOS simulators cannot be listed or driven", "xcode-select --install", true},
	{"xcodebuild", SeverityWarn, "iOS apps cannot be rebuilt", "install Xcode from the App Store", false},
	{"adb", SeverityFail, "no Android device can be reached", "install Android SDK platform-tools and add them to PATH", true},
	{"emulator", SeverityWarn, "Android emulators cannot be listed or booted", "add $ANDROID_HOME/emulator to PATH", false},
	{"avdmanager", SeverityWarn, "Android emulators cannot be created or deleted", "add $ANDROID_HOME/cmdline-tools/latest/bin to PATH", false},
	{"aapt", SeverityWarn, "APKs cannot be installed from a file", "add $ANDROID_HOME/build-tools/<version> to PATH", false},
	{"sqlite3", SeverityWarn, "app databases cannot be queried", "brew install sqlite", true},
}

func hostHealth(ctx context.Context, p hostProbe) []HealthCheck {
	var out []HealthCheck
	adbFound := false

	for _, t := range hostTools {
		path, err := p.lookPath(t.name)
		if err != nil {
			out = append(out, HealthCheck{SectionTools, t.name, t.missing, "not found: " + t.breaks, t.fix})
			continue
		}
		if t.name == "adb" {
			adbFound = true
		}
		detail := path
		if t.versions {
			if v := p.version(ctx, t.name); v != "" {
				detail = v + " · " + path
			}
		}
		out = append(out, HealthCheck{Section: SectionTools, Name: t.name, Severity: SeverityOK, Detail: detail})
	}

	if adbFound {
		out = append(out, adbServerCheck(p.adbServer()))
	}

	if dir, err := p.xcodeDir(ctx); err != nil || dir == "" {
		out = append(out, HealthCheck{SectionServices, "xcode developer dir", SeverityFail, "none selected: iOS simulators cannot be driven", "xcode-select --install"})
	} else {
		out = append(out, HealthCheck{Section: SectionServices, Name: "xcode developer dir", Severity: SeverityOK, Detail: dir})
	}

	if sdk, ok := p.androidSDK(); ok {
		out = append(out, HealthCheck{Section: SectionEnvironment, Name: "android sdk", Severity: SeverityOK, Detail: sdk})
	} else {
		out = append(out, HealthCheck{SectionEnvironment, "android sdk", SeverityWarn, "not found at " + sdk + ": emulators cannot be created", "set ANDROID_HOME or ANDROID_SDK_ROOT"})
	}

	return out
}

func adbServerCheck(s AdbServerState) HealthCheck {
	c := HealthCheck{Section: SectionServices, Name: "adb server"}
	switch s {
	case AdbServerRunning:
		c.Severity, c.Detail = SeverityOK, "running"
	default:
		c.Severity = SeverityFail
		c.Detail = "not running: Android devices are unreachable"
		c.Fix = "adb start-server"
	}
	return c
}

// toolVersion returns a short version string for the tools that report one
// cheaply, or "" when it cannot be read.
func toolVersion(ctx context.Context, tool string) string {
	switch tool {
	case "adb":
		_, v := adbVersion(ctx)
		return v
	case "xcrun":
		out, err := exec.CommandContext(ctx, "xcrun", "--version").Output()
		if err != nil {
			return ""
		}
		return strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(out)), "xcrun version")), ".")
	case "sqlite3":
		out, err := exec.CommandContext(ctx, "sqlite3", "--version").Output()
		if err != nil {
			return ""
		}
		if f := strings.Fields(string(out)); len(f) > 0 {
			return f[0]
		}
	}
	return ""
}
