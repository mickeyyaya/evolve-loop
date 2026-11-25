package policy_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestAuditRereviewStage_DefaultsToShadowAndNeverFollowsTheConvergenceStage(t *testing.T) {
	if got := (policy.Policy{}).ConvergenceConfig().AuditRereview; got != policy.AuditRereviewShadow {
		t.Fatalf("default stage = %q, want %q", got, policy.AuditRereviewShadow)
	}
	p, err := loadPolicyText(t, `{"workflow": {"convergence": {"stage": "enforce"}}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := p.ConvergenceConfig().AuditRereview; got != policy.AuditRereviewShadow {
		t.Fatalf("flipping workflow.convergence.stage moved the audit re-review stage to %q", got)
	}
}

func TestAuditRereviewStage_ItsOwnKeyFlipsItAlone(t *testing.T) {
	p, err := loadPolicyText(t, `{"workflow": {"convergence": {"audit_rereview": {"stage": "enforce"}}}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg := p.ConvergenceConfig()
	if cfg.AuditRereview != policy.AuditRereviewEnforce || cfg.Stage != "shadow" {
		t.Fatalf("config = rereview %q convergence %q, want enforce and shadow", cfg.AuditRereview, cfg.Stage)
	}
}

func TestAuditRereviewStage_AnUnknownValueFallsBackToShadowWithAWarning(t *testing.T) {
	p, err := loadPolicyText(t, `{"workflow": {"convergence": {"audit_rereview": {"stage": "warp"}}}}`)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg := p.ConvergenceConfig()
	if cfg.AuditRereview != policy.AuditRereviewShadow || !strings.Contains(strings.Join(cfg.Warnings, "|"), "audit_rereview.stage") {
		t.Fatalf("stage %q warnings %q", cfg.AuditRereview, cfg.Warnings)
	}
}

func TestAuditRereviewStage_AnUnknownSiblingKeyIsRefused(t *testing.T) {
	if _, err := loadPolicyText(t, `{"workflow": {"convergence": {"audit_rereview": {"stage": "shadow", "mode": "x"}}}}`); err == nil {
		t.Fatal("an unknown key under audit_rereview loaded")
	}
}

func TestAuditRereviewStage_ATypedBlockResolvesToItsStage(t *testing.T) {
	for _, want := range []policy.AuditRereviewStage{policy.AuditRereviewShadow, policy.AuditRereviewEnforce} {
		p := policy.Policy{Workflow: &policy.WorkflowPolicy{Convergence: &policy.ConvergencePolicy{
			AuditRereview: &policy.AuditRereviewPolicy{Stage: want},
		}}}
		if got := p.ConvergenceConfig().AuditRereview; got != want {
			t.Errorf("block %q resolved to %q", want, got)
		}
	}
	empty := policy.Policy{Workflow: &policy.WorkflowPolicy{Convergence: &policy.ConvergencePolicy{AuditRereview: &policy.AuditRereviewPolicy{}}}}
	if got := empty.ConvergenceConfig().AuditRereview; got != policy.AuditRereviewShadow {
		t.Errorf("an empty block resolved to %q, want shadow", got)
	}
}
