# Simmer - iOS/Android Device Manager TUI

TUI application to list and manage iOS simulators and Android emulators on macOS.

## Commands

`make check` runs everything CI enforces; run it before pushing. `make` targets are the source of
truth for build, test, lint and the rest — read the `Makefile` rather than assembling commands.

- **Device tests:** `make check-device` exercises attached simulators, emulators and phones.
  Unit tests cannot reach the behaviour that breaks most here — access routes into app storage,
  whether a device ships sqlite3, what a given toybox build does. Add cases to
  `internal/device/integration_device_test.go` instead of writing a throwaway harness.
- **Recording:** `vhs demo.tape`

## Architecture & Tech Stack

- **Framework:** Bubble Tea v2 (Elm Architecture: Model, Update, View)
- **Styling:** Lip Gloss v2
- **Components:** Bubbles v2
- **External Tools:**
    - iOS: `xcrun simctl list --json`
    - Android: `adb devices -l` and `emulator -list-avds`

## Reference

- **Domain terms:** `CONTEXT.md` — read before naming a new concept, and extend it when one lands.
- **Decisions:** `docs/adr/` — read before changing project resolution, persistence, or device
  routing; these are settled, not open.
- **Headless mode:** `docs/headless.md` — the full command surface and its per-platform behaviour.

## Development Guidelines

### Bubble Tea v2 Patterns
- `View()` must return `tea.View`, not a string.
- Use `tea.KeyPressMsg` for key events (not `tea.KeyMsg`).
- Terminal features (AltScreen, Mouse) are declared in `View()`.
- Use `charm.land/` import paths for the Charm stack.

### Go Patterns
- **Concurrency:** Fetch iOS and Android lists concurrently using Goroutines and Channels/Context.
- **Error Handling:** Wrap errors with context (`fmt.Errorf("context: %w", err)`).
- **Interfaces:** Keep interfaces small and focused.
- **Testing:** Use table-driven tests for logic (parsing command output).

### Code Style
- **Indentation:** Tabs (standard Go `gofmt`).
- **Naming:** CamelCase for exported symbols, camelCase for internal.
- **Imports:** Grouped: standard library, then third-party.
