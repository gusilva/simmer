## Structure (`Tab` struct)
Container for tab data:
- `Key`: keyboard shortcut (e.g., "1").
- `Label`: display name (e.g., "Info").

```
 [1] Info │[2] Apps │[3] Logs │[4] Files              
          ↑ active: cyan block, bold, padded
```

Horizontal strip of tabs. Active tab has accent background. Inactive tabs are dimmed. `│` separates adjacent tabs.

## 1. Flow (`RenderTabs` function)

### 1. Style Setup
Define looks for active vs inactive tabs:
- `activeStyle`: Bold, cyan background (`ColorAccent2`), black text (`ColorBg`), inner padding.
- `inactiveStyle`: Dim foreground, standard background.
- `keyStyleInactive`: Even dimmer color for shortcut brackets `[1]`.

### 2. Building Strip
Loop through `tabs` slice:
- **Active Tab:** Render label + key as single block with accent background. Single style usage ensure background color fill entire padding area.
- **Inactive Tab:** Nest `keyStyleInactive` inside `inactiveStyle`. Shortcut look more faint than label.
- **Separator:** Insert `│` between tabs (except after last).

### 3. Padding
- Calc current string width with `lipgloss.Width()`.
- If less than `width` param, append spaces to fill row. Return full-width string.

## Why This Way?

- **Visual Clarity:** Shortcut keys `[1]` explicitly show how to switch tabs. Accent background make selection obvious.
- **Styling Gotcha:** Line 44-46 comment note that nesting styles inside `activeStyle` would "punch hole" in background color. Single block render keep color solid.
- **Stateless:** Pure function. `MainPane` tell it which index is `active`.

## Step-by-Step Logic

1. **Input:** List of tabs + active index + width.
2. **Tab 1 (Active):** `  [1] Info  ` (Solid Cyan block).
3. **Separator:** `│`.
4. **Tab 2 (Inactive):** `  [2] Apps  ` (Dim text).
5. **Final:** Add spaces to reach terminal width.

---

## 2. `Tab` struct (lines 10–13)

```go
type Tab struct {
    Key   string  // "1"
    Label string  // "Files"
}
```

Simple data holder. `Key` is what the user presses; `Label` is what's displayed.
Keeping them separate lets the label change without touching key bindings.

---

## 3. `strings.Builder` (line 39)

```go
var b strings.Builder
```

Instead of `result += segment` in a loop, `strings.Builder` is used. **Why?**

`+=` on strings creates a new string allocation every iteration.
With N tabs you get N allocations.
`strings.Builder` writes to an internal buffer and allocates once at the end.

Better performance, same result.

---

## 4. The render loop (lines 41–54)

```go
for i, t := range tabs {
    key := "[" + t.Key + "] "
    if i == active {
        b.WriteString(activeStyle.Render(key + t.Label))
    } else {
        b.WriteString(inactiveStyle.Render(keyStyleInactive.Render(key) + t.Label))
    }
    if i < len(tabs)-1 {
        b.WriteString(sepStyle.Render("│"))
    }
}
```

### Active tab — one block render

```go
activeStyle.Render(key + t.Label)
// → "[1] Info" as one styled block with accent background
```

`activeStyle` has `Padding(0, 2)` — adds 2 cells on left and right inside the 
background color block.

**The key insight is in the comment on line 46:**

> Nesting an inner style would punch the inner segment back to `ColorBg`

If you did this:
```go
activeStyle.Render(keyStyle.Render(key) + t.Label)  // WRONG
```

The inner `keyStyle` sets its own background (`ColorBg`),
which **overrides** `activeStyle`'s accent background for those characters.
The padding cells would be accent-colored but the key text would punch a hole 
back to the default background. 
By rendering `key + t.Label` as one plain string inside `activeStyle`,
every character including the padding shares the same accent block.

### Inactive tab — nested styles are fine

```go
inactiveStyle.Render(keyStyleInactive.Render(key) + t.Label)
```

Here, both `inactiveStyle` and `keyStyleInactive` use `ColorBg` as background — so nesting doesn't cause a visible conflict. The key gets `ColorFgFaint`, the label gets `ColorFgDim`,
both on the same background.

### Separator

```go
if i < len(tabs)-1 {
    b.WriteString(sepStyle.Render("│"))
}
```

`i < len(tabs)-1` means: add separator after every tab **except the last**.
Without this guard you'd get a trailing `│` after the last tab.

---

## 5. Padding to full width (lines 56–59)

```go
cur := lipgloss.Width(b.String())
if cur < width {
    b.WriteString(bg.Render(strings.Repeat(" ", width-cur)))
}
```

After all tabs, fill the remaining horizontal space with background-colored spaces.
Same reason as everywhere else: without this, the terminal's default background color
shows through and breaks the uniform panel look.

---

## Summary — lessons in this file

| Concept                    | Where       | Lesson                                                                 |
| -------------------------- | ----------- | ---------------------------------------------------------------------- |
| `strings.Builder`          | line 39     | Avoid string concatenation allocations in loops                        |
| Single-block active render | line 47     | Nested styles override background — flatten when you need uniform fill |
| Nested styles on inactive  | line 49     | Safe when backgrounds match                                            |
| `i < len-1` guard          | line 51     | Separator between items, not after last                                |
| `lipgloss.Width` fill      | lines 56–59 | Same justify-to-width pattern seen in every component                  |
