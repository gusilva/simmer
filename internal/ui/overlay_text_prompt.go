package ui

import (
	"strings"

	"simmer/internal/device"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ── TextPromptModal ──────────────────────────────────────────────────────

const textPromptW = 52

// TextPromptKind distinguishes which action a TextPromptModal's submission
// feeds into, so the parent can branch without a second modal type.
type TextPromptKind int

const (
	TextPromptBundler TextPromptKind = iota
	TextPromptLogPath
)

// TextPromptModal collects one line of free text (bundler host:port, a log
// file path) for the RN debugging menu. validate runs on submit; a non-nil
// error is shown inline and the modal stays open for correction.
type TextPromptModal struct {
	kind     TextPromptKind
	title    string
	device   device.Device
	app      device.App
	input    textinput.Model
	errMsg   string
	validate func(string) error
}

// NewTextPromptModal returns a focused single-field prompt titled title, with
// placeholder/initial text, for the given device/app. validate may be nil.
func NewTextPromptModal(kind TextPromptKind, title, placeholder, initial string, dev device.Device, app device.App, validate func(string) error) (TextPromptModal, tea.Cmd) {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.SetValue(initial)
	ti.CharLimit = 256
	ti.SetWidth(textPromptW - 8)
	cmd := ti.Focus()
	return TextPromptModal{
		kind:     kind,
		title:    title,
		device:   dev,
		app:      app,
		input:    ti,
		validate: validate,
	}, cmd
}

// TextPromptSubmittedMsg carries the validated value for the parent to act on.
type TextPromptSubmittedMsg struct {
	Kind   TextPromptKind
	Device device.Device
	App    device.App
	Value  string
}

func (m TextPromptModal) Update(msg tea.Msg) (TextPromptModal, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	switch k.String() {
	case "esc":
		return m, func() tea.Msg { return CancelOverlayMsg{} }
	case "enter":
		val := strings.TrimSpace(m.input.Value())
		if m.validate != nil {
			if err := m.validate(val); err != nil {
				m.errMsg = err.Error()
				return m, nil
			}
		}
		kind, dev, app := m.kind, m.device, m.app
		return m, func() tea.Msg { return TextPromptSubmittedMsg{Kind: kind, Device: dev, App: app, Value: val} }
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m TextPromptModal) View() string {
	labelStyle := lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true)
	hintKey := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true)
	hintVerb := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	errStyle := lipgloss.NewStyle().Foreground(ColorErr).Background(ColorBg)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorderHi).
		Background(ColorBg).
		Padding(1, 2)

	var b strings.Builder
	b.WriteString(labelStyle.Render(m.title))
	b.WriteString("\n\n")
	b.WriteString(m.input.View())
	b.WriteString("\n")
	if m.errMsg != "" {
		b.WriteString("\n")
		b.WriteString(errStyle.Render(m.errMsg))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	sep := hintVerb.Render("  ")
	hints := strings.Join([]string{
		hintKey.Render("Enter") + hintVerb.Render(" confirm"),
		hintKey.Render("Esc") + hintVerb.Render(" cancel"),
	}, sep)
	b.WriteString(hints)

	return box.Width(textPromptW).Render(b.String())
}
