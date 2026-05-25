// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"sync"

	"golang.org/x/crypto/nacl/secretbox"

	"github.com/oluies/gjallarhorn/convo"
)

// harnessConvoState implements ConvoStateAccessor. It encapsulates a
// conversation's seed (sessionKey, sessionKeyRound) — established at
// dial time and rolled forward for each subsequent conversation round
// — plus the peer/self usernames needed to derive the per-round
// secretbox nonce and dead-drop ID.
//
// All operations are concurrency-safe.
//
// Wire compatibility: the Seal / Open / DeadDrop derivations
// (sha256/sha512 nonce, hmac dead drop, NaCl secretbox) match
// cmd/gjallarhorn-client/conversation.go exactly. Any change here
// breaks the demo CLI's expectations and vice-versa.
type harnessConvoState struct {
	peerUsername string
	myUsername   string

	// Logf is an optional callback for per-operation diagnostics
	// (Seal/Open/DeadDrop). Set by ClientFor when the TestClient
	// has a *testing.T attached; nil-safe — when nil, no logging.
	// Used to debug keywheel-sync issues across alice/bob.
	Logf func(format string, args ...any)

	mu              sync.Mutex
	sessionKey      *[32]byte
	sessionKeyRound uint32
}

func newHarnessConvoState(seedKey *[32]byte, seedRound uint32, myUsername, peerUsername string) *harnessConvoState {
	keyCopy := new([32]byte)
	copy(keyCopy[:], seedKey[:])
	return &harnessConvoState{
		peerUsername:    peerUsername,
		myUsername:      myUsername,
		sessionKey:      keyCopy,
		sessionKeyRound: seedRound,
	}
}

func (c *harnessConvoState) logf(format string, args ...any) {
	if c.Logf == nil {
		return
	}
	c.Logf(format, args...)
}

// SessionKey returns the seed session key. Use rollKeyTo to derive
// a per-round secretbox key for a specific conversation round.
func (c *harnessConvoState) SessionKey() *[32]byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := new([32]byte)
	copy(out[:], c.sessionKey[:])
	return out
}

// Round returns the seed round (the dialing round at which the
// conversation was established).
func (c *harnessConvoState) Round() uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sessionKeyRound
}

// PeerUsername returns the peer of this conversation.
func (c *harnessConvoState) PeerUsername() string {
	return c.peerUsername
}

// rollKeyTo derives the per-round secretbox key for the given convo
// round by rolling forward from the seed (sessionKey, sessionKeyRound).
// Returns nil if targetRound is in the past.
//
// Algorithm matches cmd/gjallarhorn-client/conversation.go rollKey:
// per round r in [seedRound, targetRound): key = sha512_256(r || key).
func (c *harnessConvoState) rollKeyTo(targetRound uint32) *[32]byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.sessionKeyRound > targetRound {
		return nil
	}

	newKey := new([32]byte)
	copy(newKey[:], c.sessionKey[:])

	hash := sha512.New512_256()
	key := newKey[:]
	for r := c.sessionKeyRound; r < targetRound; r++ {
		hash.Reset()
		_ = binary.Write(hash, binary.BigEndian, r)
		hash.Write(key)
		key = hash.Sum(key[:0])
	}

	out := new([32]byte)
	copy(out[:], newKey[:])
	return out
}

// Seal encrypts plaintext for the given convo round using the per-round
// key derived from the seed. Returns ciphertext (variable length, fits
// in convo.SizeEncryptedMessageBody for plaintext up to convo.SizeMessageBody).
func (c *harnessConvoState) Seal(plaintext []byte, round uint32) ([]byte, bool) {
	roundKey := c.rollKeyTo(round)
	if roundKey == nil {
		c.logf("[%s->%s] Seal round=%d FAIL rollKey nil (seedRound=%d)", c.myUsername, c.peerUsername, round, c.sessionKeyRound)
		return nil, false
	}

	var nonce [24]byte
	binary.BigEndian.PutUint32(nonce[:], round)
	nameHash := sha256.Sum256([]byte(c.peerUsername))
	copy(nonce[4:], nameHash[:16])

	ct := secretbox.Seal(nil, plaintext, &nonce, roundKey)
	c.logf("[%s->%s] Seal round=%d roundKey=%x nonce=%x ct[:8]=%x", c.myUsername, c.peerUsername, round, roundKey[:8], nonce[:8], ct[:min(8, len(ct))])
	return ct, true
}

// Open decrypts a ciphertext addressed to this client for the given
// convo round.
func (c *harnessConvoState) Open(ciphertext []byte, round uint32) ([]byte, bool) {
	roundKey := c.rollKeyTo(round)
	if roundKey == nil {
		c.logf("[%s<-%s] Open round=%d FAIL rollKey nil (seedRound=%d)", c.myUsername, c.peerUsername, round, c.sessionKeyRound)
		return nil, false
	}

	var nonce [24]byte
	binary.BigEndian.PutUint32(nonce[:], round)
	nameHash := sha256.Sum256([]byte(c.myUsername))
	copy(nonce[4:], nameHash[:16])

	pt, ok := secretbox.Open(nil, ciphertext, &nonce, roundKey)
	if !ok {
		c.logf("[%s<-%s] Open round=%d FAIL roundKey=%x nonce=%x ct[:8]=%x", c.myUsername, c.peerUsername, round, roundKey[:8], nonce[:8], ciphertext[:min(8, len(ciphertext))])
		return nil, false
	}
	c.logf("[%s<-%s] Open round=%d OK roundKey=%x nonce=%x pt[:8]=%x", c.myUsername, c.peerUsername, round, roundKey[:8], nonce[:8], pt[:min(8, len(pt))])
	return pt, true
}

// DeadDrop derives the dead-drop ID for the given round. Both peers
// of a conversation compute the same DeadDrop because the derivation
// only depends on the shared roundKey (which both derive identically
// from the shared seed).
func (c *harnessConvoState) DeadDrop(round uint32) (convo.DeadDrop, bool) {
	roundKey := c.rollKeyTo(round)
	if roundKey == nil {
		return convo.DeadDrop{}, false
	}
	var id convo.DeadDrop
	h := hmac.New(sha256.New, roundKey[:])
	h.Write([]byte("DeadDrop"))
	_ = binary.Write(h, binary.BigEndian, round)
	r := h.Sum(nil)
	copy(id[:], r)
	c.logf("[%s<->%s] DeadDrop round=%d roundKey=%x drop=%x", c.myUsername, c.peerUsername, round, roundKey[:8], id[:8])
	return id, true
}
