# Gjallarhorn

> Heimdall's horn, heard across all nine realms.

Gjallarhorn is a metadata-private messaging system: a post-quantum fork of
[Vuvuzela](https://github.com/vuvuzela/vuvuzela). Users communicating through
Gjallarhorn do not reveal who they are talking to, even in the presence of
powerful nation-state adversaries with future quantum capability.

This fork tracks the upstream Vuvuzela design (see the SOSP 2015
[paper](https://davidlazar.org/papers/vuvuzela.pdf) and
[slides](https://davidlazar.org/slides/vuvuzela-sosp2015.pdf) for the threat
model, performance, and limitations) and replaces the classical X25519 / Ed25519
primitives with **hybrid X25519 + ML-KEM-768** for onion encryption and
**hybrid Ed25519 + ML-DSA-65** for round-settings and config signatures. The
migration plan is in [docs/PQ-MIGRATION.md](docs/PQ-MIGRATION.md).

Companion friend-discovery service: [neverlur](https://github.com/oluies/neverlur)
(post-quantum fork of [Alpenhorn](https://github.com/vuvuzela/alpenhorn)).

Gjallarhorn uses efficient cryptography (NaCl + [CIRCL](https://github.com/cloudflare/circl))
to hide as much metadata as possible and adds noise to metadata that can't be
encrypted efficiently. The noise approach provides less privacy than encrypting
all metadata, but it enables the system to support millions of users while
guaranteeing
[differential privacy](https://en.wikipedia.org/wiki/Differential_privacy) for
user metadata.

## Lineage

Gjallarhorn is a fork of `github.com/vuvuzela/vuvuzela`, originally written by
**David Lazar** with contributions from **Jelle van den Hooff**,
**Nickolai Zeldovich**, and **Matei Zaharia**. All upstream code retains its
original copyright notices and the AGPL-3.0 license. See [NOTICE](NOTICE) for
the complete lineage and attribution.

## Acknowledgements

Upstream Vuvuzela is the work of David Lazar and collaborators at MIT CSAIL.
This fork exists to evolve the system with post-quantum primitives while
preserving the original design.

## See also

- [neverlur](https://github.com/oluies/neverlur) — friend-discovery (fork of Alpenhorn)
- [Vuvuzela](https://github.com/vuvuzela/vuvuzela) — upstream
- [Alpenhorn](https://github.com/vuvuzela/alpenhorn) — upstream friend-discovery
</content>
</invoke>