package phaseconfig

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = `{
  "name": "security-scan",
  "optional": true,
  "agent": "evolve-security-scanner",
  "model": "auto",
  "after": "build",
  "inputs":  {"files": [".evolve/runs/cycle-{cycle}/build-report.md"]},
  "outputs": {"files": [".evolve/runs/cycle-{cycle}/security-scan-report.md"]},
  "classify": {"require_sections": ["## Findings"]},
  "swarm_workers": 3,
  "prompt": "You are a security scanner. Audit the diff for OWASP issues.",
  "dispatch": {
    "cli": "codex-tmux",
    "cli_fallback": ["claude-tmux"],
    "model_tier_default": "deep",
    "model_tier_envelope": {"min": "balanced", "max": "deep"},
    "allowed_clis": ["codex", "claude"],
    "permission_mode": "plan",
    "sandbox": {"enabled": true, "read_only_repo": true},
    "system_prompt": "Refuse to modify source."
  }
}`

func writeCfg(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "pc.json")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad_EmbeddedSpecAndDispatch(t *testing.T) {
	pc, err := Load(writeCfg(t, sample))
	if err != nil {
		t.Fatal(err)
	}
	// Embedded PhaseSpec fields unmarshal at top level.
	if pc.Name != "security-scan" || !pc.Optional || pc.After != "build" {
		t.Errorf("spec fields: name=%q optional=%v after=%q", pc.Name, pc.Optional, pc.After)
	}
	if pc.AgentName() != "evolve-security-scanner" {
		t.Errorf("AgentName=%q", pc.AgentName())
	}
	if len(pc.Outputs.Files) != 1 || pc.Classify == nil || len(pc.Classify.RequireSections) != 1 {
		t.Errorf("IO/classify not parsed: %+v", pc.PhaseSpec)
	}
	// New top-level fields.
	if pc.SwarmWorkers != 3 {
		t.Errorf("SwarmWorkers=%d, want 3", pc.SwarmWorkers)
	}
	if _, ok := pc.PromptBody(); !ok {
		t.Error("PromptBody should report an in-band prompt")
	}
	// Nested dispatch.
	if pc.Dispatch.CLI != "codex-tmux" || pc.Dispatch.ModelTierDefault != "deep" {
		t.Errorf("dispatch: cli=%q tier=%q", pc.Dispatch.CLI, pc.Dispatch.ModelTierDefault)
	}
	if pc.Dispatch.Sandbox == nil || !pc.Dispatch.Sandbox.ReadOnlyRepo {
		t.Errorf("dispatch.sandbox not parsed: %+v", pc.Dispatch.Sandbox)
	}
}

func TestProfileName_StripsEvolvePrefix(t *testing.T) {
	pc, err := Load(writeCfg(t, sample))
	if err != nil {
		t.Fatal(err)
	}
	if got := pc.ProfileName(); got != "security-scanner" {
		t.Errorf("ProfileName=%q, want security-scanner", got)
	}
}

func TestToProfile_ReconstructsDispatch(t *testing.T) {
	pc, err := Load(writeCfg(t, sample))
	if err != nil {
		t.Fatal(err)
	}
	prof := pc.ToProfile()
	if prof.Name != "security-scanner" || prof.CLI != "codex-tmux" {
		t.Errorf("profile name/cli: %q/%q", prof.Name, prof.CLI)
	}
	if prof.ModelTierDefault != "deep" || prof.PermissionMode != "plan" {
		t.Errorf("profile tier/perm: %q/%q", prof.ModelTierDefault, prof.PermissionMode)
	}
	if len(prof.CLIFallback) != 1 || prof.CLIFallback[0] != "claude-tmux" {
		t.Errorf("profile fallback: %v", prof.CLIFallback)
	}
	if len(prof.AllowedCLIs) != 2 || prof.ModelTierEnvelope == nil || prof.ModelTierEnvelope.Max != "deep" {
		t.Errorf("profile allowed/envelope: %v / %+v", prof.AllowedCLIs, prof.ModelTierEnvelope)
	}
	if prof.SystemPrompt != "Refuse to modify source." {
		t.Errorf("profile system_prompt=%q", prof.SystemPrompt)
	}
	if prof.Sandbox == nil || !prof.Sandbox.Enabled {
		t.Errorf("profile sandbox: %+v", prof.Sandbox)
	}
}

func TestPromptBody_AbsentWhenEmpty(t *testing.T) {
	pc, err := Load(writeCfg(t, `{"name":"x","agent":"evolve-x"}`))
	if err != nil {
		t.Fatal(err)
	}
	if body, ok := pc.PromptBody(); ok || body != "" {
		t.Errorf("empty prompt should report absent, got %q/%v", body, ok)
	}
}

func TestLoad_EmptyNameIsError(t *testing.T) {
	if _, err := Load(writeCfg(t, `{"dispatch":{"cli":"claude-tmux"}}`)); err == nil {
		t.Fatal("a phase config with no name must error")
	}
}

func TestLoad_MissingFileIsError(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.json")); err == nil {
		t.Fatal("missing file must error")
	}
}
