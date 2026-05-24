// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

import (
	gmixnet "github.com/oluies/gjallarhorn/mixnet"
	"github.com/oluies/neverlur/mock"
)

// Per-component handle types wrap the real server objects so test
// consumers can inspect endpoints.

// NeverlurMixchainHandle wraps the Neverlur-side mixchain (3 mixers
// configured with AddFriend + Dialing services from
// neverlur/mock.LaunchMixchain).
type NeverlurMixchainHandle struct {
	Mixchain *mock.Mixchain
}

// NeverlurPKGHandle wraps one Neverlur-side PKG instance.
type NeverlurPKGHandle struct {
	PKG *mock.PKG
}

// NeverlurCDNHandle wraps the Neverlur-side CDN instance.
type NeverlurCDNHandle struct {
	CDN *mock.CDN
}

// GjallarhornConvoMixchainHandle wraps the Gjallarhorn-side mixchain
// (mixers configured with the Convo service). Built by
// launchConvoMixchain in gjallarhorn.go.
type GjallarhornConvoMixchainHandle struct {
	Servers    []gmixnet.PublicServerConfig
	mixServers []*gmixnet.Server
	closers    []func() error
}

// Close stops every Convo-mixchain mixer.
func (h *GjallarhornConvoMixchainHandle) Close() error {
	var firstErr error
	for _, fn := range h.closers {
		if err := fn(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
