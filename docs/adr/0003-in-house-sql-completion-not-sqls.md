---
status: accepted
---

# SQL completion is built in-house from the schema Simmer already loads, not via sqls

The DB viewer's editor completes SQL from the database's own schema (tables and views, plus every column, independent of any configured tables query, loaded in one `sqlite_master` × `pragma_table_info` query when the database opens and reloaded with the explorer — one round trip, because each query on Android is an adb call and may mean copying the database off the device) plus a fixed list of SQLite keywords and functions. It does not use sqls-server/sqls: sqls is an LSP binary whose completer is not importable, and it needs its own live connection through `mattn/go-sqlite3` (cgo). An App database is usually not on the host — Android databases are reached through `adb shell run-as` and physical-iOS databases only as transient pulled copies — so sqls could only ever work on the simulator.

**Considered:** running sqls as a sidecar (simulator-only, adds a process and cgo build); forking its `internal/completer` (inherits an unstable, connection-oriented design). sqls's importable `parser`/`token` packages remain an option for working out the cursor's syntactic position.
