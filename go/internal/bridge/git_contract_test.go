package bridge

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fakecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/gittest"
)

var gitContractSuite = fakecontract.Suite{Tool: "git"}

type gitContractCase struct {
	name       string
	commits    []string
	args       []string
	wantExit   int
	wantFields int
	wantOut    string
}

var gitContractCases = []gitContractCase{
	{name: "rev_parse_head_prints_one_sha", commits: []string{"first", "second"}, args: []string{"rev-parse", "HEAD"}, wantFields: 1},
	{name: "rev_parse_verify_unknown_ref_exits_one", commits: []string{"first"}, args: []string{"rev-parse", "--verify", "--quiet", "nosuchref"}, wantExit: 1},
	{name: "rev_list_one_commit_ahead_prints_one_sha", commits: []string{"first", "second"}, args: []string{"rev-list", "HEAD~1..HEAD"}, wantFields: 1},
	{name: "rev_list_empty_range_prints_nothing", commits: []string{"first", "second"}, args: []string{"rev-list", "HEAD..HEAD"}},
	{name: "rev_list_unknown_base_exits_128", commits: []string{"first"}, args: []string{"rev-list", "nosuchref..HEAD"}, wantExit: 128},
	{name: "log_head_prints_the_message", commits: []string{"first", "second"}, args: []string{"log", "-1", "--format=%B", "HEAD"}, wantOut: "second"},
	{name: "log_unknown_rev_exits_128", commits: []string{"first"}, args: []string{"log", "-1", "--format=%B", "nosuchref"}, wantExit: 128},
	{name: "rev_parse_head_without_commits_exits_128", args: []string{"rev-parse", "HEAD"}, wantExit: 128, wantFields: -1},
}

func gitContractViolations(run CmdRunner, dir string, tc gitContractCase) []string {
	var out strings.Builder
	args := append([]string{"-C", dir}, tc.args...)
	code, err := run(context.Background(), "git", "", args, append(os.Environ(), "GIT_TERMINAL_PROMPT=0"), nil, &out, io.Discard)
	var bad []string
	if err != nil {
		bad = append(bad, fmt.Sprintf("runner returned error %v for exit %d, want a nil error and the exit code", err, tc.wantExit))
	}
	if code != tc.wantExit {
		bad = append(bad, fmt.Sprintf("exit code = %d, want %d", code, tc.wantExit))
	}
	trimmed := strings.TrimSpace(out.String())
	if tc.wantOut == "" && tc.wantFields >= 0 && len(strings.Fields(trimmed)) != tc.wantFields {
		bad = append(bad, fmt.Sprintf("stdout %q has %d fields, want %d", trimmed, len(strings.Fields(trimmed)), tc.wantFields))
	}
	if tc.wantOut != "" && trimmed != tc.wantOut {
		bad = append(bad, fmt.Sprintf("stdout = %q, want %q", trimmed, tc.wantOut))
	}
	bad = append(bad, gitContractCmdViolations(run, dir, tc)...)
	return bad
}

func gitContractCmdViolations(run CmdRunner, dir string, tc gitContractCase) []string {
	ws := filepath.Join(dir, ".contract-ws")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		return []string{err.Error()}
	}
	if err := os.WriteFile(filepath.Join(ws, "challenge-token.txt"), []byte("tok\n"), 0o644); err != nil {
		return []string{err.Error()}
	}
	d := newGitEvidenceDetector(&Config{Workspace: ws, Worktree: dir, Agent: "scout"}, Deps{Runner: run}.withDefaults())
	_, err := d.gitCmd(context.Background(), tc.args...)
	switch {
	case tc.wantExit != 0 && err == nil:
		return []string{fmt.Sprintf("gitCmd returned a nil error for exit %d: a failed git was read as data", tc.wantExit)}
	case tc.wantExit == 0 && err != nil:
		return []string{fmt.Sprintf("gitCmd returned %v for a zero exit", err)}
	}
	return nil
}

func gitContractRepo(t *testing.T, commits []string) string {
	t.Helper()
	repo := gittest.Fixture(t)
	for _, msg := range commits {
		repo.Git("commit", "--allow-empty", "-q", "-m", msg)
	}
	return repo.Dir
}

func TestGitContract(t *testing.T) {
	t.Parallel()
	for _, side := range []string{"fake", "real"} {
		t.Run(side, func(t *testing.T) {
			t.Parallel()
			for _, tc := range gitContractCases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					dir := gitContractRepo(t, tc.commits)
					run := execRunner
					if side == "fake" {
						run = newModelGit(tc.commits...)
					}
					for _, v := range gitContractViolations(run, dir, tc) {
						t.Errorf("%s git: %s", side, v)
					}
				})
			}
		})
	}
}

func TestGitContract_ErrStyleFakeFailsEveryNonZeroCase(t *testing.T) {
	t.Parallel()
	for _, tc := range gitContractCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := gitContractRepo(t, tc.commits)
			bad := gitContractViolations(failThroughErr(newModelGit(tc.commits...)), dir, tc)
			if tc.wantExit != 0 && len(bad) == 0 {
				t.Errorf("a fake that fails through err passed the contract for exit %d", tc.wantExit)
			}
			if tc.wantExit == 0 && len(bad) != 0 {
				t.Errorf("a successful git case reported violations %v", bad)
			}
		})
	}
}

func failThroughErr(inner CmdRunner) CmdRunner {
	return func(ctx context.Context, name, dir string, args, env []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
		code, err := inner(ctx, name, dir, args, env, stdin, stdout, stderr)
		if err == nil && code != 0 {
			return 0, fmt.Errorf("git exited %d", code)
		}
		return code, err
	}
}
