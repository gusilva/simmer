package device

import "testing"

func TestApp_IsLocal(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		want bool
	}{
		{"user app is local", "User", true},
		{"system app is not local", "System", false},
		{"empty type is not local", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := App{Type: tt.typ}
			if got := app.IsLocal(); got != tt.want {
				t.Errorf("IsLocal() = %v, want %v", got, tt.want)
			}
		})
	}
}
