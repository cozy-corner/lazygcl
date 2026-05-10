package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.currentView == viewPicker {
		return m.handlePickerKey(msg)
	}
	if m.currentView == viewDetail {
		return m.handleDetailKey(msg)
	}
	switch msg.Type {
	case tea.KeyCtrlF:
		return m.openPicker(pickerField)
	}
	if m.focus == paneQuery {
		return m.handleQueryKey(msg)
	}
	return m.handleResultsKey(msg)
}

func (m Model) handleQueryKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyTab:
		if len(m.entries) > 0 {
			m.focus = paneResults
			m.query.Blur()
		}
		return m, nil
	case tea.KeyCtrlC:
		m.cancel()
		return m, tea.Quit
	}
	if msg.Type == tea.KeyEnter && msg.Alt {
		// Alt+Enter inserts a newline so users can write multi-line filters.
		var cmd tea.Cmd
		m.query, cmd = m.query.Update(msg)
		return m, cmd
	}
	if msg.Type == tea.KeyEnter {
		return m.runQuery()
	}
	var cmd tea.Cmd
	m.query, cmd = m.query.Update(msg)
	return m, cmd
}

func (m Model) handleResultsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyTab:
		m.focus = paneQuery
		m.query.Focus()
		return m, nil
	case tea.KeyDown:
		return m.moveCursor(1)
	case tea.KeyUp:
		return m.moveCursor(-1)
	case tea.KeyEnter:
		return m.openDetail()
	case tea.KeyCtrlC:
		m.cancel()
		return m, tea.Quit
	}
	switch string(msg.Runes) {
	case "j":
		return m.moveCursor(1)
	case "k":
		return m.moveCursor(-1)
	case "g":
		m.cursor = 0
		m.offset = 0
		return m, nil
	case "G":
		if len(m.entries) > 0 {
			m.cursor = len(m.entries) - 1
			m.adjustOffset()
		}
		return m.maybeFetchMore()
	case "q":
		m.cancel()
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) handleDetailKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.currentView = viewMain
		return m, nil
	case tea.KeyCtrlC:
		m.cancel()
		return m, tea.Quit
	}
	if string(msg.Runes) == "q" {
		m.cancel()
		return m, tea.Quit
	}
	var cmd tea.Cmd
	m.detail, cmd = m.detail.Update(msg)
	return m, cmd
}

func (m Model) moveCursor(delta int) (Model, tea.Cmd) {
	if len(m.entries) == 0 {
		return m, nil
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
	m.adjustOffset()
	return m.maybeFetchMore()
}

// pagingThreshold is how many entries can remain after the cursor before the
// next batch is auto-fetched.
const pagingThreshold = 10

// maybeFetchMore queues the next page if the cursor is near the tail of the
// current results and the stream has not yet been exhausted.
func (m Model) maybeFetchMore() (Model, tea.Cmd) {
	if m.streamDone || m.loading || m.stream == nil {
		return m, nil
	}
	if len(m.entries)-m.cursor-1 > pagingThreshold {
		return m, nil
	}
	m.loading = true
	gen := m.queryGen
	stream := m.stream
	ctx := m.streamCtx
	return m, func() tea.Msg {
		return fetchPage(ctx, stream, gen, pageBatch)
	}
}

func (m *Model) adjustOffset() {
	rows := m.resultsRows()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+rows {
		m.offset = m.cursor - rows + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}
