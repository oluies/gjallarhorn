# Feature Specification: Conversation Wiring (Gjallarhorn side, Phase A companion)

**Feature Branch**: `001-conversation-wiring`

**Created**: 2026-05-23

**Status**: Draft

**Input**: User description: "Phase A companion spec: Gjallarhorn-side of the conversation wiring between Neverlur's hybrid keywheel seed and Gjallarhorn's conversation mixnet — pairs with neverlur/specs/002-conversation-wiring/spec.md"

**Companion**: This is the Gjallarhorn-side spec. The Neverlur-side primary spec lives at `oluies/neverlur:specs/002-conversation-wiring/spec.md` and was drafted first because Neverlur is the producer of the keywheel seed; Gjallarhorn is the consumer. Both specs MUST land before Phase A implementation begins, per the Gjallarhorn constitution's Compatibility section.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Gjallarhorn delivers a friended pair's first message (Priority: P1)

A user whose client holds a Neverlur-derived keywheel seed for a peer can join a Gjallarhorn dialing round, send a conversation packet addressed to that peer, and have the peer receive it during the same round. The mixers, the coordinator, and any non-target client see only undistinguishable mixed traffic; the metadata-private property of the SOSP 2015 Vuvuzela design is preserved unchanged.

**Why this priority**: This is the foundational property that distinguishes Gjallarhorn from "any messaging system" — without it, the whole mixnet construction is decoration. Existing Vuvuzela code already does this for classical keys; the Phase A work is teaching the Gjallarhorn client to source its session keys from the hybrid keywheel seed (`neverlur/keywheel.Wheel.SessionKey`) instead of a classical-only construction.

**Independent Test**: A Go integration test inside the Gjallarhorn repo that constructs two in-process clients, hands each one a pre-populated `keywheel.Wheel` (with a seed derived via the Neverlur side's hybrid combiner), joins a single dialing round on an in-process Gjallarhorn coordinator + mixer set, and asserts that a conversation packet sent by Alice is received by Bob with the bytes intact.

**Acceptance Scenarios**:

1. **Given** Alice and Bob each hold a `keywheel.Wheel` containing a per-friend seed for the other (produced by a Neverlur add-friend round, real or simulated), **When** both clients join Gjallarhorn dialing round R and Alice composes a conversation packet for Bob, **Then** Bob's receive handler observes Alice's bytes in the round-R output.
2. **Given** the conversation packet in transit, **When** the coordinator, mixers, and non-target clients inspect their inputs/outputs, **Then** none of them can determine that Alice and Bob are the conversing pair (Vuvuzela mixnet anonymity, unchanged).
3. **Given** the same pair sends a second message in round R+N, **When** Bob receives it, **Then** decryption succeeds under the ratcheted per-round session key derived from the wheel.

---

### User Story 2 - Conversation session-key derivation rejects non-hybrid sources (Priority: P1)

The Gjallarhorn client MUST NOT have a code path that produces or consumes a conversation session key from raw classical key material. Specifically: the existing Vuvuzela code that derived per-round keys from a classical X25519 shared secret must be replaced (not augmented with a fallback) by the hybrid-keywheel path. Constitution Principle III ("Hybrid, Never Pure — non-negotiable") forbids any silent classical-only path.

**Why this priority**: P1 because a silent fallback would defeat the entire harvest-now-decrypt-later argument that motivates the migration. An adversary who later breaks X25519 but not ML-KEM (or vice versa) MUST still find the conversation packet undecryptable. That property carries through from Neverlur's combiner output only if the Gjallarhorn side actually uses the combiner output.

**Independent Test**: A "classical-half compromise" simulation test in the Gjallarhorn integration suite: capture a conversation packet, hand the verifier only the X25519 component of the original friend-discovery shared secret, attempt to derive the per-round session key, assert failure. Symmetric test for ML-KEM-half compromise. Plus a `grep`/static check confirming no `box.Precompute`-style classical-only call sites remain in the Gjallarhorn conversation path.

**Acceptance Scenarios**:

1. **Given** a captured Gjallarhorn conversation packet, **When** an attacker holding only the X25519 shared secret from the originating friend-discovery round attempts to derive the per-round session key, **Then** they fail (the hybrid combiner property carries through).
2. **Given** the symmetric scenario for ML-KEM compromise, **When** the attacker attempts derivation, **Then** they symmetrically fail.
3. **Given** the Gjallarhorn source tree post-Phase-A, **When** the codebase is searched for raw `box.Precompute`, `curve25519.X25519`, or equivalent classical-shared-secret call sites in the conversation path, **Then** zero occurrences are found outside the documented hybrid combiner boundary (the X25519 component lives inside `hybrid.CombineKEM` and `hybrid.CombineKEMConcat` on the Neverlur side; Gjallarhorn never sees the components directly).

---

### User Story 3 - Phase A integration test runs in CI on both repos (Priority: P2)

A Go integration test exists that exercises the full friend-discovery + first-conversation-message journey with Neverlur and Gjallarhorn in one process. It runs in linux/amd64 CI on whichever repo's PR triggered the workflow (the integration test logically belongs in Gjallarhorn since it asserts the conversation-layer behavior, but the test code transitively imports both modules). Regression in either module's session-key derivation surfaces as a CI failure.

**Why this priority**: P2 — the integration test is the *evidence* that US1 works, not US1 itself. It's high-value but the property exists before the test is written; the test just makes the property checkable on every PR.

**Independent Test**: Run `go test -run TestE2EFirstMessage -timeout 5m ./...` on a checkout with both repos at compatible commits (Go workspace setup per `docs/local-development.md`). Exit status zero is the test passing.

**Acceptance Scenarios**:

1. **Given** a fresh Linux/amd64 CI runner with both repos at compatible commits and a Go workspace in place, **When** the integration test runs, **Then** it completes within 5 minutes and asserts that Bob received Alice's bytes.
2. **Given** a regression in either repo's session-key derivation, **When** the integration test runs, **Then** the round-trip fails with a diagnostic that names the disagreement.

---

### User Story 4 - Demo CLI works end-to-end (Priority: P3)

The Phase A demo CLI binary (`neverlur/cmd/neverlur-conversation-demo`, defined in the Neverlur-side spec) interacts with Gjallarhorn's in-process coordinator/mixer/CDN through the same shared client state the integration test uses. Two terminals on one laptop demonstrate friend-discovery + first message in under 5 minutes wall-clock.

**Why this priority**: P3 — demo is for human comprehension, not correctness signal. Skipping it doesn't break the property; having it makes the system legible.

**Independent Test**: Manual on one machine. Launch terminal A and terminal B; complete the full demo journey; observe Bob's terminal print Alice's message.

**Acceptance Scenarios**:

1. **Given** a fresh laptop, **When** the demonstrator runs the demo binary in two terminals, **Then** the journey completes in under 5 minutes and Bob's terminal prints Alice's message.

---

### Edge Cases

- **Wheel state out of sync**: Alice's wheel has ratcheted past round R but Bob's hasn't. Both clients independently re-derive the session key for round R via the wheel's `getSecret` path; the wheel's existing ratchet semantics handle this and Phase A does not change it.
- **Conversation packet for an unknown sender**: Bob receives a mailbox containing a packet whose session key Bob cannot derive (because Bob doesn't have a wheel entry for the claimed sender). Bob's receive handler discards silently — same as the existing Vuvuzela behavior; Phase A does not change it.
- **Mid-handshake protocol upgrade**: An operator upgrades the Gjallarhorn coordinator mid-round. Existing Vuvuzela "no mixed-version configs in a single round" rule (Gjallarhorn constitution Compatibility section) handles this; Phase A does not change it.

## Requirements *(mandatory)*

### Functional Requirements

#### Boundary between Neverlur and Gjallarhorn

- **FR-001**: The Gjallarhorn conversation client MUST consume a `keywheel.Wheel` instance produced by the Neverlur friend-discovery side; it MUST NOT maintain its own parallel wheel state for the same friendship.
- **FR-002**: The per-round conversation session key MUST be obtained via `Wheel.SessionKey(peer, round)` for the dialing round in scope. No alternative session-key source MAY be introduced in Gjallarhorn for friendships that came through Neverlur.
- **FR-003**: The shared interface between Neverlur and Gjallarhorn MUST be a single Go package imported by both (canonical home: `github.com/oluies/neverlur/keywheel`, already established).

#### Conversation round-trip

- **FR-004**: A Gjallarhorn client that holds a populated wheel for friend Bob MUST be able to compose a conversation packet addressed to Bob's mailbox derived from `Wheel.OutgoingDialToken(bob, round, intent)`.
- **FR-005**: A Gjallarhorn client receiving its mailbox MUST be able to decrypt the packet addressed to it using `Wheel.SessionKey(friend, round)` for the appropriate `(friend, round)` pair.
- **FR-006**: The conversation packet MUST carry enough metadata (within the existing Vuvuzela packet structure) for the receiver to identify which friend's session key to use, without leaking that information to mixers or coordinator (the existing Vuvuzela construction handles this; Phase A does not change it).

#### Hybrid PQ confidentiality carry-through

- **FR-007**: The Gjallarhorn conversation path MUST NOT contain any code that derives a per-round session key from raw classical key material (X25519 shared secret, classical-only HKDF, NaCl box.Precompute, etc.) for friendships that traveled through Neverlur post-PQ-cutover.
- **FR-008**: The Gjallarhorn integration test suite MUST include a "classical-half compromise" simulation that confirms an adversary holding only the X25519 component of the original friend-discovery shared secret cannot derive the per-round session key.
- **FR-009**: The same test suite MUST include the symmetric "PQ-half compromise" simulation.

#### Integration test

- **FR-010**: The Gjallarhorn repository MUST contain (or import from Neverlur) a Go integration test that, in a single test process, stands up an in-process Gjallarhorn coordinator/mixer/CDN; populates two clients (Alice and Bob) with wheel entries for each other; runs them through one dialing round; and asserts that Bob received Alice's bytes intact.
- **FR-011**: The test MUST complete within 5 minutes on linux/amd64; it MAY skip on arm64 with a clear skip message (pre-existing bn256 limitation).
- **FR-012**: A regression in either Neverlur or Gjallarhorn session-key derivation MUST cause the test to fail with a diagnostic that names the disagreement.

#### Wire-format stability

- **FR-013**: Phase A MUST NOT change Gjallarhorn's on-wire conversation packet format, dialing-round protocol, or dead-drop convention. Those remain as documented in the Vuvuzela paper and as implemented in the inherited code.
- **FR-014**: If Phase A discovers that a wire-format change IS required to host the hybrid session-key derivation, that discovery becomes a separate spec and a separate paired PR, NOT bundled into Phase A.

### Key Entities *(include if feature involves data)*

- **Shared Wheel Instance**: A single `keywheel.Wheel` (from `github.com/oluies/neverlur/keywheel`) instance per client process, populated by Neverlur's friend-discovery path and consumed by Gjallarhorn's conversation path. The wheel state is the integration interface.
- **Per-Round Conversation Session Key**: The 32-byte key under which a single conversation packet is encrypted in dialing round R, returned by `Wheel.SessionKey(peer, R)`. Already hybrid-derived at the Neverlur-side when the wheel was populated; Gjallarhorn consumes it opaquely.
- **In-Process Test Harness**: The set of mocked coordinator, mixer, and CDN components that the Gjallarhorn integration test stands up so the full round-trip runs in one Go test process.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A green CI run of the Phase A integration test on Gjallarhorn demonstrates that one message round-trips between two friended in-process clients in under 5 minutes wall-clock on a Linux/amd64 runner.
- **SC-002**: A static check (`grep` or a `staticcheck`-style analyzer) confirms zero occurrences of classical-only session-key derivation patterns in the Gjallarhorn conversation path post-Phase-A.
- **SC-003**: A regression in Neverlur's `keywheel.Wheel.SessionKey` output causes the Gjallarhorn integration test to fail within 10 seconds of the regression, with a diagnostic that names the disagreement.
- **SC-004**: The classical-half compromise simulation test confirms that the X25519 component alone is insufficient to recover the conversation session key.
- **SC-005**: The PQ-half compromise simulation test confirms the symmetric property for ML-KEM.

## Assumptions

- **Neverlur PR #4 is the foundation**: This spec assumes the PQ foundation packages (`neverlur/pqkem`, `neverlur/pqsig`, `neverlur/hybrid`, and the `hybrid`-derived seed in `neverlur/keywheel.Wheel`) are merged to Neverlur master. They were as of the merge of `oluies/neverlur#4`.
- **Go workspace local convention**: Per `docs/local-development.md` in both repos, developers hack on both sides via a `go.work` file at the parent directory. The CI environment does NOT rely on the workspace; it uses pinned `go.mod` versions, which means a CI run of the Phase A integration test in Gjallarhorn MUST `go get` a published Neverlur version (or use a temporary CI-side `go.work`). The CI mechanism is a planning-phase decision documented in the plan, not in this spec.
- **Vuvuzela packet format is unchanged**: As stated in FR-013, the on-wire conversation packet format, dialing-round protocol, and dead-drop convention do not change in Phase A. The work is purely about WHERE the session key comes from.
- **Pre-existing bn256 / arm64 limitation**: The integration test cannot run locally on Apple Silicon; it runs in linux/amd64 CI. This is inherited from the pre-PQ codebase and is not Phase A's problem to solve.
- **PKG attestation v2 is NOT a prerequisite**: Like the Neverlur-side companion, this spec deliberately operates against the existing classical PKG attestation chain. PKG attestation v2 is its own sub-project, deferred.
- **Companion Neverlur spec is the primary reference**: When the two specs disagree on a wording or scope question, the Neverlur-side spec (`oluies/neverlur:specs/002-conversation-wiring/spec.md`) is authoritative. This spec mirrors that one's scope from the Gjallarhorn-consumer perspective.
