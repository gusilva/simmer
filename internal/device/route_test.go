package device

import (
	"context"
	"testing"
)

// capManager is a Manager that can be given or denied a Kind, so routing
// precedence can be exercised without a device.
type capManager struct {
	platform Platform
	kind     DeviceKind
	id       string
}

func (m *capManager) ListDevices(context.Context) ([]Device, error) { return nil, nil }
func (m *capManager) Platform() Platform                            { return m.platform }

// kindedCap is the same manager with a Kind, so a test can hold one of each
// and check which the router prefers.
type kindedCap struct{ capManager }

func (m *kindedCap) Kind() DeviceKind { return m.kind }

func TestRoutingPrefersAnExactKindThenFallsBack(t *testing.T) {
	phone := &kindedCap{capManager{platform: PlatformIOS, kind: KindPhysical, id: "phone"}}
	anyKind := &capManager{platform: PlatformIOS, id: "any"}
	managers := []Manager{phone, anyKind}

	// The kinded manager wins for its own kind.
	got, err := pickCapability[interface{ Platform() Platform }](managers, PlatformIOS, KindPhysical, "cap")
	if err != nil || got.(*kindedCap).id != "phone" {
		t.Errorf("physical routed to %v (err %v), want the kinded manager", got, err)
	}

	// A manager that declares no kind serves the kinds nobody claims.
	got, err = pickCapability[interface{ Platform() Platform }](managers, PlatformIOS, KindVirtual, "cap")
	if err != nil || got.(*capManager).id != "any" {
		t.Errorf("virtual routed to %v (err %v), want the unkinded manager", got, err)
	}
}

func TestKindedRoutingRefusesToSubstitute(t *testing.T) {
	// Answering for the wrong kind would be wrong rather than unsupported:
	// an emulator's app versions cannot stand in for a phone's.
	anyKind := &capManager{platform: PlatformAndroid, id: "any"}
	emulator := &kindedCap{capManager{platform: PlatformAndroid, kind: KindVirtual, id: "emulator"}}
	managers := []Manager{anyKind, emulator}

	if _, err := pickKinded[interface{ Platform() Platform }](managers, PlatformAndroid, KindPhysical, "cap"); err == nil {
		t.Error("expected no match for a kind nobody serves")
	}
	got, err := pickKinded[interface{ Platform() Platform }](managers, PlatformAndroid, KindVirtual, "cap")
	if err != nil || got.(*kindedCap).id != "emulator" {
		t.Errorf("got %v (err %v), want the emulator manager", got, err)
	}
}

func TestPlatformRoutingIgnoresKind(t *testing.T) {
	// Creating a simulator is an operation on a platform's tooling, not on
	// a device, so it has no kind to match.
	sim := &kindedCap{capManager{platform: PlatformIOS, kind: KindVirtual, id: "sim"}}
	managers := []Manager{sim}

	got, err := pickPlatform[interface{ Platform() Platform }](managers, PlatformIOS, "creator")
	if err != nil || got.(*kindedCap).id != "sim" {
		t.Errorf("got %v (err %v), want the simulator manager", got, err)
	}
	if _, err := pickPlatform[interface{ Platform() Platform }](managers, PlatformAndroid, "creator"); err == nil {
		t.Error("expected an error for a platform nobody serves")
	}
}

func TestRoutingErrorsNameTheCapability(t *testing.T) {
	_, err := pickCapability[AppLister](nil, PlatformIOS, KindVirtual, "app lister")
	if err == nil || !contains(err.Error(), "app lister") {
		t.Errorf("error %v should name the capability", err)
	}
	_, err = pickKinded[AppLister](nil, PlatformAndroid, KindPhysical, "app lister")
	if err == nil || !contains(err.Error(), "physical") {
		t.Errorf("error %v should name the kind it could not serve", err)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0) }

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
