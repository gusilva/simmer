package ui

import (
	"fmt"
	"io"
	"strings"

	"simmer/internal/device"

	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Item wraps a device.Device for use in a bubbles list.
type Item struct{ Dev device.Device }

func (i Item) FilterValue() string { return i.Dev.Name }
func (i Item) Title() string       { return i.Dev.Name }
func (i Item) Description() string { return string(i.Dev.Platform) + " " + i.Dev.Version }

// DeviceDelegate renders each device row in the list.
type DeviceDelegate struct{}

func (d DeviceDelegate) Height() int                             { return 1 }
func (d DeviceDelegate) Spacing() int                            { return 0 }
func (d DeviceDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d DeviceDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(Item)
	if !ok {
		return
	}
	dev := i.Dev
	isSelected := index == m.Index()
	width := m.Width()

	dotC := ColorFgFaint
	if dev.Status == device.StatusRunning {
		dotC = ColorOk
	}

	pglyph := ""
	pgC := ColorIOS
	if dev.Platform != device.PlatformIOS {
		pglyph = "▲"
		pgC = ColorAndroid
	}

	ver := dev.Version

	if isSelected {
		left := "● " + pglyph + " " + dev.Name
		avail := width - 2
		gap := avail - lipgloss.Width(left) - lipgloss.Width(ver)
		if gap < 1 {
			gap = 1
		}
		fmt.Fprint(w, lipgloss.NewStyle().
			Background(ColorAccent).
			Foreground(ColorBg).
			Bold(true).
			Padding(0, 1).
			Render(left+strings.Repeat(" ", gap)+ver))
	} else {
		dot := lipgloss.NewStyle().Foreground(dotC).Render("●")
		glyph := lipgloss.NewStyle().Foreground(pgC).Render(pglyph)
		name := lipgloss.NewStyle().Foreground(ColorFg).Render(dev.Name)
		meta := lipgloss.NewStyle().Foreground(ColorFgFaint).Render(ver)
		left := dot + " " + glyph + " " + name
		avail := width - 2
		gap := avail - lipgloss.Width(left) - lipgloss.Width(ver)
		if gap < 1 {
			gap = 1
		}
		fmt.Fprint(w, lipgloss.NewStyle().
			Background(ColorBg).
			Padding(0, 1).
			Render(left+strings.Repeat(" ", gap)+meta))
	}
}

// NewListStyles returns a customised bubbles list.Styles.
func NewListStyles() list.Styles {
	s := list.DefaultStyles(true)

	s.StatusEmpty = lipgloss.NewStyle().
		Background(ColorBg).
		Foreground(ColorFgFaint)
	s.NoItems = lipgloss.NewStyle().
		Background(ColorBg).
		Foreground(ColorFgFaint).
		Padding(0, 2)
	s.PaginationStyle = lipgloss.NewStyle().
		Background(ColorBg).
		Foreground(ColorFgFaint).
		PaddingLeft(2)
	s.ActivePaginationDot = lipgloss.NewStyle().
		Background(ColorBg).
		Foreground(ColorAccent).
		SetString("•")
	s.InactivePaginationDot = lipgloss.NewStyle().
		Background(ColorBg).
		Foreground(ColorBorder).
		SetString("•")
	s.DividerDot = lipgloss.NewStyle().
		Background(ColorBg).
		Foreground(ColorFgFaint).
		SetString(" • ")

	s.Filter = textinput.DefaultStyles(true)
	s.Filter.Focused.Prompt = lipgloss.NewStyle().Foreground(ColorAccent)
	s.Filter.Blurred.Prompt = lipgloss.NewStyle().Foreground(ColorAccent)
	s.Filter.Cursor.Color = ColorAccent

	return s
}
