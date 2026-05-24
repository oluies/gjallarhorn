// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

import (
	"testing"

	"github.com/oluies/neverlur/config"
)

// TestHarnessBringup confirms the full harness stand-up succeeds: the
// harness guardian's binding holds, all three v2 SignedConfigs are
// present and self-verify, and both the Neverlur and Gjallarhorn
// coordinators are populated (non-nil) after New() returns.
//
// Runs in CI on linux/amd64. Cannot run on arm64 (transitively
// pulls vuvuzela.io/crypto/bn256 via bls).
func TestHarnessBringup(t *testing.T) {
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

	// Server fields must be populated after bringup.
	if h.NeverlurCoordinator == nil {
		t.Error("NeverlurCoordinator is nil after New (should be running)")
	}
	if h.GjallarhornCoordinator == nil {
		t.Error("GjallarhornCoordinator is nil after New (should be running)")
	}
	if len(h.NeverlurMixers) == 0 {
		t.Error("NeverlurMixers is empty (expected 1 mixchain handle)")
	}
	if h.NeverlurPKG.PKG == nil {
		t.Error("NeverlurPKG.PKG is nil")
	}
	if h.NeverlurCDN.CDN == nil {
		t.Error("NeverlurCDN.CDN is nil")
	}
	if h.GjallarhornConvoMixers == nil || len(h.GjallarhornConvoMixers.Servers) == 0 {
		t.Error("GjallarhornConvoMixers has no servers")
	}
}

// TestHarnessClientFor confirms ClientFor produces a TestClient with
// a real *neverlur.Client and a freshly bound hybrid identity. Does
// NOT exercise the full registration round-trip end-to-end (that's
// Phase 3); just confirms the client is constructable.
func TestHarnessClientFor(t *testing.T) {
	h := New(t)
	tc := h.ClientFor(t, "alice@harness.test")
	if tc == nil {
		t.Fatal("ClientFor returned nil")
	}
	if tc.Client == nil {
		t.Fatal("TestClient.Client is nil")
	}
	if tc.Username != "alice@harness.test" {
		t.Errorf("Username = %q, want alice@harness.test", tc.Username)
	}
	if tc.HybridIdentity == nil {
		t.Fatal("HybridIdentity is nil")
	}
	if err := tc.HybridIdentity.VerifyBinding(); err != nil {
		t.Errorf("HybridIdentity binding: %v", err)
	}
}

// TestAdvanceRoundUnknownService asserts AdvanceRound errors
// distinguishably for an unknown service name.
func TestAdvanceRoundUnknownService(t *testing.T) {
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
