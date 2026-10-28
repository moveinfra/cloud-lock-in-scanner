# Contributing

Thanks for helping. Missing services, wrong classes, missed detections, new SDKs and docs fixes are all welcome.

## Before you start

- **Small fix** (typo, docs, a clear bug, a new or corrected rule): open a pull request directly.
- **Anything bigger** (a new detector, a new flag, a change to the score): open an issue first so we can agree on
  the approach.
- **Security issue**: do not open a public issue; see [SECURITY.md](SECURITY.md).

## Setup

```sh
git clone https://github.com/moveinfra/cloud-lock-in-scanner
cd cloud-lock-in-scanner
go test ./...
```

More in [docs/development.md](docs/development.md). Rules are explained in [docs/rules.md](docs/rules.md).

## Pull requests

- One topic per pull request, with a short description of what changes for users.
- `go test ./...` and `go vet ./...` pass; `gofmt -l .` prints nothing. CI checks all of this.
- New behaviour has a test. A new rule gets a case in `internal/rules/rules_test.go`; a new detector gets a fixture
  under `testdata/` or a case in `TestSnippets`.
- A rule change links the provider's documentation and says why the class fits.
- EU replacements use product names from the [EU Cloud Map](https://github.com/moveinfra/eu-cloud-map). If a product
  is missing there, propose it there first.

## Ground rules for the tool

1. **Static and offline.** It reads files. No cloud APIs, no network calls, no telemetry.
2. **Nothing dropped silently.** A service it does not know is reported as unknown, not skipped.
3. **Plain findings.** File and line for every use, a short replacement, no marketing.

## Commit messages

Short imperative subject (`Add Azure Event Grid rule`), a body when the reason is not obvious.

## License

By contributing you agree that your contribution is licensed under the [Apache License 2.0](LICENSE).
