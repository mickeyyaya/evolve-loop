package bridge

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
)

func TestControllerFactory_PerFamilyIsolation(t *testing.T) {
	base := "/proj/.evolve/usage-probe"
	var f *ControllerFactory = NewControllerFactory("/proj", base, "usage-probe", recipeDeps(&fakeTmux{}))

	claude := f.For("claude").(*cliController)
	codex := f.For("codex").(*cliController)

	if claude.cfg.Workspace == codex.cfg.Workspace {
		t.Fatal("families share a workspace — concurrent probes would collide on bridge scratch")
	}
	if claude.cfg.Workspace != filepath.Join(base, "claude") {
		t.Errorf("Workspace=%q, want %q", claude.cfg.Workspace, filepath.Join(base, "claude"))
	}
	if claude.cfg.CLI != "claude-tmux" || !claude.cfg.AllowBypass || claude.cfg.ProjectRoot != "/proj" {
		t.Errorf("cfg = %+v, want CLI=claude-tmux AllowBypass=true ProjectRoot=/proj", claude.cfg)
	}
	if claude.cfg.Agent != "usage-probe" {
		t.Errorf("Agent=%q, want usage-probe", claude.cfg.Agent)
	}
}

func TestControllerFactory_ForUnsupported(t *testing.T) {
	f := NewControllerFactory(t.TempDir(), t.TempDir(), "", recipeDeps(&fakeTmux{}))
	_, err := f.For("ollama").Do(context.Background(), "ollama", clicontrol.EventUsage)
	if !errors.Is(err, clicontrol.ErrUnsupported) {
		t.Fatalf("err=%v, want ErrUnsupported", err)
	}
}

func TestControllerFactory_AFleetModeUsageQueryRunsInAnOwnedScratchDirAndReadsTheFakeCLIScreen(t *testing.T) {
	base := t.TempDir()
	tm := &recipeWorkdirTmux{fakeTmux: &fakeTmux{paneSeq: []string{"❯", "Current week (all models) 93% used\n❯"}}}
	deps := recipeDeps(tm.fakeTmux)
	deps.Tmux = tm
	deps.LookupEnv = mapLookup(map[string]string{"EVOLVE_FLEET": "1"})

	resp, err := NewControllerFactory(t.TempDir(), base, "usage-evidence", deps).For("claude").Do(context.Background(), "claude", clicontrol.EventUsage)

	if err != nil {
		t.Fatalf("fleet-mode usage query err=%v, want nil: the query owns a scratch working directory, so the fleet guard has an explicit worktree", err)
	}
	if !strings.Contains(resp.Pane, "93% used") {
		t.Errorf("pane=%q, want the fake CLI's usage screen", resp.Pane)
	}
	want := filepath.Join(base, "claude", "bridge-scratch-cwd")
	if tm.bornIn != want {
		t.Errorf("usage session born in %q, want the owned scratch dir %q", tm.bornIn, want)
	}
}
