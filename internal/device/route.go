package device

import "fmt"

// pickCapability selects the Manager that satisfies capability T for a given
// platform and device kind. A KindedManager whose Kind matches wins; a
// manager that declares no Kind is the fallback (it serves every kind of its
// platform). what names the capability in the error message.
func pickCapability[T any](managers []Manager, platform Platform, kind DeviceKind, what string) (T, error) {
	var zero T
	for _, m := range managers {
		c, ok := m.(T)
		if !ok || !servesPlatform(m, platform) {
			continue
		}
		k, isKinded := m.(KindedManager)
		if !isKinded || k.Kind() != kind {
			continue
		}
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
	return zero, fmt.Errorf("no %s registered for platform %s", what, platform)
}

// servesPlatform reports whether m handles the given platform. Every manager
// in the codebase reports its platform; one that does not is assumed not to
// match, so routing fails loudly rather than reaching the wrong device.
func servesPlatform(m Manager, platform Platform) bool {
	p, ok := m.(interface{ Platform() Platform })
	return ok && p.Platform() == platform
}
