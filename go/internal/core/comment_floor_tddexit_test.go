package core

import (
	"context"
	"io"
	"path/filepath"
	"testing"
)

func TestTddCommentFloorFailures_ListsOnlyTheNamedFilesAndHonoursOff(t *testing.T) {
	t.Parallel()

	repo, _ := floorRepo(t)
	predicate := "go/acs/cycle1971/predicates_test.go"
	writeFile(t, filepath.Join(repo.Dir, filepath.FromSlash(predicate)), tddPredicateWithComment)
	writeFile(t, filepath.Join(repo.Dir, "go", "p", "a.go"), "package p\n\n// restates the helper.\nfunc a() {}\n")

	cases := []struct {
		name   string
		policy string
		files  []string
		want   []string
	}{
		{"enforce names the predicate comment", `{"comment_floor":{"stage":"enforce"}}`, []string{predicate}, []string{predicate + ": // pins the retry budget."}},
		{"shadow still names it", `{"comment_floor":{"stage":"shadow"}}`, []string{predicate}, []string{predicate + ": // pins the retry budget."}},
		{"off is silent", `{"comment_floor":{"stage":"off"}}`, []string{predicate}, nil},
		{"no files is silent", `{"comment_floor":{"stage":"enforce"}}`, nil, nil},
	}
	for _, tc := range cases {
		got := TddCommentFloorFailures(context.Background(), repo.Dir, floorRoot(t, tc.policy), tc.files, io.Discard)
		if len(got) != len(tc.want) || (len(got) == 1 && got[0] != tc.want[0]) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}
