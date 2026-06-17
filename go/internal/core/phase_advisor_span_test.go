package core

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

// WS3-S3 (ADR-0052): every advisor call persists an OTel-GenAI decision span
// (advisor-span-<kind>.json) so a routing decision is queryable by a collector
// and by `evolve routing explain` (WS3-S4). The span's prompt_sha/response_sha
// bind the SAME redacted capture artifacts the ledger (WS3-S2) and replay
// (WS3-S5) key off, so all three agree on one identity per decision.
//
// decision_type / replan_depth are intentionally NOT asserted here: until
// WS1-S3 (RePlan) and WS2-S5 (depth cap) give them varying values, asserting
// them would lock surface, not behavior (second-review critic note).
func TestPhasePlan_RecordsDecisionSpanMetadata(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	fb := &fakeBridge{stdout: `[{"phase":"scout","run":true,"justification":"x"}]`, durationMS: 1234}
	in := baseRouteInput()
	in.Workspace = ws
	adv := NewPhaseAdvisor(fb, WithProposerCLI("claude-tmux"), WithProposerModel("opus"))

	if _, err := adv.Plan(in); err != nil {
		t.Fatalf("Plan: %v", err)
	}

	raw := readAdvisorArtifact(t, filepath.Join(ws, "advisor-span-plan.json"))
	var span map[string]any
	if err := json.Unmarshal([]byte(raw), &span); err != nil {
		t.Fatalf("span json: %v\n%s", err, raw)
	}

	for _, k := range []string{"gen_ai.request.model", "gen_ai.system", "prompt_sha", "response_sha", "duration_ms"} {
		if _, ok := span[k]; !ok {
			t.Errorf("decision span missing OTel-GenAI key %q:\n%s", k, raw)
		}
	}
	if span["gen_ai.request.model"] != "opus" {
		t.Errorf("gen_ai.request.model = %v, want opus", span["gen_ai.request.model"])
	}
	// gen_ai.system is the CLI family via the canonical resolver (not a re-rolled
	// provider map): claude-tmux → claude.
	if span["gen_ai.system"] != "claude" {
		t.Errorf("gen_ai.system = %v, want claude (llmroute.Family)", span["gen_ai.system"])
	}
	if d, _ := span["duration_ms"].(float64); d != 1234 {
		t.Errorf("duration_ms = %v, want 1234", span["duration_ms"])
	}

	// The span SHAs must equal sha256 of the REDACTED capture files — the single
	// identity shared with the ledger binding and the replay path.
	if span["prompt_sha"] != sha256Hex(t, filepath.Join(ws, "advisor-prompt-plan.txt")) {
		t.Error("prompt_sha must equal sha256 of advisor-prompt-plan.txt (S1/S2/S3 must agree)")
	}
	if span["response_sha"] != sha256Hex(t, filepath.Join(ws, "advisor-response-plan.txt")) {
		t.Error("response_sha must equal sha256 of advisor-response-plan.txt (S1/S2/S3 must agree)")
	}
}
