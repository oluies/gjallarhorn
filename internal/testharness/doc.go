// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

// Package testharness is the in-process stand-up of every server-side
// component the Phase A integration test (specs/001-conversation-wiring/)
// needs: Neverlur coordinator + PKG + CDN + 3 mixers; Gjallarhorn
// coordinator + 3 mixers + CDN. Components run as goroutines on local
// addresses; no network ports are bound beyond an optional Unix socket
// (used by the Neverlur demo CLI to attach a second process).
//
// The harness mirrors a real deployment's topology so the integration
// test exercises every code path, not a mocked subset.
//
// # Status
//
// Branch 002-testharness ships this package as a TYPED SCAFFOLD: every
// exported type, function, and contract from
// specs/001-conversation-wiring/contracts/test-harness-api.md is in
// place and compiles, but the server-orchestration internals are
// documented TODOs that will be filled in by a focused follow-up.
//
// What works today: types are stable, the New/Close API exists, the
// signed-config generation (configs.go) is real, and the TestClient
// data model is real. Consumers can write code against this package
// (Neverlur's neverlur-conversation-demo binary, the integration
// tests) and that consumer code will compile.
//
// What does NOT work today: New() returns a Harness whose server
// fields are nil, ClientFor() returns a TestClient without a wired
// *neverlur.Client, AdvanceRound() is a no-op. Calling these from a
// test will fail at runtime with a clearly labeled error pointing at
// this doc comment.
//
// # Why a scaffold
//
// The orchestration code (~1500-2500 LOC across coordinator.go,
// mixers.go, pkg.go) cannot be verified locally on Apple Silicon
// because vuvuzela.io/crypto/bn256 ships x86_64-only assembly. The
// scaffolding approach lets the package shape land on master so
// downstream consumers (the demo CLI, the integration test) can
// start; the per-component wiring follows in a CI-iterating session.
//
// # Constitutional note
//
// The package is in internal/ so external modules cannot depend on
// it. The Neverlur demo CLI accesses it via the Go workspace
// (per docs/local-development.md) for cross-repo development, but
// in production neither this package nor its scaffolded servers
// runs — production deployments use the real cmd/gjallarhorn-*
// binaries.
package testharness
