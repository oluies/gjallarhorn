// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

import (
	"crypto/ed25519"
	"crypto/rand"
	"path/filepath"
	"unicode/utf8"

	"github.com/oluies/gjallarhorn"
	"github.com/oluies/gjallarhorn/convo"
	"github.com/oluies/neverlur"
	"github.com/oluies/neverlur/config"
	"github.com/oluies/neverlur/hybrid"
)

// TestClient is the harness-managed wrapper around the dual
// (Neverlur + Gjallarhorn) client pair. The Neverlur client handles
// AddFriend + Dialing; the Gjallarhorn client handles the Convo
// websocket. They are intentionally held as separate fields rather
// than embedded twice to keep the call sites unambiguous.
type TestClient struct {
	*neverlur.Client

	// ConvoClient is the Gjallarhorn-side conversation client. It
	// owns the websocket to the Convo coordinator. Connected lazily
	// via StartConvo() once the AddFriend handshake has bootstrapped
	// the dialing layer.
	ConvoClient *gjallarhorn.Client

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
	// handler callbacks once a call bootstraps; nil before.
	ConvoState ConvoStateAccessor

	// convoHandler routes Outgoing()/Replies()/Error() events from
	// the Gjallarhorn convo client back into this TestClient's
	// channels and outgoing message queue.
	convoHandler *testClientConvoHandler

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
func (h *Harness) ClientFor(tb TB, username string) *TestClient {
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
	tc.convoHandler = &testClientConvoHandler{tc: tc}

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

	// Gjallarhorn-side convo client. Not connected until
	// StartConvo() is called by the test (the websocket only
	// makes sense after AddFriend + dial have run).
	tc.ConvoClient = &gjallarhorn.Client{
		PersistPath:  filepath.Join(clientDir, "convo-client"),
		ConfigClient: h.neverlurCfgClient,
		Handler:      tc.convoHandler,
	}

	return tc
}

// StartConvo connects the underlying Gjallarhorn convo client to the
// Convo coordinator's websocket. Call once after the AddFriend +
// dial handshakes have established a ConvoState. The returned
// channel emits a value (closed) when the websocket disconnects.
func (tc *TestClient) StartConvo() (<-chan error, error) {
	return tc.ConvoClient.ConnectConvo()
}

// Start connects every websocket the TestClient owns: AddFriend +
// Dialing on the Neverlur side, Convo on the Gjallarhorn side. The
// returned Disconnects channel emits per-service errors as each
// websocket eventually closes; callers fail the test on any return
// from Start itself and inspect Disconnects only if a websocket
// drops mid-test.
//
// Idiomatic use:
//
//	disconnects, err := alice.Start()
//	if err != nil { t.Fatalf("alice Start: %v", err) }
//	defer alice.Stop()
//	... test body ...
func (tc *TestClient) Start() (disconnects <-chan ConnectError, err error) {
	out := make(chan ConnectError, 8)

	afCh, err := tc.Client.ConnectAddFriend()
	if err != nil {
		return nil, wrap("Start: ConnectAddFriend", err)
	}
	dlCh, err := tc.Client.ConnectDialing()
	if err != nil {
		_ = tc.Client.CloseAddFriend()
		return nil, wrap("Start: ConnectDialing", err)
	}
	cvCh, err := tc.ConvoClient.ConnectConvo()
	if err != nil {
		_ = tc.Client.CloseAddFriend()
		_ = tc.Client.CloseDialing()
		return nil, wrap("Start: ConnectConvo", err)
	}

	go forwardDisconnect("AddFriend", afCh, out)
	go forwardDisconnect("Dialing", dlCh, out)
	go forwardDisconnect("Convo", cvCh, out)

	return out, nil
}

// Stop closes every websocket Start opened. Idempotent.
func (tc *TestClient) Stop() {
	_ = tc.Client.CloseAddFriend()
	_ = tc.Client.CloseDialing()
	_ = tc.ConvoClient.CloseConvo()
}

// ConnectError pairs a service name with the disconnect error so
// integration tests can pinpoint which side failed.
type ConnectError struct {
	Service string
	Err     error
}

func forwardDisconnect(service string, in <-chan error, out chan<- ConnectError) {
	for err := range in {
		select {
		case out <- ConnectError{Service: service, Err: err}:
		default:
		}
	}
}

// bootstrapClient injects the AddFriend + Dialing SignedConfigs into
// the client's local state. neverlur.Client.Bootstrap requires both
// configs before ConnectAddFriend / ConnectDialing will succeed
// (they check c.addFriendConfig != nil before dialing).
func bootstrapClient(c *neverlur.Client, addFriend, dialing *config.SignedConfig) error {
	return c.Bootstrap(addFriend, dialing)
}

// SendMessage queues a plaintext message for the next outgoing convo
// round. Validates the input boundary (length, UTF-8, max size).
// Returns nil if accepted into the queue; the caller must call
// AdvanceRound (or wait for the natural Convo round tick) to see
// the message actually depart the mixer.
//
// The Convo round in which a queued message ships is driven by the
// gjallarhorn.Client's Outgoing() callback — i.e., the next time
// the Convo coordinator runs a round and asks this client for
// onion content.
func (tc *TestClient) SendMessage(body []byte) error {
	if len(body) == 0 {
		return wrap("SendMessage", errEmptyMessage)
	}
	if !utf8.Valid(body) {
		return wrap("SendMessage", errInvalidUTF8)
	}
	if len(body) > convo.SizeMessageBody {
		return wrap("SendMessage", errMessageTooLarge)
	}
	if tc.ConvoState == nil {
		return wrap("SendMessage", errNoConvoState)
	}

	tc.convoHandler.mu.Lock()
	tc.convoHandler.outQueue = append(tc.convoHandler.outQueue, append([]byte(nil), body...))
	tc.convoHandler.mu.Unlock()
	return nil
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
	seedKey := call.SessionKey()
	seedRound := call.Round()
	if seedKey == nil || seedRound == 0 {
		// Call has not been fully sent yet; the dialing layer
		// will retry. Nothing to bootstrap.
		return
	}
	h.tc.ConvoState = newHarnessConvoState(seedKey, seedRound, h.tc.Username, call.Username)
}

func (h *testClientHandler) ReceivedCall(call *neverlur.IncomingCall) {
	if call.SessionKey == nil || call.Round == 0 {
		return
	}
	h.tc.ConvoState = newHarnessConvoState(call.SessionKey, call.Round, h.tc.Username, call.Username)
}

func (h *testClientHandler) NewConfig(chain []*config.SignedConfig) {
	// No-op for tests; the test client picks up configs via the
	// ConfigClient on demand.
	_ = chain
}
