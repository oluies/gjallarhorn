// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

import (
	"testing"

	"github.com/oluies/neverlur/config"
)

// TestHarnessScaffold confirms the parts of the scaffold that ARE
// implemented (config generation, identity binding) work correctly.
// The parts that are documented TODOs (server orchestration, round
// driving, real client wiring) are NOT exercised here.
//
// This test also runs on arm64 because it doesn't transitively pull
// bn256 — it only touches the harness guardian + signed configs.
//
// Wait — actually it imports neverlur/config which DOES transit bls/bn256
// via mixnet.PublicServerConfig in the inner-config types. So this test
// is linux/amd64-only via CI. The harness package itself can be vetted
// on arm64 (`go vet ./internal/testharness/`) but tests can't run.
func TestHarnessScaffold(t *testing.T) {
	h := New(t)
	if h == nil {
		t.Fatal("New returned nil")
	}

	// HarnessGuardian must be a valid hybrid identity.
	if h.HarnessGuardian == nil {
		t.Fatal("HarnessGuardian is nil")
	}
	if err := h.HarnessGuardian.VerifyBinding(); err != nil {
		t.Fatalf("HarnessGuardian binding: %v", err)
	}

	// All three configs must be present, v2, and self-verifying.
	for _, c := range []*config.SignedConfig{
		h.AddFriendConfig, h.DialingConfig, h.ConvoConfig,
	} {
		if c == nil {
			t.Fatal("config is nil")
		}
		if c.Version != config.SignedConfigVersion {
			t.Errorf("config %q: version %d, want %d", c.Service, c.Version, config.SignedConfigVersion)
		}
		if c.MinClientVersion != config.SignedConfigVersion {
			t.Errorf("config %q: MinClientVersion %d, want %d", c.Service, c.MinClientVersion, config.SignedConfigVersion)
		}
		if err := c.Verify(); err != nil {
			t.Errorf("config %q: Verify: %v", c.Service, err)
		}
	}

	// Scaffold contract: New returns; servers are nil; AdvanceRound
	// errors with the documented scaffold message.
	if h.NeverlurCoordinator != nil {
		t.Error("expected scaffold: NeverlurCoordinator should be nil")
	}
	if h.GjallarhornCoordinator != nil {
		t.Error("expected scaffold: GjallarhornCoordinator should be nil")
	}
	if err := h.AdvanceRound("AddFriend"); err == nil {
		t.Error("expected AdvanceRound to error in scaffold mode")
	}
}

// TestHarnessScaffoldUnknownService asserts AdvanceRound errors
// distinguishably for an unknown service name.
func TestHarnessScaffoldUnknownService(t *testing.T) {
	h := New(t)
	err := h.AdvanceRound("Unknown")
	if err == nil {
		t.Fatal("expected error for unknown service")
	}
	if got := err.Error(); !contains(got, "unknown service") {
		t.Errorf("error did not mention unknown service: %v", err)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
