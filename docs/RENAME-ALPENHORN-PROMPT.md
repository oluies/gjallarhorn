# Prompt: rename oluies/alpenhorn fork to "neverlur"

Paste everything below the line into a fresh Claude Code session opened in
`~/projects/alpenhorn`. It's self-contained — the receiving session won't
have any context from the gjallarhorn rename.

---

I want to rebrand my fork of Alpenhorn from "alpenhorn" to **neverlur**
(Norwegian/Sami birch-bark shepherd's horn — Nordic analogue of an alpine
horn). This is the friend-discovery companion to my Vuvuzela fork, which I
already rebranded to **gjallarhorn** (Heimdall's horn) in
`~/projects/vuvuzela/.worktrees/task/newpq-fdae3f`.

The gjallarhorn repo already imports `github.com/oluies/neverlur/...` paths,
so its build is blocked until this rename lands. **Mirror the gjallarhorn
rebrand exactly** in scope and style. No post-quantum crypto changes in this
session — pure rename.

## Coordinates

- **Working directory:** `~/projects/alpenhorn` (this is the local clone of
  `github.com/oluies/alpenhorn`, the fork — verify with `git remote -v`).
- **Old module / vanity path:** `vuvuzela.io/alpenhorn` (and
  `vuvuzela.io/alpenhorn/<subpackage>`).
- **New module path:** `github.com/oluies/neverlur`.
- **Sibling fork (already renamed):** `github.com/oluies/gjallarhorn` (was
  `vuvuzela.io/vuvuzela`). Cross-reference its `README.md`, `NOTICE`, and
  `docs/PQ-MIGRATION.md` for style.
- **License:** AGPL-3.0 — preserve unchanged. Keep all
  `// Copyright 2015 David Lazar` headers verbatim on copied files.

## Scope of this session (do all of these)

1. **Survey.** Run grep/find to enumerate:
   - All `^package <name>` declarations in `.go` files.
   - All `vuvuzela.io/...` imports, grouped by which org repo they come from.
   - All `cmd/alpenhorn-*` (or `cmd/*-server`) binary directories.
   - Any "alpenhorn" or "Alpenhorn" string in README / docs / user-facing
     strings.

   Report what you find before touching anything.

2. **`go.mod`.** Initialize Go modules at the repo root:
   `go mod init github.com/oluies/neverlur`. Pin `go 1.22` in the file
   (overwrite whatever `go mod init` writes if it's a newer version, for
   portability — gjallarhorn used 1.22).

3. **Rewrite imports.** Across all `.go` files:
   - `vuvuzela.io/alpenhorn` → `github.com/oluies/neverlur`
     (and all subpaths follow automatically).
   - Leave other `vuvuzela.io/*` imports alone for now:
     `vuvuzela.io/concurrency`, `vuvuzela.io/crypto/...`,
     `vuvuzela.io/internal/...`. These are separate upstream repos and will
     be handled in a follow-up session. Flag them in your final report.

   Use `sed -i ''` (BSD sed, this is macOS) or equivalent. After the sweep,
   `grep -rn "vuvuzela.io/alpenhorn" --include="*.go" .` must return nothing.

4. **Rename root package.** If the repo root has `package alpenhorn`
   declarations (likely in files like `client.go`, `keywheel.go`, etc.),
   rename to `package neverlur`. Then find every external caller using
   `alpenhorn.X` style references in this repo and rewrite to `neverlur.X`
   (likely only in `cmd/` files that import the root package).

5. **Rename cmd directories.** Any `cmd/alpenhorn-*` directory becomes
   `cmd/neverlur-*` via `git mv` (preserve git history). Likely candidates:
   `cmd/alpenhorn-client`, `cmd/alpenhorn-coordinator` (or `cdn`),
   `cmd/alpenhorn-mixer`, `cmd/alpenhorn-pkg`, `cmd/alpenhorn-cdn` — verify
   by listing `cmd/` first.

6. **README + NOTICE.** Rewrite `README.md` with:
   - New name "Neverlur" and a one-line description (Norwegian/Sami
     birch-bark shepherd's horn — friend-discovery companion to gjallarhorn).
   - Lineage paragraph crediting David Lazar and the original Alpenhorn
     project at `github.com/vuvuzela/alpenhorn`.
   - Cross-link to `github.com/oluies/gjallarhorn`.

   Add a top-level `NOTICE` file mirroring the one at
   `~/projects/vuvuzela/.worktrees/task/newpq-fdae3f/NOTICE` — adapt names,
   URLs, and dependency list to match alpenhorn's actual deps.

7. **Run `gofmt -w .`** to fix import grouping after the sed sweep.

8. **Verification.**
   - `grep -rn "vuvuzela.io/alpenhorn" --include="*.go" .` → no output.
   - `grep -rn "^package alpenhorn" --include="*.go" .` → no output.
   - `gofmt -l .` → no output.
   - `go vet ./...` — expect failures for the still-`vuvuzela.io/*`
     external deps, and possibly for any cross-fork imports back to
     `github.com/oluies/gjallarhorn` (alpenhorn shouldn't import vuvuzela
     normally, but check). Report all errors but don't try to fix the
     `vuvuzela.io/*` ones — they're known and out of scope.

## What NOT to do

- **No crypto changes.** Don't touch `edtls`, `pkg/`, or any ed25519/X25519
  call sites. Post-quantum migration is the next session, separate plan.
- **Don't push or rename the GitHub repo.** I'll do the GitHub Settings
  rename myself once I review the local diff. Leave the git remote pointing
  at `github.com/oluies/alpenhorn`.
- **Don't commit yet.** Leave everything as working-tree changes so I can
  review.
- **Don't fork or modify the `vuvuzela.io/concurrency`, `vuvuzela.io/crypto/*`,
  `vuvuzela.io/internal/ioutil2` deps.** Flag them in the report; they're
  handled separately.
- **Don't touch user-facing strings** beyond README/NOTICE. Banners, help
  text, log lines that say "alpenhorn" stay as-is for now (separate pass).

## Deliverable

When done, give me a summary with:
- Files touched (count + categories: imports, package decls, dir renames, docs).
- The list of `vuvuzela.io/*` external deps still referenced and which ones
  the future PQ work will replace anyway.
- Any surprises encountered (unusual import patterns, missing files,
  protobuf-generated files needing regen, etc.).
- `git status --short` output.

## Style guidance

Be concise in your responses. Use Bash and Edit tools directly — no need
to spawn agents for this. Track progress with TaskCreate if you find it
useful but it's not required. End-of-turn summary should be one or two
sentences; the detailed report goes in your final message.
