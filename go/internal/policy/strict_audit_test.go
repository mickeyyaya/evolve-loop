package policy

import (
	"os"
	"path/filepath"
	"testing"
)

// TestStrictAudit_DefaultAndOverride locks the policy.json "workflow.strict_audit"
// field that replaces the EVOLVE_STRICT_AUDIT env read (flag-reduction, ADR-0064).
// Absent ⇒ false (fluent-by-default: ship on WARN, failure-adapter awareness-only);
// a present true flows the operator's strict (legacy-blocking) posture through.
func TestStrictAudit_DefaultAndOverride(t *testing.T) {
	if got := (Policy{}).WorkflowConfig().StrictAudit; got {
		t.Errorf("absent workflow block: WorkflowConfig().StrictAudit = %v, want false", got)
	}

	p := Policy{Workflow: &WorkflowPolicy{StrictAudit: true}}
	if got := p.WorkflowConfig().StrictAudit; !got {
		t.Errorf("WorkflowConfig().StrictAudit = %v, want true", got)
	}
}

// TestStrictAuditFor_LoadsFromDisk covers the fail-open loader the audit and ship
// phases use to source strict mode from policy.json without each threading a typed
// field through every phase-dispatch / Options construction site. Absent OR
// malformed policy.json ⇒ false (fluent default — a malformed policy can never
// silently ARM the opt-in strict tightening; the loud parse failure still surfaces
// at the cycle's own policy.Load). A present workflow.strict_audit flows through.
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

	// Fail-open: a malformed policy must NOT arm strict mode.
	if err := os.WriteFile(policyPath, []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := StrictAuditFor(dir); got {
		t.Errorf("StrictAuditFor (malformed policy) = %v, want false (fail-open)", got)
	}
}
