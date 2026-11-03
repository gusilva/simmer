package app

import (
	"strings"
	"testing"

	"simmer/internal/device"
	"simmer/internal/ui"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// sizedModel is a TUI with a device and its apps loaded, optionally with the
// DB viewer open, at the given terminal size.
func sizedModel(t *testing.T, w, h int, withDB bool) model {
	t.Helper()
	dev := device.Device{ID: "d", Name: "Pixel 9 Pro API 35", Platform: device.PlatformAndroid,
		Kind: device.KindVirtual, Status: device.StatusRunning}
	apps := []device.App{{BundleID: "com.osapiens.operations.maintenanceos.qa", Name: "Maintenance OS QA"}}
	m := newTestModel()
	for _, msg := range []tea.Msg{
		tea.WindowSizeMsg{Width: w, Height: h},
		discoveryMsg{Devices: []device.Device{dev}},
		ui.LoadDeviceMsg{Device: dev},
		appsListMsg{device: dev, apps: apps},
	} {
		m, _ = step(t, m, msg)
	}
	if withDB {
		name := "john.doe@fo-gustavo.com#BusinessObject.db"
		m, _ = step(t, m, ui.ShowSQLiteViewerMsg{Device: dev, DBPath: "/data/data/com.x/databases/" + name, DBName: name})
	}
	return m
}

// assertFrame checks the frame covers the terminal exactly: any line wider
// or any extra row means something wrapped or overflowed.
func assertFrame(t *testing.T, m model) string {
	t.Helper()
	content := m.View().Content
	lines := strings.Split(content, "\n")
	if len(lines) != m.height {
		t.Fatalf("%dx%d: %d rows, want %d", m.width, m.height, len(lines), m.height)
	}
	for i, l := range lines {
		if got := ansi.StringWidth(l); got != m.width {
			t.Fatalf("%dx%d: row %d is %d cells, want %d:\n%s", m.width, m.height, i, got, m.width, ansi.Strip(l))
		}
	}
	return ansi.Strip(content)
}

func TestLayout_FitsFromTheMinimumSize(t *testing.T) {
	for _, withDB := range []bool{false, true} {
		for _, sz := range [][2]int{{minTermWidth, minTermHeight}, {minTermWidth + 1, minTermHeight + 1}, {120, 40}, {200, 60}} {
			m := sizedModel(t, sz[0], sz[1], withDB)
			if strings.Contains(assertFrame(t, m), "too small") {
				t.Fatalf("%v: prompt shown at a supported size", sz)
			}
		}
	}
}

func TestLayout_BelowMinimumShowsPrompt(t *testing.T) {
	// Includes the heights that used to panic the DB viewer.
	for _, sz := range [][2]int{{minTermWidth - 1, 40}, {120, minTermHeight - 1}, {120, 5}, {40, 10}} {
		for _, withDB := range []bool{false, true} {
			m := sizedModel(t, sz[0], sz[1], withDB)
			out := assertFrame(t, m)
			if !strings.Contains(out, "Terminal size too small") || !strings.Contains(out, "Width = 80 Height = 24") {
				t.Fatalf("%v: no resize prompt:\n%s", sz, out)
			}
		}
	}
}

func TestLayout_BelowMinimumSwallowsInputButQuits(t *testing.T) {
	m := sizedModel(t, 60, 20, false)
	before := m.mainPane.SelectedApp()
	m, cmd := step(t, m, tea.KeyPressMsg{Code: 'j', Text: "j"})
	if cmd != nil || m.mainPane.SelectedApp().BundleID != before.BundleID {
		t.Fatal("keys must be swallowed while the prompt is up")
	}
	if _, cmd := step(t, m, tea.MouseClickMsg{Button: tea.MouseLeft, X: 5, Y: 5}); cmd != nil {
		t.Fatal("clicks must be swallowed while the prompt is up")
	}
	if m, _ = step(t, m, tea.KeyPressMsg{Code: 'q', Text: "q"}); !m.quitting {
		t.Fatal("q must still quit")
	}

	// Growing the terminal brings the TUI back where it was.
	m = sizedModel(t, 60, 20, false)
	m, _ = step(t, m, tea.WindowSizeMsg{Width: 100, Height: 30})
	if strings.Contains(assertFrame(t, m), "too small") {
		t.Fatal("prompt should go away once the terminal is big enough")
	}
}

// The Files panel splits its width between tree and preview, so every row of
// either column must fit at every terminal width, not just the minimum.
func TestLayout_FilesPanelFitsAtEveryWidth(t *testing.T) {
	long := "com.android.adservices.api.something.very.long.package.name"
	root := device.FileNode{Name: "/", Path: "/", IsDir: true, Children: []device.FileNode{
		{Name: "acct", Path: "/acct", IsDir: true, Permissions: "drwxr-xr-x"},
		{Name: long, Path: "/" + long, IsDir: true, Permissions: "drwxr-xr-x",
			Children: []device.FileNode{{Name: "a", Path: "/" + long + "/a"}}},
		{Name: long + ".db", Path: "/" + long + ".db", Size: 123456789},
		{Name: "trace.json", Path: "/trace.json", Size: 2048},
	}}
	for w := minTermWidth; w <= 200; w = nextWidth(w) {
		m := sizedModel(t, w, minTermHeight+6, false)
		m.mainPane.ShowFilesPanel()
		m.mainPane.SetTree(&root)
		// Walk the cursor over every row so each preview kind is drawn.
		for range root.Children {
			assertFrame(t, m)
			m.mainPane, _ = m.mainPane.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
		}
	}
}

// Every screen, with names long enough to need truncating, must fill the
// terminal exactly at every supported width and at several heights.
func TestLayout_EveryScreenFitsAtEverySupportedSize(t *testing.T) {
	long := "com.android.adservices.api.something.very.long.package.name"
	devs := []device.Device{
		{ID: "d", Name: "Pixel 9 Pro API 35 with an unusually long AVD name", Platform: device.PlatformAndroid,
			Kind: device.KindVirtual, Status: device.StatusRunning, Version: "35"},
		{ID: "i", Name: "iPad Pro 13-inch (M5) with a long simulator name", Platform: device.PlatformIOS,
			Kind: device.KindVirtual, Status: device.StatusOff, Version: "26.2"},
	}
	apps := []device.App{{BundleID: long, Name: "An App With A Rather Long Display Name", Type: "User"}}
	matches := []device.FileMatch{{Name: long + ".db", Path: "/data/data/" + long + "/databases/" + long + ".db", Size: 1 << 20}}

	screens := map[string]func(m model) model{
		"apps": func(m model) model { return m },
		"files": func(m model) model {
			m.mainPane.ShowFilesPanel()
			m.mainPane.SetTree(&device.FileNode{Name: "/", Path: "/", IsDir: true,
				Children: []device.FileNode{{Name: long, Path: "/" + long, IsDir: true}}})
			return m
		},
		"search": func(m model) model {
			m.mainPane.ShowFilesPanel()
			m.mainPane.SetFileSearchResults(long, matches)
			return m
		},
		"db": func(m model) model {
			m, _ = step(t, m, ui.ShowSQLiteViewerMsg{Device: devs[0], DBPath: matches[0].Path, DBName: matches[0].Name})
			return m
		},
	}
	for name, open := range screens {
		for _, h := range []int{minTermHeight, 31, 50} {
			// Every width where truncation decisions flip, then a sample of
			// wide terminals.
			for w := minTermWidth; w <= 200; w = nextWidth(w) {
				m := newTestModel()
				for _, msg := range []tea.Msg{
					tea.WindowSizeMsg{Width: w, Height: h},
					discoveryMsg{Devices: devs},
					ui.LoadDeviceMsg{Device: devs[0]},
					appsListMsg{device: devs[0], apps: apps},
				} {
					m, _ = step(t, m, msg)
				}
				m = open(m)
				t.Run(name, func(t *testing.T) { assertFrame(t, m) })
				if t.Failed() {
					return
				}
			}
		}
	}
}

func nextWidth(w int) int {
	if w < 130 {
		return w + 1
	}
	return w + 7
}
