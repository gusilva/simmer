## 1. Flow (`RenderBox` function)

Draws a bordered panel with a title cutout.
Used by `Sidebar` for both the Booted and Available boxes.

```
╭── Booted 2 ──────────────╮
│ ● iPhone 15 Pro    17.5  │
│ ● ▲ Pixel 8         14.0  │
╰──────────────────────────╯
```

- Title embedded in the top border with dashes on both sides
- Optional badge (the count) after the title
- Inner content padded to full width
- Fixed or auto height


### 1. Style Selection
Choose colors based on `focused` bool. 
- High contrast (`ColorBorderHi`) if active. 
- Dim (`ColorBorder`) if inactive.

### 2. Draw Top Edge (Cutout)
- Format `title` + `badge`.
- Calc dashes: `╭── [Title] [Badge] ──────╮`. 
- `leftDash` fixed at 2. `rightDash` fills remainder.

### 3. Draw Sides & Content
Loop through lines of `content`.
- Truncate line if wider than `innerW`.
- Pad line with spaces to hit right wall.
- Wrap with `│` characters.

### 4. Height Management
If `height > 0`:
- **Too much:** Slice `lines` to fit.
- **Too little:** Append blank rows (`│      │`) until target height met.

### 5. Draw Bottom Edge
- Finalize with `╰───────────╯`.
- Join all parts with newlines.

### Why This Way?

- **Flexible UI:** Box can grow (`height=0`) or stay fixed size for grid layouts.
- **Visual Consistency:** Padding and truncation ensure borders always line up regardless of content length.
- **No Overlap:** Cutout title design saves vertical space compared to separate header line.

### Logic Mapping

1. **Input:** Title, content, dimensions.
2. **Top:** `╭── Info (3) ────╮`
3. **Mid:** `│ Name: iPhone   │`
4. **Mid:** `│ OS: 17.0       │`
5. **Bot:** `╰────────────────╯`

---

## 2. Signature (line 14)

```go
func RenderBox(title, badge, content string, width, height int, focused bool) string
```

| Param     | Purpose                                           |
| --------- | ------------------------------------------------- |
| `title`   | Label in the top border cutout                    |
| `badge`   | Short string after the title (e.g. count "3")     |
| `content` | Multi-line string — the pre-rendered rows         |
| `width`   | Total outer width including borders               |
| `height`  | If > 0: fixed height. If ≤ 0: grow to fit content |
| `focused` | Brightens border and title color                  |

`content` arrives as a **pre-rendered string** with `\n` separators.
`RenderBox` doesn't know what's inside — it just wraps it.
That's the separation of concerns: callers like `renderBootedRows()` build 
the content; `RenderBox` frames it.

---

## 3. Focus colors (lines 19–24)

```go
borderC := ColorBorder
titleC  := ColorFgDim
if focused {
    borderC = ColorBorderHi
    titleC  = ColorBorderHi
}
```

Same pattern as `MainPane`. Focused = bright border + bright title.
Unfocused = dim. Simple conditional, no style duplication.

---

## 4. Building the top border (lines 37–42)

```go
leftDash  := 2
rightDash := max(width-2-leftDash-2-titleW, 1)

top := border.Render("╭"+strings.Repeat("─", leftDash)+" ") +
       titleText +
       border.Render(" "+strings.Repeat("─", rightDash)+"╮")
```

The top row is assembled in three segments:

```
╭──·           ← leftDash=2 dashes + space   (border style)
 Booted 2      ← titleText (title + badge)   (title/badge styles)
·──────╮       ← space + rightDash dashes    (border style)
```

`rightDash` is computed so the three segments add up to exactly `width` columns:

```
width = 1(╭) + leftDash + 1(space) + titleW + 1(space) + rightDash + 1(╮)
      = 2 fixed corners + leftDash + 2 spaces + titleW + rightDash
```

Rearranged: `rightDash = width - 2 - leftDash - 2 - titleW`. The `max(..., 1)` guarantees at least one dash even on very narrow widths.

**Why split into three `Render` calls?** The title needs its own color.
If you passed the whole line to one `border.Render(...)`, the title text would 
be colored like the border. By rendering each segment independently, each segment
gets its own ANSI codes.

---

## 5. The bottom border (line 44)

```go
bottom := border.Render("╰" + strings.Repeat("─", width-2) + "╯")
```

Simple — `width - 2` dashes between the corner glyphs. One styled block, uniform color.

---

## 6. Content loop (lines 47–56)

```go
for line := range strings.SplitSeq(content, "\n") {
    w := lipgloss.Width(line)
    if w > innerW {
        line = lipgloss.NewStyle().MaxWidth(innerW).Render(line)
        w = lipgloss.Width(line)
    }
    padded := line + bg.Render(strings.Repeat(" ", innerW-w))
    lines = append(lines, border.Render("│") + padded + border.Render("│"))
}
```

`strings.SplitSeq` — yields one line at a time without allocating a full `[]string` slice.
More memory-efficient than `strings.Split`.

For each line:
1. **Measure** visible width with `lipgloss.Width`
2. **Clip** if too wide — `MaxWidth(innerW)` truncates to fit (Lip Gloss handles ANSI-safe clipping)
3. **Pad** right side to fill `innerW` with background-colored spaces
4. **Frame** — prepend and append `│` in border color

Each `│` is its own `Render` call so it gets the border color independently of the
content.

---

## 7. Height control (lines 58–69)

```go
if height > 0 {
    innerH := max(height-2, 0)      // subtract top + bottom border rows
    if len(lines) > innerH {
        lines = lines[:innerH]      // truncate excess
    } else {
        blank := bg.Render(strings.Repeat(" ", innerW))
        fill  := border.Render("│") + blank + border.Render("│")
        for len(lines) < innerH {
            lines = append(lines, fill)   // pad with empty rows
        }
    }
}
```

Two cases:
- **Content too tall** → slice off the bottom (`lines[:innerH]`).
  Simple truncation — no scrolling here, that's for the parent to handle.
- **Content too short** → append blank rows until the box reaches `innerH`.
  This is what makes the Available box fill the remaining screen height even when there are few devices.

If `height <= 0`, skip this block entirely — the box grows to fit content naturally.

---

## 8. Final assembly (line 71)

```go
return strings.Join(append(append([]string{top}, lines...), bottom), "\n")
```

Dense but readable once unpacked:

```go
rows := []string{top}
rows  = append(rows, lines...)
rows  = append(rows, bottom)
return strings.Join(rows, "\n")
```

`[]string{top}` starts a new slice with just the top border.
Appending `lines...` spreads the content slice in.
Appending `bottom` adds the final border.
`strings.Join` with `\n` produces the final multi-line string.

---

## Summary

```
RenderBox(title, badge, content, width, height, focused)
         │
         ├─ top border: ╭── title badge ───╮  (3 render segments)
         │
         ├─ for each content line:
         │    clip if too wide
         │    pad right to innerW
         │    wrap with │ ... │
         │
         ├─ if height > 0:
         │    truncate OR pad with blank rows
         │
         └─ join: top + lines + bottom
```

Key lessons:
- **Pre-rendered content in, framed string out** — caller owns content,
  `RenderBox` owns framing
- **Three-segment top border** — each color zone needs its own `Render` call
- `strings.SplitSeq` — iterator form of split, no intermediate slice allocation
- `height` param controls fixed vs. auto sizing — same function handles both cases
