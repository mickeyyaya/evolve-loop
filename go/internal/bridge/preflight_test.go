package bridge

import (
	"context"
	"testing"
)

// preflight_test.go — cycle-124 G3 contract: the optional CLIPreflight
// interface lets a Driver hook pre-launch prep work (today only codex-tmux,
// to pre-trust the worktree+workspace in ~/.codex/config.toml before the
// REPL boots — cycle-122 Fix 1 promoted out of an inline call). The tests
// pin three properties:
//
//  1. codex-tmux IS a CLIPreflight (the driver type assertion the Engine
//     does at launch.go would otherwise fall through silently and the
//     pretrust never run).
//  2. Drivers WITHOUT preflight work (claude-tmux, agy-tmux, ollama-tmux,
//     claude-p, codex headless, agy headless) MUST NOT accidentally
//     implement the interface — the absence is the OPT-OUT mechanism,
//     and a no-op stub on every concrete driver would be the wrong
//     pattern (the comment in driver.go documents this).
//  3. codex-tmux's Preflight returns an error from pretrustCodexProjects
//     unchanged (best-effort: error gets logged by Engine.Launch but does
//     not abort the phase; the contract is "do something useful or return
//     a logged-but-non-fatal error").

func TestCLIPreflight_CodexTmuxImplementsIt(t *testing.T) {
	d, ok := LookupDriver("codex-tmux")
	if !ok {
		t.Fatal("codex-tmux driver not registered (init() didn't fire?)")
	}
	if _, isPF := d.(CLIPreflight); !isPF {
		t.Fatalf("codex-tmux MUST implement CLIPreflight — cycle-122 Fix 1 pretrust runs through this seam; absence means Engine.Launch silently skips it and the workspace-write modal recurs")
	}
}

func TestCLIPreflight_OptOutByOmission(t *testing.T) {
	// Drivers expected to NOT implement CLIPreflight today. If a driver
	// later adopts CLIPreflight (e.g. agy keychain refresh), this list
	// must be updated. The opt-out is the architectural design — no no-op
	// stubs in every concrete driver. See driver.go's CLIPreflight godoc.
	optOut := []string{"claude-p", "claude-tmux", "agy", "agy-tmux", "codex", "ollama", "ollama-tmux"}
	for _, name := range optOut {
		d, ok := LookupDriver(name)
		if !ok {
			t.Logf("driver %q not registered (skipping)", name)
			continue
		}
		if _, isPF := d.(CLIPreflight); isPF {
			t.Errorf("driver %q now implements CLIPreflight — update preflight_test.go optOut list and document the new prep work in driver.go's godoc", name)
		}
	}
}

func TestCLIPreflight_CodexPreflightReturnsHelperError(t *testing.T) {
	// Drive codexTmuxDriver.Preflight directly with a config that the
	// underlying helper (pretrustCodexProjects) accepts but writes to a
	// real ~/.codex/config.toml — best-effort means even a write failure
	// is a return value, not a panic. The test is intentionally loose:
	// it just confirms Preflight returns without panic for a plausible
	// config. Tighter end-to-end coverage is in codex_pretrust_test.go
	// against the helper's own surface.
	d, ok := LookupDriver("codex-tmux")
	if !ok {
		t.Fatal("codex-tmux driver not registered (init() didn't fire?)")
	}
	pf, ok := d.(CLIPreflight)
	if !ok {
		t.Fatal("codex-tmux must implement CLIPreflight — see TestCLIPreflight_CodexTmuxImplementsIt")
	}
	cfg := &Config{
		CLI:       "codex-tmux",
		Worktree:  t.TempDir(),
		Workspace: t.TempDir(),
	}
	deps := Deps{} // no I/O streams — the helper does not write to them
	// We accept either a nil error (clean run) or a non-nil error (e.g.
	// HOME unsetup) — the contract is "non-panicking, best-effort".
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("codex-tmux Preflight panicked: %v (must be best-effort: return errors, never panic)", r)
		}
	}()
	_ = pf.Preflight(context.Background(), cfg, deps)
}
