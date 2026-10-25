package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func contractEscalationRun(root string, env map[string]string) *cycleRun {
	return &cycleRun{req: CycleRequest{ProjectRoot: root}, envSnap: env}
}

func writeRawProfile(t *testing.T, root, agent string, doc map[string]any) {
	t.Helper()
	dir := filepath.Join(root, ".evolve", "profiles")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir profiles: %v", err)
	}
	body, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal profile: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, agent+".json"), body, 0o644); err != nil {
		t.Fatalf("write %s.json: %v", agent, err)
	}
}

func TestContractEscalation_MintedPhaseResolvesItsOwnProfile(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir()
	writeCLIProfile(t, root, "adversarial-review", "agy-tmux", nil)
	cr := contractEscalationRun(root, nil)

	if got := cr.contractDispatchCLI("adversarial-review", ""); got != "agy-tmux" {
		t.Errorf("contractDispatchCLI = %q, want \"agy-tmux\" — a minted phase's own profile must be resolved, or the demotion WARN names a CLI that never ran", got)
	}
	esc, ok := cr.contractEscalationCLI("adversarial-review", "")
	if !ok {
		t.Fatal("no escalation target for a minted phase on agy-tmux — the phase in this fix's own live evidence would still be unescalatable")
	}
	if esc != "claude-tmux" {
		t.Errorf("escalation target = %q, want %q", esc, "claude-tmux")
	}
}

func TestContractEscalation_EnvOverrideDecidesTheFailedFamily(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "claude-tmux", []string{"codex-tmux"})
	cr := contractEscalationRun(root, map[string]string{"EVOLVE_CLI": "codex"})

	if got := cr.contractDispatchCLI(PhaseBuild, ""); got != "codex" {
		t.Fatalf("contractDispatchCLI = %q, want \"codex\" (EVOLVE_CLI outranks profile.cli)", got)
	}
	esc, ok := cr.contractEscalationCLI(PhaseBuild, "")
	if !ok {
		t.Fatal("expected an escalation target away from the codex family")
	}
	if esc == "codex-tmux" || esc == "codex" {
		t.Errorf("escalation target = %q — that is the SAME family that just failed the contract; the failed family must be derived from the env-resolved primary, not profile.cli", esc)
	}
}

func TestContractEscalation_DispatchOverrideDecidesTheFailedFamily(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir()
	writeCLIProfile(t, root, "builder", "claude-tmux", []string{"agy-tmux"})
	cr := contractEscalationRun(root, nil)

	esc, ok := cr.contractEscalationCLI(PhaseBuild, "agy-tmux")
	if !ok {
		t.Fatal("expected an escalation target away from the agy family")
	}
	if esc == "agy-tmux" {
		t.Error("escalated onto the overlay CLI that just failed the contract — the dispatch override must define the failed family")
	}
	if esc != "claude-tmux" {
		t.Errorf("escalation target = %q, want %q (the only other family available)", esc, "claude-tmux")
	}
}

func TestContractEscalation_RespectsAllowedCLIs(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir()
	writeRawProfile(t, root, "builder", map[string]any{
		"name": "builder", "role": "builder", "cli": "claude-tmux",
		"cli_fallback": []string{"codex-tmux"},
		"allowed_clis": []string{"claude"},
	})
	cr := contractEscalationRun(root, nil)

	if esc, ok := cr.contractEscalationCLI(PhaseBuild, ""); ok {
		t.Errorf("escalated to %q despite allowed_clis=[claude] — the escalation must stay inside the profile guardrails policy.ValidatePin enforces", esc)
	}
}

func TestContractEscalation_NoTargetWhenChainIsOneFamily(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir()
	writeCLIProfile(t, root, "triage", "claude-tmux", []string{"claude-p"})
	cr := contractEscalationRun(root, nil)

	if esc, ok := cr.contractEscalationCLI(PhaseTriage, ""); ok {
		t.Errorf("escalation target = %q, want none — claude-tmux and claude-p are the SAME family, so re-pointing changes nothing but the log", esc)
	}
}

func TestContractEscalation_AbsentProfileIsNilSafe(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir()
	cr := contractEscalationRun(root, nil)

	if got := cr.contractDispatchCLI(PhaseBuild, ""); got != "claude-tmux" {
		t.Errorf("contractDispatchCLI = %q, want %q (the resolver's own no-profile default)", got, "claude-tmux")
	}
	if esc, ok := cr.contractEscalationCLI(PhaseBuild, ""); ok {
		t.Errorf("escalation target = %q, want none for a profile-less phase already on the default family", esc)
	}
}
