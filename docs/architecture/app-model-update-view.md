## `internal/app/app.go` heart of app. 

Use Bubble Tea v2. Orchestrate `ui` components and `device` logic.

This is the **root of the application** — the top-level Bubble Tea model
that owns all state and wires every component together.
Everything you've studied so far plugs in here.

Before diving in, the contract:

```go
type tea.Model interface {
    Init()   tea.Cmd           // called once at startup
    Update() (tea.Model, tea.Cmd)  // called on every message
    View()   tea.View          // called every render frame
}
```

`model` in `internal/app/app.go` implements all three.
Everything else is supporting infrastructure.


---

## Model (`model` struct)
State container:
- `sidebar`, `mainPane`: sub-models for UI.
- `coordinator`: interface to `adb` / `simctl`.
- `focus`: track which pane (Sidebar/Main) capture keys.
- `status`: temporary text for footer (sequence count handle auto-clear).
- `logStream`: pointer to active background log pipe.

## Flow

### 1. Init
- `Init()`: Call `fetchDevicesCmd`. Start async discovery.
- `initialModel()`: Setup `Coordinator`. Link `IOSManager` + `AndroidManager`.

### 2. Update (Message Handling)
- **`tea.WindowSizeMsg`:** Calc dimensions. Resize sub-panels.
- **`tea.KeyPressMsg`:** 
    - `q`: Quit.
    - `esc`: Return focus to Sidebar.
    - `space`: Select device → Load Info/Apps → Switch focus.
    - `b`/`s`: Boot or Shutdown selected device.
    - Forward other keys to focused component.
- **Async Result Msgs:**
    - `discoveryMsg`: Update device list + tool versions.
    - `bootResultMsg`: Trigger refresh. If Android, start `bootPollMsg` 
      loop (wait for `adb`).
    - `logLineMsg`: Push new line into `mainPane` buffer. Continue reading.
    - `ui.RequestLogStreamMsg`: Kill old stream → Start new log pipe.

### 3. View (Composition)
- Assemble `TopBar`, `Body`, and `Footer` vertically.
- `Body`: Side-by-side Sidebar and MainPane.
- `AltScreen`: Enable full-terminal mode.

## Why This Way?

- **Async I/O:** CLI tools slow. `internal/app/app.go` run them as `tea.Cmd` (goroutines) so 
  UI never hang.
- **Coordination:** `MainPane` tell `model` it want files → `model` call
  `Coordinator` → `model` send data back to `MainPane`. Component stay decoupled.
- **State Management:** Keep persistent things (log streams) at top level so 
  they live across tab switches.

## Logic Mapping

1. **Start:** Query tools.
2. **Loop:** Wait for Key or CLI result.
3. **Logic:** 
    - Key → Trigger CMD.
    - CMD Finish → Update Model → Re-render.
4. **Exit:** Clean up log streams. Stop program.

---

## 2. The `model` struct (lines 28–51)

```go
type model struct {
    // UI components
    sidebar  ui.Sidebar
    mainPane ui.MainPane
    focus    appFocus         // focusSidebar | focusMain

    // Domain
    coordinator *device.Coordinator

    // App state
    loading  bool
    errs     []error
    quitting bool
    width, height int

    // Top bar counters
    toolVersions map[device.Platform]string
    bootedCount, iosCount, androidCount int
    lastRefresh  time.Time

    // Active log stream
    logStream   *device.LogStream
    logBundleID string
    logDeviceID string

    // Footer status
    status     string
    statusKind ui.StatusKind
    statusSeq  int
}
```

This is the **single source of truth** for the whole app.
Bubble Tea requires one root model; all sub-components (`Sidebar`, `MainPane`) 
are embedded as values — they're part of this struct, not separate objects.

---

## 3. Message types (lines 53–98)

```go
type discoveryMsg    device.DiscoveryResult
type bootResultMsg   struct{ device device.Device; err error }
type shutdownResultMsg struct{ ... }
type fileTreeMsg     struct{ device device.Device; root device.FileNode; err error }
type appsListMsg     struct{ ... }
type infoMsg         struct{ ... }
type bootPollMsg     struct{ device device.Device; remaining int }
type logLineMsg      struct{ bundleID, line string }
type logEndedMsg     struct{ bundleID string; err error }
type clearStatusMsg  int
```

**Messages are how async work delivers results.** 

A goroutine (via `tea.Cmd`) runs in the background, then returns one of these structs. 
`Update` receives it and acts.

`type discoveryMsg device.DiscoveryResult` is a **type alias with a new name** — same 
underlying data as `DiscoveryResult`, but a distinct type so `Update`'s type switch
can tell it apart from other messages.

---

## 4. Commands — wrapping async work (lines 100–187)

```go
func (m model) fetchDevicesCmd() tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
        defer cancel()
        return discoveryMsg(m.coordinator.Discover(ctx))
    }
}
```

A `tea.Cmd` is just `func() tea.Msg`.
Bubble Tea runs it in a goroutine automatically.
When it returns, the result is sent to `Update` as a message.

**Why return a function instead of calling directly?**  
If you called `coordinator.Discover()` inline in `Update`,
it would block the entire UI — no redraws, no key handling.
Returning a function lets Bubble Tea run it off the main thread.

Pattern repeated for every async operation:
```
cmd = func() → do work → return resultMsg
Update sees resultMsg → update state → return new model
```

### `nextLogLineCmd` — the streaming loop (lines 178–187)

```go
func nextLogLineCmd(stream *device.LogStream, bundleID string) tea.Cmd {
    return func() tea.Msg {
        line, ok := <-stream.Lines   // BLOCKS until a line arrives
        if !ok {
            err := <-stream.Done
            return logEndedMsg{bundleID: bundleID, err: err}
        }
        return logLineMsg{bundleID: bundleID, line: line}
    }
}
```

This is the **log streaming pump**. It blocks on the channel until one line arrives,
delivers it as a message, and `Update` schedules the next call. One goroutine per line — but each goroutine lives only until one line is delivered.

Why not a goroutine that loops forever? Because Bubble Tea owns the event loop.
You can't push messages in from outside; you must return them via commands.
So the pattern is: **read one item → deliver → be re-scheduled**.

### `scheduleBootPoll` — delayed polling (lines 169–176)

```go
func scheduleBootPoll(dev device.Device, remaining int) tea.Cmd {
    return tea.Tick(4*time.Second, func(_ time.Time) tea.Msg {
        return bootPollMsg{device: dev, remaining: remaining}
    })
}
```

Android emulators register with `adb` asynchronously after boot.
This schedules a check every 4 seconds, up to `remaining` times.
Each poll decrements `remaining`; when it hits 0, polling stops.

`tea.Tick` is a Bubble Tea built-in that fires a message after a duration.

---

## 5. `Init` (lines 227–229)

```go
func (m model) Init() tea.Cmd {
    return m.fetchDevicesCmd()
}
```

Called once when the program starts.
Returns the first command — kick off device discovery immediately.

---

## 6. `Update` — the full event loop (lines 231–498)

This is the largest function. 
Every message the app will ever receive is handled here.

### Window resize (lines 233–244)

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    bodyH := m.bodyHeight() - 1
    m.sidebar.SetSize(ui.DefaultSidebarWidth, bodyH)
    mainW := max(m.width-(ui.DefaultSidebarWidth+2)-1, 0)
    m.mainPane.SetSize(mainW, bodyH)
    return m, nil
```

Bubble Tea sends this whenever the terminal is resized. The model recalculates available space and pushes sizes down to both components. `bodyHeight()` subtracts topbar and footer from total height.

### Key handling — focus-aware routing (lines 246–299)

```go
case tea.KeyPressMsg:
    // Global keys (always fire)
    switch msg.String() {
    case "q", "ctrl+c": return m, tea.Quit
    }

    if m.focus == focusMain {
        if msg.String() == "esc" { ... shift focus to sidebar }
        m.mainPane, cmd = m.mainPane.Update(msg)   // delegate to mainPane
        return m, cmd
    }

    // focus == focusSidebar
    switch msg.String() {
    case "r": refresh devices
    case "b": boot selected device
    case "s": shutdown selected device
    case "space": load device into mainPane
    }
    m.sidebar, cmd = m.sidebar.Update(msg)   // delegate to sidebar
    return m, cmd
```

**Key routing by focus:**
- `q`/`ctrl+c` — global, always handled here
- `esc` — always shifts focus back to sidebar
- Everything else — routed to whichever component has focus

When delegating, the component's `Update` returns a new value (remember: value types).
The model must **save the returned value**:
`m.mainPane, cmd = m.mainPane.Update(msg)`.

If you wrote just `m.mainPane.Update(msg)` without saving, the state change would be lost.

### `space` — loading a device (lines 283–295)

```go
case "space":
    sel := m.sidebar.SelectedDevice()
    if sel == nil || sel.Status != device.StatusRunning { return m, nil }

    m.focus = focusMain
    m.applyFocus()
    dev := *sel
    m.mainPane.SetDevice(&dev, nil)

    return m, tea.Batch(m.loadInfoCmd(dev), m.loadAppsCmd(dev))
```

`tea.Batch` runs multiple commands concurrently — info and apps load in parallel.
`SetDevice(&dev, nil)` initialises the pane with no tree (tree loads separately when
the Files tab is opened).

### `discoveryMsg` (lines 301–323)

```go
case discoveryMsg:
    m.loading = false
    m.toolVersions = msg.ToolVersions
    // count booted/iOS/Android for top bar
    m.sidebar.SetDevices(msg.Devices)
    return m, nil
```

Discovery result arrives. Update the top bar counters, push devices into the sidebar.
No new commands — the app is now idle until the user acts.

### `bootResultMsg` (lines 325–344)

```go
case bootResultMsg:
    if msg.err != nil {
        return m, m.setStatus("boot failed: "+errPreview(msg.err), ui.StatusErr)
    }
    cmds := []tea.Cmd{
        m.fetchDevicesCmd(),
        m.setStatus("booted "+msg.device.Name, ui.StatusOk),
    }
    if msg.device.Platform == device.PlatformAndroid {
        cmds = append(cmds, scheduleBootPoll(msg.device, 15))
    }
    return m, tea.Batch(cmds...)
```

On success: refresh the device list, show status, and (for Android) start the 4-second poll cycle. `tea.Batch` again for parallel execution.

### `clearStatusMsg` — the sequence trick (lines 418–423)

```go
case clearStatusMsg:
    if int(msg) == m.statusSeq {
        m.status = ""
    }
    return m, nil
```

`setStatus` bumps `statusSeq` and schedules a `clearStatusMsg(seq)` 4 seconds later. If the user triggered another action before the timer fires, `statusSeq` has advanced — the old timer's message carries a stale `seq` and is ignored. Only the **latest** status clears itself. Smart debounce with no timers to cancel.

### Log streaming messages (lines 451–494)

```go
case ui.RequestLogStreamMsg:
    // stop old stream if any
    // start new stream
    m.logStream = stream
    m.mainPane.SetLogBundle(bundleID)
    return m, tea.Batch(
        nextLogLineCmd(stream, bundleID),   // kick off the pump
        m.setStatus("streaming "+bundleID, ui.StatusOk),
    )

case logLineMsg:
    if msg.bundleID != m.logBundleID { return m, nil }  // stale stream
    m.mainPane.AppendLog(msg.line)
    return m, nextLogLineCmd(m.logStream, m.logBundleID)  // schedule next read

case logEndedMsg:
    m.logStream = nil
    return m, m.setStatus("stream ended", ...)
```

The `bundleID` check in `logLineMsg` is crucial: if the user switches apps while a log line is in-flight, the stale message is silently dropped. The pump only continues if the `bundleID` still matches the active stream.

---

## 7. `setStatus` — transient footer messages (lines 216–225)

```go
func (m *model) setStatus(text string, kind ui.StatusKind) tea.Cmd {
    m.statusSeq++
    m.status = text
    m.statusKind = kind
    seq := m.statusSeq
    return tea.Tick(4*time.Second, func(_ time.Time) tea.Msg {
        return clearStatusMsg(seq)
    })
}
```

Note: this is the only function in the file using a **pointer receiver** (`*model`).
It mutates `m` directly.

All the `Update` methods use value receivers because they return a new model — this one is called from within `Update` on a local copy, so the mutation is fine.

---

## 8. `View` (lines 526–573)

```go
func (m model) View() tea.View {
    topBar := ui.RenderTopBar(...)
    footer := ui.RenderFooter(...)
    bodyH := ...

    body = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, mainPane)
    body = lipgloss.NewStyle().Width(m.width).Height(bodyH)...Render(body)

    v := tea.NewView(lipgloss.JoinVertical(lipgloss.Left, topBar, body, footer))
    v.AltScreen = true
    v.WindowTitle = "Simmer"
    v.BackgroundColor = ui.ColorBg
    return v
}
```

**Returns `tea.View`, not a string** — this is Bubble Tea v2. `tea.NewView` wraps the 
rendered string and lets you set terminal features declaratively:

| Field              | Effect                                                                                          |
| ------------------ | ----------------------------------------------------------------------------------------------- |
| `AltScreen = true` | Use the terminal's alternate screen buffer (app takes over the full terminal, restored on exit) |
| `WindowTitle`      | Sets the terminal window/tab title                                                              |
| `BackgroundColor`  | Fills the terminal's background color, not just where text appears                              |

Layout is composed with Lip Gloss joins:
```
JoinVertical(topBar, body, footer)
               ↑
         JoinHorizontal(sidebar, mainPane)
```

---

## 9. `main` (lines 575–582)

```go
func main() {
    m := initialModel()
    p := tea.NewProgram(m)
    if _, err := p.Run(); err != nil {
        fmt.Printf("Fatal error: %v\n", err)
        os.Exit(1)
    }
}
```

Four lines. `tea.NewProgram` takes the model, `p.Run()` starts the event 
loop — blocks until `tea.Quit` is sent (when user presses `q`).

---

## 10. Full message flow — one example end to end

User presses `space` on a running device:

```
KeyPressMsg("space")
  → Update: SetDevice, tea.Batch(loadInfoCmd, loadAppsCmd)
      ↓                          ↓
  goroutine 1: coordinator.Info  goroutine 2: coordinator.ListApps
      ↓                          ↓
  infoMsg{...}               appsListMsg{...}
      ↓                          ↓
  Update: mainPane.SetInfo    Update: mainPane.SetApps
      ↓                          ↓
  View() re-renders Info tab  View() re-renders Apps tab
```

Both goroutines race. Both eventually deliver. Each delivery triggers a re-render.
The model is always consistent because `Update` processes one message at a time — no concurrent mutation.

---

## Summary — how it all connects

```
main()
  tea.NewProgram(model)
       │
       ├── Init() → fetchDevicesCmd → discoveryMsg → SetDevices
       │
       ├── Update(msg)
       │     ├── KeyPressMsg → delegate to sidebar or mainPane
       │     ├── discoveryMsg → update counters, sidebar.SetDevices
       │     ├── bootResultMsg → refresh, poll Android
       │     ├── appsListMsg → mainPane.SetApps
       │     ├── logLineMsg → mainPane.AppendLog → nextLogLineCmd (loop)
       │     └── ... all other msgs
       │
       └── View()
             topBar ─┐
             sidebar  ├── JoinHorizontal → body → JoinVertical → tea.View
             mainPane ┘
             footer  ─┘
```

- **Commands are the only way to do async work** — return a `func() tea.Msg`,
  Bubble Tea runs it off-thread
- **`tea.Batch`** runs multiple commands concurrently
- **Delegate sub-component updates and save the returned value**:
  `m.sidebar, cmd = m.sidebar.Update(msg)`
- **Message-based streaming loop**: read one item per command, re-schedule in `Update`
- **Sequence counters** to drop stale timer messages without cancelling them
- **`tea.View`** (v2) replaces plain string return and exposes terminal features 
  declaratively

