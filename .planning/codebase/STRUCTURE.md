# Codebase Structure

**Analysis Date:** 2026-05-02

## Directory Layout

```
simulight/
├── main.go             # Application entry point and orchestrator
├── ui/                 # TUI components and styles
│   ├── sidebar.go      # Device list navigation
│   ├── mainpane.go     # Device details and actions
│   ├── footer.go       # Status and keymaps
│   ├── topbar.go       # Tabs and breadcrumbs
│   └── styles.go       # Lip Gloss style definitions
├── pkg/
│   └── device/         # Domain logic and CLI integrations
│       ├── device.go   # Core interfaces and types
│       ├── ios.go      # iOS simctl integration
│       ├── android.go  # Android adb/emulator integration
│       └── logs.go     # Device log streaming
├── go.mod              # Dependency management
└── CLAUDE.md           # Development guidelines
```

## Directory Purposes

**ui/:**
- Purpose: Contains all Bubble Tea UI components.
- Contains: Component structs, Update/View methods for sub-components, and styling.
- Key files: `ui/sidebar.go`, `ui/mainpane.go`.

**pkg/device/:**
- Purpose: Encapsulates all mobile device management logic.
- Contains: Code that interacts with external processes (`simctl`, `adb`).
- Key files: `pkg/device/device.go`, `pkg/device/ios.go`, `pkg/device/android.go`.

## Key File Locations

**Entry Points:**
- `main.go`: Main program loop and top-level model.

**Configuration:**
- `go.mod`: Project dependencies.
- `CLAUDE.md`: Style and development rules.

**Core Logic:**
- `pkg/device/device.go`: Defines the `Manager` interface.
- `pkg/device/ios.go`: Implements iOS device discovery.
- `pkg/device/android.go`: Implements Android device discovery.

**Testing:**
- `pkg/device/ios_test.go`: Unit tests for parsing CLI output.
- `pkg/device/android_test.go`: Unit tests for Android logic.

## Naming Conventions

**Files:**
- lowercase with underscores (Standard Go package style).
- Example: `android_files.go`, `mainpane.go`.

**Directories:**
- Short, single-word names where possible.
- Example: `ui`, `pkg`, `device`.

## Where to Add New Code

**New Feature:**
- If it's a UI feature: Add a new component in `ui/` or update `main.go`.
- If it's a device action: Add a method to the `Manager` interface in `pkg/device/device.go` and implement it in `ios.go`/`android.go`.

**New Component/Module:**
- Implementation: `ui/[component].go`.

**Utilities:**
- Shared helpers: If UI related, `ui/styles.go` or a new `ui/utils.go`. If logic related, `pkg/device/`.

## Special Directories

**pkg/:**
- Purpose: Contains library code that can be used by other projects (though currently internal).
- Generated: No
- Committed: Yes

**.agents/ or .planning/:**
- Purpose: GSD meta-documentation and skill storage.
- Generated: Mostly Yes
- Committed: Yes

---

*Structure analysis: 2026-05-02*
