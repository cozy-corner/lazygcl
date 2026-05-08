package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.currentView == viewDetail {
		return m.handleDetailKey(msg)
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
		return m, m.runQuery()
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
		return m.moveCursor(1), nil
	case tea.KeyUp:
		return m.moveCursor(-1), nil
	case tea.KeyEnter:
		return m.openDetail()
	case tea.KeyCtrlC:
		m.cancel()
		return m, tea.Quit
	}
	switch string(msg.Runes) {
	case "j":
		return m.moveCursor(1), nil
	case "k":
		return m.moveCursor(-1), nil
	case "g":
		m.cursor = 0
		m.offset = 0
		return m, nil
	case "G":
		if len(m.entries) > 0 {
			m.cursor = len(m.entries) - 1
			m.adjustOffset()
		}
		return m, nil
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

func (m Model) moveCursor(delta int) Model {
	if len(m.entries) == 0 {
		return m
	}
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.entries) {
		m.cursor = len(m.entries) - 1
	}
	m.adjustOffset()
	return m
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
