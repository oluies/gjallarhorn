# Contract: `e2e/` integration test cases

**Owner**: Gjallarhorn
**Files**: `e2e/first_message_test.go`, `e2e/no_classical_session_key_test.go`
**Stability**: test contract is stable from this Phase A commit; subtests can be added but not removed without paired-PR coordination.

## TestE2EFirstMessage

**Purpose**: Asserts the foundational property of Phase A: two users who completed an Alpenhorn-style add-friend round can exchange one conversation packet end-to-end.

**Skip condition**: `runtime.GOARCH == "arm64"` — the inherited `vuvuzela.io/crypto/bn256` ships x86_64-only assembly. Test skips with the message `"skipped on arm64: vuvuzela.io/crypto/bn256 ships x86_64-only assembly"`.

**Body**:

```go
func TestE2EFirstMessage(t *testing.T) {
    if runtime.GOARCH == "arm64" {
        t.Skip("skipped on arm64: vuvuzela.io/crypto/bn256 ships x86_64-only assembly")
    }

    h := testharness.New(t)
    alice := h.ClientFor(t, "alice@demo.local")
    bob := h.ClientFor(t, "bob@demo.local")

    // === Friend discovery ===
    _, err := alice.SendFriendRequest("bob@demo.local", nil)
    if err != nil { t.Fatalf("SendFriendRequest: %v", err) }
    h.AdvanceRound("AddFriend")

    // Bob's handler captures the IncomingFriendRequest; reach into
    // bob.RecvCh-equivalent for friend-request notifications.
    inReq := <-bob.IncomingFriendRequestCh()
    if _, err := inReq.Approve(); err != nil { t.Fatalf("Approve: %v", err) }
    h.AdvanceRound("AddFriend")

    // Both clients now have wheel entries for the other.
    if !alice.HasFriend("bob@demo.local") { t.Fatal("alice missing friend") }
    if !bob.HasFriend("alice@demo.local") { t.Fatal("bob missing friend") }

    // === Conversation bootstrap ===
    if _, err := alice.SendCall("bob@demo.local", 0); err != nil {
        t.Fatalf("SendCall: %v", err)
    }
    h.AdvanceRound("Dialing")

    // Bob's ReceivedCall handler has fired; ConvoState non-nil on both.
    if alice.ConvoState == nil { t.Fatal("alice ConvoState nil") }
    if bob.ConvoState == nil { t.Fatal("bob ConvoState nil") }

    aliceKey := alice.ConvoState.SessionKey()
    bobKey := bob.ConvoState.SessionKey()
    if !bytes.Equal(aliceKey[:], bobKey[:]) {
        t.Fatalf("Alice session key != Bob session key:\n  alice %x\n  bob   %x",
            aliceKey[:], bobKey[:])
    }

    // === First message ===
    msg := []byte("hello bob")
    if err := alice.SendMessage(msg); err != nil { t.Fatalf("SendMessage: %v", err) }
    h.AdvanceRound("Convo")

    received := <-bob.RecvCh
    if received.Err != nil { t.Fatalf("recv: %v", received.Err) }
    if !bytes.Equal(received.Body, msg) {
        t.Fatalf("wrong message:\n  got  %q\n  want %q", received.Body, msg)
    }
    if received.FromPeer != "alice@demo.local" {
        t.Fatalf("wrong sender: %q", received.FromPeer)
    }
}
```

**Assertions**:
- Friend discovery completes within 2 add-friend rounds.
- Conversation bootstrap completes within 1 dialing round.
- Alice's and Bob's session keys are byte-identical (the contract test for FR-001 / FR-002).
- The conversation packet's body decrypts to the original bytes (the contract test for FR-005).

**Failure diagnostics** (mandated):
- "Alice session key != Bob session key" — keywheel-seed contract violation.
- "wrong message: got X, want Y" — conversation packet payload corruption.
- "wrong sender: <name>" — receiver attribution leaked or wrong.

## TestE2EFirstMessage_HybridConfidentiality_ClassicalCompromise

**Purpose**: Direct test-level satisfaction of spec SC-002 / Gjallarhorn-side SC-004. Asserts that an adversary holding only the X25519 component of the original friend-discovery shared secret CANNOT derive the conversation session key.

**Body** (Phase A implementation per research.md R8 first attempt):

```go
func TestE2EFirstMessage_HybridConfidentiality_ClassicalCompromise(t *testing.T) {
    if runtime.GOARCH == "arm64" { t.Skip("...") }

    h := testharness.New(t)
    alice := h.ClientFor(t, "alice@demo.local")
    bob := h.ClientFor(t, "bob@demo.local")
    // ... friend discovery + dialing as in TestE2EFirstMessage ...

    realKey := alice.ConvoState.SessionKey()

    // Simulate classical-half compromise at the keywheel-seed layer.
    // We don't have a test-only accessor to pull ssX25519 out of
    // alice.Client; instead, we re-run CombineKEMConcat with the PQ
    // half zeroed out and assert the result differs from the real key.
    ssX25519 := mustExtractClassicalShare(t, alice)
    ssMLKEMConcat := allZeros(64) // attacker doesn't have it
    transcript := mustExtractTranscript(t, alice, "bob@demo.local")

    attackerSeed, err := hybrid.CombineKEMConcat(
        hybrid.ContextKeywheelSeed, transcript, ssX25519, ssMLKEMConcat,
    )
    if err != nil { t.Fatalf("attacker CombineKEMConcat: %v", err) }

    // The attacker's seed feeds the same wheel ratchet; derive the
    // attacker's would-be session key for the same round.
    var attackerWheel keywheel.Wheel
    attackerWheel.Put("alice@demo.local", alice.ConvoState.Round(), attackerSeed)
    attackerKey := attackerWheel.SessionKey("alice@demo.local",
        alice.ConvoState.Round())

    if attackerKey == nil { t.Fatal("attackerKey unexpectedly nil") }
    if bytes.Equal(realKey[:], attackerKey[:]) {
        t.Fatal("hybrid carry-through failed: classical-half compromise " +
            "recovered the conversation session key")
    }
}
```

Helpers `mustExtractClassicalShare` and `mustExtractTranscript` are Gjallarhorn-side test utilities that scrape the captured `keywheelStart` state through Gjallarhorn's existing internal accessors (which already expose enough for the rolled key + round to be read; see `conversation.go`). If insufficient, the Neverlur-side accessor follow-up flagged in research.md R8 lands separately.

## TestE2EFirstMessage_HybridConfidentiality_PQCompromise

Symmetric to the above. Swaps which half the simulated attacker holds.

## TestNoClassicalSessionKeySource

**Purpose**: Static-check enforcement of spec FR-007 / SC-005 (Gjallarhorn-side SC-002). Walks the Gjallarhorn source tree via `go/ast` and asserts no forbidden classical-only session-key derivation patterns appear in the conversation path.

**Body**:

```go
func TestNoClassicalSessionKeySource(t *testing.T) {
    forbidden := []string{
        "box.Precompute",
        "curve25519.X25519",
        "nacl/box.Precompute",
    }
    paths := []string{
        "../convo",
        "../cmd/gjallarhorn-client",
        "../coordinator",
        "../mixnet",
    }
    var hits []hit
    for _, dir := range paths {
        hits = append(hits, walkForbidden(t, dir, forbidden)...)
    }
    if len(hits) > 0 {
        for _, h := range hits {
            t.Errorf("forbidden classical-only call: %s:%d: %s\n  -> %s",
                h.File, h.Line, h.Pattern, h.Snippet)
        }
    }
}
```

**Failure mode**: A new commit anywhere in `convo/`, `cmd/gjallarhorn-client/`, `coordinator/`, or `mixnet/` that introduces `box.Precompute` or `curve25519.X25519` fails this test with a per-hit diagnostic.

**Why this is enough**: Per research.md R9, the EXISTING conversation code uses `nacl/secretbox` with the rolled key — never `nacl/box`. The forbidden-pattern list catches the only realistic regression: someone reintroducing a classical-only path "for convenience".

**Allowlist**: empty. Phase A code does not need any of the forbidden patterns in any of the listed paths. If a future feature legitimately needs one, the allowlist grows with a paragraph explaining why.
