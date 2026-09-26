package device

import (
	"os"
	"path/filepath"
)

// isReactNativeBundle reports whether the app bundle at path is a React
// Native app, by checking for the JS bundle or the Hermes engine framework
// it embeds. path must be a local, on-disk directory (simulator apps only —
// physical device paths are not locally accessible).
func isReactNativeBundle(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(path, "main.jsbundle")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(path, "Frameworks", "hermes.framework")); err == nil {
		return true
	}
	return false
}
