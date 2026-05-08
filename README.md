# lazygcl

A terminal viewer for Google Cloud Logging. Type a [Logging query language](https://cloud.google.com/logging/docs/view/logging-query-language) filter, see matching entries, drill into the JSON payload — without leaving your shell.

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
| `Esc` (in detail) | Back to results |
| `q` / `Ctrl+C` | Quit |

Auto-paging fires when the cursor is near the end of the loaded results.

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

## License

MIT
