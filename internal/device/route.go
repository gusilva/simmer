package device

import "fmt"

// Routing a capability to a manager follows one of two rules, and which one
// applies is a property of the capability, not of the call site:
//
//   - pickCapability: the kinded manager wins, and a manager that declares
//     no Kind serves every kind of its platform. Most capabilities work this
//     way, because a simulator-only manager (iosManager) legitimately serves
//     physical requests it can answer identically.
//   - pickKinded: no fallback. Used where answering for the wrong kind would
//     be wrong rather than merely unsupported — reading an emulator's app
//     versions cannot stand in for a phone's.
//
// Both are written once here. Hand-rolling them per capability produced four
// subtly different rules across 29 copies, which is how `Boot` came to ignore
// Kind entirely while `ListApps` honoured it.

// pickCapability selects the Manager that satisfies capability T for a
// platform and kind, preferring an exact Kind match and falling back to a
// manager that serves every kind. what names the capability in the error.
func pickCapability[T any](managers []Manager, platform Platform, kind DeviceKind, what string) (T, error) {
	if c, ok := kindedMatch[T](managers, platform, kind); ok {
		return c, nil
	}
	for _, m := range managers {
		c, ok := m.(T)
		if !ok || !servesPlatform(m, platform) {
			continue
		}
		if _, isKinded := m.(KindedManager); isKinded {
			continue
		}
		return c, nil
	}

	var zero T
	return zero, fmt.Errorf("no %s registered for platform %s", what, platform)
}

// pickKinded selects the Manager that satisfies T for exactly this kind.
func pickKinded[T any](managers []Manager, platform Platform, kind DeviceKind, what string) (T, error) {
	if c, ok := kindedMatch[T](managers, platform, kind); ok {
		return c, nil
	}

	var zero T
	return zero, fmt.Errorf("no %s registered for %s %s", what, kindName(kind), platform)
}

// pickPlatform selects the Manager that satisfies T for a platform, ignoring
// kind. Used by the operations that are defined on a platform's tooling
// rather than on a device: creating a simulator, listing runtimes.
func pickPlatform[T any](managers []Manager, platform Platform, what string) (T, error) {
	for _, m := range managers {
		if c, ok := m.(T); ok && servesPlatform(m, platform) {
			return c, nil
		}
	}

	var zero T
	return zero, fmt.Errorf("no %s registered for platform %s", what, platform)
}

func kindedMatch[T any](managers []Manager, platform Platform, kind DeviceKind) (T, bool) {
	for _, m := range managers {
		c, ok := m.(T)
		if !ok || !servesPlatform(m, platform) {
			continue
		}
		if k, isKinded := m.(KindedManager); isKinded && k.Kind() == kind {
			return c, true
		}
	}

	var zero T
	return zero, false
}

func kindName(k DeviceKind) string {
	if k == KindPhysical {
		return "physical"
	}
	return "virtual"
}

// servesPlatform reports whether m handles the given platform. Every manager
// in the codebase reports its platform; one that does not is assumed not to
// match, so routing fails loudly rather than reaching the wrong device.
func servesPlatform(m Manager, platform Platform) bool {
	p, ok := m.(interface{ Platform() Platform })
	return ok && p.Platform() == platform
}
