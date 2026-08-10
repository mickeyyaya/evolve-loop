package profiles

// tracked_realtree_test.go — the ONE funnel for real-tree profile scans.
//
// Every test in this package (and in profiles_test) that iterates the LIVE
// .evolve/profiles directory must go through RealTreeProfiles /
// TrackedRealProfileNames so it binds only git-TRACKED profiles. The runtime
// mints untracked profile stubs into the same directory; a scanner that binds
// everything on disk reds on state that can never reach a CI checkout — the
// 2026-08-09 zero-ship batch, fingerprint cd49274beab2
// (docs/incidents/2026-08-09-zero-ship-batch.md).
//
// The helpers are EXPORTED although they live in a _test.go file (the
// export_test.go idiom): call sites span both package profiles
// (profiles_test.go, driver_agnostic_test.go) and the external package
// profiles_test (profile_model_routing_*_test.go), and the external test
// package compiles against the test-augmented package.

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/repostate"
)

// realProfilesDir resolves the on-disk .evolve/profiles directory relative to
// this test file, so the guards run against the live profiles the loop ships
// (not a fixture).
func realProfilesDir(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".evolve", "profiles")
}

// TrackedRealProfileNames returns the basenames (sans .json) of the profiles
// git tracks (incl. index-staged) under the live .evolve/profiles, or nil when
// git state is unusable — callers MUST treat nil as "no filter" and bind every
// on-disk profile (the stricter fallback). An unexpectedly EMPTY set is
// treated as a failure too: a pathspec that matches nothing exits 0, and going
// dark would unbind the gates (mirrors phasecoherence/unpaired_test.go).
func TrackedRealProfileNames(t *testing.T) map[string]bool {
	t.Helper()
	root := filepath.Join(realProfilesDir(t), "..", "..")
	set, err := repostate.TrackedSet(root, ".evolve/profiles", ".json")
	if err == nil && len(set) == 0 {
		err = fmt.Errorf("empty tracked-profile set at %s — pathspec matched nothing (misresolved root or sparse checkout)", root)
	}
	if err != nil {
		t.Logf("TrackedRealProfileNames: %v — binding all on-disk profiles", err)
		return nil
	}
	return set
}

// RealTreeProfiles returns a Loader over the live .evolve/profiles directory
// plus its List() names filtered to git-tracked profiles. Untracked names are
// runtime-minted state, logged and NOT bound (cd49274beab2 class); when git
// context is unusable the full unfiltered list is returned (bind-all
// fallback). New real-tree tests must iterate via this helper.
func RealTreeProfiles(t *testing.T) (*Loader, []string) {
	t.Helper()
	l := NewFromDir(realProfilesDir(t))
	names, err := l.List()
	if err != nil {
		t.Fatalf("List real profiles: %v", err)
	}
	tracked := TrackedRealProfileNames(t)
	if tracked == nil {
		return l, names
	}
	kept := make([]string, 0, len(names))
	for _, n := range names {
		if !tracked[n] {
			t.Logf("untracked profile %q: runtime-minted state, not bound", n)
			continue
		}
		kept = append(kept, n)
	}
	return l, kept
}

// TestRealTreeProfiles_ExcludesUntrackedDecoy is the live regression proof for
// the funnel: it plants an untracked (well-formed) decoy profile in the REAL
// .evolve/profiles directory and asserts RealTreeProfiles does not bind it.
// The decoy is canonical-tier-valid so that even a bind-all environment
// running concurrently would not red on it; the assertion here is purely
// about exclusion.
func TestRealTreeProfiles_ExcludesUntrackedDecoy(t *testing.T) {
	if TrackedRealProfileNames(t) == nil {
		t.Skip("no usable git context — filter disabled (bind-all fallback), nothing to prove")
	}
	const decoy = "zz-decoy-mint-profiles-funnel"
	path := filepath.Join(realProfilesDir(t), decoy+".json")
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("%s already exists — refusing to clobber", path)
	}
	payload := `{"name":"` + decoy + `","role":"decoy","cli":"claude","model_tier_default":"fast"}`
	if err := os.WriteFile(path, []byte(payload), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(path) })

	_, names := RealTreeProfiles(t)
	if len(names) == 0 {
		t.Fatal("filtered real-tree profile list is empty — the funnel went dark instead of filtering")
	}
	for _, n := range names {
		if n == decoy {
			t.Fatalf("untracked decoy %q bound by RealTreeProfiles — the cd49274beab2 false-RED class is re-armed", decoy)
		}
	}
}
