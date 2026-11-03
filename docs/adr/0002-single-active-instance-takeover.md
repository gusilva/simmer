---
status: accepted
---

# One Active instance, unconditional takeover, coordinated through the state file

Simmer resumes the View state the user left, and several terminals may run it at once. Every TUI launch takes over unconditionally — no prompt, no liveness check of the previous owner — so a crashed instance can never block the next one. Coordination lives entirely in a machine-owned state file under `~/.local/state/simmer/`, kept apart from the hand-edited `config.toml`: the file carries the View state plus an owner token, every read and write happens under an `flock` on a sibling lock file, and the owner re-checks the token before each save and once a second, so a superseded instance notices within a second and can never overwrite the new owner's state. Headless mode never reads, writes, or claims it.

**Considered:** a unix-socket handoff where the new instance asks the owner to flush and waits (exact, but needs a server, timeouts, and a fallback for a hung owner — the file is already at most a short debounce behind); signalling the owner's pid (`SIGUSR1` is already taken by `--heap-out`); fsnotify (a new dependency for what a one-second stat does).

**Consequences:** this does not supersede ADR-0001 — rebuild project paths and bundler ports stay in memory and are not part of the View state. A new instance may resume a state up to one debounce interval old.
