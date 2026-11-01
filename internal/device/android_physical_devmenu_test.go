package device

import (
	"reflect"
	"testing"
)

func TestParseReversePorts(t *testing.T) {
	got := parseReversePorts("0c5168480410 tcp:8082 tcp:8082\n0c5168480410 tcp:1 localabstract:x\n\n")
	if want := []string{"8082"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestRemapsMenuKey(t *testing.T) {
	if !remapsMenuKey("Xiaomi\n") || remapsMenuKey("samsung") {
		t.Fatal("unexpected")
	}
}

func TestParseDebugHostPort(t *testing.T) {
	for in, want := range map[string]string{
		`<map><string name="debug_http_host">localhost:8083</string></map>`: "8083",
		`<map></map>`: "",
		`<string name="debug_http_host">nohost</string>`: "",
	} {
		if got := parseDebugHostPort(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}
