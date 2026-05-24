// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

import (
	"crypto/ed25519"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/davidlazar/go-crypto/encoding/base32"

	gcoordinator "github.com/oluies/gjallarhorn/coordinator"
	"github.com/oluies/neverlur/config"
	ncoordinator "github.com/oluies/neverlur/coordinator"
	"github.com/oluies/neverlur/hybrid"
	"github.com/oluies/neverlur/pqsig"
)

// Harness is an in-memory stand-up of the full Neverlur + Gjallarhorn
// service set. See doc.go for the design narrative.
type Harness struct {
	// Neverlur-side servers.
	NeverlurCoordinator *ncoordinator.Server     // AddFriend coordinator (primary)
	NeverlurMixers      []NeverlurMixchainHandle // 1 element: the shared mixchain
	NeverlurPKG         NeverlurPKGHandle        // single PKG
	NeverlurCDN         NeverlurCDNHandle        // single CDN

	// Gjallarhorn-side servers.
	GjallarhornCoordinator *gcoordinator.Server            // Convo coordinator
	GjallarhornConvoMixers *GjallarhornConvoMixchainHandle // Convo mixchain

	// Bootstrap configs the harness produces at New() time, each
	// signed by HarnessGuardian (v2 hybrid format).
	AddFriendConfig *config.SignedConfig
	DialingConfig   *config.SignedConfig
	ConvoConfig     *config.SignedConfig

	// HarnessGuardian is the single hybrid identity that signs the
	// three configs above.
	HarnessGuardian *hybrid.HybridIdentity

	// ListenAddr is the Unix-socket address the harness exposes so
	// the Neverlur demo CLI can attach a second process. Empty unless
	// OptionListenAddr was supplied.
	ListenAddr string

	// Internal cross-side wiring populated by startNeverlurSide and
	// consumed by startGjallarhornSide + ClientFor.
	neverlurCoordKey  ed25519.PublicKey
	neverlurCoordAddr string
	neverlurCfgClient *config.Client
	neverlurCfgServer *config.Server

	closeOnce sync.Once
	tb        testing.TB
	opts      options
	cleanups  []func()
}

// errNeverlurNotStarted indicates startGjallarhornSide was called
// before startNeverlurSide populated the shared config plumbing.
var errNeverlurNotStarted = errors.New("internal error: Gjallarhorn side initialized before Neverlur side")

// options captures the resolved Option settings.
type options struct {
	listenAddr        string
	neverlurMixers    int
	gjallarhornMixers int
}

// defaultOptions returns the options used when no Option is supplied.
func defaultOptions() options {
	return options{
		neverlurMixers:    3,
		gjallarhornMixers: 3,
	}
}

// Option configures Harness behavior.
type Option func(*options)

// OptionListenAddr makes the harness listen on the given Unix socket
// path so the Neverlur demo CLI can attach a second process. Default:
// no listening.
func OptionListenAddr(path string) Option {
	return func(o *options) {
		o.listenAddr = path
	}
}

// OptionMixerCount overrides the default mixer count (3 each) for
// Neverlur and Gjallarhorn.
func OptionMixerCount(neverlur, gjallarhorn int) Option {
	return func(o *options) {
		o.neverlurMixers = neverlur
		o.gjallarhornMixers = gjallarhorn
	}
}

// New starts every component and returns a ready-to-use Harness.
// Registers tb.Cleanup for teardown; Close() can be called explicitly
// if a non-test caller is consuming.
//
// New panics via tb.Fatal if any component fails to start.
func New(tb testing.TB, opts ...Option) *Harness {
	tb.Helper()

	resolved := defaultOptions()
	for _, opt := range opts {
		opt(&resolved)
	}

	id, err := hybrid.GenerateHybridIdentity()
	if err != nil {
		tb.Fatalf("testharness: generate harness guardian: %v", err)
		return nil
	}
	if err := id.VerifyBinding(); err != nil {
		tb.Fatalf("testharness: harness guardian binding: %v", err)
		return nil
	}

	h := &Harness{
		HarnessGuardian: id,
		ListenAddr:      resolved.listenAddr,
		tb:              tb,
		opts:            resolved,
	}

	tmpDir := tb.TempDir()

	if err := h.startNeverlurSide(tmpDir); err != nil {
		tb.Fatalf("testharness: startNeverlurSide: %v", err)
		return nil
	}
	if err := h.startGjallarhornSide(tmpDir); err != nil {
		tb.Fatalf("testharness: startGjallarhornSide: %v", err)
		return nil
	}

	// TODO(testharness-impl): if resolved.listenAddr is set, expose
	// a Unix-socket endpoint for the Neverlur demo CLI to attach.
	// Currently a no-op; demo CLI integration is Phase 6.

	tb.Cleanup(h.Close)
	return h
}

// Close tears down every component in reverse order. Idempotent.
func (h *Harness) Close() {
	h.closeOnce.Do(func() {
		for i := len(h.cleanups) - 1; i >= 0; i-- {
			h.cleanups[i]()
		}
	})
}

// signConfigInPlace signs the given v2 SignedConfig with the harness
// guardian, populating its Guardians and Signatures fields. Called by
// startNeverlurSide and startGjallarhornSide.
func (h *Harness) signConfigInPlace(conf *config.SignedConfig) error {
	g := config.Guardian{
		Username: "harness-guardian",
		Key:      h.HarnessGuardian.EdPub,
		PQKey:    pqsig.PackPublicKey(h.HarnessGuardian.PQPub),
	}
	conf.Guardians = []config.Guardian{g}
	conf.Signatures = map[string][]byte{}

	msg := conf.SigningMessage()
	sigEd := ed25519.Sign(h.HarnessGuardian.EdPriv, msg)
	sigPQ, err := pqsig.Sign(h.HarnessGuardian.PQPriv, msg)
	if err != nil {
		return wrap("signConfigInPlace pqsig.Sign", err)
	}
	var hs config.HybridSignature
	copy(hs.Ed[:], sigEd)
	copy(hs.PQ[:], sigPQ)
	conf.Signatures[base32.EncodeToString(g.Key)] = hs.Bytes()
	return nil
}

// scaffoldedNotImplemented is retained for stubbed methods (currently
// only rounds.go::AdvanceRound) until their real implementation lands.
func scaffoldedNotImplemented(method string) error {
	return fmt.Errorf("testharness.%s: not yet implemented (deterministic round driving deferred)", method)
}

// wrap is the harness-internal error wrapping helper.
func wrap(stage string, err error) error {
	return fmt.Errorf("testharness: %s: %w", stage, err)
}
