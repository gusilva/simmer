package ui

import (
	"strings"

	"simmer/internal/device"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ── RNOptionsModal ───────────────────────────────────────────────────────

const rnOptionsW = 46

// RNOption identifies one selectable action in the RN debugging menu.
type RNOption int

const (
	RNOptionBundler RNOption = iota
	RNOptionDevMenu
	RNOptionLogFile
)

// rnOptionItem is one row of the RN options menu.
type rnOptionItem struct {
	option   RNOption
	label    string
	disabled bool
}

// RNOptionsModal lists React Native debugging actions for the app under the
// cursor when "m" is pressed on the Apps list. The header and every item are
// greyed out and non-selectable when the app isn't a detected React Native
// app; individual items may also be disabled on top of that when the
// device/platform combination doesn't support them (e.g. no scriptable
// dev-menu trigger exists for physical iOS).
type RNOptionsModal struct {
	device device.Device
	app    device.App
	isRN   bool
	items  []rnOptionItem
	idx    int
}

// NewRNOptionsModal returns the menu for app on dev. devMenuSupported gates
// the dev-menu item independently of RN status; loggingActive swaps the log
// item's label to "Stop Logging to File" when this app is already being
// logged.
func NewRNOptionsModal(dev device.Device, app device.App, devMenuSupported, loggingActive bool) RNOptionsModal {
	logLabel := "Log to File"
	if loggingActive {
		logLabel = "Stop Logging to File"
	}
	items := []rnOptionItem{
		{option: RNOptionBundler, label: "Configure Bundler", disabled: !app.IsReactNative},
		{option: RNOptionDevMenu, label: "Dev Menu", disabled: !app.IsReactNative || !devMenuSupported},
		{option: RNOptionLogFile, label: logLabel, disabled: !app.IsReactNative},
	}
	m := RNOptionsModal{device: dev, app: app, isRN: app.IsReactNative, items: items}
	m.idx = m.firstEnabled()
	return m
}

func (m RNOptionsModal) firstEnabled() int {
	for i, it := range m.items {
		if !it.disabled {
			return i
		}
	}
	return 0
}

func (m RNOptionsModal) prevEnabled() int {
	for i := m.idx - 1; i >= 0; i-- {
		if !m.items[i].disabled {
			return i
		}
	}
	return -1
}

func (m RNOptionsModal) nextEnabled() int {
	for i := m.idx + 1; i < len(m.items); i++ {
		if !m.items[i].disabled {
			return i
		}
	}
	return -1
}

// RNOptionSelectedMsg carries the chosen option for the parent to act on.
type RNOptionSelectedMsg struct {
	Option RNOption
	Device device.Device
	App    device.App
}

func (m RNOptionsModal) Update(msg tea.Msg) (RNOptionsModal, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "esc":
		return m, func() tea.Msg { return CancelOverlayMsg{} }
	case "up", "k":
		if i := m.prevEnabled(); i >= 0 {
			m.idx = i
		}
	case "down", "j":
		if i := m.nextEnabled(); i >= 0 {
			m.idx = i
		}
	case "enter":
		if m.idx < len(m.items) && !m.items[m.idx].disabled {
			opt := m.items[m.idx].option
			dev, app := m.device, m.app
			return m, func() tea.Msg { return RNOptionSelectedMsg{Option: opt, Device: dev, App: app} }
		}
	}
	return m, nil
}

func (m RNOptionsModal) View() string {
	innerW := rnOptionsW - 6

	headerStyle := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	if m.isRN {
		headerStyle = lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true)
	}
	itemSel := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true)
	itemNorm := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg)
	itemDisabled := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	hintKey := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true)
	hintVerb := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorderHi).
		Background(ColorBg).
		Padding(1, 2)

	var b strings.Builder
	b.WriteString(headerStyle.Render("--- React Native Options ---"))
	b.WriteString("\n\n")

	for i, it := range m.items {
		name := it.label
		switch {
		case it.disabled:
			b.WriteString(itemDisabled.Render("   " + name))
		case i == m.idx:
			pad := strings.Repeat(" ", max(innerW-3-lipgloss.Width(name), 1))
			b.WriteString(itemSel.Render(" ▸ " + name + pad))
		default:
			b.WriteString(itemNorm.Render("   " + name))
		}
		b.WriteString("\n")
	}

	if !m.isRN {
		b.WriteString("\n")
		b.WriteString(itemDisabled.Render("Not a React Native app"))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	sep := hintVerb.Render("  ")
	hints := strings.Join([]string{
		hintKey.Render("↑↓") + hintVerb.Render(" select"),
		hintKey.Render("Enter") + hintVerb.Render(" confirm"),
		hintKey.Render("Esc") + hintVerb.Render(" cancel"),
	}, sep)
	b.WriteString(hints)

	return box.Width(rnOptionsW).Render(b.String())
}
