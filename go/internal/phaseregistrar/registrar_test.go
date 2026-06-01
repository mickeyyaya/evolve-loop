package phaseregistrar

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phaseconfig"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasespec"
	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

type fakeBridge struct{}

func (fakeBridge) Launch(context.Context, core.BridgeRequest) (core.BridgeResponse, error) {
	return core.BridgeResponse{}, nil
}
func (fakeBridge) Probe(context.Context) (core.BridgeProbe, error) {
	return core.BridgeProbe{}, nil
}

// validCfg is a minimal, in-envelope minted phase carrying an inline prompt.
func validCfg() phaseconfig.PhaseConfig {
	return phaseconfig.PhaseConfig{
		PhaseSpec: phasespec.PhaseSpec{Name: "minted-reviewer", Optional: true},
		Dispatch: phaseconfig.Dispatch{
			CLI:               "claude",
			AllowedCLIs:       []string{"claude", "codex"},
			ModelTierDefault:  "balanced",
			ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "fast", Max: "deep"},
		},
		Prompt: "You are a reviewer.",
	}
}

func newRegistrar(t *testing.T) Registrar {
	t.Helper()
	return Registrar{
		Bridge:      fakeBridge{},
		Prompts:     prompts.NewFromFS(fstest.MapFS{}), // empty: inline prompt only
		ProfilesDir: filepath.Join(t.TempDir(), "profiles"),
		PhasesDir:   filepath.Join(t.TempDir(), "phases"),
	}
}

func TestRegister_ValidConfig_ReturnsRunnerAndPersists(t *testing.T) {
	r := newRegistrar(t)
	res, err := r.Register(validCfg())
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if res.Runner == nil {
		t.Fatal("Register returned nil runner")
	}
	if res.Runner.Name() != "minted-reviewer" {
		t.Errorf("runner Name=%q, want minted-reviewer", res.Runner.Name())
	}
	if !res.Spec.Optional {
		t.Error("normalized spec must be Optional=true")
	}
	// Dispatch profile persisted so the runner resolves cli/model from disk.
	if _, err := os.Stat(filepath.Join(r.ProfilesDir, "minted-reviewer.json")); err != nil {
		t.Errorf("expected persisted profile: %v", err)
	}
	// Spec persisted for routing discovery + --resume durability.
	if _, err := os.Stat(filepath.Join(r.PhasesDir, "minted-reviewer", "phase.json")); err != nil {
		t.Errorf("expected persisted phase.json: %v", err)
	}
}

func TestRegister_ForcesOptional(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Optional = false // advisor forgot; Registrar must force it, not reject
	res, err := r.Register(cfg)
	if err != nil {
		t.Fatalf("Register must force Optional, not reject: %v", err)
	}
	if !res.Spec.Optional {
		t.Error("Optional not forced true")
	}
}

func TestRegister_TierOutsideEnvelope_Rejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.ModelTierEnvelope = &profiles.ModelTierEnvelope{Min: "fast", Max: "fast"}
	cfg.Dispatch.ModelTierDefault = "deep" // rank 3 outside [1..1]
	if _, err := r.Register(cfg); err == nil {
		t.Fatal("expected rejection: tier deep outside [fast..fast]")
	}
}

func TestRegister_CLINotAllowed_Rejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.AllowedCLIs = []string{"codex"} // claude not allowed
	if _, err := r.Register(cfg); err == nil {
		t.Fatal("expected rejection: cli claude not in allowed_clis [codex]")
	}
}

func TestRegister_InvalidName_Rejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Name = "Minted Reviewer" // spaces + caps → not kebab-case
	if _, err := r.Register(cfg); err == nil {
		t.Fatal("expected rejection: invalid name")
	}
}

func TestRegister_SourceWriter_GetsSandbox(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.WritesSource = true
	cfg.Dispatch.Sandbox = nil // not declared; Registrar must force it on
	res, err := r.Register(cfg)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	// Unmarshal the persisted profile and assert the sandbox is enabled for a
	// source-writer AND not read-only (a writer must be able to write).
	raw, err := os.ReadFile(filepath.Join(r.ProfilesDir, "minted-reviewer.json"))
	if err != nil {
		t.Fatalf("read persisted profile: %v", err)
	}
	var prof profiles.Profile
	if err := json.Unmarshal(raw, &prof); err != nil {
		t.Fatalf("parse persisted profile: %v", err)
	}
	if prof.Sandbox == nil || !prof.Sandbox.Enabled {
		t.Errorf("source-writer profile must enable sandbox; got %+v", prof.Sandbox)
	}
	if prof.Sandbox != nil && prof.Sandbox.ReadOnlyRepo {
		t.Error("source-writer sandbox must not be read-only")
	}
	if !res.Spec.WritesSource {
		t.Error("WritesSource must be preserved on the normalized spec")
	}
}

// TestRegister_UnclassifiableTierWithEnvelope_Rejected locks the trust-kernel
// fix: a novel/typo tier string (TierRank 0) must NOT silently escape a
// declared envelope (ValidatePin alone would exempt rank 0).
func TestRegister_UnclassifiableTierWithEnvelope_Rejected(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.ModelTierDefault = "gpt-4o-mega" // unclassifiable → rank 0
	if _, err := r.Register(cfg); err == nil {
		t.Fatal("expected rejection: unclassifiable tier with an envelope set")
	}
}

// TestRegister_CLIWithDriverSuffix_ClampedByBase proves the -tmux/-p suffix is
// stripped before the allowed_clis check, so an allowed base CLI passes.
func TestRegister_CLIWithDriverSuffix_ClampedByBase(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.CLI = "claude-tmux"              // driver-suffixed
	cfg.Dispatch.AllowedCLIs = []string{"claude"} // base form
	if _, err := r.Register(cfg); err != nil {
		t.Fatalf("claude-tmux should clamp to base claude (allowed); got %v", err)
	}
}

// TestRegister_NoEnvelopeNoAllowed_Passes proves the "no restriction" state
// (nil envelope + nil allowed_clis) is valid — clamp is opt-in.
func TestRegister_NoEnvelopeNoAllowed_Passes(t *testing.T) {
	r := newRegistrar(t)
	cfg := validCfg()
	cfg.Dispatch.ModelTierEnvelope = nil
	cfg.Dispatch.AllowedCLIs = nil
	cfg.Dispatch.ModelTierDefault = "anything-goes" // rank 0 ok when no envelope
	if _, err := r.Register(cfg); err != nil {
		t.Fatalf("no-restriction minted phase should register; got %v", err)
	}
}

// TestRegister_EmptyDirs_SkipsPersistence proves persistence is opt-in: with
// both dirs empty, Register still returns a runner and writes nothing.
func TestRegister_EmptyDirs_SkipsPersistence(t *testing.T) {
	r := Registrar{Bridge: fakeBridge{}, Prompts: prompts.NewFromFS(fstest.MapFS{})}
	res, err := r.Register(validCfg())
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if res.Runner == nil {
		t.Fatal("expected a runner even when persistence is skipped")
	}
}
