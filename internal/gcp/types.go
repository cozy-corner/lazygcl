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

// ResourceDescriptor mirrors the fields of monitoredres.MonitoredResourceDescriptor
// that lazygcl actually surfaces in the picker.
type ResourceDescriptor struct {
	Type        string
	DisplayName string
	Description string
	Labels      []LabelDescriptor
}

// LabelDescriptor mirrors the fields of label.LabelDescriptor that lazygcl
// surfaces in the label-key picker. Labels come for free with
// ListMonitoredResourceDescriptors so no extra API call is needed.
type LabelDescriptor struct {
	Key         string
	Description string
}
