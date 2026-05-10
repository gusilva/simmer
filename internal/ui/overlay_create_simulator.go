package ui

import (
	"fmt"
	"strings"

	"simmer/internal/device"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ── CreateSimulatorModal ────────────────────────────────────────────────────

const (
	createModalW = 62
	listShowRows = 5
)

type createField int

const (
	fieldName       createField = iota
	fieldDeviceType             // 1
	fieldRuntime                // 2
)

// CreateSimulatorModal is an overlay form for creating a new iOS simulator.
type CreateSimulatorModal struct {
	nameInput   textinput.Model
	deviceTypes []device.DeviceType
	runtimes    []device.Runtime
	dtIdx       int
	rtIdx       int
	dtOffset    int
	rtOffset    int
	focused     createField
	loading     bool
}

// NewCreateSimulatorModal returns a modal in loading state with name field focused.
// The returned tea.Cmd drives cursor animation and must be batched by the caller.
func NewCreateSimulatorModal() (CreateSimulatorModal, tea.Cmd) {
	ti := textinput.New()
	ti.Placeholder = "e.g. My iPhone 16 Pro"
	ti.CharLimit = 64
	ti.SetWidth(createModalW - 8)
	cmd := ti.Focus()
	return CreateSimulatorModal{
		nameInput: ti,
		loading:   true,
		focused:   fieldName,
	}, cmd
}

// SetDeviceTypes populates the device type list; clears loading when both lists are ready.
func (m *CreateSimulatorModal) SetDeviceTypes(types []device.DeviceType) {
	m.deviceTypes = types
	if len(m.runtimes) > 0 {
		m.loading = false
	}
}

// SetRuntimes populates the runtime list (available runtimes only); clears loading when ready.
func (m *CreateSimulatorModal) SetRuntimes(runtimes []device.Runtime) {
	filtered := make([]device.Runtime, 0, len(runtimes))
	for _, r := range runtimes {
		if r.IsAvailable {
			filtered = append(filtered, r)
		}
	}
	m.runtimes = filtered
	if len(m.deviceTypes) > 0 {
		m.loading = false
	}
}

// Update handles key and non-key messages for the modal.
func (m CreateSimulatorModal) Update(msg tea.Msg) (CreateSimulatorModal, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		if m.focused == fieldName {
			var cmd tea.Cmd
			m.nameInput, cmd = m.nameInput.Update(msg)
			return m, cmd
		}
		return m, nil
	}

	switch k.String() {
	case "esc":
		return m, func() tea.Msg { return CancelOverlayMsg{} }

	case "tab":
		m.focused = (m.focused + 1) % 3
		if m.focused == fieldName {
			return m, m.nameInput.Focus()
		}
		m.nameInput.Blur()

	case "shift+tab":
		m.focused = createField((int(m.focused) + 2) % 3)
		if m.focused == fieldName {
			return m, m.nameInput.Focus()
		}
		m.nameInput.Blur()

	case "enter":
		if m.focused == fieldRuntime && m.canSubmit() {
			name := strings.TrimSpace(m.nameInput.Value())
			dtID := m.deviceTypes[m.dtIdx].Identifier
			rtID := m.runtimes[m.rtIdx].Identifier
			return m, func() tea.Msg {
				return ConfirmCreateSimulatorMsg{Name: name, DeviceTypeID: dtID, RuntimeID: rtID}
			}
		}
		if m.focused < fieldRuntime {
			m.focused++
			m.nameInput.Blur()
		}

	case "up", "k":
		switch m.focused {
		case fieldDeviceType:
			if m.dtIdx > 0 {
				m.dtIdx--
				if m.dtIdx < m.dtOffset {
					m.dtOffset = m.dtIdx
				}
			}
		case fieldRuntime:
			if m.rtIdx > 0 {
				m.rtIdx--
				if m.rtIdx < m.rtOffset {
					m.rtOffset = m.rtIdx
				}
			}
		}

	case "down", "j":
		switch m.focused {
		case fieldDeviceType:
			if m.dtIdx < len(m.deviceTypes)-1 {
				m.dtIdx++
				if m.dtIdx >= m.dtOffset+listShowRows {
					m.dtOffset = m.dtIdx - listShowRows + 1
				}
			}
		case fieldRuntime:
			if m.rtIdx < len(m.runtimes)-1 {
				m.rtIdx++
				if m.rtIdx >= m.rtOffset+listShowRows {
					m.rtOffset = m.rtIdx - listShowRows + 1
				}
			}
		}
	}

	if m.focused == fieldName {
		var cmd tea.Cmd
		m.nameInput, cmd = m.nameInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m CreateSimulatorModal) canSubmit() bool {
	return strings.TrimSpace(m.nameInput.Value()) != "" &&
		len(m.deviceTypes) > 0 && m.dtIdx < len(m.deviceTypes) &&
		len(m.runtimes) > 0 && m.rtIdx < len(m.runtimes)
}

// View renders the modal box as an ANSI string for overlay placement.
func (m CreateSimulatorModal) View() string {
	innerW := createModalW - 6 // border(2) + padding L+R(4)

	faint := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	hintKey := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true)
	hintVerb := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	labelAct := lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true)
	labelInact := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
	itemSel := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true)
	itemNorm := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorderHi).
		Background(ColorBg).
		Padding(1, 2)

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true).Render("New iOS Simulator"))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(faint.Render("  Loading device types and runtimes…"))
		return box.Width(createModalW).Render(b.String())
	}

	// Name
	lbl := labelInact
	if m.focused == fieldName {
		lbl = labelAct
	}
	b.WriteString(lbl.Render("Name"))
	b.WriteString("\n")
	b.WriteString(m.nameInput.View())
	b.WriteString("\n\n")

	// Device Type
	lbl = labelInact
	if m.focused == fieldDeviceType {
		lbl = labelAct
	}
	b.WriteString(modalListHeader(lbl.Render("Device Type"), len(m.deviceTypes), m.dtIdx, innerW))
	b.WriteString("\n")
	b.WriteString(renderDeviceTypeList(m.deviceTypes, m.dtIdx, m.dtOffset, m.focused == fieldDeviceType, innerW, itemSel, itemNorm, faint))
	b.WriteString("\n\n")

	// Runtime
	lbl = labelInact
	if m.focused == fieldRuntime {
		lbl = labelAct
	}
	b.WriteString(modalListHeader(lbl.Render("Runtime"), len(m.runtimes), m.rtIdx, innerW))
	b.WriteString("\n")
	b.WriteString(renderRuntimeList(m.runtimes, m.rtIdx, m.rtOffset, m.focused == fieldRuntime, innerW, itemSel, itemNorm, faint))
	b.WriteString("\n\n")

	// Hints
	sep := faint.Render("  ")
	parts := []string{
		hintKey.Render("Tab") + hintVerb.Render(" next"),
		hintKey.Render("↑↓") + hintVerb.Render(" select"),
	}
	if m.canSubmit() {
		parts = append(parts, lipgloss.NewStyle().Foreground(ColorOk).Background(ColorBg).Bold(true).Render("Enter")+hintVerb.Render(" create"))
	}
	parts = append(parts, hintKey.Render("Esc")+hintVerb.Render(" cancel"))
	b.WriteString(strings.Join(parts, sep))

	return box.Width(createModalW).Render(b.String())
}

func modalListHeader(label string, count, idx, innerW int) string {
	faint := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	counter := ""
	if count > 0 {
		counter = faint.Render(fmt.Sprintf("%d/%d", idx+1, count))
	}
	gap := max(innerW-lipgloss.Width(label)-lipgloss.Width(counter), 1)
	return label + strings.Repeat(" ", gap) + counter
}

func renderDeviceTypeList(types []device.DeviceType, selIdx, offset int, focused bool, innerW int, sel, norm, faint lipgloss.Style) string {
	if len(types) == 0 {
		return faint.Render("  (none)")
	}
	end := min(offset+listShowRows, len(types))
	rows := make([]string, 0, end-offset)
	for i := offset; i < end; i++ {
		name := truncateName(types[i].Name, innerW-4)
		if focused && i == selIdx {
			pad := strings.Repeat(" ", max(innerW-3-lipgloss.Width(name), 1))
			rows = append(rows, sel.Render(" ▸ "+name+pad))
		} else {
			rows = append(rows, norm.Render("   "+name))
		}
	}
	return strings.Join(rows, "\n")
}

func renderRuntimeList(rts []device.Runtime, selIdx, offset int, focused bool, innerW int, sel, norm, faint lipgloss.Style) string {
	if len(rts) == 0 {
		return faint.Render("  (none available)")
	}
	end := min(offset+listShowRows, len(rts))
	rows := make([]string, 0, end-offset)
	for i := offset; i < end; i++ {
		name := truncateName(rts[i].Name, innerW-4)
		if focused && i == selIdx {
			pad := strings.Repeat(" ", max(innerW-3-lipgloss.Width(name), 1))
			rows = append(rows, sel.Render(" ▸ "+name+pad))
		} else {
			rows = append(rows, norm.Render("   "+name))
		}
	}
	return strings.Join(rows, "\n")
}
