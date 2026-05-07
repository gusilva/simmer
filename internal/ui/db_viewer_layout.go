package ui

// dbViewerLayout holds all pre-computed dimensions for a single render frame.
// Call computeLayout once per frame and pass the result to all sub-renderers.
type dbViewerLayout struct {
	ModalW, ModalH int
	InnerW, InnerH int
	SidebarW, DivW, RightW int
	BodyH, QueryH, ResultsH int
	// DivRow is the body-relative row index of the horizontal divider between
	// the query pane and the results pane.
	DivRow int
}

const (
	dbSidebarW = 28
	dbDivW     = 1
)

func computeDBViewerLayout(termW, termH int) dbViewerLayout {
	modalW := termW - 4
	if termW <= 4 {
		modalW = 30
	}
	modalH := termH - 4
	if termH <= 4 {
		modalH = 10
	}

	innerW := modalW - 2
	innerH := modalH - 2
	rightW := innerW - dbSidebarW - dbDivW

	// bodyH excludes title row, title separator, status separator, and status bar.
	const titleRows = 2 // title row + title separator
	const statusRows = 2 // status separator + status bar
	bodyH := innerH - titleRows - statusRows

	queryH := max(bodyH/3, 3)
	resultsH := bodyH - queryH - 1

	return dbViewerLayout{
		ModalW:   modalW,
		ModalH:   modalH,
		InnerW:   innerW,
		InnerH:   innerH,
		SidebarW: dbSidebarW,
		DivW:     dbDivW,
		RightW:   rightW,
		BodyH:    bodyH,
		QueryH:   queryH,
		ResultsH: resultsH,
		DivRow:   queryH,
	}
}
