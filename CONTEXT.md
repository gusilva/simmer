# Context

Glossary of domain terms for Simmer (iOS/Android Device Manager TUI). No implementation details — see ADRs in `docs/adr/` for those.

## Terms

**Local app** — An app entry in the Apps panel whose install origin is a developer build rather than a store/system install. Detected on iOS Simulator via `ApplicationType == "User"` from `simctl listapps` (simulators have no App Store, so any User-type app is a dev build by construction); on Android emulator via non-system package (`pm list packages -3`). Distinct from `App.Type` ("User"/"System"), which only distinguishes system vs non-system, not install origin. Local apps carry the `[dev]` badge in the Apps panel and are the only apps eligible for the rebuild-and-reinstall shortcut.

**Rebuild-and-reinstall** — The "r" key shortcut in the Apps panel: build the local app's project from source, terminate the running instance, install the fresh build, and relaunch it on the currently selected simulator/emulator. Scoped to Local apps only; no-ops with a status message on System apps or physical devices.

**Project resolution** — The process of finding an app's build project on disk from its bundle-id/package-name alone, since Simmer stores no persisted per-app config. iOS: match `App.DisplayName`/`Name` against DerivedData folder name prefixes under `~/Library/Developer/Xcode/DerivedData/`, then read `WorkspacePath` from that folder's `info.plist`. Android: no automatic correlation exists (no DerivedData equivalent) — user is prompted for the project path on first rebuild of a given package. Resolution is cached in memory for the session only; not persisted across restarts.

**Session** — A single Simmer run's view of the devices it can reach: which devices were last discovered, how long an operation may take, and the user's configuration. Both ways of driving Simmer — the TUI and Headless mode — ask the same Session the same questions, so a device operation means the same thing in each.

**Headless mode** — Driving Simmer by naming a command instead of opening the TUI: listing devices, listing an app's databases, running a query. Headless output is meant to be read by a script as readily as by a person, so it is complete when printed rather than filled in progressively the way panels are.

**Device reference** — How a user names a device: its UDID or adb serial, its full name, an unambiguous fragment of its name, or "booted" for the one that is running. Naming nothing means "the only running device". A reference that matches several devices is never resolved by picking one — it is reported back with the candidates.

**File scope** — Which part of a device a file listing covers: the device's own filesystem, one app's own storage, or a named directory. The scope a user asks for is the same on every platform even where the access route is not; an app's storage on Android spans two areas reached two different ways, and a device that refuses one of them answers with whatever it can read.

**File filter / File search** — Two different ways of finding a file in the Files panel, kept distinct because the tree only knows the directories that have been expanded. A *filter* narrows the rows already on screen: instant, and blind to anything not yet listed. It keeps a match, the ancestors that place it, and everything beneath it — so a directory found by name can still be opened and read. A *search* asks the device itself, across every storage area the current tree covers, and answers with a flat list of paths. The filter names the search next to its input, so an empty filter result is never mistaken for a file that is not on the device.

**App database** — A SQLite file an app reads or writes, wherever it keeps it: inside its own storage, or in shared storage that belongs to no app. A database's path cannot be derived from a bundle id, so it is discovered by listing — an app's storage, or a named directory — before it can be queried.
