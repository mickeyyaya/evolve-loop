// codex_pretrust_launch_test.go — CB.3 contract (concurrency campaign W4):
// the first codex launch into a FRESH worktree never renders the trust
// prompt, because the trust entry is written BEFORE the REPL boots.
//
// DELIBERATE PLAN DEVIATION (recorded here per the no-silent-changes rule):
// the campaign WBS sketched CB.3 as "a hook beside linkGuardDeps calls
// pretrustCodexProjects at worktree PROVISIONING time". Verified ground truth:
// the launch chokepoint (Engine.LaunchArgs → CLIPreflight dispatch, cycle-124
// G3) already runs codexTmuxDriver.Preflight → pretrustCodexProjects(cfg) for
// the EXACT worktree of every launch — fresh, reused, or resumed — strictly
// before driver.Launch boots the session. A provisioning-time hook would
// duplicate the same TOML write through a new core→bridge seam (core cannot
// import bridge) for zero behavioral gain — the no-duplication command
// outranks plan literalism. What CB.3 therefore ships is the PIN: these tests
// fail if either half of the guarantee (preflight-writes-trust, or
// preflight-before-launch) ever regresses.
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

// TestCodexTmuxPreflightTrustsFreshWorktree: the acceptance fixture — a fresh
// (never-seen) worktree path handed to the codex-tmux driver's Preflight ends
// up trusted in the codex config, so the boot that follows renders no
// "Press enter to confirm" modal (the cycle-122 tdd hang).
func TestCodexTmuxPreflightTrustsFreshWorktree(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "codex", "config.toml")
	t.Setenv("EVOLVE_CODEX_CONFIG_PATH", cfgPath)
	freshWorktree := t.TempDir() // fresh = no prior trust entry anywhere

	cfg := &Config{CLI: "codex-tmux", Worktree: freshWorktree, Workspace: t.TempDir()}
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

// TestRecipeDriverPretrustsCodexWorktree: the recipe path bypasses the engine
// chokepoint entirely (newRecipeDriver → EnsureSession, no CLIPreflight
// dispatch), so a codex recipe session in a fresh worktree would boot with no
// trust entry (review finding on this slice). The recipe builder must pretrust
// codex-family CLIs itself.
func TestRecipeDriverPretrustsCodexWorktree(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "codex", "config.toml")
	t.Setenv("EVOLVE_CODEX_CONFIG_PATH", cfgPath)
	freshWorktree := t.TempDir()

	deps := recipeDeps(&fakeTmux{})
	if _, _, err := newRecipeDriver(&Config{Workspace: t.TempDir(), Worktree: freshWorktree, Agent: "recipe"}, deps, "codex-tmux"); err != nil {
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

// TestRecipeDriverSkipsPretrustForNonCodex: trusting paths in ~/.codex/config
// for a claude session would be pointless config churn — the recipe builder
// pretrusts codex-family CLIs only.
func TestRecipeDriverSkipsPretrustForNonCodex(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "codex", "config.toml")
	t.Setenv("EVOLVE_CODEX_CONFIG_PATH", cfgPath)

	deps := recipeDeps(&fakeTmux{})
	if _, _, err := newRecipeDriver(&Config{Workspace: t.TempDir(), Worktree: t.TempDir(), Agent: "recipe"}, deps, "claude-tmux"); err != nil {
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

// TestLaunchDispatchesPreflightBeforeDriverLaunch: the chokepoint half of the
// CB.3 guarantee — EVERY driver implementing CLIPreflight gets it invoked
// strictly before Launch, with the same resolved Config (so the pretrust sees
// the launch's actual worktree). Not parallel: mutates the global registry.
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
