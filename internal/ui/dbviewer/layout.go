package dbviewer

// layout holds all pre-computed dimensions for a single render frame.
// Call computeLayout once per frame and pass the result to all sub-renderers.
type layout struct {
	ModalW, ModalH          int
	InnerW, InnerH          int
	SidebarW, DivW, RightW  int
	BodyH, QueryH, ResultsH int
}

const (
	sidebarW = 28
	divW     = 1
)

func computeLayout(termW, termH int) layout {
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
	rightW := innerW - sidebarW - divW

	// bodyH excludes title row, title separator, status separator, and status bar.
	const titleRows = 2
	const statusRows = 2
	bodyH := innerH - titleRows - statusRows

	queryH := max(bodyH/3, 3)
	resultsH := bodyH - queryH

	return layout{
		ModalW:   modalW,
		ModalH:   modalH,
		InnerW:   innerW,
		InnerH:   innerH,
		SidebarW: sidebarW,
		DivW:     divW,
		RightW:   rightW,
		BodyH:    bodyH,
		QueryH:   queryH,
		ResultsH: resultsH,
	}
}
