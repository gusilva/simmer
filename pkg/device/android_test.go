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
