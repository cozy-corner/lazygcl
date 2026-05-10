package tui

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cozy-corner/lazygcl/internal/gcp"
	"github.com/sahilm/fuzzy"
)

type pickerKind int

const (
	// pickerField is the single entry point: a list of LogEntry top-level
	// fields the user can filter on. Selecting one dispatches per-field
	// (dynamic value picker, enum value picker, sub-field picker, or
	// skeleton insertion).
	pickerField pickerKind = iota
	pickerResource
	pickerLogName
	// pickerEnumValue picks among hardcoded enum values for a specific
	// field (e.g. severity levels). The field/op/quoting are carried on
	// Model.pickerEnum* so the same picker kind serves multiple fields.
	pickerEnumValue
	// pickerResourceSubField is the second step under the `resource`
	// top-level object: pick `type` (→ resource value picker) or `labels`
	// (→ pickerResourceLabelsAll).
	pickerResourceSubField
	// pickerResourceLabelsAll lists label keys de-duplicated across the
	// global resource-descriptor catalog. Selection inserts a
	// `resource.labels.<key> = ""` skeleton.
	pickerResourceLabelsAll
)

func (k pickerKind) title() string {
	switch k {
	case pickerField:
		return "Field"
	case pickerResource:
		return "Resource type"
	case pickerLogName:
		return "Log name"
	case pickerResourceLabelsAll:
		return "Label key"
	case pickerEnumValue:
		return "Value"
	case pickerResourceSubField:
		return "Resource sub-field"
	}
	return ""
}

// fieldStrategy enumerates how the field picker dispatches when an item is
// selected.
type fieldStrategy int

const (
	// fieldDynamicValues opens an API-backed value picker (logName).
	fieldDynamicValues fieldStrategy = iota
	// fieldEnumValues opens a hardcoded enum value picker (severity,
	// traceSampled).
	fieldEnumValues
	// fieldSkeleton inserts `<path> <op> ""` and positions the cursor for
	// the user to type the value (trace, timestamp, ...).
	fieldSkeleton
	// fieldObjectSubPicker opens a sub-field picker for an object-typed
	// top-level field (resource → type / labels).
	fieldObjectSubPicker
)

// fieldDef describes a queryable LogEntry top-level field surfaced in the
// field picker. Quoted=false produces unquoted values, e.g. for bool.
type fieldDef struct {
	path       string
	op         string
	quoted     bool
	strategy   fieldStrategy
	enumValues []string
}

var severityLevels = []string{
	"DEBUG", "INFO", "NOTICE", "WARNING", "ERROR", "CRITICAL", "ALERT", "EMERGENCY",
}

// topLevelFields lists LogEntry top-level fields lazygcl exposes via the
// field picker. Object-typed fields use fieldObjectSubPicker so sub-fields
// (e.g. resource.type) are reached through their parent.
var topLevelFields = []fieldDef{
	{path: "logName", op: "=", quoted: true, strategy: fieldDynamicValues},
	{path: "resource", strategy: fieldObjectSubPicker},
	{path: "severity", op: ">=", quoted: true, strategy: fieldEnumValues, enumValues: severityLevels},
	{path: "timestamp", op: ">=", quoted: true, strategy: fieldSkeleton},
	{path: "receiveTimestamp", op: ">=", quoted: true, strategy: fieldSkeleton},
	{path: "trace", op: "=", quoted: true, strategy: fieldSkeleton},
	{path: "spanId", op: "=", quoted: true, strategy: fieldSkeleton},
	{path: "insertId", op: "=", quoted: true, strategy: fieldSkeleton},
	{path: "traceSampled", op: "=", quoted: false, strategy: fieldEnumValues, enumValues: []string{"true", "false"}},
	{path: "textPayload", op: "=~", quoted: true, strategy: fieldSkeleton},
}

func fieldByPath(path string) *fieldDef {
	for i := range topLevelFields {
		if topLevelFields[i].path == path {
			return &topLevelFields[i]
		}
	}
	return nil
}

func fieldItems() []pickerItem {
	out := make([]pickerItem, 0, len(topLevelFields))
	for _, f := range topLevelFields {
		out = append(out, pickerItem{
			Display:   f.path,
			FilterKey: strings.ToLower(f.path),
			Value:     f.path,
		})
	}
	return out
}

func enumValueItems(values []string) []pickerItem {
	out := make([]pickerItem, 0, len(values))
	for _, v := range values {
		out = append(out, pickerItem{
			Display:   v,
			FilterKey: strings.ToLower(v),
			Value:     v,
		})
	}
	return out
}

// pickerItem is the unified picker row. Display is shown to the user;
// FilterKey is the lowercased haystack for substring search; Value is what
// gets injected into the query filter on selection.
type pickerItem struct {
	Display   string
	FilterKey string
	Value     string
}

// openPicker resets the picker UI and returns a Cmd to fetch its items if
// API-backed, or nil for in-memory pickers (pickerField).
func (m Model) openPicker(kind pickerKind) (Model, tea.Cmd) {
	ti := textinput.New()
	ti.Placeholder = "filter…"
	ti.Focus()

	m.currentView = viewPicker
	m.pickerKind = kind
	m.pickerInput = ti
	m.pickerItems = nil
	m.pickerCursor = 0
	m.pickerOffset = 0
	m.pickerErr = nil

	client := m.client
	switch kind {
	case pickerField:
		m.pickerItems = fieldItems()
		m.pickerLoading = false
		return m, nil
	case pickerResource:
		if m.resourceDescriptors != nil {
			m.pickerItems = resourceItems(m.resourceDescriptors)
			m.pickerLoading = false
			return m, nil
		}
		m.pickerLoading = true
		return m, func() tea.Msg {
			rs, err := client.ListResourceDescriptors(m.ctx)
			if err != nil {
				return pickerErrMsg{kind: kind, err: err}
			}
			return pickerLoadedMsg{kind: kind, resources: rs}
		}
	case pickerLogName:
		m.pickerLoading = true
		return m, func() tea.Msg {
			ns, err := client.ListLogNames(m.ctx)
			if err != nil {
				return pickerErrMsg{kind: kind, err: err}
			}
			return pickerLoadedMsg{kind: kind, logNames: ns}
		}
	case pickerResourceLabelsAll:
		if m.resourceDescriptors != nil {
			m.pickerItems = labelKeyItems(unionLabels(m.resourceDescriptors))
			m.pickerLoading = false
			return m, nil
		}
		m.pickerLoading = true
		return m, func() tea.Msg {
			rs, err := client.ListResourceDescriptors(m.ctx)
			if err != nil {
				return pickerErrMsg{kind: kind, err: err}
			}
			return pickerLoadedMsg{kind: kind, resources: rs}
		}
	}
	return m, nil
}

// unionLabels returns the de-duplicated union of label descriptors across
// all resource descriptors, sorted by key.
func unionLabels(rs []gcp.ResourceDescriptor) []gcp.LabelDescriptor {
	seen := map[string]gcp.LabelDescriptor{}
	for _, r := range rs {
		for _, l := range r.Labels {
			if _, ok := seen[l.Key]; !ok {
				seen[l.Key] = l
			}
		}
	}
	out := make([]gcp.LabelDescriptor, 0, len(seen))
	for _, l := range seen {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func resourceItems(rs []gcp.ResourceDescriptor) []pickerItem {
	out := make([]pickerItem, 0, len(rs))
	for _, r := range rs {
		display := r.Type
		if r.DisplayName != "" {
			display = fmt.Sprintf("%-32s %s", r.Type, r.DisplayName)
		}
		out = append(out, pickerItem{
			Display:   display,
			FilterKey: strings.ToLower(r.Type + " " + r.DisplayName),
			Value:     r.Type,
		})
	}
	return out
}

func labelKeyItems(labels []gcp.LabelDescriptor) []pickerItem {
	out := make([]pickerItem, 0, len(labels))
	for _, l := range labels {
		display := l.Key
		if l.Description != "" {
			display = fmt.Sprintf("%-24s %s", l.Key, l.Description)
		}
		out = append(out, pickerItem{
			Display:   display,
			FilterKey: strings.ToLower(l.Key + " " + l.Description),
			Value:     l.Key,
		})
	}
	return out
}

func logNameItems(names []string) []pickerItem {
	out := make([]pickerItem, 0, len(names))
	for _, full := range names {
		short := shortLogName(full)
		out = append(out, pickerItem{
			Display:   short,
			FilterKey: strings.ToLower(short),
			Value:     full,
		})
	}
	return out
}

// shortLogName strips the projects/X/logs/ prefix and URL-decodes the LOG_ID
// for display. The full path is what gets injected into the filter.
func shortLogName(full string) string {
	const marker = "/logs/"
	if i := strings.Index(full, marker); i >= 0 {
		id := full[i+len(marker):]
		if dec, err := url.QueryUnescape(id); err == nil {
			return dec
		}
		return id
	}
	return full
}

// filteredPickerItems returns the indices of items matching the current
// input, ranked fzf-style: characters must appear in order but may be
// non-contiguous, and tighter / earlier matches score higher.
func (m Model) filteredPickerItems() []int {
	q := strings.TrimSpace(m.pickerInput.Value())
	if q == "" {
		all := make([]int, len(m.pickerItems))
		for i := range all {
			all[i] = i
		}
		return all
	}
	keys := make([]string, len(m.pickerItems))
	for i, it := range m.pickerItems {
		keys[i] = it.FilterKey
	}
	matches := fuzzy.Find(q, keys)
	out := make([]int, 0, len(matches))
	for _, mm := range matches {
		out = append(out, mm.Index)
	}
	return out
}

func (m Model) renderPicker() string {
	var b strings.Builder
	header := fmt.Sprintf("Select %s — type to filter, enter to select, esc to cancel",
		strings.ToLower(m.pickerKind.title()))
	fmt.Fprintln(&b, headerStyle.Render(header))
	fmt.Fprintln(&b, m.pickerInput.View())

	if m.pickerLoading {
		fmt.Fprintln(&b, dimStyle.Render("loading…"))
		return b.String()
	}
	if m.pickerErr != nil {
		fmt.Fprintln(&b, errorStyle.Render("error: "+m.pickerErr.Error()))
		return b.String()
	}

	indices := m.filteredPickerItems()
	if len(indices) == 0 {
		fmt.Fprintln(&b, dimStyle.Render("(no matches)"))
		return b.String()
	}

	rows := m.pickerRows()
	end := m.pickerOffset + rows
	if end > len(indices) {
		end = len(indices)
	}
	for i := m.pickerOffset; i < end; i++ {
		idx := indices[i]
		row := m.pickerItems[idx].Display
		if i == m.pickerCursor {
			row = cursorStyle.Render("> " + row)
		} else {
			row = "  " + row
		}
		b.WriteString(row)
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "\n%s\n", dimStyle.Render(fmt.Sprintf("%d match(es)", len(indices))))
	return b.String()
}

func (m Model) pickerRows() int {
	rows := m.height - 6
	if rows < 1 {
		return 1
	}
	return rows
}

func (m Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.currentView = viewMain
		return m, nil
	case tea.KeyCtrlC:
		m.cancel()
		return m, tea.Quit
	case tea.KeyEnter:
		return m.applyPickerSelection()
	case tea.KeyDown, tea.KeyCtrlJ, tea.KeyCtrlN:
		return m.movePickerCursor(1), nil
	case tea.KeyUp, tea.KeyCtrlK, tea.KeyCtrlP:
		return m.movePickerCursor(-1), nil
	}
	var cmd tea.Cmd
	m.pickerInput, cmd = m.pickerInput.Update(msg)
	// Resetting cursor on input change keeps the highlight on a row that is
	// still part of the (now possibly smaller) filtered set.
	m.pickerCursor = 0
	m.pickerOffset = 0
	return m, cmd
}

func (m Model) movePickerCursor(delta int) Model {
	indices := m.filteredPickerItems()
	if len(indices) == 0 {
		return m
	}
	m.pickerCursor += delta
	if m.pickerCursor < 0 {
		m.pickerCursor = 0
	}
	if m.pickerCursor >= len(indices) {
		m.pickerCursor = len(indices) - 1
	}
	rows := m.pickerRows()
	if m.pickerCursor < m.pickerOffset {
		m.pickerOffset = m.pickerCursor
	}
	if m.pickerCursor >= m.pickerOffset+rows {
		m.pickerOffset = m.pickerCursor - rows + 1
	}
	if m.pickerOffset < 0 {
		m.pickerOffset = 0
	}
	return m
}

func (m Model) applyPickerSelection() (Model, tea.Cmd) {
	indices := m.filteredPickerItems()
	if len(indices) == 0 || m.pickerCursor >= len(indices) {
		return m, nil
	}
	item := m.pickerItems[indices[m.pickerCursor]]

	switch m.pickerKind {
	case pickerField:
		return m.applyFieldSelection(item.Value)

	case pickerResource:
		m.appendQueryClause(fmt.Sprintf(`resource.type = %q`, item.Value))
		return m.closePicker(), nil

	case pickerLogName:
		m.appendQueryClause(fmt.Sprintf(`logName = %q`, item.Value))
		return m.closePicker(), nil

	case pickerResourceLabelsAll:
		m.appendSkeletonClause(fmt.Sprintf("resource.labels.%s", item.Value), "=", true)
		return m.closePicker(), nil

	case pickerEnumValue:
		m.appendEnumValueClause(item.Value)
		return m.closePicker(), nil

	case pickerResourceSubField:
		switch item.Value {
		case "type":
			return m.openPicker(pickerResource)
		case "labels":
			return m.openPicker(pickerResourceLabelsAll)
		}
	}
	return m, nil
}

// applyFieldSelection dispatches based on the selected field's strategy.
func (m Model) applyFieldSelection(path string) (Model, tea.Cmd) {
	f := fieldByPath(path)
	if f == nil {
		return m.closePicker(), nil
	}
	switch f.strategy {
	case fieldDynamicValues:
		if f.path == "logName" {
			return m.openPicker(pickerLogName)
		}
	case fieldEnumValues:
		return m.openEnumValuePicker(f), nil
	case fieldSkeleton:
		m.appendSkeletonClause(f.path, f.op, f.quoted)
		return m.closePicker(), nil
	case fieldObjectSubPicker:
		if f.path == "resource" {
			return m.openResourceSubFieldPicker(), nil
		}
	}
	return m.closePicker(), nil
}

// openResourceSubFieldPicker opens a 2-item picker (type / labels) so the
// user can choose which sub-field of the top-level `resource` object to
// filter on.
func (m Model) openResourceSubFieldPicker() Model {
	ti := textinput.New()
	ti.Placeholder = "filter…"
	ti.Focus()

	m.currentView = viewPicker
	m.pickerKind = pickerResourceSubField
	m.pickerInput = ti
	m.pickerItems = []pickerItem{
		{Display: "type", FilterKey: "type", Value: "type"},
		{Display: "labels", FilterKey: "labels", Value: "labels"},
	}
	m.pickerCursor = 0
	m.pickerOffset = 0
	m.pickerLoading = false
	m.pickerErr = nil
	return m
}

// closePicker returns to the main view and re-focuses the query pane.
func (m Model) closePicker() Model {
	m.currentView = viewMain
	m.focus = paneQuery
	m.query.Focus()
	return m
}

// openEnumValuePicker opens an enum value picker for a field with hardcoded
// values (severity, traceSampled). The field metadata is stashed on the
// model so applyPickerSelection can format the clause.
func (m Model) openEnumValuePicker(f *fieldDef) Model {
	ti := textinput.New()
	ti.Placeholder = "filter…"
	ti.Focus()

	m.currentView = viewPicker
	m.pickerKind = pickerEnumValue
	m.pickerInput = ti
	m.pickerItems = enumValueItems(f.enumValues)
	m.pickerCursor = 0
	m.pickerOffset = 0
	m.pickerLoading = false
	m.pickerErr = nil

	m.pickerEnumField = f.path
	m.pickerEnumOp = f.op
	m.pickerEnumQuoted = f.quoted
	return m
}

// appendQueryClause appends a fully-formed clause, joining with AND if the
// query already has content. Cursor lands at the end of the appended text.
func (m *Model) appendQueryClause(clause string) {
	existing := strings.TrimSpace(m.query.Value())
	if existing == "" {
		m.query.SetValue(clause)
	} else {
		m.query.SetValue(existing + "\nAND " + clause)
	}
}

// appendSkeletonClause inserts `<path> <op> ""` (or `<path> <op> ` if
// unquoted) and positions the cursor where the user types the value —
// between the two quotes for quoted forms, at end-of-line for unquoted.
func (m *Model) appendSkeletonClause(path, op string, quoted bool) {
	var clause string
	if quoted {
		clause = fmt.Sprintf(`%s %s ""`, path, op)
	} else {
		clause = fmt.Sprintf(`%s %s `, path, op)
	}
	existing := strings.TrimSpace(m.query.Value())
	var lastLine string
	if existing == "" {
		lastLine = clause
		m.query.SetValue(clause)
	} else {
		lastLine = "AND " + clause
		m.query.SetValue(existing + "\nAND " + clause)
	}
	if quoted {
		m.query.SetCursor(len([]rune(lastLine)) - 1)
	} else {
		m.query.SetCursor(len([]rune(lastLine)))
	}
}

// appendEnumValueClause inserts the enum value picked for the field
// previously stashed on the model by openEnumValuePicker.
func (m *Model) appendEnumValueClause(value string) {
	var clause string
	if m.pickerEnumQuoted {
		clause = fmt.Sprintf(`%s %s %q`, m.pickerEnumField, m.pickerEnumOp, value)
	} else {
		clause = fmt.Sprintf(`%s %s %s`, m.pickerEnumField, m.pickerEnumOp, value)
	}
	m.appendQueryClause(clause)
}
