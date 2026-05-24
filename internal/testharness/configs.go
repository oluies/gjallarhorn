// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

import (
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/davidlazar/go-crypto/encoding/base32"

	"github.com/oluies/neverlur/config"
	"github.com/oluies/neverlur/hybrid"
	"github.com/oluies/neverlur/pqsig"
)

// buildConfigs generates the three v2 SignedConfig records the harness
// needs (AddFriend, Dialing, Convo), each signed by the
// HarnessGuardian with both Ed25519 and ML-DSA-65 halves.
//
// This is real implementation, NOT scaffold: it uses neverlur/config's
// v2 schema, the hybrid identity's R4-derived PQ key, and the
// HybridSignature codec to produce records that would verify against
// neverlur/config.Verify.
//
// The Inner field of each config is left as a sentinel placeholder
// (trivialInner) for now — the real harness needs concrete
// AddFriendConfig / DialingConfig / ConvoConfig values that reference
// the harness's component endpoints (coordinator addr, PKG addr,
// mixer keys, CDN addr). Those are populated when the corresponding
// per-component init code lands; see TODOs in harness.go.
func (h *Harness) buildConfigs() error {
	addFriend, err := h.signHarnessConfig("AddFriend")
	if err != nil {
		return fmt.Errorf("AddFriend config: %w", err)
	}
	h.AddFriendConfig = addFriend

	dialing, err := h.signHarnessConfig("Dialing")
	if err != nil {
		return fmt.Errorf("Dialing config: %w", err)
	}
	h.DialingConfig = dialing

	convo, err := h.signHarnessConfig("Convo")
	if err != nil {
		return fmt.Errorf("Convo config: %w", err)
	}
	h.ConvoConfig = convo

	return nil
}

// signHarnessConfig produces one v2 SignedConfig with the harness
// guardian as its sole Guardian (both Ed25519 and ML-DSA-65 halves
// populated) and a hybrid signature over a placeholder inner.
//
// The Created/Expires window is 1h around now; tests advance rounds
// manually so the real elapsed time doesn't matter for delivery.
func (h *Harness) signHarnessConfig(service string) (*config.SignedConfig, error) {
	g := config.Guardian{
		Username: "harness-guardian",
		Key:      h.HarnessGuardian.EdPub,
		PQKey:    pqsig.PackPublicKey(h.HarnessGuardian.PQPub),
	}

	now := time.Now().UTC().Round(0)
	conf := &config.SignedConfig{
		Version:          config.SignedConfigVersion,
		MinClientVersion: config.SignedConfigVersion,
		Service:          service,
		Created:          now,
		Expires:          now.Add(1 * time.Hour),
		Inner:            harnessInner{Service: service},
		Guardians:        []config.Guardian{g},
		Signatures:       map[string][]byte{},
	}

	// Sign the canonical signing message with both halves of the
	// harness guardian.
	msg := conf.SigningMessage()
	sigEd := ed25519.Sign(h.HarnessGuardian.EdPriv, msg)
	sigPQ, err := pqsig.Sign(h.HarnessGuardian.PQPriv, msg)
	if err != nil {
		return nil, fmt.Errorf("pqsig.Sign: %w", err)
	}
	var hs config.HybridSignature
	copy(hs.Ed[:], sigEd)
	copy(hs.PQ[:], sigPQ)
	conf.Signatures[base32.EncodeToString(g.Key)] = hs.Bytes()

	return conf, nil
}

// harnessInner is a minimal config.InnerConfig stand-in used by the
// harness scaffold. Real Phase A wiring replaces this with the
// concrete AddFriendConfig / DialingConfig / ConvoConfig types from
// the inherited code, populated with the harness's component
// endpoints. See TODOs in harness.go.
type harnessInner struct {
	Service string `json:"Service"`
}

// Validate is the InnerConfig interface satisfied; trivially accepts.
func (h harnessInner) Validate() error { return nil }

// UseLatestVersion is the InnerConfig interface; no-op for the
// scaffold (the real config types implement an internal version bump
// per service).
func (h harnessInner) UseLatestVersion() {}

// Compile-time check that harnessInner satisfies config.InnerConfig.
var _ config.InnerConfig = harnessInner{}

// suppress unused-import warnings when the file is read in isolation;
// real wiring will import these directly.
var _ = hybrid.GenerateHybridIdentity
