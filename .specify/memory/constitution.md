# Gjallarhorn Constitution

Gjallarhorn is a post-quantum fork of Vuvuzela. Its purpose is to evolve the
original metadata-private messaging system to remain secure against
"harvest-now, decrypt-later" adversaries with future quantum capability,
without weakening the privacy guarantees of the upstream design.

## Core Principles

### I. Upstream Lineage Is Preserved
Every file inherited from upstream Vuvuzela keeps its original
`// Copyright 2015 David Lazar` header verbatim. The AGPL-3.0 license is
never relaxed, dual-licensed, or replaced. The `NOTICE` file at the repo
root stays current: every new third-party dependency is listed with its
license. The companion service Neverlur (fork of Alpenhorn) is held to the
same standard. Lineage is not a chore; it is a precondition for the fork's
legitimacy.

### II. The Threat Model Is Sacred
The SOSP 2015 Vuvuzela threat model — differentially private noise,
multi-server mixnet anonymity, encrypted dead drops, no single party
trusted with metadata — is the baseline. No change is permitted that
weakens it. Every PR touching crypto, networking, or round protocol must
include an explicit "threat-model impact" paragraph in its description.
Privacy regressions are P0 bugs.

### III. Post-Quantum Is Hybrid, Never Pure (NON-NEGOTIABLE)
Cryptographic primitives migrate to **hybrid** constructions — classical
(X25519, Ed25519) and post-quantum (ML-KEM-768, ML-DSA-65) running side by
side, combined through a KDF that binds both transcripts. Pure-PQ
deployments are forbidden until ML-KEM and ML-DSA have at least five years
of unbroken production deployment across the industry. Lattice
cryptanalysis is young; classical primitives are the belt under the
suspenders.

### IV. Cryptographic Code Is Imported, Not Invented
All KEM, signature, and AEAD primitives come from vetted libraries:
`github.com/cloudflare/circl`, `golang.org/x/crypto`, or the Go standard
library. No hand-rolled lattice math, no cgo wrappers around C crypto, no
toy reimplementations "for performance." Every primitive integration ships
with Known-Answer Tests (KATs) drawn from the relevant NIST specification
or IETF draft. The hybrid combiner follows the IETF
`draft-ietf-tls-hybrid-design` style and is the only place where
primitives meet.

### V. Bandwidth and Latency Are Measured, Not Estimated
Post-quantum primitives expand wire formats: a single ML-KEM-768
ciphertext is 1088 bytes versus 32 bytes for an X25519 public key. Any
change to onion format, round settings, or signature carriage must be
accompanied by `mixnet/mixnet_test.go` benchmarks comparing the previous
and new wire sizes at realistic chain lengths (3, 5, 7 mixers). Numbers
appear in the PR description. Surprises at scale are unacceptable.

## Compatibility and Coordination

Gjallarhorn does not exist alone. Neverlur, the friend-discovery sibling
fork of Alpenhorn, provides identity, key wheel, and config signing. Any
change crossing the gjallarhorn/neverlur boundary — onion format,
signature scheme, edTLS, signed-config schema — lands as **paired pull
requests** referencing each other. Module paths are stable:
`github.com/oluies/gjallarhorn` and `github.com/oluies/neverlur`. Versioned
`RoundSettings` fields enable one-mixer-at-a-time upgrades; mixed-version
chains within a single round are forbidden.

## Quality Gates

- `gofmt -l .` must be silent.
- `go vet ./...` must pass.
- `go test ./...` must pass, including mixnet and convo packages.
- Crypto PRs require KAT test additions; review must explicitly confirm
  KATs were checked against an authoritative source.
- Wire-format changes require a `docs/` design note describing the format
  before code review begins.
- No commit may skip pre-commit hooks or signing (`--no-verify`,
  `--no-gpg-sign`). Hook failures are fixed at the root, not bypassed.

## Governance

This constitution supersedes ad-hoc convention. Amendments require a
commit that:

1. Edits this file with the new principle or revision.
2. Explains in the commit body why the change is necessary, what
   alternatives were considered, and what migration (if any) existing
   code requires.
3. Updates the version and amendment date below.

All PR reviews must check the change against these principles. A reviewer
who approves a PR that violates the constitution shares responsibility for
the violation. Complexity beyond what these principles require must be
justified inline in the PR description.

For day-to-day development guidance (commands, conventions, file layout),
see `CLAUDE.md` and `docs/PQ-MIGRATION.md`.

**Version**: 1.0.0 | **Ratified**: 2026-05-22 | **Last Amended**: 2026-05-22
