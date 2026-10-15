package bridge

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/evidence"
	"github.com/mickeyyaya/evolve-loop/go/internal/interaction"
)

type stubGit struct {
	heads   []string
	hi      int
	revList string
	revErr  error
	logErr  map[string]error
	msgs    map[string]string
}

func (g *stubGit) cmd(_ context.Context, args ...string) (string, error) {
	switch args[0] {
	case "rev-parse":
		v := g.heads[g.hi]
		if g.hi < len(g.heads)-1 {
			g.hi++
		}
		return v, nil
	case "rev-list":
		return g.revList, g.revErr
	case "log":
		sha := args[len(args)-1]
		return g.msgs[sha], g.logErr[sha]
	}
	return "", nil
}

func TestGitEvidencePoll_RevListFailureKeepsWatchingFromTheSameBaseline(t *testing.T) {
	t.Parallel()
	g := &stubGit{heads: []string{"sha0", "sha1"}, revErr: errors.New("rev-list boom")}
	d := &gitEvidenceDetector{phase: "scout", expectedTok: "tok", gitCmd: g.cmd}
	for i := 0; i < 2; i++ {
		ready, _, note, err := d.poll(context.Background())
		if ready || note != "" || err != nil {
			t.Fatalf("poll %d: ready=%v note=%q err=%v, want a quiet not-ready", i, ready, note, err)
		}
	}
	if d.baseline != "sha0" {
		t.Errorf("baseline = %q, want sha0: a rev-list fault must not re-baseline past unseen commits", d.baseline)
	}
}

func TestGitEvidencePoll_UnreadableCommitIsSkippedAndTheNextOneVerifies(t *testing.T) {
	t.Parallel()
	good := "subject\n" + evidence.Trailer{Phase: "scout", Challenge: "tok", Cycle: 5}.Build()
	g := &stubGit{
		heads:   []string{"sha0", "sha2"},
		revList: "shaBad\nshaGood",
		logErr:  map[string]error{"shaBad": errors.New("log boom")},
		msgs:    map[string]string{"shaGood": good},
	}
	d := &gitEvidenceDetector{phase: "scout", expectedTok: "tok", gitCmd: g.cmd}
	ready, ev := pollUntil(d, 3)
	if !ready || ev.CommitSHA != "shaGood" {
		t.Fatalf("ready=%v sha=%q, want the verifying commit behind the unreadable one", ready, ev.CommitSHA)
	}
}

func TestArtifactDetectorPoll_StatFailureAfterLocateResetsTheStabilityWindow(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ws := t.TempDir()
	artifact := filepath.Join(ws, "report.md")
	writeArtifact(t, artifact, "verdict: PASS\n", fixedMTime)
	d := &artifactDetector{cfg: &Config{Workspace: ws, Artifact: artifact}, haveLast: true, stable: 1}
	orig := statArtifact
	statArtifact = func(string) (os.FileInfo, error) { return nil, errors.New("vanished") }
	t.Cleanup(func() { statArtifact = orig })
	ready, _, _, err := d.poll(context.Background())
	if ready || err != nil {
		t.Fatalf("ready=%v err=%v, want a quiet not-ready", ready, err)
	}
	if d.haveLast || d.stable != 0 {
		t.Errorf("haveLast=%v stable=%d, want the window reset", d.haveLast, d.stable)
	}
}

func TestCodexTmuxPreflight_PretrustFailureIsReturnedBeforeTheUpdateNag(t *testing.T) {
	t.Parallel()
	blocker := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(blocker, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{Worktree: "/tmp/preflight-wt", codexConfigPath: filepath.Join(blocker, "config.toml")}
	err := codexTmuxDriver{}.Preflight(context.Background(), cfg, Deps{})
	if err == nil || !strings.Contains(err.Error(), "ensure codex config dir") {
		t.Fatalf("err = %v, want the pretrust dir failure", err)
	}
}

func TestPretrustCodexProjects_UnreadableConfigIsAnError(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	err := pretrustCodexProjects(&Config{Worktree: "/tmp/wt", codexConfigPath: path})
	if err == nil || !strings.Contains(err.Error(), "read codex config") {
		t.Fatalf("err = %v, want read codex config", err)
	}
}

func TestPretrustCodexProjects_UnresolvableHomeIsAnError(t *testing.T) {
	t.Setenv("HOME", "")
	err := pretrustCodexProjects(&Config{Worktree: "/tmp/wt"})
	if err == nil || !strings.Contains(err.Error(), "resolve codex config path") {
		t.Fatalf("err = %v, want resolve codex config path", err)
	}
}

type failingCodexTemp struct {
	name               string
	writeErr, closeErr error
}

func (f failingCodexTemp) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(p), nil
}
func (f failingCodexTemp) Close() error { return f.closeErr }
func (f failingCodexTemp) Name() string { return f.name }

func TestPretrustCodexProjects_TempAndRenameFailuresAreReported(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cases := []struct {
		name   string
		create func(string, string) (codexConfigTemp, error)
		rename func(string, string) error
		want   string
	}{
		{"create", func(string, string) (codexConfigTemp, error) { return nil, errors.New("no tmp") }, os.Rename, "create codex config tmp"},
		{"write", func(dir, pattern string) (codexConfigTemp, error) {
			return failingCodexTemp{name: filepath.Join(dir, pattern), writeErr: errors.New("full")}, nil
		}, os.Rename, "write codex config tmp"},
		{"close", func(dir, pattern string) (codexConfigTemp, error) {
			return failingCodexTemp{name: filepath.Join(dir, pattern), closeErr: errors.New("io")}, nil
		}, os.Rename, "close codex config tmp"},
		{"rename", func(dir, pattern string) (codexConfigTemp, error) {
			return failingCodexTemp{name: filepath.Join(dir, pattern)}, nil
		}, func(string, string) error { return errors.New("exdev") }, "rename codex config"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			origCreate, origRename := createCodexConfigTemp, renameCodexConfig
			createCodexConfigTemp, renameCodexConfig = tc.create, tc.rename
			t.Cleanup(func() { createCodexConfigTemp, renameCodexConfig = origCreate, origRename })
			path := filepath.Join(t.TempDir(), "config.toml")
			err := pretrustCodexProjects(&Config{Worktree: "/tmp/wt", codexConfigPath: path})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
				t.Errorf("config must not be written on failure: %v", statErr)
			}
		})
	}
}

func TestListWorkspaceFiles_MissingWorkspaceReadsAsEmpty(t *testing.T) {
	t.Parallel()
	got := listWorkspaceFiles(filepath.Join(t.TempDir(), "absent"))
	if len(got) != 1 || got[0] != "(workspace is empty)" {
		t.Fatalf("got %v", got)
	}
}

func TestListWorkspaceFiles_WalkFailureAndRelFailureAreReported(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	origWalk, origRel := walkWorkspace, relativeWorkspacePath
	t.Cleanup(func() { walkWorkspace, relativeWorkspacePath = origWalk, origRel })

	walkWorkspace = func(string, filepath.WalkFunc) error { return errors.New("walk boom") }
	got := listWorkspaceFiles("ws")
	if len(got) != 1 || got[0] != "(walk error: walk boom)" {
		t.Fatalf("walk failure: got %v", got)
	}

	ws := t.TempDir()
	file := filepath.Join(ws, "a.txt")
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	walkWorkspace = func(_ string, fn filepath.WalkFunc) error { return fn("a.txt", info, nil) }
	relativeWorkspacePath = func(string, string) (string, error) { return "", errors.New("rel boom") }
	got = listWorkspaceFiles(ws)
	var found bool
	for _, line := range got {
		found = found || line == "a.txt (1 bytes)"
	}
	if !found {
		t.Fatalf("rel failure must fall back to the full path; got %v", got)
	}
}

func TestOpenPending_NoRuleAdvancedClosesTheOldSendAndOpensNothing(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	deps := Deps{Tmux: &fakeTmux{}, Sleep: func(time.Duration) {}, LookupEnv: mapLookup(nil)}.withDefaults()
	rec := interaction.NewRecorder(ws)
	ar := newAutoResponder("claude-tmux", ws, deps, false, 0)
	ar.rec = rec
	ar.counts = map[string]int{"trust": 1}
	ar.pending = &pendingAutoRespond{rule: "old", keys: "Enter", at: deps.Now()}
	ar.openPending(map[string]int{"trust": 1}, "Enter")
	if ar.pending != nil {
		t.Errorf("pending = %+v, want none: no rule advanced", ar.pending)
	}
	outcomes := rec.Outcomes()
	if len(outcomes) != 1 || outcomes[0].Result != interaction.ResultNoEffect {
		t.Fatalf("outcomes = %+v, want the superseded send closed as no_effect", outcomes)
	}
}

func TestTryKernelAnswer_HumanModePausesBeforeInjecting(t *testing.T) {
	t.Parallel()
	facts := interaction.KernelFacts{ArtifactPath: "/ws/cycle-7/build-report.md"}
	ar, tmux, _ := brokerResponder(t, []string{blockedQ}, "enforce", facts)
	var slept int
	ar.deps.Sleep = func(time.Duration) { slept++ }
	ar.human = true
	if !ar.tryKernelAnswer(context.Background(), "s", blockedQ) {
		t.Fatal("kernel hit must inject")
	}
	if slept != 1 {
		t.Errorf("reading pauses = %d, want 1 before the answer", slept)
	}
	if !tmux.sentContains("/ws/cycle-7/build-report.md") {
		t.Errorf("answer not sent: %v", tmux.sentKeys)
	}
}
