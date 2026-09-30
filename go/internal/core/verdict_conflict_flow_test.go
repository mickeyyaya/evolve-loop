package core

import (
	"strings"
	"testing"
)

func TestVerdictConflict_ErrorDiagnosticReachesAuditFailReasons(t *testing.T) {
	const conflict = "verdict-conflict: auditor narrative=PASS but the EGPS gate forces FAIL (red_count=1)"

	dir := t.TempDir()
	cs := &CycleState{CycleID: 1124, WorkspacePath: dir}
	persistFloorFailReasons(cs, PhaseAudit, []Diagnostic{
		{Severity: "error", Message: conflict},
		{Severity: "warning", Message: "gofmt gate skipped (could not run): boom"},
	})

	if len(cs.AuditFailReasons) != 1 || !strings.Contains(cs.AuditFailReasons[0], "verdict-conflict") {
		t.Fatalf("AuditFailReasons = %v, want the single verdict-conflict record — without it the "+
			"dossier's SubstantiveError cannot distinguish a genuine defect from a poisoned predicate",
			cs.AuditFailReasons)
	}
	if !(len(cs.AuditFailReasons) > 0) {
		t.Errorf("SubstantiveError would be false with a conflict record present")
	}
	got := readFloorFailReasons(dir, PhaseAudit)
	if len(got) != 1 || !strings.Contains(got[0], "verdict-conflict") {
		t.Errorf("audit-fail-reason.json = %v, want the verdict-conflict record", got)
	}
}

func TestVerdictConflict_WarningSeverityWouldBeDropped(t *testing.T) {
	dir := t.TempDir()
	cs := &CycleState{CycleID: 1124, WorkspacePath: dir}
	persistFloorFailReasons(cs, PhaseAudit, []Diagnostic{
		{Severity: "warning", Message: "verdict-conflict: auditor narrative=PASS but the EGPS gate forces FAIL"},
	})
	if len(cs.AuditFailReasons) != 0 {
		t.Fatalf("AuditFailReasons = %v, want empty — a warning-severity record is not an explanation",
			cs.AuditFailReasons)
	}
}

// conflictReasons renders the reason set one recurrence of one defect
// produces: the gate's own evidence plus the audit verdict-conflict record,
// whose only varying token is the auditor's own narrative verdict.
func conflictReasons(narrative string) []string {
	return []string{
		"EGPS: red_count=1 [ProbeIsolation] (cycle ships only when red_count==0)",
		"verdict-conflict: auditor narrative=" + narrative + " but 1 deterministic gate(s) forced FAIL " +
			"[EGPS red_count>0] — the gate outranks the narrative (ship policy unchanged); both readings " +
			"are recorded so the disagreement is weighable.",
	}
}

func TestVerdictConflict_FingerprintIsStableAcrossTheNarrativeAlphabet(t *testing.T) {
	want := fingerprint(string(PhaseAudit), "gate-block", conflictReasons(VerdictPASS))
	for _, n := range []string{VerdictWARN, VerdictSKIPPED} {
		if got := fingerprint(string(PhaseAudit), "gate-block", conflictReasons(n)); got != want {
			t.Errorf("narrative=%s fingerprints as %s, but narrative=PASS on the SAME defect fingerprints "+
				"as %s — one recurring defect split into separate buckets, so the identical-fingerprint "+
				"breaker cannot count it to the ceiling", n, got, want)
		}
	}
}

func TestVerdictConflict_BreakerHaltsOnTheRecurringConflict(t *testing.T) {
	var digests []FailureDigest
	for i, n := range []string{VerdictPASS, VerdictWARN, VerdictSKIPPED} {
		digests = append(digests, FailureDigest{
			Cycle:       1130 + i,
			PreClass:    "gate-block",
			Fingerprint: fingerprint(string(PhaseAudit), "gate-block", conflictReasons(n)),
		})
	}
	v := EvaluateBlockerBreaker(digests, BlockerBreakerConfig{IdenticalFingerprintCeiling: 3})
	if !v.Halt || v.Rule != "identical-fingerprint" {
		t.Fatalf("breaker verdict = %+v, want Halt on identical-fingerprint — the same defect recurred "+
			"3× under three narratives and the batch kept dispatching into the same wall", v)
	}
}

func TestVerdictConflict_FingerprintStillSeparatesDifferentDefects(t *testing.T) {
	base := fingerprint(string(PhaseAudit), "gate-block", conflictReasons(VerdictPASS))
	other := conflictReasons(VerdictPASS)
	other[0] = "EGPS: red_count=1 [BridgeStaysGreen] (cycle ships only when red_count==0)"
	if got := fingerprint(string(PhaseAudit), "gate-block", other); got == base {
		t.Errorf("a DIFFERENT red predicate produced the same fingerprint %s — distinct defects collapsed "+
			"into one bucket, which false-trips the breaker on honest, unrelated failures", got)
	}
	if got := fingerprint(string(PhaseBuild), "gate-block", conflictReasons(VerdictPASS)); got == base {
		t.Errorf("a different PHASE produced the same fingerprint %s", got)
	}
}

func TestNormalizeReasonForFingerprint_TouchesOnlyTheNarrativeToken(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"auditor narrative=PASS but", "auditor narrative=<verdict> but"},
		{"auditor narrative=SKIPPED but", "auditor narrative=<verdict> but"},
		// Not the token: a non-canonical value stays verbatim (it can only reach a
		// reason via some other writer, and blurring it would hide a real defect).
		{"auditor narrative=PASSABLE but", "auditor narrative=PASSABLE but"},
		{"verdict=PASS but the gate", "verdict=PASS but the gate"},
		{"EGPS: red_count=1 [ProbeIsolation]", "EGPS: red_count=1 [ProbeIsolation]"},
	} {
		if got := normalizeReasonForFingerprint(tc.in); got != tc.want {
			t.Errorf("normalizeReasonForFingerprint(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
