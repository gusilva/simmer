# Technology Stack

**Analysis Date:** 2026-05-02

## Languages

**Primary:**
- Go 1.25.4 - Main application logic, TUI management, and external process orchestration.

## Runtime

**Environment:**
- macOS (required for iOS `simctl` and general binary execution)

**Package Manager:**
- Go Modules
- Lockfile: `go.mod` present

## Frameworks

**Core:**
- Bubble Tea v2 (charm.land/bubbletea/v2) - TUI framework using the Elm Architecture (Model, Update, View).

**Testing:**
- Standard Go `testing` package - Unit and integration testing.

**Build/Dev:**
- `vhs` - For recording terminal demos (`demo.tape`).
- `golangci-lint` - For static analysis and linting.

## Key Dependencies

**Critical:**
- `charm.land/bubbletea/v2` v2.0.6 - Core TUI orchestration.
- `charm.land/lipgloss/v2` v2.0.3 - Styling and layout.
- `charm.land/bubbles/v2` v2.1.0 - UI components (lists, viewports, etc.).

**Infrastructure:**
- `golang.org/x/sync` - Concurrency primitives for parallel device scanning.
- `github.com/atotto/clipboard` - Clipboard integration for copying device UDIDs/serials.

## Configuration

**Environment:**
- Configured via system `PATH` (requires `xcrun`, `adb`, and `emulator`).

**Build:**
- Standard Go build system (`go build`).

## Platform Requirements

**Development:**
- Go 1.25.4+
- macOS (for iOS simulator support)
- Android SDK (for emulator support)

**Production:**
- macOS (CLI tool)

---

*Stack analysis: 2026-05-02*
