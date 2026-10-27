package policy_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestRetryLadderConfig_AnAbsentBlockIsShadowWithTheDefaultBudget(t *testing.T) {
	for name, p := range map[string]policy.Policy{
		"no workflow block":     {},
		"no retry_ladder block": {Workflow: &policy.WorkflowPolicy{}},
		"an empty block":        {Workflow: &policy.WorkflowPolicy{RetryLadder: &policy.RetryLadderPolicy{}}},
	} {
		t.Run(name, func(t *testing.T) {
			got := p.WorkflowConfig().RetryLadder
			want := policy.RetryLadderConfig{Stage: policy.RetryLadderShadow, DigestTokens: policy.DefaultRetryLadderDigestTokens, AuditFix: policy.Policy{}.WorkflowConfig().RetryLadder.AuditFix}
			if got != want {
				t.Fatalf("RetryLadder = %+v, want %+v", got, want)
			}
			if got.Enforced() {
				t.Fatalf("the default stage enforces")
			}
		})
	}
}

func TestRetryLadderConfig_NamedValuesOverrideTheDefault(t *testing.T) {
	p, err := loadPolicyText(t, `{"workflow": {"retry_ladder": {"stage": "enforce", "digest_tokens": 900}}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := p.WorkflowConfig().RetryLadder
	if got.Stage != policy.RetryLadderEnforce || got.DigestTokens != 900 || !got.Enforced() {
		t.Fatalf("RetryLadder = %+v, want enforce with 900 tokens", got)
	}
}

func TestRetryLadderPolicy_RefusesABadValue(t *testing.T) {
	for name, text := range map[string]string{
		"unknown stage":        `{"workflow": {"retry_ladder": {"stage": "on"}}}`,
		"negative budget":      `{"workflow": {"retry_ladder": {"digest_tokens": -1}}}`,
		"budget under the min": `{"workflow": {"retry_ladder": {"digest_tokens": 8}}}`,
		"budget over the cap":  `{"workflow": {"retry_ladder": {"digest_tokens": 100000}}}`,
		"unknown key":          `{"workflow": {"retry_ladder": {"stages": "shadow"}}}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, err := loadPolicyText(t, text)
			if err == nil {
				t.Fatalf("Load accepted %s", text)
			}
			if name != "unknown key" && !strings.Contains(err.Error(), "workflow.retry_ladder") {
				t.Fatalf("error %q does not name workflow.retry_ladder", err)
			}
		})
	}
}

func TestRetryLadderConfig_TheCheckedInPolicyRecordsInShadow(t *testing.T) {
	p, err := policy.Load(checkedInPolicyPath)
	if err != nil {
		t.Fatalf("Load %s: %v", checkedInPolicyPath, err)
	}
	if p.Workflow == nil || p.Workflow.RetryLadder == nil || p.Workflow.RetryLadder.Stage != policy.RetryLadderShadow {
		t.Fatalf("the checked-in policy must name workflow.retry_ladder.stage shadow for the L5 wave, got %+v", p.Workflow)
	}
}

func TestRetryLadderConfig_AuditFixBoundsHaveDefaultsAndOverrides(t *testing.T) {
	off := false
	explicit := policy.Policy{Workflow: &policy.WorkflowPolicy{RetryLadder: &policy.RetryLadderPolicy{
		DigestTokens: policy.MinRetryLadderDigestTokens, AuditFix: &policy.AuditFixPolicy{MaxFiles: 1, NoNewProductionLogic: &off},
	}}}.WorkflowConfig().RetryLadder
	if explicit.DigestTokens != policy.MinRetryLadderDigestTokens || explicit.AuditFix.MaxFiles != 1 || explicit.AuditFix.NoNewProductionLogic || explicit.AuditFix.MaxLines != 40 {
		t.Fatalf("explicit block = %+v", explicit)
	}
	def := policy.Policy{}.WorkflowConfig().RetryLadder.AuditFix
	want := policy.AuditFixConfig{Stage: policy.RetryLadderShadow, MaxFiles: 3, MaxLines: 40, MaxCodeLines: 10, MaxTokens: 6000, NoNewProductionLogic: true}
	if def != want {
		t.Fatalf("default audit_fix = %+v, want %+v", def, want)
	}
	p, err := loadPolicyText(t, `{"workflow": {"retry_ladder": {"audit_fix": {"max_files": 2, "max_lines": 25, "max_code_lines": 4, "max_tokens": 900, "no_new_production_logic": false}}}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got := p.WorkflowConfig().RetryLadder.AuditFix
	if got != (policy.AuditFixConfig{Stage: policy.RetryLadderShadow, MaxFiles: 2, MaxLines: 25, MaxCodeLines: 4, MaxTokens: 900}) {
		t.Fatalf("audit_fix = %+v", got)
	}
	for name, text := range map[string]string{
		"negative files":  `{"workflow": {"retry_ladder": {"audit_fix": {"max_files": -1}}}}`,
		"lines over cap":  `{"workflow": {"retry_ladder": {"audit_fix": {"max_lines": 100000}}}}`,
		"tokens over cap": `{"workflow": {"retry_ladder": {"audit_fix": {"max_tokens": 1000000}}}}`,
		"code over cap":   `{"workflow": {"retry_ladder": {"audit_fix": {"max_code_lines": 100000}}}}`,
	} {
		if _, err := loadPolicyText(t, text); err == nil || !strings.Contains(err.Error(), "workflow.retry_ladder.audit_fix") {
			t.Fatalf("%s: Load error %v, want a refusal naming workflow.retry_ladder.audit_fix", name, err)
		}
	}
}

func TestAuditFixConfig_ReadsItsOwnStrictStage(t *testing.T) {
	for stage, enforced := range map[string]bool{policy.AuditFixOff: false, policy.RetryLadderShadow: false, policy.RetryLadderEnforce: true} {
		p, err := loadPolicyText(t, `{"workflow": {"retry_ladder": {"stage": "shadow", "audit_fix": {"stage": "`+stage+`"}}}}`)
		if err != nil {
			t.Fatalf("Load %s: %v", stage, err)
		}
		got := p.WorkflowConfig().RetryLadder
		if got.AuditFix.Stage != stage || got.AuditFix.Enforced() != enforced || got.Enforced() {
			t.Fatalf("stage %s: audit_fix = %+v, ladder enforced %v", stage, got.AuditFix, got.Enforced())
		}
	}
	_, err := loadPolicyText(t, `{"workflow": {"retry_ladder": {"audit_fix": {"stage": "sometimes"}}}}`)
	if err == nil || !strings.Contains(err.Error(), "workflow.retry_ladder.audit_fix.stage") {
		t.Fatalf("Load error %v, want a refusal naming workflow.retry_ladder.audit_fix.stage", err)
	}
}
