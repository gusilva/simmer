package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"simmer/internal/device"
	"simmer/internal/ui"

	tea "charm.land/bubbletea/v2"
)

// logFileName derives a filesystem-safe "<label>.log" name from an app.
// It uses App.Label() (CFBundleDisplayName → CFBundleName → bundle id),
// replaces path separators and other reserved characters with "_", and
// collapses whitespace runs to a single "_". Falls back to the bundle id
// when sanitising leaves nothing usable.
func logFileName(a device.App) string {
	base := sanitizeLogBase(a.Label())
	if base == "" {
		base = sanitizeLogBase(a.BundleID)
	}
	if base == "" {
		base = "app"
	}
	return base + ".log"
}

func sanitizeLogBase(s string) string {
	const reserved = `/\:*?"<>|`
	var b strings.Builder
	prevUnderscore := false
	for _, r := range strings.TrimSpace(s) {
		switch {
		case unicode.IsSpace(r) || strings.ContainsRune(reserved, r) || unicode.IsControl(r):
			if !prevUnderscore {
				b.WriteByte('_')
				prevUnderscore = true
			}
		default:
			b.WriteRune(r)
			prevUnderscore = false
		}
	}
	return strings.Trim(b.String(), "_")
}

// startAppLogging opens (truncating) the app-log file in the launch directory
// and writes a one-line header identifying the app, device, and start time.
// The caller owns the returned file and must eventually closeLogFile it.
func (m *model) startAppLogging(dev device.Device, app device.App) (*os.File, error) {
	path := filepath.Join(m.launchDir, logFileName(app))
	return m.startAppLoggingAtPath(dev, app, path)
}

// resolveLogPath returns the absolute log-file path for user-supplied input
// (the RN debugging menu's "log to file" prompt): an absolute path is used
// as-is, anything else — a bare filename or relative path — is resolved
// against the launch directory (the process's cwd at startup).
func (m *model) resolveLogPath(input string) string {
	if filepath.IsAbs(input) {
		return input
	}
	return filepath.Join(m.launchDir, input)
}

// startAppLoggingAtPath is startAppLogging with an explicit destination path,
// truncating any existing file there.
func (m *model) startAppLoggingAtPath(dev device.Device, app device.App, path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open app-log file %s: %w", filepath.Base(path), err)
	}

	fmt.Fprintf(f, "# %s (%s) on %s — %s\n",
		app.Label(), app.BundleID, dev.Name, time.Now().Format(time.RFC3339))

	return f, nil
}

// beginAppLogStream opens the log file at path and starts streaming dev/app's
// logs into it via the normal syslog/logcat StreamLogs path, replacing any
// previous session. Used by the "l"-key flow (ui.StartAppLoggingMsg) and, for
// every device kind except physical iOS, the RN debugging menu's "log to
// file" flow (rnLogRelaunchedMsg) — physical iOS uses the devicectl
// --console capture instead (see startConsoleLogCmd/attachAppLogStream).
func (m *model) beginAppLogStream(dev device.Device, app device.App, path string) tea.Cmd {
	stream, err := m.coordinator.StreamLogs(context.Background(), dev, app)
	if err != nil {
		m.errs = append(m.errs, err)
		return m.setStatus("app logging failed: "+errPreview(err), ui.StatusErr)
	}
	return m.attachAppLogStream(dev, app, path, stream)
}

// attachAppLogStream opens the log file at path and wires an already-started
// stream into it, replacing any previous session. Shared by beginAppLogStream
// (syslog/logcat-based streams) and the RN debugging menu's physical-iOS
// "log to file" flow (a devicectl --console stream).
func (m *model) attachAppLogStream(dev device.Device, app device.App, path string, stream *device.LogStream) tea.Cmd {
	m.teardownAppLogging()

	f, err := m.startAppLoggingAtPath(dev, app, path)
	if err != nil {
		stream.Stop()
		m.errs = append(m.errs, err)
		return m.setStatus("app logging failed: "+errPreview(err), ui.StatusErr)
	}
	m.logFile = f
	m.logStream = stream
	m.logBundleID = app.BundleID
	m.logDeviceID = dev.ID
	m.mainPane.SetLoggingBundle(app.BundleID)
	return tea.Batch(
		nextLogBatchCmd(stream, app.BundleID),
		m.setStatus("logging "+app.Label()+" → "+filepath.Base(f.Name()), ui.StatusOk),
	)
}

// closeLogFile flushes and closes m.logFile if set, then clears it. Safe to
// call multiple times and on a zero model.
func (m *model) closeLogFile() {
	if m == nil || m.logFile == nil {
		return
	}
	_ = m.logFile.Sync()
	_ = m.logFile.Close()
	m.logFile = nil
}

// teardownAppLogging stops any active log stream and closes the app-log file,
// clearing all associated tracking state. It does not emit a status message —
// callers add one where a user-visible reason applies.
func (m *model) teardownAppLogging() {
	if m.logStream != nil {
		m.logStream.Stop()
		m.logStream = nil
	}
	m.closeLogFile()
	m.logBundleID = ""
	m.logDeviceID = ""
	m.mainPane.SetLoggingBundle("")
}
