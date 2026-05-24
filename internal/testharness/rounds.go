// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

package testharness

import "fmt"

// AdvanceRound moves the named service forward one round, bypassing
// real-clock dialing. Valid service names: "AddFriend", "Dialing",
// "Convo".
//
// NOT safe under concurrent calls; the integration test drives rounds
// from a single test goroutine.
//
// TODO(testharness-impl): once the per-coordinator wiring lands, this
// invokes the coordinator's manual-step entry point. The Neverlur
// coordinator's coordinator.Server exposes a Tick-style method via
// PersistedState manipulation; the Gjallarhorn coordinator likewise.
// Implementation choices for how-to-drive-deterministically are
// captured in research.md R3 of the Gjallarhorn plan; this stub will
// be replaced by the real driver in the follow-up commit.
func (h *Harness) AdvanceRound(service string) error {
	switch service {
	case "AddFriend", "Dialing", "Convo":
		return scaffoldedNotImplemented(fmt.Sprintf("AdvanceRound(%q)", service))
	default:
		return fmt.Errorf("testharness.AdvanceRound: unknown service %q (want AddFriend|Dialing|Convo)", service)
	}
}
