package gcp

import (
	"encoding/json"
	"time"
)

type PayloadKind int

const (
	PayloadText PayloadKind = iota
	PayloadJSON
	PayloadProto
)

// Payload is a unified representation of logging.Entry.Payload.
// Text payloads keep the raw string; JSON and Proto payloads are
// marshaled to JSON at ingestion time so the TUI can pretty-print
// without re-inspecting the original Go type.
type Payload struct {
	Kind PayloadKind
	Text string
	JSON json.RawMessage
}

type Resource struct {
	Type   string
	Labels map[string]string
}

type LogEntry struct {
	Timestamp time.Time
	Severity  string
	LogName   string
	Resource  Resource
	Payload   Payload
	InsertID  string
}

type SearchParams struct {
	ProjectID   string
	Filter      string
	NewestFirst bool
	PageSize    int32
}
