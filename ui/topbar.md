## Structure (`TopBarParams` struct)

```
 simmer v0.1.2 · ● 2 booted   3 iOS  ▲ 2 Android          xcrun 15.3 · adb 1.0.41 
```

Left side: app identity + device counts. Right side: tool versions.
Gap fills the space between them.

Container for dynamic data:
- `Width`: terminal width.
- `Counts`: stats for booted/iOS/Android.
- `ToolVersions`: map for `adb` and `xcrun` version strings.

## 1. Flow (`RenderTopBar` function)

### 1. Build Left Side
Assemble brand name, version, and device stats. 
- Use specific colors for icons: `ColorOk` (green dot), `ColorIOS`, `ColorAndroid` (▲).
- Join with `sep` (dot separator).

### 2. Build Right Side
Check `ToolVersions` map. 
- If `xcrun` or `adb` versions exist, append to `metaParts`.
- Join with dots. Render in `StyleFaint` (dim color).

### 3. Layout & Render
- Calculate `gap`: `innerWidth - len(left) - len(right)`.
- Use `strings.Repeat(" ", gap)` to push metadata to far right edge (CSS `justify-content: space-between` effect).
- Wrap everything in `StyleTopBar`.

## Why This Way?

- **Stateless/Pure:** Function not part of `Sidebar` or `MainPane`.
  Parent pass params, get string. Easy to test.
- **Lipgloss Width:** `lipgloss.Width()` used instead of `len()` because 
  icons/colors use multiple bytes/terminal cells.
- **Dynamic Meta:** Right side only show tools that are actually installed/detected.

1. **Input:** Width + Stats + Versions.
2. **Left:** `simmer 1.0.0 · ● 2 booted  1 iOS  ▲ 3 Android`
3. **Right:** `xcrun 15.0 · adb 34.0.4`
4. **Gap:** Fill middle with spaces.
5. **Output:** Full width colored string.


---

## 2. `TopBarParams` struct (lines 13–21)

```go
type TopBarParams struct {
    Width        int
    AppVersion   string
    BootedCount  int
    IOSCount     int
    AndroidCount int
    ToolVersions map[device.Platform]string
}
```

**Why a params struct instead of function arguments?**

If `RenderTopBar` took 6 positional args, callers look like:

```go
RenderTopBar(w, "v0.1", 2, 3, 2, versions)  
```

With a struct, callers are self-documenting:
```go
RenderTopBar(TopBarParams{Width: w, BootedCount: 2, IOSCount: 3, ...})
```

Also: adding a new field later doesn't break any existing call sites.

---

## 3. `RenderTopBar` — pure function (lines 23–56)

```go
func RenderTopBar(p TopBarParams) string
```

**No receiver, no state.** Takes params, returns a string. Same input always 
produces same output. Easy to test, easy to reason about.

This is different from `MainPane` and `Sidebar` which are stateful components 
with `Update`/`View`. The top bar has **no interaction** (no key handling, no cursor)
— so a plain function is the right tool.

---

## 4. Building the left segment (lines 30–40)

```go
left := StyleBrand.Render("simmer") +
    StyleFaint.Render(" "+p.AppVersion) +
    sep +
    lipgloss.NewStyle().Foreground(ColorOk).Render("●") +
    StyleDim.Render(fmt.Sprintf(" %d booted", p.BootedCount)) +
    ...
```

Each visual "token" is styled independently and **concatenated as strings**.
Lip Gloss wraps each segment in ANSI escape codes:

```
\e[bold]simmer\e[reset]  \e[faint]v0.1\e[reset]  ...
```

**Why not one big styled block?** Because each token has a different color/weight.
You'd need nested styles which Lip Gloss doesn't support — composing small styled strings is the idiomatic approach.

---

## 5. Building the right segment (lines 42–50)

```go
var metaParts []string
if v := p.ToolVersions[device.PlatformIOS]; v != "" {
    metaParts = append(metaParts, "xcrun "+v)
}
if v := p.ToolVersions[device.PlatformAndroid]; v != "" {
    metaParts = append(metaParts, "adb "+v)
}
right := StyleFaint.Render(strings.Join(metaParts, " · "))
```

Tool versions are **optional** — if `xcrun` or `adb` isn't available,
the key won't be in the map and the part is skipped.
`strings.Join` handles the separator cleanly whether there are 0, 1, or 2 parts.

Map lookup `p.ToolVersions[key]` returns `""` for missing keys in Go (zero value) — so no `ok` check needed,
the `v != ""` guard is enough.

---

## 6. The gap calculation (lines 52–55)

```go
innerWidth := p.Width - 2
gap := max(innerWidth - lipgloss.Width(left) - lipgloss.Width(right), 0)

return StyleTopBar.Width(p.Width).Render(left + strings.Repeat(" ", gap) + right)
```

This is the **manual flex justify-between** pattern — same as in sidebar rows,
same as in mainpane rows.

**Why `lipgloss.Width` instead of `len`?**  
`len(s)` counts **bytes**. ANSI escape codes add invisible bytes.
A styled string like `"\e[1msimmer\e[0m"` has `len` = 15 but only **6 visible cells**.
`lipgloss.Width` strips ANSI codes and counts actual terminal columns — also handles wide characters (CJK = 2 cells each).

**Why `max(..., 0)`?**  
If the terminal is very narrow, `left + right` might already exceed `innerWidth`.
Without the clamp, `strings.Repeat(" ", negative)` would panic.
`max` ensures gap is at least 0.

`StyleTopBar.Width(p.Width)` tells Lip Gloss to pad/clip the final string to exactly
`p.Width` columns — guarantees the bar fills the full terminal width with background color.

---

## Summary

| Concept                | Where               | Lesson                                          |
| ---------------------- | ------------------- | ----------------------------------------------- |
| Params struct          | `TopBarParams`      | Self-documenting, extensible call sites         |
| Pure render function   | `RenderTopBar`      | No state = no `Update`, just a function         |
| Per-token styling      | `left` construction | Compose small styled strings, not one big block |
| `lipgloss.Width`       | gap calc            | Measures visible cells, not bytes               |
| Manual justify-between | gap calc            | Same pattern used across all UI components      |
| `max(..., 0)` guard    | gap calc            | Prevents panic on narrow terminals              |
