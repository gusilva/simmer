# Changelog

## Unreleased

### Added

- **SQL editor with vim keys.** The SQLite viewer's query pane is now a
  real editor (goeditor) with normal, insert, visual, command and search
  modes. The footer badge shows the current mode. Vim mode is on by default
  and is a global setting: turn it off with "Vim keys in the query editor" in
  the settings form (`ctrl+s` outside the editor), or `vim_mode = false` in
  `config.toml`. With it off the editor is always inserting.
  - `:w` saves the script, `:w name.sql` saves it under a new name in the
    database's script directory, and `:q` / `:wq` close the viewer.
  - Esc steps out: insert → normal → explorer. A half-typed command such as
    `d` is cancelled on the way out, so it never completes on your next key.
  - A terminal paste goes in verbatim at the cursor as one undo step, in any
    mode.
- **Syntax highlighting** for SQL in the editor, in Simmer's own colours.
- **SQL completion** from the open database. The menu opens as you type (or
  on `ctrl+space`); `ctrl+n` / `ctrl+p` move, `tab` / `enter` accept, and
  `esc` closes the menu before anything else. What it offers depends on where
  the cursor is:
  - after `alias.` or `table.`: that table's columns, even when the alias is
    defined later in the statement;
  - after `FROM`, `JOIN`, `INTO`, `UPDATE`: tables and views;
  - elsewhere: the statement's columns, SQL functions and keywords, in the
    case you are typing.

  Tables a custom tables query gives a display name (`custom_table_name`)
  are listed and matched by that name, and the real table name is inserted,
  quoted when needed. Columns come from a single query when the database
  opens. On SQLite older than 3.16 completion falls back to table names.

### Changed

- **F5 runs the statement under the cursor exactly.** Statements are split
  the way SQLite reads them: a `;` inside a string, a quoted identifier
  (`"…"`, `` `…` ``, `[…]`) or a comment no longer ends a statement, and a
  `CREATE TRIGGER … BEGIN … END` body runs as one. The cursor's column counts
  too, so two statements on one line run separately, and a cursor just after
  a `;` runs the statement it ends.
- **`config.toml` follows `XDG_CONFIG_HOME`** when it is set
  (`$XDG_CONFIG_HOME/simmer/config.toml`), as the saved view already follows
  `XDG_STATE_HOME`. If you set `XDG_CONFIG_HOME` to anything other than
  `~/.config`, move your config there.
- Building from source needs Go 1.26.6 or later.

### Fixed

- Typing `q` in the query editor closed the viewer, and so did Esc.
- Tables and the completion schema no longer go missing when they arrive
  while the save-as prompt is open.

## v1.9.1 — 2026-11-03

### Added

- **herdr plugin.** Simmer can open as a popup inside
  [herdr](https://herdr.dev) (0.9.3 or later). Run
  `herdr plugin install gusilva/simmer`, then bind a key to the
  `gusilva.simmer.open` action. The plugin runs the `simmer` binary on your
  `PATH`, so nothing is built at install time and Go is not required. Closing
  the popup closes Simmer. The next popup takes over and resumes the last view,
  as in ADR-0002. See "Use it as a herdr plugin" in the README.

## v1.9.0 — 2026-11-03

### Added

- **Resume the last view** (#24). The TUI reopens on the device, panel, app and
  database you left, recorded under `~/.local/state/simmer/`. `--fresh` skips it.
- **Live indicator and takeover.** Each launch takes over from any Simmer open in
  another terminal; the top bar (and the SQLite viewer title) shows `● live` or
  `○ not live · ^T take over`. `ctrl+t` or a click takes back control. See
  `docs/adr/0002-single-active-instance-takeover.md`.
- **Minimum terminal size.** Below 80×24 the TUI shows the current and needed
  size instead of a wrapped or overflowing layout; only `q` / `ctrl+c` act
  until the window is big enough. This also stops the SQLite viewer panicking
  on terminals under 13 rows.

### Fixed

- The SQLite viewer title row no longer pushes the right border out when the
  database name is long.
- The Files preview (and the file-search preview) no longer wraps a long line
  into the sidebar and shifts the rows below it; the device header truncates a
  long device name instead of wrapping. Every main-pane row is now clipped to
  the pane width.

## v1.8.0 — 2026-11-01

### Added

- **Headless mode.** Running `simmer` with a command instead of no arguments
  skips the TUI and prints to stdout: `devices`, `apps`, `db ls`, `db query`.
  Every command takes `--json`; exit status is `1` for a failed operation and
  `2` for a command-line error. Device orchestration moved behind a session
  module that imports no UI library, so the TUI and the commands are two
  adapters over one implementation rather than two copies. See
  [docs/headless.md](docs/headless.md). (`97a6a60`)
- **Device references.** `--device` accepts a UDID, an adb serial, an AVD
  name, an unambiguous fragment of a name, or `booted`. Omitting it uses the
  only running device; a reference matching several is reported with the
  candidates rather than resolved arbitrarily. A running emulator answers to
  both its AVD name and its adb serial.
- **File search in the TUI.** `/` filters the rows already on screen and
  `ctrl+f` takes the same query to the device, searching everything the tree
  covers — so a file in a directory never opened is still found. Enter on a
  result opens a database or moves the tree to it.
- **Lazy directory expansion.** Directories load as they are opened, so
  shared storage is reachable to any depth instead of stopping at the initial
  walk's limit.
- **Device-backed test suite.** `make check-device` exercises attached
  simulators, emulators and phones; each test skips when its device is
  absent. (`5fa915d`)

### Fixed

- **Physical Android: an app's databases were invisible.** Sandbox requests
  were answered with external storage, on the assumption that `run-as` needs
  a rooted device. It needs the app to be debuggable, which every development
  build is. An app's storage spans its private data dir *and* its external
  app directory, and both are now listed. (`bdd2baf`)
- **Physical Android: file and database operations failed outright.** Device
  lookup matched only serials beginning with `emulator-`, so every path
  through it refused real hardware.
- **Android devices without `sqlite3` can be queried.** Images have shipped
  without the binary since Android 9. The database and its WAL sidecars are
  copied to this machine and queried locally. Reads are current; **writes
  reach only the copy**, and devices that do have `sqlite3` still query in
  place.
- **A private-storage path is read as its owning app** whether or not
  `--app` names it, so `db ls` and `db query` no longer disagree about the
  same file.

### Performance

- **React Native detection stopped pulling whole APKs**, reading the APK's
  zip central directory off the device instead. On a 56-app phone: 5.3 GB
  over USB became ~20 MB, and `simmer apps` went from 4m25s to 2.3s.
  `--react-native=false` skips the check entirely.
- Benchmarks for the app and device layers, CPU and trace profiling flags,
  and `scripts/perf.sh`. See
  [docs/architecture/performance.md](docs/architecture/performance.md).
  (`c9191bc`, `28e5ad1`)

### Internal

- Capability routing collapsed from 29 hand-written loops into three named
  rules; `Boot` had come to ignore device kind while `ListApps` honoured it.
- CI gates files excluded from the build by their filename (a `_android.go`
  suffix compiles nowhere while every other check stays green) and code
  unreachable from both the binary and the tests. (`5fa915d`)
- ~1,300 lines of unreachable UI code removed, including a SQLite modal the
  database viewer had replaced.

## v1.7.1 — 2026-09-27

### Added

- **React Native detection badge.** Apps in the Apps tab now show an `⚛` icon
  when they're a React Native app: iOS simulator checks the local bundle for
  `main.jsbundle`/a Hermes framework, physical iOS checks for `RCT`-prefixed
  keys in the app's preferences plist, and Android (emulator + physical)
  inspects the installed APK. (`cfbf29c`, `1c353b3`)
- **React Native debugging menu.** Press `m` on an app in the Apps tab to open
  a menu of RN-specific actions, greyed out for non-RN apps:
  - **Configure Bundler Location** points a running app at a different Metro
    server (`host:port`) and relaunches it to apply. The default host is
    platform-aware — `localhost` for simulators, `10.0.2.2` for Android
    emulators, this machine's detected LAN IP for physical devices.
  - **Dev Menu (Trigger)** opens the RN dev menu without shaking the device:
    `osascript` on iOS simulator, an `adb` keyevent on Android, and a direct
    WebSocket broadcast to Metro on physical iOS (the only mechanism that
    reaches real hardware, since there's no way to simulate a shake
    gesture remotely there).
  - **Log to File** captures a fresh app session's console output to a file
    you choose, reusing the existing log-streaming pipeline for
    simulators/emulators/physical Android; physical iOS instead uses
    `devicectl --console`, which captures the app's actual stdout (where
    Hermes/`console.log` output lands) rather than filtering the noisier
    system log. (`c4ad618`)

### Fixed

- Android version fetch scraped one `dumpsys package packages` blob per
  device, whose format silently drops entries across Android
  versions/vendor skins; switched to one scoped `dumpsys package <id>` call
  per app. (`1c353b3`)
- App enrichment (versions + RN detection) fired one command per app at once,
  flooding a physical device's single USB transport and failing silently at
  random; replaced with a bounded worker pool that streams results back as
  they land. (`1c353b3`)
- Android RN detection only recognized `assets/index.android.bundle`, missing
  Expo/custom Metro configs that ship the bundle under other names; broadened
  to any `assets/*.bundle` entry. Also broadened to match any Hermes
  framework name (e.g. `hermesvm.framework`), not just the legacy
  `hermes.framework`, which was silently missing newer builds. (`1c353b3`,
  `c4ad618`)
- Physical iOS RN detection read `main.jsbundle`/`hermes.framework` via
  AFC/house_arrest, which only vends the app's data container and never
  finds the bundle — confirmed dead code against a real device. Replaced
  with a check for `RCT`-prefixed keys in the app's NSUserDefaults plist,
  which is reachable and always written by React Native's iOS modules.
  (`1c353b3`)

## v1.7.0 — 2026-09-20

### Added

- **Stacked-panel main pane.** Replaces the Info/Apps/Files tabs with two
  stacked panels (Apps, Files) — one expanded, the other a one-line "[N]
  Label" strip. `2`/`3` toggle which is expanded; `esc` unwinds one level at
  a time. Apps is expanded by default on device load so the list shows
  immediately, and device info moves from a tab to a right-side overlay
  opened with `i` (`c` copies the UDID, `r` reveals it in Finder for local
  devices). Install app rebinds from `i` to `a`. (`8264357`)
- **Launch and close apps.** `enter` launches the selected app on the active
  device; `c` opens a confirm modal before terminating it, matching the
  Delete flow. Android resolves the launcher activity via `cmd package
  resolve-activity`, since `App` only stores the package name. (`ea3b1b7`)
- **Mouse click-to-focus and click-to-select.** Clicking a panel (sidebar,
  Apps, Files, DB-viewer explorer/tables) moves focus there and selects the
  clicked row, matching arrow-key navigation. Simple alert modals
  (delete/delete-app/close-app) dismiss on an outside click. (`df11949`)
- **Double-click action menu.** Double-clicking a selected sidebar device,
  Apps row, or Files row opens a menu of its available actions
  (boot/shutdown/delete, launch/close/rebuild, expand/open db) instead of
  requiring the per-row key shortcut. Each menu item runs the exact same
  message/command its key binding already does. (`33b678f`)

### Fixed

- Sidebar row hit-testing was off by one below the iOS/Android group
  separator, so clicking selected the device one row below the one clicked.
  (`df11949`)
- DB viewer mouse click/wheel handling hardcoded the modal's screen offset,
  so every click resolved one row/column off from where it landed.
  (`df11949`)

## v1.6.0 — 2026-09-01

### Added

- **File-based app logging.** Press `l` on an app in the Apps tab to stream its
  logs to `<AppLabel>.log` in the directory simmer was launched from (e.g.
  `Calendar.log`). Press `l` again to stop. The file is written with a header
  line identifying the app, device, and start time, then raw log lines; each
  batch is `fsync`'d so a hard crash keeps what was already written. Logging
  stops and flushes on quit, on a panic inside a command, on device shutdown,
  and on app deletion. The app being logged shows a red `[logging]` badge next
  to `[dev]`. (`1d6afda`)
- **Rebuild & reinstall shortcut.** Press `r` on a locally-built app (User
  type, simulator/emulator only) to terminate, rebuild, reinstall, and relaunch
  it, reusing the existing build-stream install pipeline. iOS resolves the
  Xcode project automatically via DerivedData; Android prompts once per session
  and caches the chosen project directory. Local apps are marked with a `[dev]`
  badge. (`8ccef5b`)

### Changed

- **Logs tab removed.** Log viewing moved out of a dedicated tab into the file
  logging above. Tabs renumber to `[1] Info  [2] Apps  [3] Files`. (`1d6afda`)
- **`space` in the Apps tab no longer starts a log stream.** It now only pins
  an app for Files-tab sandbox browsing, marked with a cyan `⊙`. (`1d6afda`)

### Fixed

- iOS project resolution for the rebuild flow always failed because
  `plutil -convert json` rejects `info.plist`'s `LastAccessedDate` field. Reads
  the plist with `plutil -p` and parses `WorkspacePath` directly. (`1dc0df3`)
- Rebuild project-resolution and Xcode scheme-fetch failures were only shown
  briefly in the status bar and never written to `simmer.log`. Adds
  `Logger.LogError` for non-exec failures and wires it into both flows.
  (`f4d4489`)
- The `i` (install) and `d` (delete) shortcuts were missing from the `?` help
  overlay in the Apps tab. Both are now declared bindings and shown. (`124c52c`)
