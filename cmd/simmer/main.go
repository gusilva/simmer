package main

import (
	"flag"
	"fmt"
	"os"

	"simmer/internal/app"
	"simmer/internal/logging"
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

	if err := app.Run(version, logger); err != nil {
		fmt.Printf("Fatal error: %v\n", err)
		os.Exit(1)
	}

	stopProfiling()
}
