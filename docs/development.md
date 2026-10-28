# Development

Requirements: Go (version in `go.mod`). No other dependencies.

## Test

```sh
go test ./...        # detectors against testdata/, rule matching, score, ignore patterns, CLI exit codes
gofmt -l .           # must print nothing
go vet ./...
```

## Run

```sh
go run ./cmd/lockin testdata                     # all samples
go run ./cmd/lockin --format json testdata/go    # one detector's sample
go run ./cmd/lockin ~/code/some-repo
go run ./cmd/lockin --rules                      # the rule list
```

The repository has its own `.lockinignore`, so `go run ./cmd/lockin .` skips `testdata/` and `internal/` and finds
nothing.

## Fixtures

`testdata/` has one directory per source: `terraform`, `cloudformation`, `sam`, `serverless`, `python`, `js`, `cdk`,
`go`, `java`. `TestFixtures` in `internal/scan/scan_test.go` lists every hit expected from each file, with its line.
`testdata/ignored/` is excluded by `testdata/.lockinignore`; the tests check that it does not show up. Skipped
directories (`node_modules`, `vendor`, `build`, `.terraform`), large and binary files are tested in a temporary
directory (`TestPathLargeAndBinaryFiles`).

When you change a fixture or a rule, the fixture score can change. Update `testdataScore` in
`cmd/lockin/main_test.go` and the two examples in the README.

## Where to change things

| Change | Files |
|---|---|
| A service, its class or its replacement | `internal/rules/rules.json`, a case in `internal/rules/rules_test.go` ([rules.md](rules.md)) |
| A new SDK, language or file type | `internal/scan` (`code.go` or a new file, and `detectorsFor` in `scan.go`), a fixture, `docs/architecture.md` |
| Score or output wording | `internal/report` |
| Flags | `cmd/lockin/main.go`, the README usage table |
| GitHub Action | `action.yml`, the README action section |

Keep it static and offline: no network calls, no cloud SDKs, no reading credentials.
