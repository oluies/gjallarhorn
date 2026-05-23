# Prompt: rename oluies/alpenhorn fork to "neverlur" + bootstrap spec-kit

Paste everything below the line into a fresh Claude Code session opened in
`~/projects/alpenhorn`. It's self-contained — the receiving session won't
have any context from the gjallarhorn rename.

---

I want to rebrand my fork of Alpenhorn from "alpenhorn" to **neverlur**
(Norwegian/Sami birch-bark shepherd's horn — Nordic analogue of an alpine
horn) and bootstrap the same Spec-Kit spec-driven development scaffolding
that I just added to its companion fork **gjallarhorn**.

This is the friend-discovery sibling of my Vuvuzela fork. Gjallarhorn
(Heimdall's horn) lives at `~/projects/vuvuzela/.worktrees/task/newpq-fdae3f`
on branch `task/newpq-fdae3f` of `github.com/oluies/vuvuzela` (GitHub repo
will be renamed to gjallarhorn in Settings later). The gjallarhorn branch
already imports `github.com/oluies/neverlur/...` paths, so its build is
blocked until this rename lands. **Mirror the gjallarhorn work exactly** in
scope, structure, and commit shape. Three commits, one branch, then push.

No post-quantum crypto changes in this session — pure rename + scaffolding.

## Coordinates

- **Working directory:** `~/projects/alpenhorn` (this is the local clone of
  `github.com/oluies/alpenhorn`, the fork — verify with `git remote -v`).
- **Old module / vanity path:** `vuvuzela.io/alpenhorn` (and
  `vuvuzela.io/alpenhorn/<subpackage>`).
- **New module path:** `github.com/oluies/neverlur`.
- **Sibling fork to cross-reference:** `github.com/oluies/gjallarhorn` (was
  `vuvuzela.io/vuvuzela`) at
  `~/projects/vuvuzela/.worktrees/task/newpq-fdae3f`. Read its `README.md`,
  `NOTICE`, `.gitignore`, `.specify/memory/constitution.md`, and
  `docs/PQ-MIGRATION.md` for style and lineage block templates.
- **License:** AGPL-3.0 — preserve unchanged. Keep all
  `// Copyright 2015 David Lazar` headers verbatim on copied files.
- **Tools required:** Go (any 1.22+), `uv` (`brew install uv` if missing).
- **Spec-Kit version:** v0.8.13.

## Commit shape (three commits, in this order)

1. `all: add .gitignore for local Claude Code state` — gitignore only.
2. `all: rebrand to neverlur (post-quantum fork foundation)` — the full
   mechanical rename.
3. `all: bootstrap spec-kit (spec-driven development scaffolding)` —
   spec-kit + neverlur constitution.

Stage selectively. Use heredoc commit messages. **Do not amend** if a hook
fails — fix the root cause and add a new commit. **Do not push or rename
the GitHub repo yet** — I'll do the GitHub Settings rename myself once I
review the diff. Push branch as feature branch:
`git push -u origin <branch-name>`.

## Phase 1 — Survey

Before touching anything, run:

```bash
git remote -v                                    # verify oluies/alpenhorn
ls cmd/                                          # find alpenhorn-* binaries
grep -rn "^package " --include="*.go" .          # all package decls
grep -rn '"vuvuzela.io/' --include="*.go" . \
  | awk -F'"' '{print $2}' | sort -u             # all vuvuzela.io imports
grep -rn "vuvuzela.io/alpenhorn" --include="*.go" . | wc -l   # count
ls .specify/ .claude/ CLAUDE.md MEMORY.md 2>&1   # spec-kit collisions
cat .gitignore 2>/dev/null                       # existing ignore rules
```

Report findings briefly before edits. Flag any surprises (unusual package
layouts, protobuf-generated files, alpenhorn imports of itself).

## Phase 2 — .gitignore commit

Create `.gitignore` matching the gjallarhorn one:

```
# Claude Code local state (skills are committed; settings/plans are not)
.claude/settings.local.json
.claude/plans/

# Editor / OS
.DS_Store
*.swp
*~

# Build artifacts
*.test
*.out
```

`git add .gitignore && git commit -m "..."` with this message:

```
all: add .gitignore for local Claude Code state

Exclude .claude/settings.local.json and .claude/plans/ (user-specific
state) so they don't get committed during routine staging. Skills and
project-shared settings remain tracked.
```

## Phase 3 — Rename commit

### 3.1 `go.mod`
```bash
go mod init github.com/oluies/neverlur
```
Then edit `go.mod` to pin `go 1.22` (overwrite whatever `go mod init`
writes — gjallarhorn uses 1.22 for portability).

### 3.2 Rewrite imports
```bash
find . -name "*.go" -not -path "./.git/*" \
  -exec grep -l "vuvuzela.io/alpenhorn" {} + \
  | xargs -I {} sed -i '' 's|vuvuzela\.io/alpenhorn|github.com/oluies/neverlur|g' {}
grep -rn "vuvuzela.io/alpenhorn" --include="*.go" .   # must be empty
```

Leave `vuvuzela.io/concurrency`, `vuvuzela.io/crypto/*`,
`vuvuzela.io/internal/*` alone. Flag them in your final report; they're
handled in a future session.

### 3.3 Rename root package
If files at the repo root declare `package alpenhorn` (likely `client.go`,
`keywheel.go`, or similar), rename to `package neverlur`. Then find every
external caller using `alpenhorn.X` and rewrite to `neverlur.X` (likely
only in `cmd/` files that import the root package). Use targeted sed —
BSD sed has no `\b`, so use specific patterns like
`s|alpenhorn\.Client|neverlur.Client|g`.

### 3.4 Rename cmd directories
For each `cmd/alpenhorn-*` directory, use `git mv` to preserve history:
```bash
git mv cmd/alpenhorn-client cmd/neverlur-client
# repeat for coordinator, mixer, pkg, cdn, etc.
```

### 3.5 README + NOTICE
Rewrite `README.md` with the new name "Neverlur", one-line description
(Norwegian/Sami birch-bark shepherd's horn — friend-discovery companion to
gjallarhorn), lineage paragraph crediting David Lazar and upstream
Alpenhorn at `github.com/vuvuzela/alpenhorn`, and cross-link to
`github.com/oluies/gjallarhorn`. Mirror the structure of gjallarhorn's
`README.md` at `~/projects/vuvuzela/.worktrees/task/newpq-fdae3f/README.md`.

Add a top-level `NOTICE` file mirroring
`~/projects/vuvuzela/.worktrees/task/newpq-fdae3f/NOTICE`. Adapt:
- Project name "Neverlur".
- Upstream URL: `github.com/vuvuzela/alpenhorn`.
- Dependency list to match alpenhorn's actual third-party imports.

### 3.6 Format and verify
```bash
gofmt -w .
gofmt -l .                          # must be empty
grep -rn "vuvuzela.io/alpenhorn" --include="*.go" .   # must be empty
grep -rn "^package alpenhorn" --include="*.go" .      # must be empty
go vet ./...                        # will fail on still-vuvuzela.io/* deps;
                                    # report errors but don't try to fix
```

### 3.7 Commit
```bash
git add -A
git commit -m "..."
```
Commit message body should explain: it's a mechanical rebrand, no crypto
changes, module is now `github.com/oluies/neverlur`, build is currently
red because of the lingering `vuvuzela.io/*` deps, license unchanged.

## Phase 4 — Spec-Kit bootstrap commit

### 4.1 Install spec-kit CLI
```bash
uv tool install specify-cli \
  --from "git+https://github.com/github/spec-kit.git@v0.8.13"
specify --version    # confirms 0.8.13
```

### 4.2 Initialize
```bash
specify init . --here --ai claude --force
```

This creates `.specify/` (templates, scripts, workflows, integrations,
extensions, memory) and `.claude/skills/speckit-*/` (14 skill files).

### 4.3 Author the constitution
Read the gjallarhorn constitution as a style reference:
`~/projects/vuvuzela/.worktrees/task/newpq-fdae3f/.specify/memory/constitution.md`.

Overwrite `.specify/memory/constitution.md` with a **neverlur-specific**
constitution. Five core principles, mirroring gjallarhorn's structure but
adapted to the friend-discovery role:

- **I. Upstream Lineage Is Preserved.** Same as gjallarhorn — copyright
  headers, AGPL-3.0, NOTICE current, paired with gjallarhorn.
- **II. The Threat Model Is Sacred.** Same baseline (SOSP 2015) plus the
  Alpenhorn paper's friend-discovery threat model
  (https://davidlazar.org/papers/alpenhorn.pdf). Friend-discovery leaks are
  P0 bugs.
- **III. Post-Quantum Is Hybrid, Never Pure (NON-NEGOTIABLE).** Same as
  gjallarhorn. Mention the specific surfaces: edTLS server certs, signed
  config chains, key-wheel session keys, friend-request handshake.
- **IV. Cryptographic Code Is Imported, Not Invented.** Same — CIRCL,
  x/crypto, stdlib only. KATs required.
- **V. Identity Continuity.** Neverlur-specific: a user's long-term
  identity may be rotated but **never silently downgraded**. PQ-augmented
  identities must remain verifiable by hybrid-aware clients during any
  transition window. Friend-list state is migrated, not invalidated.

Compatibility section: paired PRs with gjallarhorn, stable module path
`github.com/oluies/neverlur`. Quality gates: gofmt, vet, test, KATs for
crypto, design notes for wire-format changes. Governance: amendments
require commit body with rationale, version bump.

Version: 1.0.0, ratified today (use the actual current date).

### 4.4 Commit
```bash
git add -A
git commit -m "..."
```
Message structure: mirror gjallarhorn's spec-kit-bootstrap commit message.
Cross-reference the constitution principles in the body.

## Phase 5 — Push

```bash
git push -u origin <branch-name>
```

If the current branch is `master` or `main` and you don't want to push
directly, create a feature branch first:
```bash
git checkout -b task/neverlur-rebrand
git push -u origin task/neverlur-rebrand
```

Otherwise push the existing branch you're on.

## Verification checklist

Before reporting done:
- [ ] `git log --oneline -5` shows three new commits in expected order.
- [ ] `gofmt -l .` is empty.
- [ ] `grep -rn "vuvuzela.io/alpenhorn" --include="*.go" .` is empty.
- [ ] `grep -rn "^package alpenhorn" --include="*.go" .` is empty.
- [ ] `.specify/memory/constitution.md` is filled in (no `[PLACEHOLDER]`
      text remains).
- [ ] `.gitignore` excludes `.claude/settings.local.json` and
      `.claude/plans/` but allows `.claude/skills/` and
      `.claude/settings.json`.
- [ ] `git status --short` is clean.
- [ ] `git ls-remote --heads origin <branch>` shows the new branch.

## What NOT to do

- **No crypto changes.** Don't touch `edtls`, `pkg/`, `keywheel`, or any
  ed25519/X25519 call sites. Post-quantum migration is a separate session.
- **Don't push or rename the GitHub repo.** I'll handle the Settings
  rename.
- **Don't fork or modify** `vuvuzela.io/concurrency`,
  `vuvuzela.io/crypto/*`, `vuvuzela.io/internal/*` — flag them, don't
  touch them.
- **Don't touch user-facing strings** beyond README/NOTICE/constitution.
  Banners, help text, log lines that say "alpenhorn" stay as-is for now.
- **Don't run `go mod tidy`** until after `github.com/oluies/neverlur`
  exists as a renamed remote — it will fail and pollute go.sum.
- **Don't skip pre-commit hooks** (`--no-verify`). Fix the underlying
  cause if a hook fails.

## Deliverable

When done, give me a summary with:
- Three commit SHAs and their subjects.
- Files touched per commit (count + categories).
- The list of `vuvuzela.io/*` external deps still referenced.
- Any surprises (unusual package layouts, protobuf-generated files
  needing regen, etc.).
- `git status --short` output (should be clean).
- The remote branch name and a confirmation it was pushed.

## Style guidance

Be concise. Use Bash and Edit tools directly — no need to spawn agents
for this. Track progress with TaskCreate if you find it useful. End-of-turn
summary should be one or two sentences; the detailed report goes in your
final message before that.
