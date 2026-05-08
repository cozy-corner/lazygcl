package tui

import (
	"github.com/cozy-corner/lazygcl/internal/gcp"
)

// pickerLoadedMsg delivers items for the picker view. Either Resources or
// LogNames is populated depending on which kind was requested.
type pickerLoadedMsg struct {
	kind      pickerKind
	resources []gcp.ResourceDescriptor
	logNames  []string
}

// pickerErrMsg surfaces a load failure for the picker.
type pickerErrMsg struct {
	kind pickerKind
	err  error
}

// queryResultMsg delivers a page of entries from a Search. The generation
// field lets the model drop messages from a query that was superseded by a
// newer one.
type queryResultMsg struct {
	gen     int
	entries []gcp.LogEntry
	done    bool // true when the underlying stream returned io.EOF
}

// errMsg surfaces an error to the model. Filter-syntax errors are routed to
// the query pane; everything else becomes a status banner.
type errMsg struct {
	gen    int
	err    error
	syntax bool // true when the error is a Cloud Logging filter syntax error
}

func (e errMsg) Error() string { return e.err.Error() }
