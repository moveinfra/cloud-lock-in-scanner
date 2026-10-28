## What changes for users

<!-- One or two sentences. For rule changes: which services, and why the class fits (link the provider docs). -->

## Checks

- [ ] `go test ./...` and `go vet ./...` pass, `gofmt -l .` is empty
- [ ] New behaviour has a test (rule case in `internal/rules/rules_test.go`, fixture or `TestSnippets` case)
- [ ] EU replacements use product names from the EU Cloud Map
- [ ] If the fixture score changed: `testdataScore` and the README examples are updated
- [ ] Still static and offline
