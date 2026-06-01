package policy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/profiles"
)

func writePolicy(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, "policy.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_AbsentIsEmptyNoError(t *testing.T) {
	p, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("absent policy must not error: %v", err)
	}
	if len(p.MandatoryPhases) != 0 || len(p.Pins) != 0 {
		t.Errorf("absent policy must be empty, got %+v", p)
	}
}

func TestLoad_MalformedIsError(t *testing.T) {
	dir := t.TempDir()
	if _, err := Load(writePolicy(t, dir, "{ not json")); err == nil {
		t.Fatal("malformed policy must error (fail loudly, not silently disable)")
	}
}

func TestLoad_ParsesMandatoryAndPins(t *testing.T) {
	dir := t.TempDir()
	path := writePolicy(t, dir, `{
		"mandatory_phases": ["scout","build","audit","ship"],
		"pins": { "audit": {"cli":"claude-tmux","model":"claude-opus-4-8"} }
	}`)
	p, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.MandatoryPhases) != 4 || p.MandatoryPhases[2] != "audit" {
		t.Errorf("MandatoryPhases=%v", p.MandatoryPhases)
	}
	pin, ok := p.PinFor("audit")
	if !ok || pin.CLI != "claude-tmux" || pin.Model != "claude-opus-4-8" {
		t.Errorf("PinFor(audit)=%+v ok=%v", pin, ok)
	}
}

func TestPinFor_EmptyPinIsAbsent(t *testing.T) {
	p := Policy{Pins: map[string]Pin{"audit": {}}}
	if _, ok := p.PinFor("audit"); ok {
		t.Error("an all-empty pin must report absent")
	}
	if _, ok := p.PinFor("missing"); ok {
		t.Error("missing phase must report absent")
	}
}

func TestValidatePin_NilProfileOK(t *testing.T) {
	if err := ValidatePin("audit", Pin{CLI: "codex-tmux"}, nil); err != nil {
		t.Errorf("nil profile must validate ok, got %v", err)
	}
}

func TestValidatePin_CLIOutsideAllowed(t *testing.T) {
	prof := &profiles.Profile{AllowedCLIs: []string{"claude", "agy"}}
	if err := ValidatePin("audit", Pin{CLI: "codex-tmux"}, prof); err == nil {
		t.Error("codex pin must be rejected when allowed_clis=[claude,agy]")
	}
	if err := ValidatePin("audit", Pin{CLI: "claude-tmux"}, prof); err != nil {
		t.Errorf("claude pin must pass allowed_clis=[claude,agy]: %v", err)
	}
}

func TestValidatePin_AllowedAllPasses(t *testing.T) {
	prof := &profiles.Profile{AllowedCLIs: []string{"all"}}
	if err := ValidatePin("audit", Pin{CLI: "codex-tmux"}, prof); err != nil {
		t.Errorf("allowed_clis=[all] must accept any cli: %v", err)
	}
}

func TestValidatePin_ModelTierWithinEnvelope(t *testing.T) {
	// envelope deep..deep — exact model claude-opus-4-8 classifies to deep (3).
	prof := &profiles.Profile{ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "deep", Max: "deep"}}
	if err := ValidatePin("audit", Pin{Model: "claude-opus-4-8"}, prof); err != nil {
		t.Errorf("opus pin must sit within deep..deep: %v", err)
	}
}

func TestValidatePin_ModelTierOutsideEnvelope(t *testing.T) {
	// envelope deep..deep — a haiku/fast model is rank 1, outside → reject.
	prof := &profiles.Profile{ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "deep", Max: "deep"}}
	if err := ValidatePin("audit", Pin{Model: "claude-haiku-4-5"}, prof); err == nil {
		t.Error("haiku pin must be rejected when envelope=deep..deep")
	}
}

func TestValidatePin_UnclassifiableModelSkipsEnvelope(t *testing.T) {
	// A model tierRank can't classify (rank 0) skips the envelope check rather
	// than spuriously rejecting.
	prof := &profiles.Profile{ModelTierEnvelope: &profiles.ModelTierEnvelope{Min: "deep", Max: "deep"}}
	if err := ValidatePin("audit", Pin{Model: "gpt-5.5"}, prof); err != nil {
		t.Errorf("unclassifiable model must skip envelope check, got %v", err)
	}
}
