// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package e2e

import (
	"bytes"
	"testing"
	"time"

	"github.com/oluies/gjallarhorn/internal/testharness"
)

// TestE2EFirstMessage drives a full add-friend → dial → first-message
// round-trip between two TestClients (Alice and Bob) standing up a
// fresh testharness.Harness.
//
// Constitutional Principle II: this test exercises the full hybrid
// keywheel pipeline end-to-end, giving confidence that the
// production add-friend → dial → conversation chain still derives
// session keys via the hybrid combiner. Same code path the
// gjallarhorn-conversation-demo binary uses for human-facing
// smoke checks.
//
// Runtime budget: ~120-180s of coordinator rounds.
//
// STILL SKIPPED — the mixnet/log.Fatal/race chain (G2/G5/G6) is
// resolved and add-friend + dial + ConvoState bootstrap all complete
// reliably in CI. But the final message round-trip times out at the
// 60s wait on bob.RecvCh: Bob's trial-decrypt of the per-round
// reply set never matches Alice's sealed message.
//
// The crypto is byte-identical to cmd/gjallarhorn-client/
// conversation.go (rollKey + secretbox + HMAC-SHA256 dead-drop)
// so the most likely remaining issue is keywheel synchronization
// across the harness: alice's wheel.SessionKey(bob, dialRound)
// not equaling bob's wheel.SessionKey(alice, dialRound). That would
// produce divergent rollKey outputs at every convo round and bob's
// Open would never match any ciphertext.
//
// G7 (future): instrument harnessConvoState.Seal/Open/DeadDrop with
// t.Logf to dump alice's and bob's per-round keys + drop bytes, then
// fix whatever wiring discrepancy that exposes. Likely candidate:
// the add-friend handshake completes in the harness but the keywheel
// state on the two sides doesn't actually converge to the same
// shared secret per (peer, round) tuple.
func TestE2EFirstMessage(t *testing.T) {
	// G7 debug branch: instrumentation on; test unskipped to capture
	// CI logs of alice's and bob's per-round Seal/Open/DeadDrop key
	// material. Compare on success of the run to find where they
	// diverge.

	h := testharness.New(t)
	alice := h.ClientFor(t, "alice@harness.test")
	bob := h.ClientFor(t, "bob@harness.test")
	alice.Logf = t.Logf
	bob.Logf = t.Logf

	aliceDisc, err := alice.Start()
	if err != nil {
		t.Fatalf("alice.Start: %v", err)
	}
	bobDisc, err := bob.Start()
	if err != nil {
		t.Fatalf("bob.Start: %v", err)
	}
	t.Cleanup(alice.Stop)
	t.Cleanup(bob.Stop)
	go drainDisconnects(t, "alice", aliceDisc)
	go drainDisconnects(t, "bob", bobDisc)

	// --- Add-friend handshake -----------------------------------------------
	if _, err := alice.SendFriendRequest(bob.Username, nil); err != nil {
		t.Fatalf("alice SendFriendRequest: %v", err)
	}
	select {
	case req := <-bob.IncomingFriendRequestCh():
		if _, err := req.Approve(); err != nil {
			t.Fatalf("bob Approve: %v", err)
		}
	case <-time.After(90 * time.Second):
		t.Fatal("bob never received friend request within 90s")
	}
	waitFor(t, 90*time.Second, "alice ↔ bob friendship confirmed", func() bool {
		return alice.HasFriend(bob.Username) && bob.HasFriend(alice.Username)
	})

	// --- Dial: bootstrap conversation ---------------------------------------
	f := alice.GetFriend(bob.Username)
	if f == nil {
		t.Fatal("alice has no Friend object for bob after confirmation")
	}
	_ = f.Call(0)
	waitFor(t, 90*time.Second, "ConvoState bootstrapped on both sides", func() bool {
		return alice.ConvoState != nil && bob.ConvoState != nil
	})

	// --- First message round-trip -------------------------------------------
	body := []byte("hello bob")
	if err := alice.SendMessage(body); err != nil {
		t.Fatalf("alice SendMessage: %v", err)
	}
	select {
	case msg := <-bob.RecvCh:
		if !bytes.Equal(msg.Body, body) {
			t.Errorf("body mismatch: got %q want %q", msg.Body, body)
		}
		if msg.FromPeer != alice.Username {
			t.Errorf("FromPeer = %q, want %q", msg.FromPeer, alice.Username)
		}
	case <-time.After(60 * time.Second):
		t.Fatal("bob never received the message within 60s")
	}
}

// waitFor polls condition every 250ms until it returns true or the
// deadline expires. Used for handshake/dialing steps where the
// natural coordinator loop takes multiple rounds to converge.
func waitFor(t *testing.T, timeout time.Duration, label string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %q after %s", label, timeout)
}

// drainDisconnects logs (via t.Logf) per-service websocket close
// events from a TestClient.Start() Disconnects channel. The test
// doesn't fail on disconnects directly — the assertion on RecvCh
// will time out if a disconnect kills the flow.
func drainDisconnects(t *testing.T, who string, ch <-chan testharness.ConnectError) {
	for ce := range ch {
		t.Logf("[%s] %s websocket disconnected: %v", who, ce.Service, ce.Err)
	}
}
