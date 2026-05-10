package ui

import (
	"strings"

	"simmer/internal/device"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ── CreateAndroidEmulatorModal ───────────────────────────────────────────────

const androidModalW = 64

// CreateAndroidEmulatorModal is an overlay form for creating a new Android emulator.
type CreateAndroidEmulatorModal struct {
	nameInput      textinput.Model
	systemImages   []device.Runtime    // required
	deviceProfiles []device.DeviceType // optional
	imgIdx         int
	profIdx        int
	imgOffset      int
	profOffset     int
	focused        createField
	loading        bool
}

// NewCreateAndroidEmulatorModal returns a modal in loading state with name field focused.
func NewCreateAndroidEmulatorModal() (CreateAndroidEmulatorModal, tea.Cmd) {
	ti := textinput.New()
	ti.Placeholder = "e.g. Pixel_8_API34"
	ti.CharLimit = 64
	ti.SetWidth(androidModalW - 8)
	cmd := ti.Focus()
	return CreateAndroidEmulatorModal{
		nameInput: ti,
		loading:   true,
		focused:   fieldName,
	}, cmd
}

// SetSystemImages populates the system image list; clears loading when both lists ready.
func (m *CreateAndroidEmulatorModal) SetSystemImages(imgs []device.Runtime) {
	m.systemImages = imgs
	if m.deviceProfiles != nil {
		m.loading = false
	}
}

// SetDeviceProfiles populates the device profile list; clears loading when both lists ready.
func (m *CreateAndroidEmulatorModal) SetDeviceProfiles(profiles []device.DeviceType) {
	m.deviceProfiles = profiles
	if m.systemImages != nil {
		m.loading = false
	}
}

func (m CreateAndroidEmulatorModal) canSubmit() bool {
	return strings.TrimSpace(m.nameInput.Value()) != "" &&
		len(m.systemImages) > 0 && m.imgIdx < len(m.systemImages)
}

func (m CreateAndroidEmulatorModal) Update(msg tea.Msg) (CreateAndroidEmulatorModal, tea.Cmd) {
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
		// fieldDeviceType = system images (required), fieldRuntime = device profiles (optional)
		if m.focused == fieldRuntime && m.canSubmit() {
			return m, m.buildConfirmCmd()
		}
		if m.focused < fieldRuntime {
			m.focused++
			m.nameInput.Blur()
		}

	case "up", "k":
		switch m.focused {
		case fieldDeviceType:
			if m.imgIdx > 0 {
				m.imgIdx--
				if m.imgIdx < m.imgOffset {
					m.imgOffset = m.imgIdx
				}
			}
		case fieldRuntime:
			if m.profIdx > 0 {
				m.profIdx--
				if m.profIdx < m.profOffset {
					m.profOffset = m.profIdx
				}
			}
		}

	case "down", "j":
		switch m.focused {
		case fieldDeviceType:
			if m.imgIdx < len(m.systemImages)-1 {
				m.imgIdx++
				if m.imgIdx >= m.imgOffset+listShowRows {
					m.imgOffset = m.imgIdx - listShowRows + 1
				}
			}
		case fieldRuntime:
			if m.profIdx < len(m.deviceProfiles)-1 {
				m.profIdx++
				if m.profIdx >= m.profOffset+listShowRows {
					m.profOffset = m.profIdx - listShowRows + 1
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

func (m CreateAndroidEmulatorModal) buildConfirmCmd() tea.Cmd {
	name := strings.TrimSpace(m.nameInput.Value())
	sysPkg := m.systemImages[m.imgIdx].Identifier
	profID := ""
	if len(m.deviceProfiles) > 0 && m.profIdx < len(m.deviceProfiles) {
		profID = m.deviceProfiles[m.profIdx].Identifier
	}
	return func() tea.Msg {
		return ConfirmCreateAndroidEmulatorMsg{
			Name:            name,
			SystemImagePkg:  sysPkg,
			DeviceProfileID: profID,
		}
	}
}

func (m CreateAndroidEmulatorModal) View() string {
	innerW := androidModalW - 6

	faint := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	hintKey := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true)
	hintVerb := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	labelAct := lipgloss.NewStyle().Foreground(ColorAndroid).Background(ColorBg).Bold(true)
	labelInact := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
	itemSel := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAndroid).Bold(true)
	itemNorm := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorAndroid).
		Background(ColorBg).
		Padding(1, 2)

	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(ColorAndroid).Background(ColorBg).Bold(true).Render("▲ New Android Emulator"))
	b.WriteString("\n\n")

	if m.loading {
		b.WriteString(faint.Render("  Loading system images and device profiles…"))
		return box.Width(androidModalW).Render(b.String())
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

	// System Image (uses fieldDeviceType slot, but labeled "System Image")
	lbl = labelInact
	if m.focused == fieldDeviceType {
		lbl = labelAct
	}
	b.WriteString(modalListHeader(lbl.Render("System Image"), len(m.systemImages), m.imgIdx, innerW))
	b.WriteString("\n")
	b.WriteString(renderRuntimeList(m.systemImages, m.imgIdx, m.imgOffset, m.focused == fieldDeviceType, innerW, itemSel, itemNorm, faint))
	b.WriteString("\n\n")

	// Device Profile (optional) (uses fieldRuntime slot)
	lbl = labelInact
	if m.focused == fieldRuntime {
		lbl = labelAct
	}
	optLabel := lbl.Render("Device Profile") + faint.Render(" (optional)")
	b.WriteString(modalListHeader(optLabel, len(m.deviceProfiles), m.profIdx, innerW))
	b.WriteString("\n")
	b.WriteString(renderDeviceTypeList(m.deviceProfiles, m.profIdx, m.profOffset, m.focused == fieldRuntime, innerW, itemSel, itemNorm, faint))
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

	return box.Width(androidModalW).Render(b.String())
}
