# Performance measurement

How to benchmark and profile Simmer, and how to record results so a
before/after comparison (e.g. around an architecture change) is trustworthy.

## What is measured

| Benchmark | File | Covers |
|---|---|---|
| `BenchmarkView/devices=N` | `internal/app/bench_test.go` | Full `View()` render with 5/50/200 devices |
| `BenchmarkUpdateKeyNav` | same | `Update()` for `j`/`k` navigation |
| `BenchmarkUpdateAndView` | same | One keypress + re-render (the interactive hot loop) |
| `BenchmarkResize` | same | `tea.WindowSizeMsg` handling |
| `BenchmarkParseSimctlOutput` | `internal/device/bench_test.go` | `simctl list --json` parsing |
| `BenchmarkParseLsLaR` | same | Android `ls -laR` tree parsing |

Most real-world latency is spent waiting on `xcrun simctl` / `adb`, which these
benchmarks do not cover. For that, use `hyperfine 'xcrun simctl list --json'`
and the execution trace (below).

Benchmarks call internal functions (`newTestModel`, `parseSimctlOutput`,
`sidebar.SetDevices`, ...). If a refactor renames them, update the benchmarks
**in a commit that is applied to both sides of the comparison**; changing the
benchmark between "before" and "after" invalidates the result.

## Recording results: `scripts/perf.sh`

```bash
scripts/perf.sh run               # -> perf/<git describe>.bench.txt
scripts/perf.sh list              # saved results + dates
scripts/perf.sh compare A [B]     # benchstat A vs B (B defaults to current rev)
```

Environment: `COUNT` (iterations, default 10), `ALLOW_DIRTY=1` (allow an
uncommitted tree; the file is then named `...-dirty`).

Each result file starts with a metadata header (`commit`, `describe`, `date`,
`go`, `machine`) followed by standard `go test -bench` output. `benchstat`
ignores the header lines.

`compare` needs benchstat:

```bash
go install golang.org/x/perf/cmd/benchstat@latest
```

### Naming and tagging

Files are named after `git describe --tags --always --dirty`, so the name tells
you exactly which code was measured:

- `v1.7.0.bench.txt` - exactly the `v1.7.0` tag
- `v1.7.0-3-gc4ad618.bench.txt` - 3 commits after `v1.7.0`
- `...-dirty` - uncommitted changes; do not trust or commit these

Tag the baseline commit so you can re-measure it later:

```bash
git tag perf-baseline
scripts/perf.sh run                 # perf/perf-baseline.bench.txt
git add perf && git commit -m "perf: baseline"
```

Tag *after* the benchmarks and script are committed, otherwise the tag points
at code that has no benchmarks.

### Before / after workflow

```bash
# 1. benchmarks + script committed; tag and measure the baseline
git tag perf-baseline
scripts/perf.sh run

# 2. refactor, commit
# 3. measure again and compare
scripts/perf.sh run
scripts/perf.sh compare perf-baseline
```

`benchstat` reports the median delta with a confidence interval; `~` means no
statistically significant change. Look at `sec/op`, `B/op` and `allocs/op`.

### Rules for trustworthy numbers

- Compare only results from the **same machine**, ideally the same day.
  Re-run the baseline if in doubt (`git checkout perf-baseline && scripts/perf.sh run`).
- Plug in power, close other apps, keep `COUNT` >= 10.
- Commit `perf/*.bench.txt` milestones (small, diffable). Never commit `.prof`
  or `trace.out`; they are ignored by `.gitignore`.
- Shared CI runners are noisy; use CI results for trends, not exact deltas.

## Profiling

Profiling flags are compiled out of release builds (`-tags release`).

| Flag | Output | Viewer |
|---|---|---|
| `-cpu-out cpu.prof` | CPU profile for the whole run | `go tool pprof -http=:8080 cpu.prof` |
| `-trace-out trace.out` | Runtime execution trace (goroutines, blocking, GC) | `go tool trace trace.out` |
| `-heap-out /tmp/heap` | Heap profile on `SIGUSR1` and on exit | `go tool pprof -http=:8080 /tmp/heap_HHMMSS.out` |
| `-pprof localhost:6060` | Live pprof HTTP endpoint | `go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30` |

`-cpu-out` and `-trace-out` are written on a clean exit, so quit with the normal
quit key rather than killing the process. Heap workflows are described in the
README ("Profiling & Heap Analysis").

Profile a single benchmark instead of the whole app:

```bash
go test -run='^$' -bench=View -cpuprofile=cpu.prof -memprofile=mem.prof ./internal/app
go tool pprof -http=:8080 cpu.prof
```

Diff two profiles taken before and after a change:

```bash
go tool pprof -base before.prof after.prof
```

Note that a CPU profile of the live TUI is mostly idle time, since the app
waits on external tools. Prefer the execution trace to inspect how the iOS and
Android fetches overlap.
