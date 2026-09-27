package device

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsReactNativeBundle(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  bool
	}{
		{
			name:  "empty bundle",
			setup: func(t *testing.T, dir string) {},
			want:  false,
		},
		{
			name: "main.jsbundle present",
			setup: func(t *testing.T, dir string) {
				touch(t, filepath.Join(dir, "main.jsbundle"))
			},
			want: true,
		},
		{
			name: "hermes.framework (legacy name)",
			setup: func(t *testing.T, dir string) {
				mkdir(t, filepath.Join(dir, "Frameworks", "hermes.framework"))
			},
			want: true,
		},
		{
			name: "hermesvm.framework (newer name)",
			setup: func(t *testing.T, dir string) {
				mkdir(t, filepath.Join(dir, "Frameworks", "hermesvm.framework"))
			},
			want: true,
		},
		{
			name: "React.framework, no Hermes",
			setup: func(t *testing.T, dir string) {
				mkdir(t, filepath.Join(dir, "Frameworks", "React.framework"))
			},
			want: true,
		},
		{
			name: "unrelated frameworks only",
			setup: func(t *testing.T, dir string) {
				mkdir(t, filepath.Join(dir, "Frameworks", "Alamofire.framework"))
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)
			if got := isReactNativeBundle(dir); got != tt.want {
				t.Errorf("isReactNativeBundle() = %v, want %v", got, tt.want)
			}
		})
	}

	if isReactNativeBundle("") {
		t.Error("isReactNativeBundle(\"\") = true, want false")
	}
}

func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, nil, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
