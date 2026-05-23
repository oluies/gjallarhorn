# Contract: CI workflow (`.github/workflows/ci.yml`)

**Owner**: Gjallarhorn
**Stability**: contract is stable from Phase A; new jobs can be added but the existing gates cannot be relaxed without paired-PR coordination.

## Triggers

```yaml
on:
  push:           # every push to any branch
  pull_request:   # every PR
```

Rationale matches Neverlur's CI: cross-repo dev needs CI signal on feature branches before PR review.

## Job: `build & test`

**Runner**: `ubuntu-latest` (linux/amd64; required for bn256 to build).

**Go version matrix**: `['1.25']`.

**Steps** (in order):

| Step | Behavior | Failure mode |
|---|---|---|
| `actions/checkout@v4` | check out the repo | hard fail |
| `actions/setup-go@v5` | install Go with module cache | hard fail |
| `go mod download` | resolve all module deps | hard fail (catches go.mod tampering) |
| `gofmt -l .` check | empty output required | hard fail (with the offending files in the error message) |
| `go build ./...` | full-module compile | hard fail |
| `go vet ./...` | with `continue-on-error: true` | soft fail (existing inherited Vuvuzela vet warnings) |
| `go test (unit)` | `go test -race -count=1 -timeout 5m $(go list ./... | grep -v '/e2e$')` | hard fail |
| `go test (e2e)` | `go test -race -count=1 -timeout 5m ./e2e/...` | hard fail |

The unit/e2e split lets unit failures surface before the longer-running e2e step.

## Job: `golangci-lint` (Phase A NOT required, deferred to follow-up)

Phase A defers `golangci-lint` setup. The Neverlur side has it; Gjallarhorn matches in a follow-up cleanup PR (lint findings will mostly be inherited Vuvuzela patterns and need a scoped lint config similar to Neverlur's `.golangci.yml`).

## What this commit does NOT add

- No deployment/release workflow.
- No fuzz job (the integration test doesn't need one yet).
- No coverage reporting (out of scope).
- No matrix expansion (Go 1.25 only; we don't yet test against 1.24 or main).

## Cross-repo signal flow

```
Neverlur PR opens   ───> Neverlur CI runs (unit + integration)
Gjallarhorn PR opens ──> Gjallarhorn CI runs (unit + e2e)
Either repo's master  ──> both repos' CI eventually picks up the
                            change via go.mod pin / temporary replace
```

For paired PRs where Gjallarhorn changes need to validate against an un-merged Neverlur PR, the Gjallarhorn PR temporarily contains:

```text
replace github.com/oluies/neverlur => github.com/oluies/neverlur@<commit-sha>
```

This is removed before merging the Gjallarhorn PR; the Neverlur PR merges first, the Gjallarhorn PR then `go get`s the merged commit.

## Future work (post Phase A)

- Lint job (`.golangci.yml` + workflow) parity with Neverlur.
- Benchmark smoke job similar to Neverlur's `Benchmark smoke (PQ foundations)`.
- A `Coverage` job that uploads to codecov or similar.
