package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// typeRunes feeds each rune of s to the input as a KeyRunes message.
func typeRunes(qi queryInput, s string) queryInput {
	for _, r := range s {
		qi, _ = qi.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return qi
}

// press sends a single non-rune key (optionally with Alt) to the input.
func press(qi queryInput, t tea.KeyType, alt bool) queryInput {
	qi, _ = qi.Update(tea.KeyMsg{Type: t, Alt: alt})
	return qi
}

func TestQueryInputValueRoundTrip(t *testing.T) {
	qi := newQueryInput()
	if got := qi.Value(); got != "" {
		t.Fatalf("new input: want empty value, got %q", got)
	}

	qi.SetValue(`severity >= "ERROR"`)
	if got := qi.Value(); got != `severity >= "ERROR"` {
		t.Fatalf("round trip: got %q", got)
	}
}

func TestQueryInputTypeInsertsAtCursor(t *testing.T) {
	qi := typeRunes(newQueryInput(), "abc")
	if got := qi.Value(); got != "abc" {
		t.Fatalf("typing appended: got %q", got)
	}

	// Move left twice and insert: "aXbc".
	qi = press(qi, tea.KeyLeft, false)
	qi = press(qi, tea.KeyLeft, false)
	qi = typeRunes(qi, "X")
	if got := qi.Value(); got != "aXbc" {
		t.Fatalf("insert at cursor: got %q", got)
	}
}

func TestQueryInputBackspaceDeleteHomeEnd(t *testing.T) {
	qi := typeRunes(newQueryInput(), "abc")
	qi = press(qi, tea.KeyBackspace, false) // "ab", cursor at end
	if got := qi.Value(); got != "ab" {
		t.Fatalf("backspace: got %q", got)
	}

	qi = press(qi, tea.KeyHome, false) // cursor at col 0
	qi = press(qi, tea.KeyDelete, false)
	if got := qi.Value(); got != "b" {
		t.Fatalf("delete at home: got %q", got)
	}

	qi = press(qi, tea.KeyEnd, false)
	qi = typeRunes(qi, "z")
	if got := qi.Value(); got != "bz" {
		t.Fatalf("end then type: got %q", got)
	}
}

func TestQueryInputBackspaceJoinsLines(t *testing.T) {
	qi := newQueryInput()
	qi.SetValue("ab\ncd")
	qi = press(qi, tea.KeyEnd, false)       // end of line 1
	qi = press(qi, tea.KeyRight, false)     // start of line 2
	qi = press(qi, tea.KeyBackspace, false) // join: "abcd", cursor between b and c
	if got := qi.Value(); got != "abcd" {
		t.Fatalf("backspace join: got %q", got)
	}
	qi = typeRunes(qi, "-")
	if got := qi.Value(); got != "ab-cd" {
		t.Fatalf("cursor after join: got %q", got)
	}
}

func TestQueryInputAltEnterSplitsLine(t *testing.T) {
	qi := typeRunes(newQueryInput(), "ab")
	qi = press(qi, tea.KeyEnter, true) // Alt+Enter -> newline
	qi = typeRunes(qi, "cd")
	if got := qi.Value(); got != "ab\ncd" {
		t.Fatalf("alt+enter newline: got %q", got)
	}
}

func TestQueryInputPlainEnterIgnored(t *testing.T) {
	qi := typeRunes(newQueryInput(), "ab")
	qi = press(qi, tea.KeyEnter, false) // top-level Enter submits; widget ignores it
	qi = typeRunes(qi, "cd")
	if got := qi.Value(); got != "abcd" {
		t.Fatalf("plain enter must not insert newline: got %q", got)
	}
}

func TestQueryInputSetCursorOffsetInLastLine(t *testing.T) {
	qi := newQueryInput()
	qi.SetValue("a\nbcd")
	qi.SetCursor(2) // offset within last line "bcd"
	qi = typeRunes(qi, "X")
	if got := qi.Value(); got != "a\nbcXd" {
		t.Fatalf("SetCursor offset in last line: got %q", got)
	}
}

func TestQueryInputPasteSplitsOnNewline(t *testing.T) {
	qi := typeRunes(newQueryInput(), "x")
	// Bracketed paste arrives as KeyRunes with Paste set and may carry newlines.
	qi, _ = qi.Update(tea.KeyMsg{Type: tea.KeyRunes, Paste: true, Runes: []rune("a\nb")})
	if got := qi.Value(); got != "xa\nb" {
		t.Fatalf("paste with newline: got %q", got)
	}
	// The paste must split the buffer into two real lines: Home then a keystroke
	// lands at the start of the second line, not inside a line holding a literal
	// newline rune.
	qi = press(qi, tea.KeyHome, false)
	qi = typeRunes(qi, "Q")
	if got := qi.Value(); got != "xa\nQb" {
		t.Fatalf("paste must split into lines: got %q", got)
	}
}

func TestLexLQLTokenizes(t *testing.T) {
	line := []rune(`severity >= "ERROR" AND n=100 (x)`)
	toks := lexLQL(line)

	// Concatenating token spans must reproduce the input exactly.
	var sb []rune
	for _, tk := range toks {
		sb = append(sb, line[tk.start:tk.end]...)
	}
	if string(sb) != string(line) {
		t.Fatalf("tokens do not cover input: got %q", string(sb))
	}

	// Check the non-whitespace tokens.
	type pair struct {
		kind tokKind
		text string
	}
	var got []pair
	for _, tk := range toks {
		if tk.kind == tokWhitespace {
			continue
		}
		got = append(got, pair{tk.kind, string(line[tk.start:tk.end])})
	}
	want := []pair{
		{tokIdent, "severity"},
		{tokOperator, ">="},
		{tokString, `"ERROR"`},
		{tokKeyword, "AND"},
		{tokIdent, "n"},
		{tokOperator, "="},
		{tokNumber, "100"},
		{tokPunct, "("},
		{tokIdent, "x"},
		{tokPunct, ")"},
	}
	if len(got) != len(want) {
		t.Fatalf("token count: got %d %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestQueryInputViewShowsValueAndPlaceholder(t *testing.T) {
	qi := newQueryInput()
	qi.Placeholder = "type a filter"
	qi.SetWidth(60)
	if got := stripANSI(qi.View()); !strings.Contains(got, "type a filter") {
		t.Fatalf("empty view should show placeholder: %q", got)
	}

	qi.SetValue(`severity >= "ERROR"`)
	if got := stripANSI(qi.View()); !strings.Contains(got, `severity >= "ERROR"`) {
		t.Fatalf("view should show value: %q", got)
	}
}

func TestQueryInputCursorDisplayColCountsCJK(t *testing.T) {
	qi := newQueryInput()
	qi.SetValue(`あいA`) // widths: 2 + 2 + 1
	qi.SetCursor(2)    // after the two CJK runes, before "A"
	if got := qi.cursorDisplayCol(); got != 4 {
		t.Fatalf("cursor display column after two CJK runes: got %d, want 4", got)
	}
}

func TestQueryInputTokenKindUnderCursor(t *testing.T) {
	line := []rune(`resource.type = "あいう"`)
	// A column inside the quoted string must report tokString even though a
	// CJK rune precedes it.
	idx := strings.Index(string(line), "あ")
	col := len([]rune(string(line)[:idx])) + 1 // inside the string token
	if got := tokenKindAt(line, col); got != tokString {
		t.Fatalf("token under cursor inside string: got %d, want tokString(%d)", got, tokString)
	}
	// A column inside the leading field path must report tokIdent.
	if got := tokenKindAt(line, 2); got != tokIdent {
		t.Fatalf("token under cursor inside identifier: got %d, want tokIdent(%d)", got, tokIdent)
	}
}

func TestQueryInputCursorRendersReverseWhenFocused(t *testing.T) {
	// lipgloss suppresses styling unless a colour profile is set, so force ANSI
	// to make the reverse-video cursor cell observable.
	prev := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.ANSI)
	t.Cleanup(func() { lipgloss.SetColorProfile(prev) })

	qi := newQueryInput()
	qi.SetWidth(60)
	qi.SetValue("abc")

	qi.Blur()
	if strings.Contains(qi.View(), "\x1b[7m") {
		t.Fatalf("blurred view must not draw a reverse cursor cell")
	}
	qi.Focus()
	if !strings.Contains(qi.View(), "\x1b[7m") {
		t.Fatalf("focused view must draw a reverse cursor cell")
	}
}

func TestDisplayWidthCountsCJKAsTwo(t *testing.T) {
	if w := displayWidth([]rune("あA")); w != 3 {
		t.Fatalf("display width of \"あA\": got %d, want 3", w)
	}
}

func TestQueryInputViewClipsToWidth(t *testing.T) {
	qi := newQueryInput()
	qi.SetWidth(10)
	qi.Blur() // avoid cursor cell affecting width
	qi.SetValue(strings.Repeat("x", 40))
	for _, line := range strings.Split(stripANSI(qi.View()), "\n") {
		if displayWidth([]rune(line)) > 10 {
			t.Fatalf("line exceeds width 10: %q (w=%d)", line, displayWidth([]rune(line)))
		}
	}
}
