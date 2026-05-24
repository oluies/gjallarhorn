// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

import (
	"fmt"
	"sync"
	"testing"

	gcoordinator "github.com/oluies/gjallarhorn/coordinator"
	"github.com/oluies/neverlur/config"
	ncoordinator "github.com/oluies/neverlur/coordinator"
	"github.com/oluies/neverlur/hybrid"
)

// Harness is an in-memory stand-up of the full Neverlur + Gjallarhorn
// service set. See doc.go for status (currently a typed scaffold; the
// server-orchestration internals are TODOs).
type Harness struct {
	// Neverlur-side servers.
	NeverlurCoordinator *ncoordinator.Server
	NeverlurMixers      []NeverlurMixerHandle // see mixers.go
	NeverlurPKG         NeverlurPKGHandle     // see pkg.go
	NeverlurCDN         NeverlurCDNHandle     // see cdn.go (forthcoming)

	// Gjallarhorn-side servers.
	GjallarhornCoordinator *gcoordinator.Server
	GjallarhornMixers      []GjallarhornMixerHandle
	GjallarhornCDN         GjallarhornCDNHandle

	// Bootstrap configs the harness produces at New() time, each signed
	// by HarnessGuardian. v2 (hybrid) format per
	// neverlur/config.SignedConfig with PQKey-bound guardians.
	AddFriendConfig *config.SignedConfig
	DialingConfig   *config.SignedConfig
	ConvoConfig     *config.SignedConfig

	// HarnessGuardian is the single hybrid identity that signs the
	// three configs above. In production each config is signed by
	// multiple independent guardians; for tests one is enough.
	HarnessGuardian *hybrid.HybridIdentity

	// ListenAddr is the Unix-socket address the harness exposes so
	// the Neverlur demo CLI can attach a second process. Empty unless
	// OptionListenAddr was supplied.
	ListenAddr string

	// Implementation-internal.
	closeOnce sync.Once
	tb        testing.TB
	opts      options
	cleanups  []func()
}

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

// Option configures Harness behavior. See OptionListenAddr,
// OptionMixerCount.
type Option func(*options)

// OptionListenAddr makes the harness listen on the given Unix socket
// path so the Neverlur demo CLI can attach a second process. Default:
// no listening (pure in-process test usage).
func OptionListenAddr(path string) Option {
	return func(o *options) {
		o.listenAddr = path
	}
}

// OptionMixerCount overrides the default mixer count (3 each) for
// Neverlur and Gjallarhorn. Useful for stress tests.
func OptionMixerCount(neverlur, gjallarhorn int) Option {
	return func(o *options) {
		o.neverlurMixers = neverlur
		o.gjallarhornMixers = gjallarhorn
	}
}

// New starts every component and returns a ready-to-use Harness.
// Registers tb.Cleanup for teardown; Close() can be called explicitly
// if a non-test caller (like the demo CLI) is consuming.
//
// TODO(testharness-impl): orchestrate the real per-component startup
// per coordinator.go / mixers.go / pkg.go / configs.go. Current
// implementation builds the harness guardian and the three signed
// configs (real) but leaves every server field nil.
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

	// configs.go (real): generate the three v2 SignedConfigs.
	if err := h.buildConfigs(); err != nil {
		tb.Fatalf("testharness: build configs: %v", err)
		return nil
	}

	// TODO(testharness-impl): wire the per-component servers.
	//   h.startNeverlurPKG()
	//   h.startNeverlurCDN()
	//   h.startNeverlurMixers(resolved.neverlurMixers)
	//   h.startNeverlurCoordinator()
	//   h.startGjallarhornCDN()
	//   h.startGjallarhornMixers(resolved.gjallarhornMixers)
	//   h.startGjallarhornCoordinator()
	//   if resolved.listenAddr != "" { h.startListener(resolved.listenAddr) }

	tb.Cleanup(h.Close)
	return h
}

// Close tears down every component cleanly. Idempotent.
//
// TODO(testharness-impl): close the server goroutines, remove tempdirs,
// unlink the listen socket. Current implementation runs the registered
// cleanup callbacks (the only one populated today is the configs.go
// nothing — there's nothing to clean up because nothing was started).
func (h *Harness) Close() {
	h.closeOnce.Do(func() {
		for i := len(h.cleanups) - 1; i >= 0; i-- {
			h.cleanups[i]()
		}
	})
}

// scaffoldedNotImplemented is the error returned by methods that are
// part of the scaffold but not yet wired. Callers see a clear message
// pointing at the doc.go status section.
func scaffoldedNotImplemented(method string) error {
	return fmt.Errorf("testharness.%s: scaffold-only; see internal/testharness/doc.go status section", method)
}
