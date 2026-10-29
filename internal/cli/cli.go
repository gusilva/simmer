// Package cli is Simmer's headless adapter: it turns argv into calls on a
// session.Session and writes the results as a table or as JSON. It holds no
// device logic of its own — anything it needs that the session cannot
// answer belongs in internal/session, not here.
package cli

import (
	"context"
	"flag"
	"io"

	"simmer/internal/logging"
	"simmer/internal/session"
)

// Exit codes: 1 for a failed operation, 2 for a misuse of the command line.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

// Streams is where a verb writes its output and its diagnostics.
type Streams struct {
	Out io.Writer
	Err io.Writer
}

// Run executes one headless verb. args is the argument list after the
// program name and the global flags, and is never empty — an empty args
// list means the TUI should run instead. The returned value is the process
// exit code.
func Run(ctx context.Context, args []string, logger *logging.Logger, io Streams) int {
	verb, rest := args[0], args[1:]

	switch verb {
	case "help", "-h", "--help":
		usage(io.Out)
		return exitOK
	case "devices", "apps", "db":
	default:
		writef(io.Err, "unknown command %q\n\n", verb)
		usage(io.Err)
		return exitUsage
	}

	sess, err := session.NewDefault(logger)
	if err != nil {
		// A bad config file is worth reporting but not fatal: defaults apply.
		writef(io.Err, "warning: %v\n", err)
	}

	cmd := &command{sess: sess, io: io}

	switch verb {
	case "devices":
		return cmd.devices(ctx, rest)
	case "apps":
		return cmd.apps(ctx, rest)
	case "db":
		return cmd.db(ctx, rest)
	}

	return exitUsage
}

// command carries the session and streams through the verb handlers.
type command struct {
	sess *session.Session
	io   Streams
}

// fail reports an operation failure and returns the error exit code.
func (c *command) fail(err error) int {
	writef(c.io.Err, "error: %v\n", err)
	return exitError
}

// misuse reports a command-line error and returns the usage exit code.
func (c *command) misuse(format string, a ...any) int {
	writef(c.io.Err, "error: "+format+"\n", a...)
	return exitUsage
}

// flags builds a FlagSet that reports parse errors on the error stream
// instead of exiting the process.
func (c *command) flags(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(c.io.Err)
	return fs
}

// Usage writes the command-line help. Exported so the process entry point
// can use it for its own flag errors.
func Usage(w io.Writer) { usage(w) }

func usage(w io.Writer) {
	writef(w, "%s", `simmer — iOS/Android device manager

Running simmer with no command opens the TUI.

Commands:
  simmer devices                       list simulators, emulators and phones
  simmer apps     --device <ref>       list the apps installed on a device
  simmer db ls    --device <ref> --app <bundle>
                                       list SQLite databases in an app's storage
  simmer db ls    --device <ref> --path <dir>
                                       list SQLite databases under any directory
  simmer db query --device <ref> --app <bundle> --db <path> "<SQL>"
                                       run a query against a device database

Device references (--device):
  a UDID or adb serial, a device name, a unique fragment of a name, or
  "booted". Omitted, it means the only running device.

Common flags:
  --json      emit JSON instead of an aligned table
  --fast      (apps) skip per-app version and React Native lookups
  --react-native=false
              (apps) keep versions, skip React Native detection
  --log       write command logging to ./simmer.log
  --log-level ERROR|WARN|INFO|TRACE

Examples:
  simmer devices --json
  simmer apps --device "iPhone 16 Pro"
  simmer db ls --device booted --app com.acme.fieldkit
  simmer db ls --device booted --path /storage/emulated/0/Download
  simmer db query --device booted --app com.acme.fieldkit \
    --db /path/to/cache.sqlite "select count(*) from workorders"
`)
}
