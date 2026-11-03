package integration

import (
	"context"
	"testing"
)

func TestDeliverableRecordHoldsCommit_NamesOnlyItsOwnCommits(t *testing.T) {
	t.Parallel()
	r := fullRecord()
	for _, sha := range []string{"a1", "a2"} {
		if !r.HoldsCommit(sha) {
			t.Errorf("HoldsCommit(%q) = false, want true for a recorded commit", sha)
		}
	}
	for _, sha := range []string{"b0", "", "p1"} {
		if r.HoldsCommit(sha) {
			t.Errorf("HoldsCommit(%q) = true, want false: a base, an empty id or a patch-id is not a recorded commit", sha)
		}
	}
}

func TestGitFunc_IsThePortThatRepoCallsWithItsDirectory(t *testing.T) {
	t.Parallel()
	var gotDir string
	var git GitFunc = func(_ context.Context, dir, _ string, _ ...string) (string, error) {
		gotDir = dir
		return "", nil
	}
	if _, err := (Repo{Dir: "/the/repo", Git: git}).PatchID(context.Background(), "0123456789abcdef0123456789abcdef01234567"); err == nil {
		t.Fatal("an empty diff must be an error")
	}
	if gotDir != "/the/repo" {
		t.Fatalf("GitFunc got dir %q, want the Repo directory", gotDir)
	}
}
