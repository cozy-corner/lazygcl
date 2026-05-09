package tui

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/cozy-corner/lazygcl/internal/gcp"
	"github.com/sahilm/fuzzy"
)

type pickerKind int

const (
	pickerResource pickerKind = iota
	pickerLogName
)

func (k pickerKind) title() string {
	switch k {
	case pickerResource:
		return "Resource type"
	default:
		return "Log name"
	}
}

// pickerItem is the unified picker row. Display is shown to the user;
// FilterKey is the lowercased haystack for substring search; Value is what
// gets injected into the query filter on selection.
type pickerItem struct {
	Display   string
	FilterKey string
	Value     string
}

// openPicker resets the picker UI to a loading state and returns the Cmd that
// fetches its items.
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
	m.pickerLoading = true
	m.pickerErr = nil

	client := m.client
	switch kind {
	case pickerResource:
		return m, func() tea.Msg {
			rs, err := client.ListResourceDescriptors(m.ctx)
			if err != nil {
				return pickerErrMsg{kind: kind, err: err}
			}
			return pickerLoadedMsg{kind: kind, resources: rs}
		}
	default:
		return m, func() tea.Msg {
			ns, err := client.ListLogNames(m.ctx)
			if err != nil {
				return pickerErrMsg{kind: kind, err: err}
			}
			return pickerLoadedMsg{kind: kind, logNames: ns}
		}
	}
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
	clause := pickerClause(m.pickerKind, item.Value)

	existing := strings.TrimSpace(m.query.Value())
	if existing == "" {
		m.query.SetValue(clause)
	} else {
		m.query.SetValue(existing + "\nAND " + clause)
	}
	m.currentView = viewMain
	m.focus = paneQuery
	m.query.Focus()
	return m, nil
}

func pickerClause(kind pickerKind, value string) string {
	switch kind {
	case pickerResource:
		return fmt.Sprintf(`resource.type = %q`, value)
	default:
		return fmt.Sprintf(`logName = %q`, value)
	}
}
