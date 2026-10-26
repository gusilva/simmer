package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"simmer/internal/app"
	"simmer/internal/cli"
	"simmer/internal/logging"

	"github.com/sirupsen/logrus"
)

var version = "dev"

func main() {
	logEnabled := flag.Bool("log", false, "enable structured command logging")
	logLevel := flag.String("log-level", "INFO", "logging level: ERROR, WARN, INFO, TRACE")

	// Help is answered before flag parsing so that `simmer --help` prints to
	// stdout and exits 0, rather than being rejected as an unknown global
	// flag. Only the first argument counts, so a "help" inside a SQL
	// statement or a flag value is left alone.
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "-h", "--help", "help":
			cli.Usage(os.Stdout)
			return
		}
	}

	flag.Usage = func() { cli.Usage(os.Stderr) }
	flag.Parse()
	startProfiling()

	var logger *logging.Logger
	if *logEnabled {
		level, err := logging.ParseLevel(*logLevel)
		if err != nil {
			fmt.Printf("Invalid log level: %v\n", err)
			os.Exit(1)
		}

		logger, err = logging.New(level)
		if err != nil {
			fmt.Printf("Failed to initialize logger: %v\n", err)
			os.Exit(1)
		}
		defer logger.Close()
	}

	// go-ios uses logrus for internal progress logs. Route them into our log
	// file when --log is active, otherwise discard so they don't bleed into
	// the TUI's terminal output.
	if w := logger.Writer(); w != nil {
		logrus.SetOutput(w)
	} else {
		logrus.SetOutput(io.Discard)
	}

	// A command word selects headless mode; bare "simmer" opens the TUI.
	// Both modes share the global logging flags parsed above.
	if args := flag.Args(); len(args) > 0 {
		code := cli.Run(context.Background(), args, logger, cli.Streams{Out: os.Stdout, Err: os.Stderr})
		stopProfiling()
		os.Exit(code)
	}

	// Capture the launch directory once: app-log files (Apps tab, "l") are
	// written here regardless of any later working-directory changes.
	launchDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("Failed to determine working directory: %v\n", err)
		os.Exit(1)
	}

	if err := app.Run(version, logger, launchDir); err != nil {
		fmt.Printf("Fatal error: %v\n", err)
		os.Exit(1)
	}

	stopProfiling()
}
