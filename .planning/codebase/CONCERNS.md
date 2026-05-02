# Codebase Concerns

**Analysis Date:** 2026-05-02

## Tech Debt

**UI State Management:**
- Issue: The main application model (`main.go`) is becoming a "God Object," managing focus, device lists, loading states, error aggregation, log streaming, and status sequencing.
- Files: `main.go`, `ui/mainpane.go`
- Impact: Increased complexity makes it harder to add new features or test UI logic in isolation.
- Fix approach: Implement a more granular messaging system or use sub-models with their own `Update` functions for the main pane and sidebar.

**Platform Manager Abstraction:**
- Issue: While the `Manager` interface exists, many features (Booting, Shutdown, App Listing, Log Streaming) rely on optional interface casting (`m.(Booter)`, etc.).
- Files: `pkg/device/device.go`, `pkg/device/ios.go`, `pkg/device/android.go`
- Impact: The `Coordinator` logic is repetitive and fragile when adding new capabilities.
- Fix approach: Unify common device operations into a more comprehensive interface or use a capability-based registry.

**Shell Command Execution:**
- Issue: Heavy reliance on wrapping CLI tools (`xcrun`, `adb`, `emulator`, `plutil`) with `exec.Command`.
- Files: `pkg/device/ios.go`, `pkg/device/android.go`
- Impact: Performance is limited by process spawning overhead; parsing command output (especially plain text) is prone to breakage if tool versions change.
- Fix approach: Use JSON output formats wherever available (already partially done for `simctl`) and consider native libraries if performance becomes a bottleneck.

## Known Bugs

**Incomplete Feature: Logs Tab:**
- Symptoms: The "Logs" tab in the UI is declared but explicitly marked as not implemented.
- Files: `ui/mainpane.go` (`TabLogs`), `ui/mainpane.go:712`
- Trigger: Selecting the "Logs" tab (Key "3").
- Workaround: None; shows "(not implemented yet)".

**Status Bar Race Condition:**
- Symptoms: Status messages may disappear prematurely or persist longer than expected if multiple operations happen in rapid succession.
- Files: `main.go` (status sequencing logic)
- Trigger: Rapidly triggering multiple commands (e.g., refresh while a boot is failing).
- Workaround: A sequence counter (`statusSeq`) is used, but it's a "HACK" to manage transient state.

## Security Considerations

**Shell Injection Risk:**
- Issue: Use of `exec.Command` with user-supplied or device-supplied strings (IDs, AVD names).
- Files: `pkg/device/android.go:32`, `pkg/device/ios.go`, `pkg/device/android_files.go`
- Current mitigation: Most commands use slice-based arguments (not shell strings), and `//nolint:gosec` is used on the emulator boot command.
- Recommendations: Sanitize all identifiers used in command arguments and ensure `exec.CommandContext` is consistently used to prevent orphaned processes.

## Performance Bottlenecks

**Concurrent Discovery Overhead:**
- Problem: Starting multiple shell commands for discovery on every refresh.
- Files: `pkg/device/device.go`, `main.go`
- Cause: `adb devices`, `xcrun simctl list`, and `emulator -list-avds` are called concurrently but frequently.
- Improvement path: Implement a caching layer with TTL for device lists or use a long-lived watcher process where supported by the platform tools.

**File System Crawling:**
- Problem: Retrieving deep file trees from devices can be slow over ADB or Simctl.
- Files: `pkg/device/android_files.go`, `pkg/device/ios_files.go`
- Cause: Recursively calling `ls` or `find` over a bridge.
- Improvement path: Lazy-load file tree nodes as they are expanded in the TUI rather than fetching the whole tree at once.

## Fragile Areas

**Android Serial Mapping:**
- Files: `pkg/device/android.go` (`findAndroidSerial`)
- Why fragile: Maps AVD names to ADB serials by iterating over all devices and running `emu avd name`. This is slow and fails if multiple emulators are in a weird state.
- Safe modification: Cache the mapping once discovered and handle "offline" or "unauthorized" devices gracefully.

**Plist Parsing via plutil:**
- Files: `pkg/device/ios.go`
- Why fragile: Uses a pipe to `plutil -convert json` to avoid a plist dependency. If the plist structure from `simctl listapps` changes, the JSON parsing will fail.
- Test coverage: Gaps in parsing edge-case plist outputs.

## Missing Critical Features

**App Interaction:**
- Problem: Cannot currently launch or stop apps from the TUI, only list them.
- Blocks: Primary workflow for developers testing apps.

**Device Logs:**
- Problem: Log streaming is defined in the package but not connected to the UI.
- Blocks: Debugging applications on the device.

## Test Coverage Gaps

**Platform CLI Parsers:**
- What's not tested: The logic that parses `adb` and `simctl` output strings.
- Files: `pkg/device/android.go`, `pkg/device/ios.go`
- Risk: Changes in tool output format will break the app silently.
- Priority: High

---

*Concerns audit: 2026-05-02*
