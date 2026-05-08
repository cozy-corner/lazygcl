package tui

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/cozy-corner/lazygcl/internal/gcp"
)

// stripANSI removes the ANSI escape sequences that chroma adds so the test
// can assert against the underlying JSON shape.
func stripANSI(s string) string {
	for {
		i := strings.Index(s, "\x1b[")
		if i < 0 {
			return s
		}
		j := strings.IndexByte(s[i:], 'm')
		if j < 0 {
			return s
		}
		s = s[:i] + s[i+j+1:]
	}
}

func TestFormatDetail_WireShape(t *testing.T) {
	ts := time.Date(2026, 5, 9, 0, 37, 32, 0, time.UTC)
	e := gcp.LogEntry{
		Timestamp: ts,
		Severity:  "Error",
		LogName:   "projects/p/logs/run.googleapis.com%2Fstdout",
		Resource: gcp.Resource{
			Type:   "cloud_run_revision",
			Labels: map[string]string{"service_name": "api"},
		},
		InsertID: "abc",
		Payload:  gcp.Payload{Kind: gcp.PayloadJSON, JSON: json.RawMessage(`{"msg":"hi"}`)},
	}

	out := stripANSI(formatDetail(e))

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, out)
	}

	if got["timestamp"] != "2026-05-09T00:37:32Z" {
		t.Errorf("timestamp = %v", got["timestamp"])
	}
	if got["severity"] != "ERROR" {
		t.Errorf("severity = %v, want ERROR (uppercased)", got["severity"])
	}
	if got["logName"] != "projects/p/logs/run.googleapis.com%2Fstdout" {
		t.Errorf("logName = %v", got["logName"])
	}
	res, ok := got["resource"].(map[string]any)
	if !ok || res["type"] != "cloud_run_revision" {
		t.Errorf("resource = %v", got["resource"])
	}
	jp, ok := got["jsonPayload"].(map[string]any)
	if !ok || jp["msg"] != "hi" {
		t.Errorf("jsonPayload = %v", got["jsonPayload"])
	}
	if _, ok := got["textPayload"]; ok {
		t.Errorf("textPayload should be omitted when jsonPayload is set")
	}
}

func TestFormatDetail_TextPayload(t *testing.T) {
	e := gcp.LogEntry{
		Timestamp: time.Now(),
		Payload:   gcp.Payload{Kind: gcp.PayloadText, Text: "hello"},
	}
	out := stripANSI(formatDetail(e))
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if got["textPayload"] != "hello" {
		t.Errorf("textPayload = %v", got["textPayload"])
	}
	if _, ok := got["jsonPayload"]; ok {
		t.Errorf("jsonPayload should be omitted")
	}
}
