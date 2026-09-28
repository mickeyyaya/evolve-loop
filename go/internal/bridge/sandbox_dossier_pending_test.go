package bridge

import (
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/dossier"
)

func TestNoProfileCanWriteTheDossierPendingDir(t *testing.T) {
	for name, prof := range realSandboxProfiles(t) {
		root := t.TempDir()
		grants, err := resolveSandboxWriteGrants(prof.Sandbox.WriteSubpaths, root, t.TempDir())
		if err != nil {
			t.Fatalf("%s: the resolver refused the profile's own declarations: %v", name, err)
		}
		denies, err := resolveSandboxDenials(prof.Sandbox.DenySubpaths, root, "", true)
		if err != nil {
			continue
		}
		canonicalRoot, err := canonicalSandboxPath(root)
		if err != nil {
			t.Fatal(err)
		}
		pending := filepath.Join(dossier.PendingDir(canonicalRoot), "cycle-1705.json")
		if coveredByAGrant(pending, grants) && !coveredByAGrant(pending, denies) {
			t.Errorf("%s can write %s (write_subpaths %v, no deny covers it)", name, pending, prof.Sandbox.WriteSubpaths)
		}
	}
}
