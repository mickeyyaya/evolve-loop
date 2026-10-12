//go:build integration

package cliroutetest

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

func profileJSONF3(name string) string { return `{"name":"` + name + `"}` }

func TestTrackedProfilesIn_FatalWhenTheProfileDirIsMissing(t *testing.T) {
	requireFatalF3(t, "list profiles", t.TempDir(), func(tb testing.TB) { trackedProfilesIn(tb, t.TempDir()) })
}

func TestTrackedProfilesIn_FatalWhenTheRootIsNotARepository(t *testing.T) {
	root := t.TempDir()
	fileAtF3(t, filepath.Join(root, ".evolve", "profiles", "a.json"), profileJSONF3("a"))

	requireFatalF3(t, "tracked profiles under", t.TempDir(), func(tb testing.TB) { trackedProfilesIn(tb, root) })
}

func TestTrackedProfilesIn_FatalWhenNothingIsTracked(t *testing.T) {
	repo := gittest.Fixture(t)
	fileAtF3(t, filepath.Join(repo.Dir, ".evolve", "profiles", "a.json"), profileJSONF3("a"))

	requireFatalF3(t, "set of 0", t.TempDir(), func(tb testing.TB) { trackedProfilesIn(tb, repo.Dir) })
}

func TestTrackedProfilesIn_KeepsOnlyTheTrackedProfiles(t *testing.T) {
	repo := gittest.Fixture(t)
	fileAtF3(t, filepath.Join(repo.Dir, ".evolve", "profiles", "tracked.json"), profileJSONF3("tracked"))
	fileAtF3(t, filepath.Join(repo.Dir, ".evolve", "profiles", "loose.json"), profileJSONF3("loose"))
	repo.Git("add", ".evolve/profiles/tracked.json")
	repo.Git("commit", "-q", "-m", "track one profile")
	var dir string
	var names []string

	requireCleanF3(t, t.TempDir(), func(tb testing.TB) { dir, names = trackedProfilesIn(tb, repo.Dir) })

	if !slices.Equal(names, []string{"tracked"}) || dir != filepath.Join(repo.Dir, ".evolve", "profiles") {
		t.Fatalf("dir=%s names=%v", dir, names)
	}
}
