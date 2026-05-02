# Coding Conventions

**Analysis Date:** 2026-05-02

## Naming Patterns

**Files:**
- lowercase with underscores for multi-word files: `ios_files.go`, `mainpane.go`
- `_test.go` suffix for test files: `android_test.go`
- Component files in `ui/` are generally single words: `sidebar.go`, `footer.go`, `styles.go`

**Functions:**
- Exported: PascalCase (`NewCoordinator`, `ListDevices`, `Update`, `View`)
- Internal: camelCase (`parseSimctlOutput`, `fetchDevicesCmd`, `renderAppRow`)
- Constructor-like: `New[Type]` pattern used consistently (`NewSidebar`, `NewCoordinator`, `NewIOSManager`)

**Variables:**
- Short, single-letter or abbreviated names for local receivers and loop variables: `m` (model/manager), `d` (device), `ctx` (context), `err` (error), `msg` (message)
- camelCase for multi-word internal variables: `bootedCount`, `toolVersions`
- PascalCase for exported struct fields: `ID`, `Name`, `Platform`

**Types:**
- Structs and interfaces use PascalCase: `Device`, `Manager`, `MainPane`, `Sidebar`
- Enums using `iota` for internal types: `appFocus`, `MainTab`
- Enums using string types for exported constants: `Platform`, `Status`

## Code Style

**Formatting:**
- standard Go `gofmt` (tabs for indentation)
- Import grouping: standard library first, then internal/third-party grouped together

**Linting:**
- `golangci-lint` mentioned in `CLAUDE.md`, though no config file was detected in root.

## Import Organization

**Order:**
1. Standard library (e.g., `context`, `fmt`, `os`)
2. Internal project packages (e.g., `simmer/pkg/device`, `simmer/ui`)
3. Third-party dependencies (e.g., `charm.land/bubbletea/v2`)

**Path Aliases:**
- `tea` for `charm.land/bubbletea/v2`
- None used for internal packages

## Error Handling

**Patterns:**
- Standard Go `if err != nil` check and return
- Bubble Tea messages often carry an `err` field: `bootResultMsg { err error }`
- Coordinator patterns often aggregate errors into a slice: `DiscoveryResult { Errors []error }`
- Error wrapping with context: `fmt.Errorf("context: %w", err)` is the prescribed pattern in `CLAUDE.md`

## Logging

**Framework:**
- Custom `LogStream` in `pkg/device/logs.go` for device log streaming
- `fmt.Printf` for fatal errors in `main.go`

**Patterns:**
- Logs are streamed via channels: `stream.Lines` (chan string) and `stream.Done` (chan error)

## Comments

**When to Comment:**
- Exported types and functions have documentation comments
- Complex UI rendering logic (e.g., `View`, `Update`) has explanatory comments
- Internal helpers for layout and rendering often include short descriptions

**JSDoc/TSDoc:**
- Not applicable (Go-style doc comments used)

## Function Design

**Size:**
- Model `Update` and `View` functions can be large due to switch statements and layout logic
- Logic is decomposed into smaller rendering helpers (e.g., `renderAppRow`, `renderInfo`)

**Parameters:**
- Uses `context.Context` as the first parameter for operations that may time out or be cancelled
- Struct parameters (e.g., `TopBarParams`, `FooterParams`) for complex UI rendering to avoid long argument lists

**Return Values:**
- Commands return `tea.Cmd` in the UI layer
- Package `device` methods often return `(T, error)`

## Module Design

**Exports:**
- Controlled via capitalization (PascalCase)
- UI components follow a "Model-Update-View" encapsulation pattern within their respective files

**Barrel Files:**
- Not used (standard Go package structure)

---

*Convention analysis: 2026-05-02*
