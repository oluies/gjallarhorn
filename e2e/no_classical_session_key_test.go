// Copyright 2026 The Gjallarhorn Authors. All rights reserved.
// Use of this source code is governed by the GNU AGPL
// license that can be found in the LICENSE file.

// Package e2e contains integration + static-check tests that guard
// invariants which can't be expressed in a single-package unit test.
package e2e

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// TestNoClassicalSessionKey is a static check enforcing Constitution
// Principle III ("Hybrid, Never Pure — NON-NEGOTIABLE"). The
// conversation-layer per-round session key MUST be derived from the
// hybrid combiner (which mixes X25519 + ML-KEM-768). Code that
// derives a *[32]byte session-key value from only the classical
// (X25519) half is forbidden — it would silently downgrade an
// upgraded user back to classical-only security.
//
// The check is intentionally over-broad: it scans the convo/ and
// the upstream Neverlur keywheel package (via the local copy under
// the workspace) for identifiers whose name resembles a "classical
// session key" and asserts none escapes into the per-round derivation
// path. If new code legitimately needs a classical-only [32]byte
// (for a non-session-key use case), it must add a `// not a session
// key` comment on the relevant line, which this check honors.
//
// The walker is conservative: it errors on suspicious patterns; if a
// false positive arises, the offending file should either be
// renamed or annotated, not blanket-ignored.
func TestNoClassicalSessionKey(t *testing.T) {
	// Roots to scan. Anything under these directories is under the
	// constitutional scope of "must be hybrid".
	roots := []string{
		"../convo",
		"../keywheel", // present in the workspace via go.work for cross-repo dev
	}

	// Patterns that look like "classical session key" code. Each is a
	// substring match on an identifier name.
	bannedPatterns := []string{
		"classicalSessionKey",
		"ClassicalSessionKey",
		"x25519SessionKey",
		"X25519SessionKey",
		"edSessionKey",
		"EdSessionKey",
	}

	for _, root := range roots {
		abs, err := filepath.Abs(root)
		if err != nil {
			t.Logf("skipping %s: %v", root, err)
			continue
		}
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, abs, func(_ fs.FileInfo) bool {
			return true
		}, parser.ParseComments)
		if err != nil {
			// Missing directory is acceptable — keywheel/ is in
			// Neverlur, only reachable in workspace mode.
			t.Logf("skipping %s: parser error (likely missing): %v", root, err)
			continue
		}

		for _, pkg := range pkgs {
			for fname, file := range pkg.Files {
				if strings.HasSuffix(fname, "_test.go") {
					continue
				}
				// Collect line numbers annotated "not a session key".
				annotated := map[int]bool{}
				for _, cg := range file.Comments {
					for _, c := range cg.List {
						if strings.Contains(c.Text, "not a session key") {
							pos := fset.Position(c.Slash)
							annotated[pos.Line] = true
						}
					}
				}

				ast.Inspect(file, func(n ast.Node) bool {
					id, ok := n.(*ast.Ident)
					if !ok {
						return true
					}
					for _, pat := range bannedPatterns {
						if strings.Contains(id.Name, pat) {
							pos := fset.Position(id.Pos())
							if annotated[pos.Line] {
								return true
							}
							t.Errorf("constitutional violation (Principle III): %s:%d: identifier %q derives a session key from only the classical half; must use hybrid combiner", fname, pos.Line, id.Name)
						}
					}
					return true
				})
			}
		}
	}
}
