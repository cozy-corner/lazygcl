// Package history persists per-project query history under
// $XDG_DATA_HOME/lazygcl/history (or ~/.local/share/lazygcl/history) as JSONL.
// JSONL — not plain newline-delimited — is required because queries can
// contain literal newlines (Alt+Enter in the TUI query pane).
package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// maxEntries caps the file size. When Append would push the file past this,
// the oldest entries are dropped.
const maxEntries = 500

// unsafeFilenameChars matches anything outside a conservative whitelist. Project
// IDs are normally [a-z0-9-]+; this defends against a resolved value that
// somehow contains separators or traversal.
var unsafeFilenameChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

type record struct {
	Q string `json:"q"`
}

// dataDir returns the directory that holds history files. It does not create
// the directory.
func dataDir() (string, error) {
	base := os.Getenv("XDG_DATA_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(base, "lazygcl", "history"), nil
}

func sanitize(projectID string) string {
	safe := unsafeFilenameChars.ReplaceAllString(projectID, "_")
	// Reject names that would still be path-significant after the regex pass.
	if safe == "" || safe == "." || safe == ".." {
		return "_"
	}
	return safe
}

func pathFor(projectID string) (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, sanitize(projectID)+".jsonl"), nil
}

// readAll returns entries in file order (oldest-first). Returns (nil, nil)
// when the file does not yet exist.
func readAll(projectID string) ([]string, error) {
	path, err := pathFor(projectID)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var entries []string
	sc := bufio.NewScanner(f)
	// Allow large single-line records — multi-line queries become long lines
	// once JSON-encoded.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		var r record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			continue
		}
		entries = append(entries, r.Q)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

// Load returns history entries for projectID, newest-first. Missing file is
// not an error.
func Load(projectID string) ([]string, error) {
	entries, err := readAll(projectID)
	if err != nil {
		return nil, err
	}
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}
	return entries, nil
}

// Append records query for projectID. Empty / whitespace-only queries are
// dropped, and a query equal to the most recent entry is dropped
// (consecutive-dedup). When the file would grow past maxEntries the oldest
// entries are evicted.
func Append(projectID, query string) error {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}

	existing, err := readAll(projectID)
	if err != nil {
		return err
	}
	if n := len(existing); n > 0 && existing[n-1] == q {
		return nil
	}
	existing = append(existing, q)
	if len(existing) > maxEntries {
		existing = existing[len(existing)-maxEntries:]
	}
	return writeAll(projectID, existing)
}

// writeAll rewrites the history file atomically: write a tmp file in the same
// directory, then rename over the target.
func writeAll(projectID string, entries []string) error {
	path, err := pathFor(projectID)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "history-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	// Best-effort cleanup if anything below fails before the rename succeeds.
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	bw := bufio.NewWriter(tmp)
	enc := json.NewEncoder(bw)
	for _, q := range entries {
		if err := enc.Encode(record{Q: q}); err != nil {
			_ = tmp.Close()
			return err
		}
	}
	if err := bw.Flush(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
