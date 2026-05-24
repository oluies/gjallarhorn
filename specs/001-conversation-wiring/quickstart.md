# Quickstart: Conversation Wiring (Gjallarhorn side, Phase A)

**Feature**: 001-conversation-wiring (Gjallarhorn)
**Companion**: [neverlur/specs/002-conversation-wiring/quickstart.md](https://github.com/oluies/neverlur/blob/master/specs/002-conversation-wiring/quickstart.md)
**Audience**: a developer or demonstrator who wants to verify the Gjallarhorn side of Phase A locally.

## Prerequisites

- Go 1.25+
- Linux/amd64 OR macOS/arm64 (with the bn256 limitation: full `go build` / `go test` only on linux/amd64).
- Both repos cloned as siblings:
  ```
  ~/projects/
  ├── neverlur/
  └── gjallarhorn/      (this repo)
  ```
- `~/projects/go.work` per the documented convention.

## 1. Verify the rebrand is finished

```sh
$ cd ~/projects/gjallarhorn
$ grep -rn '\balpenhorn\.' cmd/gjallarhorn-client/
# Expected: NO matches.
```

If any `alpenhorn.X` references remain, the Phase A rename patch (Gjallarhorn-side T-rebrand-fix) has not been applied yet.

## 2. Verify the go.mod cleanup

```sh
$ grep 'vuvuzela.io/alpenhorn' go.mod
# Expected: NO matches (the dead replace was removed).
```

## 3. Build the module

```sh
$ go build ./...
# Linux/amd64: expected to succeed.
# macOS/arm64: expected to fail with bn256 .s assembly errors — that's pre-existing
# and not Phase A's problem to solve.
```

## 4. Run the unit tests

```sh
$ go test -race -count=1 -timeout 5m $(go list ./... | grep -v '/e2e$')
ok  	github.com/oluies/gjallarhorn/convo
ok  	github.com/oluies/gjallarhorn/coordinator
ok  	github.com/oluies/gjallarhorn/mixnet
...
```

## 5. Run the e2e integration test

```sh
$ go test -race -count=1 -timeout 5m ./e2e/...
=== RUN   TestE2EFirstMessage
--- PASS: TestE2EFirstMessage (28.4s)
=== RUN   TestE2EFirstMessage_HybridConfidentiality_ClassicalCompromise
--- PASS: TestE2EFirstMessage_HybridConfidentiality_ClassicalCompromise (29.1s)
=== RUN   TestE2EFirstMessage_HybridConfidentiality_PQCompromise
--- PASS: TestE2EFirstMessage_HybridConfidentiality_PQCompromise (28.9s)
=== RUN   TestNoClassicalSessionKeySource
--- PASS: TestNoClassicalSessionKeySource (0.42s)
PASS
ok  	github.com/oluies/gjallarhorn/e2e	87.0s
```

On arm64:

```sh
$ go test ./e2e/...
--- SKIP: TestE2EFirstMessage (0.00s)
    skipped on arm64: vuvuzela.io/crypto/bn256 ships x86_64-only assembly
--- SKIP: TestE2EFirstMessage_HybridConfidentiality_ClassicalCompromise (0.00s)
    skipped on arm64: ...
--- SKIP: TestE2EFirstMessage_HybridConfidentiality_PQCompromise (0.00s)
    skipped on arm64: ...
--- PASS: TestNoClassicalSessionKeySource (0.42s)
PASS
ok  	github.com/oluies/gjallarhorn/e2e	0.42s
```

`TestNoClassicalSessionKeySource` runs on arm64 because it's a pure-AST static check.

## 6. Run the Neverlur-side demo CLI against this harness

See `neverlur/specs/002-conversation-wiring/quickstart.md` step 4 for the two-terminal demo. The harness this Gjallarhorn-side repo exports is what that demo wraps.

## 7. CI status

Once the Phase A CI workflow lands, every push triggers:
- `gofmt -l .` (silent required)
- `go build ./...`
- `go vet ./...` (continue-on-error)
- Unit tests (race-enabled, 5min timeout)
- e2e tests (race-enabled, 5min timeout)

Local equivalent:

```sh
$ gofmt -l . && go build ./... && go vet ./...; \
    go test -race -count=1 $(go list ./... | grep -v '/e2e$') && \
    go test -race -count=1 ./e2e/...
```

## Troubleshooting

| Symptom | Likely cause | Fix |
|---|---|---|
| `undefined: alpenhorn` in `cmd/gjallarhorn-client/alpenhorn.go` | rebrand-fix patch not applied | Apply Phase A rename: `alpenhorn.X` → `neverlur.X` in that file |
| `conflicting replacements for vuvuzela.io/alpenhorn` | dead replace not removed from go.mod | Remove the line `vuvuzela.io/alpenhorn => github.com/vuvuzela/alpenhorn ...` from `go.mod` |
| `TestNoClassicalSessionKeySource` fails | a new commit added `box.Precompute` or `curve25519.X25519` in the conversation path | Remove the call; use `keywheel.Wheel.SessionKey` instead |
| `TestE2EFirstMessage` times out | likely a deadlock in `AdvanceRound` due to round desynchronization | Re-run with `go test -race -v` to surface goroutine traces; check whether the test goroutine is waiting on a receive that the harness should produce |
| `Alice session key != Bob session key` | regression in Neverlur's `Wheel.SessionKey` or in the call-bootstrap path | Re-run `go test` on `keywheel` and `hybrid` in Neverlur; check whether `TestKeywheelHybridSeed` still passes there |
