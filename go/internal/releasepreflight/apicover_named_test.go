package releasepreflight

import (
	"bytes"
	"testing"
	"time"
)

func TestMaxAuditAge_Value(t *testing.T) {
	if MaxAuditAge != 7*24*time.Hour {
		t.Errorf("MaxAuditAge = %v, want 7*24h (168h)", MaxAuditAge)
	}
}

func TestMaxAuditAge_IsTheStaleAuditBoundary(t *testing.T) {
	r := makeRepo(t, "1.0.0")
	base := time.Now().UTC()

	opts := stubOpts(r, "1.0.1")
	opts.Now = func() time.Time { return base.Add(MaxAuditAge - time.Hour) }
	if _, err := Run(opts); err != nil {
		t.Fatalf("audit just under MaxAuditAge should pass, got %v", err)
	}

	var buf bytes.Buffer
	opts2 := stubOpts(r, "1.0.1")
	opts2.Stderr = &buf
	opts2.Now = func() time.Time { return base.Add(MaxAuditAge) }
	if _, err := Run(opts2); err == nil {
		t.Fatalf("audit at exactly MaxAuditAge should be stale (>= bound), got nil err\nlog=%s", buf.String())
	}
}

func TestResult_FieldsFromRun(t *testing.T) {
	want := Result{
		StepsPassed:     5,
		StepsTotal:      5,
		CurrentVersion:  "1.0.0",
		AuditVerdict:    "PASS",
		GateTestsPassed: 0,
	}
	r := makeRepo(t, "1.0.0")
	got, err := Run(stubOpts(r, "1.0.1"))
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
