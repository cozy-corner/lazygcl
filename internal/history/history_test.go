package history

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestLoad_NoFile returns an empty slice (no error) when the project has no
// history file yet — fresh install case.
func TestLoad_NoFile(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	got, err := Load("never-touched")
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("Load returned %d entries, want 0", len(got))
	}
}

// TestAppendLoad_NewestFirst verifies the ordering contract: callers see the
// most recent entry at index 0.
func TestAppendLoad_NewestFirst(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	for _, q := range []string{`severity >= "ERROR"`, `severity >= "WARNING"`} {
		if err := Append("p", q); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	got, err := Load("p")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	want := []string{`severity >= "WARNING"`, `severity >= "ERROR"`}
	if !equal(got, want) {
		t.Errorf("Load = %q, want %q", got, want)
	}
}

// TestAppend_ConsecutiveDedup drops a repeat of the most recent entry, but
// non-adjacent repeats are kept (per issue: "consecutive-dedup").
func TestAppend_ConsecutiveDedup(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	for _, q := range []string{"a", "a", "b", "a"} {
		if err := Append("p", q); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}
	got, _ := Load("p")
	want := []string{"a", "b", "a"} // newest-first; the duplicate "a" right after "a" is dropped
	if !equal(got, want) {
		t.Errorf("Load = %q, want %q", got, want)
	}
}

// TestAppend_SkipsEmpty does not record an empty / whitespace-only query.
func TestAppend_SkipsEmpty(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	for _, q := range []string{"", "   ", "\n", "real"} {
		if err := Append("p", q); err != nil {
			t.Fatalf("Append(%q): %v", q, err)
		}
	}
	got, _ := Load("p")
	if !equal(got, []string{"real"}) {
		t.Errorf("Load = %q, want [\"real\"]", got)
	}
}

// TestAppend_MultilineRoundTrip preserves embedded newlines, which is why the
// on-disk format is JSONL rather than plain newline-delimited.
func TestAppend_MultilineRoundTrip(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	q := "severity >= \"ERROR\" AND\nresource.type = \"gce_instance\""
	if err := Append("p", q); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, _ := Load("p")
	if !equal(got, []string{q}) {
		t.Errorf("Load = %q, want %q", got, []string{q})
	}
}

// TestAppend_Trim keeps the file at maxEntries when it would otherwise grow
// past the cap, dropping the oldest.
func TestAppend_Trim(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	// maxEntries+1 distinct entries → the first must be evicted.
	total := maxEntries + 1
	for i := 0; i < total; i++ {
		if err := Append("p", entryAt(i)); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}
	got, _ := Load("p")
	if len(got) != maxEntries {
		t.Fatalf("len(Load) = %d, want %d", len(got), maxEntries)
	}
	// Newest-first: got[0] is the last entry, got[maxEntries-1] is the oldest kept.
	if got[0] != entryAt(total-1) {
		t.Errorf("newest = %q, want %q", got[0], entryAt(total-1))
	}
	if got[maxEntries-1] != entryAt(1) {
		t.Errorf("oldest kept = %q, want %q (entry 0 should have been dropped)",
			got[maxEntries-1], entryAt(1))
	}
}

// TestAppend_PerProjectIsolation ensures queries for project A do not leak
// into project B's history.
func TestAppend_PerProjectIsolation(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	if err := Append("foo", "from foo"); err != nil {
		t.Fatal(err)
	}
	if err := Append("bar", "from bar"); err != nil {
		t.Fatal(err)
	}
	gotFoo, _ := Load("foo")
	gotBar, _ := Load("bar")
	if !equal(gotFoo, []string{"from foo"}) {
		t.Errorf("foo = %q", gotFoo)
	}
	if !equal(gotBar, []string{"from bar"}) {
		t.Errorf("bar = %q", gotBar)
	}
}

// TestAppend_SanitizesProjectID guards the filename path: an attacker-supplied
// projectID with separators or traversal must not escape the data dir.
func TestAppend_SanitizesProjectID(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", root)
	bad := "../escape/attempt"
	if err := Append(bad, "x"); err != nil {
		t.Fatalf("Append: %v", err)
	}
	// Whatever filename was chosen, the file must live directly under
	// <root>/lazygcl/history — no traversal upward, no extra subdirs.
	wantDir := filepath.Join(root, "lazygcl", "history")
	matches, err := filepath.Glob(filepath.Join(wantDir, "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("expected exactly one jsonl file under data dir, found %d: %v", len(matches), matches)
	}
	gotDir := filepath.Dir(matches[0])
	if gotDir != wantDir {
		t.Errorf("file landed in %q, want %q", gotDir, wantDir)
	}
	// And the filename itself must contain none of the path separators or
	// traversal sequences from the input.
	base := filepath.Base(matches[0])
	for _, bad := range []string{"/", `\`, "../"} {
		if strings.Contains(base, bad) {
			t.Errorf("filename %q contains forbidden sequence %q", base, bad)
		}
	}
}

func entryAt(i int) string { return "q" + itoa(i) }

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
