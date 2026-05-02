<!-- refreshed: 2026-05-02 -->
# Architecture

**Analysis Date:** 2026-05-02

## System Overview

```text
┌─────────────────────────────────────────────────────────────┐
│                       Entry Point                           │
│                      `main.go`                              │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│         ┌─────────────────────────────────────────┐         │
│         │           UI Layer (Bubble Tea)         │         │
│         │           `ui/`                         │         │
│         └────────────────────┬────────────────────┘         │
│                              │                              │
│                              ▼                              │
│         ┌─────────────────────────────────────────┐         │
│         │           Device Logic Layer            │         │
│         │           `pkg/device/`                 │         │
│         └────────────────────┬────────────────────┘         │
│                              │                              │
│                              ▼                              │
│         ┌─────────────────────────────────────────┐         │
│         │           System CLI Tools              │         │
│         │   (xcrun simctl, adb, emulator)         │         │
└─────────┴─────────────────────────────────────────┴─────────┘
```

## Component Responsibilities

| Component | Responsibility | File |
|-----------|----------------|------|
| Main Model | Orchestrates state between UI and device logic | `main.go` |
| Sidebar | Device selection and grouping logic | `ui/sidebar.go` |
| MainPane | Detail view and action interface | `ui/mainpane.go` |
| Device Manager | Platform-agnostic device management interface | `pkg/device/device.go` |
| iOS Manager | Integration with Apple `simctl` | `pkg/device/ios.go` |
| Android Manager | Integration with Android `adb` and `emulator` | `pkg/device/android.go` |

## Pattern Overview

**Overall:** Elm Architecture (Model-Update-View) via Bubble Tea v2.

**Key Characteristics:**
- **Unidirectional Data Flow:** User input triggers messages, which update the model, which renders the view.
- **Concurrent Data Fetching:** iOS and Android device lists are fetched in parallel using Go commands.
- **Pure UI Components:** Sub-components like `Sidebar` and `MainPane` handle their own rendering but bubble actions up to the main model.

## Layers

**UI Layer:**
- Purpose: Handles terminal rendering and user interaction.
- Location: `ui/`
- Contains: Bubble Tea components, Lip Gloss styles, TUI layouts.
- Depends on: `pkg/device/`, `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`
- Used by: `main.go`

**Domain Layer:**
- Purpose: Defines the core entities and behaviors of the system.
- Location: `pkg/device/`
- Contains: `Device` struct, `Manager` interface, `Platform` and `Status` types.
- Depends on: Standard library (context, os/exec, json).
- Used by: `ui/`, `main.go`

**Platform Layer:**
- Purpose: Concrete implementations for specific mobile platforms.
- Location: `pkg/device/ios.go`, `pkg/device/android.go`
- Contains: CLI parsing logic, command execution.
- Depends on: `pkg/device/device.go`
- Used by: Initialized in `main.go` or factory methods.

## Data Flow

### Primary Request Path (Device Listing)

1. `main.go` sends `fetchDevicesCmd` (`main.go:270`)
2. `fetchDevicesCmd` calls `Manager.ListDevices` for both iOS and Android concurrently.
3. Managers execute CLI tools (`simctl`, `adb`) and parse JSON/text output.
4. Results are wrapped in a `tea.Msg` and sent back to `Update`.
5. `Update` updates the `Sidebar` and `MainPane` models.
6. `View` renders the updated state.

### Action Flow (Booting a Device)

1. User presses 'b' in `Sidebar` (`main.go:271`).
2. `main.go` triggers `m.bootDeviceCmd(*sel)`.
3. Command calls platform-specific boot logic in `pkg/device/`.
4. On completion, a success/error message is sent to `Update`.

**State Management:**
- Centralized in `main.go` `model` struct.
- Sub-state for UI components managed within `ui/` types but synchronized via `Update` calls.

## Key Abstractions

**Manager:**
- Purpose: Abstract interface for platform-specific device operations.
- Examples: `pkg/device/device.go`
- Pattern: Strategy Pattern for handling different mobile OSs.

**Device:**
- Purpose: Unified representation of a simulator or emulator.
- Examples: `pkg/device/device.go`

## Entry Points

**Main Executable:**
- Location: `main.go`
- Triggers: User runs `go run main.go` or executes the binary.
- Responsibilities: Initializes the Bubble Tea program, sets up global styles, and starts the update loop.

## Architectural Constraints

- **Threading:** Heavy use of `tea.Cmd` for non-blocking I/O. Device listing and actions are performed in background goroutines.
- **Global state:** Styles are managed via `ui/styles.go` which provides shared `lipgloss` styles.
- **CLI Dependency:** The system relies on `xcrun`, `adb`, and `emulator` being present in the PATH.

## Anti-Patterns

### Logic in View

**What happens:** Calculating grouped lists or complex filtering inside `View()`.
**Why it's wrong:** `View()` should be as pure and fast as possible.
**Do this instead:** Pre-calculate grouped data in `Update()` or via setter methods like `Sidebar.SetDevices`.

## Error Handling

**Strategy:** Wrap errors with context and display them in the UI footer or a dedicated error pane.

**Patterns:**
- Errors from commands are returned as `tea.Msg`.
- Errors in `pkg/device` use `fmt.Errorf("context: %w", err)`.

---

*Architecture analysis: 2026-05-02*
