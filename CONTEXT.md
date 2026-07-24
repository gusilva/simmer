# Context

Glossary of domain terms for Simmer (iOS/Android Device Manager TUI). No implementation details — see ADRs in `docs/adr/` for those.

## Terms

**Local app** — An app entry in the Apps panel whose install origin is a developer build rather than a store/system install. Detected on iOS Simulator via `ApplicationType == "User"` from `simctl listapps` (simulators have no App Store, so any User-type app is a dev build by construction); on Android emulator via non-system package (`pm list packages -3`). Distinct from `App.Type` ("User"/"System"), which only distinguishes system vs non-system, not install origin. Local apps carry the `[dev]` badge in the Apps panel and are the only apps eligible for the rebuild-and-reinstall shortcut.

**Rebuild-and-reinstall** — The "r" key shortcut in the Apps panel: build the local app's project from source, terminate the running instance, install the fresh build, and relaunch it on the currently selected simulator/emulator. Scoped to Local apps only; no-ops with a status message on System apps or physical devices.

**Project resolution** — The process of finding an app's build project on disk from its bundle-id/package-name alone, since Simmer stores no persisted per-app config. iOS: match `App.DisplayName`/`Name` against DerivedData folder name prefixes under `~/Library/Developer/Xcode/DerivedData/`, then read `WorkspacePath` from that folder's `info.plist`. Android: no automatic correlation exists (no DerivedData equivalent) — user is prompted for the project path on first rebuild of a given package. Resolution is cached in memory for the session only; not persisted across restarts.
