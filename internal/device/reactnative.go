package device

import (
	"os"
	"path/filepath"
	"strings"
)

// isReactNativeBundle reports whether the app bundle at path is a React
// Native app, by checking for the JS bundle or a React/Hermes framework it
// embeds. path must be a local, on-disk directory (simulator apps only —
// physical device paths are not locally accessible).
//
// The Frameworks dir listing (rather than a fixed "hermes.framework" name)
// is needed because the Hermes engine framework's name varies by RN
// version/build — e.g. "hermesvm.framework" on newer builds — where a fixed
// name silently misses the app.
func isReactNativeBundle(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, "main.jsbundle")); err == nil {
		return true
	}
	entries, err := os.ReadDir(filepath.Join(path, "Frameworks"))
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		if strings.Contains(name, "hermes") || name == "react.framework" {
			return true
		}
	}
	return false
}
