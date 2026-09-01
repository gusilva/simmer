# Plan: Native Device Libraries — go-ios & goadb

## Goal

Replace subprocess calls to `xcrun`, `adb`, and `emulator` for **physical device** management
with pure-Go libraries embedded in the binary. Users of physical-device features need zero
external tool installation beyond the USB driver.

Virtual device management (simulators, emulators) retains its current xcrun / Android SDK
dependencies — those tools are inseparable from the virtual device runtime.

---

## Library selection

### iOS — `github.com/danielpaulus/go-ios`

Pure Go reimplementation of Apple's Mobile Device Protocol (MuxedDevice / Lockdown / AFC).
No CGo, no shared libraries, no Xcode dependency.

### Android — `github.com/zach-klippenstein/goadb`

Pure Go ADB wire-protocol client. Chosen over `electricbubble/gadb` for three reasons:

| Feature | `goadb` (zach-klippenstein) | `gadb` (electricbubble) |
|---------|----------------------------|------------------------|
| Last commit | Dec 2020 | Mar 2024 |
| **`DeviceWatcher`** | ✅ channel-based connect/disconnect events | ❌ poll only |
| File streaming | `OpenRead` → `io.ReadCloser` | `Pull` → buffers to writer |
| File writing | `OpenWrite` → `io.WriteCloser` | `Push` from reader |
| ADB server control | `StartServer()`, `KillServer()` | ❌ |
| USB detection | `DeviceInfo.IsUsb()` | `Device.IsUsb()` |

The **`DeviceWatcher`** is the deciding factor: it fires `CameOnline` / `WentOffline` events
the instant a USB device is plugged in, letting us refresh the Online box without polling.
The 2020 timestamp is acceptable because the ADB wire protocol is stable and unchanged.

### Why not CGo / libimobiledevice?

libimobiledevice requires statically linking openssl, libplist, libimobiledevice-glue, and
usbmuxd. That build graph breaks cross-compilation and forces platform-specific artefacts.
`go-ios` implements the same protocols entirely in Go.

---

## Architecture

### Manager split

```
Coordinator
├── iosManager              (simulators only — xcrun simctl)
├── physicalIOSManager      (physical iOS — go-ios)
├── androidManager          (emulators only — emulator / avdmanager / adb)
└── physicalAndroidManager  (physical Android — goadb)
```

### New interface: `KindedManager`

```go
// KindedManager is optionally implemented by managers that serve exactly
// one DeviceKind. Coordinator uses this to avoid routing a physical device
// to an emulator manager and vice versa.
type KindedManager interface {
    Kind() DeviceKind
}
```

`Coordinator` routing for `ListApps`, `DeleteApp`, `StreamLogs`, `Info`:
1. Prefer a manager whose `Platform()` matches **and** whose `Kind()` matches `dev.Kind`.
2. Fall back to a manager that matches `Platform()` only (no `KindedManager` implemented).

---

## Phase 1 — go-ios (physical iOS)

### 1.1 Add dependency

```
go get github.com/danielpaulus/go-ios@latest
```

### 1.2 New file `internal/device/ios_physical.go`

Struct `physicalIOSManager{}` implementing:

| Interface | Method |
|-----------|--------|
| `Manager` | `ListDevices` |
| `KindedManager` | `Kind() → KindPhysical` |
| `AppLister` | `ListApps` |
| `AppDeleter` | `DeleteApp` |
| `LogStreamer` | `StreamLogs` |
| `InfoLister` | `Info` |

### 1.3 `ListDevices`

```go
func (m *physicalIOSManager) ListDevices(ctx context.Context) ([]Device, error) {
    list, err := ios.ListDevices()
    if err != nil {
        // usbmuxd unavailable — not an error, just no physical devices
        return nil, nil
    }
    var out []Device
    for _, entry := range list.DeviceList {
        values, err := ios.GetAllValues(entry)
        if err != nil {
            // Locked / untrusted — still emit a placeholder so the user sees it
            out = append(out, Device{
                ID:       entry.Properties.SerialNumber,
                Name:     "iPhone (locked — unlock to trust)",
                Platform: PlatformIOS,
                Status:   StatusRunning,
                Kind:     KindPhysical,
            })
            continue
        }
        out = append(out, Device{
            ID:       entry.Properties.SerialNumber,
            Name:     stringVal(values, "DeviceName"),
            Platform: PlatformIOS,
            Version:  stringVal(values, "ProductVersion"),
            Status:   StatusRunning,
            Kind:     KindPhysical,
        })
    }
    return out, nil
}
```

**Edge cases**
- `ios.ListDevices` fails → usbmuxd socket absent or permission denied → return `nil, nil` (coordinator skips gracefully, no error shown).
- `ios.GetAllValues` fails → device locked or not yet trusted → emit placeholder with descriptive name.
- Multiple devices connected → all emitted; each has a unique `SerialNumber`.

### 1.4 `ListApps`

Uses `installationproxy` service.

```go
import "github.com/danielpaulus/go-ios/ios/installationproxy"

svc, err := installationproxy.New(device)
all, err := svc.BrowseAllApps()
// map plist keys to App{BundleID, DisplayName, Name, ShortVersion, Type, Path}
```

**Edge cases**
- Developer mode not enabled (iOS 16+) → `installationproxy.New` returns a lockdown error containing `DeveloperModeIsNotEnabled` → wrap with user-readable message: `"enable Developer Mode in Settings → Privacy & Security"`.
- App list > 500 entries → process in batches or stream; do not buffer entire plist in memory.
- `CFBundleDisplayName` absent → fall back to `CFBundleName` → fall back to bundle ID.

### 1.5 `DeleteApp`

```go
svc, _ := installationproxy.New(device)
err = svc.Uninstall(bundleID, nil, nil)
```

**Edge cases**
- System / non-removable app → `Uninstall` returns an error; surface it verbatim in the status bar.
- Device locks during uninstall → propagate error.

### 1.6 `StreamLogs`

```go
import "github.com/danielpaulus/go-ios/ios/syslog"

svc, err := syslog.New(device)
// read lines in goroutine, optional predicate filter by process name / bundle ID
```

```go
func appRelevant(line string, app App) bool {
    return strings.Contains(line, app.BundleID) ||
           (app.Name != "" && strings.Contains(line, app.Name))
}
```

**Edge cases**
- `syslog.New` fails with lockdown error (device locked) → return wrapped error: `"log stream failed: device locked"`.
- High-volume syslog → `appRelevant` filter reduces noise. Channel buffer = 256, providing backpressure.
- Device unplugged mid-stream → `ReadLine` returns EOF → close channels cleanly, send `nil` error (not an error from the user's perspective).

### 1.7 `Info`

Use `ios.GetAllValues` to populate `InfoField` list.
Key fields: `DeviceName`, `ProductVersion`, `ProductType`, `UniqueDeviceID`,
`WiFiAddress`, `SerialNumber`, `ModelNumber`, `HardwarePlatform`, `DeviceClass`,
`BatteryCurrentCapacity`.

### 1.8 Remove from `iosManager`

- Delete `listPhysicalDevices`, `parseXctraceDevices`, `parseXctraceDeviceLine`.
- Delete `devicectlAppsOutput`, `listPhysicalApps`.
- Delete `streamPhysicalLogs` (the `log stream --device` subprocess approach).
- Revert `DeleteApp` to simctl-only.
- Remove `KindPhysical` guards from `IOSAppFileSystem.Tree` and `IOSRootFileSystem.Tree`
  (those paths are no longer reached for physical devices).

### 1.9 Registration

```go
// internal/app/model.go
coord := device.NewCoordinator(
    device.NewIOSManager(),
    device.NewPhysicalIOSManager(),       // ← new
    device.NewAndroidManager(),
    device.NewPhysicalAndroidManager(),   // ← Phase 2
)
```

---

## Phase 2 — goadb (physical Android)

### 2.1 Add dependency

```
go get github.com/zach-klippenstein/goadb@latest
```

### 2.2 New file `internal/device/android_physical.go`

Struct `physicalAndroidManager{}` implementing `Manager`, `KindedManager`, `AppLister`,
`AppDeleter`, `LogStreamer`.

### 2.3 `ListDevices`

```go
import adb "github.com/zach-klippenstein/goadb"

client, err := adb.New() // dials ADB server on :5037
if err != nil {
    return nil, nil // ADB server not running — no physical Android devices
}
infos, err := client.ListDevices()
for _, info := range infos {
    if !info.IsUsb() { continue } // emulators handled by androidManager
    // get model and Android version via shell
    dev := client.Device(adb.DeviceWithSerial(info.Serial))
    model, _ := dev.RunCommand("getprop", "ro.product.model")
    version, _ := dev.RunCommand("getprop", "ro.build.version.release")
    out = append(out, Device{
        ID:       info.Serial,
        Name:     strings.TrimSpace(model),
        Platform: PlatformAndroid,
        Version:  strings.TrimSpace(version),
        Status:   StatusRunning,
        Kind:     KindPhysical,
    })
}
```

**Edge cases**
- ADB server not running → `adb.New()` returns connection-refused → `nil, nil`.
- Device in `StateUnauthorized` → `info.State` → emit placeholder: `"(unauthorized — accept USB prompt on device)"`.
- Device in `StateOffline` → skip; reappears on reconnect.
- WiFi ADB (`serial` = `192.168.x.x:5555`) → `info.IsUsb()` is false → treat separately:
  include if `info.State == StateOnline` and serial contains `:`, mark as `StatusRunning`.
- Emulator serials start with `emulator-` → `IsUsb()` false → excluded correctly.

### 2.4 `DeviceWatcher` integration

```go
// internal/app/update.go  or  internal/app/messages.go

type deviceConnectedMsg struct{ serial string }
type deviceDisconnectedMsg struct{ serial string }

func watchADBDevices(client *adb.Adb) tea.Cmd {
    return func() tea.Msg {
        w := client.NewDeviceWatcher()
        defer w.Shutdown()
        for event := range w.C() {
            if event.CameOnline() {
                return deviceConnectedMsg{serial: event.Serial}
            }
            if event.WentOffline() {
                return deviceDisconnectedMsg{serial: event.Serial}
            }
        }
        return nil
    }
}
```

When `deviceConnectedMsg` / `deviceDisconnectedMsg` received → trigger a device refresh
(`fetchDevicesCmd`) so the Online box updates instantly without polling.

**Edge cases**
- ADB server dies → watcher restarts it automatically (built into `DeviceWatcher`).
- Watcher channel closed unexpectedly → goroutine exits cleanly; next refresh cycle
  re-registers the watcher.
- Multiple devices connect simultaneously → each fires a separate `CameOnline` event;
  each triggers an independent refresh (coalesce if refresh already in flight).

### 2.5 `ListApps`

Reuse existing `pm list packages -3 -f` parsing logic from `androidManager`:

```go
dev := client.Device(adb.DeviceWithSerial(deviceID))
out, err := dev.RunCommand("pm", "list", "packages", "-3", "-f")
// parse same format as androidManager.ListApps
// enrich versions via "dumpsys package packages" (reuse androidFetchVersions)
```

**Edge cases**
- Output > 64 KB → goadb buffers the full response; no issue for typical package counts.
- Very old Android (< 5.0) → `pm list packages -f` format varies; fall back to `-3` without `-f`.

### 2.6 `DeleteApp`

```go
dev := client.Device(adb.DeviceWithSerial(deviceID))
out, err := dev.RunCommand("pm", "uninstall", bundleID)
if strings.Contains(out, "Failure") {
    return fmt.Errorf("pm uninstall %s: %s", bundleID, strings.TrimSpace(out))
}
```

### 2.7 `StreamLogs`

`goadb` does not expose a streaming shell API in v0.0.0-20201208. Two options:

1. **Keep the `adb logcat` subprocess** for physical Android — acceptable since the ADB binary
   ships with `android-platform-tools` which is already required for the server daemon.
2. **Implement raw shell stream** by opening an ADB transport directly via `client.Dial()` —
   significantly more work.

**Decision: keep `adb logcat` subprocess for physical Android log streaming only.**
All other physical Android operations (list, delete, file browse) use goadb.

### 2.8 File browsing — `internal/device/android_physical_files.go`

```go
type AndroidPhysicalFileSystem struct {
    RootPath string // "/storage/emulated/0"
    MaxDepth int
}

func (f *AndroidPhysicalFileSystem) Tree(ctx context.Context, dev Device) (FileNode, error) {
    client, _ := adb.New()
    d := client.Device(adb.DeviceWithSerial(dev.ID))
    return buildGoadbTree(ctx, d, f.RootPath, 0, f.MaxDepth)
}

func buildGoadbTree(ctx context.Context, d *adb.Device, path string, depth, max int) (FileNode, error) {
    entries, err := d.ListDirEntries(path)
    // entries is *DirEntries — iterate with entries.Next() / entries.Entry()
    node := FileNode{Name: filepath.Base(path), Path: path, IsDir: true}
    for entries.Next() {
        e := entries.Entry()
        child := FileNode{
            Name:     e.Name,
            Path:     path + "/" + e.Name,
            IsDir:    e.Mode.IsDir(),
            Size:     int64(e.Size),
            Modified: e.ModifiedAt,
            Permissions: e.Mode.String(),
        }
        if e.Mode.IsDir() && depth < max-1 {
            child, _ = buildGoadbTree(ctx, d, child.Path, depth+1, max)
        }
        node.Children = append(node.Children, child)
    }
    if err := entries.Err(); err != nil {
        return node, fmt.Errorf("ListDirEntries %s: %w", path, err)
    }
    return node, nil
}
```

Accessible paths on production (non-rooted) physical Android:

| Path | Accessible without root |
|------|------------------------|
| `/storage/emulated/0` | ✅ external storage |
| `/storage/emulated/0/Android/data/<pkg>` | ✅ external app data |
| `/sdcard` | ✅ symlink to above |
| `/data/data/<pkg>` | ❌ internal sandbox |
| `/data` | ❌ |

Wire up in `commands.go`:
```go
case device.PlatformAndroid:
    if sel.Kind == device.KindPhysical {
        return m, m.loadAndroidPhysicalFileTreeCmd(*sel)
    }
    return m, m.loadAndroidRootFileTreeCmd(*sel)
```

**Edge cases**
- `/storage/emulated/0` returns permission denied (device locked) → propagate error.
- Deep subtree → `MaxDepth = 4` caps it; unexpanded dirs show `▸` caret.
- Symlinks → `e.Mode` has `ModeSymlink` set; render as file, do not follow.
- `ListDirEntries` returns partial error mid-iteration → return partial tree (better than error).

### 2.9 Remove from `androidManager`

- Delete `getPhysicalDevices` method.
- `ListDevices` becomes emulator-only (no physical devices appended).
- `ListApps`, `DeleteApp`, `Boot`, `Shutdown` unchanged — they only operate on AVD names.

---

## Phase 3 — Graceful degradation

### 3.1 go-ios unavailable

`ios.ListDevices()` communicates via `/var/run/usbmuxd`. If the socket is absent, it returns
an error. `physicalIOSManager.ListDevices` returns `nil, nil` — treated as "zero devices
of this type", not a configuration failure.

### 3.2 ADB server unavailable

`adb.New()` dials `localhost:5037`. On connection-refused, `physicalAndroidManager.ListDevices`
returns `nil, nil`. The status bar is not polluted with an error; physical Android simply
doesn't appear.

### 3.3 Error classification

```go
var ErrToolUnavailable = errors.New("tool unavailable")

// Coordinator.Discover only propagates errors that indicate a configuration
// problem (wrong tool version, permission denied), not "nothing found".
if r.err != nil && !errors.Is(r.err, ErrToolUnavailable) {
    res.Errors = append(res.Errors, r.err)
}
```

### 3.4 `ToolVersioner`

```go
func (m *physicalIOSManager) ToolVersion(_ context.Context) (Platform, string) {
    return PlatformIOS, "go-ios " + goIOSVersion // from go.mod
}

func (m *physicalAndroidManager) ToolVersion(_ context.Context) (Platform, string) {
    client, err := adb.New()
    if err != nil { return PlatformAndroid, "adb (unavailable)" }
    v, _ := client.ServerVersion()
    return PlatformAndroid, fmt.Sprintf("adb-server %d (goadb)", v)
}
```

---

## File map

### New files

| File | Purpose |
|------|---------|
| `internal/device/ios_physical.go` | `physicalIOSManager` — go-ios backed |
| `internal/device/android_physical.go` | `physicalAndroidManager` — goadb backed |
| `internal/device/android_physical_files.go` | `AndroidPhysicalFileSystem` — goadb `ListDirEntries` |

### Modified files

| File | Change |
|------|--------|
| `internal/device/device.go` | Add `KindedManager` interface |
| `internal/device/ios.go` | Remove physical device code (xctrace, devicectl, streamPhysicalLogs) |
| `internal/device/android.go` | Remove `getPhysicalDevices`; `ListDevices` emulator-only |
| `internal/device/ios_files.go` | Remove `KindPhysical` guards |
| `internal/device/logs.go` | Kind-aware routing in `Coordinator.StreamLogs` |
| `internal/device/apps.go` | Kind-aware routing in `Coordinator.ListApps`, `DeleteApp` |
| `internal/device/info.go` | Kind-aware routing in `Coordinator.Info` |
| `internal/app/model.go` | Register new managers |
| `internal/app/commands.go` | `loadAndroidPhysicalFileTreeCmd`; bump timeouts for physical ops |
| `internal/app/update.go` | Handle `deviceConnectedMsg`/`deviceDisconnectedMsg`; Android file tree routing |
| `internal/app/messages.go` | Add `deviceConnectedMsg`, `deviceDisconnectedMsg` |
| `go.mod` / `go.sum` | Add go-ios, goadb |

---

## Tests

### Unit tests (no device required)

| File | Test | Covers |
|------|------|--------|
| `ios_physical_test.go` | `TestPhysicalIOS_LockedDevicePlaceholder` | `GetAllValues` error → placeholder device, no panic |
| `ios_physical_test.go` | `TestPhysicalIOS_ParseAppValues` | plist map → `App` struct (name fallback chain) |
| `android_physical_test.go` | `TestPhysicalAndroid_NoADBServer` | `adb.New` fails → `nil, nil` |
| `android_physical_test.go` | `TestPhysicalAndroid_UnauthorizedDevice` | `StateUnauthorized` → placeholder device |
| `android_physical_test.go` | `TestPhysicalAndroid_SkipsEmulators` | `!IsUsb()` serials excluded from physical list |
| `android_physical_test.go` | `TestBuildGoadbTree_MaxDepth` | Recursion stops at `MaxDepth` |
| `android_physical_test.go` | `TestBuildGoadbTree_PartialError` | Permission error mid-tree → partial result returned, no panic |
| `device_test.go` | `TestCoordinator_KindRouting` | Physical device routed to physical manager, not simulator manager |
| `device_test.go` | `TestCoordinator_FallbackToUnkinded` | No kinded manager → fall back to unkinded manager |
| `device_test.go` | `TestCoordinator_DeviceWatcher_OnConnect` | `CameOnline` event triggers refresh command |

### Integration tests (device required — manual / CI with device farm)

| Test | Requires |
|------|---------|
| `TestPhysicalIOS_ListDevices` | 1 trusted iOS device |
| `TestPhysicalIOS_ListApps_NonEmpty` | Developer mode enabled |
| `TestPhysicalIOS_DeleteApp` | Test app installed |
| `TestPhysicalAndroid_ListDevices_USB` | USB debugging enabled |
| `TestPhysicalAndroid_ListFiles_ExternalStorage` | Device unlocked |
| `TestPhysicalAndroid_ListApps_Count` | Any Android device |

---

## Rollout order

| Step | Work | Merge criteria |
|------|------|----------------|
| 1 | Add `KindedManager` interface + kind-aware coordinator routing | Unit tests green, no behaviour change |
| 2 | `physicalIOSManager` (go-ios) + remove xctrace/devicectl from `iosManager` | Physical iOS list/apps/logs working |
| 3 | `physicalAndroidManager` (goadb) — `ListDevices` + `ListApps` + `DeleteApp` | Physical Android list/apps working |
| 4 | `AndroidPhysicalFileSystem` (goadb `ListDirEntries`) + commands wiring | File tab shows `/storage/emulated/0` |
| 5 | `DeviceWatcher` integration — real-time connect/disconnect | Online box reacts without polling |
| 6 | Graceful degradation + `ToolVersioner` + error classification | Clean UX when tools absent |

Each step is independently mergeable with passing tests.

---

## Non-goals

- iOS app installation on physical device (requires signed IPA + provisioning profile).
- Android WiFi ADB pairing flow (QR / PIN exchange).
- Jailbroken device file access.
- Windows / Linux host support (macOS-only tool).
