// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

import (
	"fmt"
	"time"
)

// AdvanceRound is retained for backward compatibility with earlier
// scaffolding. It does NOT drive rounds — the coordinators run a
// natural loop and their per-round delays are small (set to 1-2s
// by the harness bringup). For deterministic per-round assertions
// in tests, use WaitForConvoRound on a TestClient instead.
//
// Returns nil for known services so existing callers don't break.
// Returns an error for unknown services so typos surface.
//
// Rationale (per spec 001-conversation-wiring research R3): adding
// a public Tick() entry-point to coordinator.Server would require
// changes to Neverlur + Gjallarhorn coordinator packages plus a
// constitutional review (the dialing-round monotonicity invariant
// is part of the threat model). For the integration-test surface
// we get the same determinism by polling LatestRound on the convo
// client, which is read-only and side-effect free.
func (h *Harness) AdvanceRound(service string) error {
	switch service {
	case "AddFriend", "Dialing", "Convo":
		return nil
	default:
		return fmt.Errorf("testharness.AdvanceRound: unknown service %q (want AddFriend|Dialing|Convo)", service)
	}
}

// WaitForConvoRound blocks until the underlying convo client
// observes a coordinator round >= target, or the timeout elapses.
// Returns the observed round and any timeout error.
//
// Tests use this after SendMessage to ensure the message has had
// at least one round to propagate through the Convo mixchain and
// arrive at the peer.
func (tc *TestClient) WaitForConvoRound(target uint32, timeout time.Duration) (uint32, error) {
	if tc.ConvoClient == nil {
		return 0, fmt.Errorf("testharness: WaitForConvoRound: convo client not initialized")
	}
	deadline := time.Now().Add(timeout)
	for {
		got := tc.ConvoClient.LatestRound()
		if got >= target {
			return got, nil
		}
		if time.Now().After(deadline) {
			return got, fmt.Errorf("testharness: WaitForConvoRound: timeout waiting for round %d (latest=%d)", target, got)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
