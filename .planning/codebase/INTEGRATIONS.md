# External Integrations

**Analysis Date:** 2026-05-02

## APIs & External Services

**iOS Simulator Management:**
- `xcrun simctl` - Primary interface for listing, booting, and managing iOS simulators.
  - SDK/Client: Native `exec.Command` calls.
  - Auth: None (local system).

**Android Emulator Management:**
- `adb` (Android Debug Bridge) - Used for device discovery, app management, and log streaming.
  - SDK/Client: Native `exec.Command` calls.
  - Auth: None (local system).
- `emulator` - Used for listing AVDs and launching emulators.
  - SDK/Client: Native `exec.Command` calls.

**System Utilities:**
- `plutil` - Used to convert binary property lists (plist) to JSON for parsing.
- `sw_vers` / `uname` - Used via `simctl spawn` to gather OS info from running simulators.

## Data Storage

**Databases:**
- None.

**File Storage:**
- Local filesystem only - Reading device metadata and app container information.

**Caching:**
- None detected.

## Authentication & Identity

**Auth Provider:**
- Custom / None - Relies on local system permissions for executing CLI tools.

## Monitoring & Observability

**Error Tracking:**
- None.

**Logs:**
- Streams output from `adb logcat` (Android) and `xcrun simctl spawn log stream` (iOS).

## CI/CD & Deployment

**Hosting:**
- Local execution.

**CI Pipeline:**
- None detected in repository.

## Environment Configuration

**Required env vars:**
- `PATH` - Must include paths to `xcrun`, `adb`, and `emulator`.
- `ANDROID_HOME` / `ANDROID_SDK_ROOT` - Typically required for Android tools to function.

**Secrets location:**
- Not applicable (no cloud credentials used).

## Webhooks & Callbacks

**Incoming:**
- None.

**Outgoing:**
- None.

---

*Integration audit: 2026-05-02*
