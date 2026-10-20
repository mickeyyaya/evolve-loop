package bridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/tokenusage"
)

type transcriptWritingTmux struct {
	*FakeTmuxController
	workspace, recordedWorkspace, configRoot, model string
}

func (f *transcriptWritingTmux) PasteBuffer(ctx context.Context, session string) error {
	if err := f.FakeTmuxController.PasteBuffer(ctx, session); err != nil {
		return err
	}
	dir := filepath.Join(f.configRoot, "projects", "scratch")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	cwd := filepath.Join(f.recordedWorkspace, "bridge-scratch-cwd")
	line := `{"type":"assistant","cwd":"` + cwd + `","timestamp":"` + time.Now().UTC().Format(time.RFC3339Nano) + `","message":{"id":"m1","model":"` + f.model + `","usage":{"input_tokens":1,"output_tokens":1}}}` + "\n"
	if err := os.WriteFile(filepath.Join(dir, "session.jsonl"), []byte(line), 0o644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(f.workspace, LiveSmokeArtifact), []byte("OK\n"), 0o644)
}

func healthyTranscriptTmux(t *testing.T, model string) (*transcriptWritingTmux, string) {
	t.Helper()
	ws := t.TempDir()
	base := &FakeTmuxController{CaptureFrames: []string{"❯", "working ❯", "working ❯", "done ❯", "cleanup"}}
	return &transcriptWritingTmux{FakeTmuxController: base, workspace: ws, recordedWorkspace: ws, configRoot: t.TempDir(), model: model}, ws
}

func TestLiveSmokeTest_ReportsTheModelThatServedTheReplyFromTheTranscript(t *testing.T) {
	t.Parallel()
	tm, ws := healthyTranscriptTmux(t, "claude-sonnet-5-5")
	deps := liveSmokeDeps(tm)
	deps.TokenResolver = tokenusage.DefaultResolver(tm.configRoot)

	got := LiveSmokeTest(context.Background(), "claude-tmux", &Config{Workspace: ws, Model: "sonnet"}, deps)

	if got.RC != ExitOK || got.ServedModel != "claude-sonnet-5-5" {
		t.Fatalf("rc=%d served=%q, want ExitOK served by claude-sonnet-5-5", got.RC, got.ServedModel)
	}
}

func TestLiveSmokeTest_NoResolverLeavesTheServedModelUnreported(t *testing.T) {
	t.Parallel()
	tm, ws := healthyTranscriptTmux(t, "claude-sonnet-5-5")

	got := LiveSmokeTest(context.Background(), "claude-tmux", &Config{Workspace: ws}, liveSmokeDeps(tm))

	if got.RC != ExitOK || got.ServedModel != "" {
		t.Fatalf("rc=%d served=%q, want ExitOK with no served model", got.RC, got.ServedModel)
	}
}

func TestLiveSmokeTest_AResolverErrorLeavesTheServedModelUnreportedAndSaysWhy(t *testing.T) {
	t.Parallel()
	tm, ws := healthyTranscriptTmux(t, "claude-sonnet-5-5")
	var stderr bytes.Buffer
	deps := liveSmokeDeps(tm)
	deps.Stderr = &stderr
	deps.TokenResolver = func(tokenusage.Window) (tokenusage.Result, error) {
		return tokenusage.Result{ServedModel: "ignored"}, errors.New("transcript unreadable")
	}

	got := LiveSmokeTest(context.Background(), "claude-tmux", &Config{Workspace: ws}, deps)

	if got.RC != ExitOK || got.ServedModel != "" || !strings.Contains(stderr.String(), "transcript unreadable") {
		t.Fatalf("rc=%d served=%q stderr=%q, want ExitOK, no served model, and the resolver error on stderr", got.RC, got.ServedModel, stderr.String())
	}
}

func TestLiveSmokeTest_ATranscriptThatRecordsTheResolvedCwdOfASymlinkedWorkspaceStillReportsTheModel(t *testing.T) {
	t.Parallel()
	tm, real := healthyTranscriptTmux(t, "claude-opus-5-5")
	link := filepath.Join(t.TempDir(), "ws-link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	tm.workspace, tm.recordedWorkspace = link, resolved
	deps := liveSmokeDeps(tm)
	deps.TokenResolver = tokenusage.DefaultResolver(tm.configRoot)

	got := LiveSmokeTest(context.Background(), "claude-tmux", &Config{Workspace: link}, deps)

	if got.RC != ExitOK || got.ServedModel != "claude-opus-5-5" {
		t.Fatalf("rc=%d served=%q, want claude-opus-5-5 read from a transcript whose cwd is the resolved path", got.RC, got.ServedModel)
	}
}

func TestServedModel_NoScratchCwdAttributesNoTranscript(t *testing.T) {
	t.Parallel()
	called := false
	deps := Deps{TokenResolver: func(tokenusage.Window) (tokenusage.Result, error) {
		called = true
		return tokenusage.Result{ServedModel: "another-sessions-model"}, nil
	}}

	if got := servedModel(deps, tokenusage.Window{Driver: "claude-tmux"}); got != "" || called {
		t.Fatalf("servedModel = %q (resolver called %t), want no served model and no transcript scan without a scratch cwd", got, called)
	}
}

func TestServedModel_TriesTheWorkspaceAsGivenThenResolvedAndReportsNothingWhenNeitherMatches(t *testing.T) {
	t.Parallel()
	real := t.TempDir()
	link := filepath.Join(t.TempDir(), "ws-link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	var asked []string
	deps := Deps{TokenResolver: func(w tokenusage.Window) (tokenusage.Result, error) {
		asked = append(asked, w.Worktree)
		return tokenusage.Result{}, nil
	}}

	got := servedModel(deps, tokenusage.Window{Worktree: link})

	if got != "" || len(asked) != 2 || asked[0] != link || asked[1] == link {
		t.Fatalf("servedModel = %q after asking %q, want no served model after the link and then its resolved path", got, asked)
	}
}

func TestCwdSpellings_APathWithNothingToResolveHasOneSpelling(t *testing.T) {
	t.Parallel()
	resolved, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	missing := filepath.Join(resolved, "missing")
	for _, dir := range []string{resolved, missing} {
		if got := cwdSpellings(dir); len(got) != 1 || got[0] != dir {
			t.Errorf("cwdSpellings(%q) = %q, want only the path itself", dir, got)
		}
	}
}
