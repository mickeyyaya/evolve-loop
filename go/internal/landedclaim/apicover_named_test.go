package landedclaim

import (
	"context"
	"io"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

func TestCheck_RefusesAMalformedClaimBeforeAnyGitRun(t *testing.T) {
	t.Parallel()
	ran := 0
	git := gitexec.Git{Exec: func(context.Context, string, string, []string, []string, io.Reader, io.Writer, io.Writer) (int, error) {
		ran++
		return 0, nil
	}}
	var check Check = OnMain(git)

	cases := map[string]struct {
		sha   string
		paths []string
	}{
		"a short sha":      {"abc12", []string{"go/a.go"}},
		"an upper sha":     {"ABCDEF1", []string{"go/a.go"}},
		"no declared path": {"abcdef1", nil},
	}
	for name, tc := range cases {
		if check(tc.sha, tc.paths) {
			t.Errorf("%s: a malformed claim was answered landed", name)
		}
	}
	if ran != 0 {
		t.Errorf("git ran %d time(s) for malformed claims, want 0", ran)
	}
}
