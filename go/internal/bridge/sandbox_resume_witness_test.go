package bridge

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestEveryAgentProfileRunsInTheSandbox(t *testing.T) {
	t.Parallel()
	dir := realProfilesDir(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	agents := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		var peek struct {
			Name    string          `json:"name"`
			Sandbox *ProfileSandbox `json:"sandbox"`
		}
		if err := json.Unmarshal(raw, &peek); err != nil {
			t.Fatalf("%s: %v", e.Name(), err)
		}
		if peek.Name == "" {
			continue
		}
		agents++
		if peek.Sandbox == nil || !peek.Sandbox.Enabled {
			t.Errorf("%s runs unconfined: an unsandboxed agent can write the host resume witness and every other host file", e.Name())
		}
	}
	if agents < 50 {
		t.Fatalf("only %d agent profiles found under %s", agents, dir)
	}
}

func TestNoProfileCanWriteTheResumeWitness(t *testing.T) {
	t.Parallel()
	var unlaunchable []string
	for name, prof := range realSandboxProfiles(t) {
		root := t.TempDir()
		grants, err := resolveSandboxWriteGrants(prof.Sandbox.WriteSubpaths, root, t.TempDir(), filepath.Join(root, ".evolve", "runs", "cycle-7"))
		if err != nil {
			t.Fatalf("%s: the resolver refused the profile's own declarations: %v", name, err)
		}
		denies, err := resolveSandboxDenials(prof.Sandbox.DenySubpaths, root, "", true)
		if err != nil {
			unlaunchable = append(unlaunchable, name)
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
	slices.Sort(unlaunchable)
	if !slices.Equal(unlaunchable, []string{"orchestrator.json"}) {
		t.Fatalf("profiles whose denials do not resolve, so the launcher refuses them: %v; each other profile must pass the witness check", unlaunchable)
	}
}
