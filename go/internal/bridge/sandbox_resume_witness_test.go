package bridge

import (
	"path/filepath"
	"testing"
)

func TestNoProfileCanWriteTheResumeWitness(t *testing.T) {
	t.Parallel()
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
		witness := filepath.Join(canonicalRoot, ".evolve", "resume-witness", "cycle-7.json")
		if coveredByAGrant(witness, grants) && !coveredByAGrant(witness, denies) {
			t.Errorf("%s can write %s (write_subpaths %v, no deny covers it)", name, witness, prof.Sandbox.WriteSubpaths)
		}
	}
}
