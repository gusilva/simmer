package ui

import (
	"testing"

	"charm.land/bubbles/v2/key"
)

// Verify that all key maps satisfy the help.KeyMap interface via ShortHelp and
// FullHelp. These calls also drive coverage for the otherwise-zero-percent
// FullHelp implementations.

func TestGlobalKeyMap_Help(t *testing.T) {
	short := GlobalKeys.ShortHelp()
	if len(short) == 0 {
		t.Error("GlobalKeys.ShortHelp() must not be empty")
	}
	full := GlobalKeys.FullHelp()
	if len(full) == 0 {
		t.Error("GlobalKeys.FullHelp() must not be empty")
	}
	for i, row := range full {
		if len(row) == 0 {
			t.Errorf("GlobalKeys.FullHelp() row %d is empty", i)
		}
	}
}

func TestSidebarKeyMap_Help(t *testing.T) {
	short := SidebarKeys.ShortHelp()
	if len(short) == 0 {
		t.Error("SidebarKeys.ShortHelp() must not be empty")
	}
	full := SidebarKeys.FullHelp()
	if len(full) == 0 {
		t.Error("SidebarKeys.FullHelp() must not be empty")
	}
}

func TestMainPaneKeyMap_Help(t *testing.T) {
	short := MainPaneKeys.ShortHelp()
	if len(short) == 0 {
		t.Error("MainPaneKeys.ShortHelp() must not be empty")
	}
	full := MainPaneKeys.FullHelp()
	if len(full) == 0 {
		t.Error("MainPaneKeys.FullHelp() must not be empty")
	}
}

func TestMainPaneKeyMap_FullHelp_IncludesAllAppsShortcuts(t *testing.T) {
	full := MainPaneKeys.FullHelp()
	var flat []key.Binding
	for _, row := range full {
		flat = append(flat, row...)
	}

	want := map[string]key.Binding{
		"filter apps":         MainPaneKeys.Filter,
		"install app":         MainPaneKeys.Install,
		"delete app":          MainPaneKeys.Delete,
		"browse app files":    MainPaneKeys.PinApp,
		"log app to file":     MainPaneKeys.Log,
		"rebuild & reinstall": MainPaneKeys.Rebuild,
	}
	for desc, binding := range want {
		found := false
		for _, b := range flat {
			if b.Help() == binding.Help() {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected Apps tab shortcut %q in FullHelp(), got %v", desc, full)
		}
	}
}

func TestInstallPickerKeyMap_Help(t *testing.T) {
	short := InstallPickerKeys.ShortHelp()
	if len(short) == 0 {
		t.Error("InstallPickerKeys.ShortHelp() must not be empty")
	}
	full := InstallPickerKeys.FullHelp()
	if len(full) == 0 {
		t.Error("InstallPickerKeys.FullHelp() must not be empty")
	}
}

func TestDBViewerKeyMap_Help(t *testing.T) {
	short := DBViewerKeys.ShortHelp()
	if len(short) == 0 {
		t.Error("DBViewerKeys.ShortHelp() must not be empty")
	}
	full := DBViewerKeys.FullHelp()
	if len(full) == 0 {
		t.Error("DBViewerKeys.FullHelp() must not be empty")
	}
}
