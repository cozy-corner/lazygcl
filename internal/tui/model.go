package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cozy-corner/lazygcl/internal/gcp"
)

type view int

const (
	viewMain view = iota // query + results
	viewDetail
)

type pane int

const (
	paneQuery pane = iota
	paneResults
)

type Model struct {
	ctx     context.Context
	cancel  context.CancelFunc
	client  *gcp.Client
	project string

	width, height int

	currentView view
	focus       pane

	query   textarea.Model
	detail  viewport.Model
	entries []gcp.LogEntry
	cursor  int
	offset  int

	// Active search state. queryGen is bumped on each submission so stale
	// results from a superseded query can be dropped in Update.
	queryGen     int
	stream       gcp.EntryStream
	streamCtx    context.Context
	streamCancel context.CancelFunc
	streamDone   bool

	loading     bool
	err         error
	errIsSyntax bool
}

type Options struct {
	Client  *gcp.Client
	Project string
}

func NewModel(opts Options) Model {
	ta := textarea.New()
	ta.Placeholder = `severity >= "ERROR" AND timestamp > "2026-05-01T00:00:00Z"`
	ta.SetHeight(4)
	ta.Focus()

	vp := viewport.New(0, 0)

	ctx, cancel := context.WithCancel(context.Background())
	return Model{
		ctx:         ctx,
		cancel:      cancel,
		client:      opts.Client,
		project:     opts.Project,
		currentView: viewMain,
		focus:       paneQuery,
		query:       ta,
		detail:      vp,
	}
}

func (m Model) Init() tea.Cmd { return textarea.Blink }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.query.SetWidth(msg.Width - 2)
		m.detail.Width = msg.Width - 2
		m.detail.Height = msg.Height - 4
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	case errMsg:
		if msg.gen != m.queryGen {
			return m, nil
		}
		m.loading = false
		m.err = msg.err
		m.errIsSyntax = msg.syntax
		return m, nil
	case queryResultMsg:
		if msg.gen != m.queryGen {
			return m, nil
		}
		m.loading = false
		m.entries = append(m.entries, msg.entries...)
		m.streamDone = msg.done
		if m.cursor >= len(m.entries) {
			m.cursor = len(m.entries) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		if len(m.entries) > 0 && m.focus == paneQuery {
			m.focus = paneResults
			m.query.Blur()
		}
		return m, nil
	}

	if m.focus == paneQuery && m.currentView == viewMain {
		var cmd tea.Cmd
		m.query, cmd = m.query.Update(msg)
		return m, cmd
	}
	if m.currentView == viewDetail {
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) View() string {
	switch m.currentView {
	case viewDetail:
		return m.renderDetail()
	default:
		return m.renderMain()
	}
}

var (
	headerStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errorStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	cursorStyle    = lipgloss.NewStyle().Background(lipgloss.Color("236")).Bold(true)
	severityStyles = map[string]lipgloss.Style{
		"Error":    lipgloss.NewStyle().Foreground(lipgloss.Color("9")),
		"Warning":  lipgloss.NewStyle().Foreground(lipgloss.Color("11")),
		"Info":     lipgloss.NewStyle().Foreground(lipgloss.Color("10")),
		"Debug":    lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		"Critical": lipgloss.NewStyle().Foreground(lipgloss.Color("9")).Bold(true),
	}
)

func (m Model) renderMain() string {
	var b strings.Builder

	queryHeader := "Query"
	if m.focus == paneQuery {
		queryHeader = "Query *"
	}
	fmt.Fprintln(&b, headerStyle.Render(queryHeader))
	fmt.Fprintln(&b, m.query.View())
	if m.err != nil && m.errIsSyntax {
		fmt.Fprintln(&b, errorStyle.Render("filter syntax: "+m.err.Error()))
	}

	resultsHeader := fmt.Sprintf("Results (%d)", len(m.entries))
	if m.streamDone {
		resultsHeader += " — end"
	} else if len(m.entries) > 0 {
		resultsHeader += " — more available"
	}
	if m.focus == paneResults {
		resultsHeader += " *"
	}
	fmt.Fprintln(&b, headerStyle.Render(resultsHeader))
	b.WriteString(m.renderResults())

	if m.err != nil && !m.errIsSyntax {
		fmt.Fprintln(&b, errorStyle.Render("error: "+m.err.Error()))
	}
	if m.loading {
		fmt.Fprintln(&b, dimStyle.Render("loading…"))
	}
	fmt.Fprintln(&b, dimStyle.Render("[tab] focus  [enter] run/open  [esc] back  [q] quit  project="+m.project))
	return b.String()
}

func (m Model) renderResults() string {
	if len(m.entries) == 0 {
		return dimStyle.Render("(no results — focus query and press enter to run)\n")
	}
	limit := m.resultsRows()
	end := m.offset + limit
	if end > len(m.entries) {
		end = len(m.entries)
	}
	var b strings.Builder
	for i := m.offset; i < end; i++ {
		row := formatRow(m.entries[i])
		if i == m.cursor {
			row = cursorStyle.Render("> " + row)
		} else {
			row = "  " + row
		}
		b.WriteString(row)
		b.WriteByte('\n')
	}
	return b.String()
}

func formatRow(e gcp.LogEntry) string {
	ts := e.Timestamp.Local().Format("15:04:05")
	sev := e.Severity
	if sev == "" {
		sev = "Default"
	}
	if style, ok := severityStyles[sev]; ok {
		sev = style.Render(padRight(sev, 8))
	} else {
		sev = padRight(sev, 8)
	}
	preview := payloadPreview(e.Payload)
	resource := e.Resource.Type
	return fmt.Sprintf("%s  %s  %-20s  %s", ts, sev, truncate(resource, 20), truncate(preview, 80))
}

func payloadPreview(p gcp.Payload) string {
	switch p.Kind {
	case gcp.PayloadText:
		return strings.ReplaceAll(p.Text, "\n", " ")
	default:
		return strings.ReplaceAll(string(p.JSON), "\n", " ")
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func padRight(s string, n int) string {
	if len(s) >= n {
		return s
	}
	return s + strings.Repeat(" ", n-len(s))
}

func (m Model) resultsRows() int {
	rows := m.height - 4 - m.query.Height() - 2
	if rows < 1 {
		return 1
	}
	return rows
}

func (m Model) renderDetail() string {
	var b strings.Builder
	fmt.Fprintln(&b, headerStyle.Render("Detail"))
	b.WriteString(m.detail.View())
	b.WriteByte('\n')
	fmt.Fprintln(&b, dimStyle.Render("[esc] back  [q] quit"))
	return b.String()
}
