package gcp

import (
	"encoding/json"
	"testing"
	"time"

	"cloud.google.com/go/logging"
	mrpb "google.golang.org/genproto/googleapis/api/monitoredres"
)

func TestConvert_TextPayload(t *testing.T) {
	ts := time.Date(2026, 5, 8, 12, 0, 0, 0, time.UTC)
	in := &logging.Entry{
		Timestamp: ts,
		Severity:  logging.Error,
		LogName:   "projects/p/logs/stdout",
		InsertID:  "abc",
		Payload:   "boom",
		Resource:  &mrpb.MonitoredResource{Type: "gce_instance", Labels: map[string]string{"zone": "us-central1-a"}},
	}
	got := convert(in)

	if got.Timestamp != ts {
		t.Errorf("Timestamp = %v, want %v", got.Timestamp, ts)
	}
	if got.Severity != "Error" {
		t.Errorf("Severity = %q, want %q", got.Severity, "Error")
	}
	if got.LogName != "projects/p/logs/stdout" {
		t.Errorf("LogName = %q", got.LogName)
	}
	if got.Payload.Kind != PayloadText || got.Payload.Text != "boom" {
		t.Errorf("Payload = %+v, want text=boom", got.Payload)
	}
	if got.Resource.Type != "gce_instance" || got.Resource.Labels["zone"] != "us-central1-a" {
		t.Errorf("Resource = %+v", got.Resource)
	}
}

func TestConvert_JSONPayload(t *testing.T) {
	in := &logging.Entry{
		Payload: map[string]any{"msg": "hi", "code": 42},
	}
	got := convert(in)
	if got.Payload.Kind != PayloadJSON {
		t.Fatalf("Kind = %v, want PayloadJSON", got.Payload.Kind)
	}
	var round map[string]any
	if err := json.Unmarshal(got.Payload.JSON, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if round["msg"] != "hi" {
		t.Errorf("msg = %v", round["msg"])
	}
}

func TestConvert_NilPayload(t *testing.T) {
	got := convert(&logging.Entry{})
	if got.Payload.Kind != PayloadText || got.Payload.Text != "" {
		t.Errorf("nil payload: %+v", got.Payload)
	}
}
