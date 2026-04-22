---
name: kalistat
description: Use when the user asks to search, browse, or fetch data from ISTAT, Eurostat, or Kalistat — anything involving statistical datasets, time series, category trees, or the `kalistat` CLI.
---

# Kalistat CLI

The `kalistat` CLI queries ISTAT and Eurostat statistical data through the Kalistat API. This skill teaches you to drive it correctly without guessing flags or inventing dataset codes.

## Quick reference (read first)

- **Auth first, always.** Before any other command, run `kalistat auth status`. If not logged in, run `kalistat auth login` — it prompts for a token (hidden input), or reads the first stdin line when piped.
- **Happy path** for getting data:
  ```
  kalistat search "<query>"                       # find dataset CODE
  kalistat dataset get <CODE>                     # see dimensions
  kalistat dataset values <CODE> <DIM>            # see allowed values
  kalistat series list <CODE> '<pattern-with-*>'  # resolve tickers
  kalistat series get  <CODE> <TICKER>            # fetch observations
  ```
- **Scripting**: add `--json` to any command for machine-readable output. Default is human tables — do not parse them.
- **Pagination**: `--page N --page-size M` (max 200). Look for "showing X of Y" in plain mode, or `meta` fields in JSON.
- **Identifier rule**: dataset codes, series codes, dimension keys, and category keys must match `^[A-Za-z0-9][A-Za-z0-9._-]*$`. No `/`, no leading `.`, no `..`. Validate user-supplied values before passing them.
- **Exit codes**: `0` success, `1` any error. Errors go to stderr in plain mode, stdout in JSON mode.
- **Never invent codes.** Always discover them through `search`, `sources`, or `category tree` first.

## Full reference (consult as needed)

### Global flags (persistent)

- `--json` — emit raw API JSON on stdout instead of formatted tables.
- `--base-url URL` — override API endpoint. Precedence:
  1. `--base-url` flag
  2. `$KALISTAT_API_URL`
  3. default `https://app.kalistat.com/api/v1`

  `http://` is only accepted for loopback hosts (`localhost`, `127.0.0.1`, `[::1]`); everything else must be `https://`.

### auth

- `kalistat auth login` — prompts for token (hidden) in a TTY, or reads first stdin line when piped. Stored in the OS keychain (macOS Keychain / Windows Credential Manager / freedesktop Secret Service).
- `kalistat auth status` — verifies the stored token against the API.
- `kalistat auth logout` — removes the stored token.

### info / sources

- `kalistat info` — CLI version, API version, available sources, rate limit (requests/minute).
- `kalistat sources` — table of data sources (ID, NAME, COUNTRY, KEY).

### search `<query>`

Weighted full-text search across datasets. Query is a required positional; empty queries are rejected.

Flags:
- `--source istat|eurostat` — restrict to one source.
- `--category-key KEY` — restrict to a category subtree.
- `--page N` (default 1)
- `--page-size M` (default 50, max 200)

Examples:
```
kalistat search employment
kalistat search employment --source istat
kalistat search labour --category-key EU.5.2.1 --page-size 20
kalistat search employment --json
```

### category

Navigate the hierarchical category tree.

- `kalistat category tree [KEY]` — render the tree. No key = show all roots.
  Flags: `--depth 1-5` (default 2), `--with-datasets`, `--source istat|eurostat` (only honored when no KEY), `--ascii` (use ASCII connectors).
  Note: `--json` with `--depth > 1` and no KEY is rejected (multiple requests, no single body).
- `kalistat category get <KEY>` — show a node and its direct children.
- `kalistat category ancestors <KEY>` — breadcrumb from root to KEY.
- `kalistat category datasets <KEY>` — list datasets attached to a category.
  Flags: `--recursive` (include descendants), `--page`, `--page-size`.

### dataset

- `kalistat dataset get <CODE>` — metadata: source, dataflow ID, category, series count, dimensions (sorted by position), time dimension.
- `kalistat dataset values <CODE> <DIM_KEY>` — allowed values for a dimension. Output includes LEVEL column for hierarchical dimensions.

Example:
```
kalistat dataset get IT.LAMA.132
kalistat dataset values IT.LAMA.132 AGE
kalistat dataset values IT.LAMA.132 REF_AREA --json | jq '.data[].code'
```

### series

- `kalistat series list <CODE> '<pattern>'` — resolve a ticker pattern into concrete series. Use `*` as wildcard in any dimension position. **Always single-quote the pattern** to prevent shell glob expansion.
- `kalistat series get <CODE> <TICKER>` — fetch observations for one series. Null values render as `—` in plain mode, `null` in JSON.

Example:
```
kalistat series list IT.LAMA.132 'M.IT.EMP.Y.9.*.9.9.CURRENT'
kalistat series get  IT.LAMA.132 M.IT.EMP.Y.9.Y15-24.9.9.CURRENT
kalistat series get  IT.LAMA.132 M.IT.EMP.Y.9.Y15-24.9.9.CURRENT --json | jq '.data.values[-1]'
```

### Error shapes

- **Plain mode**: `Error: <message>` on stderr, exit code 1.
- **JSON mode**: API body `{"error":{"code":"...","message":"..."}}` on stdout, exit code 1.

Common errors and recovery:

| Message substring | Recovery |
|---|---|
| `no API token found` | Run `kalistat auth login` |
| `token is not valid` | Re-login with a fresh token |
| `contains invalid characters` | Fix the identifier to match the regex |
| `search query cannot be empty` | Provide a non-empty query |
| `API is unreachable` | Network issue, or wrong `--base-url` |

### Gotchas

- **Dimension order**: API returns dimensions unsorted; the CLI sorts them by `position`. In JSON mode, sort them yourself if order matters.
- **Name truncation**: plain tables truncate long names (category ~40 chars, dataset ~60). Use `--json` for full strings.
- **Rate limits**: surfaced by `kalistat info` as requests/minute. The CLI does not auto-retry; back off manually on 429.
- **Response body cap**: API responses over 10 MiB are rejected. If you hit this, paginate harder.
- **Pattern quoting**: unquoted `*` in a shell will glob to filenames. Always single-quote series patterns.

## When you are unsure

- Run `kalistat <command> --help` to see live flag definitions.
- Run `kalistat info` to confirm the API is reachable and see the version / sources / rate limit.
- Prefer discovery commands (`search`, `sources`, `category tree`) over assuming codes.
