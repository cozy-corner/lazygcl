package gcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"cloud.google.com/go/logging"
	"cloud.google.com/go/logging/logadmin"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const defaultPageSize int32 = 200

type Client struct {
	lc      *logadmin.Client
	project string
}

func NewClient(ctx context.Context, projectID string) (*Client, error) {
	lc, err := logadmin.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("logadmin.NewClient: %w", err)
	}
	return &Client{lc: lc, project: projectID}, nil
}

func (c *Client) Close() error { return c.lc.Close() }

// EntryStream yields log entries one at a time. Returns io.EOF when the
// stream is exhausted. v2 live-tail will implement the same interface.
type EntryStream interface {
	Next(ctx context.Context) (LogEntry, error)
}

// Search starts a paginated query against Cloud Logging. The returned stream
// auto-fetches pages from the underlying logadmin iterator.
func (c *Client) Search(ctx context.Context, p SearchParams) EntryStream {
	opts := []logadmin.EntriesOption{}
	if p.Filter != "" {
		opts = append(opts, logadmin.Filter(p.Filter))
	}
	if p.NewestFirst {
		opts = append(opts, logadmin.NewestFirst())
	}
	size := p.PageSize
	if size <= 0 {
		size = defaultPageSize
	}
	opts = append(opts, logadmin.PageSize(size))
	return &pagedStream{it: c.lc.Entries(ctx, opts...)}
}

type pagedStream struct {
	it *logadmin.EntryIterator
}

func (s *pagedStream) Next(_ context.Context) (LogEntry, error) {
	e, err := s.it.Next()
	if errors.Is(err, iterator.Done) {
		return LogEntry{}, io.EOF
	}
	if err != nil {
		return LogEntry{}, err
	}
	return convert(e), nil
}

func convert(e *logging.Entry) LogEntry {
	out := LogEntry{
		Timestamp: e.Timestamp,
		Severity:  e.Severity.String(),
		LogName:   e.LogName,
		InsertID:  e.InsertID,
		Payload:   convertPayload(e.Payload),
	}
	if e.Resource != nil {
		out.Resource = Resource{Type: e.Resource.Type, Labels: e.Resource.Labels}
	}
	return out
}

func convertPayload(p any) Payload {
	switch v := p.(type) {
	case nil:
		return Payload{Kind: PayloadText}
	case string:
		return Payload{Kind: PayloadText, Text: v}
	case protoreflect.ProtoMessage:
		raw, err := protojson.Marshal(v)
		if err != nil {
			return Payload{Kind: PayloadText, Text: fmt.Sprintf("<proto marshal failed: %v>", err)}
		}
		return Payload{Kind: PayloadProto, JSON: raw}
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return Payload{Kind: PayloadText, Text: fmt.Sprintf("%v", v)}
		}
		return Payload{Kind: PayloadJSON, JSON: raw}
	}
}
