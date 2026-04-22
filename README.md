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

Override it with an environment variable:

```bash
export KALISTAT_API_URL=https://app.kalistat.com/api/v1
```

This is useful for local development or testing against another environment.

## Output

Commands are designed to work well both for humans and for scripts.

Human-readable output is the default. For machine-readable output, pass `--json`:

```bash
kalistat info --json
kalistat sources --json
```

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
├── go.mod
├── go.sum
└── main.go
```

- `cmd/` contains Cobra commands
- `internal/` contains API, keychain, and supporting logic

## Status

This is an early version of the CLI.

Current focus:
- authentication
- API info
- sources

Planned next:
- more endpoint coverage
- richer output formatting
- shell completion
- release packaging

## License

MIT
