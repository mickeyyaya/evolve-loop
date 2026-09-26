package bridge

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCodexTmuxPreflightTrustsFreshWorktree(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "codex", "config.toml")
	versionPath := filepath.Join(dir, "codex", "version.json")
	old := codexVersionPathFn
	t.Cleanup(func() { codexVersionPathFn = old })
	codexVersionPathFn = func() (string, error) { return versionPath, nil }
	freshWorktree := t.TempDir() // fresh = no prior trust entry anywhere

	cfg := &Config{CLI: "codex-tmux", Worktree: freshWorktree, Workspace: t.TempDir(), codexConfigPath: cfgPath}
	if err := (codexTmuxDriver{}).Preflight(context.Background(), cfg, Deps{}.withDefaults()); err != nil {
		t.Fatalf("Preflight: %v", err)
	}

	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read codex config: %v", err)
	}
	conf := string(b)
	if !strings.Contains(conf, `[projects."`+freshWorktree+`"]`) {
		t.Errorf("fresh worktree %s has no [projects] entry after Preflight; config:\n%s", freshWorktree, conf)
	}
	if !strings.Contains(conf, `trust_level = "trusted"`) {
		t.Errorf("no trust_level entry written; config:\n%s", conf)
	}
}

func TestRecipeDriverPretrustsCodexWorktree(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "codex", "config.toml")
	freshWorktree := t.TempDir()

	deps := recipeDeps(&fakeTmux{})
	if _, _, err := newRecipeDriver(&Config{Workspace: t.TempDir(), Worktree: freshWorktree, Agent: "recipe", codexConfigPath: cfgPath}, deps, "codex-tmux"); err != nil {
		t.Fatalf("newRecipeDriver: %v", err)
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read codex config (was pretrust ever called on the recipe path?): %v", err)
	}
	if !strings.Contains(string(b), `[projects."`+freshWorktree+`"]`) {
		t.Errorf("recipe-path codex session has no trust entry for its worktree; config:\n%s", string(b))
	}
}

func TestRecipeDriverSkipsPretrustForNonCodex(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "codex", "config.toml")

	deps := recipeDeps(&fakeTmux{})
	if _, _, err := newRecipeDriver(&Config{Workspace: t.TempDir(), Worktree: t.TempDir(), Agent: "recipe", codexConfigPath: cfgPath}, deps, "claude-tmux"); err != nil {
		t.Fatalf("newRecipeDriver: %v", err)
	}
	if _, err := os.Stat(cfgPath); !os.IsNotExist(err) {
		t.Errorf("codex config was written for a claude recipe session (stat err=%v) — pretrust must be codex-family-only", err)
	}
}

// orderPinDriver records the relative order of Preflight and Launch.
type orderPinDriver struct{ calls *[]string }

func (orderPinDriver) Name() string { return "cb3-orderpin" }
func (d orderPinDriver) Preflight(context.Context, *Config, Deps) error {
	*d.calls = append(*d.calls, "preflight")
	return nil
}
func (d orderPinDriver) Launch(context.Context, *Config, Deps) (int, error) {
	*d.calls = append(*d.calls, "launch")
	return ExitOK, nil
}

// Not parallel: it mutates the global driver registry.
func TestLaunchDispatchesPreflightBeforeDriverLaunch(t *testing.T) {
	var calls []string
	Register(orderPinDriver{calls: &calls})
	defer func() { ResetDriversForTesting(); registerBuiltins() }()

	fx := newFixture(t, "cb3-orderpin", "")
	eng := NewEngine(Deps{Sleep: func(d time.Duration) {}, LookupEnv: mapLookup(nil)})
	var stdout, stderr bytes.Buffer
	code := eng.LaunchArgs(context.Background(), fx.args("cb3-orderpin"), nil, &stdout, &stderr)
	if code != ExitOK {
		t.Fatalf("LaunchArgs=%d, want ExitOK; stderr:\n%s", code, stderr.String())
	}
	if len(calls) != 2 || calls[0] != "preflight" || calls[1] != "launch" {
		t.Errorf("dispatch order=%v, want [preflight launch] — the CLIPreflight hook must run before the driver boots anything", calls)
	}
}
