package tui

import (
	"context"
	"errors"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cozy-corner/lazygcl/internal/gcp"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// pageBatch is how many entries each fetchPage Cmd pulls from the stream
// before yielding a queryResultMsg. Smaller than the API page size on
// purpose: the user sees the first batch quickly even if the underlying
// page is bigger.
const pageBatch = 50

// runQuery starts a new search using the textarea contents. It mutates the
// model (resets entries, increments the generation, opens a fresh stream)
// and returns a Cmd that fetches the first batch.
func (m Model) runQuery() (Model, tea.Cmd) {
	if m.streamCancel != nil {
		m.streamCancel()
	}
	filter := strings.TrimSpace(m.query.Value())

	ctx, cancel := context.WithCancel(context.Background())
	m.streamCtx = ctx
	m.streamCancel = cancel

	m.queryGen++
	m.entries = nil
	m.cursor = 0
	m.offset = 0
	m.err = nil
	m.loading = true
	m.streamDone = false

	m.stream = m.client.Search(ctx, gcp.SearchParams{
		ProjectID:   m.project,
		Filter:      filter,
		NewestFirst: true,
	})

	gen := m.queryGen
	stream := m.stream
	return m, func() tea.Msg {
		return fetchPage(ctx, stream, gen, pageBatch)
	}
}

// fetchPage pulls up to n entries from the stream. Returns queryResultMsg on
// success (with done=true if the stream is exhausted), errMsg on failure.
func fetchPage(ctx context.Context, s gcp.EntryStream, gen, n int) tea.Msg {
	entries := make([]gcp.LogEntry, 0, n)
	for i := 0; i < n; i++ {
		e, err := s.Next(ctx)
		if errors.Is(err, io.EOF) {
			return queryResultMsg{gen: gen, entries: entries, done: true}
		}
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return queryResultMsg{gen: gen, entries: entries, done: true}
			}
			return errMsg{gen: gen, err: err, syntax: isFilterSyntaxError(err)}
		}
		entries = append(entries, e)
	}
	return queryResultMsg{gen: gen, entries: entries, done: false}
}

func isFilterSyntaxError(err error) bool {
	if s, ok := status.FromError(err); ok {
		return s.Code() == codes.InvalidArgument
	}
	return false
}
