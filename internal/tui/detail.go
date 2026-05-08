package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

// wireLogEntry mirrors the field names of the Cloud Logging REST LogEntry
// resource so the detail view reads identically to `gcloud logging read
// --format=json` and the API response. Field order is fixed by Go's struct
// marshaling, which is what we want here.
type wireLogEntry struct {
	Timestamp    string          `json:"timestamp,omitempty"`
	Severity     string          `json:"severity,omitempty"`
	LogName      string          `json:"logName,omitempty"`
	Resource     *wireResource   `json:"resource,omitempty"`
	InsertID     string          `json:"insertId,omitempty"`
	TextPayload  string          `json:"textPayload,omitempty"`
	JSONPayload  json.RawMessage `json:"jsonPayload,omitempty"`
	ProtoPayload json.RawMessage `json:"protoPayload,omitempty"`
}

type wireResource struct {
	Type   string            `json:"type"`
	Labels map[string]string `json:"labels,omitempty"`
}

func formatDetail(e gcp.LogEntry) string {
	w := wireLogEntry{
		LogName:  e.LogName,
		InsertID: e.InsertID,
	}
	if !e.Timestamp.IsZero() {
		w.Timestamp = e.Timestamp.UTC().Format(time.RFC3339Nano)
	}
	if e.Severity != "" {
		w.Severity = strings.ToUpper(e.Severity)
	}
	if e.Resource.Type != "" {
		w.Resource = &wireResource{Type: e.Resource.Type, Labels: e.Resource.Labels}
	}
	switch e.Payload.Kind {
	case gcp.PayloadText:
		w.TextPayload = e.Payload.Text
	case gcp.PayloadJSON:
		w.JSONPayload = e.Payload.JSON
	case gcp.PayloadProto:
		w.ProtoPayload = e.Payload.JSON
	}

	raw, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return fmt.Sprintf("error formatting entry: %v", err)
	}
	return highlightJSON(string(raw))
}

// highlightJSON adds ANSI color escapes around JSON tokens so the viewport
// renders the entry with chroma's monokai palette. Returns the input
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
