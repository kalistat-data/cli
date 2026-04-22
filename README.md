# kalistat

Command-line access to the Kalistat API.

`kalistat` is a Go CLI built with Cobra. It stores API credentials in the system keychain and provides a simple command surface for exploring Kalistat from the terminal or from scripts.

## Quick start

Build the CLI:

```bash
go build -o kalistat
```

Save your API token:

```bash
./kalistat auth login
```

Check authentication:

```bash
./kalistat auth status
```

Fetch API information:

```bash
./kalistat info
./kalistat sources
```

Use `--json` for structured output when scripting:

```bash
./kalistat info --json
./kalistat sources --json
```

## Installation

### Build from source

```bash
git clone https://github.com/kalistat-data/cli.git
cd cli
go build -o kalistat
```

### Install to your path

```bash
go build -o kalistat
mv kalistat /usr/local/bin/
```

Then run:

```bash
kalistat --help
```

## Authentication

Kalistat uses bearer-token authentication.

The CLI stores your token in the system keychain using `zalando/go-keyring`, so the token is not written to disk in plain text.

Log in by saving an existing API token:

```bash
kalistat auth login
```

The command will prompt for your token. Input is hidden.

Check whether a token is configured:

```bash
kalistat auth status
```

Remove the saved token:

```bash
kalistat auth logout
```

## Commands

### `kalistat info`

Calls the API root endpoint and prints basic API information.

```bash
kalistat info
kalistat info --json
```

### `kalistat sources`

Lists available data sources.

```bash
kalistat sources
kalistat sources --json
```

### `kalistat search`

Full-text search across datasets. The query is required; optional flags narrow the result set and paginate. The pretty output includes the `CATEGORY KEY` column so you can copy a key and use it to filter a subsequent search.

```bash
kalistat search employment
kalistat search employment --source istat
kalistat search labour --category-key EU.5.2.1
kalistat search employment --page 2 --page-size 20
kalistat search employment --json
```

Flags:

- `--source istat|eurostat` — restrict to one source
- `--category-key KEY` — restrict to a category subtree (use a key printed in the `CATEGORY KEY` column)
- `--page N`, `--page-size N` — paginate (page size max 200)

### `kalistat category`

Navigate the hierarchical category tree. Four subcommands.

`category tree [<key>]` — render a visual tree. With no key, shows both sources' roots. With a key, shows the subtree rooted there. `--depth` controls how deep (1-5, default 2). `--with-datasets` embeds dataset stubs under their category. `--source istat|eurostat` filters roots when no key is given. `--ascii` switches box-drawing characters to ASCII for terminals that don't render Unicode cleanly.

```bash
kalistat category tree                           # both roots, depth 2
kalistat category tree --source istat            # ISTAT roots only
kalistat category tree IT.5 --depth 3            # Labor market, 3 levels deep
kalistat category tree IT.5.2.1 --with-datasets  # leaf category + its datasets
kalistat category tree IT.5 --ascii              # ASCII connectors (|--, `--)
```

A node with unexplored descendants (the depth cap truncated the view) is marked with `…` after its name.

`category get <key>` — one node and its direct children as a detail view.

```bash
kalistat category get IT.5
```

`category ancestors <key>` — breadcrumb trail from root to the node, with the target flagged `>`.

```bash
kalistat category ancestors IT.5.2.1
#   IT  Italy
#     IT.5  Labor market
#       IT.5.2  Employment
#       > IT.5.2.1  Employed - monthly data
```

`category datasets <key>` — paginated list of datasets attached to a category. `--recursive` walks descendants.

```bash
kalistat category datasets IT.5.2.1
kalistat category datasets IT.5 --recursive --page-size 10
```

Typical navigation: `category tree` → `category tree <key>` → `category datasets <key>` → `dataset get <code>` → `series list <code> '<pattern>'` → `series get <code> <series-code>`.

### `kalistat dataset`

Inspect dataset metadata and dimension values. Two subcommands:

`dataset get <code>` — metadata for a single dataset. Output shows source, dataflow ID, category key, series count, and a sorted table of dimensions (plus time dimensions). Use this to discover the dimension **keys** you'll need for `series list` patterns or `dataset values`.

```bash
kalistat dataset get IT.LAMA.132
kalistat dataset get IT.LAMA.132 --json
```

`dataset values <code> <dim-key>` — allowed values for one dimension, in the same order the filter panel shows. Output is a `CODE / NAME` table (plus a `LEVEL` column when the codelist is hierarchical).

```bash
kalistat dataset values IT.LAMA.132 AGE
kalistat dataset values IT.LAMA.132 REF_AREA --json | jq '.data[].code'
```

Typical discovery workflow: `search` → `dataset get` → `dataset values` → `series list`.

### `kalistat series`

Resolve and fetch time series from a dataset. Two subcommands:

`series list <dataset> <pattern>` — resolve a wildcarded ticker pattern into concrete series. Use `*` as a wildcard in any dimension position.

```bash
kalistat series list IT.LAMA.132 'M.IT.EMP.Y.9.*.9.9.CURRENT'
# → one row per matched ticker, with observation count and time range
```

`series get <dataset> <series-code>` — fetch a single series. Output shows the ticker, a legend of dimension values, and a `TIME / VALUE` table for every observation. Null observations render as `—`.

```bash
kalistat series get IT.LAMA.132 M.IT.EMP.Y.9.Y15-24.9.9.CURRENT
kalistat series get IT.LAMA.132 M.IT.EMP.Y.9.Y15-24.9.9.CURRENT --json | jq '.data.values[-1]'
```

For large series, pipe to standard tools:

```bash
kalistat series get IT.LAMA.132 M.IT.EMP.Y.9.Y15-24.9.9.CURRENT | tail -20
```

### `kalistat auth`

Authentication commands.

```bash
kalistat auth login
kalistat auth status
kalistat auth logout
```

## Configuration

The CLI uses the production API by default:

```text
https://app.kalistat.com/api/v1
```

Override it with the `--base-url` flag (takes precedence) or the `KALISTAT_API_URL` environment variable:

```bash
kalistat --base-url https://staging.kalistat.com/api/v1 sources
export KALISTAT_API_URL=https://staging.kalistat.com/api/v1
```

The flag beats the env var; the env var beats the default. Only `https` URLs are accepted, except for loopback hosts (`localhost`, `127.0.0.1`, `[::1]`) for local development.

## Output

Commands are designed to work well both for humans and for scripts.

Human-readable output is the default. For machine-readable output, pass `--json`:

```bash
kalistat info --json
kalistat sources --json
```

## Using with AI assistants

An Agent Skill that teaches AI assistants how to drive this CLI lives at [`skills/kalistat/SKILL.md`](skills/kalistat/SKILL.md).

- **Claude Code** — clone this repo; the skill is auto-discovered from `skills/kalistat/SKILL.md` when you work in the repo.
- **Claude.ai Projects** — paste the contents of [`skills/kalistat/AGENTS.md`](skills/kalistat/AGENTS.md) into your project's custom instructions.
- **ChatGPT / Cursor / other assistants** — paste [`skills/kalistat/AGENTS.md`](skills/kalistat/AGENTS.md) as a system prompt or rules file.

Edit `SKILL.md` (the source of truth), then run `./skills/kalistat/sync.sh` to regenerate `AGENTS.md`.

## Development

Run the CLI locally:

```bash
go run .
```

Format code:

```bash
go fmt ./...
```

Check modules:

```bash
go mod tidy
```

Run tests:

```bash
go test ./...
```

## Project layout

```text
.
├── cmd
├── internal
├── skills
├── go.mod
├── go.sum
└── main.go
```

- `cmd/` contains Cobra commands
- `internal/` contains API, keychain, and supporting logic
- `skills/` contains Agent Skills that teach AI assistants how to use the CLI

## Status

This is an early version of the CLI.

Current focus:
- authentication (`auth login/status/logout`)
- API info (`info`)
- sources (`sources`)
- full-text search (`search`)
- categories (`category tree`, `category get`, `category ancestors`, `category datasets`)
- datasets (`dataset get`, `dataset values`)
- time series (`series list`, `series get`)

Planned next:
- more endpoint coverage (categories, datasets detail, batch series)
- CSV output alongside JSON
- shell completion
- release packaging

## License

MIT
