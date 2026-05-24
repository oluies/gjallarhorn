// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

import (
	"testing"

	"github.com/oluies/neverlur"
	"github.com/oluies/neverlur/hybrid"
)

// TestClient is the harness-managed wrapper around *neverlur.Client.
// Embeds the client so all standard methods are available; adds RecvCh
// and ConvoState for the integration test to read.
//
// Scaffold status: the struct shape is real; ClientFor() returns a
// TestClient with HybridIdentity populated but *neverlur.Client and
// the channels stubbed until the harness's PKG/coordinator wiring
// lands.
type TestClient struct {
	*neverlur.Client

	// Username is the PKG-registered username (e.g. "alice@demo.local").
	Username string

	// HybridIdentity is the 3-primitive identity (Ed25519 + ML-DSA-65
	// + ML-KEM-768) generated for this client.
	HybridIdentity *hybrid.HybridIdentity

	// RecvCh is a buffered channel that captures every conversation
	// packet this client receives (decrypted body, sender, round).
	// Closed when the client is torn down by the harness.
	RecvCh <-chan IncomingMessage

	// ConvoState is the conversation-layer state for the current
	// active conversation, populated by SendingCall / ReceivedCall
	// callbacks. Nil until a call bootstraps.
	ConvoState ConvoStateAccessor

	// internal channels for harness-side population
	recvCh chan IncomingMessage
}

// IncomingMessage captures one decrypted conversation packet delivery
// for the integration test or demo CLI to read off RecvCh.
type IncomingMessage struct {
	FromPeer string
	Body     []byte
	Round    uint32
	Err      error
}

// ConvoStateAccessor is a narrow read-only interface over the
// conversation state. Hides Gjallarhorn-internal types from test
// consumers. Implemented by the per-conversation state object the
// harness wires up; nil until a call bootstraps.
type ConvoStateAccessor interface {
	SessionKey() *[32]byte
	Round() uint32
	PeerUsername() string
}

// ClientFor returns a Neverlur client wired against this harness with
// the given username. The client's hybrid identity is freshly generated
// each call. The client is automatically registered with the PKG;
// failure fails the test via tb.Fatal.
//
// TODO(testharness-impl): construct the real *neverlur.Client with:
//   - the harness's PKG endpoint
//   - the harness's coordinator endpoints (AddFriend + Dialing)
//   - EventHandler callbacks wired to push into recvCh (for
//     RecvCh) and an IncomingFriendRequest channel
//   - register with the PKG; await registration confirmation
//
// Current scaffold behavior: returns a TestClient with HybridIdentity
// real but Client nil and a closed recvCh.
func (h *Harness) ClientFor(tb testing.TB, username string) *TestClient {
	tb.Helper()

	id, err := hybrid.GenerateHybridIdentity()
	if err != nil {
		tb.Fatalf("testharness: ClientFor: generate identity: %v", err)
		return nil
	}

	ch := make(chan IncomingMessage, 16)
	close(ch) // scaffold: nothing will produce; closed so reads return zero-value immediately.

	tc := &TestClient{
		Username:       username,
		HybridIdentity: id,
		RecvCh:         ch,
		recvCh:         ch,
		// Client and ConvoState left nil per scaffold notes above.
	}
	return tc
}

// SendMessage validates a message at the input boundary (length,
// UTF-8) and submits it through the active Conversation. Validation
// matches the demo CLI's so the integration test reuses the same
// surface.
//
// TODO(testharness-impl): once ClientFor wires the real client and
// conversation state, this calls into Conversation.Seal + the onion
// submission path.
func (tc *TestClient) SendMessage(body []byte) error {
	if len(body) == 0 {
		return scaffoldedNotImplemented("TestClient.SendMessage (empty message)")
	}
	// TODO: check len(body) <= convo.ConvoMessageSize and utf8.Valid(body)
	return scaffoldedNotImplemented("TestClient.SendMessage")
}

// IncomingFriendRequestCh returns a channel of incoming friend
// requests (used by the integration test to drive the Bob-approve
// step).
//
// TODO(testharness-impl): wire up via the EventHandler.ReceivedFriendRequest
// callback set in ClientFor.
func (tc *TestClient) IncomingFriendRequestCh() <-chan *neverlur.IncomingFriendRequest {
	ch := make(chan *neverlur.IncomingFriendRequest)
	close(ch)
	return ch
}

// HasFriend returns true if this client has an established friendship
// with the named peer.
//
// TODO(testharness-impl): proxy to tc.Client.GetFriend(peer) != nil.
func (tc *TestClient) HasFriend(peer string) bool {
	return false
}
