package device

import (
	"context"
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

// StartInstall starts a build+install+launch pipeline asynchronously.
func (c *Coordinator) StartInstall(dev Device, path, scheme string) (*BuildStream, error) {
	if s, err := pickCapability[AppInstallStreamer](c.Managers, dev.Platform, dev.Kind, "install streamer"); err == nil {
		return s.StartInstall(dev, path, scheme)
	}

	// A platform that has not adopted the streaming interface still has a
	// blocking installer; wrapping it keeps one caller-facing shape.
	i, err := pickCapability[AppInstaller](c.Managers, dev.Platform, dev.Kind, "app installer")
	if err != nil {
		return nil, err
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

func newBuildContext() (context.Context, context.CancelFunc) {
	return context.WithCancel(context.Background())
}

func sendEvent(ctx context.Context, ch chan<- string, msg string) {
	select {
	case ch <- msg:
	case <-ctx.Done():
	}
}
