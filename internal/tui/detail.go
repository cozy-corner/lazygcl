package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
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
			b.WriteString(highlightJSON(pretty.String()))
		}
	}
	return b.String()
}

// highlightJSON adds ANSI color escapes around JSON tokens so the viewport
// renders the payload with chroma's monokai palette. Returns the input
// unchanged if any chroma stage fails — a colorless detail view is better
// than nothing.
func highlightJSON(src string) string {
	lexer := lexers.Get("json")
	if lexer == nil {
		return src
	}
	style := styles.Get("monokai")
	if style == nil {
		style = styles.Fallback
	}
	formatter := formatters.Get("terminal256")
	if formatter == nil {
		return src
	}
	iter, err := lexer.Tokenise(nil, src)
	if err != nil {
		return src
	}
	var buf bytes.Buffer
	if err := formatter.Format(&buf, style, iter); err != nil {
		return src
	}
	return buf.String()
}
