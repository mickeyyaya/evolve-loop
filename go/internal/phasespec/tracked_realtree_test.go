package phasespec

// tracked_realtree_test.go — the ONE funnel for real-tree phase-catalog scans.
//
// The unit of the phase catalog is a SUBDIRECTORY carrying phase.json under
// .evolve/phases. The runtime can mint untracked phase dirs (and untracked
// profile stubs under .evolve/profiles) into the live tree; a scanner that
// binds everything on disk reds on state that can never reach a CI checkout —
// the 2026-08-09 zero-ship batch, fingerprint cd49274beab2
// (docs/incidents/2026-08-09-zero-ship-batch.md). Real-tree tests in this
// package (and in phasespec_test) must derive their iteration set from these
// helpers so future tests inherit the tracked-only filter.
//
// TrackedPhaseDirs / TrackedUserPhaseNames are EXPORTED although they live in
// a _test.go file (the export_test.go idiom): call sites span both package
// phasespec (repo_phaseconfigs_test.go, userphases_validate_test.go) and the
// external package phasespec_test (catalog_metadata_test.go,
// usercatalog_research_test.go), and the external test package compiles
// against the test-augmented package.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
)

// TrackedPhaseDirs returns the set of .evolve/phases/<sub> directory names
// whose phase.json git tracks (incl. index-staged) under projectRoot, or nil
// when git state is unusable — callers MUST treat nil as "no filter" and bind
// every on-disk phase dir (the stricter fallback). An unexpectedly EMPTY set
// on a tree that has phase dirs is treated as a failure too: going dark would
// unbind the gates (mirrors phasecoherence/unpaired_test.go).
//
// repostate.TrackedFiles returns only DIRECT children of a dir, so the
// nested phase.json files are found by asking per subdirectory — simple and
// readable at ~70 dirs.
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
	if sawDir && len(set) == 0 {
		t.Logf("TrackedPhaseDirs: empty tracked set under %s — misresolved root or sparse checkout; binding all on-disk phases", phasesDir)
		return nil
	}
	return set
}

// TrackedUserPhaseNames returns the effective CATALOG names contributed by
// tracked phase dirs: the dir name plus a declared phase.json "name" when
// present (mirroring DiscoverUserSpecs's dir-name default). Nil means "no
// filter" (bind all), exactly like TrackedPhaseDirs. Use this to filter
// user/overlay entries of a merged catalog; built-in registry entries are
// always kept by callers.
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
			continue // discovery-level concerns; the dir-name default stands
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

// trackedRepoProfileNames returns the basenames (sans .json) of the profiles
// git tracks under projectRoot/.evolve/profiles, or nil (= no filter) when
// git state is unusable — same contract as TrackedPhaseDirs. Used by the
// profile-schema raw reads in userphases_validate_test.go.
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

// TestTrackedPhaseDirs_FixtureRepo pins the helper's semantics on a throwaway
// git repo: a dir with a tracked phase.json binds; an untracked mint and a
// dir without phase.json do not; a non-repo dir yields nil (loud bind-all).
func TestTrackedPhaseDirs_FixtureRepo(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	writePhase := func(dir string) {
		t.Helper()
		p := filepath.Join(root, ".evolve", "phases", dir)
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(p, userSpecFile), []byte(`{"name":"`+dir+`"}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writePhase("alpha")
	git("add", ".evolve/phases/alpha/phase.json")
	writePhase("zz-mint") // never added: the runtime-mint shape
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "phases", "no-spec"), 0o755); err != nil {
		t.Fatal(err)
	}

	set := TrackedPhaseDirs(t, root)
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

// TestTrackedPhaseDirs_RealTreeExcludesUntrackedDecoy is the live regression
// proof: it plants an untracked decoy phase dir in the REAL .evolve/phases
// and asserts the funnel does not bind it while still binding the tracked
// catalog.
func TestTrackedPhaseDirs_RealTreeExcludesUntrackedDecoy(t *testing.T) {
	root := repoRoot()
	if TrackedPhaseDirs(t, root) == nil {
		t.Skip("no usable git context — filter disabled (bind-all fallback), nothing to prove")
	}
	const decoy = "zz-decoy-phasespec-funnel"
	dir := filepath.Join(root, ".evolve", "phases", decoy)
	if _, err := os.Stat(dir); err == nil {
		t.Fatalf("%s already exists — refusing to clobber", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	if err := os.WriteFile(filepath.Join(dir, userSpecFile), []byte(`{"name":"`+decoy+`"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	set := TrackedPhaseDirs(t, root)
	if set == nil {
		t.Fatal("filter went dark after planting a decoy — must stay active")
	}
	if len(set) == 0 {
		t.Fatal("tracked phase set empty on the real tree")
	}
	if set[decoy] {
		t.Fatalf("untracked decoy dir %q bound by TrackedPhaseDirs — the cd49274beab2 false-RED class is re-armed", decoy)
	}
	if names := TrackedUserPhaseNames(t, root); names != nil && names[decoy] {
		t.Fatalf("untracked decoy name %q bound by TrackedUserPhaseNames", decoy)
	}
}
