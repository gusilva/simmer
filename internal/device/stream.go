package device

import (
	"context"
	"fmt"
)

// BuildStream is a live stream of build/install progress events.
//
// Consumers receive text lines from Events until that channel is closed; after
// closure, exactly one error (possibly nil) is delivered on Done. Calling Stop
// cancels the underlying process; Stop is safe to call multiple times.
type BuildStream struct {
	Events <-chan string
	Done   <-chan error
	Stop   func()
}

// AppInstallStreamer is an optional interface a Manager may implement to run
// a non-blocking build+install+launch pipeline. The returned BuildStream is
// independent of any caller context; callers must invoke Stop to free resources.
type AppInstallStreamer interface {
	Platform() Platform
	StartInstall(dev Device, path, scheme string) (*BuildStream, error)
}

// StartInstall routes to the Manager that implements AppInstallStreamer for
// the device's platform and starts the pipeline asynchronously.
func (c *Coordinator) StartInstall(dev Device, path, scheme string) (*BuildStream, error) {
	for _, m := range c.Managers {
		s, ok := m.(AppInstallStreamer)
		if !ok || s.Platform() != dev.Platform {
			continue
		}
		return s.StartInstall(dev, path, scheme)
	}
	// Fallback: wrap synchronous AppInstaller in a stream for platforms that
	// have not yet adopted AppInstallStreamer.
	for _, m := range c.Managers {
		i, ok := m.(AppInstaller)
		if !ok || i.Platform() != dev.Platform {
			continue
		}
		events := make(chan string, 64)
		done := make(chan error, 1)
		ctx, cancel := newBuildContext()
		go func() {
			defer close(events)
			defer cancel()
			events <- "Installing…"
			done <- i.InstallApp(ctx, dev.ID, path, scheme)
		}()
		return &BuildStream{Events: events, Done: done, Stop: cancel}, nil
	}
	return nil, fmt.Errorf("no install streamer registered for platform %s", dev.Platform)
}

func newBuildContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

func sendEvent(ctx context.Context, ch chan<- string, msg string) {
	select {
	case ch <- msg:
	case <-ctx.Done():
	}
}
