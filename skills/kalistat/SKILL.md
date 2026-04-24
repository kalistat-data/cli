---
name: kalistat
description: Use when the user asks to search, browse, or fetch data from ISTAT, Eurostat, or Kalistat — anything involving statistical datasets, time series, category trees, or the `kalistat` CLI.
---

# Kalistat CLI

The `kalistat` CLI queries ISTAT and Eurostat statistical data through the Kalistat API. This skill teaches you to drive it correctly without guessing flags or inventing dataset codes.

## Quick reference (read first)

- **Auth first.** Run `kalistat auth status`. If not logged in, run `kalistat auth login` (hidden prompt, or first stdin line when piped).
- **Two entry points** — pick based on what the user gave you. **Prefer the tree** when the topic is known; it shows neighbouring datasets that search misses.
  - *Topic is known* (labor, public finance, prices, ...):
    ```
    kalistat category tree IT --depth 1            # top-level domains
    kalistat category tree <KEY> --depth 2-3       # drill in
    kalistat category datasets <KEY>               # datasets in the leaf
    kalistat dataset get <CODE>                    # dimensions (+ FIXED VALUE column)
    kalistat dataset values <CODE> <DIM>           # allowed codes for one dimension
    kalistat series list <CODE> '<pattern>'        # resolve tickers (* = wildcard)
    kalistat series get  <CODE> <TICKER>           # fetch observations
    ```
  - *Only keywords*: `kalistat search "<query>" --source istat|eurostat`, then continue as above.
- **Pattern building** — before writing a `series list` pattern:
  1. Run `dataset get <CODE>` to see dimensions. The `FIXED VALUE` column tells you which dimensions have exactly one legal code — copy it into the pattern verbatim; a `*` there is wasted.
  2. For the `EDITION` dimension (release-date versioning), **use `CURRENT`** — the server resolves it to the latest release. `PREVIOUS`, `PREVIOUS_1`, `PREVIOUS_2` resolve to earlier ones. Never leave `EDITION` as `*` or you'll multiply the match count by every prior release.
  3. For the remaining dimensions, use `dataset values <CODE> <DIM>` to find codes.
- **Scripting**: add `--json` to any command for raw API JSON. Default is human tables — do not parse them. Slice long series with `jq`, e.g. `kalistat series get … --json | jq '.data.values | (.[0:3] + .[-12:])'` (head + tail without dumping everything).
- **Identifier rule**: dataset codes, series codes, dimension keys, category keys must match `^[A-Za-z0-9][A-Za-z0-9._-]*$`. No `/`, no leading `.`, no `..`.
- **Always single-quote patterns** (`'A.*.B'`) — unquoted `*` globs to filenames.

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
- `kalistat category ancestors <KEY>` — breadcrumb from root to KEY. Flag: `--ascii`.
- `kalistat category datasets <KEY>` — list datasets attached to a category.
  Flags: `--recursive` (include descendants), `--page`, `--page-size`.

### dataset

- `kalistat dataset get <CODE>` — metadata: source, dataflow ID, category, series count, dimensions (sorted by position), time dimension. Dimensions constrained to a single code show a `FIXED VALUE` column with `CODE (Name)` — copy it into the pattern verbatim.
- `kalistat dataset ancestors <CODE>` — breadcrumb from root to the dataset's parent category. Flag: `--ascii`.
- `kalistat dataset values <CODE> <DIM_KEY>` — allowed codes for a dimension. Output includes `LEVEL` for hierarchical dimensions. For `EDITION`, the last rows are virtual aliases (`PREVIOUS_2`, `PREVIOUS_1`, `PREVIOUS`, `CURRENT`) that resolve server-side — prefer `CURRENT` in patterns.
  **Caveat**: this lists codes the dimension *can* take, not codes with realized series (see "Sparse Cartesian product" in Gotchas).

Example:
```
kalistat dataset get IT.LAMA.132
kalistat dataset ancestors IT.LAMA.132
kalistat dataset values IT.LAMA.132 AGE
kalistat dataset values IT.LAMA.132 REF_AREA --json | jq '.data[].code'
```

### series

- `kalistat series list <CODE> '<pattern>'` — resolve a ticker pattern into concrete series. Use `*` as wildcard in any dimension position.
  - Pattern must have exactly as many dot-separated segments as the dataset has dimensions (mismatches return an error listing the expected segment order).
  - Hard cap: 500 series per pattern. Beyond that, the server returns the first 500 and the CLI prints a `Warning: results truncated to first 500 matches` banner (JSON: `meta.warning.code = "result_truncated"`). Narrow the pattern.
  - No pagination here — `--page` / `--page-size` are not accepted.
- `kalistat series get <CODE> <TICKER>` — fetch observations for one series. Null values render as `—` in plain mode, `null` in JSON.

Example:
```
kalistat series list IT.LAMA.132 'M.IT.EMP.Y.9.*.9.9.CURRENT'
kalistat series get  IT.LAMA.132 M.IT.EMP.Y.9.Y15-24.9.9.CURRENT
kalistat series get  IT.LAMA.132 M.IT.EMP.Y.9.Y15-24.9.9.CURRENT --json | jq '.data.values[-1]'
```

**Recovering from "No matching series found"**: the pattern is syntactically valid but no realized series matches — usually one dimension's code is wrong, not the whole pattern. Widen one dimension at a time to `*` until results appear; the one you had to widen holds the bad code. Re-check it with `dataset values` (and watch for cross-dataset code conventions — see Gotchas).

### Error shapes

- **Plain mode**: `Error: <message>` on stderr, exit code 1.
- **JSON mode**: API body `{"error":{"code":"...","message":"..."}}` on stdout, exit code 1.

Common errors and recovery:

| Message substring | Recovery |
|---|---|
| `no API token found` | Run `kalistat auth login` |
| `token is not valid` | Re-login with a fresh token |
| `contains invalid characters` | Fix the identifier to match the regex |
| `API is unreachable` | Network issue, or wrong `--base-url` |
| `Pattern has N segment(s) but dataset has M` | Wrong dimension count in `series list` pattern; the error lists the expected segment order — copy it. |
| `No matching series found` | Pattern is valid but matches nothing. Widen one dimension at a time to `*` to locate the bad code. Common culprits: `EDITION` not set to `CURRENT`, or a `SEX`/`AGE` code that differs from a convention used elsewhere. |
| `Warning: results truncated to first 500 matches` | Pattern is too broad; the server capped results. Narrow by replacing wildcards with concrete codes — `series list` does not paginate. |
| `unknown flag: --page-size` (on `series list`) | Pagination is not supported here. Narrow the pattern. |

### Gotchas

- **`EDITION` dimension** (release-date versioning): many ISTAT monthly/quarterly datasets carry an `EDITION` dimension whose concrete values are release dates like `2026M4G1` (= 01-Apr-2026). Use the virtual alias `CURRENT` in your pattern — the server resolves it to the latest release. `PREVIOUS`, `PREVIOUS_1`, `PREVIOUS_2` resolve to earlier ones. Never leave `EDITION` as `*`: each real release is a separate series, so a wildcard multiplies your match count by dozens and almost always hits the 500-cap warning.
- **Fixed-value dimensions**: `dataset get` shows a `FIXED VALUE` column when a dimension has exactly one legal code. Copy that code into the pattern segment — `*` there is wasted.
- **Dimension code conventions vary across datasets** — never assume. `SEX` is `M/F/T` in `IT.LAMA.461` but `1/2/9` in `IT.LAMA.132`. Same concept, different codes. Re-run `dataset values <CODE> <DIM>` per dataset.
- **Sparse Cartesian product**: `dataset values` lists *allowed* codes, not codes that have realized series. This is common in ISTAT (e.g. `Y_GE15` may appear in AGE but no series exists with it); Eurostat is denser. If you pick a specific code and get `No matching series found`, try a wildcard in that dimension to see which codes are actually populated.
- **Dimension order**: API returns dimensions unsorted; the CLI sorts them by `position`. In `--json` mode, sort yourself if order matters.
- **Name truncation**: plain tables truncate long names (category ~40, dataset ~60). Use `--json` for full strings.
- **Rate limits**: see `kalistat info` (requests/minute). The CLI does not auto-retry; back off manually on 429.
- **Response body cap**: responses over 10 MiB are rejected. Paginate harder if you hit this.

## When you are unsure

- Run `kalistat <command> --help` to see live flag definitions.
- Run `kalistat info` to confirm the API is reachable and see the version / sources / rate limit.
- Prefer discovery commands (`search`, `sources`, `category tree`) over assuming codes.
