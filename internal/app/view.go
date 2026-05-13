package app

import (
	"simmer/internal/ui"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func (m model) View() tea.View {
	topBar := ui.RenderTopBar(ui.TopBarParams{
		Width:        m.width,
		AppVersion:   m.appVersion,
		BootedCount:  m.bootedCount,
		IOSCount:     m.iosCount,
		AndroidCount: m.androidCount,
		ToolVersions: m.toolVersions,
	})

	footerHelp := ui.NewHelpModel()
	footerHelp.SetWidth(m.width - 4)
	footer := ui.RenderFooter(ui.FooterParams{
		Width:  m.width,
		Status: m.status,
		Kind:   m.statusKind,
		Help:   footerHelp,
	})

	bodyH := max(m.height-lipgloss.Height(topBar)-lipgloss.Height(footer), 0)

	var body string
	if m.loading && m.sidebar.SelectedDevice() == nil {
		body = "\n  " + lipgloss.NewStyle().
			Foreground(ui.ColorFgFaint).
			Background(ui.ColorBg).
			Render("fetching devices…")
	} else {
		sidebar := lipgloss.NewStyle().
			Padding(1, 1, 0, 1).
			Background(ui.ColorBg).
			Render(m.sidebar.View())
		mainPane := lipgloss.NewStyle().
			Padding(1, 1, 0, 0).
			Background(ui.ColorBg).
			Render(m.mainPane.View())
		body = lipgloss.JoinHorizontal(lipgloss.Top, sidebar, mainPane)
	}

	body = lipgloss.NewStyle().
		Width(m.width).
		Height(bodyH).
		Background(ui.ColorBg).
		Render(body)

	baseStr := lipgloss.JoinVertical(lipgloss.Left, topBar, body, footer)

	// Primary overlays (db viewer, create/delete dialogs). Rendered before help
	// so the help panel always sits on top.
	if m.platformPicker != nil || m.createIOSModal != nil || m.createAndModal != nil || m.deleteAlert != nil || m.deleteAppAlert != nil || m.sqliteModal != nil || m.dbViewerModal != nil {
		var overlayStr string
		switch {
		case m.dbViewerModal != nil:
			overlayStr = m.dbViewerModal.View()
		case m.platformPicker != nil:
			overlayStr = m.platformPicker.View()
		case m.createIOSModal != nil:
			overlayStr = m.createIOSModal.View()
		case m.createAndModal != nil:
			overlayStr = m.createAndModal.View()
		case m.sqliteModal != nil:
			overlayStr = m.sqliteModal.View()
		case m.deleteAppAlert != nil:
			overlayStr = m.deleteAppAlert.View()
		default:
			overlayStr = m.deleteAlert.View()
		}
		mW := lipgloss.Width(overlayStr)
		mH := lipgloss.Height(overlayStr)
		x := max((m.width-mW)/2, 0)
		y := max((m.height-mH)/2, 0)
		bg := lipgloss.NewLayer(baseStr)
		fg := lipgloss.NewLayer(overlayStr).X(x).Y(y).Z(1)
		if m.dbViewerModal != nil {
			scrim := lipgloss.NewLayer(m.dbViewerModal.ScrimView()).Z(0)
			baseStr = lipgloss.NewCompositor(bg, scrim, fg).Render()
		} else {
			baseStr = lipgloss.NewCompositor(bg, fg).Render()
		}
	}

	// Help overlay is always the topmost layer so it appears over the db viewer.
	if m.helpOverlay != nil {
		overlayStr := m.helpOverlay.View()
		mW := lipgloss.Width(overlayStr)
		mH := lipgloss.Height(overlayStr)
		x := max((m.width-mW)/2, 0)
		y := max((m.height-mH)/2, 0)
		bg := lipgloss.NewLayer(baseStr)
		fg := lipgloss.NewLayer(overlayStr).X(x).Y(y).Z(1)
		baseStr = lipgloss.NewCompositor(bg, fg).Render()
	}

	v := tea.NewView(baseStr)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	v.WindowTitle = "Simmer"
	v.BackgroundColor = ui.ColorBg
	v.KeyboardEnhancements.ReportAllKeysAsEscapeCodes = true
	return v
}
