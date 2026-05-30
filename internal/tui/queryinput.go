package tui

import (
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// queryInput is an in-place, syntax-highlighting text input for the LQL filter.
// It replaces bubbles/textarea so the query pane can colour LQL tokens directly
// inside the editable field with a single cursor. The backing store is a slice
// of lines (queries are short, so a gap buffer is unnecessary); the cursor is a
// (line, col) pair in rune coordinates.
type queryInput struct {
	lines       [][]rune
	row         int // cursor line index
	col         int // cursor column, in runes, within lines[row]
	width       int // render width in cells; 0 means unconstrained
	focused     bool
	Placeholder string
}

// newQueryInput returns an empty, single-line input.
func newQueryInput() queryInput {
	return queryInput{lines: [][]rune{{}}}
}

// Focus and Blur toggle whether the cursor cell is drawn.
func (q *queryInput) Focus() { q.focused = true }
func (q *queryInput) Blur()  { q.focused = false }

// SetWidth sets the render width in cells.
func (q *queryInput) SetWidth(w int) { q.width = w }

// Height returns the number of buffer lines.
func (q queryInput) Height() int { return len(q.lines) }

// Value returns the buffer contents, lines joined by "\n".
func (q queryInput) Value() string {
	parts := make([]string, len(q.lines))
	for i, line := range q.lines {
		parts[i] = string(line)
	}
	return strings.Join(parts, "\n")
}

// SetValue replaces the buffer with s, splitting on "\n", and parks the cursor
// at the end (matching bubbles/textarea, so history recall lands the cursor
// after the recalled query rather than before it).
func (q *queryInput) SetValue(s string) {
	raw := strings.Split(s, "\n")
	q.lines = make([][]rune, len(raw))
	for i, line := range raw {
		q.lines[i] = []rune(line)
	}
	q.row = len(q.lines) - 1
	q.col = len(q.lines[q.row])
}

// SetCursor places the cursor on the last line at rune offset pos. This mirrors
// the contract the picker relies on (picker.go), which inserts a clause and then
// positions the cursor within the final line.
func (q *queryInput) SetCursor(pos int) {
	q.row = len(q.lines) - 1
	q.col = pos
	q.clamp()
}

// Update handles a single message, returning the new input and any command.
func (q queryInput) Update(msg tea.Msg) (queryInput, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyRunes, tea.KeySpace:
			q.insertRunes(msg.Runes)
		case tea.KeyLeft:
			q.moveLeft()
		case tea.KeyRight:
			q.moveRight()
		case tea.KeyHome:
			q.col = 0
		case tea.KeyEnd:
			q.col = len(q.lines[q.row])
		case tea.KeyBackspace:
			q.backspace()
		case tea.KeyDelete:
			q.deleteForward()
		case tea.KeyEnter:
			// Top-level Enter submits the query (handled in keys.go); only
			// Alt+Enter inserts a newline here.
			if msg.Alt {
				q.splitLine()
			}
		}
	}
	return q, nil
}

// insertRunes inserts rs at the cursor and advances past them. A newline rune
// (e.g. from a bracketed paste, which arrives as KeyRunes with embedded "\n")
// splits the line rather than landing in the buffer literally.
func (q *queryInput) insertRunes(rs []rune) {
	for _, r := range rs {
		if r == '\n' {
			q.splitLine()
			continue
		}
		line := q.lines[q.row]
		next := make([]rune, 0, len(line)+1)
		next = append(next, line[:q.col]...)
		next = append(next, r)
		next = append(next, line[q.col:]...)
		q.lines[q.row] = next
		q.col++
	}
}

// splitLine breaks the current line at the cursor, moving the tail to a new
// line below and placing the cursor at its start.
func (q *queryInput) splitLine() {
	line := q.lines[q.row]
	head := append([]rune{}, line[:q.col]...)
	tail := append([]rune{}, line[q.col:]...)
	q.lines[q.row] = head
	q.lines = append(q.lines, nil)
	copy(q.lines[q.row+2:], q.lines[q.row+1:])
	q.lines[q.row+1] = tail
	q.row++
	q.col = 0
}

// backspace deletes the rune before the cursor, joining with the previous line
// when the cursor is at column 0.
func (q *queryInput) backspace() {
	if q.col > 0 {
		line := q.lines[q.row]
		q.lines[q.row] = append(line[:q.col-1], line[q.col:]...)
		q.col--
		return
	}
	if q.row > 0 {
		prev := q.lines[q.row-1]
		q.col = len(prev)
		q.lines[q.row-1] = append(prev, q.lines[q.row]...)
		q.lines = append(q.lines[:q.row], q.lines[q.row+1:]...)
		q.row--
	}
}

// deleteForward deletes the rune under the cursor, joining with the next line
// when the cursor is at the end of a line.
func (q *queryInput) deleteForward() {
	line := q.lines[q.row]
	if q.col < len(line) {
		q.lines[q.row] = append(line[:q.col], line[q.col+1:]...)
		return
	}
	if q.row < len(q.lines)-1 {
		q.lines[q.row] = append(line, q.lines[q.row+1]...)
		q.lines = append(q.lines[:q.row+1], q.lines[q.row+2:]...)
	}
}

func (q *queryInput) moveLeft() {
	if q.col > 0 {
		q.col--
	} else if q.row > 0 {
		q.row--
		q.col = len(q.lines[q.row])
	}
}

func (q *queryInput) moveRight() {
	if q.col < len(q.lines[q.row]) {
		q.col++
	} else if q.row < len(q.lines)-1 {
		q.row++
		q.col = 0
	}
}

// clamp keeps the cursor within the buffer bounds.
func (q *queryInput) clamp() {
	if q.row > len(q.lines)-1 {
		q.row = len(q.lines) - 1
	}
	if q.row < 0 {
		q.row = 0
	}
	if q.col > len(q.lines[q.row]) {
		q.col = len(q.lines[q.row])
	}
	if q.col < 0 {
		q.col = 0
	}
}

// View renders the buffer with LQL tokens coloured in place and the cursor cell
// reverse-styled. Lines are clipped to the render width (no soft-wrap, no
// horizontal scroll); queries are short and multi-clause filters break across
// lines, so this is sufficient for the MVP.
func (q queryInput) View() string {
	if q.isEmpty() {
		placeholder := dimStyle.Render(q.Placeholder)
		if q.focused {
			return cursorCellStyle.Render(" ") + placeholder
		}
		return placeholder
	}

	rendered := make([]string, len(q.lines))
	for li, line := range q.lines {
		rendered[li] = q.renderLine(li, line)
	}
	return strings.Join(rendered, "\n")
}

// renderLine colours one line and overlays the cursor when it sits on this line.
func (q queryInput) renderLine(li int, line []rune) string {
	styleOf := make([]lipgloss.Style, len(line))
	for _, tk := range lexLQL(line) {
		for c := tk.start; c < tk.end; c++ {
			styleOf[c] = lqlStyles[tk.kind]
		}
	}

	var b strings.Builder
	w := 0
	for c, r := range line {
		rw := runewidth.RuneWidth(r)
		if q.width > 0 && w+rw > q.width {
			return b.String() // clip: no horizontal scroll in the MVP
		}
		if q.focused && li == q.row && c == q.col {
			b.WriteString(cursorCellStyle.Render(string(r)))
		} else {
			b.WriteString(styleOf[c].Render(string(r)))
		}
		w += rw
	}
	// Cursor parked at the end of the line: draw it over a trailing space.
	if q.focused && li == q.row && q.col == len(line) && (q.width == 0 || w < q.width) {
		b.WriteString(cursorCellStyle.Render(" "))
	}
	return b.String()
}

// isEmpty reports whether the buffer holds no text at all.
func (q queryInput) isEmpty() bool {
	return len(q.lines) == 1 && len(q.lines[0]) == 0
}

// lqlStyles colours tokens with a small monokai-ish palette using 256-colour
// codes (matching the styles in model.go). Identifiers and whitespace have no
// entry, so they render on the default foreground.
var lqlStyles = map[tokKind]lipgloss.Style{
	tokKeyword:  lipgloss.NewStyle().Foreground(lipgloss.Color("197")), // pink
	tokOperator: lipgloss.NewStyle().Foreground(lipgloss.Color("197")), // pink
	tokString:   lipgloss.NewStyle().Foreground(lipgloss.Color("186")), // yellow
	tokNumber:   lipgloss.NewStyle().Foreground(lipgloss.Color("141")), // purple
	tokPunct:    lipgloss.NewStyle().Foreground(lipgloss.Color("245")), // grey
}

// cursorCellStyle draws the cursor as a reverse-video cell over the rune beneath
// it. The cursor is static (no blink); the colour underneath is irrelevant
// because reverse video wins.
var cursorCellStyle = lipgloss.NewStyle().Reverse(true)

// displayWidth returns the terminal cell width of rs, counting CJK runes as 2.
func displayWidth(rs []rune) int { return runewidth.StringWidth(string(rs)) }

// cursorDisplayCol returns the cursor's column in terminal cells, accounting for
// the width of any wide runes to its left on the current line.
func (q queryInput) cursorDisplayCol() int {
	return displayWidth(q.lines[q.row][:q.col])
}

// tokKind classifies an LQL token for syntax colouring. The set is intentionally
// minimal; regex operators, quoted field keys, functions and timestamp literals
// are deferred (see issue #20).
type tokKind int

const (
	tokIdent      tokKind = iota // field paths and bare values (the catch-all)
	tokKeyword                   // AND / OR / NOT (uppercase only)
	tokOperator                  // = != > >= < <= :
	tokString                    // "..."
	tokNumber                    // integer / decimal literals
	tokPunct                     // ( )
	tokWhitespace                // runs of spaces
)

// lqlToken is a half-open rune span [start,end) over a single line.
type lqlToken struct {
	kind       tokKind
	start, end int
}

// lexLQL splits one line into tokens. The concatenated spans always reproduce
// the input, so the renderer can colour each span in place without losing text.
func lexLQL(line []rune) []lqlToken {
	var toks []lqlToken
	i := 0
	for i < len(line) {
		start := i
		r := line[i]
		switch {
		case r == ' ':
			for i < len(line) && line[i] == ' ' {
				i++
			}
			toks = append(toks, lqlToken{tokWhitespace, start, i})
		case r == '"':
			i++ // opening quote
			for i < len(line) && line[i] != '"' {
				i++
			}
			if i < len(line) {
				i++ // closing quote
			}
			toks = append(toks, lqlToken{tokString, start, i})
		case unicode.IsDigit(r):
			for i < len(line) && unicode.IsDigit(line[i]) {
				i++
			}
			if i < len(line) && line[i] == '.' && i+1 < len(line) && unicode.IsDigit(line[i+1]) {
				i++ // decimal point
				for i < len(line) && unicode.IsDigit(line[i]) {
					i++
				}
			}
			toks = append(toks, lqlToken{tokNumber, start, i})
		case isIdentStart(r):
			for i < len(line) && isIdentChar(line[i]) {
				i++
			}
			kind := tokIdent
			switch string(line[start:i]) {
			case "AND", "OR", "NOT":
				kind = tokKeyword
			}
			toks = append(toks, lqlToken{kind, start, i})
		case r == '=' || r == '>' || r == '<' || r == '!' || r == ':':
			i++
			if i < len(line) && line[i] == '=' && (r == '>' || r == '<' || r == '!') {
				i++ // >= <= !=
			}
			toks = append(toks, lqlToken{tokOperator, start, i})
		case r == '(' || r == ')':
			i++
			toks = append(toks, lqlToken{tokPunct, start, i})
		default:
			i++ // unknown rune: keep it as punctuation so lexing always advances
			toks = append(toks, lqlToken{tokPunct, start, i})
		}
	}
	return toks
}

func isIdentStart(r rune) bool { return unicode.IsLetter(r) || r == '_' }

func isIdentChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune("_.-/", r)
}

// tokenKindAt reports the token kind covering rune offset col on line. Offsets
// at or past the end (no covering token) report tokIdent.
func tokenKindAt(line []rune, col int) tokKind {
	for _, tk := range lexLQL(line) {
		if col >= tk.start && col < tk.end {
			return tk.kind
		}
	}
	return tokIdent
}
