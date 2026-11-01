package device

import (
	"reflect"
	"testing"
)

func TestAndroidReverseArgs(t *testing.T) {
	got := androidReverseArgs("ABC123", "8081")
	want := []string{"-s", "ABC123", "reverse", "tcp:8081", "tcp:8081"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
