package policy

import (
	"encoding/json"
	"strings"
	"testing"
)

func codeReviewRepairFrom(t *testing.T, raw string) JudgeRepairConfig {
	t.Helper()
	var p Policy
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("decode %s: %v", raw, err)
	}
	return p.WorkflowConfig().CodeReviewRepair
}

func TestCodeReviewRepair_CompiledDefaultIsShadowAtMedium(t *testing.T) {
	for _, raw := range []string{`{}`, `{"workflow":{}}`, `{"workflow":{"findings_repair":{}}}`} {
		got := codeReviewRepairFrom(t, raw)
		if got.Stage != "shadow" || got.Threshold != "MEDIUM" || len(got.Warnings) != 0 {
			t.Errorf("%s resolves %+v, want {Stage: shadow, Threshold: MEDIUM} with no warning: a strict finding is MEDIUM or above (Q-D2)", raw, got)
		}
	}
}

func TestCodeReviewRepair_ThresholdIsNormalizedToTheFindingVocabulary(t *testing.T) {
	got := codeReviewRepairFrom(t, `{"workflow":{"findings_repair":{"code-review":{"stage":"shadow","threshold":" high "}}}}`)
	if got.Threshold != "HIGH" || len(got.Warnings) != 0 {
		t.Errorf("threshold %q resolves %+v, want HIGH with no warning", " high ", got)
	}
}

func TestCodeReviewRepair_EnforceIsHeldAtShadowUntilTheFixRoundLands(t *testing.T) {
	got := codeReviewRepairFrom(t, `{"workflow":{"findings_repair":{"code-review":{"stage":"enforce"}}}}`)
	if got.Stage != "shadow" {
		t.Fatalf("stage enforce resolves %q, want shadow: enforce without the fix round leaves OPEN rows nothing works on", got.Stage)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], `"enforce"`) || !strings.Contains(got.Warnings[0], "shadow") {
		t.Errorf("warnings = %q, want one naming the refused value and the fallback", got.Warnings)
	}
}

func TestCodeReviewRepair_UnknownValuesFallBackLoudly(t *testing.T) {
	got := codeReviewRepairFrom(t, `{"workflow":{"findings_repair":{"code-review":{"stage":"strict","threshold":"SEVERE"}}}}`)
	if got.Stage != "shadow" || got.Threshold != "MEDIUM" {
		t.Errorf("resolves %+v, want shadow at the MEDIUM default", got)
	}
	if len(got.Warnings) != 2 {
		t.Fatalf("warnings = %q, want one for the stage and one for the threshold", got.Warnings)
	}
	if !strings.Contains(got.Warnings[0], `"strict"`) || !strings.Contains(got.Warnings[1], `"SEVERE"`) {
		t.Errorf("warnings = %q, want each to quote the value it refused", got.Warnings)
	}
}

func TestCodeReviewRepair_ABlockBuiltInCodeResolvesLikeTheDecodedOne(t *testing.T) {
	built := Policy{Workflow: &WorkflowPolicy{FindingsRepair: &FindingsRepairPolicy{CodeReview: &JudgeRepairPolicy{Stage: "shadow", Threshold: "critical"}}}}
	decoded := codeReviewRepairFrom(t, `{"workflow":{"findings_repair":{"code-review":{"stage":"shadow","threshold":"critical"}}}}`)
	if got := built.WorkflowConfig().CodeReviewRepair; got.Stage != decoded.Stage || got.Threshold != "CRITICAL" || got.Threshold != decoded.Threshold || len(got.Warnings) != 0 {
		t.Errorf("built %+v, decoded %+v, want both shadow at CRITICAL with no warning", got, decoded)
	}
}
