package bridge

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/sessionrecord"
)

func TestNewRecipeDriverBackfillF1_AFailedCodexPretrustWarnsAndStillBuildsTheDriver(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	blocker := filepath.Join(root, "blocker")
	if err := os.WriteFile(blocker, []byte("a file where the codex config directory must go"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{CLI: "codex-tmux", Workspace: t.TempDir(), Worktree: t.TempDir(), Agent: "f1", codexConfigPath: filepath.Join(blocker, "sub", "config.toml")}
	var stderr bytes.Buffer
	deps := covDeps()
	deps.Stderr = &stderr
	drv, marker, err := newRecipeDriver(cfg, deps, "codex-tmux")
	if err != nil || drv == nil || marker == "" {
		t.Fatalf("a best-effort pretrust failure must not fail the driver: drv=%v marker=%q err=%v", drv, marker, err)
	}
	if got := stderr.String(); !strings.Contains(got, "[recipe] WARN codex pretrust:") || !strings.Contains(got, "continuing — best-effort") {
		t.Fatalf("stderr = %q", got)
	}
}

func TestRecipeDriverEnsureSessionBackfillF1_AnUnwritableSessionRegistryWarnsButBoots(t *testing.T) {
	t.Parallel()
	tx := &fakeTmux{paneSeq: []string{"booting", "ready\n❯"}}
	d := newTestRecipeDriver(t, tx, "claude-tmux", "f1-sess")
	var stderr bytes.Buffer
	d.deps.Stderr = &stderr
	if err := os.Mkdir(sessionrecord.PathIn(d.cfg.Workspace), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := d.EnsureSession(context.Background()); err != nil {
		t.Fatalf("a registry failure must not block the boot: %v", err)
	}
	if got := stderr.String(); !strings.Contains(got, "WARN session registry append failed") || !strings.Contains(got, "session f1-sess will not be registry-reapable") {
		t.Fatalf("stderr = %q", got)
	}
	if !tx.sentContains("claude --dangerously-skip-permissions") {
		t.Fatalf("the launch line must still be sent: %v", tx.sentKeys)
	}
}

func controlConfigF1(t *testing.T) (*Config, *fakeTmux) {
	t.Helper()
	cfg := &Config{
		CLI: "claude-tmux", Workspace: t.TempDir(), Agent: "introspect", SessionName: "f1ctl",
		Realization: RealizeFor("claude-tmux", LaunchIntent{}),
	}
	return cfg, &fakeTmux{existing: map[string]bool{"evolve-bridge-named-f1ctl": true}, paneSeq: []string{"settled ❯"}}
}

func TestCaptureControlBackfillF1_ACommandThatCannotBeInjectedIsReturnedNotSwallowed(t *testing.T) {
	t.Parallel()
	cfg, tx := controlConfigF1(t)
	if err := os.WriteFile(filepath.Join(cfg.Workspace, ".bridge-inbox"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	pane, err := captureControl(context.Background(), cfg, recipeDeps(tx), "claude-tmux", "/usage", 3)
	if err == nil {
		t.Fatal("an inject transport failure must surface")
	}
	if pane != "" {
		t.Fatalf("no pane may be reported when the command never reached it, got %q", pane)
	}
}

func TestCaptureControlBackfillF1_AnInterruptWinsOverTheSettleTicks(t *testing.T) {
	t.Parallel()
	cfg, tx := controlConfigF1(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pane, err := captureControl(ctx, cfg, recipeDeps(tx), "claude-tmux", "/usage", 5)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if pane != "" {
		t.Fatalf("no settle tick ran, so no pane was captured, got %q", pane)
	}
}

func TestCaptureControlBackfillF1_StopsPollingAtTheFirstTickThatShowsThePromptMarker(t *testing.T) {
	t.Parallel()
	cfg, tx := controlConfigF1(t)
	tx.paneSeq = []string{"still working", "done\n❯", "never reached ❯"}
	pane, err := captureControl(context.Background(), cfg, recipeDeps(tx), "claude-tmux", "/usage", 6)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pane, "done") || strings.Contains(pane, "never reached") {
		t.Fatalf("pane = %q, want the first frame carrying the marker", pane)
	}
}
