package dbviewer

import "charm.land/bubbles/v2/table"

// fixedRenderedW is the sum of all column rendered widths (content + padding 0,1 = +2)
// except the flex "name" column:
//
//	#(3+2) + udid(14+2) + os(8+2) + state(10+2) + runtime(7+2) +
//	apps(4+2) + cpu_pct(7+2) + booted_at(16+2) = 85
const fixedRenderedW = 85

func defaultResultsColumns(nameW int) []table.Column {
	return []table.Column{
		{Title: "#", Width: 3},
		{Title: "⚿ udid", Width: 14},
		{Title: "name", Width: nameW},
		{Title: "os", Width: 8},
		{Title: "state", Width: 10},
		{Title: "runtime", Width: 7},
		{Title: "apps", Width: 4},
		{Title: "cpu_pct", Width: 7},
		{Title: "booted_at", Width: 16},
	}
}

func defaultResultsRows() []table.Row {
	return []table.Row{
		{"1", "9C3A2F-…-71BE", "iPhone 17 Pro", "iOS", "● Booted", "26.1", "142", "12.4", "2026-04-28 13:40"},
		{"2", "F2E4A1-…-A09D", "iPhone 17", "iOS", "● Booted", "26.1", "87", "4.1", "2026-04-28 11:08"},
		{"3", "B71D08-…-3F12", "iPhone 16 Pro Max", "iOS", "◐ Booting", "26.0", "54", "22.7", "2026-04-28 10:44"},
		{"4", "3A0928-…-CE51", `iPad Pro 13"`, "iPadOS", "○ Shutdown", "26.1", "0", "NULL", "2026-04-27 18:22"},
		{"5", "5D33C9-…-9182", "Pixel 9 Pro", "Android", "● Booted", "15.0", "33", "8.6", "2026-04-28 09:11"},
		{"6", "E81F44-…-7AB2", "Pixel 9", "Android", "○ Shutdown", "15.0", "0", "NULL", "2026-04-26 22:06"},
		{"7", "2F87C3-…-D040", "Galaxy S25 Ultra", "Android", "● Booted", "15.0", "61", "7.9", "2026-04-28 08:49"},
		{"8", "7B9211-…-15CA", "Apple Watch S10", "watchOS", "◐ Booting", "12.1", "4", "15.2", "2026-04-28 07:33"},
		{"9", "CC4E1A-…-8E07", "Apple TV 4K", "tvOS", "○ Shutdown", "19.0", "2", "NULL", "2026-04-25 16:58"},
		{"10", "81A655-…-B339", "Vision Pro", "visionOS", "● Booted", "3.2", "11", "18.0", "2026-04-28 06:14"},
		{"11", "A0DE77-…-F284", "iPhone 15 Pro", "iOS", "○ Shutdown", "26.1", "0", "NULL", "2026-04-24 19:22"},
		{"12", "D5B3F0-…-C711", "Pixel Tablet", "Android", "○ Shutdown", "15.0", "0", "NULL", "2026-04-23 12:01"},
		{"13", "4F2B89-…-A0DE", "Galaxy Z Fold 6", "Android", "○ Shutdown", "14.0", "0", "NULL", "2026-04-22 09:45"},
		{"14", "61E3C8-…-7DD4", "iPhone SE (3rd gen)", "iOS", "○ Shutdown", "26.1", "0", "NULL", "2026-04-21 14:07"},
	}
}
