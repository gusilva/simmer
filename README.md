# Simmer

> A lightweight, modern TUI for managing iOS Simulators and Android Emulators on macOS.

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![CI](https://github.com/gusilva/simmer/actions/workflows/ci.yml/badge.svg)](https://github.com/gusilva/simmer/actions/workflows/ci.yml)
[![Coverage](https://codecov.io/gh/gusilva/simmer/branch/main/graph/badge.svg)](https://codecov.io/gh/gusilva/simmer)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

<p align="center">
  <img src="./app.gif" width="350" alt="Simulight Demo">
</p>

## 🚀 Features

- **Unified View:** See all your iOS and Android virtual devices in one list.
- **Real-time Status:** Instantly see if a device is `Running` or `Shutdown`.
- **Fast Discovery:** Parallelized scanning using `xcrun simctl` and `adb`.
- **Filtering:** Quickly find devices by name or OS version using built-in fuzzy search.
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
go run main.go
```

### Keybindings
- `↑/↓/k/j`: Navigate list
- `/`: Search/Filter
- `r`: Refresh device list
- `q/ctrl+c`: Quit
- `enter`: (Future) Launch selected device

### Building for production
```bash
go build -o simmer main.go
```

### Running Tests
```bash
go test ./... -v
```

## 🏗 Project Structure

- `main.go`: Application entry point and TUI state management (Model-Update-View).
- `pkg/device/`: 
    - `device.go`: Core interfaces and the concurrent `Coordinator`.
    - `ios.go`: `xcrun simctl` integration and parsing.
    - `android.go`: `adb` and `emulator` integration.
