package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"simmer/internal/app"
	"simmer/internal/logging"

	"github.com/sirupsen/logrus"
)

var version = "dev"

func main() {
	logEnabled := flag.Bool("log", false, "enable structured command logging")
	logLevel := flag.String("log-level", "INFO", "logging level: ERROR, WARN, INFO, TRACE")

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
