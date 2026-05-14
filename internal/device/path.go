package device

import (
	"os"
	"path/filepath"
	"strings"
)

// expandPath expands ~ and environment variables in p.
func expandPath(p string) string {
	p = os.ExpandEnv(p)
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[2:])
		}
	}
	return p
}
