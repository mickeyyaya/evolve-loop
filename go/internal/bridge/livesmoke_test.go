package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func liveSmokeDeps(tmux TmuxController) Deps {
	return Deps{
		Tmux:      tmux,
		Sleep:     func(time.Duration) {},
		LookupEnv: mapLookup(nil),
	}
}

type liveArtifactTmux struct {
	*FakeTmuxController
	artifact string
}

func (a *liveArtifactTmux) PasteBuffer(ctx context.Context, session string) error {
	if err := a.FakeTmuxController.PasteBuffer(ctx, session); err != nil {
		return err
	}
	return os.WriteFile(a.artifact, []byte("OK\n"), 0o644)
}

func TestLiveSmokeTest_HealthyWritesArtifact(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	base := &FakeTmuxController{CaptureFrames: []string{"❯", "working ❯", "working ❯", "done ❯", "cleanup"}}
	tm := &liveArtifactTmux{FakeTmuxController: base, artifact: filepath.Join(ws, LiveSmokeArtifact)}
	got := LiveSmokeTest(context.Background(), "claude-tmux", &Config{Workspace: ws}, liveSmokeDeps(tm))
	rc, pattern := got.RC, got.Pattern
	if rc != ExitOK {
		t.Fatalf("rc=%d, want ExitOK", rc)
	}
	if pattern != "" {
		t.Errorf("pattern=%q, want empty on healthy probe", pattern)
	}
	if tm.PasteCount == 0 {
		t.Error("live smoke must actually SUBMIT the trivial prompt (PasteBuffer never called)")
	}
}

func TestLiveSmokeTest_QuotaWallClassified(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	wall := "■ You've hit your usage limit. Upgrade to Pro or try again at 6:11 AM."
	tm := &FakeTmuxController{CaptureFrames: []string{"›", "thinking", wall, wall, wall, wall, wall, wall}}
	got := LiveSmokeTest(context.Background(), "codex-tmux", &Config{Workspace: ws}, liveSmokeDeps(tm))
	rc, pattern, scrollback := got.RC, got.Pattern, got.Scrollback
	if rc != ExitUnknownPrompt {
		t.Fatalf("rc=%d, want ExitUnknownPrompt(85); scrollback tail: %s", rc, ScrollbackTail(scrollback, 4))
	}
	if pattern != "rate_limit" {
		t.Errorf("pattern=%q, want rate_limit (from the escalation report)", pattern)
	}
	if !strings.Contains(scrollback, "usage limit") {
		t.Errorf("scrollback must carry the wall text for reset-hint parsing; got %q", ScrollbackTail(scrollback, 4))
	}
}

func TestLiveSmokeTest_NonTmuxDriverRejected(t *testing.T) {
	t.Parallel()
	rc := LiveSmokeTest(context.Background(), "claude-p", nil, liveSmokeDeps(&FakeTmuxController{})).RC
	if rc != ExitBadFlags {
		t.Fatalf("rc=%d, want ExitBadFlags for non-tmux driver", rc)
	}
}

func TestLiveSmokeResult_ARejectedDriverCarriesOnlyTheExitCode(t *testing.T) {
	t.Parallel()
	got := LiveSmokeTest(context.Background(), "claude-p", nil, liveSmokeDeps(&FakeTmuxController{}))
	want := LiveSmokeResult{RC: ExitBadFlags}
	if got != want {
		t.Fatalf("LiveSmokeTest = %+v, want %+v", got, want)
	}
}

type livePathTmux struct {
	*FakeTmuxController
	artifact string
	answer   func() string
}

func (a *livePathTmux) PasteBuffer(ctx context.Context, session string) error {
	if err := a.FakeTmuxController.PasteBuffer(ctx, session); err != nil {
		return err
	}
	return os.WriteFile(a.artifact, []byte(a.answer()+"\n"), 0o644)
}

func TestLiveSmokeTest_AsksTheCellWhereEvolveResolvesAndComparesItToTheCellBinEntry(t *testing.T) {
	t.Parallel()
	for name, answer := range map[string]func(string) string{
		"the cell-bin entry": func(root string) string {
			entries, _ := filepath.Glob(filepath.Join(root, "*", "evolve"))
			return strings.Join(entries, "")
		},
		"another evolve": func(string) string { return "/usr/local/bin/evolve" },
	} {
		ws := t.TempDir()
		base := &FakeTmuxController{CaptureFrames: []string{"❯", "working ❯", "working ❯", "done ❯", "cleanup"}}
		deps := liveSmokeDeps(nil)
		root := filepath.Join(t.TempDir(), "cell-bin")
		deps.CellBinRoot = fixedRoot(root)
		tm := &livePathTmux{FakeTmuxController: base, artifact: filepath.Join(ws, LiveSmokeArtifact), answer: func() string { return answer(root) }}
		deps.Tmux = tm
		got := LiveSmokeTest(context.Background(), "claude-tmux", &Config{Workspace: ws}, deps)
		if got.RC != ExitOK {
			t.Fatalf("%s: rc=%d", name, got.RC)
		}
		wantMismatch := name == "another evolve"
		if got.EvolveMismatch() != wantMismatch || got.EvolveEntry == "" || !strings.HasPrefix(got.EvolveEntry, root) {
			t.Errorf("%s: entry %q found %q mismatch %v, want mismatch %v under %s", name, got.EvolveEntry, got.EvolveFound, got.EvolveMismatch(), wantMismatch, root)
		}
		prompt, err := os.ReadFile(filepath.Join(ws, "live-smoke-prompt.txt"))
		if err != nil || !strings.Contains(string(prompt), "command -v evolve") {
			t.Errorf("%s: the probe prompt must ask for command -v evolve, got %q (err %v)", name, prompt, err)
		}
	}
}

func TestCellBinEntry_IsEmptyUntilTheLaunchResolvedTheDirectory(t *testing.T) {
	t.Parallel()
	if got := cellBinEntry(&Config{}); got != "" {
		t.Errorf("entry of an unresolved config = %q, want empty", got)
	}
	if got := cellBinEntry(&Config{cellBin: "/cb/x"}); got != "/cb/x/evolve" {
		t.Errorf("entry = %q, want /cb/x/evolve", got)
	}
}

func TestLiveSmokeResult_NoEntryNeverMismatches(t *testing.T) {
	t.Parallel()
	if (LiveSmokeResult{EvolveFound: "x"}).EvolveMismatch() {
		t.Error("a result with no cell-bin entry cannot mismatch")
	}
}
