package core

import (
	"context"
	"strings"
	"testing"
)

func carryGitE4(failOn string) *scriptedGit {
	return &scriptedGit{respond: func(args []string) (string, int, error) {
		verb := joined(args)
		if failOn != "" && strings.Contains(verb, failOn) {
			return "", 1, nil
		}
		switch {
		case strings.HasPrefix(verb, "write-tree"):
			return "tree-sha\n", 0, nil
		case strings.HasPrefix(verb, "merge-base HEAD main"):
			return "base-sha\n", 0, nil
		case strings.Contains(verb, "commit-tree"):
			return "carrier-sha\n", 0, nil
		}
		return "", 0, nil
	}}
}

func TestCarryResolvedTree_EachFailingStepStopsTheCarryBeforeTheReset(t *testing.T) {
	t.Parallel()
	cases := []struct {
		failOn  string
		wantErr string
	}{
		{"add -u", "git add -u: exit 1"},
		{"write-tree", "git write-tree: exit 1"},
		{"merge-base HEAD main", "git merge-base HEAD main: exit 1"},
		{"commit-tree", "commit-tree tree-sha -p base-sha"},
		{"reset --soft", "git reset --soft carrier-sha: exit 1"},
	}
	for _, tc := range cases {
		t.Run(tc.failOn, func(t *testing.T) {
			g := carryGitE4(tc.failOn)

			err := carryResolvedTree(context.Background(), "/wt", g.capture, "cycle-4/run-4")

			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("carry err = %v, want it to name %q", err, tc.wantErr)
			}
			if last := joined(g.calls[len(g.calls)-1]); !strings.Contains(last, tc.failOn) {
				t.Fatalf("the carry ran %q after the failing %q: %v", last, tc.failOn, g.calls)
			}
		})
	}
}

func TestCarryResolvedTree_CommitsTheResolvedTreeOnTheForkPointAndPendsIt(t *testing.T) {
	t.Parallel()
	g := carryGitE4("")

	if err := carryResolvedTree(context.Background(), "/wt", g.capture, "cycle-4/run-4"); err != nil {
		t.Fatalf("carry: %v", err)
	}
	for _, want := range []string{"commit-tree tree-sha -p base-sha", "Evolve-Carrier: cycle-4/run-4", "reset --soft carrier-sha"} {
		if !g.ran(want) {
			t.Errorf("git calls lack %q: %v", want, g.calls)
		}
	}
}
