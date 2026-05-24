# Phase 0 Research: Conversation Wiring (Gjallarhorn side, Phase A companion)

**Feature**: 001-conversation-wiring
**Date**: 2026-05-23

This document resolves every open technical question raised by the plan's Technical Context section. The Neverlur-side research.md (R1–R10) is authoritative for cross-cutting questions; this document covers the Gjallarhorn-specific decisions.

---

## R1. Why the cmd/gjallarhorn-client/alpenhorn.go rebrand was missed (and how to finish it cleanly)

**Finding**: When the rebrand commit (`50839a7` "all: rebrand to gjallarhorn") renamed Vuvuzela → Gjallarhorn across the codebase, the file `cmd/gjallarhorn-client/alpenhorn.go` had its imports updated (`"vuvuzela.io/alpenhorn"` → `"github.com/oluies/neverlur"`) but the 12 identifiers using `alpenhorn.X` were not renamed to `neverlur.X`. The file still defines GuiClient methods like:

```go
func (gc *GuiClient) ConfirmedFriend(f *alpenhorn.Friend) { ... }
func (gc *GuiClient) SentFriendRequest(r *alpenhorn.OutgoingFriendRequest) { ... }
```

Locally on arm64 this didn't surface as a build break because the `vuvuzela.io/crypto/bn256` assembly failure happens earlier in the build pipeline. Gjallarhorn has no CI, so it didn't surface there either.

**Decision**: Apply the rename mechanically — `alpenhorn.X` → `neverlur.X` everywhere in `cmd/gjallarhorn-client/alpenhorn.go`. Don't add an import alias (`alpenhorn "github.com/oluies/neverlur"`); the file's name is incidental and the rename reads cleaner. Confirm the rename by `git grep "alpenhorn\." cmd/gjallarhorn-client/` returning empty afterwards.

**Rationale**: Cleaner long-term answer. Future readers won't be confused about why a Gjallarhorn file uses `alpenhorn` symbols.

**Alternatives considered**:
- Import alias (`alpenhorn "github.com/oluies/neverlur"`): one-line patch, but leaves the file as a permanent reminder that "this used to be alpenhorn".
- Rename the file from `alpenhorn.go` to `neverlur.go`: pure aesthetics; not in scope unless adjacent rename work is happening.

---

## R2. Why the dead `replace vuvuzela.io/alpenhorn` directive in `go.mod` is safe to remove

**Finding**: Gjallarhorn's `go.mod` has:

```text
replace (
    vuvuzela.io/alpenhorn => github.com/vuvuzela/alpenhorn v0.0.0-20190912152808-6b33518f681e
    vuvuzela.io/concurrency => github.com/vuvuzela/concurrency v0.0.0-20190327123758-e608f351e310
    vuvuzela.io/crypto => github.com/vuvuzela/crypto v0.0.0-20220523120157-1709ed3a3b66
    vuvuzela.io/internal => github.com/vuvuzela/internal v0.0.0-20190910144301-7321cf92c8ba
    vuvuzela.io/vuvuzela => github.com/vuvuzela/vuvuzela v0.0.0-20190912153956-55ba49f81ad0
)
```

`git grep "vuvuzela.io/alpenhorn"` across the Gjallarhorn source tree returns zero hits — no Go file imports it; no module manifest requires it transitively. The `replace` line for `vuvuzela.io/alpenhorn` is dead.

When the Neverlur and Gjallarhorn modules are joined in a Go workspace, Neverlur's own `replace vuvuzela.io/alpenhorn => ./` (Neverlur is itself the canonical home for what `vuvuzela.io/alpenhorn` used to be) conflicts with Gjallarhorn's dead replace. `go build` errors:

```text
go: conflicting replacements for vuvuzela.io/alpenhorn:
    /Users/.../neverlur
    github.com/vuvuzela/alpenhorn@v0.0.0-20190912152808-6b33518f681e
use "go work edit -replace ..." to resolve
```

**Decision**: Remove only the `vuvuzela.io/alpenhorn` line from Gjallarhorn's `replace` block. Leave the other four replaces (`concurrency`, `crypto`, `internal`, `vuvuzela`) alone — they're still load-bearing for the inherited Vuvuzela code that hasn't been re-rooted under `github.com/oluies/...` yet.

**Rationale**: Minimal surgical change. The four remaining replaces don't conflict with Neverlur's go.mod (Neverlur has the same four replaces). Phase A doesn't need to migrate the other four.

---

## R3. What does the in-process test harness need to stand up?

**Finding**: Looking at `cmd/gjallarhorn-coordinator/main.go`, `cmd/gjallarhorn-mixer/main.go`, and `cmd/neverlur-coordinator/main.go` (which exists as the Neverlur PKG/coordinator entry), a real deployment runs five processes:

1. Neverlur coordinator (for add-friend rounds + dialing rounds)
2. 3 Neverlur mixers (for add-friend + dialing onion routing)
3. Neverlur PKG (for identity attestation)
4. Neverlur CDN (for mailbox storage)
5. Gjallarhorn coordinator + 3 mixers + CDN (for conversation rounds)

For the in-process harness, the minimum viable set is the same components running in goroutines on local sockets, with signed configs the harness itself produces and signs (using a one-shot guardian identity created at harness `New()`).

**Decision**: The harness exposes:

```go
type Harness struct {
    NeverlurCoordinator  *coordinator.Server
    NeverlurMixers       []*mixnet.MixServer
    NeverlurPKG          *pkg.Server
    NeverlurCDN          *cdn.Server
    GjallarhornCoordinator *coordinator.Server  // distinct from Neverlur's
    GjallarhornMixers    []*mixnet.MixServer
    GjallarhornCDN       *cdn.Server
    AddFriendConfig      *config.SignedConfig
    DialingConfig        *config.SignedConfig
    ConvoConfig          *config.SignedConfig
    HarnessGuardian      *hybrid.HybridIdentity  // signs all configs
    ListenAddr           string                   // Unix socket the demo CLI connects to
}
```

**Rationale**: Mirror real deployment topology so the harness exercises every code path. The single `HarnessGuardian` signs all three configs; for tests we don't need realistic multi-guardian setups.

**Alternatives considered**:
- One coordinator instance serving both add-friend and convo: rejected because the production code paths assume two distinct coordinators with distinct configs.
- No PKG (mock the registration step): rejected because the friend-discovery flow tests fail without a real PKG.

---

## R4. What exactly does `TestE2EFirstMessage` assert?

**Decision**: The test has a `main` body plus two subtests:

**Main body** (`TestE2EFirstMessage`):

1. Skip on `runtime.GOARCH == "arm64"` with the standard message.
2. `h := testharness.New(t); defer h.Close()`
3. `alice := h.ClientFor(t, "alice@demo.local")`
4. `bob := h.ClientFor(t, "bob@demo.local")`
5. Both register with the PKG.
6. `alice.SendFriendRequest("bob@demo.local", nil)`.
7. `h.AdvanceRound("AddFriend")` (one add-friend round to deliver the intro).
8. Bob's `ReceivedFriendRequest` handler fires; capture the `IncomingFriendRequest`.
9. `bob.IncomingFriendRequest.Approve()`.
10. `h.AdvanceRound("AddFriend")` again (Bob's confirmation goes back).
11. Both clients now have wheel entries for each other.
12. `alice.SendCall(bob, /*intent*/ 0)`; this enters a dialing round.
13. `h.AdvanceRound("Dialing")`; both clients now have an active `Conversation` with matching `sessionKey`.
14. `alice.Conversation.Seal([]byte("hello"), round, key)`; the packet enters one Gjallarhorn convo round.
15. `h.AdvanceRound("Convo")`.
16. Assert `bob.received == []byte("hello")`.

**Subtest** (`TestE2EFirstMessage_HybridConfidentiality_ClassicalCompromise`):

Same setup through step 11. Then:

a. Capture `(aliceX25519Priv, bobX25519Pub, aliceMLKEMPriv, bobMLKEMCT)` from Alice's friend-discovery state (test-only accessor on `neverlur.Client`).
b. Compute `ssX25519 := curve25519.X25519(aliceX25519Priv[:], bobX25519Pub[:])` — the classical half only.
c. Pretend ML-KEM was broken: try to derive the wheel seed using only `ssX25519` and known transcript bytes. Compute `seedAttacker := hybrid.CombineKEMConcat(ContextKeywheelSeed, transcript, ssX25519, allZeros64)`.
d. Compute the real seed: `seedReal := hybrid.CombineKEMConcat(ContextKeywheelSeed, transcript, ssX25519, realSSMLKEMConcat)`.
e. Assert `seedAttacker != seedReal`.

**Subtest** (`TestE2EFirstMessage_HybridConfidentiality_PQCompromise`): symmetric (zero out ssX25519, keep ssMLKEM).

**Rationale**: Direct test-level satisfaction of spec SC-001 (round-trip), SC-002 (classical compromise), SC-003 (PQ compromise).

---

## R5. How does the static-check test (`TestNoClassicalSessionKeySource`) work?

**Decision**: A Go test that walks the source tree using `go/ast` (NOT `os/exec grep` — the AST walk is portable and produces structured diagnostics):

```go
func TestNoClassicalSessionKeySource(t *testing.T) {
    forbiddenPatterns := []string{
        `box.Precompute`,
        `curve25519.X25519`,
        `nacl/box.Precompute`,
    }
    allowedFiles := []string{
        // Files where the call is part of the hybrid combiner itself
        // (these live in neverlur/keywheel_seed.go, NOT in gjallarhorn).
    }
    // Walk gjallarhorn/convo/, gjallarhorn/cmd/gjallarhorn-client/,
    // gjallarhorn/coordinator/, gjallarhorn/mixnet/ for any of the
    // forbidden patterns. Fail if any are found outside allowedFiles.
}
```

The test lives in `e2e/no_classical_session_key_test.go` and is the load-bearing enforcement of spec SC-005 ("zero occurrences of session key from non-hybrid source").

**Rationale**: A real Go test is reviewable, runnable locally, and diagnoses precisely the file:line of the violation. A shell `grep` would have the same coverage but worse diagnostics.

**Alternatives considered**:
- `staticcheck` custom analyzer: heavier; would require a separate tool installation. The AST walk is in-process and uses only stdlib.
- `grep` in a CI step: less testable; a developer with no CI access can't reproduce.

---

## R6. What does Gjallarhorn's CI workflow look like?

**Decision**: A new `.github/workflows/ci.yml` modeled on Neverlur's existing one:

```yaml
name: CI

on:
  push:
  pull_request:

jobs:
  build:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        go: ['1.25']
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '${{ matrix.go }}', cache: true }
      - name: Verify modules
        run: go mod download
      - name: Check gofmt
        run: |
          out=$(gofmt -l .)
          if [ -n "$out" ]; then echo "::error::gofmt: $out"; exit 1; fi
      - name: Build
        run: go build ./...
      - name: Vet
        continue-on-error: true
        run: go vet ./...
      - name: Test (unit)
        run: go test -race -count=1 -timeout 5m $(go list ./... | grep -v '/e2e$')
      - name: Test (e2e)
        run: go test -race -count=1 -timeout 5m ./e2e/...
```

The `e2e` step is the gate that exercises the Phase A integration test.

**Rationale**: Mirrors Neverlur's existing CI structure for consistency. The split between "unit" and "e2e" lets unit failures surface first; e2e failures are still surfaced but after unit.

---

## R7. CI ↔ workspace handoff: how does the e2e test resolve `github.com/oluies/neverlur`?

**Decision**: In CI, Gjallarhorn's `go.mod` pins a specific Neverlur version (`require github.com/oluies/neverlur vX.Y.Z`). The CI workflow does NOT use `go.work`; it uses standard module resolution. For paired-PR development (where Gjallarhorn changes need to test against an un-merged Neverlur PR), the Gjallarhorn PR temporarily includes `replace github.com/oluies/neverlur => github.com/oluies/neverlur@<commit-sha>` and removes it before merging.

**Rationale**: The Go workspace is a per-developer convenience; CI uses pinned module versions. This is the established Go pattern for cross-module dev that lands as paired PRs.

(Same finding as Neverlur-side R6.)

---

## R8. What test-only accessors does `neverlur.Client` need?

**Finding**: Subtest in R4 needs to extract the X25519 / ML-KEM private/public material from Alice's friend-discovery state to simulate the classical-half and PQ-half compromises. The current `neverlur.Client` doesn't expose this — `sentFriendRequest.DHPrivateKey` and `sentFriendRequest.MLKEMPrivateKey` are unexported fields.

**Decision**: Add a test-only accessor on the **Neverlur side** (not Gjallarhorn) — `func (c *neverlur.Client) FriendDiscoveryStateForTest(peer string) *FriendDiscoveryStateForTest` in a `_test.go`-style file, OR via a Go build tag `//go:build testing`. The accessor returns a struct with the captured key material for one friendship.

This is a Neverlur-side dependency for Gjallarhorn's e2e test. Document as a cross-repo todo in the Gjallarhorn plan and as a Neverlur-side follow-up issue.

**Alternatives considered**:
- Reflect into the private fields from the Gjallarhorn-side test: works but ugly and brittle.
- Add the accessor in Neverlur production code: undesirable because it leaks private key material.
- Make the carry-through test less direct (e.g. just assert the seeds at the wheel boundary differ): weaker test but acceptable.

The third option is the easiest to land without coordinating a Neverlur-side change. Phase A's first implementation pass uses option 3; the deeper accessor-based test is a follow-up.

---

## R9. How is the "no classical session key source" property enforced for inherited code paths?

**Finding**: Gjallarhorn's `convo/` package and the inherited `vuvuzela.io/vuvuzela/convo` package both use `nacl/secretbox` (via the rolled per-round key), NOT `nacl/box`. So the forbidden-patterns list in R5 doesn't actually fire on existing code. Good.

But: the conversation BOOTSTRAP currently calls `alpenhornClient.OutgoingCall.SessionKey()`. That call's return value is the wheel's session key, which is hybrid-derived post-Neverlur-PR-#4. So the conversation key IS hybrid-derived already, transitively.

**Decision**: The static-check test enforces the rule "no NEW classical-only call site" and the existing `OutgoingCall.SessionKey()` is allowlisted because it transitively goes through `hybrid.CombineKEM`. The allowlist in `e2e/no_classical_session_key_test.go` documents this with a paragraph explaining the transitive chain.

**Rationale**: The constitution forbids silent fallback; it does not forbid every appearance of the word `X25519`. The test is structured to catch regressions where someone adds a NEW classical-only call.

---

## R10. What's the rollback strategy if Phase A regresses the existing classical-keyed conversation tests?

**Decision**: Phase A makes no changes to the conversation code path except for the rebrand-rename in `cmd/gjallarhorn-client/alpenhorn.go`. If the rename introduces a regression detectable by the existing `convo/config_test.go` and `mixnet/mixnet_test.go`, the rebrand patch is the rollback target.

If the e2e test fails for cross-repo reasons (Neverlur changed something), the Gjallarhorn-side fix is either:
- Pin to an older Neverlur version and reopen the integration as a separate change, OR
- Roll forward — fix the Neverlur side to restore the contract.

The constitution's Compatibility section ("paired PRs referencing each other") makes the latter the preferred path.

---

## Open items

None. All Technical Context unknowns resolved.
