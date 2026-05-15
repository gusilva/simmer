//go:build !release

package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime/pprof"
	"syscall"
	"time"
)

var (
	pprofAddr = flag.String("pprof", "", "start pprof HTTP server on this address (e.g. localhost:6060)")
	heapOut   = flag.String("heap-out", "", "write heap profiles to this path prefix on SIGUSR1 and on exit (e.g. /tmp/simmer-heap)")
)

func startProfiling() {
	if *pprofAddr != "" {
		go func() {
			if err := http.ListenAndServe(*pprofAddr, nil); err != nil {
				fmt.Fprintf(os.Stderr, "pprof: %v\n", err)
			}
		}()
	}
	if *heapOut != "" {
		setupHeapDumps(*heapOut)
	}
}

func stopProfiling() {
	if *heapOut != "" {
		writeHeapDump(*heapOut)
	}
}

// setupHeapDumps installs a SIGUSR1 handler that writes a timestamped heap
// profile to prefix_<timestamp>.out each time the signal is received.
func setupHeapDumps(prefix string) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGUSR1)
	go func() {
		for range ch {
			writeHeapDump(prefix)
		}
	}()
}

func writeHeapDump(prefix string) {
	path := fmt.Sprintf("%s_%s.out", prefix, time.Now().Format("150405"))
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "heap dump: %v\n", err)
		return
	}
	defer f.Close()
	if err := pprof.WriteHeapProfile(f); err != nil {
		fmt.Fprintf(os.Stderr, "heap dump write: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "heap dump → %s\n", path)
}
