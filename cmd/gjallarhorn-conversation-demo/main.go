// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

// gjallarhorn-conversation-demo boots an in-process Gjallarhorn +
// Neverlur universe via the testharness and walks a scripted Alice ↔
// Bob bringup. It is a human-facing diagnostic / demo binary, not
// an automated test — its job is to show the full pipeline to
// somebody who wants to understand the wiring before reading the
// e2e test code.
//
// Why a binary rather than another test? Because the testharness
// runs many in-process servers and websockets and the failure modes
// are visual (which mixer didn't ACK a round, which coordinator
// timed out). A standalone binary surfaces that as live stdout. The
// automated tests in e2e/ check assertions but suppress the noise.
//
// Constitutional note: this binary is a development/debug tool only.
// It never ships in a production release artifact.
//
// Usage:
//
//	go run ./cmd/gjallarhorn-conversation-demo
//
// What it does:
//
//  1. Boots a fresh testharness.Harness (config server + AddFriend
//     coordinator + Dialing coordinator + Convo coordinator + PKG +
//     CDN + Neverlur mixchain + Gjallarhorn convo mixchain).
//
//  2. Creates two TestClients with fresh hybrid identities (Alice
//     and Bob) and registers them with the harness PKG.
//
//  3. Prints the state of the world (config service names, guardian
//     binding status, mixchain composition, hybrid-identity binding
//     for each client).
//
//  4. Calls Start() on both clients to bring up AddFriend + Dialing
//     + Convo websockets.
//
//  5. Drives a real scripted conversation:
//     a. alice.SendFriendRequest(bob.Username, nil)
//     b. bob receives, calls Approve()
//     c. wait for ConfirmedFriend on both sides
//     d. alice.GetFriend(bob).Call(0) to bootstrap conversation
//     e. wait for ConvoState bootstrapped on both sides
//     f. alice.SendMessage("hello bob ...")
//     g. wait for bob.RecvCh; assert plaintext matches
//
//  6. Prints success, shuts down cleanly via deferred Stop+Close.
//
// Total runtime: ~60-120 seconds depending on coordinator round
// timings (AddFriend RoundWait=2s, Dialing RoundWait=2s, Convo
// RoundDelay=2s; each handshake needs multiple rounds).
//
// Limits:
//
//   - bn256-related limitations apply: must be run on linux/amd64.
//     Apple Silicon builds will fail in the inherited bls/bn256
//     assembly until that's swapped out.
package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/oluies/gjallarhorn/internal/testharness"
)

// stdoutTB satisfies testharness.TB (Helper / TempDir / Cleanup /
// Fatalf), the narrow interface the harness consumes. Routes Fatalf
// to log.Fatalf so the deferred cleanups still fire via Cleanup().
type stdoutTB struct {
	mu       sync.Mutex
	cleanups []func()
}

func (s *stdoutTB) Helper() {}

func (s *stdoutTB) TempDir() string {
	dir, err := os.MkdirTemp("", "gjallarhorn-demo-")
	if err != nil {
		s.Fatalf("MkdirTemp: %v", err)
	}
	s.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func (s *stdoutTB) Cleanup(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanups = append(s.cleanups, fn)
}

func (s *stdoutTB) Fatalf(format string, args ...any) {
	s.runCleanups()
	log.Fatalf(format, args...)
}

func (s *stdoutTB) runCleanups() {
	s.mu.Lock()
	cleanups := s.cleanups
	s.cleanups = nil
	s.mu.Unlock()
	for i := len(cleanups) - 1; i >= 0; i-- {
		cleanups[i]()
	}
}

// Compile-time assertion.
var _ testharness.TB = (*stdoutTB)(nil)

func main() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	log.Print("==> gjallarhorn-conversation-demo: booting in-process universe")

	tb := &stdoutTB{}
	defer tb.runCleanups()

	h := testharness.New(tb)
	log.Print("==> harness up")
	log.Printf("    AddFriend service: %s", h.AddFriendConfig.Service)
	log.Printf("    Dialing service:   %s", h.DialingConfig.Service)
	log.Printf("    Convo service:     %s", h.ConvoConfig.Service)

	if err := h.HarnessGuardian.VerifyBinding(); err != nil {
		log.Fatalf("harness guardian binding: %v", err)
	}
	log.Print("    Harness guardian hybrid binding: OK (Ed25519 + ML-DSA-65 + ML-KEM-768)")

	log.Print("==> creating Alice and Bob")
	alice := h.ClientFor(tb, "alice@gjallarhorn-demo.test")
	bob := h.ClientFor(tb, "bob@gjallarhorn-demo.test")
	if err := alice.HybridIdentity.VerifyBinding(); err != nil {
		log.Fatalf("alice binding: %v", err)
	}
	if err := bob.HybridIdentity.VerifyBinding(); err != nil {
		log.Fatalf("bob binding: %v", err)
	}
	log.Printf("    Alice ed25519 pub: %x...", alice.HybridIdentity.EdPub[:8])
	log.Printf("    Bob   ed25519 pub: %x...", bob.HybridIdentity.EdPub[:8])

	log.Print("==> starting both clients (AddFriend + Dialing + Convo websockets)")
	aliceDisc, err := alice.Start()
	if err != nil {
		log.Fatalf("alice.Start: %v", err)
	}
	bobDisc, err := bob.Start()
	if err != nil {
		log.Fatalf("bob.Start: %v", err)
	}
	defer alice.Stop()
	defer bob.Stop()
	go drainDisconnects("alice", aliceDisc)
	go drainDisconnects("bob", bobDisc)

	log.Print("==> alice -> bob: SendFriendRequest")
	if _, err := alice.SendFriendRequest(bob.Username, nil); err != nil {
		log.Fatalf("SendFriendRequest: %v", err)
	}

	log.Print("==> waiting for bob to receive friend request (up to 60s)")
	select {
	case req := <-bob.IncomingFriendRequestCh():
		log.Printf("    bob received from: %s", req.Username)
		log.Print("==> bob: Approve")
		if _, err := req.Approve(); err != nil {
			log.Fatalf("Approve: %v", err)
		}
	case <-time.After(60 * time.Second):
		log.Fatal("bob never received friend request within 60s")
	}

	log.Print("==> waiting for both sides to see ConfirmedFriend (up to 60s)")
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if alice.HasFriend(bob.Username) && bob.HasFriend(alice.Username) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !alice.HasFriend(bob.Username) || !bob.HasFriend(alice.Username) {
		log.Fatalf("friendship not confirmed within 60s (alice has bob=%v, bob has alice=%v)",
			alice.HasFriend(bob.Username), bob.HasFriend(alice.Username))
	}
	log.Print("    friendship confirmed on both sides")

	log.Print("==> alice: dial bob (intent 0)")
	f := alice.GetFriend(bob.Username)
	if f == nil {
		log.Fatalf("alice has no friend object for bob despite HasFriend=true")
	}
	_ = f.Call(0)

	log.Print("==> waiting for ConvoState to bootstrap on both sides (up to 60s)")
	deadline = time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		if alice.ConvoState != nil && bob.ConvoState != nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if alice.ConvoState == nil || bob.ConvoState == nil {
		log.Fatalf("ConvoState not bootstrapped within 60s (alice=%v bob=%v)",
			alice.ConvoState != nil, bob.ConvoState != nil)
	}
	log.Printf("    alice ConvoState round=%d peer=%s", alice.ConvoState.Round(), alice.ConvoState.PeerUsername())
	log.Printf("    bob   ConvoState round=%d peer=%s", bob.ConvoState.Round(), bob.ConvoState.PeerUsername())

	body := []byte("hello bob — sealed via NaCl secretbox with key derived from hybrid X25519+ML-KEM-768 keywheel")
	log.Printf("==> alice: SendMessage (%d bytes)", len(body))
	if err := alice.SendMessage(body); err != nil {
		log.Fatalf("SendMessage: %v", err)
	}

	log.Print("==> waiting for bob to receive (up to 30s)")
	select {
	case msg := <-bob.RecvCh:
		if !bytes.Equal(msg.Body, body) {
			log.Fatalf("body mismatch: got %q want %q", msg.Body, body)
		}
		log.Printf("    bob received round=%d from=%s body=%q", msg.Round, msg.FromPeer, msg.Body)
	case <-time.After(30 * time.Second):
		log.Fatal("bob never received the message")
	}

	fmt.Println()
	fmt.Println("==> demo: full pipeline COMPLETE")
	fmt.Println("    add-friend handshake → dial → first-message round-trip succeeded")
	fmt.Println("    Session key derived from hybrid combiner (X25519 + ML-KEM-768)")
	fmt.Println()

	log.Print("==> shutting down")
}

// drainDisconnects logs any per-service websocket close events.
// Each TestClient.Start() returns a channel that fires when one of
// the three (AddFriend / Dialing / Convo) websockets drops; in the
// demo we just log them and keep going.
func drainDisconnects(who string, ch <-chan testharness.ConnectError) {
	for ce := range ch {
		log.Printf("    [%s] %s websocket disconnected: %v", who, ce.Service, ce.Err)
	}
}
