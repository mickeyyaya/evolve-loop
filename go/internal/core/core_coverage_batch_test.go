package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/sysexec"
)

type queuedGitReply struct {
	stdout string
	exit   int
	err    error
}

type queuedGit struct {
	replies []queuedGitReply
	calls   [][]string
}

func (s *queuedGit) run(_ context.Context, _, _ string, args, _ []string, _ io.Reader, out, _ io.Writer) (int, error) {
	s.calls = append(s.calls, append([]string(nil), args...))
	if len(s.replies) == 0 {
		return 0, nil
	}
	reply := s.replies[0]
	s.replies = s.replies[1:]
	if out != nil && reply.stdout != "" {
		_, _ = out.Write([]byte(reply.stdout))
	}
	return reply.exit, reply.err
}

func useQueuedGit(t *testing.T, replies ...queuedGitReply) *queuedGit {
	t.Helper()
	s := &queuedGit{replies: replies}
	orig := gitRunner
	gitRunner = sysexec.RunFunc(s.run)
	t.Cleanup(func() { gitRunner = orig })
	return s
}

var errGitSpawn = errors.New("git spawn failed")

func TestCoreCoverageBatch_WorktreeBaseTreeSHA(t *testing.T) {
	cases := []struct {
		name     string
		worktree string
		base     string
		reply    queuedGitReply
		want     string
		wantArgs []string
	}{
		{name: "no worktree", want: ""},
		{name: "HEAD tree", worktree: "/wt", reply: queuedGitReply{stdout: "tree1\n"}, want: "tree1", wantArgs: []string{"rev-parse", "HEAD^{tree}"}},
		{name: "base commit tree", worktree: "/wt", base: " abc \n", reply: queuedGitReply{stdout: "tree2\n"}, want: "tree2", wantArgs: []string{"rev-parse", "abc^{tree}"}},
		{name: "nonzero exit", worktree: "/wt", reply: queuedGitReply{exit: 128}, want: "", wantArgs: []string{"rev-parse", "HEAD^{tree}"}},
		{name: "spawn error", worktree: "/wt", reply: queuedGitReply{err: errGitSpawn}, want: "", wantArgs: []string{"rev-parse", "HEAD^{tree}"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			git := useQueuedGit(t, tc.reply)
			if got := worktreeBaseTreeSHA(context.Background(), tc.worktree, tc.base); got != tc.want {
				t.Errorf("worktreeBaseTreeSHA = %q; want %q", got, tc.want)
			}
			if tc.wantArgs == nil {
				if len(git.calls) != 0 {
					t.Errorf("an empty worktree ran git: %v", git.calls)
				}
				return
			}
			if len(git.calls) != 1 || !reflect.DeepEqual(git.calls[0], tc.wantArgs) {
				t.Errorf("git calls = %v; want one call %v", git.calls, tc.wantArgs)
			}
		})
	}
}

type bindingCase struct {
	name        string
	replies     []queuedGitReply
	artifact    bool
	failAppend  bool
	wantEntries int
}

var bindingCases = []bindingCase{
	{name: "rev-parse fails", replies: []queuedGitReply{{err: errGitSpawn}}},
	{name: "diff fails to spawn", replies: []queuedGitReply{{stdout: "head1\n"}, {err: errGitSpawn}}},
	{name: "diff exits above one", replies: []queuedGitReply{{stdout: "head1\n"}, {exit: 2}}},
	{name: "append fails", replies: []queuedGitReply{{stdout: "head1\n"}, {stdout: "d"}}, failAppend: true},
	{name: "no artifact", replies: []queuedGitReply{{stdout: "head1\n"}, {stdout: "d", exit: 1}}, wantEntries: 1},
	{name: "artifact hashed", replies: []queuedGitReply{{stdout: "head1\n"}, {stdout: "d"}}, artifact: true, wantEntries: 1},
}

func runBindingCase(t *testing.T, tc bindingCase, phase Phase, record func(o *Orchestrator, workspace string)) (*fakeLedger, string) {
	t.Helper()
	useQueuedGit(t, tc.replies...)
	workspace := t.TempDir()
	if tc.artifact {
		writeBatchFile(t, filepath.Join(workspace, artifactNameFor(phase)), "report")
	}
	ledger := &fakeLedger{failOnAppend: tc.failAppend}
	fixed := time.Date(2026, 10, 18, 1, 2, 3, 0, time.UTC)
	record(&Orchestrator{ledger: ledger, now: func() time.Time { return fixed }}, workspace)
	if len(ledger.entries) != tc.wantEntries {
		t.Fatalf("ledger entries = %d; want %d", len(ledger.entries), tc.wantEntries)
	}
	return ledger, workspace
}

func assertBindingEntry(t *testing.T, tc bindingCase, e LedgerEntry, role string, cycle int) {
	t.Helper()
	if e.Role != role || e.Cycle != cycle || e.GitHEAD != "head1" || e.TS != "2026-10-18T01:02:03Z" {
		t.Errorf("entry = %+v; want role %s cycle %d head head1", e, role, cycle)
	}
	if e.TreeStateSHA != batchSHA256("d") {
		t.Errorf("TreeStateSHA = %s; want the sha256 of the diff", e.TreeStateSHA)
	}
	wantArtifact := ""
	if tc.artifact {
		wantArtifact = batchSHA256("report")
	}
	if e.ArtifactSHA256 != wantArtifact {
		t.Errorf("ArtifactSHA256 = %q; want %q", e.ArtifactSHA256, wantArtifact)
	}
}

func TestCoreCoverageBatch_RecordBuildBinding(t *testing.T) {
	for _, tc := range bindingCases {
		t.Run(tc.name, func(t *testing.T) {
			ledger, workspace := runBindingCase(t, tc, PhaseBuild, func(o *Orchestrator, ws string) {
				o.recordBuildBinding(context.Background(), 7, "/proj", ws)
			})
			if tc.wantEntries == 1 {
				e := ledger.entries[0]
				assertBindingEntry(t, tc, e, "builder", 7)
				if e.ArtifactPath != filepath.Join(workspace, artifactNameFor(PhaseBuild)) {
					t.Errorf("ArtifactPath = %s", e.ArtifactPath)
				}
			}
		})
	}
}

func TestCoreCoverageBatch_RecordGenericBinding(t *testing.T) {
	for _, tc := range bindingCases {
		t.Run(tc.name, func(t *testing.T) {
			ledger, _ := runBindingCase(t, tc, PhaseAudit, func(o *Orchestrator, ws string) {
				o.recordGenericBinding(context.Background(), PhaseAudit, bindingInputs{cycle: 9, projectRoot: "/proj", workspace: ws})
			})
			if tc.wantEntries == 1 {
				assertBindingEntry(t, tc, ledger.entries[0], "auditor", 9)
			}
		})
	}
}

func TestCoreCoverageBatch_PackageNewThisCycle(t *testing.T) {
	cases := []struct {
		name  string
		reply queuedGitReply
		want  bool
	}{
		{name: "absent from HEAD", reply: queuedGitReply{stdout: "\n"}, want: true},
		{name: "tracked at HEAD", reply: queuedGitReply{stdout: "go/internal/foo\n"}, want: false},
		{name: "ls-tree exits nonzero", reply: queuedGitReply{exit: 128}, want: false},
		{name: "ls-tree spawn error", reply: queuedGitReply{err: errGitSpawn}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			git := useQueuedGit(t, tc.reply)
			if got := packageNewThisCycle(context.Background(), "/wt", "./internal/foo"); got != tc.want {
				t.Errorf("packageNewThisCycle = %v; want %v", got, tc.want)
			}
			want := []string{"ls-tree", "--name-only", "HEAD", "--", "go/internal/foo"}
			if len(git.calls) != 1 || !reflect.DeepEqual(git.calls[0], want) {
				t.Errorf("git calls = %v; want %v", git.calls, want)
			}
		})
	}
}

func TestCoreCoverageBatch_WriteBuildSelfCheckArtifact(t *testing.T) {
	t.Parallel()
	fails := []selfCheckFailure{{Pkg: "./internal/foo", Output: "FAIL"}}

	t.Run("writes the failures", func(t *testing.T) {
		wt := t.TempDir()
		writeBuildSelfCheckArtifact(wt, fails)
		data, err := os.ReadFile(filepath.Join(wt, ".evolve", "build-selfcheck.json"))
		if err != nil {
			t.Fatalf("artifact not written: %v", err)
		}
		var got []selfCheckFailure
		if err := json.Unmarshal(data, &got); err != nil || !reflect.DeepEqual(got, fails) {
			t.Errorf("artifact = %s (err %v); want %v", data, err, fails)
		}
	})

	t.Run("evolve path is a file", func(t *testing.T) {
		wt := t.TempDir()
		writeBatchFile(t, filepath.Join(wt, ".evolve"), "not a dir")
		writeBuildSelfCheckArtifact(wt, fails)
		if data, _ := os.ReadFile(filepath.Join(wt, ".evolve")); string(data) != "not a dir" {
			t.Errorf("the blocking .evolve file changed: %q", data)
		}
	})

	t.Run("artifact path is a directory", func(t *testing.T) {
		wt := t.TempDir()
		dst := filepath.Join(wt, ".evolve", "build-selfcheck.json")
		if err := os.MkdirAll(dst, 0o755); err != nil {
			t.Fatal(err)
		}
		writeBuildSelfCheckArtifact(wt, fails)
		if info, err := os.Stat(dst); err != nil || !info.IsDir() {
			t.Errorf("the blocking directory was replaced: %v", err)
		}
	})
}

func TestCoreCoverageBatch_ContentionBackoff(t *testing.T) {
	t.Parallel()
	cases := []struct {
		depth  int
		window time.Duration
	}{
		{depth: 0, window: contentionBackoffBase},
		{depth: 2, window: 3 * contentionBackoffBase},
		{depth: 1000, window: contentionBackoffCap},
	}
	for _, tc := range cases {
		for i := 0; i < 50; i++ {
			got := contentionBackoff(tc.depth)
			if got < time.Millisecond || got >= time.Millisecond+tc.window {
				t.Fatalf("contentionBackoff(%d) = %v; want in [1ms, 1ms+%v)", tc.depth, got, tc.window)
			}
		}
	}
}

func artifactNameFor(phase Phase) string {
	return phasecontract.ArtifactFilename(string(phase))
}

func batchSHA256(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func writeBatchFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
