package core

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gitexec"
)

type scriptedHeadGit struct {
	branch      string
	branchRC    int
	revParseRC  int
	localSHA    string
	remoteSHA   string
	leftRight   string
	leftRightRC int
}

func (s scriptedHeadGit) run(_ context.Context, _, _ string, args, _ []string, _ io.Reader, outw, _ io.Writer) (int, error) {
	write := func(text string) {
		if outw != nil {
			_, _ = outw.Write([]byte(text))
		}
	}
	switch {
	case args[0] == "symbolic-ref":
		write(s.branch + "\n")
		return s.branchRC, nil
	case args[0] == "rev-parse" && args[1] == "HEAD":
		write(s.localSHA + "\n")
		return s.revParseRC, nil
	case args[0] == "rev-parse":
		write(s.remoteSHA + "\n")
		return 0, nil
	case args[0] == "rev-list":
		write(s.leftRight)
		return s.leftRightRC, nil
	}
	return 0, nil
}

func TestIntegrationHead_ChoosesBaseByRelationOfLocalBranchToRemote(t *testing.T) {
	t.Parallel()
	const localSHA = "1111111111111111111111111111111111111111"
	const remoteSHA = "2222222222222222222222222222222222222222"
	cases := []struct {
		name      string
		git       scriptedHeadGit
		want      string
		wantError string
	}{
		{"detached head keeps the remote tip", scriptedHeadGit{branchRC: 1}, "origin/main", ""},
		{"feature branch keeps the remote tip", scriptedHeadGit{branch: "feature"}, "origin/main", ""},
		{"unreadable relation keeps the remote tip", scriptedHeadGit{branch: "main", revParseRC: 1}, "origin/main", ""},
		{"local ahead by commits no boundary step wrote is refused", scriptedHeadGit{branch: "main", localSHA: localSHA, remoteSHA: remoteSHA, leftRight: "2\t0\n"}, "", "origin does not hold"},
		{"diverged local main is refused", scriptedHeadGit{branch: "main", localSHA: localSHA, remoteSHA: remoteSHA, leftRight: "1\t1\n"}, "", "DIVERGED"},
		{"behind local main fast-forwards to the remote tip", scriptedHeadGit{branch: "main", localSHA: localSHA, remoteSHA: remoteSHA, leftRight: "0\t3\n"}, "origin/main", ""},
		{"current local main uses the remote tip", scriptedHeadGit{branch: "main", localSHA: localSHA, remoteSHA: localSHA, leftRight: "0\t0\n"}, "origin/main", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := integrationHead(context.Background(), gitexec.Git{Dir: t.TempDir(), Exec: tc.git.run}, "origin/main")
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("err = %v, want it to contain %q", err, tc.wantError)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("integrationHead = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}
