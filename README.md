# lazygcl

A terminal viewer for Google Cloud Logging. Type a [Logging query language](https://cloud.google.com/logging/docs/view/logging-query-language) filter, see matching entries, and view each one as the same wire-format JSON `gcloud logging read --format=json` would emit — without leaving your shell.

Inspired by [lazycwl](https://github.com/myuron/lazycwl) (its AWS counterpart).

## Install

```sh
go install github.com/cozy-corner/lazygcl/cmd/lazygcl@latest
```

## Authenticate

`lazygcl` uses Application Default Credentials. Run once:

```sh
gcloud auth application-default login
```

Your account needs `roles/logging.viewer` (or higher) on the project.

## Run

```sh
lazygcl --project my-gcp-project
```

Project resolution order: `--project` flag → `GOOGLE_CLOUD_PROJECT` env → `gcloud config get-value project`.

## Keys

| Key | Action |
|---|---|
| `Tab` | Toggle focus between query pane and results |
| `Enter` (in query) | Run the search |
| `Alt+Enter` (in query) | Insert a newline |
| `Enter` (in results) | Open the entry's detail view |
| `j` / `k`, `↑` / `↓` | Move cursor |
| `g` / `G` | Jump to top / bottom |
| `Ctrl+R` | Open the resource type picker |
| `Ctrl+L` | Open the logName picker |
| In picker: `↑` / `↓`, `Ctrl+J` / `Ctrl+K`, `Ctrl+N` / `Ctrl+P` | Move cursor |
| In picker: `Enter` | Accept the highlighted item |
| `Esc` | Back (close detail / cancel picker) |
| `q` / `Ctrl+C` | Quit |

Both pickers support fzf-style fuzzy filtering: `gci` matches `gce_instance`, `run` matches `cloud_run_revision`, and so on. The resource type picker lists every monitored resource type Cloud Logging knows about (global catalog) — picking one that isn't producing logs in your project just yields an empty result. The logName picker lists names that actually have entries in the bound project.

Auto-paging fires when the cursor is near the end of the loaded results.

## Detail view

Pressing `Enter` on a result renders the entry as a single LogEntry JSON object — the same shape as the Cloud Logging REST API and `gcloud logging read --format=json`. JSON is syntax-highlighted with [chroma](https://github.com/alecthomas/chroma) (monokai). Copy/pipe-friendly for further `jq`-style work.

## Example queries

```
severity >= "ERROR"
```

```
resource.type = "cloud_run_revision" AND
severity >= "WARNING" AND
timestamp > "2026-05-01T00:00:00Z"
```

```
jsonPayload.message =~ "timeout.*"
```

Boolean operators (`AND` / `OR` / `NOT`) **must be uppercase** — lowercase is parsed as a search term. Timestamps are RFC 3339.

## Scope

v1 is read-only history queries. Live tail (`entries.tail`) is planned for v2. The Cloud Logging API caps `entries.list` at **60 requests / minute / project**, so very rapid scrolling through large result sets can hit the quota.

## Development

Toolchain pinned via [mise](https://mise.jdx.dev/). After cloning:

```sh
mise install        # installs go, golangci-lint, lefthook, goimports, govulncheck at the pinned versions
mise run install    # wires up lefthook pre-commit hooks (gofmt + goimports auto-format)
```

Common tasks (defined in `.mise.toml`, run with the pinned tool versions):

```sh
mise run build      # go build
mise run test       # go test ./...
mise run lint       # golangci-lint run ./...
mise run fmt        # in-place gofmt + goimports
mise run vuln       # govulncheck ./...
mise run check      # lint + test + vuln
```

## License

MIT
