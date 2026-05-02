package ui

import (
	"fmt"
	"strings"

	"simmer/pkg/device"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// ── Messages ────────────────────────────────────────────────────────────────

// ShowPlatformPickerMsg is emitted by Sidebar when 'a' is pressed in Available pane.
type ShowPlatformPickerMsg struct{}

// ConfirmPlatformPickerMsg is emitted when the user picks a platform.
type ConfirmPlatformPickerMsg struct{ Platform device.Platform }

// ShowDeleteSimulatorMsg is emitted by Sidebar when 'd' is pressed on a selected device.
type ShowDeleteSimulatorMsg struct{ Device device.Device }

// ConfirmCreateSimulatorMsg is emitted when the iOS create form is submitted.
type ConfirmCreateSimulatorMsg struct {
	Name         string
	DeviceTypeID string
	RuntimeID    string
}

// ConfirmCreateAndroidEmulatorMsg is emitted when the Android create form is submitted.
type ConfirmCreateAndroidEmulatorMsg struct {
	Name            string
	SystemImagePkg  string
	DeviceProfileID string // may be empty
}

// ConfirmDeleteSimulatorMsg is emitted when the user confirms deletion.
type ConfirmDeleteSimulatorMsg struct{ Device device.Device }

// CancelOverlayMsg is emitted when the user dismisses any overlay.
type CancelOverlayMsg struct{}

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

// ── DeleteSimulatorAlert ─────────────────────────────────────────────────────

const deleteAlertW = 50

// DeleteSimulatorAlert is a confirmation dialog before deleting a simulator.
type DeleteSimulatorAlert struct {
	device       device.Device
	confirmFocus bool // false = Cancel focused, true = Delete focused
}

// NewDeleteSimulatorAlert returns an alert with Cancel focused by default.
func NewDeleteSimulatorAlert(dev device.Device) DeleteSimulatorAlert {
	return DeleteSimulatorAlert{device: dev}
}

// Update handles key events for the delete confirmation.
func (m DeleteSimulatorAlert) Update(msg tea.Msg) (DeleteSimulatorAlert, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "esc", "n", "N":
		return m, func() tea.Msg { return CancelOverlayMsg{} }
	case "y", "Y":
		dev := m.device
		return m, func() tea.Msg { return ConfirmDeleteSimulatorMsg{Device: dev} }
	case "enter":
		if m.confirmFocus {
			dev := m.device
			return m, func() tea.Msg { return ConfirmDeleteSimulatorMsg{Device: dev} }
		}
		return m, func() tea.Msg { return CancelOverlayMsg{} }
	case "tab", "left", "h", "right", "l":
		m.confirmFocus = !m.confirmFocus
	}
	return m, nil
}

// View renders the alert box as an ANSI string for overlay placement.
func (m DeleteSimulatorAlert) View() string {
	innerW := deleteAlertW - 6

	norm := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg)
	faint := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg)
	hintKey := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true)
	hintVerb := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)

	// Buttons: always rounded-border so both are 3 lines tall and align correctly.
	cancelStyle := lipgloss.NewStyle().
		Foreground(ColorFgDim).Background(ColorBg).
		Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).
		Padding(0, 2)
	cancelFocusStyle := lipgloss.NewStyle().
		Foreground(ColorBg).Background(ColorFgDim).Bold(true).
		Border(lipgloss.RoundedBorder()).BorderForeground(ColorFgDim).
		Padding(0, 2)
	deleteStyle := lipgloss.NewStyle().
		Foreground(ColorErr).Background(ColorBg).
		Border(lipgloss.RoundedBorder()).BorderForeground(ColorBorder).
		Padding(0, 2)
	deleteFocusStyle := lipgloss.NewStyle().
		Foreground(ColorBg).Background(ColorErr).Bold(true).
		Border(lipgloss.RoundedBorder()).BorderForeground(ColorErr).
		Padding(0, 2)

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorErr).
		Background(ColorBg).
		Padding(1, 2)

	name := truncateName(m.device.Name, innerW-4)

	var b strings.Builder
	titleStr := "Delete Simulator"
	if m.device.Platform == device.PlatformAndroid {
		titleStr = "Delete Emulator"
	}
	b.WriteString(lipgloss.NewStyle().Foreground(ColorErr).Background(ColorBg).Bold(true).Render(titleStr))
	b.WriteString("\n\n")
	b.WriteString(norm.Render(`"` + name + `"`))
	b.WriteString("\n")
	b.WriteString(faint.Render("This cannot be undone."))
	b.WriteString("\n\n")

	cancelBtn := cancelStyle.Render("Cancel")
	deleteBtn := deleteStyle.Render("Delete")
	if !m.confirmFocus {
		cancelBtn = cancelFocusStyle.Render("Cancel")
	} else {
		deleteBtn = deleteFocusStyle.Render("Delete")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Top, cancelBtn, "   ", deleteBtn)
	b.WriteString(lipgloss.NewStyle().Width(innerW).Align(lipgloss.Center).Background(ColorBg).Render(buttons))
	b.WriteString("\n\n")

	sep := hintVerb.Render("  ")
	hints := strings.Join([]string{
		hintKey.Render("Tab") + hintVerb.Render(" switch"),
		hintKey.Render("y") + hintVerb.Render(" delete"),
		hintKey.Render("Esc") + hintVerb.Render(" cancel"),
	}, sep)
	b.WriteString(hints)

	return box.Width(deleteAlertW).Render(b.String())
}

// ── PlatformPickerModal ──────────────────────────────────────────────────────

const platformPickerW = 46

var platformPickerItems = []struct {
	label    string
	platform device.Platform
}{
	{"iOS Simulator", device.PlatformIOS},
	{"Android Emulator", device.PlatformAndroid},
}

// PlatformPickerModal lets the user choose iOS or Android before creating a device.
type PlatformPickerModal struct {
	idx int
}

// NewPlatformPickerModal returns a picker with iOS selected by default.
func NewPlatformPickerModal() PlatformPickerModal { return PlatformPickerModal{} }

func (m PlatformPickerModal) Update(msg tea.Msg) (PlatformPickerModal, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch k.String() {
	case "esc":
		return m, func() tea.Msg { return CancelOverlayMsg{} }
	case "up", "k":
		if m.idx > 0 {
			m.idx--
		}
	case "down", "j":
		if m.idx < len(platformPickerItems)-1 {
			m.idx++
		}
	case "enter":
		p := platformPickerItems[m.idx].platform
		return m, func() tea.Msg { return ConfirmPlatformPickerMsg{Platform: p} }
	}
	return m, nil
}

func (m PlatformPickerModal) View() string {
	innerW := platformPickerW - 6

	title := lipgloss.NewStyle().Foreground(ColorAccent).Background(ColorBg).Bold(true).Render("Add Device")
	lbl := lipgloss.NewStyle().Foreground(ColorFgDim).Background(ColorBg).Render("Select platform:")
	itemSel := lipgloss.NewStyle().Foreground(ColorBg).Background(ColorAccent).Bold(true)
	itemNorm := lipgloss.NewStyle().Foreground(ColorFg).Background(ColorBg)
	hintKey := lipgloss.NewStyle().Foreground(ColorBorderHi).Background(ColorBg).Bold(true)
	hintVerb := lipgloss.NewStyle().Foreground(ColorFgFaint).Background(ColorBg)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorBorderHi).
		Background(ColorBg).
		Padding(1, 2)

	var b strings.Builder
	b.WriteString(title)
	b.WriteString("\n\n")
	b.WriteString(lbl)
	b.WriteString("\n\n")

	for i, item := range platformPickerItems {
		name := item.label
		if i == m.idx {
			pad := strings.Repeat(" ", max(innerW-3-lipgloss.Width(name), 1))
			b.WriteString(itemSel.Render(" ▸ " + name + pad))
		} else {
			b.WriteString(itemNorm.Render("   " + name))
		}
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

	return box.Width(platformPickerW).Render(b.String())
}

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
