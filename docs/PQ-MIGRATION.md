# Post-Quantum Migration Design for Vuvuzela + Alpenhorn

Status: design proposal, not yet implemented
Target: hybrid X25519 + ML-KEM-768 KEM, hybrid Ed25519 + ML-DSA-65 signatures
Library: `github.com/cloudflare/circl`
Audience: maintainers, reviewers familiar with the mixnet design

## 1. Why hybrid, why these primitives

Vuvuzela's threat model assumes a network-level adversary who records traffic.
A "harvest now, decrypt later" attacker can wait for a CRQC (cryptographically
relevant quantum computer) to break the X25519 ephemeral keys recorded today
and de-anonymise past conversations. Signatures protect liveness only and have
a lower urgency, but should still be PQ-secure for long-lived server identities
and signed config chains.

**Hybrid** (classical + PQ run side by side, results combined with a KDF)
is the conservative posture recommended by BSI, ANSSI, NLNCSA, and the IETF
hybrid drafts. It preserves classical security if ML-KEM/ML-DSA are later
weakened (cryptanalytic surprise on lattice problems is not unheard of) and
PQ security if X25519/Ed25519 are broken by a CRQC. The cost is bandwidth and
signature size; for a mixnet with bounded chains the cost is tolerable.

**ML-KEM-768 / ML-DSA-65** are the NIST FIPS 203/204 standards. Both are
co-designed by European researchers (CWI, Radboud, ENS Lyon, Ruhr-Bochum).
There is no Swedish-origin standardised PQC primitive at present; this is the
closest pragmatic "European" pick.

**CIRCL** is Cloudflare's pure-Go crypto library and exposes both ML-KEM and
ML-DSA along with hybrid KEM constructions. Pure-Go matters for arm64
portability and avoids cgo.

## 2. Current crypto map

| Role | Primitive | Call sites |
| --- | --- | --- |
| Per-hop onion KEM | X25519 via `nacl/box` and `vuvuzela.io/crypto/onionbox` | `client.go:230` (Seal), `client.go:279` (Open), `mixnet/mixnet.go:221` (GenerateKey), `mixnet/mixnet.go:354-358` (Precompute/Open) |
| Per-hop AEAD | XSalsa20-Poly1305 inside `nacl/box` | same as above (used implicitly) |
| Server identity / config sig | Ed25519 | `mixnet/mixnet.go:228` (pubkey extraction), `mixnet/mixnet.go:311` (Sign), `client.go:208` (Verify) |
| Transport auth | Ed25519 via alpenhorn `edtls` | `vuvuzela.io/alpenhorn/edtls` (external module) |
| Pairwise E2E session | XSalsa20-Poly1305 keyed from alpenhorn key wheel | `cmd/vuvuzela-client/conversation.go:385,395` |

Symmetric primitives (XSalsa20-Poly1305) are already PQ-secure with their
existing 256-bit keys; no migration needed there. The migration touches KEM
and signature roles only.

## 3. Migration boundaries

Three independent surfaces, each can land separately:

1. **Onion KEM** — mixnet ephemeral encryption (every round).
2. **Round-settings signatures** — coordinator-orchestrated, server-signed.
3. **edTLS transport** — alpenhorn module, used by every RPC and websocket.

A fourth implied surface is the **alpenhorn key wheel / friend discovery**
(also Ed25519 + X25519). That lives in `vuvuzela.io/alpenhorn` and is out of
scope for this doc but on the same migration arc.

## 4. Onion KEM redesign (the hard one)

### 4.1 The size problem

Today each mixer prepends a 32-byte X25519 ephemeral pubkey and a 16-byte
Poly1305 tag (`box.Overhead`). For an N-mixer chain the onion grows by
`N * 48` bytes plus the encrypted payload, padded to a fixed
`SizeEncryptedMessageBody`.

A hybrid X25519+ML-KEM-768 layer per hop costs roughly:

| Component | Bytes |
| --- | --- |
| X25519 ephemeral pubkey | 32 |
| ML-KEM-768 ciphertext | 1088 |
| AEAD tag | 16 |
| **Per-hop overhead** | **~1136** |

For a 3-mixer chain that is ~3.4 KB of overhead per onion vs. 144 B today
— a **24x** blow-up on the per-hop layers. Convo round payloads are small,
so total onion size grows from O(hundreds of bytes) to a few KB. At Vuvuzela's
scale of ~10^5–10^6 onions per round this becomes a real bandwidth ask on
mixers, but it is **not architecturally fatal**. Cover traffic and noise
messages pay the same cost.

ML-KEM-768 public keys are 1184 B but they only appear in `RoundSettings`
(one per mixer per round), not in every onion, so that cost is amortised.

### 4.2 Construction

For each hop, the client does:

```
(ek_x, c_x) = X25519-KEM(server_x25519_pk)        // 32 + 32 bytes wire
(c_mlkem, ss_mlkem) = ML-KEM-768.Encaps(server_mlkem_pk)
ss = HKDF-SHA256(salt="vz-pq-v1", ikm = c_x || c_mlkem || dh_x || ss_mlkem,
                 info = round || hop_index, len = 32)
ct = ChaCha20-Poly1305(ss, nonce = round_nonce, payload = next_layer)
```

The hybrid combiner follows the IETF
`draft-ietf-tls-hybrid-design` style: concatenate both shared secrets plus
both ciphertexts into the KDF so that an attacker who breaks one primitive
still has to break the other. Including ciphertexts (`c_x`, `c_mlkem`) gives
**binding** to the transcript and prevents re-encapsulation attacks against
ML-KEM (the FO-style binding gap in early ML-KEM analyses).

Note we move from XSalsa20-Poly1305 inside `nacl/box` to ChaCha20-Poly1305
because CIRCL's hybrid KEMs and the rest of the Go ecosystem standardise on
the IETF nonce length. This is a wire-format change, not a security change.

### 4.3 `onionbox` API change

Today's `onionbox.Seal(msg, nonce, []*[32]byte)` is asymmetric in shape: it
takes a slice of X25519 public keys. The PQ replacement needs **per-hop key
pairs** — both classical and PQ. Proposed new package
`vuvuzela.io/crypto/pqonionbox` (or a v2 of `onionbox`):

```go
type HopKey struct {
    X25519 [32]byte
    MLKEM  []byte   // 1184 B for ML-KEM-768
}

// Seal applies hybrid KEM layers in reverse (innermost first).
// Returns the wire onion plus the per-hop derived symmetric keys
// (kept by the client so it can decrypt the reply onion).
func Seal(msg []byte, fwdNonce *[24]byte, hops []HopKey) (onion []byte, keys [][32]byte, err error)

func Open(onion []byte, fwdNonce *[24]byte, x25519Priv *[32]byte, mlkemPriv []byte) (next []byte, sharedKey [32]byte, err error)

const Overhead = 1136 // approx, fix in implementation
```

`Overhead` becomes a function or constant depending on the hop algorithm
selection; the simplest path is a hard-coded hybrid v1 and bumping the value.

### 4.4 Server-side change

`mixnet/mixnet.go:221` (`box.GenerateKey`) becomes:

```go
xPub, xPriv, _ := nacl.GenerateKey(cryptoRand.Reader)
mlkemPub, mlkemPriv, _ := mlkem768.GenerateKeyPair(cryptoRand.Reader)
```

Round state grows to carry the ML-KEM private key. `RoundSettings.OnionKeys`
changes from `[][32]byte` to `[]HopKey`. The proto/easyjson schema
(`mixnet/mixnet_easyjson.go`, `mixnet/settings.go`) needs a new field; this
should be a **new field**, not a replacement, so old/new servers can interop
during rollout (see §7).

The bidirectional precomputation (`box.Precompute` at line 354) is a
classical-only optimisation. For the hybrid construction, the equivalent is
caching the derived `sharedKey [32]byte` per onion slot — which is already
the natural shape of the API above. The forward and backward directions
share the same hybrid `ss`, so this maps cleanly.

## 5. Signature redesign

### 5.1 RoundSettings signature

`mixnet/mixnet.go:311` (`ed25519.Sign`) becomes hybrid:

```go
edSig := ed25519.Sign(srv.SigningKey, msg)
mldsaSig := mldsa65.Sign(srv.MLDSAKey, msg, nil)
sig := append(edSig, mldsaSig...) // or length-prefixed struct
```

Verifier at `client.go:208` checks **both** signatures and accepts only if
both pass (`Verify_ed25519 AND Verify_mldsa`). This is the "strong unforgeable"
hybrid composition — the safest one for a mixnet because it preserves
unforgeability under either primitive.

ML-DSA-65 signatures are **3309 B** vs. Ed25519's 64 B. Each round currently
carries N signatures (one per mixer); with hybrid that becomes
`N * (64 + 3309) ≈ N * 3.4 KB`. For a 5-mixer chain that's 17 KB of signature
material in `NewRound` broadcasts — non-trivial but rare (once per round, not
per onion).

### 5.2 Server identity

Servers grow a **second** long-term key, the ML-DSA key. The
`PublicServerConfig` struct in `mixnet/settings.go` adds `MLDSAKey []byte`
alongside the existing Ed25519 `Key`. Generation and persistence on disk
needs a small migration (`vuvuzela-mixer` startup: if no ML-DSA key on disk,
generate one; warn until both are present).

### 5.3 Signed config chains (alpenhorn `config` package)

`alpenhorn/config.SignedConfig` uses Ed25519 across the verifier chain. Same
hybrid pattern applies. This is an **alpenhorn** change — the migration must
fork or branch alpenhorn (`vuvuzela.io/alpenhorn/config`) and version it.

## 6. edTLS transport

Alpenhorn's `edtls` builds TLS certificates from Ed25519 keys. Options:

1. **Stay classical here for v1.** TLS is session-scoped; recording it does
   not enable harvest-now-decrypt-later on conversation content (the onion
   layer is what protects conversations end-to-end). This is the **recommended
   v1 scope**.
2. Move to hybrid X25519+ML-KEM key agreement and Ed25519+ML-DSA cert
   signatures in v2 once Go's stdlib TLS or `crypto/tls` exposes PQ KEM
   suites (Go 1.24 added experimental ML-KEM TLS hooks).

Defer (2) to a follow-up; the win for v1 is concentrated in §4 and §5.

## 7. Rollout / interop strategy

The mixnet has no graceful protocol negotiation today — clients and servers
agree because they ship the same code. Three options:

**A. Flag day.** Cut a release, all servers and clients upgrade at once.
Simplest, but coordination cost. Realistic for a research deployment.

**B. Versioned `RoundSettings`.** Add `Version uint32` and new fields
(`MLKEMKeys`, `MLDSASigs`). Old clients use v1 fields and ignore new ones;
new clients require v2. Coordinator advertises supported versions. **This
is the recommended path** because it lets one mixer upgrade at a time and
shakes out integration bugs.

**C. Pure-PQ side service.** Run a parallel `convoPQ` service alongside
`convo`; clients opt in. Clean but doubles infra cost.

For (B), the dirty detail is the onion **format**: it cannot be silently
versioned per-hop because each mixer needs to know the layer shape it
strips. The simplest model is "all mixers in a chain run the same version,
chosen by coordinator." Mixed-version chains are a rabbit hole; avoid.

## 8. Concrete change list (files)

Vuvuzela repo (this repo):

- `client.go` — replace `onionbox.Seal/Open` with `pqonionbox` equivalents (lines 230, 279); update `expectedOnionSize` calculation (line 271) for new per-hop overhead.
- `mixnet/mixnet.go` — replace `box.GenerateKey` (221), `box.Precompute`/`box.Open`/`box.OpenAfterPrecomputation` (354–358), `ed25519.Sign` (311); add ML-DSA key field to `Server`; thread `MLKEMKey` and `MLKEMPriv` through `roundState`.
- `mixnet/settings.go`, `mixnet/mixnet_easyjson.go` — add `MLKEMKeys [][]byte` and `MLDSASignatures [][]byte` fields; regenerate easyjson.
- `convo/mixer.go`, `convo/config.go` — `SizeEncryptedMessageBody` is fine, but `incomingOnionSize` derivation in `mixnet.go:247` must use the new `Overhead`.
- `cmd/vuvuzela-mixer` — generate and persist ML-DSA key on startup if absent.
- `cmd/vuvuzela-coordinator` — propagate new `RoundSettings` fields; gather hybrid sigs from each mixer.
- New: `go.mod` (the repo is still GOPATH-style; the migration is a natural moment to modulize) with `github.com/cloudflare/circl` dependency.

Alpenhorn repo (separate, must be forked or PR'd):

- `config/sign.go` — hybrid signature verification on `SignedConfig`.
- `edtls/edtls.go` — leave as-is in v1 per §6.
- `pkg/` (friend discovery / key wheel) — out of scope for v1, on the roadmap.

## 9. Risk register

| Risk | Mitigation |
| --- | --- |
| Onion size growth saturates mixer bandwidth | Benchmark with `mixnet/mixnet_test.go` at realistic onion counts before merging; consider ML-KEM-512 instead of -768 if margins are tight (NIST level 1 vs 3) |
| Hybrid combiner mis-implementation (e.g., not binding ciphertext) | Use CIRCL's `hybrid` package directly where possible; cite the IETF draft in code comments; add KAT tests |
| ML-DSA sig size in `NewRound` broadcast hits websocket fragmentation | Profile; if needed, send signatures out-of-band via the coordinator's HTTP config endpoint instead of inline in `NewRound` |
| CIRCL version churn (CIRCL has had API changes around ML-KEM as NIST standardised) | Pin to a tagged release in `go.mod`; isolate behind the new `pqonionbox` package so upgrades touch one file |
| Mixed-version chain confusion | Forbid it: coordinator rejects chains spanning versions; document in README |
| Loss of `box.Precompute` perf optimisation | Replace with per-onion derived-key cache (same shape, slightly more memory); benchmark to confirm |
| Alpenhorn dependency drift | Either upstream the change (preferred) or vendor a fork under a new module path |

## 10. Phased plan

1. **Phase 0 — module-ize.** Convert vuvuzela to Go modules, pin Go ≥ 1.22, add CIRCL. No crypto changes. Independently reviewable.
2. **Phase 1 — `pqonionbox`.** Implement hybrid KEM onion library with property tests and KATs. No integration yet.
3. **Phase 2 — mixnet onion swap behind a `Version` field.** Servers advertise both v1 and v2; coordinator picks v1 by default. Run a v2-only testnet.
4. **Phase 3 — hybrid `RoundSettings` signatures.** Independent of Phase 2; can land in either order.
5. **Phase 4 — alpenhorn `SignedConfig` hybrid sigs.** Fork or PR upstream.
6. **Phase 5 — flip coordinator default to v2; deprecate v1.**
7. **Phase 6 (future) — edTLS and alpenhorn key-wheel PQ.**

## 11. Open questions for the maintainers

- Is converting to Go modules acceptable as part of this work, or should the build stay GOPATH-style?
- Is the alpenhorn module a hard dependency (upstream-and-wait) or are we comfortable maintaining a fork?
- NIST security level — ML-KEM-512 (level 1, smaller, faster) vs ML-KEM-768 (level 3, default)? The bandwidth budget at mixer scale will likely decide this.
- Is "European-co-designed primitives in a NIST standard" close enough to the "European/Swedish" requirement, or does the project want a deliberately non-NIST option like FrodoKEM (much larger, slower, but free of any NIST involvement)?
