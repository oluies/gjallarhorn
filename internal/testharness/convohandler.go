// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

import (
	cryptorand "crypto/rand"
	"sync"

	"github.com/oluies/gjallarhorn/convo"
	nconfig "github.com/oluies/neverlur/config"
)

// testClientConvoHandler implements gjallarhorn.ConvoHandler. It
// drives one conversation at a time per TestClient — the harness is
// scoped to 1:1 alice/bob style tests, not group chat.
//
// The handler bridges the outgoing queue (populated by TestClient.
// SendMessage) into the per-round Outgoing() callback that the
// gjallarhorn.Client invokes when it needs onion payloads for the
// next round. Cover traffic is emitted when there is no queued
// message or the conversation state is not yet bootstrapped.
type testClientConvoHandler struct {
	tc *TestClient

	mu       sync.Mutex
	outQueue [][]byte
}

// Outgoing returns one DeadDropMessage per Outgoing call. If a real
// body is queued, seal it via the active ConvoState; otherwise emit
// cover traffic (a random dead-drop with random ciphertext) so the
// observer cannot distinguish whether the client has anything to say.
func (h *testClientConvoHandler) Outgoing(round uint32) []*convo.DeadDropMessage {
	h.mu.Lock()
	state, _ := h.tc.ConvoState.(*harnessConvoState)
	var body []byte
	if state != nil && len(h.outQueue) > 0 {
		body = h.outQueue[0]
		h.outQueue = h.outQueue[1:]
	}
	h.mu.Unlock()

	if state == nil || body == nil {
		return []*convo.DeadDropMessage{coverDeadDrop()}
	}

	// Pad to SizeMessageBody.
	padded := make([]byte, convo.SizeMessageBody)
	copy(padded, body)

	ctxt, ok := state.Seal(padded, round)
	if !ok {
		return []*convo.DeadDropMessage{coverDeadDrop()}
	}
	dd, ok := state.DeadDrop(round)
	if !ok {
		return []*convo.DeadDropMessage{coverDeadDrop()}
	}
	msg := &convo.DeadDropMessage{DeadDrop: dd}
	copy(msg.EncryptedMessage[:], ctxt)
	return []*convo.DeadDropMessage{msg}
}

// Replies receives the per-round dead-drop candidate ciphertexts the
// mixnet returned to our slot. Each candidate is trial-decrypted
// against the active ConvoState; the first that opens cleanly is
// pushed on tc.RecvCh.
func (h *testClientConvoHandler) Replies(round uint32, messages [][]byte) {
	state, _ := h.tc.ConvoState.(*harnessConvoState)
	if state == nil {
		return
	}
	for _, ct := range messages {
		pt, ok := state.Open(ct, round)
		if !ok {
			continue
		}
		// Strip trailing zero-padding to the first NUL byte for
		// printable test fixtures. (Production would carry a length
		// prefix; the harness keeps it simple.)
		trimmed := pt
		for i, b := range pt {
			if b == 0 {
				trimmed = pt[:i]
				break
			}
		}
		select {
		case h.tc.recvCh <- IncomingMessage{
			FromPeer: state.PeerUsername(),
			Body:     trimmed,
			Round:    round,
		}:
		default:
		}
		return
	}
}

func (h *testClientConvoHandler) NewConfig(chain []*nconfig.SignedConfig) {
	_ = chain
}

func (h *testClientConvoHandler) Error(err error) {
	select {
	case h.tc.errCh <- err:
	default:
	}
}

func (h *testClientConvoHandler) DebugError(err error) {
	_ = err
}

func (h *testClientConvoHandler) GlobalAnnouncement(message string) {
	_ = message
}

// coverDeadDrop returns a DeadDropMessage filled with cryptographic
// noise. Indistinguishable to an observer from real traffic.
func coverDeadDrop() *convo.DeadDropMessage {
	dd := &convo.DeadDropMessage{}
	_, _ = cryptorand.Read(dd.DeadDrop[:])
	_, _ = cryptorand.Read(dd.EncryptedMessage[:])
	return dd
}
