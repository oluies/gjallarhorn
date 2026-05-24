// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

import (
	"bytes"
	"crypto/rand"
	"testing"

	"github.com/oluies/gjallarhorn/convo"
)

// TestConvoStateRoundTrip confirms two harnessConvoStates seeded with
// the same key + round produce matching ciphertexts/plaintexts when
// they treat each other as peers. This is the core conversation
// round-trip invariant.
func TestConvoStateRoundTrip(t *testing.T) {
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	const seedRound = uint32(1000)

	alice := newHarnessConvoState(&seed, seedRound, "alice@harness.test", "bob@harness.test")
	bob := newHarnessConvoState(&seed, seedRound, "bob@harness.test", "alice@harness.test")

	// alice -> bob across several rounds.
	for _, round := range []uint32{1001, 1042, 1500, 2500} {
		plain := make([]byte, convo.SizeMessageBody)
		copy(plain, []byte("hello round X"))

		ct, ok := alice.Seal(plain, round)
		if !ok {
			t.Fatalf("alice.Seal round=%d: rollKey returned nil", round)
		}

		got, ok := bob.Open(ct, round)
		if !ok {
			t.Fatalf("bob.Open round=%d: decryption failed", round)
		}
		if !bytes.Equal(got, plain) {
			t.Fatalf("round=%d plaintext mismatch", round)
		}

		// Dead-drop IDs must match between peers (both derive from
		// the same per-round key).
		addA, okA := alice.DeadDrop(round)
		addB, okB := bob.DeadDrop(round)
		if !okA || !okB {
			t.Fatalf("round=%d DeadDrop unavailable", round)
		}
		if addA != addB {
			t.Fatalf("round=%d DeadDrop mismatch", round)
		}
	}
}

// TestConvoStateRejectsPastRound confirms rollKeyTo(target<seed)
// returns nil — once a key has rolled forward we cannot decrypt a
// past round (one-way forward secrecy property).
func TestConvoStateRejectsPastRound(t *testing.T) {
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	const seedRound = uint32(2000)

	s := newHarnessConvoState(&seed, seedRound, "alice@harness.test", "bob@harness.test")
	if _, ok := s.Seal([]byte("x"), 1999); ok {
		t.Errorf("Seal at past round should have failed")
	}
}

// TestConvoStateBadPeerHashFails confirms a wrong peer username
// (in the nonce derivation) makes Open fail — the implicit identity
// binding inside the secretbox nonce.
func TestConvoStateBadPeerHashFails(t *testing.T) {
	var seed [32]byte
	if _, err := rand.Read(seed[:]); err != nil {
		t.Fatal(err)
	}
	const seedRound = uint32(100)
	const round = uint32(150)

	alice := newHarnessConvoState(&seed, seedRound, "alice@harness.test", "bob@harness.test")
	mallory := newHarnessConvoState(&seed, seedRound, "mallory@harness.test", "alice@harness.test")

	plain := make([]byte, convo.SizeMessageBody)
	ct, ok := alice.Seal(plain, round)
	if !ok {
		t.Fatal("Seal failed")
	}
	if _, ok := mallory.Open(ct, round); ok {
		t.Fatalf("Open succeeded against wrong-name nonce (should fail)")
	}
}
