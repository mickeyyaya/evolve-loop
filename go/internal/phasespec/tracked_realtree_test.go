package phasespec

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
)

// TrackedPhaseDirs returns the .evolve/phases subdirs whose phase.json git tracks, or nil (bind all) when git state is unusable.
// It is exported from a _test.go file so the external phasespec_test package can call it too.
func TrackedPhaseDirs(t *testing.T, projectRoot string) map[string]bool {
	t.Helper()
	phasesDir := filepath.Join(projectRoot, ".evolve", "phases")
	entries, err := os.ReadDir(phasesDir)
	if err != nil {
		t.Logf("TrackedPhaseDirs: read %s: %v — binding all on-disk phases", phasesDir, err)
		return nil
	}
	set := map[string]bool{}
	sawDir := false
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sawDir = true
		// TrackedFiles lists only direct children, so each subdirectory is asked separately.
		files, ferr := repostate.TrackedFiles(projectRoot, filepath.Join(".evolve", "phases", e.Name()))
		if ferr != nil {
			t.Logf("TrackedPhaseDirs: %v — binding all on-disk phases", ferr)
			return nil
		}
		for _, f := range files {
			if filepath.Base(f) == userSpecFile {
				set[e.Name()] = true
			}
		}
	}
	// An empty filter would silently unbind every gate, so fall back to binding all.
	if sawDir && len(set) == 0 {
		t.Logf("TrackedPhaseDirs: empty tracked set under %s — misresolved root or sparse checkout; binding all on-disk phases", phasesDir)
		return nil
	}
	return set
}

// TrackedUserPhaseNames returns the catalog names of tracked phase dirs (dir name plus declared name), or nil to bind all.
func TrackedUserPhaseNames(t *testing.T, projectRoot string) map[string]bool {
	t.Helper()
	dirs := TrackedPhaseDirs(t, projectRoot)
	if dirs == nil {
		return nil
	}
	names := make(map[string]bool, len(dirs))
	for dir := range dirs {
		names[dir] = true
		raw, err := os.ReadFile(filepath.Join(projectRoot, ".evolve", "phases", dir, userSpecFile))
		if err != nil {
			continue
		}
		var s struct {
			Name string `json:"name"`
		}
		if json.Unmarshal(raw, &s) == nil && s.Name != "" {
			names[s.Name] = true
		}
	}
	return names
}

// trackedRepoProfileNames returns the git-tracked profile names, or nil (bind all) when git state is unusable.
func trackedRepoProfileNames(t *testing.T, projectRoot string) map[string]bool {
	t.Helper()
	set, err := repostate.TrackedSet(projectRoot, ".evolve/profiles", ".json")
	if err == nil && len(set) == 0 {
		err = fmt.Errorf("empty tracked-profile set under %s — pathspec matched nothing", projectRoot)
	}
	if err != nil {
		t.Logf("trackedRepoProfileNames: %v — binding all on-disk profiles", err)
		return nil
	}
	return set
}

func writePhaseSpec(t *testing.T, root, dir string, spec []byte) {
	t.Helper()
	p := filepath.Join(root, ".evolve", "phases", dir)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p, userSpecFile), spec, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTrackedPhaseDirs_FixtureRepo(t *testing.T) {
	repo := gittest.Fixture(t)
	writePhaseSpec(t, repo.Dir, "alpha", []byte(`{"name":"alpha"}`))
	repo.Git("add", ".evolve/phases/alpha/phase.json")
	writePhaseSpec(t, repo.Dir, "zz-mint", []byte(`{"name":"zz-mint"}`))
	if err := os.MkdirAll(filepath.Join(repo.Dir, ".evolve", "phases", "no-spec"), 0o755); err != nil {
		t.Fatal(err)
	}

	set := TrackedPhaseDirs(t, repo.Dir)
	if set == nil {
		t.Fatal("TrackedPhaseDirs = nil on a healthy fixture repo — filter must be active")
	}
	if !set["alpha"] {
		t.Error("tracked phase dir alpha missing from the binding set")
	}
	if set["zz-mint"] {
		t.Error("untracked minted phase dir bound — the cd49274beab2 false-RED class is re-armed")
	}
	if set["no-spec"] {
		t.Error("dir without phase.json is not a phase definition and must not bind")
	}

	if got := TrackedPhaseDirs(t, t.TempDir()); got != nil {
		t.Errorf("TrackedPhaseDirs(non-repo) = %v, want nil (loud bind-all fallback)", got)
	}
}

func TestTrackedPhaseDirs_RealTrackedSetExcludesAnUntrackedDecoyWithoutTouchingTheRealTree(t *testing.T) {
	real := TrackedPhaseDirs(t, repoRoot())
	if real == nil {
		t.Skip("no usable git context — filter disabled (bind-all fallback), nothing to prove")
	}
	if len(real) == 0 {
		t.Fatal("tracked phase set empty on the real tree")
	}
	repo := gittest.Fixture(t)
	for dir := range real {
		spec, err := os.ReadFile(filepath.Join(repoRoot(), ".evolve", "phases", dir, userSpecFile))
		if err != nil {
			t.Fatal(err)
		}
		writePhaseSpec(t, repo.Dir, dir, spec)
	}
	repo.Git("add", ".evolve/phases")
	const decoy = "zz-decoy-phasespec-funnel"
	writePhaseSpec(t, repo.Dir, decoy, []byte(`{"name":"`+decoy+`"}`))

	if set := TrackedPhaseDirs(t, repo.Dir); !maps.Equal(set, real) {
		t.Fatalf("TrackedPhaseDirs over a copy of the real tracked set plus an untracked decoy = %v, want exactly the real set %v", set, real)
	}
	if names := TrackedUserPhaseNames(t, repo.Dir); names == nil || names[decoy] {
		t.Fatalf("TrackedUserPhaseNames over the copy = %v: want an active filter without the untracked decoy %q", names, decoy)
	}
}
