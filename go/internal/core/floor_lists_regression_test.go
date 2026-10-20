package core

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func regressionInput(t *testing.T, enforce, strict string) ReviewInput {
	t.Helper()
	repo, base := floorListRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", ".apicover-enforce"), enforce)
	writeFile(t, filepath.Join(repo.Dir, "go", ".cover-strict"), strict)
	return ReviewInput{Worktree: repo.Dir, WorktreeBaseSHA: base}
}

func TestFloorListRegressions_RefusesADroppedOrLoweredLineAndAcceptsARaise(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		enforce, strict string
		want            []string
	}{
		"an unchanged list":   {"./p\n", "./p 100\n", nil},
		"an added line":       {"./p\n./q\n", "./p 100\n./q 90\n", nil},
		"a respelled pattern": {"p/...\n", "./p 100\n", nil},
		"a commented line":    {"#./p\n", "./p 100\n", []string{"go/.apicover-enforce drops ./p"}},
		"a dropped apicover":  {"./q\n", "./p 100\n", []string{"go/.apicover-enforce drops ./p"}},
		"a dropped floor":     {"./p\n", "./q 100\n", []string{"go/.cover-strict drops the floor of ./p (100 at the base)"}},
		"a lowered floor":     {"./p\n", "./p 10\n", []string{"go/.cover-strict lowers the floor of ./p from 100 to 10"}},
		"a malformed floor":   {"./p\n", "./p 100\nbogus\n", []string{"go/.cover-strict does not parse"}},
		"a deleted list file": {"", "./p 100\n", []string{"go/.apicover-enforce drops ./p"}},
	} {
		t.Run(name, func(t *testing.T) {
			in := regressionInput(t, tc.enforce, tc.strict)
			if tc.enforce == "" {
				if err := os.Remove(filepath.Join(in.Worktree, "go", ".apicover-enforce")); err != nil {
					t.Fatal(err)
				}
			}
			got := floorListRegressions(context.Background(), in)
			if len(got) != len(tc.want) {
				t.Fatalf("regressions = %q, want %q", got, tc.want)
			}
			for i, w := range tc.want {
				if !strings.Contains(got[i], w) {
					t.Errorf("regression %d = %q, want it to hold %q", i, got[i], w)
				}
			}
		})
	}
}

func TestFloorListRegressions_NoWorktreeOrAnUnreadableBaseIsNoRegression(t *testing.T) {
	if got := floorListRegressions(context.Background(), ReviewInput{}); got != nil {
		t.Fatalf("regressions = %q without a worktree", got)
	}
	in := regressionInput(t, "./q\n", "./p 10\n")
	in.WorktreeBaseSHA = "0000000000000000000000000000000000000000"
	var got []string
	stderr := captureStderr(t, func() { got = floorListRegressions(context.Background(), in) })
	if got != nil || !strings.Contains(stderr, "the floor list check did not run") {
		t.Fatalf("regressions = %q, stderr = %q: an unreadable base WARNs that the check did not run", got, stderr)
	}
}

func TestDefaultBuildFloorChecks_ADroppedEnforcedPackageGoesBackToTheBuild(t *testing.T) {
	t.Parallel()
	in := regressionInput(t, "./q\n", "./p 100\n")
	in.Phase = string(PhaseBuild)
	in.ProjectRoot = t.TempDir()
	var sink injectedStderr
	in.Stderr = &sink

	var found []string
	for _, f := range DefaultBuildFloorChecks(context.Background(), in) {
		if strings.Contains(f, "floor list") {
			found = append(found, f)
		}
	}
	if !reflect.DeepEqual(len(found), 1) || !strings.Contains(found[0], "go/.apicover-enforce drops ./p") {
		t.Fatalf("floor failures = %q, want the dropped package refused", found)
	}
}

func TestWeakenedFloors_AMalformedBaseWarnsAndTheStrictestTreeLineCounts(t *testing.T) {
	var got []string
	stderr := captureStderr(t, func() { got = weakenedFloors("bogus\n", "./p 10\n", func(string) bool { return false }) })
	if got != nil || !strings.Contains(stderr, "WARN the base go/.cover-strict does not parse") {
		t.Fatalf("regressions = %q, stderr = %q", got, stderr)
	}
	if got := weakenedFloors("./p 100\n", "./p 10\n./p 100\n", func(string) bool { return false }); got != nil {
		t.Errorf("regressions = %q: a tree that still holds the base floor keeps it", got)
	}
}

func TestFloorListRegressions_AnUnreadableTreeListWarnsAndCountsAsDeleted(t *testing.T) {
	in := regressionInput(t, "./p\n", "./p 100\n")
	path := filepath.Join(in.Worktree, "go", ".apicover-enforce")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	var got []string
	stderr := captureStderr(t, func() { got = floorListRegressions(context.Background(), in) })
	if len(got) != 1 || !strings.Contains(got[0], "drops ./p") || !strings.Contains(stderr, "WARN read .apicover-enforce") {
		t.Fatalf("regressions = %q, stderr = %q: an unreadable tree list holds no line of the base", got, stderr)
	}
}

func TestFloorListRegressions_ARemovedPackageMayLeaveBothLists(t *testing.T) {
	t.Parallel()
	in := regressionInput(t, "", "")
	if err := os.Remove(filepath.Join(in.Worktree, "go", ".apicover-enforce")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(in.Worktree, "go", "p")); err != nil {
		t.Fatal(err)
	}
	if got := floorListRegressions(context.Background(), in); got != nil {
		t.Fatalf("regressions = %q: ./p has no directory in the tree, so its lines may go with it", got)
	}
	writeFile(t, filepath.Join(in.Worktree, "go", "p", "a.go"), "package p\n")
	if got := floorListRegressions(context.Background(), in); len(got) != 2 {
		t.Fatalf("regressions = %q, want both drops refused while ./p still exists", got)
	}
}
