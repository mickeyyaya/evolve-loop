package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/paths"
)

func TestRunRecipe_AWorktreeRecipeRefusesWhenTheSeatIsIsolated(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, seatCanaryToken+"\n", 0o600)
	tx := &fakeTmux{}
	cfg := &Config{CLI: "claude-tmux", Workspace: t.TempDir(), Worktree: t.TempDir()}
	_, err := RunRecipe(context.Background(), cfg, recipeDeps(tx), "claude-tmux", "plugin-install", map[string]string{"marketplace": "x", "plugin": "y"})
	if err == nil || !strings.Contains(err.Error(), "seat") {
		t.Fatalf("err=%v: a worktree recipe must refuse while the seat is isolated", err)
	}
	if len(tx.sentKeys) != 0 || len(tx.existing) != 0 {
		t.Fatalf("the refusal must come before any pane call, got %v", tx.sentKeys)
	}
}

func TestRunRecipe_ARecipeWithoutAWorktreeIsNotRefusedByAnIsolatedSeat(t *testing.T) {
	home := seatHome(t)
	plantHolder(t, home, seatCanaryToken+"\n", 0o600)
	cfg := &Config{CLI: "claude-tmux", Workspace: t.TempDir()}
	_, err := RunRecipe(context.Background(), cfg, recipeDeps(&fakeTmux{}), "claude-tmux", "no-such-recipe", nil)
	if err == nil || strings.Contains(err.Error(), "seat") {
		t.Fatalf("err=%v: only a worktree recipe is refused", err)
	}
}

func TestRunRecipe_AWorktreeRecipeIsUnchangedWithNoHolder(t *testing.T) {
	seatHome(t)
	cfg := &Config{CLI: "claude-tmux", Workspace: t.TempDir(), Worktree: t.TempDir()}
	_, err := RunRecipe(context.Background(), cfg, recipeDeps(&fakeTmux{}), "claude-tmux", "no-such-recipe", nil)
	if err == nil || strings.Contains(err.Error(), "seat") {
		t.Fatalf("err=%v: no holder must leave the recipe path as today", err)
	}
}

func isolatedProfile(t *testing.T, isolated bool, goos string) string {
	t.Helper()
	home := seatHome(t)
	seatRoot := paths.SeatRoot(home)
	for _, f := range []string{filepath.Join(home, ".claude.json"), filepath.Join(seatRoot, ".claude.json"), filepath.Join(seatRoot, "settings.json")} {
		if err := os.MkdirAll(filepath.Dir(f), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	req := SandboxWrapRequest{Phase: "itest", CLI: "claude-tmux", Worktree: t.TempDir(), Workspace: t.TempDir(), RepoRoot: t.TempDir()}
	if isolated {
		req.HostIsolated, req.SeatRoot = true, seatRoot
	}
	prefix, ok := defaultSandboxWrapWithProbe(Deps{SandboxMode: "on"}, fakeProbe(goos, true))(req)
	if !ok {
		t.Fatal("the wrap was refused")
	}
	if goos == "darwin" {
		body, err := os.ReadFile(prefix[2])
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	return strings.Join(prefix, "\x00")
}

func TestSandboxWrap_AnIsolatedSeatMasksTheStoredClaudeJSONAndFreezesTheSeatConfigFiles(t *testing.T) {
	sbpl := isolatedProfile(t, true, "darwin")
	home, _ := os.UserHomeDir()
	seatRoot := canonicalOrSelf(paths.SeatRoot(home))
	for _, want := range []string{
		`(deny file-read* (literal "` + filepath.Join(home, ".claude.json") + `"))`,
		`(deny file-write* (literal "` + filepath.Join(seatRoot, ".claude.json") + `"))`,
		`(deny file-write* (literal "` + filepath.Join(seatRoot, "settings.json") + `"))`,
	} {
		if !strings.Contains(sbpl, want) {
			t.Errorf("sbpl lacks %s", want)
		}
	}
	bw := isolatedProfile(t, true, "linux")
	home, _ = os.UserHomeDir()
	if !strings.Contains(bw, "--ro-bind\x00/dev/null\x00"+filepath.Join(home, ".claude.json")) {
		t.Errorf("bwrap does not mask ~/.claude.json: %q", bw)
	}
}

func TestSandboxWrap_AHostWithoutATokenGetsNoFileRows(t *testing.T) {
	sbpl := isolatedProfile(t, false, "darwin")
	if strings.Contains(sbpl, "(deny file-read* (literal") || strings.Contains(sbpl, "(deny file-write* (literal \""+canonicalOrSelf(paths.SeatRoot(os.Getenv("HOME")))) {
		t.Errorf("a no-token wrap must add no file row:\n%s", sbpl)
	}
}
