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
	"runtime/trace"
	"syscall"
	"time"
)

var (
	pprofAddr = flag.String("pprof", "", "start pprof HTTP server on this address (e.g. localhost:6060)")
	heapOut   = flag.String("heap-out", "", "write heap profiles to this path prefix on SIGUSR1 and on exit (e.g. /tmp/simmer-heap)")
)

var (
	cpuOut   = flag.String("cpu-out", "", "write a CPU profile to this file (covers the whole run)")
	traceOut = flag.String("trace-out", "", "write a runtime execution trace to this file (view with `go tool trace`)")

	cpuFile   *os.File
	traceFile *os.File
)

func startProfiling() {
	if *cpuOut != "" {
		if f, err := os.Create(*cpuOut); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "cpu profile: %v\n", err)
		} else if err := pprof.StartCPUProfile(f); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "cpu profile: %v\n", err)
			_ = f.Close()
		} else {
			cpuFile = f
		}
	}
	if *traceOut != "" {
		if f, err := os.Create(*traceOut); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "trace: %v\n", err)
		} else if err := trace.Start(f); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "trace: %v\n", err)
			_ = f.Close()
		} else {
			traceFile = f
		}
	}
	if *pprofAddr != "" {
		go func() {
			if err := http.ListenAndServe(*pprofAddr, nil); err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "pprof: %v\n", err)
			}
		}()
	}
	if *heapOut != "" {
		setupHeapDumps(*heapOut)
	}
}

func stopProfiling() {
	if cpuFile != nil {
		pprof.StopCPUProfile()
		_ = cpuFile.Close()
	}
	if traceFile != nil {
		trace.Stop()
		_ = traceFile.Close()
	}
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
		_, _ = fmt.Fprintf(os.Stderr, "heap dump: %v\n", err)
		return
	}
	defer func() { _ = f.Close() }()
	if err := pprof.WriteHeapProfile(f); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "heap dump write: %v\n", err)
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "heap dump → %s\n", path)
}
