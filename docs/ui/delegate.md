## 1. Context — what is a Bubbles list?

`charm.land/bubbles/v2/list` is a pre-built scrollable list component.
It handles keyboard navigation, filtering, pagination — but it
has **no idea what your data looks like**. You teach it by implementing 
two interfaces: `list.Item` and `list.ItemDelegate`.

---

### Item Wrapper (`Item` struct)
Wrap `device.Device` to satisfy `list.Item` interface.
- `FilterValue()`: Return device name for search/filter.
- `Title()`/`Description()`: Standard metadata for list components.

### Custom Renderer (`DeviceDelegate` struct)
Custom drawing logic for list rows.
- `Height()`/`Spacing()`: Set row size (1 line, no gap).
- `Render()`: Write colored string to `io.Writer`.
    - **Logic:** Check status (running = green dot) and platform (Android = ▲).
    - **Active Row:** Draw with `ColorAccent` background.
      Push version to right edge with spaces.
    - **Inactive Row:** Dim colors. Status dot show state.

### Global Styles (`NewListStyles` function)
Override default `bubbles/v2/list` look.
- Set background to `ColorBg` for empty states, pagination, and dividers.
- Customize search/filter prompt with `ColorAccent`.

### Why This Way?

- **Custom Look:** Standard list delegate is too generic.
  Manual `Render` allow "flexbox" layout (Name left, Version right).
- **Interface Satisfaction:** `Item` struct bridge custom domain 
  model (`device.Device`) to framework model (`list.Item`).
- **Styling:** Centralize list-wide aesthetics (pagination dots, search cursor) 
  in one constructor.

### Logic Flow

1. **Input:** List model + index + current item.
2. **Setup:** Get device data + terminal width.
3. **Selection:**
    - If cursor on this row → High contrast background + white text.
    - If not → Subtle colors.
4. **Layout:** Calculate gap based on string widths to align version to right wall.
5. **Output:** Formatted row printed to terminal buffer.

---

## 2. `Item` — wrapping your data (lines 17–21)

```go
type Item struct{ Dev device.Device }

func (i Item) FilterValue() string { return i.Dev.Name }
func (i Item) Title() string       { return i.Dev.Name }
func (i Item) Description() string { return string(i.Dev.Platform) + " " + i.Dev.Version }
```

`list.Item` is an interface.
The list doesn't accept `device.Device` directly — it only accepts values that
satisfy that interface.

`Item` wraps `device.Device` and implements the three required methods.

- `FilterValue()` — what text the list searches when the user types a filter
- `Title()` / `Description()` — used by the **default** delegate renderer (not used here since we write our own, but required by the interface)

**Why a wrapper instead of implementing the interface on `device.Device` directly?**  
`device.Device` lives in a separate package.
Adding UI-specific methods there would pollute the domain layer with UI concerns.
The wrapper keeps the boundary clean.

---

## 3. `DeviceDelegate` — the rendering contract (lines 24–28)

```go
type DeviceDelegate struct{}

func (d DeviceDelegate) Height() int                             { return 1 }
func (d DeviceDelegate) Spacing() int                           { return 0 }
func (d DeviceDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
```

`list.ItemDelegate` is an interface with four methods.
The list calls these to know how to lay out and render items:

| Method      | Purpose                                                  |
| ----------- | -------------------------------------------------------- |
| `Height()`  | How many terminal rows each item occupies                |
| `Spacing()` | Extra blank rows between items                           |
| `Update()`  | Handle messages per-item (we don't need it → return nil) |
| `Render()`  | Draw the item (implemented below)                        |

`DeviceDelegate{}` has no fields — it's a **zero-size struct**. 
It exists purely to carry the method set.

---

## 4. `Render` — the actual drawing (lines 30–82)

```go
func (d DeviceDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
```

The list calls this **once per visible item per frame**. It passes:
- `w` — an `io.Writer` to write the rendered row into
- `m` — the list model (use it to query width, current index)
- `index` — this item's position in the list
- `listItem` — the raw `list.Item` interface value

### Type assertion (lines 31–34)

```go
i, ok := listItem.(Item)
if !ok {
    return
}
```

`listItem` is an interface — you need a **type assertion** to get back your 
concrete `Item`.

The `ok` form is safe: if for some reason a non-`Item` ends up in the list,
`Render` silently skips it instead of panicking.

### Building the row

Same pattern as `sidebar.go`: dot color by status, platform glyph + color,
manual justify-between gap.
The selected row renders as one accent block (same single-block lesson from `tabs.go`). Non-selected renders each token styled independently.

### Writing output

```go
fmt.Fprint(w, lipgloss.NewStyle()...Render(...))
```

**`fmt.Fprint` not `return`** — because the signature gives you a writer,
not a return value. The list collects output from multiple delegates into one buffer.
You write into `w`; the list decides where it goes.

---

## 5. `NewListStyles` — theming the list widget (lines 85–118)

```go
func NewListStyles() list.Styles {
    s := list.DefaultStyles(true)
    s.StatusEmpty = lipgloss.NewStyle()...
    s.NoItems = ...
    s.PaginationStyle = ...
    ...
    return s
}
```

The Bubbles list has many built-in chrome elements (pagination dots, filter prompt,
"no items" text). 
`list.DefaultStyles` returns the default styles for all of them.
Here, each one gets overridden to match the app's color scheme — specifically,
all get `Background(ColorBg)` so nothing punches through to the terminal's 
default background.

Notable overrides:

```go
s.ActivePaginationDot = lipgloss.NewStyle()...SetString("•")
s.InactivePaginationDot = lipgloss.NewStyle()...SetString("•")
s.DividerDot = lipgloss.NewStyle()...SetString(" • ")
```

`SetString` bakes a fixed string into the style itself — the list renders these
styles without passing a string, so the glyph must be embedded in the style.

```go
s.Filter = textinput.DefaultStyles(true)
s.Filter.Focused.Prompt = lipgloss.NewStyle().Foreground(ColorAccent)
s.Filter.Cursor.Color = ColorAccent
```

The filter input (shown when user presses `/`) is itself a `textinput` component
with its own style tree.
You can drill into it and override just the prompt and cursor color.

---

## 6. Summary — the delegate pattern

```
list.Model
  │
  ├── holds []list.Item  (your Items)
  │
  ├── calls delegate.Height() / Spacing()  → layout math
  ├── calls delegate.Update(msg)           → per-item side effects
  └── calls delegate.Render(w, m, i, item) → draws each row
             │
             └── type-asserts list.Item → Item
                 reads Item.Dev
                 writes styled string to w
```

**Why this design?** The list component is generic and reusable.
By accepting an interface (`ItemDelegate`) instead of knowing about `device.Device`,
it can render any kind of data.

You own the rendering logic; the list owns scrolling, filtering, and pagination.


