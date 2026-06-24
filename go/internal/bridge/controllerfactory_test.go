package bridge

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/bridge/clicontrol"
)

// TestControllerFactory_PerFamilyIsolation verifies the factory mints a
// per-family Controller with an ISOLATED workspace and the right driver — the
// concurrency-safety invariant (no shared bridge scratch across families) lives
// here, not in the caller.
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

// TestControllerFactory_ForUnsupported verifies a minted Controller behaves like
// any other: an unsupported family/event pairing is a clean ErrUnsupported with
// no boot.
func TestControllerFactory_ForUnsupported(t *testing.T) {
	f := NewControllerFactory(t.TempDir(), t.TempDir(), "", recipeDeps(&fakeTmux{}))
	_, err := f.For("ollama").Do(context.Background(), "ollama", clicontrol.EventUsage)
	if !errors.Is(err, clicontrol.ErrUnsupported) {
		t.Fatalf("err=%v, want ErrUnsupported", err)
	}
}
