package policy

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/auditcascade"
)

func TestAuditCascadeConfig_IsOffUntilThePolicyNamesTheShadowStageAndItsTriggers(t *testing.T) {
	cases := []struct {
		name, json string
		want       AuditCascadeConfig
	}{
		{"absent block is off", `{}`, AuditCascadeConfig{Stage: AuditCascadeOff}},
		{"enforce is not built, so it reads as off", `{"audit_cascade":{"stage":"enforce"}}`, AuditCascadeConfig{Stage: AuditCascadeOff}},
		{"shadow with its triggers", `{"audit_cascade":{"stage":"shadow","risk_paths":["go/internal/core/"],"max_changed_files":40,"escalate_verdicts":["FAIL","WARN"]}}`,
			AuditCascadeConfig{Stage: AuditCascadeShadow, Rule: auditcascade.Rule{RiskPaths: []string{"go/internal/core/"}, MaxChangedFiles: 40, EscalateVerdicts: []string{"FAIL", "WARN"}}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := loadCIWatchPolicy(t, c.json).AuditCascadeConfig(); !reflect.DeepEqual(got, c.want) {
				t.Errorf("AuditCascadeConfig() = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestAuditCascadeConfigFor_ReadsTheProjectPolicyAndDegradesToOff(t *testing.T) {
	root := writePolicyJSON(t, `{"audit_cascade":{"stage":"shadow","max_changed_files":9}}`)
	if got := AuditCascadeConfigFor(root); got.Stage != AuditCascadeShadow || got.MaxChangedFiles != 9 {
		t.Errorf("AuditCascadeConfigFor = %+v, want the shadow block", got)
	}
	for name, root := range map[string]string{"malformed": writePolicyJSON(t, `{"audit_cascade":`), "no file": t.TempDir()} {
		if got := AuditCascadeConfigFor(root); got.Stage != AuditCascadeOff {
			t.Errorf("%s: AuditCascadeConfigFor = %+v, want off", name, got)
		}
	}
}

func TestAuditCascadePolicy_ALiteralBlockResolvesLikeTheDecodedOne(t *testing.T) {
	p := Policy{AuditCascade: &AuditCascadePolicy{Stage: AuditCascadeShadow, MaxChangedFiles: 3}}
	if got := p.AuditCascadeConfig(); got.Stage != AuditCascadeShadow || got.MaxChangedFiles != 3 {
		t.Errorf("AuditCascadeConfig() = %+v, want the literal block", got)
	}
}
