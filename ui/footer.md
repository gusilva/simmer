## Structure (`FooterParams` struct)
Data for render:
- `Width`: terminal width.
- `Status`: message string (e.g., "Device booted").
- `Kind`: enum for status color (`Ok`, `Warn`, `Err`, `Info`).

## 1. Flow (`RenderFooter` function)

```
 ↑↓/jk select  ←→/hl pane  b boot  s shutdown  1-4 tab  space load device  ? help  q quit      ✓ app loaded 
```

Left: keyboard hints. Right: transient status message (last action result).


### 1. Build Left Side (Hints)
Loop through `footerHints` slice.
- Format each as `[Key] [Verb]` (e.g., `q quit`).
- Apply `keyStyle` (bold) and `verbStyle` (dim).
- Join with double space separator.

### 2. Build Right Side (Status)
Check if `p.Status` is empty.
- If set, call `statusColor()` to pick foreground: Green (`Ok`), Yellow (`Warn`), Red (`Err`).
- Wrap message in `lipgloss` style.

### 3. Layout & Render
- Calculate `gap`: `innerWidth - len(left) - len(right)`.
- Use `strings.Repeat(" ", gap)` to push status message to far right.
- Wrap result in `StyleFooter`.

## Why This Way?

- **Stateless/Pure:** Function take data, return string. No internal state.
- **Visual Hierarchy:** Bold keys vs dim verbs help user scan shortcuts fast.
- **Transient Feedback:** Right side used for "last action" results. Color indicate success/failure at glance.

## Logic Mapping

1. **Input:** Terminal width + status message.
2. **Left:** `↑↓/jk select  ←→/hl pane  b boot ...`
3. **Right:** `Device booted` (Green if `StatusOk`).
4. **Gap:** Push Right to edge.
5. **Output:** Single row colored string.

---

## 2. `StatusKind` — typed enum (lines 11–22)

```go
type StatusKind int

const (
    StatusInfo StatusKind = iota  // neutral
    StatusOk                       // green
    StatusWarn                     // yellow
    StatusErr                      // red
)
```

**Why a named type instead of plain `int`?**

Without it, the caller could pass any integer accidentally:
```go
RenderFooter(FooterParams{Kind: 99})  // compiles, wrong
```

With `StatusKind`, the type system documents the valid values. `statusColor()` has a `default` fallback anyway, but the named type makes intent clear at the call site.

---

## 3. `footerHints` — package-level slice (lines 31–45)

```go
var footerHints = []struct{ key, verb string }{
    {"↑↓/jk", "select"},
    {"←→/hl", "pane"},
    {"b", "boot"},
    ...
}
```

**Anonymous struct slice** — no need to name the type when it's only used here.
Keeps key/verb pairs together, easy to add or reorder.

Declared at package level (not inside the function) so it's allocated **once** at 
startup, not on every render call. The footer re-renders every frame, so avoiding
allocations here matters.

The commented-out entries show planned features — left in as a reminder without 
cluttering the live hints.

---

## 4. `RenderFooter` — building the hints (lines 57–61)

```go
var hintParts []string
for _, h := range footerHints {
    hintParts = append(hintParts, keyStyle.Render(h.key) + verbStyle.Render(" "+h.verb))
}
left := strings.Join(hintParts, StyleFaint.Render("  "))
```

Each hint is two styled tokens concatenated: **bold key** + **dim verb**.
All hints joined with a faint double-space separator.

**Why `strings.Join` instead of building in the loop?**  
`Join` puts the separator *between* items, never trailing.
A loop with `+= sep + item` would need an `if i > 0` guard.
`Join` handles that cleanly.

---

## 5. The status message (lines 63–69)

```go
right := ""
if p.Status != "" {
    right = lipgloss.NewStyle().
        Foreground(statusColor(p.Kind)).
        Background(ColorBg).
        Render(p.Status)
}
```

Status is **optional** — empty string means nothing on the right.
`statusColor` maps the enum to an actual `color.Color` value.
The parent sets `Status` after an action (e.g. "app loaded", "boot failed") and
clears it after a timeout.

---

## 6. `statusColor` (lines 77–88)

```go
func statusColor(k StatusKind) color.Color {
    switch k {
    case StatusOk:   return ColorOk
    case StatusWarn:  return ColorWarn
    case StatusErr:   return ColorErr
    default:          return ColorFgDim
    }
}
```

Maps enum → color. 
`default` handles `StatusInfo` and any unknown value — defensive but simple.
Extracted as its own function so `RenderFooter` stays focused on layout,
not color logic.

---

## 7. Gap + final render (lines 71–74)

Identical pattern to `topbar.go`:

```go
innerWidth := p.Width - 2
gap := max(innerWidth - lipgloss.Width(left) - lipgloss.Width(right), 0)
return StyleFooter.Width(p.Width).Render(left + strings.Repeat(" ", gap) + right)
```

Same `lipgloss.Width` for ANSI-safe measurement, same `max(..., 0)` guard,
same `StyleFooter.Width(p.Width)` to fill the terminal width with background color.

---

## Summary — comparison with topbar

|                  | `topbar.go`                   | `footer.go`                              |
| ---------------- | ----------------------------- | ---------------------------------------- |
| Pattern          | Pure function + params struct | Same                                     |
| Left content     | Brand + live counts           | Static keyboard hints                    |
| Right content    | Tool versions (optional)      | Status message (optional)                |
| Extra concept    | —                             | `StatusKind` typed enum + color dispatch |
| State            | None                          | None                                     |
| `Update` needed? | No                            | No                                       |

Both files show the same lesson: **when a UI element has no interaction, skip the component pattern entirely — a plain function is simpler and easier to test.**
