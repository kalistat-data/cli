
# Kalistat CLI

The `kalistat` CLI queries ISTAT and Eurostat statistical data through the Kalistat API. This skill teaches you to drive it correctly without guessing flags or inventing dataset codes.

## Quick reference (read first)

- **Auth first, always.** Before any other command, run `kalistat auth status`. If not logged in, run `kalistat auth login` — it prompts for a token (hidden input), or reads the first stdin line when piped.
- **Two happy paths** — pick based on what the user gave you:
  - *Topic is known* (labor, public finance, prices, ...) — **walk the tree**:
    ```
    kalistat category tree IT --depth 1            # top-level domains
    kalistat category tree <KEY> --depth 2-3       # drill in
    kalistat category datasets <KEY>               # datasets in the leaf
    kalistat dataset get <CODE>                    # see dimensions
    kalistat dataset values <CODE> <DIM>           # see allowed values
    kalistat series list <CODE> '<pattern-with-*>' # resolve tickers
    kalistat series get  <CODE> <TICKER>           # fetch observations
    ```
  - *Only keywords* — **search**:
    ```
    kalistat search "<query>" --source istat       # find dataset CODE
    # ... then continue with dataset/series steps above
    ```
  Tree gives you scope and confidence in coverage; search is faster when you don't know the domain. **When in doubt, prefer the tree** — it shows neighboring datasets you'd otherwise miss.
- **Scripting**: add `--json` to any command for machine-readable output. Default is human tables — do not parse them.
- **Pagination**: `--page N --page-size M` (max 200) — supported on `search`, `category datasets`. **Not supported on `series list`**, which has a hard cap of 500 series per pattern; narrow the pattern instead. Look for "showing X of Y" in plain mode, or `meta` fields in JSON.
- **Identifier rule**: dataset codes, series codes, dimension keys, and category keys must match `^[A-Za-z0-9][A-Za-z0-9._-]*$`. No `/`, no leading `.`, no `..`. Validate user-supplied values before passing them.
- **Exit codes**: `0` success, `1` any error. Errors go to stderr in plain mode, stdout in JSON mode.
- **Never invent codes.** Always discover them through `search`, `sources`, or `category tree` first.
- **Slicing long series**: pipe to `jq` to show head + tail without dumping hundreds of points, e.g. `kalistat series get ... --json | jq '.data.values | (.[0:3] + .[-12:])'`.

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
  **Caveat**: this lists values the dimension *can* take, not values that have realized series. The Cartesian product is sparse — e.g. `Y_GE15` may appear in AGE values but no series exists with it. Confirm coverage with `series list` using a wildcard in that dimension before picking a value.

Example:
```
kalistat dataset get IT.LAMA.132
kalistat dataset values IT.LAMA.132 AGE
kalistat dataset values IT.LAMA.132 REF_AREA --json | jq '.data[].code'
```

### series

- `kalistat series list <CODE> '<pattern>'` — resolve a ticker pattern into concrete series. Use `*` as wildcard in any dimension position. **Always single-quote the pattern** to prevent shell glob expansion.
  - The pattern must have exactly as many dot-separated segments as the dataset has dimensions (mismatches return a "Pattern has N segment(s) but dataset has M" error that lists the expected segment order).
  - Hard cap: 500 series per pattern. Beyond that the API rejects with "Too many series requested"; narrow the pattern by replacing wildcards with concrete codes.
  - No `--page` / `--page-size` flags are accepted here.
- `kalistat series get <CODE> <TICKER>` — fetch observations for one series. Null values render as `—` in plain mode, `null` in JSON.

Example:
```
kalistat series list IT.LAMA.132 'M.IT.EMP.Y.9.*.9.9.2026M4G1'
kalistat series get  IT.LAMA.132 M.IT.EMP.Y.9.Y15-24.9.9.2026M4G1
kalistat series get  IT.LAMA.132 M.IT.EMP.Y.9.Y15-24.9.9.2026M4G1 --json | jq '.data.values[-1]'
```

**Recovering from "No matching series found"**: this means the pattern is syntactically valid but no realized series matches — usually a single dimension's code is wrong, not the whole pattern. Widen one dimension at a time to `*` until results appear; the dimension you had to widen is the one with the bad code. Re-check it with `dataset values` (and watch for cross-dataset code conventions — see Gotchas).

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
| `Pattern has N segment(s) but dataset has M` | Your `series list` pattern has the wrong dimension count; the error lists the expected segment order — copy it. |
| `No matching series found` | Pattern is valid but matches nothing. Widen one dimension at a time to `*` to locate the bad code. Often the culprit is `EDITION` (use the latest release) or a SEX/AGE code that differs from the convention you assumed. |
| `Too many series requested (max 500)` | Pattern is too broad. Replace wildcards with concrete codes — `series list` does not paginate. |
| `unknown flag: --page-size` (on `series list`) | Pagination is not supported here; narrow the pattern. |

### Gotchas

- **Dimension order**: API returns dimensions unsorted; the CLI sorts them by `position`. In JSON mode, sort them yourself if order matters.
- **Name truncation**: plain tables truncate long names (category ~40 chars, dataset ~60). Use `--json` for full strings.
- **Rate limits**: surfaced by `kalistat info` as requests/minute. The CLI does not auto-retry; back off manually on 429.
- **Response body cap**: API responses over 10 MiB are rejected. If you hit this, paginate harder.
- **Pattern quoting**: unquoted `*` in a shell will glob to filenames. Always single-quote series patterns.
- **`EDITION` dimension** (release-date versioning): many monthly/quarterly/annual datasets — including labor (`IT.LAMA.132`, `IT.LAMA.301`) and public finance (`IT.PFIN.032`, `IT.PFIN.022`) — carry an `EDITION` dimension whose values are release dates like `2026M4G1` (= 01-Apr-2026). **Always pin the latest edition** in your pattern; otherwise you'll either match every prior release (Cartesian explosion → 500-cap error) or get nothing. Find it with `kalistat dataset values <CODE> EDITION | tail -10`.
- **Dimension code conventions vary across datasets** — never assume. Today's example: `SEX` is `M/F/T` in `IT.LAMA.461` but `1/2/9` in `IT.LAMA.132`. Same concept, different codes. Always re-run `dataset values <CODE> <DIM>` per dataset, don't reuse what you saw elsewhere.
- **Sparse Cartesian product**: a value listed by `dataset values` may not have any realized series (e.g. `Y_GE15` in AGE for some datasets). Verify with `series list` before fetching.

## When you are unsure

- Run `kalistat <command> --help` to see live flag definitions.
- Run `kalistat info` to confirm the API is reachable and see the version / sources / rate limit.
- Prefer discovery commands (`search`, `sources`, `category tree`) over assuming codes.
