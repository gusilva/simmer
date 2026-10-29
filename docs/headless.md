# Headless mode

Simmer is a TUI, but every read-only thing it shows you is also available as a command you can
pipe, grep and script. That is headless mode: no terminal UI, no keybindings — argv in, text or
JSON out, a meaningful exit code.

- [Choosing the mode](#choosing-the-mode)
- [Command-line shape](#command-line-shape)
- [Device references](#device-references)
- [Commands](#commands)
  - [`simmer help`](#simmer-help)
  - [`simmer devices`](#simmer-devices)
  - [`simmer apps`](#simmer-apps)
  - [`simmer db ls`](#simmer-db-ls)
  - [`simmer db query`](#simmer-db-query)
- [Output formats](#output-formats)
- [Exit codes](#exit-codes)
- [Warnings and errors](#warnings-and-errors)
- [How each platform is reached](#how-each-platform-is-reached)
- [Recipes](#recipes)
- [Troubleshooting](#troubleshooting)
- [Not available headlessly](#not-available-headlessly)

## Choosing the mode

The rule is one line: **a command word means headless, no command word means the TUI.**

```bash
simmer                 # opens the TUI
simmer devices         # prints a table and exits
```

Both modes do the same work underneath — the same device discovery, the same timeouts, the same
`~/.config/simmer/config.toml` — so a device that appears in the sidebar appears in `simmer devices`,
and a query that works in the DB viewer works in `simmer db query`.

## Command-line shape

```
simmer [global flags] <command> [subcommand] [command flags] [arguments]
```

Three ordering rules follow from Go's flag parsing, and all three bite if ignored:

**1. Global flags go before the command.**

```bash
simmer --log devices          # ✅ logging enabled, devices listed
simmer devices --log          # ❌ flag provided but not defined: -log
```

**2. Command flags go after the command**, and after the subcommand for `db`:

```bash
simmer db ls --device booted --app com.acme.app      # ✅
simmer --json db ls --device booted                  # ❌ --json is not a global flag
```

**3. Positional arguments go last.** `db query` takes its SQL as a positional argument, so every
flag must precede it:

```bash
simmer db query --device booted --db /p/app.db "select 1"   # ✅
simmer db query "select 1" --device booted --db /p/app.db   # ❌ flags after the SQL are ignored
```

Flags accept one or two dashes (`-json` and `--json` are the same flag) and booleans accept
`--json=false` if you need to override a wrapper script.

### Global flags

| Flag | Default | Meaning |
| --- | --- | --- |
| `--log` | off | Write every underlying `simctl`/`adb`/`go-ios`/`sqlite3` invocation to `./simmer.log` |
| `--log-level ERROR\|WARN\|INFO\|TRACE` | `INFO` | How much of that to record; `TRACE` includes full command output |
| `--pprof <addr>` | off | Serve `net/http/pprof` (development builds only) |
| `--heap-out <prefix>` | off | Write heap profiles on `SIGUSR1` and on exit (development builds only) |

### Flags accepted by every command

| Flag | Meaning |
| --- | --- |
| `--json` | Emit JSON instead of an aligned table |

## Device references

Every command except `devices` needs to know which device to talk to. A **device reference** is
whatever is most convenient to type:

| Form | Example | Notes |
| --- | --- | --- |
| UDID | `--device 3C30DCEC-5213-4254-8208-A39E5184B362` | iOS simulators and physical iPhones |
| adb serial | `--device emulator-5554` | What `adb devices` prints, for emulators and phones alike |
| AVD name | `--device pixel_8` | How Simmer lists an emulator; either this or its serial works |
| Full device name | `--device "iPhone 17 Pro"` | Quote it: names contain spaces |
| Unambiguous fragment | `--device 17pro` → ❌, `--device "17 Pro"` → ✅ | Substring of the name, case-insensitive |
| `booted` | `--device booted` | The one device that is currently running |
| *omitted* | `simmer apps` | Same as `booted` |

A running emulator answers to both its AVD name and its adb serial, so a script written against
`adb` can pass the serial straight through. `simmer devices --json` reports both (`id` and `serial`).

Resolution stops at the first level that matches: **exact identifier → exact name → name fragment.** So
`--device "iPhone 16"` picks the device actually called *iPhone 16*, even though *iPhone 16 Pro* and
*iPhone 16 Plus* also contain that text.

A reference that matches several devices is never resolved by guessing. It fails and names the
candidates, so you can narrow it:

```console
$ simmer apps --device "iPad Air"
error: "iPad Air" matches 3 devices by name fragment: iPad Air 13-inch (M3) (32D89768-…), iPad Air 11-inch (M3) (36AFEA53-…), iPad Air 11-inch (M3) (16CD88FB-…)
```

The same applies to the implicit `booted` reference when more than one device is running:

```console
$ simmer apps
error: 2 devices are running, name one explicitly: iPhone 16 (49960287-…), iPhone 17 Pro (3C30DCEC-…)
```

Two further messages you will meet:

```console
$ simmer apps                       # nothing is running
error: no running device: boot one, or name an offline device explicitly

$ simmer apps --device nope
error: no device matches "nope"
```

Offline devices are perfectly addressable by name — whether the operation then succeeds is up to the
underlying tool (listing apps on a shut-down simulator fails; `simmer devices` shows it fine).

## Commands

### `simmer help`

Prints the command summary. `simmer -h` and `simmer --help` do the same.

```bash
simmer help
```

### `simmer devices`

Lists every simulator, emulator and physical device Simmer can see. Both platforms are queried
concurrently, so one broken toolchain does not hide the other — it produces a warning on stderr
while the working platform still prints.

```
simmer devices [--json]
```

No device reference: this is the command that tells you what references exist.

```console
$ simmer devices
NAME                    PLATFORM  VERSION  STATUS    KIND     ID
pixel_8                 Android   35       Shutdown  virtual  pixel_8
pixel_9_pro             Android   35       Shutdown  virtual  pixel_9_pro
iPad (10th generation)  iOS       16.4     Shutdown  virtual  70CD6524-56EB-45B1-9AE0-A90B07BDD2FF
iPhone 16               iOS       18.5     Running   virtual  49960287-5E96-451C-B50B-C641B42EEE9D
iPhone 17 Pro           iOS       26.2     Shutdown  virtual  3C30DCEC-5213-4254-8208-A39E5184B362
```

| Column | Meaning |
| --- | --- |
| `NAME` | Device name (simulator name, AVD name, or the phone's name) |
| `PLATFORM` | `iOS` or `Android` |
| `VERSION` | OS version, or API level on Android |
| `STATUS` | `Running` or `Shutdown` |
| `KIND` | `virtual` (simulator/emulator) or `physical` (real hardware) |
| `ID` | UDID, adb serial, or AVD name — the unambiguous `--device` value |

JSON form:

```console
$ simmer devices --json
[
  {
    "id": "3C30DCEC-5213-4254-8208-A39E5184B362",
    "name": "iPhone 17 Pro",
    "platform": "iOS",
    "version": "26.2",
    "status": "Shutdown",
    "kind": "virtual"
  },
  {
    "id": "pixel_8",
    "name": "pixel_8",
    "platform": "Android",
    "version": "35",
    "status": "Running",
    "kind": "virtual",
    "serial": "emulator-5554"
  }
]
```

`serial` appears only when it differs from `id` — that is, for a running emulator. Both values are
accepted by `--device`.

### `simmer apps`

Lists the applications installed on one device.

```
simmer apps --device <ref> [--json] [--fast]
```

| Flag | Meaning |
| --- | --- |
| `--device <ref>` | Which device; omitted means the only running device |
| `--fast` | Print the bare list immediately, without version names or React Native marks |
| `--react-native=false` | Keep version names but skip React Native detection, saving a device round-trip per app |
| `--json` | Emit JSON |

```console
$ simmer apps --device "iPhone 17 Pro"
APP             BUNDLE ID                                VERSION          TYPE    MARKS
Calendar        com.apple.mobilecal                      1                System
Files           com.apple.DocumentsApp                   367.2.4          System
FieldKit        com.acme.fieldkit                        2.4.0            User    dev,rn
Maps            com.apple.Maps                           2911.32.10.2.15  System
```

| Column | Meaning |
| --- | --- |
| `APP` | Display name, falling back to bundle name, then bundle id |
| `BUNDLE ID` | iOS bundle identifier or Android package name — the `--app` value elsewhere |
| `VERSION` | Marketing version when known (`CFBundleShortVersionString`), otherwise the build version |
| `TYPE` | `User` or `System` |
| `MARKS` | `dev` for a Local app (a developer build), `rn` for a React Native app; both as `dev,rn` |

#### What Android reports

Android listing covers **third-party packages only** (`pm list packages -3 -f`): system packages are
not enumerated, so `TYPE` is always `User` and every row carries the `dev` mark. There is also no
display name in that output, so `APP` shows the package name — the same value as `BUNDLE ID`:

```console
$ simmer apps --device emulator-5554
APP                BUNDLE ID          VERSION  TYPE  MARKS
com.acme.fieldkit  com.acme.fieldkit  2.4.0    User  dev,rn
com.acme.dispatch  com.acme.dispatch  1.0.3    User  dev
```

iOS lists system apps as well, which is why `TYPE` is meaningful there.

#### Why `--fast` exists

How much a single device round-trip tells you differs by platform, and the slow extras are fetched
separately:

| Device | Apps come from | Version names | React Native detection |
| --- | --- | --- | --- |
| iOS simulator | one `simctl listapps` | included | inspected locally, included |
| Physical iOS | `go-ios` app list | included | needs a per-app AFC lookup |
| Android emulator | one `pm list packages -3 -f` | needs a per-app `dumpsys` call | needs a per-app read of the APK's entry list |
| Physical Android | one `pm list packages -3 -f` | needs a per-app `dumpsys` call | needs a per-app read of the APK's entry list |

By default headless output **waits for all of it**, because a script that sometimes gets a version
column is worse than a script that takes longer. The per-app calls run four at a time, and each
app's two questions are asked concurrently, so the wait stays short: a 56-app phone over USB takes
about 2 seconds.

Physical iOS is the exception and is still enriched one app at a time — its detection opens an AFC
session per app over the same USB transport, and that has not been measured under concurrency.

Two ways to pay less:

```bash
simmer apps --device emulator-5554 --react-native=false   # versions, no RN marks: one call per app
simmer apps --device emulator-5554 --fast                 # bundle ids only: no per-app calls at all
```

Rough costs on a 56-app physical Android device:

| | Time |
| --- | --- |
| `--fast` | 0.2 s |
| `--react-native=false` | 1.1 s |
| default (versions + React Native) | 2.3 s |

JSON form carries both the raw fields and the derived marks:

```console
$ simmer apps --device booted --json
[
  {
    "bundle_id": "com.acme.fieldkit",
    "label": "FieldKit",
    "version": "2.4.0",
    "short_version": "2.4.0",
    "type": "User",
    "local": true,
    "react_native": true,
    "path": "/Users/you/Library/Developer/CoreSimulator/Devices/…/FieldKit.app"
  }
]
```

| Field | Meaning |
| --- | --- |
| `local` | A Local app: a developer build rather than a store or system install |
| `react_native` | Detected React Native bundle (`main.jsbundle` / Hermes) |
| `path` | On-disk bundle path; omitted when the platform does not report one |

### `simmer db ls`

Lists the SQLite databases inside an app's sandbox. This command exists because **a database path
cannot be derived from a bundle id** — the sandbox has to be walked to find one, and `db query`
needs the path.

```
simmer db ls --device <ref> --app <bundle> [--json]
simmer db ls --device <ref> --path <dir>     [--json]
```

| Flag | Meaning |
| --- | --- |
| `--app <bundle>` | Search an app's own storage, by bundle id / package name |
| `--path <dir>` | Search an absolute device directory instead |
| `--device <ref>` | Which device; omitted means the only running device |
| `--json` | Emit JSON |

Exactly one of `--app` and `--path` is required: they name different trees, so there is no sensible
merge of the two.

Any file ending in `.db`, `.sqlite` or `.sqlite3` counts, at any depth in the sandbox. WAL and SHM
sidecars (`…-wal`, `…-shm`) are deliberately not listed: they are not separately queryable.

On Android an app owns **two** storage areas, and both are searched:

| Area | Path | Read as |
| --- | --- | --- |
| private | `/data/data/<pkg>` | the app, via `run-as` |
| external, app-specific | `/storage/emulated/0/Android/data/<pkg>` | the shell user |

They need different access routes — `run-as` is refused on the external path, and the shell user
cannot enter the private one — so a database is reachable regardless of which area it lives in, and
`db query` picks the route from the path you give it. Apps that keep their data under
`files/<account>/databases` are the common case for the external area.

#### Searching a directory instead

`--path` searches anywhere the device lets the shell user read, which is how shared storage is
covered: it belongs to no app, so no bundle id can name it.

```console
$ simmer db ls --device booted --path /storage/emulated/0/Download
DATABASE    BYTES   PATH
export.db   40960   /storage/emulated/0/Download/export.db
```

Scanning all of shared storage is viable — one `ls -laR`, about 1.5 s on a phone with 21 databases
across it:

```bash
simmer db ls --device booted --path /storage/emulated/0
```

What a path means follows the device: an Android path is a device path (`/storage/emulated/0`,
`/sdcard`, `/data/local/tmp`, or an app's own `/data/data/<pkg>/…`); an **iOS simulator** path is a
path on your Mac, since that is where a simulator's storage lives; a **physical iOS** path is a
media-partition path served over AFC (`/DCIM`, `/Downloads`), because a non-jailbroken device offers
no shell. Relative paths are rejected rather than resolved against whatever directory the shell
starts in.

A path inside an app's private storage is read as that app, with the package taken from the path
itself — so `--path /data/data/com.acme.app/databases` works without naming the app twice.

```console
$ simmer db ls --device booted --app com.acme.fieldkit
DATABASE      BYTES    PATH
cache.sqlite  1200384  /data/data/com.acme.fieldkit/cache.sqlite
rooms.db      90112    /data/data/com.acme.fieldkit/databases/rooms.db
```

An empty result is success, not an error — the app may simply keep no SQLite file:

```console
$ simmer db ls --device "iPhone 17 Pro" --app com.apple.mobilecal
DATABASE  BYTES  PATH
$ echo $?
0
```

JSON form:

```console
$ simmer db ls --device booted --app com.acme.fieldkit --json
[
  {
    "name": "rooms.db",
    "path": "/data/data/com.acme.fieldkit/databases/rooms.db",
    "size": 90112
  }
]
```

### `simmer db query`

Runs one SQL statement against a database on a device and prints the result set.

```
simmer db query --device <ref> --app <bundle> --db <path> "<SQL>" [--json]
```

| Flag | Required | Meaning |
| --- | --- | --- |
| `--db <path>` | yes | Path to the database, as reported by `simmer db ls` |
| `--device <ref>` | no | Which device; omitted means the only running device |
| `--app <bundle>` | no | The app whose sandbox owns the database (see below) |
| `--json` | no | Emit JSON |

The SQL is a positional argument and must come **after** all flags. Quote it so your shell keeps it
in one piece; multiple words are joined with spaces, so an unquoted statement usually still works,
but anything containing `*`, `;`, `(`, `'` or `"` will not survive.

```console
$ simmer db query --device booted --app com.acme.fieldkit \
    --db /data/data/com.acme.fieldkit/databases/rooms.db \
    "select id, name, synced_at from rooms order by id limit 3"
id  name       synced_at
1   Kitchen    2026-04-27T14:18:02Z
2   Workshop   2026-04-27T14:18:05Z
3   Yard       
```

The first row of the result is used as the header. A statement that matches no rows prints the header
alone (`[]` with `--json`); a statement that returns no result set at all — an `insert`, a `create
table` — prints nothing. Either way the exit code is `0`. Writes are not blocked — `insert`, `update` and `delete` run as readily as `select`, and print no
rows. Treat that power accordingly: this is a developer tool pointed at a developer device. Note the
one case where a write silently does nothing: a device without its own `sqlite3` binary is served
from a local copy (see [How each platform is reached](#how-each-platform-is-reached)).

#### Omitting `--app`

`--app` names the sandbox the database lives in, which is how a private file becomes reachable at
all. Omit it only when the path is readable without an app context:

- **Android**: with `--app`, the query runs as `run-as <package> sqlite3 …`; without it, `sqlite3`
  runs directly, which needs the path to be world-readable or the device to be rooted.
- **iOS simulator**: `--app` is not needed — the "device" is a directory on your Mac and the path is
  read directly by the host's `sqlite3`.
- **Physical iOS**: `--app` selects the `house_arrest` container to pull the file from; without it
  the pull goes through plain AFC (media-level access only).

JSON form turns each row into an object keyed by column name. Values are always strings, because
that is what the SQLite CLI returns — cast in SQL if you need numbers downstream:

```console
$ simmer db query --device booted --app com.acme.fieldkit \
    --db /data/data/com.acme.fieldkit/databases/rooms.db \
    "select count(*) as rooms from rooms" --json
[
  {
    "rooms": "42"
  }
]
```

#### Using the configured tables query

The DB viewer's per-database settings in `~/.config/simmer/config.toml` (script directory, custom
tables query) are read by headless mode too, keyed by the database's file name. A custom
`tables_query` therefore affects what the viewer's explorer lists; `db query` always runs exactly the
SQL you pass.

## Output formats

**Table** (default) is meant to be readable by eye and still cuttable by script: columns are padded
to their widest cell and separated by two spaces, there is no trailing whitespace, and no box
characters or colour are emitted. The header row is always present, even when there are no data
rows, so a consumer can tell "no rows" from "no output".

```bash
simmer devices | awk '$4 == "Running" { print $NF }'     # ids of running devices
```

**JSON** (`--json`) is a top-level array, pretty-printed with two-space indentation, with stable
snake_case keys. An empty result is `[]`, except for a statement that produces no result set at all,
which prints nothing. Use it whenever values can contain spaces — device names
and app labels usually do.

Warnings and errors never appear on stdout, so `--json` output is always valid JSON:

```bash
simmer apps --device booted --json 2>/dev/null | jq length
```

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | The command ran. An empty list or empty result set is still success. |
| `1` | The operation failed: unresolvable device reference, a tool that errored, a SQL error. |
| `2` | The command line was wrong: unknown command, missing required flag, unparseable flag. |

```bash
if ! simmer db query --device booted --app com.acme.app --db "$DB" "select 1" >/dev/null; then
  echo "device database unreachable" >&2
fi
```

## Warnings and errors

Discovery asks every platform at once, and a platform that cannot answer is a **warning**, not a
failure — the devices that did answer are still printed and the exit code is still `0`:

```console
$ simmer devices
warning: Android SDK not found; set ANDROID_HOME or ANDROID_SDK_ROOT
NAME           PLATFORM  VERSION  STATUS    KIND     ID
iPhone 17 Pro  iOS       26.2     Shutdown  virtual  3C30DCEC-…
```

An **error** is prefixed `error:`, goes to stderr, and sets a non-zero exit code. A malformed
`config.toml` is reported as a warning and factory defaults are used, so a bad config never blocks a
query.

## How each platform is reached

Useful when a command behaves differently than you expected, or when you are deciding what to
install on a CI machine:

| Device | Apps | File listing | SQLite |
| --- | --- | --- | --- |
| iOS simulator | `simctl listapps` | host filesystem under the device's data container | host `sqlite3` on the container path |
| Physical iOS | `go-ios` | AFC, or `house_arrest` for an app sandbox | file is copied to a temp dir (with its `-wal`/`-shm` sidecars) and queried by the host `sqlite3` |
| Android emulator | `adb shell pm list packages -3 -f`, `dumpsys package <pkg>`, `adb exec-out tail` over the APK's zip directory | `adb shell run-as <pkg> ls` | `adb shell run-as <pkg> sqlite3`, else a local copy |
| Physical Android | as above, over USB | `adb shell run-as <pkg> ls`, falling back to external storage for apps that refuse `run-as` | `adb shell run-as <pkg> sqlite3`, else a local copy |

Two consequences are worth spelling out.

**`run-as` needs a debuggable app, not a rooted device.** Any development build — including on real
hardware — exposes its private sandbox to `simmer db ls` and `simmer db query`. A release-signed app
refuses, and its file listing falls back to external storage; `--log` records the refusal.

**Android system images stopped shipping `sqlite3`** around Android 9, and most phones have none. When
the device cannot run a query itself, Simmer copies the database and its `-wal`/`-shm` sidecars to a
temp directory and queries it with your Mac's `sqlite3`, exactly as it already does for physical iOS.
Reads are current, because each query takes a fresh copy. **Writes are not: an `insert` or `update`
changes only the local copy and never reaches the device.** On an emulator, or a device that does
have `sqlite3`, queries run in place and writes do land.

Prerequisites follow from that table: Xcode command line tools for iOS, the Android SDK platform
tools (`adb`) for Android, and a `sqlite3` binary on your Mac. Physical iOS needs the device paired
and trusted.

## Recipes

```bash
# Every running device's id
simmer devices --json | jq -r '.[] | select(.status == "Running") | .id'

# Local (developer-built) apps on the booted device
simmer apps --json | jq -r '.[] | select(.local) | .bundle_id'

# React Native apps, across every running device
for id in $(simmer devices --json | jq -r '.[] | select(.status=="Running") | .id'); do
  simmer apps --device "$id" --json | jq -r --arg id "$id" \
    '.[] | select(.react_native) | "\($id) \(.bundle_id)"'
done

# Databases sitting in shared storage, where no bundle id can find them
simmer db ls --device booted --path /storage/emulated/0 --json | jq -r '.[].path'

# Biggest database in an app sandbox
simmer db ls --device booted --app com.acme.app --json \
  | jq -r 'max_by(.size) | .path'

# Dump a table to CSV
DB=$(simmer db ls --device booted --app com.acme.app --json | jq -r '.[0].path')
simmer db query --device booted --app com.acme.app --db "$DB" \
  "select * from rooms" --json | jq -r '.[] | [.id, .name] | @csv'

# Row counts for every table, using the app's own sandbox
for t in $(simmer db query --device booted --app com.acme.app --db "$DB" \
    "select name from sqlite_master where type='table'" --json | jq -r '.[].name'); do
  n=$(simmer db query --device booted --app com.acme.app --db "$DB" \
    "select count(*) as n from \"$t\"" --json | jq -r '.[0].n')
  printf '%-30s %s\n' "$t" "$n"
done

# Assert a migration ran, in CI
version=$(simmer db query --device booted --app com.acme.app --db "$DB" \
  "pragma user_version" --json | jq -r '.[0].user_version')
[ "$version" = "7" ] || { echo "schema is at $version, expected 7" >&2; exit 1; }
```

## Troubleshooting

Start by replaying the underlying tool invocations — `--log` records each one with its exit status,
and `--log-level TRACE` adds full output:

```bash
simmer --log --log-level TRACE db query --device booted --app com.acme.app --db "$DB" "select 1"
tail -40 simmer.log
```

| Symptom | Likely cause |
| --- | --- |
| `flag provided but not defined: -log` | A global flag was placed after the command; see [Command-line shape](#command-line-shape) |
| `no running device` | Nothing is booted, and no `--device` was given |
| `… matches N devices` | The reference is ambiguous; use the `ID` column from `simmer devices` |
| `no device matches "emulator-5554"` | Fixed — an emulator now resolves by serial as well as AVD name. On an older build, use the AVD name from `simmer devices`. |
| `simctl listapps …: exit status 149` | The target simulator is shut down |
| `run-as: package not debuggable` | A release-signed Android app does not permit sandbox access; file listing falls back to external storage |
| `db ls` lists external-storage paths, not `/data/data/…` | Same cause: `run-as` was refused for that package |
| `db ls` prints only a header | The app keeps no `.db`/`.sqlite`/`.sqlite3` file, or neither storage area could be read |
| `db ls` misses databases you can see in a file manager | Check `--log`: one of the two Android storage areas may have been refused; the other is still listed |
| `sqlite3: not found` reported by `--log` | Expected on most phones; the query is served from a local copy instead |
| `database is locked` | The app is mid-write; retry, or stop the app first |
| `apps` is slow on a phone | Check `--log` for one `adb exec-out` per app; a device with very large APKs pays more for React Native detection. `--react-native=false` removes that call. |

## Not available headlessly

Headless mode covers reading. State-changing operations are still TUI-only: booting and shutting
down devices, creating and deleting simulators and emulators, installing, launching, closing and
uninstalling apps, Rebuild-and-reinstall, app log streaming, and the React Native debugging menu.

They are not blocked by anything structural — they run through the same session the headless
commands use — so adding them is a matter of defining the command surface, not reworking the
plumbing.
