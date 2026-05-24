// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

// This file defines per-component handle types used by Harness. Each
// handle wraps the real server type with the listen address the
// harness assigned and a Close() callback the harness invokes at
// teardown.
//
// Handles are exported so test consumers can inspect the harness's
// component endpoints (e.g. to wire a custom *neverlur.Client whose
// PKG endpoint is the harness's PKG handle's address).
//
// Scaffold status: types are real; the Server field of each is nil
// until the corresponding per-component init code lands. See TODOs in
// harness.go.

// NeverlurMixerHandle wraps one Neverlur mixer instance.
type NeverlurMixerHandle struct {
	Address string
	Server  interface{} // TODO(testharness-impl): *mixnet.Server
}

// NeverlurPKGHandle wraps the single Neverlur PKG instance.
type NeverlurPKGHandle struct {
	Address string
	Server  interface{} // TODO(testharness-impl): *pkg.Server
}

// NeverlurCDNHandle wraps the single Neverlur CDN instance.
type NeverlurCDNHandle struct {
	Address string
	Server  interface{} // TODO(testharness-impl): *cdn.Server
}

// GjallarhornMixerHandle wraps one Gjallarhorn mixer instance.
type GjallarhornMixerHandle struct {
	Address string
	Server  interface{} // TODO(testharness-impl): *gmixnet.Server
}

// GjallarhornCDNHandle wraps the single Gjallarhorn CDN instance.
type GjallarhornCDNHandle struct {
	Address string
	Server  interface{} // TODO(testharness-impl): *gcdn.Server
}
