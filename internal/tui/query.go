package tui

import tea "github.com/charmbracelet/bubbletea"

// runQuery returns a Cmd that executes the current filter against Cloud Logging.
// Wired to gcp.Client.Search in the next commit; returns nil for now so the
// skeleton compiles and the keystroke is observable as "nothing happened".
func (m *Model) runQuery() tea.Cmd {
	return nil
}
