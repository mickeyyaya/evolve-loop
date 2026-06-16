package releasepreflight

import (
	"bytes"
	"testing"
	"time"
)

// TestMaxAuditAge_Value pins the staleness bound to exactly 7 days — the bash
// preflight contract this package ports. A drift here would silently widen or
// narrow the window in which a stale auditor PASS still gates a release.
func TestMaxAuditAge_Value(t *testing.T) {
	if MaxAuditAge != 7*24*time.Hour {
		t.Errorf("MaxAuditAge = %v, want 7*24h (168h)", MaxAuditAge)
	}
}

// TestMaxAuditAge_IsTheStaleAuditBoundary names MaxAuditAge and pins its role as
// the consumed boundary in checkRecentAudit: an audit exactly at the bound is
// stale (>= comparison), one just under it passes step 4. This exercises the
// real consumer path through Run, not the constant in isolation.
func TestMaxAuditAge_IsTheStaleAuditBoundary(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	base := time.Now().UTC()

	// just under the bound → step 4 passes
	opts := stubOpts(r, "1.0.1")
	opts.Now = func() time.Time { return base.Add(MaxAuditAge - time.Hour) }
	if _, err := Run(opts); err != nil {
		t.Fatalf("audit just under MaxAuditAge should pass, got %v", err)
	}

	// at the bound → stale (>= MaxAuditAge rejects)
	var buf bytes.Buffer
	opts2 := stubOpts(r, "1.0.1")
	opts2.Stderr = &buf
	opts2.Now = func() time.Time { return base.Add(MaxAuditAge) }
	if _, err := Run(opts2); err == nil {
		t.Fatalf("audit at exactly MaxAuditAge should be stale (>= bound), got nil err\nlog=%s", buf.String())
	}
}

// TestResult_FieldsFromRun binds a Result value directly (naming the type) and
// pins the field shape callers depend on, including the SimulationAdvisoryOK
// tri-state pointer (nil = advisory step skipped). This is the diagnostics
// struct Run returns and cmd_loop_preflight-style callers read field-by-field.
func TestResult_FieldsFromRun(t *testing.T) {
	want := Result{
		StepsPassed:     5,
		StepsTotal:      5,
		CurrentVersion:  "1.0.0",
		AuditVerdict:    "PASS",
		GateTestsPassed: 0,
	}
	r := makeRepo(t, "1.0.0")
	got, err := Run(stubOpts(r, "1.0.1")) // SkipTests=true → GateTestsPassed stays 0
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.StepsPassed != want.StepsPassed ||
		got.StepsTotal != want.StepsTotal ||
		got.CurrentVersion != want.CurrentVersion ||
		got.AuditVerdict != want.AuditVerdict ||
		got.GateTestsPassed != want.GateTestsPassed {
		t.Errorf("Result = %+v, want fields %+v", got, want)
	}
	if got.SimulationAdvisoryOK != nil {
		t.Errorf("SimulationAdvisoryOK = %v, want nil (advisory skipped under SkipTests)", *got.SimulationAdvisoryOK)
	}
}
