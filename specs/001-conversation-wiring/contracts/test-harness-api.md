# Contract: `internal/testharness` package API

**Owner**: Gjallarhorn
**Stability**: stable from this Phase A commit; changes require both repos to update in lockstep (the Neverlur-side demo CLI imports the package).
**Import path**: `github.com/oluies/gjallarhorn/internal/testharness`

## Exported API

```go
package testharness

import (
    "testing"

    "github.com/oluies/gjallarhorn/coordinator"
    gmixnet "github.com/oluies/gjallarhorn/mixnet"
    gcdn "github.com/oluies/gjallarhorn/cdn"
    "github.com/oluies/neverlur/cdn"
    "github.com/oluies/neverlur/config"
    "github.com/oluies/neverlur/hybrid"
    ncoord "github.com/oluies/neverlur/coordinator"
    nmixnet "vuvuzela.io/vuvuzela/mixnet"
    "github.com/oluies/neverlur/pkg"
    "github.com/oluies/neverlur"
)

// Harness is an in-memory stand-up of the full Neverlur + Gjallarhorn
// service set. Suitable for both integration tests and the Neverlur
// demo CLI. Every component runs in goroutines on local sockets; no
// network ports are bound beyond the optional ListenAddr.
type Harness struct {
    NeverlurCoordinator    *ncoord.Server
    NeverlurMixers         []*nmixnet.MixServer
    NeverlurPKG            *pkg.Server
    NeverlurCDN            *cdn.Server
    GjallarhornCoordinator *coordinator.Server
    GjallarhornMixers      []*gmixnet.MixServer
    GjallarhornCDN         *gcdn.Server

    AddFriendConfig *config.SignedConfig
    DialingConfig   *config.SignedConfig
    ConvoConfig     *config.SignedConfig

    HarnessGuardian *hybrid.HybridIdentity
    ListenAddr      string // empty unless OptionListenAddr is supplied
}

// Option configures Harness behavior.
type Option func(*options)

// OptionListenAddr makes the harness listen on the given Unix socket
// path so the Neverlur demo CLI can connect a second process. Defaults
// to no listening (pure in-process integration test usage).
func OptionListenAddr(path string) Option { ... }

// OptionMixerCount overrides the default mixer count (3) for either
// service. Mostly useful for stress tests.
func OptionMixerCount(neverlurMixers, gjallarhornMixers int) Option { ... }

// New starts every component and returns a ready-to-use Harness.
// Registers tb.Cleanup for full teardown; Close() can be called
// explicitly if a non-test caller (like the demo CLI) is consuming.
//
// New panics via tb.Fatal if any component fails to start.
func New(tb testing.TB, opts ...Option) *Harness

// Close tears down every component cleanly. Idempotent.
func (h *Harness) Close()

// ClientFor returns a Neverlur client wired against this harness with
// the given username. The client's hybrid identity is freshly generated
// each call. The client is automatically registered with the PKG; if
// registration fails the test is failed via tb.Fatal.
//
// The returned *TestClient embeds *neverlur.Client and adds RecvCh +
// ConvoState fields for the integration test to read.
func (h *Harness) ClientFor(tb testing.TB, username string) *TestClient

// AdvanceRound moves the named service forward one round, bypassing
// the real clock. Valid service names: "AddFriend", "Dialing", "Convo".
// Returns after the round's onion has been delivered to all mailboxes.
func (h *Harness) AdvanceRound(service string)

// TestClient is the harness-managed wrapper around *neverlur.Client.
// Embeds the client so all standard methods are available.
type TestClient struct {
    *neverlur.Client

    Username       string
    HybridIdentity *hybrid.HybridIdentity
    RecvCh         <-chan IncomingMessage
    // ConvoState becomes non-nil after a SendCall or ReceivedCall handler
    // bootstraps the conversation.
    ConvoState     ConvoStateAccessor
}

// IncomingMessage captures a single decrypted conversation packet
// the receiver got via the Conversation.Open path.
type IncomingMessage struct {
    FromPeer string
    Body     []byte
    Round    uint32
    Err      error
}

// ConvoStateAccessor is a narrow interface over gjallarhorn.Conversation
// exposing only the read-side state the integration test needs.
// Hides Gjallarhorn-internal types from test consumers.
type ConvoStateAccessor interface {
    SessionKey() *[32]byte
    Round() uint32
    PeerUsername() string
}
```

## Lifecycle invariants

- `New` MUST be called from the goroutine that owns the test (`t.Run` etc.). It registers `tb.Cleanup`.
- `Close` is safe to call multiple times.
- `AdvanceRound` is NOT safe under concurrent calls from multiple goroutines. The integration test drives rounds from a single test goroutine.
- A `TestClient` is bound to the `Harness` that produced it; do not mix clients across harnesses.

## What this package does NOT expose

- Direct access to any server-side handler. Tests interact via standard client-facing methods (`SendFriendRequest`, `SendCall`, etc.) and harness-mediated round advancement.
- Real network listeners (beyond the optional `ListenAddr` Unix socket). Production CI doesn't open any TCP port.
- Real-clock dialing rounds. Production code paths use a `time.Ticker`; tests advance rounds manually.

## Versioning

This contract is at version 1, defined by Phase A. Future versions:
- New `OptionXxx` flags can be added (backward-compatible).
- New methods on `Harness` are additive.
- Removing a method or changing a signature is a breaking change requiring paired-PR coordination.
