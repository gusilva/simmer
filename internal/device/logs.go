package device

import (
	"context"
	"fmt"
)

// LogStream is a live stream of log lines from a launched app.
//
// Consumers receive lines from Lines until that channel is closed; after
// closure, exactly one error (possibly nil) is delivered on Done. Calling
// Stop cancels the underlying process; Stop is safe to call multiple times.
type LogStream struct {
	Lines <-chan string
	Done  <-chan error
	Stop  func()
}

// LogStreamer is an optional interface a Manager may implement to stream an
// app's log output. The full App is passed so implementations can build
// predicates from BundleID, Name, Path, etc. as needed.
type LogStreamer interface {
	Platform() Platform
	StreamLogs(ctx context.Context, dev Device, app App) (*LogStream, error)
}

// StreamLogs routes to the Manager that implements LogStreamer for the
// device's platform, preferring a KindedManager whose Kind matches dev.Kind.
// The returned stream's lifetime is independent of ctx once started; callers
// must invoke Stream.Stop to free resources.
func (c *Coordinator) StreamLogs(ctx context.Context, dev Device, app App) (*LogStream, error) {
	for _, m := range c.Managers {
		s, ok := m.(LogStreamer)
		if !ok || s.Platform() != dev.Platform {
			continue
		}
		k, isKinded := m.(KindedManager)
		if !isKinded || k.Kind() != dev.Kind {
			continue
		}
		return s.StreamLogs(ctx, dev, app)
	}
	for _, m := range c.Managers {
		s, ok := m.(LogStreamer)
		if !ok || s.Platform() != dev.Platform {
			continue
		}
		if _, isKinded := m.(KindedManager); isKinded {
			continue
		}
		return s.StreamLogs(ctx, dev, app)
	}
	return nil, fmt.Errorf("no log streamer registered for platform %s", dev.Platform)
}

// ConsoleLogStreamer is an optional interface a Manager may implement to
// stream a freshly (re)launched app's own stdout/stderr, as opposed to
// syslog. Only physicalIOSManager implements this: `devicectl ... --console`
// captures exactly the process's own output stream — where React Native's
// console.log/Hermes output lands — which syslog-based StreamLogs often
// drowns in unrelated system noise or misses outright.
//
// hostPort, if non-empty, is re-applied as a `-RCT_jsLocation` launch
// argument on the relaunch this triggers. This matters specifically for
// physical iOS: a custom bundler location set there has no persistence of
// its own (no `defaults write` equivalent exists for real hardware), so
// without re-supplying it here, this relaunch would silently revert to the
// app's compiled-in default and never reach Metro.
type ConsoleLogStreamer interface {
	Platform() Platform
	StreamConsoleLog(ctx context.Context, deviceID, bundleID, hostPort string) (*LogStream, error)
}

// StreamConsoleLog routes to the Manager that implements ConsoleLogStreamer
// for the device's Platform and Kind.
func (c *Coordinator) StreamConsoleLog(ctx context.Context, dev Device, bundleID, hostPort string) (*LogStream, error) {
	for _, m := range c.Managers {
		s, ok := m.(ConsoleLogStreamer)
		if !ok || s.Platform() != dev.Platform {
			continue
		}
		k, isKinded := m.(KindedManager)
		if !isKinded || k.Kind() != dev.Kind {
			continue
		}
		return s.StreamConsoleLog(ctx, dev.ID, bundleID, hostPort)
	}
	return nil, fmt.Errorf("no console log streamer registered for platform %s", dev.Platform)
}
