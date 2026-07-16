//go:build acs

// Package protectedsurface is the L4 durable guard (architecture review
// 2026-07-16): every "gate-shaped" Go file in the repo must be covered by the
// protected-surface manifest, so a NEW gate or guard file can never sit
// silently OUTSIDE the control-plane write boundary.
//
// Why this guard exists: guards.IsProtectedSurface denies in-cycle writes
// against guards.ProtectedSurfaceManifest (go/internal/guards/
// integrity_surface.go) — the SSOT for the pipeline integrity control plane.
// Before L4 that list was a narrow hardcoded literal, so a new gate file
// created outside the listed fragments was writable by the very cycle it
// judges: the trust kernel's perimeter rotted as it grew. This predicate makes
// perimeter growth LOUD — when a gate-shaped file appears that the manifest
// does not cover, the audit REDs until an operator extends the manifest via a
// human-gated manual ship (the manifest itself is protected surface, so no
// autonomous cycle can both add a gate and quietly bless it).
//
// "Gate-shaped" is a deliberate, mechanical class: every .go file under the
// known control-plane directories (go/internal/guards, go/internal/commitgate,
// go/internal/phaseintegrity, go/acs/regression), plus any file named
// *_gate.go or *guard*.go anywhere under go/internal. Coverage is checked by
// calling the REAL guards.IsProtectedSurface — this package holds no duplicate
// fragment list, so it can never drift from the boundary it verifies.
package protectedsurface

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/guards"
	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
)

// gateDirs are the repo-relative control-plane directories whose EVERY .go
// file is gate-shaped regardless of filename. A missing dir fails the walk
// loudly — a renamed control-plane package must never let the guard pass
// vacuously.
var gateDirs = []string{
	"go/internal/guards",
	"go/internal/commitgate",
	"go/internal/phaseintegrity",
	"go/acs/regression",
}

// nameScanRoot is the repo-relative root scanned for gate-shaped FILENAMES
// (*_gate.go / *guard*.go) outside the gateDirs.
const nameScanRoot = "go/internal"

// TestEveryGateShapedFileIsProtectedSurface is the durable guard: every
// gate-shaped Go file must be covered by guards.ProtectedSurfaceManifest. A
// newly added gate/guard file outside the manifest fails HERE, at
// ship-gate/CI time, instead of staying silently in-cycle-writable.
func TestEveryGateShapedFileIsProtectedSurface(t *testing.T) {
	root := acsassert.RepoRoot(t)
	uncovered, err := uncoveredGateFiles(root)
	if err != nil {
		t.Fatalf("scan for gate-shaped files: %v", err)
	}
	for _, rel := range uncovered {
		t.Errorf("gate-shaped file outside the protected-surface manifest: %s — "+
			"a cycle can silently edit this gate/guard (L4 perimeter rot). Add a covering "+
			"entry to guards.ProtectedSurfaceManifest (go/internal/guards/integrity_surface.go) "+
			"via an operator-gated `evolve ship --class manual`, or rename the file if it is "+
			"genuinely not a gate.", rel)
	}
}

// knownGateFiles anchor the anti-vacuity check: one file per detection lane
// (dir lanes + both filename lanes), including this predicate itself. If the
// walker stops seeing any of them it silently broke, and the guard above would
// pass vacuously against any regression.
var knownGateFiles = []string{
	"go/internal/guards/integrity_surface.go",               // dir lane: the guards + the manifest SSOT
	"go/internal/commitgate/commitgate.go",                  // dir lane: the commit gate
	"go/internal/phaseintegrity/source.go",                  // dir lane: ADR-0065 integrity chain
	"go/acs/regression/protectedsurface/predicates_test.go", // dir lane: this tripwire protects itself
	"go/internal/cli/guardcmd/commit_prefix_gate.go",        // *_gate.go filename lane
	"go/internal/core/workspace_guard.go",                   // *guard*.go filename lane
}

// TestWalkerStillSeesKnownGateFiles is the anti-vacuity check: the scanner
// must find every known anchor file.
func TestWalkerStillSeesKnownGateFiles(t *testing.T) {
	root := acsassert.RepoRoot(t)
	files, err := gateShapedFiles(root)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	seen := make(map[string]bool, len(files))
	for _, f := range files {
		seen[f] = true
	}
	for _, want := range knownGateFiles {
		if !seen[want] {
			t.Errorf("walker no longer sees known gate file %s — the detector silently "+
				"broke (the coverage guard would pass vacuously); fix the walk or update "+
				"knownGateFiles with a deliberate, reviewed reason.", want)
		}
	}
}

// TestClassifier_MutationProof proves the filename classifier actually bites:
// it must flag the *_gate.go and *guard*.go classes (case-folded, like
// IsProtectedSurface's path matching) while NOT flagging near-misses — the
// suffix class is `_gate.go`, not every mention of "gate" (evalgate.go,
// topngate's gate.go are dir-level protection decisions, not name-class hits).
func TestClassifier_MutationProof(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"ship_gate.go", true},
		{"commit_prefix_gate.go", true},
		{"workspace_guard.go", true},
		{"binaryguard.go", true},
		{"orchestrator_guard_test.go", true},
		{"safeguard.go", true},        // the *guard*.go class is deliberately broad
		{"SHIP_GATE.GO", true},        // case-insensitive FS parity (M1)
		{"gate.go", false},            // no underscore — not the *_gate.go class
		{"evalgate.go", false},        // "gate" mention without the suffix class
		{"gates_test.go", false},      // plural near-miss
		{"guard.md", false},           // not a Go file
		{"vanguard_notes.txt", false}, // not a Go file
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isGateShapedName(tc.name); got != tc.want {
				t.Errorf("isGateShapedName(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

// TestPredicate_SelfProof_SyntheticUncoveredGateFileTrips injects a fake gate
// file into a synthetic tree and proves the REAL production path
// (uncoveredGateFiles → guards.IsProtectedSurface) flags exactly it: covered
// neighbors in manifest-protected dirs stay quiet, the uncovered fake trips.
// This is the guard's own red test — it can never rot into a scanner that
// finds nothing and passes.
func TestPredicate_SelfProof_SyntheticUncoveredGateFileTrips(t *testing.T) {
	root := t.TempDir()
	files := []string{
		// Covered: inside manifest-protected dirs (dir lanes must stay quiet).
		"go/internal/guards/role.go",
		"go/internal/commitgate/commitgate.go",
		"go/internal/phaseintegrity/source.go",
		"go/acs/regression/fake/predicates_test.go",
		// Covered: filename lane hit inside a manifest-covered dir.
		"go/internal/acssuite/tagguard_test.go",
		// Ordinary source: not gate-shaped, must not even be scanned in.
		"go/internal/core/orchestrator.go",
		// THE INJECTION: gate-shaped by name, outside every manifest fragment.
		"go/internal/newpkg/sneaky_gate.go",
	}
	for _, rel := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(p, []byte("package p\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	uncovered, err := uncoveredGateFiles(root)
	if err != nil {
		t.Fatalf("uncoveredGateFiles: %v", err)
	}
	want := []string{"go/internal/newpkg/sneaky_gate.go"}
	if len(uncovered) != len(want) || uncovered[0] != want[0] {
		t.Fatalf("uncoveredGateFiles = %v, want %v — the tripwire must flag exactly "+
			"the injected uncovered gate file", uncovered, want)
	}
}

// uncoveredGateFiles returns the repo-relative gate-shaped .go files under
// root that guards.IsProtectedSurface does NOT cover — the production seam
// both the real-tree guard and the synthetic self-proof drive.
func uncoveredGateFiles(root string) ([]string, error) {
	files, err := gateShapedFiles(root)
	if err != nil {
		return nil, err
	}
	var uncovered []string
	for _, rel := range files {
		if !guards.IsProtectedSurface(rel) {
			uncovered = append(uncovered, rel)
		}
	}
	return uncovered, nil
}

// gateShapedFiles walks root and returns every gate-shaped .go file as a
// sorted, deduped, repo-relative slash path: all .go files under gateDirs plus
// every isGateShapedName hit under nameScanRoot.
func gateShapedFiles(root string) ([]string, error) {
	seen := map[string]bool{}
	collect := func(dir string, nameFilter func(string) bool) error {
		return filepath.WalkDir(filepath.Join(root, filepath.FromSlash(dir)),
			func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || !strings.HasSuffix(path, ".go") {
					return nil
				}
				if nameFilter != nil && !nameFilter(d.Name()) {
					return nil
				}
				rel, rerr := filepath.Rel(root, path)
				if rerr != nil {
					return rerr
				}
				seen[filepath.ToSlash(rel)] = true
				return nil
			})
	}
	for _, dir := range gateDirs {
		if err := collect(dir, nil); err != nil {
			return nil, err
		}
	}
	if err := collect(nameScanRoot, isGateShapedName); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(seen))
	for rel := range seen {
		out = append(out, rel)
	}
	sort.Strings(out)
	return out, nil
}

// isGateShapedName reports whether a bare filename marks a gate/guard file
// regardless of directory: *_gate.go or *guard*.go, case-folded for parity
// with IsProtectedSurface's case-insensitive-filesystem matching.
func isGateShapedName(name string) bool {
	n := strings.ToLower(name)
	if !strings.HasSuffix(n, ".go") {
		return false
	}
	return strings.HasSuffix(n, "_gate.go") || strings.Contains(n, "guard")
}
