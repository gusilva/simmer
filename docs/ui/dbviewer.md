## `internal/ui/dbviewer/` — DB Viewer UI components

Four interactive panes inside the DB viewer overlay, plus a settings form.
Each is a plain Go struct with `Update` / `Rows` methods — no nested Bubble Tea programs.

---

## Overall layout

```
╭──────────────────────────────────────────────────────────────────────╮
│  myapp.db  ·  SQLite 3.45.1                                          │  ← title row
│──────────────────────────────────────────────────────────────────────│  ← title sep
│ [s] Schema        │ [q] Query  untitled-1.sql ●  F5 run  ^Enter all  │  ← pane heads
│───────────────────│───────────────────────────────────────────────── │
│ > Tables          │ SELECT * FROM users;                             │
│   ▸ users         │                                                  │  ← sidebar
│     id            │                                                  │     left
│     name          │                                                  │  ← query
│     email         │                                                  │     pane
│   ▸ sessions      │                                                  │     right-top
│   ▸ logs          │                                                  │
│                   │ ──────────────────────────────────────────────── │  ← h-divider
│                   │  id   name        email                          │
│                   │   1   Alice       alice@example.com              │  ← results
│                   │   2   Bob         bob@example.com                │     pane
│                   │                                                  │     right-bottom
│──────────────────────────────────────────────────────────────────────│  ← status sep
│  myapp.db  ·  42 rows · 3 cols · 12ms  ·  ⌘S save · ^S settings      │  ← status bar
╰──────────────────────────────────────────────────────────────────────╯
```

The sidebar is always 28 columns wide. The right pane takes the remainder.
The query pane gets `bodyH / 3` rows; results take the rest minus 1 (divider row).

---

## 1. Explorer (`explorer.go`, `explorer_data.go`)

Tree sidebar that lists schema objects grouped by type.

```
[s] Schema
──────────
▸ Tables                   ← group header (collapsed: ▸, expanded: ▼)
  ▼ users                  ← table node (expanded)
    id                     ← column node
    name
    email
  ▸ sessions
▸ Views
▸ Indexes
▸ Triggers
```

### State

```go
type Explorer struct {
    nodes    []*explorerNode
    cursor   int
    filter   string
    loading  bool
}

type explorerNode struct {
    kind     nodeKind   // group | table | col | trigger
    label    string     // display name (may be custom_table_name)
    realName string     // SQL identifier (always the name column value)
    icon     string
    children []*explorerNode
    expanded bool
    loading  bool
    indent   int
}
```

`label` and `realName` are separate to support custom display names.
When the user sets a custom `tables_query` that returns a `custom_table_name`
column, `label` shows that value while `realName` holds the underlying SQL
identifier used in `SELECT * FROM realName`.

`realNameOrLabel()` returns `realName` if non-empty, else `label`.

### Key bindings

| Key | Action |
|---|---|
| `↑` / `k` | Move cursor up |
| `↓` / `j` | Move cursor down |
| `enter` | Expand node / fetch columns async |
| `space` | Build `SELECT` and run query immediately |
| `/` | Enter filter mode |
| `ctrl+h` | (from modal) Collapse sidebar focus |

### Column loading

When a table node is expanded for the first time, `fetchColumnsCmd` fires
(`node.loading = true`). On `ColumnsLoadedMsg` the node's children are
populated and `expanded` set to `true`. Subsequent expansions reuse the
cached children.

### `filteredVisibleItems`

Returns a flat `[]explorerItem{row int, node *explorerNode}` for the current
filter and expansion state. The `cursor` index is into this flat list, not the
raw `nodes` slice. Scroll offset is derived on render as
`max(cursor - visibleRows + 1, 0)`.

---

## 2. Query pane (`query_pane.go`)

SQL editor above the horizontal divider.

```
[q] Query  untitled-1.sql ●  F5 run stmt  ^Enter run all  ⌘S save  ⇥ complete
──────────────────────────────────────────────────────────────────────────────
  1│ SELECT *
  2│ FROM users
  3│ WHERE id = 1;
  4│
  5│ SELECT count(*) FROM sessions;
```

### State

```go
type QueryPane struct {
    editor   textarea.Model
    filePath string        // default: "untitled-1.sql"
    state    fileState     // New | Clean | Dirty
}

type fileState int  // fileStateNew | fileStateClean | fileStateDirty
```

`fileState` drives the dot indicator on the tab:
- `●` red — file does not exist on disk yet
- `●` green — file matches disk
- `●` yellow — unsaved local changes

On startup, `newQueryPane` tries to read `untitled-1.sql` from the current
directory. If found, content is loaded and state is `fileStateClean`; if
missing, state is `fileStateNew`.

### Statement at cursor (`StatementUnderCursor`)

```go
func (p QueryPane) StatementUnderCursor() string {
    return statementAtLine(p.editor.Value(), p.editor.Line())
}
```

`statementAtLine` scans backward from the cursor line to find the previous `;`
(start boundary), then forward to find the next `;` (end boundary), and returns
the trimmed joined lines. Cursor on a blank line between statements returns the
next statement.

This is what `F5` runs. `Ctrl+Enter` runs `editor.Value()` (full content).

### File save

`SaveCmd()` captures `editor.Value()` and `filePath` into a closure; the file
write happens in a goroutine. `FileSavedMsg{Err}` is returned. On success the
state transitions to `fileStateClean`.

---

## 3. Results pane (`results_pane.go`)

Tabular result display below the horizontal divider, backed by `bubbles/table`.

```
  id   name        email
   1   Alice       alice@example.com
   2   Bob         bob@example.com
  ─────────────────────────────────
  (loading…)  /  (no results)  /  error message
```

### States

`ResultsPane` renders differently based on which setter was called last:

| Setter | Display |
|---|---|
| `SetLoading()` | Spinner / "Loading…" row |
| `SetResults(rows)` | `bubbles/table` with header + data rows |
| `SetError(err)` | Red error message row |

Column widths are auto-distributed: total table width divided equally, minimum 6
cells per column.

### Mouse scroll

The modal's `handleMouseWheel` calls `tbl.MoveDown(3)` / `tbl.MoveUp(3)`
directly on the inner table when the wheel event lands below `l.DivRow`.

---

## 4. Settings form (`settings.go`)

Rendered by the modal **instead of** the normal three-pane body when `settingsOpen == true`.

```
Settings
────────────────────────────────────────────────────────────
Script Path
╭────────────────────────────────────────────────────────╮
│ ./                                                     │
╰────────────────────────────────────────────────────────╯

Tables List Query  SQL
╭────────────────────────────────────────────────────────╮
│ SELECT type, name FROM sqlite_master…                  │
│                                                        │
│                                                        │
╰────────────────────────────────────────────────────────╯


                                      [ Cancel ]  [ Save ]
Tab next  ⌘S save  Esc cancel
```

Tab order: Script Path → Tables List Query → Cancel → Save → (wraps).

The active field's label and border are rendered in accent color.
Inactive fields use dim foreground.

`Rows(width, height)` pushes Cancel/Save to `height - 2` regardless of how
many rows the fields occupy, so buttons are always near the bottom.

### Saving

On `SettingsSavedMsg` the modal sets `settingsOpen = false` and fires a new
`fetchTablesCmd` so the sidebar immediately reflects the updated query — no
manual refresh needed.

If the user clears the Tables List Query field and saves, `EffectiveTablesQuery()`
falls back to `DefaultTablesQuery` automatically.

---

## 5. Status bar (`modal_view.go`)

Single row at the bottom of the modal.

```
myapp.db  ·  42 rows · 3 cols · 12ms  ·  ⌘S save · ^S settings
```

Left side: database name + last query stats (rows, cols, elapsed).
Right side: keyboard hints.

`formatDuration` compacts the `time.Duration`:
- < 1 ms → `"<1ms"`
- < 1 s  → `"Xms"`
- ≥ 1 s  → `"X.Xs"`

Stats only appear after the first successful query (`lastStats.hasData == true`).

---

## 6. `render_helpers.go` — low-level primitives

```go
type renderHelpers struct{}

func (r renderHelpers) BlankN(n int) string       // n spaces with bg color
func (r renderHelpers) FillTo(s string, n int) string  // pad/truncate to exactly n cells
func (r renderHelpers) Sep(n int) string          // full-width ─ separator
```

`FillTo` uses `lipgloss.Width` for ANSI-aware measurement. It pads with spaces
when the visual width is short, and does nothing when it is already at `n`
(overflow is not truncated — callers are expected to size correctly).

All renderers (`Explorer.Rows`, `QueryPane.Rows`, `ResultsPane.Rows`,
`Settings.Rows`) call `FillTo` on every line so the compositor can safely
overlay columns without re-measuring.

---

## Why this way?

**`Rows(width, height) []string` instead of `View() string`:**
The modal compositor needs to stitch panes horizontally (sidebar left, right pane
right). Splitting on `"\n"` after the fact is fragile with ANSI codes. Receiving
a pre-sized `[]string` makes column stitching a simple index operation.

**No bubbles/list for the explorer tree:** `bubbles/list` assumes a flat list
with optional filtering. The tree needs arbitrary depth, lazy column loading,
type-specific icons, and expansion state. A custom struct is simpler than
bending the list component to fit.

**`bubbles/table` for results:** Results are always a flat 2D grid — exactly
what `bubbles/table` is built for. No custom rendering needed.

**Inline settings vs compositor overlay:** The settings form replaces the body
content rather than floating above it. This avoids the Lip Gloss compositor
API and keeps the z-order logic trivial: one boolean flag.
