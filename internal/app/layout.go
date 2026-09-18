package app

// rect is a screen region in absolute (col, row) coordinates, matching the
// coordinate space tea.Mouse events use.
type rect struct{ x, y, w, h int }

// contains reports whether (x, y) falls inside the rect. A zero-value rect
// never contains anything, which is the safe default before the first
// render.
func (r rect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.w && y >= r.y && y < r.y+r.h
}

// appLayout captures where the sidebar, main pane, and (if one is open) the
// topmost modal/alert were drawn on the last frame. It is populated at the
// end of View() and read by the mouse-click dispatch in Update(), so
// hit-testing always matches exactly what's on screen — there is no second
// copy of the layout math to keep in sync with rendering.
type appLayout struct {
	sidebar  rect
	mainPane rect
	// overlay is the bounding box of whichever modal/alert is currently
	// topmost (only meaningful while one of the simple alert modals is
	// open — see the mouse-click dispatch in update.go).
	overlay rect
}
