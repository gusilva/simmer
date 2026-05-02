package device

import (
	"reflect"
	"testing"
)

func TestParseAVDs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "Single AVD",
			input:    "Pixel_6_API_33\n",
			expected: []string{"Pixel_6_API_33"},
		},
		{
			name:     "Multiple AVDs",
			input:    "Nexus_5X_API_29\nPixel_XL_API_30\n",
			expected: []string{"Nexus_5X_API_29", "Pixel_XL_API_30"},
		},
		{
			name:     "Empty input",
			input:    "",
			expected: []string{},
		},
		{
			name:     "Whitespace input",
			input:    "  \n  \n",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseAVDs([]byte(tt.input))
			if !reflect.DeepEqual(got, tt.expected) && !(len(got) == 0 && len(tt.expected) == 0) {
				t.Errorf("parseAVDs() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// func avdAPILevel(avdName string) string {
// 	home, err := os.UserHomeDir()
// 	if err != nil {
// 		return ""
// 	}
//
// 	path := filepath.Join(home, ".android", "avd", avdName+".ini")
// 	f, err := os.Open(path)
// 	if err != nil {
// 		return ""
// 	}
// 	defer f.Close()
//
// 	s := bufio.NewScanner(f)
// 	for s.Scan() {
// 		k, v, ok := strings.Cut(s.Text(), "=")
// 		if !ok {
// 			continue
// 		}
//
// 		if strings.TrimSpace(k) == "target" {
// 			return strings.TrimPrefix(strings.TrimSpace(v), "android-")
// 		}
// 	}
//
// 	return ""
// }

func TestAvdAPILevel(t *testing.T) {
	tests := []struct {
		name     string
		avdName  string
		expected string
	}{
		{
			name:     "Existing AVD",
			avdName:  "pixel_8",
			expected: "35",
		},
		{
			name:     "Non-existing AVD",
			avdName:  "NonExistingAVD",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := avdAPILevel(tt.avdName)
			if got != tt.expected {
				t.Errorf("avdAPILevel() = %v, want %v", got, tt.expected)
			}
		})
	}
}
