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
//  4. Holds for 5 seconds so the user can observe coordinator log
//     output, then shuts down cleanly.
//
// Limits:
//
//   - Does NOT (yet) run a real add-friend / dial / message round-
//     trip. The harness end-to-end path is blocked on the same two
//     upstream items as e2e/first_message_test.go — coordinator data
//     race fix + TestClient.Start() helper. When those land, this
//     binary will grow a "step 5: scripted conversation" section.
//
//   - bn256-related limitations apply: must be run on linux/amd64.
//     Apple Silicon builds will fail in the inherited bls/bn256
//     assembly until that's swapped out.
package main

import (
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

	fmt.Println()
	fmt.Println("==> demo: harness bring-up + dual hybrid-identity registration COMPLETE")
	fmt.Println()
	fmt.Println("    Next step (blocked, see e2e/first_message_test.go for unblocking criteria):")
	fmt.Println("    Drive a real add-friend → dial → first-message round-trip.")
	fmt.Println()

	log.Print("==> holding 5s so coordinator logs can stream...")
	time.Sleep(5 * time.Second)
	log.Print("==> shutting down")
}
