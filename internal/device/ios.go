package device

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"simmer/internal/logging"
)

type iosManager struct {
	logger *logging.Logger
}

// NewIOSManager returns a Manager implementation for Apple iOS simulators.
func NewIOSManager(logger *logging.Logger) Manager {
	return &iosManager{logger: logger}
}

type simctlList struct {
	Devices map[string][]simctlDevice `json:"devices"`
}

type simctlDevice struct {
	State        string `json:"state"`
	IsAvailable  bool   `json:"isAvailable"`
	Name         string `json:"name"`
	UDID         string `json:"udid"`
	DeviceTypeID string `json:"deviceTypeIdentifier"`
	DataPath     string `json:"dataPath"`
	LogPath      string `json:"logPath"`
}

// ToolVersion returns the xcrun version string (e.g. "64").
// Output of `xcrun --version` is "xcrun version 64.\n".
func (m *iosManager) ToolVersion(ctx context.Context) (Platform, string) {
	out, err := exec.CommandContext(ctx, "xcrun", "--version").Output()
	if err != nil {
		return PlatformIOS, "n/a"
	}
	s := strings.TrimSuffix(strings.TrimSpace(string(out)), ".")
	parts := strings.Fields(s)
	if len(parts) > 0 {
		return PlatformIOS, parts[len(parts)-1]
	}
	return PlatformIOS, s
}

// Platform reports the platform this manager handles.
func (m *iosManager) Platform() Platform { return PlatformIOS }

// Boot starts the simulator with the given UDID via `xcrun simctl boot`.
// simctl returns immediately after the boot is initiated; the caller can
// re-list devices to observe the resulting state transition.
func (m *iosManager) Boot(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "boot", id)
	out, err := cmd.CombinedOutput()
	m.logger.LogExec("xcrun", []string{"simctl", "boot", id}, string(out), err)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("xcrun simctl boot %s: %w", id, err)
		}
		return fmt.Errorf("xcrun simctl boot %s: %w: %s", id, err, msg)
	}
	return nil
}

// Shutdown stops the simulator with the given UDID via `xcrun simctl shutdown`.
func (m *iosManager) Shutdown(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "shutdown", id)
	out, err := cmd.CombinedOutput()
	m.logger.LogExec("xcrun", []string{"simctl", "shutdown", id}, string(out), err)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("xcrun simctl shutdown %s: %w", id, err)
		}
		return fmt.Errorf("xcrun simctl shutdown %s: %w: %s", id, err, msg)
	}
	return nil
}

// ListApps runs `xcrun simctl listapps <UDID>` and converts the resulting
// plist to JSON via `plutil` so it can be parsed without an extra dep.
// Returns apps sorted alphabetically by display label.
func (m *iosManager) ListApps(ctx context.Context, id string) ([]App, error) {
	listCmd := exec.CommandContext(ctx, "xcrun", "simctl", "listapps", id)
	listOut, err := listCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("xcrun simctl listapps %s: %w", id, err)
	}

	plutilCmd := exec.CommandContext(ctx, "plutil", "-convert", "json", "-o", "-", "-")
	plutilCmd.Stdin = bytes.NewReader(listOut)
	jsonOut, err := plutilCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("plutil convert json: %w", err)
	}

	var raw map[string]map[string]any
	if err := json.Unmarshal(jsonOut, &raw); err != nil {
		return nil, fmt.Errorf("parse listapps json: %w", err)
	}

	apps := make([]App, 0, len(raw))
	for bundleID, info := range raw {
		app := App{BundleID: bundleID}
		if v, ok := info["CFBundleDisplayName"].(string); ok {
			app.DisplayName = v
		}
		if v, ok := info["CFBundleName"].(string); ok {
			app.Name = v
		}
		if v, ok := info["CFBundleVersion"].(string); ok {
			app.Version = v
		}
		if v, ok := info["CFBundleShortVersionString"].(string); ok {
			app.ShortVersion = v
		}
		if v, ok := info["ApplicationType"].(string); ok {
			app.Type = v
		}
		if v, ok := info["Path"].(string); ok {
			app.Path = v
		}
		apps = append(apps, app)
	}

	sort.SliceStable(apps, func(i, j int) bool {
		a, b := apps[i].Label(), apps[j].Label()
		if a == b {
			return apps[i].BundleID < apps[j].BundleID
		}
		return strings.ToLower(a) < strings.ToLower(b)
	})
	return apps, nil
}

// StreamLogs streams the unified log entries emitted by the given app. It
// best-effort-launches the app first (no error if already running, since
// modern iOS apps log via os_log/NSLog into the unified log rather than
// stdout) then runs `simctl spawn <UDID> log stream` filtered by a predicate
// covering process name, subsystem, and sender image path.
func (m *iosManager) StreamLogs(parent context.Context, dev Device, app App) (*LogStream, error) {
	ctx, cancel := context.WithCancel(parent)

	// Make sure the app is running so it can emit logs. simctl launch returns
	// quickly; we don't care if it errors (e.g. "already running").
	launchCtx, launchCancel := context.WithTimeout(ctx, 5*time.Second)
	_ = exec.CommandContext(launchCtx, "xcrun", "simctl", "launch", dev.ID, app.BundleID).Run()
	launchCancel()

	procName := app.Name
	if procName == "" {
		if i := strings.LastIndex(app.BundleID, "."); i >= 0 && i < len(app.BundleID)-1 {
			procName = app.BundleID[i+1:]
		} else {
			procName = app.BundleID
		}
	}
	predicate := fmt.Sprintf(
		`(process == %q) OR (subsystem CONTAINS[cd] %q) OR (senderImagePath CONTAINS[cd] %q)`,
		procName, app.BundleID, app.BundleID,
	)

	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "spawn", dev.ID,
		"log", "stream",
		"--level=debug",
		"--style=compact",
		"--predicate", predicate,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("xcrun simctl spawn log stream %s: %w", dev.ID, err)
	}

	lines := make(chan string, 256)
	done := make(chan error, 1)

	scan := func(r io.Reader, wg *sync.WaitGroup) {
		defer wg.Done()
		s := bufio.NewScanner(r)
		s.Buffer(make([]byte, 64*1024), 1024*1024)
		for s.Scan() {
			select {
			case lines <- s.Text():
			case <-ctx.Done():
				return
			}
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go scan(stdout, &wg)
	go scan(stderr, &wg)

	go func() {
		wg.Wait()
		close(lines)
		done <- cmd.Wait()
		close(done)
	}()

	return &LogStream{
		Lines: lines,
		Done:  done,
		Stop:  cancel,
	}, nil
}

// Info gathers details about a single iOS simulator from the local
// CoreSimulator data directory plus, for booted devices, sw_vers via
// `simctl spawn`. Best-effort: missing data is reported as "—".
func (m *iosManager) Info(ctx context.Context, dev Device) (DeviceInfo, error) {
	info := DeviceInfo{}
	add := func(k, v string) {
		if v == "" {
			v = "—"
		}
		info.Fields = append(info.Fields, InfoField{Key: k, Value: v})
	}

	add("Name", dev.Name)
	add("UDID", dev.ID)
	add("Platform", string(dev.Platform))
	add("Status", string(dev.Status))
	add("Runtime", dev.Version)

	home, err := os.UserHomeDir()
	if err != nil {
		return info, nil
	}
	deviceDir := filepath.Join(home, "Library", "Developer", "CoreSimulator", "Devices", dev.ID)

	if pl, err := readDevicePlist(ctx, filepath.Join(deviceDir, "device.plist")); err == nil {
		if v, ok := pl["deviceType"].(string); ok {
			add("Device Type", trimDevicePrefix(v))
		}
		if v, ok := pl["runtime"].(string); ok {
			add("Runtime ID", trimDevicePrefix(v))
		}
		if v, ok := pl["state"].(float64); ok {
			add("State", simctlStateName(int(v)))
		}
		if v, ok := pl["lastBootedAt"].(string); ok {
			add("Last Booted", v)
		}
	}

	add("Data Path", filepath.Join(deviceDir, "data"))
	if size, err := dirSizeKB(ctx, filepath.Join(deviceDir, "data")); err == nil {
		add("Data Size", formatBytes(size*1024))
	}

	if dev.Status == StatusRunning {
		if out, err := exec.CommandContext(ctx, "xcrun", "simctl", "spawn", dev.ID, "sw_vers").Output(); err == nil {
			for line := range strings.SplitSeq(string(out), "\n") {
				k, v, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				k = strings.TrimSpace(k)
				v = strings.TrimSpace(v)
				switch k {
				case "ProductName":
					add("OS Name", v)
				case "ProductVersion":
					add("OS Version", v)
				case "BuildVersion":
					add("OS Build", v)
				}
			}
		}
		if out, err := exec.CommandContext(ctx, "xcrun", "simctl", "spawn", dev.ID, "uname", "-m").Output(); err == nil {
			add("Architecture", strings.TrimSpace(string(out)))
		}
	}

	return info, nil
}

// readDevicePlist converts a binary/XML plist on disk to JSON via plutil and
// returns it as a generic map.
func readDevicePlist(ctx context.Context, path string) (map[string]any, error) {
	out, err := exec.CommandContext(ctx, "plutil", "-convert", "json", "-o", "-", path).Output()
	if err != nil {
		return nil, fmt.Errorf("plutil %s: %w", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return m, nil
}

// dirSizeKB invokes `du -sk` and returns the directory's apparent size in KB.
func dirSizeKB(ctx context.Context, path string) (int64, error) {
	out, err := exec.CommandContext(ctx, "du", "-sk", path).Output()
	if err != nil {
		return 0, fmt.Errorf("du -sk %s: %w", path, err)
	}
	parts := strings.Fields(string(out))
	if len(parts) < 1 {
		return 0, fmt.Errorf("unexpected du output: %q", string(out))
	}
	kb, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse du size %q: %w", parts[0], err)
	}
	return kb, nil
}

// trimDevicePrefix strips the well-known SimDeviceType / SimRuntime DNS-style
// prefix from a CoreSimulator identifier.
func trimDevicePrefix(s string) string {
	for _, p := range []string{
		"com.apple.CoreSimulator.SimDeviceType.",
		"com.apple.CoreSimulator.SimRuntime.",
	} {
		if strings.HasPrefix(s, p) {
			return s[len(p):]
		}
	}
	return s
}

// simctlStateName maps the integer state field in device.plist to a label.
func simctlStateName(s int) string {
	switch s {
	case 0:
		return "Creating"
	case 1:
		return "Shutdown"
	case 2:
		return "Booting"
	case 3:
		return "Booted"
	case 4:
		return "Shutting Down"
	default:
		return fmt.Sprintf("State(%d)", s)
	}
}

type simctlDeviceTypesList struct {
	DeviceTypes []struct {
		Name       string `json:"name"`
		Identifier string `json:"identifier"`
	} `json:"devicetypes"`
}

type simctlRuntimesList struct {
	Runtimes []struct {
		Name        string `json:"name"`
		Identifier  string `json:"identifier"`
		Version     string `json:"version"`
		IsAvailable bool   `json:"isAvailable"`
	} `json:"runtimes"`
}

func (m *iosManager) Create(ctx context.Context, name, deviceTypeID, runtimeID string) (string, error) {
	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "create", name, deviceTypeID, runtimeID)
	out, err := cmd.CombinedOutput()
	m.logger.LogExec("xcrun", []string{"simctl", "create", name, deviceTypeID, runtimeID}, string(out), err)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return "", fmt.Errorf("xcrun simctl create: %w", err)
		}
		return "", fmt.Errorf("xcrun simctl create: %w: %s", err, msg)
	}
	return strings.TrimSpace(string(out)), nil
}

// InstallApp builds a .xcworkspace/.xcodeproj with xcodebuild (or installs a
// pre-built .app/.ipa directly) and installs the result onto the simulator.
func (m *iosManager) InstallApp(ctx context.Context, deviceID, path, scheme string) error {
	p := expandPath(path)
	lower := strings.ToLower(p)

	if strings.HasSuffix(lower, ".app") || strings.HasSuffix(lower, ".ipa") {
		return simctlInstall(ctx, m.logger, deviceID, p)
	}

	derivedData, err := os.MkdirTemp("", "simmer-build-*")
	if err != nil {
		return fmt.Errorf("create build dir: %w", err)
	}
	defer os.RemoveAll(derivedData)

	appPath, err := xcodeBuild(ctx, m.logger, p, scheme, derivedData)
	if err != nil {
		return err
	}
	return simctlInstall(ctx, m.logger, deviceID, appPath)
}

func simctlInstall(ctx context.Context, logger *logging.Logger, deviceID, appPath string) error {
	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "install", deviceID, appPath)
	out, err := cmd.CombinedOutput()
	logger.LogExec("xcrun", []string{"simctl", "install", deviceID, appPath}, string(out), err)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("xcrun simctl install: %w", err)
		}
		return fmt.Errorf("xcrun simctl install: %w: %s", err, msg)
	}
	return nil
}

func xcodeBuild(ctx context.Context, logger *logging.Logger, projectPath, scheme, derivedData string) (string, error) {
	lower := strings.ToLower(projectPath)
	projectFlag := "-project"
	if strings.HasSuffix(lower, ".xcworkspace") {
		projectFlag = "-workspace"
	}
	args := []string{
		projectFlag, projectPath,
		"-scheme", scheme,
		"-configuration", "Debug",
		"-sdk", "iphonesimulator",
		"-derivedDataPath", derivedData,
		"build",
	}
	cmd := exec.CommandContext(ctx, "xcodebuild", args...)
	out, err := cmd.CombinedOutput()
	logger.LogExec("xcodebuild", args, string(out), err)
	if err != nil {
		return "", fmt.Errorf("xcodebuild: %w: %s", err, trimBuildOutput(string(out)))
	}
	return findBuiltApp(derivedData)
}

func findBuiltApp(derivedData string) (string, error) {
	productsDir := filepath.Join(derivedData, "Build", "Products")
	var found string
	_ = filepath.Walk(productsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || found != "" {
			return err
		}
		if info.IsDir() && strings.HasSuffix(path, ".app") {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("no .app found in build products (%s)", productsDir)
	}
	return found, nil
}

func trimBuildOutput(output string) string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var errLines []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.Contains(l, "error:") || l == "BUILD FAILED" {
			errLines = append(errLines, l)
		}
	}
	if len(errLines) > 0 {
		if len(errLines) > 3 {
			errLines = errLines[len(errLines)-3:]
		}
		return strings.Join(errLines, "; ")
	}
	if len(lines) > 3 {
		lines = lines[len(lines)-3:]
	}
	return strings.Join(lines, "; ")
}

// ListXcodeSchemes returns the scheme names for a .xcworkspace or .xcodeproj.
// It scans for shared .xcscheme files on disk (no subprocess, works even when
// xcodebuild can't open the workspace) and falls back to xcodebuild -list only
// when no scheme files are found that way.
func ListXcodeSchemes(ctx context.Context, projectPath string) ([]string, error) {
	p := expandPath(projectPath)

	if _, err := os.Stat(p); err != nil {
		return nil, fmt.Errorf("path not found: %s", p)
	}

	lower := strings.ToLower(p)
	if !strings.HasSuffix(lower, ".xcworkspace") && !strings.HasSuffix(lower, ".xcodeproj") {
		return nil, fmt.Errorf("expected .xcworkspace or .xcodeproj, got: %s", projectPath)
	}

	schemes := scanXcschemeFiles(p)
	if len(schemes) > 0 {
		return schemes, nil
	}

	return listSchemesViaXcodebuild(ctx, p)
}

// scanXcschemeFiles finds shared .xcscheme files without invoking xcodebuild.
// For a workspace it also follows project references in contents.xcworkspacedata.
func scanXcschemeFiles(projectPath string) []string {
	lower := strings.ToLower(projectPath)
	var dirs []string

	if strings.HasSuffix(lower, ".xcworkspace") {
		dirs = append(dirs, filepath.Join(projectPath, "xcshareddata", "xcschemes"))
		if projects, err := workspaceProjectPaths(projectPath); err == nil {
			base := filepath.Dir(projectPath)
			for _, rel := range projects {
				dirs = append(dirs, filepath.Join(base, rel, "xcshareddata", "xcschemes"))
			}
		}
	} else {
		dirs = append(dirs, filepath.Join(projectPath, "xcshareddata", "xcschemes"))
	}

	seen := map[string]bool{}
	var schemes []string
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".xcscheme") {
				continue
			}
			name := strings.TrimSuffix(e.Name(), ".xcscheme")
			if !seen[name] {
				seen[name] = true
				schemes = append(schemes, name)
			}
		}
	}
	sort.Strings(schemes)
	return schemes
}

// workspaceProjectPaths parses contents.xcworkspacedata and returns the
// relative .xcodeproj paths referenced by the workspace.
func workspaceProjectPaths(workspacePath string) ([]string, error) {
	data, err := os.ReadFile(filepath.Join(workspacePath, "contents.xcworkspacedata"))
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "location") || !strings.Contains(line, ".xcodeproj") {
			continue
		}
		start := strings.IndexByte(line, '"')
		end := strings.LastIndexByte(line, '"')
		if start < 0 || end <= start {
			continue
		}
		loc := line[start+1 : end]
		if i := strings.IndexByte(loc, ':'); i >= 0 {
			loc = loc[i+1:]
		}
		if strings.HasSuffix(loc, ".xcodeproj") {
			paths = append(paths, loc)
		}
	}
	return paths, nil
}

// listSchemesViaXcodebuild is the fallback when no .xcscheme files are found.
func listSchemesViaXcodebuild(ctx context.Context, projectPath string) ([]string, error) {
	lower := strings.ToLower(projectPath)
	flag := "-project"
	if strings.HasSuffix(lower, ".xcworkspace") {
		flag = "-workspace"
	}
	cmd := exec.CommandContext(ctx, "xcodebuild", "-list", flag, projectPath, "-json")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			return nil, fmt.Errorf("xcodebuild -list: %w", err)
		}
		return nil, fmt.Errorf("xcodebuild -list: %s", msg)
	}
	return parseXcodeSchemes(out)
}

func parseXcodeSchemes(data []byte) ([]string, error) {
	var v struct {
		Workspace struct {
			Schemes []string `json:"schemes"`
		} `json:"workspace"`
		Project struct {
			Schemes []string `json:"schemes"`
		} `json:"project"`
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("parse schemes: %w", err)
	}
	schemes := v.Workspace.Schemes
	if len(schemes) == 0 {
		schemes = v.Project.Schemes
	}
	if len(schemes) == 0 {
		return nil, fmt.Errorf("no schemes found in project")
	}
	return schemes, nil
}

// DeleteApp uninstalls an app from an iOS simulator via `xcrun simctl uninstall`.
// TerminateApp stops a running app via `xcrun simctl terminate`. Best-effort:
// callers typically ignore the error since the app may not be running.
func (m *iosManager) TerminateApp(ctx context.Context, deviceID, bundleID string) error {
	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "terminate", deviceID, bundleID)
	out, err := cmd.CombinedOutput()
	m.logger.LogExec("xcrun", []string{"simctl", "terminate", deviceID, bundleID}, string(out), err)
	return err
}

// LaunchApp starts an already-installed app via `xcrun simctl launch`.
func (m *iosManager) LaunchApp(ctx context.Context, deviceID, bundleID string) error {
	err := simctlLaunch(ctx, deviceID, bundleID)
	m.logger.LogExec("xcrun", []string{"simctl", "launch", deviceID, bundleID}, "", err)
	return err
}

// ResolveIOSProjectPath finds the Xcode project/workspace for a locally
// built app by matching its display name against DerivedData folder names,
// then reading WorkspacePath from that folder's info.plist. See
// docs/adr/0001-project-resolution-not-persisted.md.
func ResolveIOSProjectPath(appName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	root := filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData")
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", fmt.Errorf("read DerivedData: %w", err)
	}

	prefix := appName + "-"
	var best string
	var bestMod time.Time
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), prefix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if best == "" || info.ModTime().After(bestMod) {
			best = filepath.Join(root, e.Name())
			bestMod = info.ModTime()
		}
	}
	if best == "" {
		return "", fmt.Errorf("no DerivedData folder found for %q", appName)
	}

	// info.plist contains a Date field (LastAccessedDate), which `plutil
	// -convert json` rejects ("Invalid object in plist for JSON format").
	// -p's human-readable dump handles it fine.
	out, err := exec.Command("plutil", "-p", filepath.Join(best, "info.plist")).Output()
	if err != nil {
		return "", fmt.Errorf("read %s/info.plist: %w", filepath.Base(best), err)
	}
	workspacePath, ok := parsePlutilWorkspacePath(string(out))
	if !ok {
		return "", fmt.Errorf("WorkspacePath not found in %s/info.plist", filepath.Base(best))
	}
	if _, err := os.Stat(workspacePath); err != nil {
		return "", fmt.Errorf("project no longer exists at %s", workspacePath)
	}
	return workspacePath, nil
}

// parsePlutilWorkspacePath extracts the WorkspacePath value from `plutil -p`
// output, e.g. `  "WorkspacePath" => "/path/to/Foo.xcodeproj"`.
func parsePlutilWorkspacePath(output string) (string, bool) {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, `"WorkspacePath"`) {
			continue
		}
		idx := strings.Index(line, "=>")
		if idx < 0 {
			continue
		}
		val := strings.Trim(strings.TrimSpace(line[idx+2:]), `"`)
		if val != "" {
			return val, true
		}
	}
	return "", false
}

func (m *iosManager) DeleteApp(ctx context.Context, deviceID, bundleID string) error {
	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "uninstall", deviceID, bundleID)
	out, err := cmd.CombinedOutput()
	m.logger.LogExec("xcrun", []string{"simctl", "uninstall", deviceID, bundleID}, string(out), err)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("xcrun simctl uninstall %s %s: %w", deviceID, bundleID, err)
		}
		return fmt.Errorf("xcrun simctl uninstall %s %s: %w: %s", deviceID, bundleID, err, msg)
	}
	return nil
}

// StartInstall runs build → install → launch in a background goroutine and
// streams progress events. The pipeline runs entirely outside the caller's
// context; call BuildStream.Stop to cancel it.
func (m *iosManager) StartInstall(dev Device, path, scheme string) (*BuildStream, error) {
	events := make(chan string, 64)
	done := make(chan error, 1)
	ctx, cancel := newBuildContext()

	go func() {
		defer close(events)
		defer cancel()

		p := expandPath(path)
		lower := strings.ToLower(p)

		var appPath string

		if strings.HasSuffix(lower, ".app") || strings.HasSuffix(lower, ".ipa") {
			appPath = p
		} else {
			derivedData, err := os.MkdirTemp("", "simmer-build-*")
			if err != nil {
				done <- fmt.Errorf("create build dir: %w", err)
				return
			}
			defer os.RemoveAll(derivedData)

			sendEvent(ctx, events, "Building "+scheme+"…")
			built, err := xcodeBuildStream(ctx, m.logger, events, p, scheme, derivedData)
			if err != nil {
				done <- err
				return
			}
			appPath = built
		}

		sendEvent(ctx, events, "Installing…")
		if err := simctlInstall(ctx, m.logger, dev.ID, appPath); err != nil {
			done <- err
			return
		}

		bundleID, err := readBundleID(appPath)
		if err != nil {
			// install succeeded; launch is best-effort
			done <- nil
			return
		}

		sendEvent(ctx, events, "Launching…")
		_ = simctlLaunch(ctx, dev.ID, bundleID)
		done <- nil
	}()

	return &BuildStream{Events: events, Done: done, Stop: cancel}, nil
}

// xcodeBuildStream runs xcodebuild and forwards filtered output to events.
func xcodeBuildStream(ctx context.Context, logger *logging.Logger, events chan<- string, projectPath, scheme, derivedData string) (string, error) {
	lower := strings.ToLower(projectPath)
	projectFlag := "-project"
	if strings.HasSuffix(lower, ".xcworkspace") {
		projectFlag = "-workspace"
	}
	args := []string{
		projectFlag, projectPath,
		"-scheme", scheme,
		"-configuration", "Debug",
		"-sdk", "iphonesimulator",
		"-derivedDataPath", derivedData,
		"build",
	}
	cmd := exec.CommandContext(ctx, "xcodebuild", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("xcodebuild stdout pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		logger.LogStart("xcodebuild", args, err)
		return "", fmt.Errorf("xcodebuild start: %w", err)
	}
	logger.LogStart("xcodebuild", args, nil)

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if line := filterBuildLine(scanner.Text()); line != "" {
			sendEvent(ctx, events, line)
		}
	}

	if err := cmd.Wait(); err != nil {
		logger.LogExec("xcodebuild", args, stderrBuf.String(), err)
		msg := trimBuildOutput(stderrBuf.String())
		if msg == "" {
			return "", fmt.Errorf("xcodebuild: %w", err)
		}
		return "", fmt.Errorf("xcodebuild: %w: %s", err, msg)
	}
	return findBuiltApp(derivedData)
}

// filterBuildLine returns a short human-readable summary for interesting
// xcodebuild output lines, or "" to suppress the line.
func filterBuildLine(line string) string {
	line = strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(line, "CompileSwift "),
		strings.HasPrefix(line, "CompileC "),
		strings.HasPrefix(line, "Compile "):
		// Extract just the source filename
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			return "Compiling " + filepath.Base(parts[len(parts)-1])
		}
	case strings.HasPrefix(line, "Ld "):
		return "Linking…"
	case strings.HasPrefix(line, "CodeSign "):
		return "Signing…"
	case line == "BUILD SUCCEEDED":
		return "Build succeeded"
	case strings.Contains(line, "error:"):
		return line
	}
	return ""
}

// readBundleID reads CFBundleIdentifier from an .app bundle's Info.plist via plutil.
func readBundleID(appPath string) (string, error) {
	plist := filepath.Join(appPath, "Info.plist")
	out, err := exec.Command("plutil", "-convert", "json", "-o", "-", plist).Output()
	if err != nil {
		return "", fmt.Errorf("read Info.plist: %w", err)
	}
	var info struct {
		BundleID string `json:"CFBundleIdentifier"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		return "", fmt.Errorf("parse Info.plist: %w", err)
	}
	if info.BundleID == "" {
		return "", fmt.Errorf("CFBundleIdentifier not found in Info.plist")
	}
	return info.BundleID, nil
}

// simctlLaunch launches an app in a simulator via `xcrun simctl launch`.
func simctlLaunch(ctx context.Context, deviceID, bundleID string) error {
	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "launch", deviceID, bundleID)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("xcrun simctl launch: %w", err)
		}
		return fmt.Errorf("xcrun simctl launch: %w: %s", err, msg)
	}
	return nil
}

func (m *iosManager) Delete(ctx context.Context, id string) error {
	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "delete", id)
	out, err := cmd.CombinedOutput()
	m.logger.LogExec("xcrun", []string{"simctl", "delete", id}, string(out), err)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return fmt.Errorf("xcrun simctl delete %s: %w", id, err)
		}
		return fmt.Errorf("xcrun simctl delete %s: %w: %s", id, err, msg)
	}
	return nil
}

func (m *iosManager) ListDeviceTypes(ctx context.Context) ([]DeviceType, error) {
	out, err := exec.CommandContext(ctx, "xcrun", "simctl", "list", "devicetypes", "--json").Output()
	m.logger.LogExec("xcrun", []string{"simctl", "list", "devicetypes", "--json"}, string(out), err)
	if err != nil {
		return nil, fmt.Errorf("xcrun simctl list devicetypes: %w", err)
	}
	var list simctlDeviceTypesList
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("parse devicetypes: %w", err)
	}
	types := make([]DeviceType, 0, len(list.DeviceTypes))
	for _, dt := range list.DeviceTypes {
		types = append(types, DeviceType{Name: dt.Name, Identifier: dt.Identifier})
	}
	return types, nil
}

func (m *iosManager) ListRuntimes(ctx context.Context) ([]Runtime, error) {
	out, err := exec.CommandContext(ctx, "xcrun", "simctl", "list", "runtimes", "--json").Output()
	m.logger.LogExec("xcrun", []string{"simctl", "list", "runtimes", "--json"}, string(out), err)
	if err != nil {
		return nil, fmt.Errorf("xcrun simctl list runtimes: %w", err)
	}
	var list simctlRuntimesList
	if err := json.Unmarshal(out, &list); err != nil {
		return nil, fmt.Errorf("parse runtimes: %w", err)
	}
	runtimes := make([]Runtime, 0, len(list.Runtimes))
	for _, r := range list.Runtimes {
		runtimes = append(runtimes, Runtime{
			Name:        r.Name,
			Identifier:  r.Identifier,
			Version:     r.Version,
			IsAvailable: r.IsAvailable,
		})
	}
	return runtimes, nil
}

func (m *iosManager) ListDevices(ctx context.Context) ([]Device, error) {
	cmd := exec.CommandContext(ctx, "xcrun", "simctl", "list", "devices", "available", "--json")
	output, err := cmd.Output()
	m.logger.LogExec("xcrun", []string{"simctl", "list", "devices", "available", "--json"}, string(output), err)
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
		version := parseRuntimeVersion(runtime)
		for _, d := range runtimeDevices {
			status := StatusOff
			if d.State == "Booted" {
				status = StatusRunning
			}

			devices = append(devices, Device{
				ID:       d.UDID,
				Name:     d.Name,
				Platform: PlatformIOS,
				Version:  version,
				Status:   status,
			})
		}
	}

	return devices, nil
}

// parseRuntimeVersion extracts a human-friendly version string from a simctl
// runtime identifier. Examples:
//
//	"com.apple.CoreSimulator.SimRuntime.iOS-18-5"     -> "18.5"
//	"com.apple.CoreSimulator.SimRuntime.watchOS-12-0" -> "12.0"
//	"iOS 17.0"                                        -> "iOS 17.0" (passthrough)
func parseRuntimeVersion(id string) string {
	last := id
	if i := strings.LastIndex(id, "."); i >= 0 {
		last = id[i+1:]
	}
	if _, after, ok := strings.Cut(last, "-"); ok {
		return strings.ReplaceAll(after, "-", ".")
	}
	return last
}
