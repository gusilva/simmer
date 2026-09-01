package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"simmer/internal/device"
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
	name := logFileName(app)
	path := filepath.Join(m.launchDir, name)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open app-log file %s: %w", name, err)
	}

	fmt.Fprintf(f, "# %s (%s) on %s — %s\n",
		app.Label(), app.BundleID, dev.Name, time.Now().Format(time.RFC3339))

	return f, nil
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
