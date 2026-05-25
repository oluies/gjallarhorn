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
// STILL SKIPPED — one upstream blocker remains after the Phase A
// fix-up rounds. The reference implementation below COMPILES (the
// test body is no longer commented out) and runs to the point where
// the Neverlur AddFriend coordinator tries to drive a real round
// through its mock mixchain. That fails with:
//
//   adding onions: rpc error: code = Internal desc = cardinality
//   violation: received no response message from non-server-
//   streaming RPC  call=mixnet.RunRound round=2
//
// because neverlur/mock/mix.go uses vuvuzela.io/vuvuzela/mixnet
// (upstream Vuvuzela), whose Server.AddOnions handler is missing
// the SendAndClose call that modern gRPC requires. We fixed the
// same bug in github.com/oluies/gjallarhorn/mixnet (PR #8) but the
// upstream-Vuvuzela mixnet that the Neverlur mock imports is
// outside our module replace scope.
//
// The 1-mixer workaround (testharness defaultOptions
// neverlurMixers=1) only dodges the inter-mixer AddOnions call;
// the coordinator → mixer-0 AddOnions call still hits this bug.
//
// G6 (future): fork vuvuzela.io/vuvuzela/mixnet into an
// oluies-owned repo, apply the one-line SendAndClose fix, update
// Neverlur's go.mod replace, then unskip this test.
//
// Constitutional Principle II: this test exercises the full hybrid
// keywheel pipeline end-to-end, giving confidence that the
// production add-friend → dial → conversation chain still derives
// session keys via the hybrid combiner. Same code path the
// gjallarhorn-conversation-demo binary uses for human-facing
// smoke checks.
//
// Runtime budget (once unskipped): ~120-180s of coordinator rounds.
func TestE2EFirstMessage(t *testing.T) {
	t.Skip("blocked on vuvuzela.io/vuvuzela/mixnet AddOnions SendAndClose fix (G6); see comment")

	h := testharness.New(t)
	alice := h.ClientFor(t, "alice@harness.test")
	bob := h.ClientFor(t, "bob@harness.test")

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
