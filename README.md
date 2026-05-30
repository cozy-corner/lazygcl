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
| `Ctrl+F` | Open the field picker (top-level LogEntry fields) |
| `Ctrl+R` (in query) | Recall a previously-submitted query from history |
| In picker: `↑` / `↓`, `Ctrl+J` / `Ctrl+K`, `Ctrl+N` / `Ctrl+P` | Move cursor |
| In picker: `Enter` | Accept the highlighted item |
| `Esc` | Back (close detail / cancel picker) |
| `q` / `Ctrl+C` | Quit |

The query pane highlights LQL syntax in place as you type — keywords (`AND`/`OR`/`NOT`), operators, strings, and numbers are coloured directly in the editable field.

`Ctrl+F` opens a single picker over the LogEntry top-level fields you can filter on. Selecting a field dispatches by strategy:

- **Dynamic value picker** — values fetched live from Cloud Logging (e.g. `logName`, resource types, label keys).
- **Object sub-picker** — drills into a nested object (e.g. `resource`, `httpRequest`) and dispatches by strategy on the chosen sub-field.
- **Enum picker** — picks from a fixed value list (severities, bools, HTTP methods).
- **Skeleton** — inserts `<field> <op> ""` (unquoted for numeric fields) with the cursor positioned for the value.
- **Label key entry** — for maps with app-defined keys (top-level `labels`), prompts for the key in a textinput and inserts `labels.<key> = ""` (key auto-quoted when Cloud Logging requires it).

All pickers support fzf-style fuzzy filtering: `gci` matches `gce_instance`, `run` matches `cloud_run_revision`, and so on. The resource type picker lists every monitored resource type Cloud Logging knows about (global catalog) — picking one that isn't producing logs in your project just yields an empty result. The logName picker lists names that actually have entries in the bound project.

`Ctrl+R` from the query pane opens a per-project history picker (newest first, same fzf filter). Selecting an entry replaces the query value but does not auto-submit — press Enter to run it. Multi-line queries are collapsed to one line in the list but restored intact when chosen. History is stored as JSONL under `$XDG_DATA_HOME/lazygcl/history/<project>.jsonl` (defaults to `~/.local/share/lazygcl/history/...`), capped at 500 entries per project, with consecutive duplicates dropped.

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
mise install        # installs all tools pinned in .mise.toml
mise run install    # installs the lefthook pre-commit hook (see lefthook.yml for what it runs)
```

Run `mise tasks` to see available tasks (defined in `.mise.toml`). CI runs `mise run check` — pass that locally before opening a PR.

## License

MIT
