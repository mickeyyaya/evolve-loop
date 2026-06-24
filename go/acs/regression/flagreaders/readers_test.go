//go:build acs

// Package flagreaders is a durable ACS regression guard for the flag-reduction
// campaign: every well-formed EVOLVE_* reference on a production surface MUST
// have a flagregistry row. Surfaces scanned: production Go (non-_test.go, outside
// go/acs), CI workflows (.github/), skill/agent instructions (skills/, agents/),
// shell scripts (*.sh anywhere), the go/Makefile, and current-state instruction
// docs at the repo root (AGENTS.md, CLAUDE.md, README.md, …) — but NOT historical
// or policy prose (CHANGELOG.md, SECURITY.md, …), which legitimately names removed
// flags and flag families.
//
// It catches the silent-orphan class — removing a registry entry while a reader
// still exists, or adding a reader without documenting the flag — which the
// registry has no other guard for (the read path does not funnel through the
// registry). It replaces the blunt `len(All) >= 250` count-floor that used to
// (accidentally) block intentional reduction.
//
// Cross-surface scope is load-bearing: cycle-360 removed two "dead" flags that
// still had live readers in adapters/claude.sh because the scan (and the Scout
// grep, and the per-cycle guard) inspected Go ONLY. A Go-only "dead" verdict for
// a flag with a shell/skill/CI reader is a false-dead — the recurring FAIL class
// this guard now forecloses (knowledge-base/research/flag-reduction-campaign-2026-06-18.md).
//
// Discrimination: Go is inspected at the STRING-LITERAL AST level (not comments,
// not substrings); text surfaces are scanned line-by-line with a \b-anchored
// token regex. Both reject mid-sentence mentions ("EVOLVE_FOO is deprecated")
// and dynamic prefixes ("EVOLVE_E2E_MODEL_${cli}" ends in '_'). docs/ and
// control-flags.md are NOT scanned: they catalog every flag by design.
package flagreaders

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/flagregistry"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// flagNameRE matches a complete, well-formed flag name. Anchored, so it never
// matches a flag mentioned inside a longer string, and the (_[A-Z0-9]+)+ tail
// rejects dynamic prefixes that end in '_'.
var flagNameRE = regexp.MustCompile(`^EVOLVE(_[A-Z0-9]+)+$`)

// textFlagRE finds well-formed flag tokens inside a line of a text surface. The
// \b on both ends rejects mid-identifier fragments and dynamic prefixes
// (e.g. EVOLVE_E2E_MODEL_${cli}), mirroring the Go guard's anchored literal match.
var textFlagRE = regexp.MustCompile(`\bEVOLVE(_[A-Z0-9]+)+\b`)

// skipDirs are subtrees whose EVOLVE_* references are not production surfaces:
// vendor/testdata are fixtures; .git is VCS metadata; node_modules is deps;
// .evolve is gitignored runtime state (worktrees/runs), not committed source;
// dist is build output (goreleaser dist/, landing/dist/ — gitignored, generated,
// e.g. landing/dist/install.sh is a copy of the source install.sh).
// ipcenv is the SSOT for IPC protocol constants — its literals ARE the protocol
// values, not readers; it has no flagregistry row by design (cycle-14).
// Matched by basename (a future production dir named "acs" is pruned by PATH in
// the Go walk, not here). _test.go files are skipped per-file.
var skipDirs = map[string]bool{
	"vendor": true, "testdata": true, ".git": true, "node_modules": true, ".evolve": true,
	"ipcenv": true, "dist": true,
}

// textExts are the text-file surfaces scanned in textSurfaceRoots. *.sh is NOT
// here — it is scanned repo-wide by shellExts so a shell script anywhere is
// covered exactly once.
var textExts = map[string]bool{
	".md": true, ".markdown": true, ".yml": true, ".yaml": true,
	".json": true, ".txt": true, ".toml": true,
}

// shellExts drives the repo-wide *.sh scan. The script→Go migration removed all
// shell; this keeps the invariant if shell ever returns (the exact gap that made
// cycle-360's Go-only scan unsafe).
var shellExts = map[string]bool{".sh": true}

// textSurfaceRoots are the non-Go behavioral surfaces scanned relative to repo
// root. docs/ and control-flags.md are intentionally EXCLUDED: they catalog
// flags (every flag name appears there by design), so scanning them would
// re-assert the registry against itself.
var textSurfaceRoots = []string{"skills", "agents", ".github"}

// rootProseExclusions are repo-root .md files that are HISTORICAL or POLICY prose,
// not current-state instructions: CHANGELOG records flag REMOVALS, SECURITY names
// flag families (e.g. EVOLVE_BYPASS_*). They legitimately reference flags with no
// row, so they are exempt. Every OTHER root .md (AGENTS.md, CLAUDE.md, README.md,
// …) documents current usage and must name only registered flags.
var rootProseExclusions = map[string]bool{
	"CHANGELOG.md": true, "SECURITY.md": true, "CODE_OF_CONDUCT.md": true,
	"PRIVACY.md": true, "CONTRIBUTING.md": true,
}

// retiredFlagsByFile maps repo-relative paths (forward-slash) to sets of retired
// flag tokens that must remain in those files for documented backward-compat. A
// token listed here is exempt from orphan detection for that file only. These are
// NOT active readers — the registry row is gone; the reference is documentation.
var retiredFlagsByFile = map[string]map[string]bool{
	// agents/evolve-tester.md preserves the dual-var worktree shell pattern required
	// by the cycle-50/C50_009 regression invariant. EVOLVE_WORKTREE_PATH was retired
	// in cycle-10; the snippet documents the backward-compat form for reference.
	"agents/evolve-tester.md": {"EVOLVE_WORKTREE_PATH": true},
	// skills/adversarial-testing/SKILL.md uses EVOLVE_WORKTREE_BASE as the WORKED
	// EXAMPLE of the cycle-20 split-const dodge (the dodge it teaches reviewers to
	// catch). The dial was legitimately removed in 2026-06 (policy.json worktree.base
	// + WithWorktreeBase DI, ADR-0064); the name survives only as documentation of
	// the historical dodge, not a live reader.
	"skills/adversarial-testing/SKILL.md": {"EVOLVE_WORKTREE_BASE": true},
}

// TestEveryProductionReaderHasRegistryRow fails if any standalone EVOLVE_*
// reference — in production Go, a CI workflow, a skill/agent instruction, or a
// shell script — lacks a flagregistry row. Broadened beyond Go after cycle-360.
func TestEveryProductionReaderHasRegistryRow(t *testing.T) {
	repoRoot := acsassert.RepoRoot(t)
	hasRow := func(name string) bool { _, ok := flagregistry.Lookup(name); return ok }
	orphans := map[string][]string{} // flag name -> surface locations

	skipped, err := collectGoOrphans(filepath.Join(repoRoot, "go"), hasRow, orphans)
	if err != nil {
		t.Fatalf("scan go/: %v", err)
	}
	if len(skipped) > 0 {
		t.Logf("flagreaders: %d unparseable file(s) skipped: %s", len(skipped), strings.Join(skipped, ", "))
	}

	for _, sub := range textSurfaceRoots {
		if err := scanTextTree(filepath.Join(repoRoot, sub), textExts, hasRow, orphans); err != nil {
			t.Fatalf("scan %s/: %v", sub, err)
		}
	}
	if err := scanTextTree(repoRoot, shellExts, hasRow, orphans); err != nil {
		t.Fatalf("scan *.sh: %v", err)
	}
	if err := scanRootMarkdown(repoRoot, hasRow, orphans); err != nil {
		t.Fatalf("scan root *.md: %v", err)
	}
	if err := scanTextFile(filepath.Join(repoRoot, "go", "Makefile"), hasRow, orphans); err != nil {
		t.Fatalf("scan go/Makefile: %v", err)
	}

	for name, locs := range orphans {
		t.Errorf("orphan flag %q is referenced in a production surface but has no flagregistry row — "+
			"add it to go/internal/flagregistry/registry_table.go (sorted) or remove the reference.\n  referenced at: %s",
			name, strings.Join(locs, ", "))
	}
}

// collectGoOrphans walks goDir and records standalone EVOLVE_* string literals
// in non-test production Go that lack a registry row. Returns unparseable files
// (logged, not failed — the compiler catches syntax errors separately, so the
// skip is never silent).
func collectGoOrphans(goDir string, hasRow func(string) bool, orphans map[string][]string) ([]string, error) {
	acsDir := filepath.Join(goDir, "acs")
	fset := token.NewFileSet()
	var skipped []string
	err := filepath.Walk(goDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if path == acsDir || skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		// Parse syntactically (mode 0): no comment scanning, build tags ignored
		// so tagged production files are still covered.
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			skipped = append(skipped, path)
			return nil
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			val, uerr := strconv.Unquote(lit.Value)
			if uerr != nil {
				// Raw (backtick) string literals don't Unquote — strip delimiters
				// so a `EVOLVE_FOO`-style const/map-key read is still seen.
				if len(lit.Value) >= 2 && lit.Value[0] == '`' && lit.Value[len(lit.Value)-1] == '`' {
					val = lit.Value[1 : len(lit.Value)-1]
				} else {
					return true
				}
			}
			if !flagNameRE.MatchString(val) {
				return true
			}
			if !hasRow(val) {
				orphans[val] = append(orphans[val], fset.Position(lit.Pos()).String())
			}
			return true
		})
		return nil
	})
	return skipped, err
}

// scanTextTree walks root and records EVOLVE_* tokens in files whose extension is
// in exts and that lack a registry row. A missing root is not an error (skills/
// or agents/ may be absent in some checkouts).
func scanTextTree(root string, exts map[string]bool, hasRow func(string) bool, orphans map[string][]string) error {
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			if skipDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !exts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		// Wrap hasRow with per-file retired-token exemptions.
		effectiveHasRow := hasRow
		slash := filepath.ToSlash(path)
		for relPath, retired := range retiredFlagsByFile {
			if strings.HasSuffix(slash, "/"+relPath) {
				r := retired
				orig := hasRow
				effectiveHasRow = func(name string) bool { return orig(name) || r[name] }
				break
			}
		}
		return scanTextFile(path, effectiveHasRow, orphans)
	})
	if err != nil {
		return fmt.Errorf("walk %s: %w", root, err)
	}
	return nil
}

// scanTextFile records EVOLVE_* tokens on each line of a text file that lack a
// registry row. A missing file is not an error (callers may scan an optional path
// such as go/Makefile). Binary files (NUL byte) are skipped.
func scanTextFile(path string, hasRow func(string) bool, orphans map[string][]string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil
	}
	for i, line := range strings.Split(string(data), "\n") {
		for _, tok := range textFlagRE.FindAllString(line, -1) {
			// flagNameRE here is a defensive re-validation: textFlagRE's \b anchors
			// already guarantee a well-formed token, but the explicit anchored check
			// keeps scanTextFile correct if textFlagRE is ever loosened.
			if !flagNameRE.MatchString(tok) || hasRow(tok) {
				continue
			}
			orphans[tok] = append(orphans[tok], fmt.Sprintf("%s:%d", path, i+1))
		}
	}
	return nil
}

// scanRootMarkdown scans current-state instruction docs at the repo root (.md
// files), skipping rootProseExclusions (historical/policy prose that legitimately
// names removed flags). Non-recursive: nested docs/ and knowledge-base/ trees are
// reference prose and are not scanned.
func scanRootMarkdown(repoRoot string, hasRow func(string) bool, orphans map[string][]string) error {
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		return fmt.Errorf("readdir %s: %w", repoRoot, err)
	}
	for _, e := range entries {
		if e.IsDir() || strings.ToLower(filepath.Ext(e.Name())) != ".md" || rootProseExclusions[e.Name()] {
			continue
		}
		if err := scanTextFile(filepath.Join(repoRoot, e.Name()), hasRow, orphans); err != nil {
			return err
		}
	}
	return nil
}
