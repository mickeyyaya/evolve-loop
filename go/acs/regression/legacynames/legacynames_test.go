//go:build acs

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

func TestManifestValidates(t *testing.T) {
	m, _ := loadManifest(t)
	if err := m.Validate(); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
	if len(m.Forbidden) == 0 {
		t.Fatal("manifest declares no forbidden tokens — the guard would be a no-op")
	}
}
