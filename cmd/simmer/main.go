package main

import (
	"fmt"
	"os"

	"simmer/internal/app"
)

var version = "dev"

func main() {
	if err := app.Run(version); err != nil {
		fmt.Printf("Fatal error: %v\n", err)
		os.Exit(1)
	}
}
