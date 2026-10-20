package ship

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/shiperr"
)

const foreignPredicate = "//go:build acs\n\npackage cycle%d\n"

func TestForeignPredicateFiles_NamesAddedFilesOfOtherCyclesOnly(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"empty diff", "", nil},
		{"own package", "go/acs/cycle1712/predicates_test.go\x00", nil},
		{"two ancestors beside the own package", "go/acs/cycle1707/predicates_test.go\x00go/acs/cycle1708/predicates_test.go\x00go/acs/cycle1712/predicates_test.go\x00",
			[]string{"go/acs/cycle1707/predicates_test.go", "go/acs/cycle1708/predicates_test.go"}},
		{"a longer number with the own prefix", "go/acs/cycle17120/predicates_test.go\x00", []string{"go/acs/cycle17120/predicates_test.go"}},
		{"the curated regression", "go/acs/regression/acsnames/names_test.go\x00", nil},
		{"outside go/acs", "go/internal/acssuite/cycle1707.go\x00docs/private/research/archived-2026-09-26/superseded-predicate-packages/cycle1707/predicates_test.go\x00", nil},
	}
	for _, tc := range cases {
		if got := foreignPredicateFiles(tc.in, 1712); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: foreignPredicateFiles(%q, 1712) = %q, want %q", tc.name, tc.in, got, tc.want)
		}
	}
}

func predicateLane(t *testing.T, cycles ...int) (root, wt, pending string) {
	t.Helper()
	root = docRepo(t)
	wt = filepath.Join(t.TempDir(), "lane")
	gitIn(t, root, "worktree", "add", "-q", "-b", "lane", wt)
	for _, c := range cycles {
		writeDoc(t, wt, fmt.Sprintf("go/acs/cycle%d/predicates_test.go", c), fmt.Sprintf(foreignPredicate, c))
	}
	gitIn(t, wt, "add", "-A")
	tree := gitOut(t, wt, "write-tree")
	pending = gitOut(t, wt, "commit-tree", tree, "-p", "HEAD", "-m", "evolve-cycle 1712")
	return root, wt, pending
}

func TestWorktreeShipLand_RefusesALaneThatLandsAnotherCyclesPredicatePackage(t *testing.T) {
	t.Parallel()
	root, wt, pending := predicateLane(t, 1707, 1712)
	main := gitOut(t, root, "rev-parse", "main")
	opts := &Options{Class: ClassCycle, CycleID: 1712, ProjectRoot: root, Stdout: io.Discard, Stderr: io.Discard}
	s := newWorktreeShip(context.Background(), opts, &RunResult{}, "main", wt)
	s.cycleBranch = "lane"

	err := s.land(pending)

	se := wantShipErr(t, err, shiperr.CodeForeignPredicatePackage, core.ShipClassPrecondition, "go/acs/cycle1707/predicates_test.go")
	if se.Stage != core.StageAtomicShip || se.Debug["paths"] != "go/acs/cycle1707/predicates_test.go" || se.Debug["cycle"] != "1712" {
		t.Errorf("stage %q debug %v: the refusal is an atomic-ship stop that names the path and the shipping cycle", se.Stage, se.Debug)
	}
	if got := gitOut(t, root, "rev-parse", "main"); got != main {
		t.Errorf("main moved to %s: a refused landing leaves main alone", got)
	}
}

func TestRefuseForeignPredicatePackages_TheOwnPackageAndOtherClassesShip(t *testing.T) {
	t.Parallel()
	root, _, own := predicateLane(t, 1712)
	_, _, foreign := predicateLane(t, 1707)
	cases := []struct {
		name   string
		opts   *Options
		commit string
	}{
		{"the shipping cycle's own package", &Options{Class: ClassCycle, CycleID: 1712, ProjectRoot: root}, own},
		{"a manual ship, the console's repair path", &Options{Class: ClassManual, CycleID: 1712, ProjectRoot: root}, foreign},
		{"no cycle id, which the landing intent refuses", &Options{Class: ClassCycle, ProjectRoot: t.TempDir()}, foreign},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := refuseForeignPredicatePackages(context.Background(), tc.opts, "main", tc.commit); err != nil {
				t.Fatalf("want the landing to pass this check: %v", err)
			}
		})
	}
}

func TestRefuseForeignPredicatePackages_AGitFailureFailsClosed(t *testing.T) {
	t.Parallel()
	root := docRepo(t)

	err := refuseForeignPredicatePackages(context.Background(), &Options{Class: ClassCycle, CycleID: 1712, ProjectRoot: root}, "no-such-rev", "main")

	wantShipErr(t, err, core.CodeGitIO, core.ShipClassTransient, "")
}

func TestRefuseForeignPredicatePackages_AnUnreadableCycleStateFailsClosed(t *testing.T) {
	t.Parallel()
	root := docRepo(t)
	writeDoc(t, root, ".evolve/cycle-state.json", "{not json")

	err := refuseForeignPredicatePackages(context.Background(), &Options{Class: ClassCycle, ProjectRoot: root}, "main", "main")

	wantShipErr(t, err, core.CodeStateIO, core.ShipClassTransient, "read the shipping cycle")
}

func TestRefuseForeignPredicatePackages_ARepairOfAnExistingPredicateOfAnotherCycleShips(t *testing.T) {
	t.Parallel()
	root := docRepo(t)
	writeDoc(t, root, "go/acs/cycle1707/predicates_test.go", fmt.Sprintf(foreignPredicate, 1707))
	gitIn(t, root, "add", "-A")
	gitIn(t, root, "commit", "-q", "-m", "an old cycle's predicates")
	wt := filepath.Join(t.TempDir(), "lane")
	gitIn(t, root, "worktree", "add", "-q", "-b", "lane", wt)
	writeDoc(t, wt, "go/acs/cycle1707/predicates_test.go", fmt.Sprintf(foreignPredicate, 1707)+"\nfunc TestRepaired(t *testing.T) {}\n")
	gitIn(t, wt, "add", "-A")
	pending := gitOut(t, wt, "commit-tree", gitOut(t, wt, "write-tree"), "-p", "HEAD", "-m", "evolve-cycle 1712")

	if err := refuseForeignPredicatePackages(context.Background(), &Options{Class: ClassCycle, CycleID: 1712, ProjectRoot: root}, "main", pending); err != nil {
		t.Fatalf("a repair of a predicate that main already holds is no new foreign package: %v", err)
	}
}
