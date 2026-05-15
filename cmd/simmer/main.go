package main

import (
	"flag"
	"fmt"
	"os"

	"simmer/internal/app"
)

var version = "dev"

func main() {
	flag.Parse()
	startProfiling()

	if err := app.Run(version); err != nil {
		fmt.Printf("Fatal error: %v\n", err)
		os.Exit(1)
	}

	stopProfiling()
}
