
<div align="center">
  <img alt="simmer logo" src="./docs/simmer-logo.PNG" width="300" style="vertical-align: middle;" /> 
</div>

> A lightweight, modern TUI for managing iOS Simulators and Android Emulators on macOS.


[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![CI](https://github.com/gusilva/simmer/actions/workflows/ci.yml/badge.svg)](https://github.com/gusilva/simmer/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/gusilva/simmer/graph/badge.svg?token=OMG9LKM6WZ)](https://codecov.io/gh/gusilva/simmer)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

---

<p align="center">
  <img src="./app.gif" width="100%" alt="Simulight Demo">
</p>

## 🚀 Features

- **Unified View:** See all your iOS and Android virtual devices in one list.
- **Real-time Status:** Instantly see if a device is `Running` or `Shutdown`.
- **Fast Discovery:** Parallelized scanning using `xcrun simctl` and `adb`.
- **Filtering:** Quickly find devices by name or OS version using built-in fuzzy search.
- **Database Viewer:** Full-screen SQLite inspector — browse tables/views/indexes/triggers, write and run SQL queries, and view results in a scrollable table. Supports custom table-list queries and persists settings to `~/.config/simmer/config.toml`.
- **Modern UI:** Built with the latest Charm v2 terminal stack.

## 🛠 Tech Stack

- **Go 1.25+**
- **[Bubble Tea v2](https://charm.land/bubbletea/v2):** The TUI framework.
- **[Bubbles v2](https://charm.land/bubbles/v2):** Reusable UI components (List, Status Bar).
- **[Lip Gloss v2](https://charm.land/lipgloss/v2):** Beautiful terminal styling.

## 📦 Installation

### Download pre-built binary (recommended)

Go to the [Releases](../../releases/latest) page and download the binary for your Mac:

- `simmer` — universal binary (Intel + Apple Silicon)
- `simmer-darwin-arm64` — Apple Silicon only
- `simmer-darwin-amd64` — Intel only

Or via `curl`:

```bash
curl -L https://github.com/gusilva/simmer/releases/latest/download/simmer -o simmer
chmod +x simmer
./simmer
```

### Install with Go

```bash
go install github.com/gusilva/simmer@latest
```

## 📋 Prerequisites

- **macOS** (Required for iOS simulators).
- **Xcode** with Command Line Tools installed.
- **Android SDK** with `adb` and `emulator` in your `PATH`.

## 📖 Usage Guide

### Running locally
```bash
go run ./cmd/simmer
```

### Keybindings
- `↑/↓/k/j`: Navigate list
- `/`: Search/Filter
- `r`: Refresh device list
- `q/ctrl+c`: Quit
- `enter`: (Future) Launch selected device

### Building for production
```bash
go build -tags release -o simmer ./cmd/simmer
```

The `release` build tag strips all profiling code — `--pprof` and `--heap-out` flags are not registered,
and no `net/http/pprof` or `runtime/pprof` symbols are linked into the binary.
Dev builds (no tag) include profiling support by default.

### Versioning

Release binaries embed the git tag as the app version (for example, `v1.2.3`), injected at build time by the release workflow.
Local builds default to `dev` unless you set the linker variable manually.

### Running Tests
```bash
go test ./... -v
```

## 🔬 Profiling & Heap Analysis

> **Dev only.** Profiling is compiled out of release binaries (`-tags release`). Run a dev build to use these flags.

Simmer ships two flags for memory profiling. Because it is a full-screen TUI that owns the terminal,
the standard approach of curling a pprof HTTP endpoint from the same shell is awkward.
The flags below give you two complementary workflows.

### Live HTTP endpoint (`--pprof`)

Starts a `net/http/pprof` server in the background. The TUI runs normally; you query the endpoint from a second terminal.

```bash
go run ./cmd/simmer --pprof localhost:6060
```

**Useful queries from a second terminal:**

```bash
# Interactive browser (flamegraph, top, source)
go tool pprof -http=:8080 http://localhost:6060/debug/pprof/heap

# Dump to file (add ?gc=1 to force GC before sampling)
curl -s "http://localhost:6060/debug/pprof/heap?gc=1" > heap.out
go tool pprof heap.out
```

### Signal-triggered file dumps (`--heap-out`)

Writes a timestamped heap profile every time you send `SIGUSR1` to the process, and one final
profile on clean exit. No second terminal needed for the curl — just `kill -USR1`.

```bash
go run ./cmd/simmer --heap-out /tmp/simmer-heap
```

Each signal creates a file named `<prefix>_HHMMSS.out`:

```
/tmp/simmer-heap_143022.out   ← baseline
/tmp/simmer-heap_143158.out   ← after doing something
/tmp/simmer-heap_143310.out   ← final (also written on exit)
```

**Taking a snapshot:**

```bash
kill -USR1 $(pgrep simmer)
```

### Recommended investigation workflow

Use `--heap-out` to capture snapshots at known points, then diff them with `go tool pprof -base` to isolate what grew.

```bash
# 1. Start
go run ./cmd/simmer --heap-out /tmp/h

# 2. Baseline — app just launched
kill -USR1 $(pgrep simmer)          # /tmp/h_HH0000.out

# 3. Peak — do the thing (load sims, stream logs, etc.)
kill -USR1 $(pgrep simmer)          # /tmp/h_HH0100.out

# 4. Recovery — undo the thing (stop sims, switch device)
kill -USR1 $(pgrep simmer)          # /tmp/h_HH0200.out

# 5. Diff peak vs recovery — anything still live is a candidate leak
go tool pprof -base /tmp/h_HH0100.out /tmp/h_HH0200.out
(pprof) top10
(pprof) list <FuncName>
```

Both flags can be combined:

```bash
go run ./cmd/simmer --pprof localhost:6060 --heap-out /tmp/simmer-heap
```

### Reading the output

| pprof type | What it measures |
|---|---|
| `inuse_space` (default) | Bytes currently live on the heap |
| `alloc_space` (`-alloc_space`) | Total bytes ever allocated (ignores GC) |

`inuse_space` is the right view for leak hunting — if a symbol shows up here after you expect it to be freed,
it is retained. Use `-alloc_space` to find hot allocation paths regardless of liveness.

```bash
# Show allocation hot-paths instead of live memory
go tool pprof -alloc_space heap.out
(pprof) top10
```

## 🏗 Project Structure

- `cmd/simmer/main.go`: CLI entry point.
- `internal/app/`: Bubble Tea app model/update/view orchestration.
- `internal/ui/`: TUI components (sidebar, main pane, overlays, styles).
- `internal/ui/dbviewer/`: Self-contained SQLite database viewer overlay (explorer, query editor, results pane, settings form).
- `internal/device/`: Device domain contracts, coordinator, and platform integrations.
- `internal/config/`: User settings persistence (`~/.config/simmer/config.toml`).

## 📚 Documentation

- **Architecture**
  - [App model/update/view flow](docs/architecture/app-model-update-view.md)
  - [Device layer design](docs/architecture/device-layer.md)
  - [Database viewer architecture](docs/architecture/db-viewer.md)
  - [Config package](docs/architecture/config.md)
- **UI Components**
  - [Main pane](docs/ui/mainpane.md)
  - [Sidebar](docs/ui/sidebar.md)
  - [Overlays](docs/ui/overlays.md)
  - [Top bar](docs/ui/topbar.md)
  - [Footer](docs/ui/footer.md)
  - [Tabs](docs/ui/tabs.md)
  - [Delegate](docs/ui/delegate.md)
  - [Box](docs/ui/box.md)
  - [Database viewer UI](docs/ui/dbviewer.md)
