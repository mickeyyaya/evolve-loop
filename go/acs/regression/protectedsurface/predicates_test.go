//go:build acs

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

var gateDirs = []string{
	"go/internal/guards",
	"go/internal/commitgate",
	"go/internal/phaseintegrity",
	"go/acs/regression",
}

const nameScanRoot = "go/internal"

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

var knownGateFiles = []string{
	"go/internal/guards/integrity_surface.go",
	"go/internal/commitgate/commitgate.go",
	"go/internal/phaseintegrity/source.go",
	"go/acs/regression/protectedsurface/predicates_test.go",
	"go/internal/cli/guardcmd/commit_prefix_gate.go",
	"go/internal/core/workspace_guard.go",
}

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
		{"safeguard.go", true},
		{"SHIP_GATE.GO", true},
		{"gate.go", false},
		{"evalgate.go", false},
		{"gates_test.go", false},
		{"guard.md", false},
		{"vanguard_notes.txt", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isGateShapedName(tc.name); got != tc.want {
				t.Errorf("isGateShapedName(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestPredicate_SelfProof_SyntheticUncoveredGateFileTrips(t *testing.T) {
	root := t.TempDir()
	files := []string{
		"go/internal/guards/role.go",
		"go/internal/commitgate/commitgate.go",
		"go/internal/phaseintegrity/source.go",
		"go/acs/regression/fake/predicates_test.go",
		"go/internal/acssuite/tagguard_test.go",
		"go/internal/core/orchestrator.go",
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

func isGateShapedName(name string) bool {
	n := strings.ToLower(name)
	if !strings.HasSuffix(n, ".go") {
		return false
	}
	return strings.HasSuffix(n, "_gate.go") || strings.Contains(n, "guard")
}
