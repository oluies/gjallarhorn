// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

import (
	"crypto/ed25519"
	"crypto/rand"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/oluies/neverlur"
	"github.com/oluies/neverlur/config"
	"github.com/oluies/neverlur/hybrid"
)

// TestClient is the harness-managed wrapper around *neverlur.Client.
// Embeds the client so all standard methods are available; adds
// IdentityFor* and RecvCh helpers for the integration test.
type TestClient struct {
	*neverlur.Client

	// Username is the PKG-registered username.
	Username string

	// HybridIdentity is the 3-primitive identity (Ed25519 + ML-DSA-65
	// + ML-KEM-768) generated for this client. The classical
	// (Ed25519) half is also stored on the embedded *neverlur.Client
	// as LongTermPublicKey/LongTermPrivateKey for backward compat.
	HybridIdentity *hybrid.HybridIdentity

	// RecvCh fires for every conversation packet this client
	// receives (decrypted body, sender, round).
	RecvCh <-chan IncomingMessage

	// ConvoState is the conversation-layer state for the current
	// active conversation. Populated by SendingCall / ReceivedCall
	// handler callbacks; nil until a call bootstraps.
	ConvoState ConvoStateAccessor

	// internal channels populated by the handler callbacks below.
	recvCh      chan IncomingMessage
	friendReqCh chan *neverlur.IncomingFriendRequest
	sentReqCh   chan *neverlur.OutgoingFriendRequest
	confirmedCh chan *neverlur.Friend
	errCh       chan error
}

// IncomingMessage captures one decrypted conversation packet delivery.
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
//
// Phase 3 (the e2e integration test) is where this interface gets
// its real implementation tied to gjallarhorn.Conversation.
type ConvoStateAccessor interface {
	SessionKey() *[32]byte
	Round() uint32
	PeerUsername() string
}

// ClientFor returns a Neverlur client wired against this harness with
// the given username. Generates a fresh hybrid identity, builds a
// *neverlur.Client, and registers the username with the harness's
// PKG. The returned TestClient is ready for SendFriendRequest /
// SendCall / SendMessage; the event-handler channels are populated
// by the standard EventHandler callbacks.
//
// Failure (key generation, PKG registration) fails the test via
// tb.Fatal.
//
// Note: registration sets registration tokens to the empty string;
// the harness's PKG uses a noop RegTokenHandler that always accepts.
func (h *Harness) ClientFor(tb testing.TB, username string) *TestClient {
	tb.Helper()

	id, err := hybrid.GenerateHybridIdentity()
	if err != nil {
		tb.Fatalf("testharness: ClientFor: generate identity: %v", err)
		return nil
	}

	_, pkgLoginPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		tb.Fatalf("testharness: ClientFor: PKG login keygen: %v", err)
		return nil
	}

	tc := &TestClient{
		Username:       username,
		HybridIdentity: id,
		recvCh:         make(chan IncomingMessage, 16),
		friendReqCh:    make(chan *neverlur.IncomingFriendRequest, 16),
		sentReqCh:      make(chan *neverlur.OutgoingFriendRequest, 16),
		confirmedCh:    make(chan *neverlur.Friend, 16),
		errCh:          make(chan error, 16),
	}
	tc.RecvCh = tc.recvCh

	clientDir := tb.TempDir()
	c := &neverlur.Client{
		Username:            username,
		LongTermPublicKey:   id.EdPub,
		LongTermPrivateKey:  id.EdPriv,
		PKGLoginKey:         pkgLoginPriv,
		ConfigClient:        h.neverlurCfgClient,
		Handler:             &testClientHandler{tc: tc},
		ClientPersistPath:   filepath.Join(clientDir, "client"),
		KeywheelPersistPath: filepath.Join(clientDir, "keywheel"),
	}
	tc.Client = c

	// Bootstrap configs the client knows about.
	if err := bootstrapClient(c, h.AddFriendConfig, h.DialingConfig); err != nil {
		tb.Fatalf("testharness: ClientFor: bootstrap: %v", err)
		return nil
	}

	// Register with the harness's PKG. Token can be empty; harness
	// PKG accepts everything.
	pkgServer := h.NeverlurPKG.PKG.PublicServerConfig
	if err := c.Register(pkgServer, ""); err != nil {
		tb.Fatalf("testharness: ClientFor: Register: %v", err)
		return nil
	}

	return tc
}

// bootstrapClient injects the AddFriend + Dialing SignedConfigs into
// the client's local state without going through the on-disk persist
// path. Real production clients fetch these via the ConfigClient on
// first Connect; for tests we inject them directly so the integration
// test doesn't have to wait for the first round.
//
// Phase 3 may need to extend this to also bootstrap the Convo config
// for the Gjallarhorn-side client wiring.
func bootstrapClient(c *neverlur.Client, addFriend, dialing *config.SignedConfig) error {
	// neverlur.Client doesn't expose a direct config-injection API;
	// the ConfigClient's CurrentConfig() reads from the harness's
	// in-memory config server, so the standard ConnectAddFriend /
	// ConnectDialing methods will pick them up. No explicit injection
	// needed.
	_ = addFriend
	_ = dialing
	return nil
}

// SendMessage validates a message at the input boundary (length,
// UTF-8) and submits it through the active Conversation. Matches the
// demo CLI's validation surface.
//
// TODO(testharness-impl): once ConvoStateAccessor is wired with a
// real implementation (Phase 3), this calls into Conversation.Seal +
// the onion submission path. Today it validates inputs but errors
// with scaffoldedNotImplemented if the conversation state is nil.
func (tc *TestClient) SendMessage(body []byte) error {
	if len(body) == 0 {
		return wrap("SendMessage", errEmptyMessage)
	}
	if !utf8.Valid(body) {
		return wrap("SendMessage", errInvalidUTF8)
	}
	// TODO: enforce convo.ConvoMessageSize cap once we have a stable
	// reference (it's defined in gjallarhorn/convo).
	if tc.ConvoState == nil {
		return scaffoldedNotImplemented("SendMessage (no active conversation)")
	}
	return scaffoldedNotImplemented("SendMessage")
}

// IncomingFriendRequestCh returns the channel of incoming friend
// requests (Bob's view of an alice-initiated friend request).
func (tc *TestClient) IncomingFriendRequestCh() <-chan *neverlur.IncomingFriendRequest {
	return tc.friendReqCh
}

// HasFriend returns true if this client has an established friendship
// with the named peer.
func (tc *TestClient) HasFriend(peer string) bool {
	for _, f := range tc.GetFriends() {
		if f.Username == peer {
			return true
		}
	}
	return false
}

// testClientHandler implements neverlur.EventHandler by pushing
// every event into the TestClient's channels.
type testClientHandler struct {
	tc *TestClient
}

func (h *testClientHandler) Error(err error) {
	select {
	case h.tc.errCh <- err:
	default:
	}
}

func (h *testClientHandler) ConfirmedFriend(f *neverlur.Friend) {
	select {
	case h.tc.confirmedCh <- f:
	default:
	}
}

func (h *testClientHandler) SentFriendRequest(r *neverlur.OutgoingFriendRequest) {
	select {
	case h.tc.sentReqCh <- r:
	default:
	}
}

func (h *testClientHandler) ReceivedFriendRequest(r *neverlur.IncomingFriendRequest) {
	select {
	case h.tc.friendReqCh <- r:
	default:
	}
}

func (h *testClientHandler) UnexpectedSigningKey(in *neverlur.IncomingFriendRequest, out *neverlur.OutgoingFriendRequest) {
	// Treat as an error for the test.
	select {
	case h.tc.errCh <- errUnexpectedSigningKey:
	default:
	}
}

func (h *testClientHandler) SendingCall(call *neverlur.OutgoingCall) {
	// TODO(phase-3): wire conversation state from the call's session
	// key into tc.ConvoState (a real implementation of
	// ConvoStateAccessor backed by gjallarhorn.Conversation).
	_ = call
}

func (h *testClientHandler) ReceivedCall(call *neverlur.IncomingCall) {
	// TODO(phase-3): symmetric to SendingCall.
	_ = call
}

func (h *testClientHandler) NewConfig(chain []*config.SignedConfig) {
	// No-op for tests; the test client picks up configs via the
	// ConfigClient on demand.
	_ = chain
}
