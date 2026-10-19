package core

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

const tddPredicateWithComment = "//go:build acs\n\npackage cycle1971\n\n// pins the retry budget.\nfunc TestC1971_001_RetryBudget() {}\n"

func TestCommentFloorFailures_IgnoresTheCyclesTddOwnedPredicateFile(t *testing.T) {
	t.Parallel()

	repo, base := floorRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "acs", "cycle1971", "predicates_test.go"), tddPredicateWithComment)
	in := floorInput(t, floorRoot(t, `{"comment_floor":{"stage":"enforce"}}`), repo, base)
	in.Cycle = 1971

	if failures := commentFloorFailures(context.Background(), in); failures != nil {
		t.Fatalf("the build floor must not count the comments of the predicate file the TDD phase owns: %v", failures)
	}
}

func TestCommentFloorFailures_StillNamesTheBuildsOwnCommentBesideTddOwnedOnes(t *testing.T) {
	t.Parallel()

	repo, base := floorRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "acs", "cycle1971", "predicates_test.go"), tddPredicateWithComment)
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\n// restates the helper.\nfunc a() {}\n")
	in := floorInput(t, floorRoot(t, `{"comment_floor":{"stage":"enforce"}}`), repo, base)
	in.Cycle = 1971

	failures := commentFloorFailures(context.Background(), in)

	if len(failures) != 1 || !strings.Contains(failures[0], "go/p/a.go: // restates the helper.") || strings.Contains(failures[0], "pins the retry budget") {
		t.Fatalf("the floor must name the build's comment only, got %v", failures)
	}
}

func TestCommentFloorFailures_StillCountsAnotherCyclesPredicateFile(t *testing.T) {
	t.Parallel()

	repo, base := floorRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "acs", "cycle1970", "predicates_test.go"), strings.ReplaceAll(tddPredicateWithComment, "cycle1971", "cycle1970"))
	in := floorInput(t, floorRoot(t, `{"comment_floor":{"stage":"enforce"}}`), repo, base)
	in.Cycle = 1971

	failures := commentFloorFailures(context.Background(), in)

	if len(failures) != 1 || !strings.Contains(failures[0], "go/acs/cycle1970/predicates_test.go: // pins the retry budget.") {
		t.Fatalf("only this cycle's TDD-owned files are exempt; another cycle's predicate comment must be named: %v", failures)
	}
}

func TestCommentFloorFailures_ACommentOnlyInTheTddOwnedPredicateFilePassesTheWholeBuildFloor(t *testing.T) {
	t.Parallel()

	repo, base := floorRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "acs", "cycle1971", "predicates_test.go"), tddPredicateWithComment)
	in := floorInput(t, floorRoot(t, `{"comment_floor":{"stage":"enforce"}}`), repo, base)
	in.Cycle = 1971

	for _, failure := range DefaultBuildFloorChecks(context.Background(), in) {
		if strings.Contains(failure, "comment line(s)") {
			t.Fatalf("a cycle whose only comments are TDD's must pass the comment floor of the build floor: %s", failure)
		}
	}
}

func TestCommentFloorFailures_UnboundCycleExemptsNumberedPredicateDirs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		predicate string
		wantNamed bool
	}{
		{"a numbered predicate dir is exempt", "cycle42", false},
		{"the cycle of the fixture body is exempt", "cycle1971", false},
		{"a non numeric dir is counted", "cyclex", true},
		{"a bare cycle dir is counted", "cycle", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, base := floorRepo(t)
			writeFile(t, filepath.Join(repo.Dir, "go", "acs", tc.predicate, "predicates_test.go"), strings.ReplaceAll(tddPredicateWithComment, "cycle1971", tc.predicate))
			in := floorInput(t, floorRoot(t, `{"comment_floor":{"stage":"enforce"}}`), repo, base)
			in.Cycle = 0

			failures := commentFloorFailures(context.Background(), in)

			named := len(failures) == 1 && strings.Contains(failures[0], "go/acs/"+tc.predicate+"/predicates_test.go: // pins the retry budget.")
			if named != tc.wantNamed || (!tc.wantNamed && failures != nil) {
				t.Fatalf("an unbound cycle: named=%v want %v, failures=%v", named, tc.wantNamed, failures)
			}
		})
	}
}

func TestCommentFloorFailures_UnboundCycleStillNamesTheBuildsOwnComment(t *testing.T) {
	t.Parallel()

	repo, base := floorRepo(t)
	writeFile(t, filepath.Join(repo.Dir, "go", "acs", "cycle42", "predicates_test.go"), strings.ReplaceAll(tddPredicateWithComment, "cycle1971", "cycle42"))
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\n// restates the helper.\nfunc a() {}\n")
	in := floorInput(t, floorRoot(t, `{"comment_floor":{"stage":"enforce"}}`), repo, base)
	in.Cycle = 0

	failures := commentFloorFailures(context.Background(), in)

	if len(failures) != 1 || !strings.Contains(failures[0], "go/p/a.go: // restates the helper.") || strings.Contains(failures[0], "pins the retry budget") {
		t.Fatalf("an unbound cycle must still name the build's comment only, got %v", failures)
	}
}
