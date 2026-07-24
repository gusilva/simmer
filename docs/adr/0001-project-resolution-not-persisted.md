---
status: accepted
---

# Resolve local-app project paths live, don't persist a config file

Rebuild-and-reinstall needs a project path + build scheme for each local app, but Simmer has no config file anywhere in the codebase. Rather than introduce one (e.g. `.simmer.yaml` mapping bundle-id → project path), we resolve it fresh on each `r` press: iOS matches the app's display name against DerivedData folder names and reads `WorkspacePath` from `info.plist`; Android has no equivalent cache dir, so we prompt for the project path on first rebuild of a package. Resolution is cached in memory for the running session only.

**Considered:** a persisted bundle-id → project-path config, either hand-edited or written automatically on first resolution. Rejected for now to avoid introducing a config format and its associated staleness/invalidation problems (moved projects, cleaned DerivedData) before the live-resolution approach proves insufficient.

**Consequences:** every rebuild re-scans DerivedData on iOS, and Android re-prompts once per session per package (not once ever). If this becomes annoying, add persistence as a superseding decision rather than bolting caching on ad hoc.
