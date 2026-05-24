---

description: "Task list for 001-conversation-wiring (Gjallarhorn side, Phase A companion)"
---

# Tasks: Conversation Wiring (Gjallarhorn side, Phase A companion)

**Input**: Design documents in `specs/001-conversation-wiring/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: REQUIRED. The constitutional Principle III ("Hybrid, Never Pure — non-negotiable") demands the no-classical-fallback property be enforced in CI. Phase A delivers two real tests + one static-check test on the Gjallarhorn side:
- `TestE2EFirstMessage` (round-trip)
- `TestE2EFirstMessage_HybridConfidentiality_{Classical,PQ}Compromise` (carry-through)
- `TestNoClassicalSessionKeySource` (AST static check)

**Organization**: Tasks are grouped by user story per spec.md priorities. Phase A's Gjallarhorn side owns the **majority of the actual implementation work** (the Neverlur side is mostly a demo CLI).

## Format: `[ID] [P?] [Story?] Description with file path`

- **[P]**: Can run in parallel (different files, no dependencies on incomplete tasks)
- **[Story]**: US1 / US2 / US3 / US4 — maps to spec.md user stories
- All paths repository-root-relative

## Path Conventions

Single Go module at the repository root. Phase A additions: `internal/testharness/`, `e2e/`, `.github/workflows/`. Existing edits: `cmd/gjallarhorn-client/alpenhorn.go`, `go.mod`.

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Make the repo buildable in the cross-repo workspace, finish the rebrand, and stand up CI so Phase A's tests have somewhere to run.

- [X] T001 Remove the dead `replace vuvuzela.io/alpenhorn => github.com/vuvuzela/alpenhorn ...` line from `go.mod`. Keep the other four `vuvuzela.io/*` replaces (they're still load-bearing for inherited Vuvuzela code). Run `go mod tidy` to confirm no transitive importer surfaces.
- [X] T002 Rebrand-finish in `cmd/gjallarhorn-client/alpenhorn.go`: rename every `alpenhorn.X` identifier to `neverlur.X`. 12 sites per `research.md` R1. After: `git grep '\balpenhorn\.' cmd/gjallarhorn-client/` returns empty.
- [X] T003 Create `.github/workflows/ci.yml` per `contracts/ci-workflow.md` — `gofmt`, `go build`, `go vet` (continue-on-error), `go test (unit)`, `go test (e2e)` steps on `ubuntu-latest` with Go 1.25. Fires on every push and every pull request.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Build the in-process test harness package that Phase A's user stories all rely on.

**⚠️ CRITICAL**: T004–T008 below MUST land before any of Phase 3+ tasks. They are the load-bearing infrastructure.

### Harness package skeleton

- [ ] T004 Create the package directory `internal/testharness/` with a `doc.go` file describing the package's purpose (per `data-model.md` E1 / `contracts/test-harness-api.md`) and the constitutional note that the package is test-only despite being callable from the Neverlur demo CLI.
- [ ] T005 [P] Implement `internal/testharness/harness.go` with the exported `Harness` struct, `Option` type, `OptionListenAddr`, `OptionMixerCount` per `contracts/test-harness-api.md`. `New(tb, opts...)` orchestrates the per-component startup in goroutines; registers `tb.Cleanup` for full teardown. `Close()` is idempotent.
- [ ] T006 [P] Implement `internal/testharness/coordinator.go` — starts both `NeverlurCoordinator` and `GjallarhornCoordinator` on local sockets. Wires them with the bootstrap configs signed by `HarnessGuardian`.
- [ ] T007 [P] Implement `internal/testharness/mixers.go` — starts N Neverlur + N Gjallarhorn mixers (default 3 each) in goroutines.
- [ ] T008 [P] Implement `internal/testharness/pkg.go` — starts the Neverlur PKG, wires it with `BadgerDB`-in-tempdir storage, registers handler for client registration calls. Uses `tb.TempDir()` for the PKG database.
- [ ] T009 Implement `internal/testharness/client.go` — exports `ClientFor(tb, username) *TestClient`. Generates a fresh `hybrid.HybridIdentity` per call; registers with the PKG; wires `EventHandler` callbacks that push into the `RecvCh` channel. Also defines `TestClient`, `IncomingMessage`, and `ConvoStateAccessor` types per `contracts/test-harness-api.md`.
- [ ] T010 Implement `internal/testharness/rounds.go` — exports `AdvanceRound(service string)`. Drives the named coordinator's next round to completion in a deterministic synchronous step. Safe only from a single test goroutine.
- [ ] T011 [P] Implement `internal/testharness/configs.go` — generates the three v2 `SignedConfig` records (AddFriend, Dialing, Convo) at harness `New()` time, signed by the harness guardian. Each lists the harness guardian as its sole `Guardian` with the R4-bound `PQKey` populated.

**Checkpoint**: Foundational complete. `go build ./internal/testharness/...` succeeds; the harness package compiles and is ready for use by user-story phases.

---

## Phase 3: User Story 1 - Gjallarhorn delivers a friended pair's first message (Priority: P1) 🎯 MVP

**Goal**: Two friended clients exchange one conversation packet end-to-end across the in-process harness, with byte-identical session keys derived from the hybrid keywheel.

**Independent Test**: `go test -run TestE2EFirstMessage ./e2e/...` (linux/amd64) passes.

### Tests for User Story 1

- [ ] T012 [P] [US1] Write `e2e/first_message_test.go::TestE2EFirstMessage` per `contracts/e2e-test-cases.md`. Skips on `runtime.GOARCH == "arm64"` with the standard message. Stands up the harness, creates Alice + Bob, runs them through add-friend (2 rounds), dialing (1 round), and convo (1 round), asserts Bob received Alice's "hello bob" bytes intact and that Alice's session key equals Bob's session key.
- [ ] T013 [US1] Wire the test client to expose `IncomingFriendRequestCh()` (a buffered channel that captures `*neverlur.IncomingFriendRequest` notifications) and `RecvCh` (already declared in `TestClient`); both feed off the existing `EventHandler` callbacks. Add to `internal/testharness/client.go`.
- [ ] T014 [US1] Wire `TestClient.SendMessage(body []byte) error` as a thin helper around the existing `Conversation.Seal` + onion-submit path. Validates length and UTF-8 at the boundary so the integration test reuses the same input-validation surface the demo CLI does.

**Checkpoint**: US1 complete. The Phase A foundational property is delivered and asserted in CI.

---

## Phase 4: User Story 2 - Conversation session-key derivation rejects non-hybrid sources (Priority: P1)

**Goal**: Constitutionally enforce that no classical-only session-key derivation path exists in the Gjallarhorn conversation code (post-PQ-cutover friendships).

**Independent Test**: `go test -run TestNoClassicalSessionKeySource ./e2e/...` passes on ANY platform (no bn256 dependency).

### Tests for User Story 2

- [ ] T015 [P] [US2] Write `e2e/no_classical_session_key_test.go::TestNoClassicalSessionKeySource` per `contracts/e2e-test-cases.md`. AST walk over `convo/`, `cmd/gjallarhorn-client/`, `coordinator/`, `mixnet/` looking for forbidden patterns (`box.Precompute`, `curve25519.X25519`, `nacl/box.Precompute`). Reports per-hit diagnostics; empty allowlist for Phase A.
- [ ] T016 [P] [US2] Write `e2e/first_message_test.go::TestE2EFirstMessage_HybridConfidentiality_ClassicalCompromise` per `contracts/e2e-test-cases.md`. After the round-trip completes, simulate a classical-half compromise: derive a would-be session key with only the X25519 component (PQ component zeroed); assert it differs from the real session key.
- [ ] T017 [P] [US2] Write `e2e/first_message_test.go::TestE2EFirstMessage_HybridConfidentiality_PQCompromise` (symmetric).

### Helper for Phase A compromise simulation

- [ ] T018 [US2] Implement the per-friendship classical/PQ secret extraction helpers `extractClassicalShare(tb, *TestClient, peer) []byte` and `extractTranscript(tb, *TestClient, peer) []byte` in `internal/testharness/test_only_accessors.go`. Per `research.md` R8 first-attempt: these scrape state from Gjallarhorn's existing `keywheelStart`-equivalent accessors. If the existing accessors are insufficient, document a follow-up Neverlur-side accessor request and use a coarser "session key bytes differ" assertion as the Phase A fallback.

**Checkpoint**: US2 complete. The hybrid carry-through property is enforced by both a structural check (T015) and two simulation tests (T016, T017).

---

## Phase 5: User Story 3 - Phase A integration test runs in CI on both repos (Priority: P2)

**Goal**: Wire the e2e tests into the Gjallarhorn CI so regressions in either Neverlur or Gjallarhorn session-key derivation surface on every PR.

**Independent Test**: A green CI run on a Gjallarhorn PR includes the `Test (e2e)` step passing.

### Implementation for User Story 3

- [ ] T019 [US3] Confirm the `Test (e2e)` step in `.github/workflows/ci.yml` (created in T003) invokes `go test -race -count=1 -timeout 5m ./e2e/...` and reports its result as a hard CI gate (NOT `continue-on-error`).
- [ ] T020 [US3] Add a paragraph to `docs/local-development.md` (or create it if needed) documenting the cross-repo CI signal flow per `contracts/ci-workflow.md` "Cross-repo signal flow" section. Includes the temporary-replace pattern for paired-PR development.

**Checkpoint**: US3 complete. Every Gjallarhorn PR runs the e2e tests; regression in either repo is caught at PR-review time.

---

## Phase 6: User Story 4 - Demo CLI works end-to-end (Priority: P3)

**Goal**: The Neverlur-side `neverlur-conversation-demo` binary (owned by the Neverlur Phase A tasks per `neverlur/specs/002-conversation-wiring/tasks.md` Phase 6) successfully consumes Gjallarhorn's harness package and completes the manual demo journey.

**Independent Test**: Manual smoke test per `quickstart.md` step 6, driven from the Neverlur side. From the Gjallarhorn side: nothing new; the harness package was already used by the integration test.

### Implementation for User Story 4

- [ ] T021 [US4] Ensure the harness's `OptionListenAddr` path is correctly exported and accepts the Unix socket convention documented in `contracts/test-harness-api.md`. Verify by running the Neverlur-side demo CLI smoke test once Gjallarhorn's Phase A merges; if the harness needs adjustment, file a follow-up task here.
- [ ] T022 [US4] Verify the harness's `Close()` correctly tears down the Unix socket on harness exit so a second `demo -as alice` invocation doesn't fail with "address already in use". Add a regression test if needed.

**Checkpoint**: US4 complete. The demo CLI runs end-to-end on a developer's laptop and the harness cleanly tears down between invocations.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [ ] T023 [P] Update `README.md` to mention the `e2e/` test directory and the no-classical-session-key static check as constitutional Principle III enforcement.
- [ ] T024 [P] Add `internal/testharness/` to the package list in `docs/` if a package overview exists; otherwise add a one-paragraph mention to `README.md`.
- [ ] T025 [P] Verify `gofmt -l .` is silent and `go vet ./...` is green on the merged branch. (CI gates this; this is a pre-PR self-check.)
- [ ] T026 [P] Commit message audit: every commit touching `convo/`, `cmd/gjallarhorn-client/`, `internal/testharness/`, `e2e/`, or `go.mod` must include the constitution-mandated "Threat-model impact" paragraph (Principle II).
- [ ] T027 Run the full `quickstart.md` from top to bottom on a clean linux/amd64 checkout. Capture any deviation as a bug; fix or document.

---

## Dependencies & Execution Order

### Phase dependencies

- **Phase 1 (Setup)**: T001 / T002 / T003 are independent and can land in parallel.
- **Phase 2 (Foundational)**: depends on Phase 1; T004 → (T005, T006, T007, T008, T011) in parallel → (T009, T010) sequentially.
- **Phase 3 (US1)**: depends on Phase 2 completion. T012 → T013 → T014.
- **Phase 4 (US2)**: T015 depends only on Phase 1 (it's an AST walk; no harness needed). T016 / T017 depend on T012 (they reuse its setup). T018 depends on Phase 2.
- **Phase 5 (US3)**: depends on T015 + (T012 or T016/T017) being implemented so the CI step has something to run.
- **Phase 6 (US4)**: depends on Phase 2 (the demo CLI imports the harness package). The actual demo binary lives in Neverlur; this phase is just verifying the harness contract holds end-to-end.
- **Phase 7 (Polish)**: depends on all Phase 3–6.

### User-story dependencies summary

- US1 (P1): full implementation here. 3 tasks.
- US2 (P1): independent of US1 once the harness exists. 4 tasks (1 static check + 2 sim subtests + 1 helper).
- US3 (P2): wraps US1 + US2 into CI; minimal new tasks. 2 tasks.
- US4 (P3): consumed via cross-repo; mostly a verification that the harness API holds. 2 tasks.

### Within Phase 2 (Foundational)

- T004 first (creates the directory + doc.go).
- T005, T006, T007, T008, T011 in parallel — different files in the same package.
- T009 depends on T005 (it references the `Harness` type).
- T010 depends on T005 and T006 (it drives coordinators).

### Parallel opportunities

- T001 / T002 / T003 in Phase 1.
- T005 / T006 / T007 / T008 / T011 in Phase 2.
- T015 / T016 / T017 in Phase 4 (they're separate test functions in separate files for T015 vs the others).
- T023 / T024 / T025 / T026 in Phase 7.

---

## Parallel Example: Phase 2 Foundational

```bash
# After T004 lands the package skeleton:
Task: "Implement internal/testharness/harness.go per T005"
Task: "Implement internal/testharness/coordinator.go per T006"
Task: "Implement internal/testharness/mixers.go per T007"
Task: "Implement internal/testharness/pkg.go per T008"
Task: "Implement internal/testharness/configs.go per T011"
# Then sequential:
Task: "Implement internal/testharness/client.go per T009"
Task: "Implement internal/testharness/rounds.go per T010"
```

## Parallel Example: Phase 4 (US2) tests

```bash
# All three test files can land in parallel once the harness exists:
Task: "Write e2e/no_classical_session_key_test.go per T015"
Task: "Write e2e/first_message_test.go::TestE2EFirstMessage_HybridConfidentiality_ClassicalCompromise per T016"
Task: "Write e2e/first_message_test.go::TestE2EFirstMessage_HybridConfidentiality_PQCompromise per T017"
```

---

## Implementation Strategy

### MVP First (US1 + US2 together)

US1 is the foundational property and US2 is its constitutional carry-through enforcement. Shipping US1 without US2 would be a Principle III violation (no enforcement against silent classical fallback). Land them together:

1. Phase 1 Setup (T001–T003).
2. Phase 2 Foundational (T004–T011).
3. Phase 3 US1 (T012–T014).
4. Phase 4 US2 (T015–T018).
5. **STOP and VALIDATE**: both `TestE2EFirstMessage` and `TestNoClassicalSessionKeySource` green in CI.
6. Phase 5 US3 (T019, T020) — wraps the above into the CI signal.
7. Phase 6 US4 (T021, T022) — pair with Neverlur's demo CLI work.
8. Phase 7 polish.

### Incremental delivery

- After Phase 2: harness package is usable but no tests yet. Can land as its own PR if needed.
- After Phase 3 + 4: full Phase A on Gjallarhorn side. CI is hard-gating. Neverlur-side demo work can begin.
- After Phase 5: cross-repo CI signal flow is documented and operational.
- After Phase 6 + 7: demo CLI works end-to-end; polish complete.

### Parallel team strategy

1 dev: in order, T001→T003 (~1 day), T004→T011 (~3 days), T012→T018 (~2 days), polish (~1 day). ~1-2 weeks total.

2 devs after Phase 1:
- Dev A: Phase 2 harness package
- Dev B: Phase 4 static-check test (T015) — can land independently with no harness dependency
- Reconverge at Phase 3 (US1 needs the harness; T016/T017 need T012's setup).

---

## Notes

- Every task is independently re-runnable; rerunning a completed task is a no-op (re-run `go mod tidy`, KAT-style re-run of `TestNoClassicalSessionKeySource`, etc.).
- Commit at task boundaries; every commit touching `convo/`, `cmd/gjallarhorn-client/`, `internal/testharness/`, `e2e/`, or `go.mod` MUST include the constitution-mandated "Threat-model impact" paragraph (Principle II).
- `go test ./...` MUST be green at every Checkpoint.
- Cross-repo coordination: this Gjallarhorn-side work lands BEFORE the Neverlur-side demo CLI work. Neverlur's tasks.md T001 pins to the merged Gjallarhorn commit-sha after Phase 2 lands on master.
- Avoid: silently bypassing the hybrid combiner in any new code path. The static-check test (T015) is the load-bearing enforcement; do not allowlist new patterns there without documenting the constitutional justification in the test's allowlist comment.
