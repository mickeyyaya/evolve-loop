package policy

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStrictAudit_DefaultAndOverride(t *testing.T) {
	if got := (Policy{}).WorkflowConfig().StrictAudit; got {
		t.Errorf("absent workflow block: WorkflowConfig().StrictAudit = %v, want false", got)
	}

	p := Policy{Workflow: &WorkflowPolicy{StrictAudit: true}}
	if got := p.WorkflowConfig().StrictAudit; !got {
		t.Errorf("WorkflowConfig().StrictAudit = %v, want true", got)
	}
}

func TestStrictAuditFor_LoadsFromDisk(t *testing.T) {
	dir := t.TempDir()

	if got := StrictAuditFor(dir); got {
		t.Errorf("absent policy.json: StrictAuditFor = %v, want false", got)
	}

	evolveDir := filepath.Join(dir, ".evolve")
	if err := os.MkdirAll(evolveDir, 0o755); err != nil {
		t.Fatal(err)
	}
	policyPath := filepath.Join(evolveDir, "policy.json")

	if err := os.WriteFile(policyPath, []byte(`{"workflow":{"strict_audit":true}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := StrictAuditFor(dir); !got {
		t.Errorf("StrictAuditFor (strict_audit:true) = %v, want true", got)
	}

	if err := os.WriteFile(policyPath, []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := StrictAuditFor(dir); got {
		t.Errorf("StrictAuditFor (malformed policy) = %v, want false (fail-open)", got)
	}
}
