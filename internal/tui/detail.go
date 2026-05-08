package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cozy-corner/lazygcl/internal/gcp"
)

func (m Model) openDetail() (tea.Model, tea.Cmd) {
	if m.cursor < 0 || m.cursor >= len(m.entries) {
		return m, nil
	}
	e := m.entries[m.cursor]
	m.detail.SetContent(formatDetail(e))
	m.detail.GotoTop()
	m.currentView = viewDetail
	return m, nil
}

func formatDetail(e gcp.LogEntry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Timestamp: %s\n", e.Timestamp.Local().Format("2006-01-02 15:04:05 MST"))
	if e.Severity != "" {
		fmt.Fprintf(&b, "Severity:  %s\n", e.Severity)
	}
	if e.LogName != "" {
		fmt.Fprintf(&b, "Log:       %s\n", e.LogName)
	}
	if e.Resource.Type != "" {
		fmt.Fprintf(&b, "Resource:  %s\n", e.Resource.Type)
		for k, v := range e.Resource.Labels {
			fmt.Fprintf(&b, "  %s: %s\n", k, v)
		}
	}
	if e.InsertID != "" {
		fmt.Fprintf(&b, "InsertID:  %s\n", e.InsertID)
	}
	b.WriteString("\nPayload:\n")
	switch e.Payload.Kind {
	case gcp.PayloadText:
		b.WriteString(e.Payload.Text)
	default:
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, e.Payload.JSON, "", "  "); err != nil {
			b.Write(e.Payload.JSON)
		} else {
			b.Write(pretty.Bytes())
		}
	}
	return b.String()
}
