package ui

import (
	"os"
	"path/filepath"
	"strings"

	"simmer/internal/device"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ── InstallAppModal ───────────────────────────────────────────────────────────

const (
	installModalW     = 68
	installPickerH    = 12
	installSchemeRows = 5
)

type installStep int

const (
	stepPick    installStep = iota // browsing file picker
	stepLoading                   // fetching schemes (iOS only)
	stepScheme                    // picking scheme (iOS only)
	stepErrLoad                   // scheme fetch failed
)

// InstallAppModal is a multi-step overlay:
// 1. File picker — Enter navigates into folders, Space selects.
// 2. (iOS only) Scheme picker after scheme detection.
type InstallAppModal struct {
	dev        device.Device
	fp         filepicker.Model
	step       installStep
	schemes    []string
	schemeIdx  int
	schemeOff  int
	loadErrMsg string
	pickedPath string // path confirmed in step 1
}

// NewInstallAppModal returns a ready modal. The returned Cmd triggers the
// first directory read and must be batched by the caller.
func NewInstallAppModal(dev device.Device) (InstallAppModal, tea.Cmd) {
	fp := filepicker.New()
	fp.ShowHidden = false
	fp.ShowPermissions = false
	fp.ShowSize = false
	fp.AutoHeight = false
	fp.SetHeight(installPickerH)

	// Customise keymap:
	//   • Remove esc from Back so we can use it to cancel the whole modal.
	//   • Remove Enter from Select so Enter only navigates, never auto-selects.
	//     Space selection is handled manually in updatePick.
	km := filepicker.DefaultKeyMap()
	km.Back = key.NewBinding(
		key.WithKeys("h", "backspace", "left"),
		key.WithHelp("h/←", "back"),
	)
	km.Select = key.NewBinding(
		key.WithKeys("space"),
		key.WithHelp("space", "select"),
	)
	fp.KeyMap = km

	// Disable built-in selection — we handle it ourselves on Space so that
	// Enter purely navigates and Space picks files AND directories uniformly.
	fp.FileAllowed = false
	fp.DirAllowed = false

	// AllowedTypes drives visual dimming only (disabled style for non-matching
	// files). Dirs are never dimmed regardless of AllowedTypes.
	switch dev.Platform {
	case device.PlatformIOS:
		// .xcworkspace / .xcodeproj are dirs — always shown enabled.
		// .app bundles are dirs — always shown enabled.
		// .ipa are files — shown enabled only if in AllowedTypes.
		fp.AllowedTypes = []string{".ipa"}
	default: // Android
		fp.AllowedTypes = []string{".apk"}
	}

	// Style to match app palette.
	s := filepicker.DefaultStyles()
	s.Cursor = lipgloss.NewStyle().Foreground(ColorAccent)
	s.Selected = lipgloss.NewStyle().Foreground(ColorAccent).Bold(true)
	s.Directory = lipgloss.NewStyle().Foreground(ColorBorderHi)
	s.File = lipgloss.NewStyle().Foreground(ColorFg)
	s.DisabledFile = lipgloss.NewStyle().Foreground(ColorFgFaint)
	s.DisabledSelected = lipgloss.NewStyle().Foreground(ColorFgFaint)
	s.EmptyDirectory = lipgloss.NewStyle().Foreground(ColorFgFaint).
		PaddingLeft(2).SetString("(empty directory)")
	fp.Styles = s

	home, _ := os.UserHomeDir()
	if home == "" {
		home = "."
	}
	fp.CurrentDirectory = home

	cmd := fp.Init()
	return InstallAppModal{dev: dev, fp: fp}, cmd
}

// SetSchemes is called by the parent when scheme detection completes.
func (m *InstallAppModal) SetSchemes(schemes []string) {
	if len(schemes) == 0 {
		m.step = stepErrLoad
		m.loadErrMsg = "no schemes found in project"
		return
	}
	m.schemes = schemes
	m.schemeIdx = 0
	m.schemeOff = 0
	m.step = stepScheme
}

// SetSchemesError is called by the parent when scheme detection fails.
func (m *InstallAppModal) SetSchemesError(errMsg string) {
	m.step = stepErrLoad
	m.loadErrMsg = errMsg
}

// Update handles all messages for the modal.
func (m InstallAppModal) Update(msg tea.Msg) (InstallAppModal, tea.Cmd) {
	switch m.step {
	case stepPick:
		return m.updatePick(msg)
	case stepLoading:
		if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "esc" {
			return m, func() tea.Msg { return CancelOverlayMsg{} }
		}
	case stepScheme:
		if k, ok := msg.(tea.KeyPressMsg); ok {
			return m.updateScheme(k)
		}
	case stepErrLoad:
		if k, ok := msg.(tea.KeyPressMsg); ok {
			switch k.String() {
			case "esc":
				return m, func() tea.Msg { return CancelOverlayMsg{} }
			case "enter", "space":
				m.step = stepPick
				return m, nil
			}
		}
	}
	return m, nil
}

func (m InstallAppModal) updatePick(msg tea.Msg) (InstallAppModal, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "esc":
			return m, func() tea.Msg { return CancelOverlayMsg{} }

		case "space":
			return m.selectHighlighted()
		}
	}

	// Pass everything else (including Enter) to the filepicker.
	// Enter navigates into directories; Back/h/← goes up; j/k/↑/↓ moves cursor.
	var cmd tea.Cmd
	m.fp, cmd = m.fp.Update(msg)
	return m, cmd
}

// selectHighlighted resolves the currently highlighted path and emits the
// appropriate message. Same logic for both iOS and Android.
func (m InstallAppModal) selectHighlighted() (InstallAppModal, tea.Cmd) {
	hi := m.fp.HighlightedPath()
	if hi == "" {
		return m, nil
	}

	ext := strings.ToLower(filepath.Ext(hi))

	// iOS Xcode project/workspace → load schemes first.
	if m.dev.Platform == device.PlatformIOS {
		switch ext {
		case ".xcworkspace", ".xcodeproj":
			m.pickedPath = hi
			m.step = stepLoading
			return m, func() tea.Msg { return RequestXcodeSchemesMsg{Path: hi} }
		case ".app", ".ipa":
			dev := m.dev
			return m, func() tea.Msg {
				return ConfirmInstallAppMsg{Device: dev, Path: hi, Scheme: ""}
			}
		default:
			return m, nil
		}
	}

	// Android: .apk file or any directory (treated as gradle project root).
	info, err := os.Stat(hi)
	if err != nil {
		return m, nil
	}
	if info.IsDir() || ext == ".apk" {
		dev := m.dev
		return m, func() tea.Msg {
			return ConfirmInstallAppMsg{Device: dev, Path: hi, Scheme: ""}
		}
	}
	return m, nil
}

func (m InstallAppModal) updateScheme(k tea.KeyPressMsg) (InstallAppModal, tea.Cmd) {
	switch k.String() {
	case "esc":
		return m, func() tea.Msg { return CancelOverlayMsg{} }
	case "shift+tab":
		m.step = stepPick
		return m, nil
	case "up", "k":
		if m.schemeIdx > 0 {
			m.schemeIdx--
			if m.schemeIdx < m.schemeOff {
				m.schemeOff = m.schemeIdx
			}
		}
	case "down", "j":
		if m.schemeIdx < len(m.schemes)-1 {
			m.schemeIdx++
			if m.schemeIdx >= m.schemeOff+installSchemeRows {
				m.schemeOff = m.schemeIdx - installSchemeRows + 1
			}
		}
	case "enter":
		if len(m.schemes) == 0 {
			return m, nil
		}
		dev, path, scheme := m.dev, m.pickedPath, m.schemes[m.schemeIdx]
		return m, func() tea.Msg {
			return ConfirmInstallAppMsg{Device: dev, Path: path, Scheme: scheme}
		}
	}
	return m, nil
}

// View renders the modal for overlay placement.
func (m InstallAppModal) View() string {
	innerW := installModalW - 6

	faint := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	hintKey := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true)
	hintVerb := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	hintOk := lipgloss.NewStyle().Foreground(ColorOk).Background(ColorBg).Bold(true)
	labelAct := lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true)
	itemSel := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true)
	itemNorm := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg)
	errStyle := lipgloss.NewStyle().Foreground(ColorErr).Background(ColorBg)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorderHi).
		Background(ColorBg).
		Padding(1, 2)

	sep := faint.Render("  ")

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true).Render("Install App"))
	b.WriteString("\n\n")

	switch m.step {
	case stepPick:
		// Current directory breadcrumb.
		dirStyle := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
		b.WriteString(dirStyle.Render(truncateName(m.fp.CurrentDirectory, innerW)))
		b.WriteString("\n\n")

		// File picker rows (apply bg for consistent colouring).
		for i, line := range strings.Split(m.fp.View(), "\n") {
			if i >= installPickerH {
				break
			}
			b.WriteString(lipgloss.NewStyle().Background(ColorBg).Render(line))
			b.WriteString("\n")
		}
		b.WriteString("\n")

		// Hints — show context-sensitive Space action for the highlighted item.
		spaceHint := m.spaceHintLabel()
		hints := []string{
			hintKey.Render("Enter") + hintVerb.Render(" open"),
			hintKey.Render("↑↓") + hintVerb.Render(" navigate"),
			hintKey.Render("h/←") + hintVerb.Render(" back"),
			hintKey.Render("Esc") + hintVerb.Render(" cancel"),
		}
		if spaceHint != "" {
			hints = append([]string{hintOk.Render("Space") + hintVerb.Render(" "+spaceHint)}, hints...)
		}
		b.WriteString(strings.Join(hints, sep))

	case stepLoading:
		b.WriteString(faint.Render("Loading schemes…"))
		b.WriteString("\n\n")
		b.WriteString(hintKey.Render("Esc") + hintVerb.Render(" cancel"))

	case stepScheme:
		b.WriteString(labelAct.Render(modalListHeader(
			labelAct.Render("Scheme"),
			len(m.schemes), m.schemeIdx, innerW,
		)))
		b.WriteString("\n")
		end := min(m.schemeOff+installSchemeRows, len(m.schemes))
		for i := m.schemeOff; i < end; i++ {
			name := truncateName(m.schemes[i], innerW-4)
			if i == m.schemeIdx {
				pad := strings.Repeat(" ", max(innerW-3-lipgloss.Width(name), 1))
				b.WriteString(itemSel.Render(" ▸ " + name + pad))
			} else {
				b.WriteString(itemNorm.Render("   " + name))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(strings.Join([]string{
			hintOk.Render("Enter") + hintVerb.Render(" build & install"),
			hintKey.Render("↑↓") + hintVerb.Render(" select"),
			hintKey.Render("Shift+Tab") + hintVerb.Render(" back"),
			hintKey.Render("Esc") + hintVerb.Render(" cancel"),
		}, sep))

	case stepErrLoad:
		b.WriteString(errStyle.Render("Error: " + truncateName(m.loadErrMsg, innerW-8)))
		b.WriteString("\n\n")
		b.WriteString(strings.Join([]string{
			hintKey.Render("Space") + hintVerb.Render(" back to picker"),
			hintKey.Render("Esc") + hintVerb.Render(" cancel"),
		}, sep))
	}

	return box.Width(installModalW).Render(b.String())
}

// spaceHintLabel returns the label for the Space hint based on the highlighted
// path. Returns "" when Space would have no effect.
func (m InstallAppModal) spaceHintLabel() string {
	hi := m.fp.HighlightedPath()
	if hi == "" {
		return ""
	}
	ext := strings.ToLower(filepath.Ext(hi))

	if m.dev.Platform == device.PlatformIOS {
		switch ext {
		case ".xcworkspace", ".xcodeproj":
			return "load schemes"
		case ".app", ".ipa":
			return "select"
		}
		return ""
	}

	// Android
	info, err := os.Stat(hi)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return "select project dir"
	}
	if ext == ".apk" {
		return "select"
	}
	return ""
}
