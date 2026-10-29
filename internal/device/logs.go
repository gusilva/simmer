package device

import (
	"context"
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

// StreamLogs starts streaming an app's log output. The returned stream's
// lifetime is independent of ctx once started; callers must invoke
// Stream.Stop to free resources.
func (c *Coordinator) StreamLogs(ctx context.Context, dev Device, app App) (*LogStream, error) {
	s, err := pickCapability[LogStreamer](c.Managers, dev.Platform, dev.Kind, "log streamer")
	if err != nil {
		return nil, err
	}
	return s.StreamLogs(ctx, dev, app)
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
	s, err := pickKinded[ConsoleLogStreamer](c.Managers, dev.Platform, dev.Kind, "console log streamer")
	if err != nil {
		return nil, err
	}
	return s.StreamConsoleLog(ctx, dev.ID, bundleID, hostPort)
}
