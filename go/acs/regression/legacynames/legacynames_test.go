//go:build acs

// Package legacynames is the durable, config-driven regression guard that no
// dead naming token from a plugin/skill/repo rename survives in any tracked
// file. It replaces hand-written per-token tests with one scan driven by
// .evolve/naming.json — the single source of truth shared with `evolve release`
// preflight and the `evolve names` command. To guard a new dead token, add a
// forbidden row to the manifest; do NOT add a test here.
//
// acs-tagged like every go/acs/regression predicate; CI runs it via
//
//	go test -count=1 -tags acs ./acs/regression/...
//
// It is a test-only package outside ./internal/..., so it needs no
// .apicover-enforce enrollment (same as acs/regression/pluginnamespace).
package legacynames

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/pkg/acsassert"
	"github.com/mickeyyaya/evolve-loop/go/pkg/naminguard"
)

func loadManifest(t *testing.T) (*naminguard.Manifest, string) {
	t.Helper()
	root := acsassert.RepoRoot(t)
	m, err := naminguard.Load(filepath.Join(root, naminguard.DefaultManifestPath))
	if err != nil {
		t.Fatalf("load naming manifest (%s): %v", naminguard.DefaultManifestPath, err)
	}
	return m, root
}

// TestNoForbiddenTokens is the guard: any tracked file containing a dead naming
// token fails CI. This is exactly what would have caught the hyphen-less 404
// slug that lingered in README + landing content after the repo rename.
func TestNoForbiddenTokens(t *testing.T) {
	m, root := loadManifest(t)
	vs, err := naminguard.Scan(root, m)
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(vs) > 0 {
		var b strings.Builder
		for _, v := range vs {
			b.WriteString("\n  " + v.String())
		}
		t.Errorf("%d dead naming token(s) in tracked files — run `evolve names fix` (or add an exclude if historical):%s",
			len(vs), b.String())
	}
}

// TestManifestValidates pins that the SSOT is well-formed: a broken or empty
// manifest must fail loudly here, never silently turn the guard into a no-op.
func TestManifestValidates(t *testing.T) {
	m, _ := loadManifest(t)
	if err := m.Validate(); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
	if len(m.Forbidden) == 0 {
		t.Fatal("manifest declares no forbidden tokens — the guard would be a no-op")
	}
}
