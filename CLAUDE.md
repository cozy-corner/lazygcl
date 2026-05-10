# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

`lazygcl` is a terminal viewer for Google Cloud Logging. The user types a [Logging Query Language](https://cloud.google.com/logging/docs/view/logging-query-language) filter; matching entries are streamed in and each one can be opened as the same JSON shape `gcloud logging read --format=json` returns.

## Toolchain

All Go and lint tooling versions are pinned in `.mise.toml`. After cloning, run `mise install` once to fetch them, then `mise run install` to wire up the lefthook pre-commit hook (which auto-formats staged `*.go` files with `goimports`).

## Common commands

Use the `mise run` wrappers — they pin tool versions matching CI. Calling `go`, `golangci-lint`, etc. directly may use whatever is on your `$PATH` instead.

```sh
mise run build      # go build -o lazygcl ./cmd/lazygcl
mise run test       # go test -race ./...
mise run lint       # golangci-lint run ./...
mise run fmt        # goimports -w .
mise run fmt-check  # fail if any file needs goimports
mise run tidy-check # fail if go.mod / go.sum are not tidy
mise run vuln       # govulncheck ./...
mise run check      # everything CI runs (lint + fmt-check + tidy-check + test + vuln)
```

CI runs `mise run check` and nothing else — keep the two in sync.

## Architecture

### `internal/gcp` — Cloud Logging wrapper

Wraps `cloud.google.com/go/logging/logadmin` so the TUI never touches GCP types directly.

- **`EntryStream` interface**: any new fetch path (including the planned v2 live-tail) must implement it — don't surface `logadmin.EntryIterator` or other `cloud.google.com/go/logging` types to the TUI.
- **`ErrInvalidFilter`**: filter-syntax errors must keep wrapping this so the TUI can route them via `errors.Is` to the query pane instead of the generic error banner.

### `internal/tui` — Bubble Tea UI

Standard Elm-style `Model` / `Update` / `View`. `Model` carries all state; `Update` handlers are split into per-concern files via methods on `Model`.

- **`queryGen` versioning**: any new async command that mutates result state must carry a generation token and be dropped in `Update` when `msg.gen != m.queryGen`. Otherwise stale results from a superseded query leak into the UI.
- **Stream cancellation**: starting a new search without first calling the previous `streamCancel` leaks goroutines blocked in `iterator.Next`.
- **API quota**: Cloud Logging caps `entries.list` at **60 requests / minute / project**. Keep this ceiling in mind when changing `pageBatch` / `pagingThreshold` / `defaultPageSize` or adding retry/backoff.
- **Adding a new picker kind**: extend `pickerKind`, the switch in `title()`, and the dispatch in `applyPickerSelection`. API-backed pickers also need a case in `openPicker`; in-memory pickers use a dedicated `openXxxPicker` helper. The single entry point is `pickerField` (Ctrl+F): top-level fields live in `topLevelFields` with one of four `fieldStrategy` values (dynamic / enum / skeleton / object sub-picker).
- **`wireLogEntry`** (`detail.go`) uses Cloud Logging REST field names (`logName`, `insertId`, `jsonPayload`, ...) so detail-view JSON is byte-equivalent to `gcloud logging read --format=json`. New `gcp.LogEntry` fields must be mirrored here with REST names, not Go-idiomatic ones.
- **Boolean operators in user filters must be uppercase** (`AND`/`OR`/`NOT`) — Cloud Logging requirement. Preserve uppercase when generating filter fragments programmatically.
