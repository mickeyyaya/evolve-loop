package core

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommentFloorFailures_TddExemptionBoundaries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		dir        string
		cycle      int
		wantListed bool
	}{
		{"longer digits sharing the prefix are not exempt", "cycle19710", 1971, true},
		{"shorter digits sharing the prefix are not exempt", "cycle197", 1971, true},
		{"unbound cycle exempts any numbered predicate dir", "cycle42", 0, false},
		{"unbound cycle does not exempt a non numeric dir", "cyclex", 0, true},
		{"the bound cycle is exempt", "cycle1971", 1971, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, base := floorRepo(t)
			body := strings.ReplaceAll(tddPredicateWithComment, "cycle1971", tc.dir)
			writeFile(t, filepath.Join(repo.Dir, "go", "acs", tc.dir, "predicates_test.go"), body)
			in := floorInput(t, floorRoot(t, `{"comment_floor":{"stage":"enforce"}}`), repo, base)
			in.Cycle = tc.cycle

			failures := commentFloorFailures(context.Background(), in)

			listed := len(failures) == 1 && strings.Contains(failures[0], "go/acs/"+tc.dir+"/predicates_test.go: // pins the retry budget.")
			if listed != tc.wantListed || (!tc.wantListed && failures != nil) {
				t.Fatalf("listed=%v want %v, failures=%v", listed, tc.wantListed, failures)
			}
		})
	}
}

func TestTddCommentFloorFailures_NamedFileWithoutCommentsAndMissingFileAreQuiet(t *testing.T) {
	t.Parallel()

	repo, _ := floorRepo(t)
	clean := "go/acs/cycle1971/clean_test.go"
	writeFile(t, filepath.Join(repo.Dir, filepath.FromSlash(clean)), "//go:build acs\n\npackage cycle1971\n\nfunc TestC1971_009() {}\n")

	got := TddCommentFloorFailures(context.Background(), repo.Dir, floorRoot(t, `{"comment_floor":{"stage":"enforce"}}`), []string{clean, "go/acs/cycle1971/absent_test.go"}, &strings.Builder{})

	if len(got) != 0 {
		t.Fatalf("a comment-free file and an absent file must yield no failures, got %v", got)
	}
}
