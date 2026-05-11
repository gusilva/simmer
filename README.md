
<div align="center">
  <img alt="simmer logo" src="./docs/simmer-logo.PNG" width="90" style="vertical-align: middle;" /> 
  <span style="font-size: 2em; font-weight: bold; vertical-align: middle; line-height: 90px;">Simmer</span>
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
go build -o simmer ./cmd/simmer
```

### Versioning

Release binaries embed the git tag as the app version (for example, `v1.2.3`), injected at build time by the release workflow.
Local builds default to `dev` unless you set the linker variable manually.

### Running Tests
```bash
go test ./... -v
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
