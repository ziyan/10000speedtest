# Contributing to 10000speedtest

## Development setup

```bash
git clone https://github.com/ziyan/10000speedtest.git
cd 10000speedtest
make build
make test
```

Note that the speed-test servers are only reachable from inside the China
Telecom Guangdong network, so a full end-to-end run must be done from a host on
that network. Unit tests use a local mock server and run anywhere.

### Requirements

- Go 1.25+
- golangci-lint
- mulint (optional; `make lint` runs it when it is on your `PATH`)

### Build commands

```bash
make build      # build the binary
make test       # run tests
make lint       # run golangci-lint (and mulint if installed)
make format     # run gofmt and goimports
make vendor     # tidy and vendor dependencies
make clean      # remove build artifacts
```

## Code conventions

This project uses a modified naming convention that differs from standard Go.
All contributors must follow these rules.

### Acronym casing

When the **first alphabetical character is capitalized**, capitalize the entire
acronym; when it is **lowercase**, capitalize only the first letter:

```go
// Correct
type JSONOutput bool
var serverURL string
downloadSize := 20
megabitsPerSecond := 0.0

// Wrong
type JsonOutput bool
var serverUrl string
```

### No abbreviations

Spell out names in full. Package names are the only exception (keep them brief).

```go
// Correct
command, response, request, connection, duration

// Wrong
cmd, resp, req, conn, dur
```

### Variables and receivers

- Errors are named `err`.
- Avoid single-letter variables except in very short range loops.
- Use `self` for struct method receivers.

```go
func (self *Tester) Download() Result { ... }
```

## Project structure

```
main.go                 # entrypoint: version/commit vars + cli.Run
internal/
  cli/                  # urfave/cli v3 wiring: flags, action, JSON output
  speedtest/            # test engine: parallel workers + live renderers
  logging/              # go-logging setup
  deferutil/            # deferred panic recovery for goroutines
vendor/                 # vendored dependencies
```

The `speedtest` package holds the measurement engine and a `renderer` interface
with four variants — chart, line, plain, and silent — chosen from the `--json`
flag, terminal detection, and the `--chart` flag.

## Dependencies

All dependencies are vendored. After changing `go.mod`:

```bash
go mod tidy
go mod vendor
```

Always build and test with `-mod=vendor` (the Makefile does this for you).

## Linting

`make lint` runs golangci-lint and, when installed, mulint. The `.golangci.yml`
suppresses the staticcheck rules that conflict with the naming convention above
(ST1000, ST1003, ST1006). Run it before submitting a pull request.

## Testing

- Unit-test pure functions and rendering logic (no network needed).
- Worker tests run against an in-process mock HTTP server.
- Run `go test -race ./...` before submitting changes.

## Releasing

Releases are built by the **Release** GitHub Actions workflow, which runs only
when a version tag is pushed — pushing to `main` alone does **not** create a
release. To cut one:

1. Update `CHANGELOG.md` with a section for the new version (for example
   `## [0.3.0]`); the workflow pulls the release notes from it.
2. Tag the commit and push the tag:

   ```bash
   git tag v0.3.0
   git push origin v0.3.0
   ```

The workflow cross-compiles binaries for linux, darwin, and windows
(amd64/arm64), packages each with the README, CHANGELOG, and LICENSE, generates
a `SHA256SUMS` file, and publishes a GitHub Release.

## Commit messages

- Use imperative mood: "Add feature", not "Added feature".
- First line: concise summary under 72 characters.
- Body: explain what and why, not how.
