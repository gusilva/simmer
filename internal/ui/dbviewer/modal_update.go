package dbviewer

import tea "charm.land/bubbletea/v2"

func (m Modal) Update(msg tea.Msg) (Modal, tea.Cmd) {
	if vm, ok := msg.(SQLiteVersionMsg); ok {
		if vm.Err == nil {
			m.sqliteVersion = vm.Version
		}
		return m, nil
	}

	if tm, ok := msg.(TablesLoadedMsg); ok {
		if tm.Err == nil {
			if ep, ok := m.panes[paneSidebar].(explorerPane); ok {
				ep.inner.SetTables(m.dbName, tm.Objects)
				m.panes[paneSidebar] = ep
			}
		}
		return m, nil
	}

	if _, ok := msg.(FileSavedMsg); ok {
		if qp, ok := m.panes[paneQuery].(queryPaneAdapter); ok {
			next, cmd := qp.inner.Update(msg)
			m.panes[paneQuery] = queryPaneAdapter{next}
			return m, cmd
		}
		return m, nil
	}

	if k, ok := msg.(tea.KeyPressMsg); ok && (k.String() == "ctrl+s" || k.String() == "super+s") {
		if qp, ok := m.panes[paneQuery].(queryPaneAdapter); ok {
			return m, qp.inner.SaveCmd()
		}
		return m, nil
	}

	if cm, ok := msg.(ColumnsLoadedMsg); ok {
		cm.node.loading = false
		if cm.err == nil {
			cm.node.children = buildColumnNodes(cm.cols)
			cm.node.expanded = true
		}
		return m, nil
	}

	if qr, ok := msg.(QueryResultMsg); ok {
		if rp, ok := m.panes[paneResults].(resultsPaneAdapter); ok {
			if qr.Err != nil {
				rp.inner.SetError(qr.Err)
			} else {
				rp.inner.SetResults(qr.Rows)
			}
			m.panes[paneResults] = resultsPaneAdapter{rp.inner}
		}
		return m, nil
	}

	if mc, ok := msg.(tea.MouseClickMsg); ok {
		return m.handleMouseClick(mc)
	}

	if mw, ok := msg.(tea.MouseWheelMsg); ok {
		return m.handleMouseWheel(mw)
	}

	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		var cmd tea.Cmd
		m.panes[m.focus], cmd = m.panes[m.focus].Update(msg)
		return m, cmd
	}

	switch k.String() {
	case "esc", "q":
		return m, m.onClose

	case "tab":
		return m.setFocus((m.focus + 1) % paneCount)

	case "shift+tab":
		return m.setFocus((m.focus + paneCount - 1) % paneCount)

	case "ctrl+l":
		return m.setFocus(paneQuery)

	case "ctrl+j":
		if m.focus == paneQuery {
			return m.setFocus(paneResults)
		}

	case "ctrl+k":
		if m.focus == paneResults {
			return m.setFocus(paneQuery)
		}

	case "ctrl+h":
		if m.focus != paneSidebar {
			return m.setFocus(paneSidebar)
		}
		var cmd tea.Cmd
		m.panes[paneSidebar], cmd = m.panes[paneSidebar].Update(msg)
		return m, cmd

	case "enter":
		if m.focus == paneSidebar {
			if cmd := m.tryFetchColumns(); cmd != nil {
				return m, cmd
			}
		}
		var cmd tea.Cmd
		m.panes[m.focus], cmd = m.panes[m.focus].Update(msg)
		return m, cmd

	case "space":
		if m.focus == paneSidebar {
			if sql, ok := m.sidebarSQL(); ok {
				m2 := m.setQueryText(sql)
				execCmd := m2.executeQueryCmd()
				if execCmd != nil {
					if rp, ok := m2.panes[paneResults].(resultsPaneAdapter); ok {
						rp.inner.SetLoading()
						m2.panes[paneResults] = resultsPaneAdapter{rp.inner}
					}
					return m2, execCmd
				}
				return m2, nil
			}
		}
		var cmd tea.Cmd
		m.panes[m.focus], cmd = m.panes[m.focus].Update(msg)
		return m, cmd

	case "f5", "ctrl+enter":
		if cmd := m.executeQueryCmd(); cmd != nil {
			if rp, ok := m.panes[paneResults].(resultsPaneAdapter); ok {
				rp.inner.SetLoading()
				m.panes[paneResults] = resultsPaneAdapter{rp.inner}
			}
			return m, cmd
		}

	default:
		var cmd tea.Cmd
		m.panes[m.focus], cmd = m.panes[m.focus].Update(msg)
		return m, cmd
	}

	return m, nil
}

// handleMouseClick maps a terminal click to a pane focus and optional navigation.
//
// Modal layout in terminal coords (modal is always at offset 1,1):
//
//	row 1            top border
//	row 2            title row
//	row 3            title separator
//	rows 4..4+bodyH  body rows
//	  cols 2..1+SidebarW   sidebar
//	  col  2+SidebarW      vertical divider
//	  cols 3+SidebarW..    right pane (query above DivRow, results below)
func (m Modal) handleMouseClick(mc tea.MouseClickMsg) (Modal, tea.Cmd) {
	if mc.Button != tea.MouseLeft {
		return m, nil
	}
	l := computeLayout(m.width, m.height)
	const offX, offY = 1, 1
	bodyStartY := offY + 3
	bodyRow := mc.Y - bodyStartY
	if bodyRow < 0 || bodyRow >= l.BodyH {
		return m, nil
	}

	const sidebarColStart = 2
	sidebarColEnd := offX + l.SidebarW
	rightColStart := offX + l.SidebarW + 2

	switch {
	case mc.X >= sidebarColStart && mc.X <= sidebarColEnd:
		m, cmd := m.setFocus(paneSidebar)
		if ep, ok := m.panes[paneSidebar].(explorerPane); ok {
			itemRows := l.BodyH - 5
			scroll := max(ep.inner.cursor-itemRows+1, 0)
			displayRow := bodyRow - 3
			if displayRow >= 0 {
				newCursor := scroll + displayRow
				items := ep.inner.filteredVisibleItems()
				ep.inner.cursor = min(newCursor, max(len(items)-1, 0))
				m.panes[paneSidebar] = ep
			}
		}
		return m, cmd

	case mc.X >= rightColStart:
		if bodyRow < l.DivRow {
			return m.setFocus(paneQuery)
		}
		if bodyRow > l.DivRow {
			m, cmd := m.setFocus(paneResults)
			if rp, ok := m.panes[paneResults].(resultsPaneAdapter); ok {
				ri := bodyRow - l.DivRow - 1
				if ri >= 3 {
					displayDataRow := ri - 3
					viewStart := max(rp.inner.tbl.Cursor()-rp.inner.tbl.Height(), 0)
					targetRow := viewStart + displayDataRow
					rp.inner.tbl.GotoTop()
					rp.inner.tbl.MoveDown(targetRow)
					m.panes[paneResults] = resultsPaneAdapter{rp.inner}
				}
			}
			return m, cmd
		}
	}
	return m, nil
}

// handleMouseWheel scrolls the sidebar or results pane on mouse wheel events.
func (m Modal) handleMouseWheel(mw tea.MouseWheelMsg) (Modal, tea.Cmd) {
	l := computeLayout(m.width, m.height)
	const offX, offY = 1, 1
	bodyStartY := offY + 3
	bodyRow := mw.Y - bodyStartY
	if bodyRow < 0 || bodyRow >= l.BodyH {
		return m, nil
	}

	rightColStart := offX + l.SidebarW + 2
	down := mw.Button == tea.MouseWheelDown

	if mw.X < rightColStart {
		if ep, ok := m.panes[paneSidebar].(explorerPane); ok {
			items := ep.inner.filteredVisibleItems()
			if down {
				if ep.inner.cursor < len(items)-1 {
					ep.inner.cursor++
				}
			} else {
				if ep.inner.cursor > 0 {
					ep.inner.cursor--
				}
			}
			m.panes[paneSidebar] = ep
		}
		return m, nil
	}

	if bodyRow > l.DivRow {
		if rp, ok := m.panes[paneResults].(resultsPaneAdapter); ok {
			rp.inner.tbl.Focus()
			if down {
				rp.inner.tbl.MoveDown(3)
			} else {
				rp.inner.tbl.MoveUp(3)
			}
			m.panes[paneResults] = resultsPaneAdapter{rp.inner}
		}
		return m, nil
	}

	if bodyRow < l.DivRow {
		var cmd tea.Cmd
		m.panes[paneQuery], cmd = m.panes[paneQuery].Update(mw)
		return m, cmd
	}

	return m, nil
}
