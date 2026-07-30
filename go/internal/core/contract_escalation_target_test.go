package core

// contract_escalation_target_test.go — white-box coverage for the escalation
// TARGET decision (inbox contract-block-cli-escalation). The ladder-level
// integration tests in contract_escalation_test.go prove the target is applied to
// the right re-dispatch; these prove it IS the right target, across the
// phase/profile shapes that actually exist on the live tree.
//
// Production caller: reviewAndGuard's correction ladder (cyclerun_review.go)
// calls contractEscalationCLI + contractDispatchCLI on every contract-blocked
// re-dispatch, and the integration tests reach both through RunCycle.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// contractEscalationRun builds the minimal cycleRun the target decision needs: the
// project root it resolves profiles from plus the per-dispatch env snapshot the
// dispatch resolver reads. No orchestrator handles are touched by this path.
func contractEscalationRun(root string, env map[string]string) *cycleRun {
	return &cycleRun{req: CycleRequest{ProjectRoot: root}, envSnap: env}
}

// writeRawProfile writes an arbitrary profile document (needed for shapes
// writeCLIProfile does not model, e.g. allowed_clis).
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

// TestContractEscalation_MintedPhaseResolvesItsOwnProfile is the coverage fix for
// the phase this whole item's batch-19 evidence came from. phaseAgentName maps
// only the 10 built-in spine phases, so `adversarial-review` — which HAS a real
// .evolve/profiles/adversarial-review.json — resolves to a nil profile through
// the built-in table alone, and a nil profile means "primary is claude-tmux",
// which means "same family", which means NO escalation. Every minted/user phase
// under .evolve/phases/ shares that shape, and they are exactly the phases an
// operator points at a non-claude CLI.
func TestContractEscalation_MintedPhaseResolvesItsOwnProfile(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir()
	// Not in phaseAgentName — resolvable only by the <phase>.json convention.
	writeCLIProfile(t, root, "adversarial-review", "agy-tmux", nil)
	cr := contractEscalationRun(root, nil)

	if got := cr.contractDispatchCLI("adversarial-review", ""); got != "agy-tmux" {
		t.Errorf("contractDispatchCLI = %q, want \"agy-tmux\" — a minted phase's own profile must be resolved, or the demotion WARN names a CLI that never ran", got)
	}
	esc, ok := cr.contractEscalationCLI("adversarial-review", "")
	if !ok {
		t.Fatal("no escalation target for a minted phase on agy-tmux — the phase in this fix's own live evidence would still be unescalatable")
	}
	if esc != universalContractFallbackCLI {
		t.Errorf("escalation target = %q, want %q", esc, universalContractFallbackCLI)
	}
}

// TestContractEscalation_EnvOverrideDecidesTheFailedFamily: the failed family must
// come from the CLI that actually ran, and EVOLVE_CLI / EVOLVE_<AGENT>_CLI outrank
// profile.cli in the dispatch resolver. Reading profile.cli instead would compute
// the failed family from a CLI that never ran and "escalate" onto the family that
// just failed twice — while logging that it escalated.
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

// TestContractEscalation_DispatchOverrideDecidesTheFailedFamily: an advisor soft
// overlay (model_routing=auto) outranks env and profile alike, so a phase
// dispatched on the overlay's CLI must be escalated away from THAT family.
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
	if esc != universalContractFallbackCLI {
		t.Errorf("escalation target = %q, want %q (the only other family available)", esc, universalContractFallbackCLI)
	}
}

// TestContractEscalation_RespectsAllowedCLIs is the security guardrail. The
// sanctioned ModelRoutingCLI writer only ever emits values validated by
// policy.ValidatePin (the single validator for a profile's allowed_clis), so the
// escalation must not become the one path that routes a phase to a CLI family its
// operator forbade — e.g. a `tester` profile pinned to allowed_clis=["claude"].
func TestContractEscalation_RespectsAllowedCLIs(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir()
	// Primary is claude-family, chain offers codex, but only claude is permitted.
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

// TestContractEscalation_NoTargetWhenChainIsOneFamily is the precision guard: a
// phase whose entire chain is the universal-fallback family has nowhere to
// escalate, and "escalating" onto the CLI that just failed twice would be a lie
// in the log plus a wasted final correction.
func TestContractEscalation_NoTargetWhenChainIsOneFamily(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir()
	writeCLIProfile(t, root, "triage", universalContractFallbackCLI, []string{"claude-p"})
	cr := contractEscalationRun(root, nil)

	if esc, ok := cr.contractEscalationCLI(PhaseTriage, ""); ok {
		t.Errorf("escalation target = %q, want none — claude-tmux and claude-p are the SAME family, so re-pointing changes nothing but the log", esc)
	}
}

// TestContractEscalation_AbsentProfileIsNilSafe: a phase with no profile on disk
// must degrade to the dispatch resolver's own default (claude-tmux) and report no
// escalation target, rather than panicking or inventing a family.
func TestContractEscalation_AbsentProfileIsNilSafe(t *testing.T) {
	t.Setenv("EVOLVE_CLI", "")
	root := t.TempDir() // no .evolve/profiles at all
	cr := contractEscalationRun(root, nil)

	if got := cr.contractDispatchCLI(PhaseBuild, ""); got != universalContractFallbackCLI {
		t.Errorf("contractDispatchCLI = %q, want %q (the resolver's own no-profile default)", got, universalContractFallbackCLI)
	}
	if esc, ok := cr.contractEscalationCLI(PhaseBuild, ""); ok {
		t.Errorf("escalation target = %q, want none for a profile-less phase already on the default family", esc)
	}
}
