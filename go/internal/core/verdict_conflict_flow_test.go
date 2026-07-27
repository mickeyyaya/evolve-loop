package core

import (
	"strings"
	"testing"
)

// verdict_conflict_flow_test.go — the CONSUMER half of the cycle-1124
// verdict-conflict wiring proof (producer half: internal/phases/audit/
// audit_verdict_conflict_test.go).
//
// The cycle-1124 fix deliberately adds NO new plumbing: it emits the conflict
// record as an ERROR-severity diagnostic so the existing chain carries it —
// errorSeverityMessages → CycleState.AuditFailReasons (the ADR-0072 coherence
// floor's only authoritative source) → <phase>-fail-reason.json (forensics) →
// failure dossier SubstantiveError/FailReasons (failure_dossier.go:86).
//
// That "no new plumbing" claim is load-bearing, so it is pinned here rather
// than assumed. This half is expected to be pre-existing GREEN (it locks the
// contract the producer relies on); the audit-package half is the RED one.
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
	// SubstantiveError is literally len(cs.AuditFailReasons) > 0 (failure_dossier.go:86 /
	// system_failure.go:184) — the conflict record therefore marks the FAIL as DIAGNOSED.
	if !(len(cs.AuditFailReasons) > 0) {
		t.Errorf("SubstantiveError would be false with a conflict record present")
	}
	// Forensic artifact carries it untruncated for retros/operators.
	got := readFloorFailReasons(dir, PhaseAudit)
	if len(got) != 1 || !strings.Contains(got[0], "verdict-conflict") {
		t.Errorf("audit-fail-reason.json = %v, want the verdict-conflict record", got)
	}
}

// TestVerdictConflict_WarningSeverityWouldBeDropped — why the producer MUST use
// error severity. A warning-severity conflict record is silently discarded by
// errorSeverityMessages, and worse, clears the carriers entirely: it would look
// exactly like today's silent-discard defect.
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

// conflictReasons renders the reason set ONE recurrence of ONE defect produces:
// the gate's own evidence (byte-identical across the three attempts — the
// defect did not change) plus the audit verdict-conflict record, whose only
// varying token is the auditor's own narrative verdict.
func conflictReasons(narrative string) []string {
	return []string{
		"EGPS: red_count=1 [ProbeIsolation] (cycle ships only when red_count==0)",
		"verdict-conflict: auditor narrative=" + narrative + " but 1 deterministic gate(s) forced FAIL " +
			"[EGPS red_count>0] — the gate outranks the narrative (ship policy unchanged); both readings " +
			"are recorded so the disagreement is weighable.",
	}
}

// TestVerdictConflict_FingerprintIsStableAcrossTheNarrativeAlphabet — cycle-1127
// audit finding C1, as the property the breaker actually needs. IsVerdict bounds
// the narrative to four values; three of them (PASS/WARN/SKIPPED) reach the
// conflict record. If each yields its own fingerprint, one recurring poisoned
// gate lands in three buckets of one against IdenticalFingerprintCeiling=3 and
// the batch halt never fires — the 862-899 storm shape the breaker exists to
// stop. Bounded is not stable; this pins stable.
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

// TestVerdictConflict_BreakerHaltsOnTheRecurringConflict — the consumer half of
// the test above, through the real breaker rather than a hash comparison: three
// attempts at one defect whose narratives differ must reach
// IdenticalFingerprintCeiling and HALT the batch.
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

// TestVerdictConflict_FingerprintStillSeparatesDifferentDefects — the anti-overfit
// half. Normalizing the narrative token must not blur DEFECT identity: a
// different tripped predicate, a different gate, and a different phase each
// still fingerprint apart. A normalizer that over-reached (e.g. stripping the
// gate evidence too) would green the test above and fail here.
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

// TestNormalizeReasonForFingerprint_TouchesOnlyTheNarrativeToken — the
// normalizer's blast radius, stated directly. It rewrites exactly
// `narrative=<canonical verdict>` and leaves every other reason shape alone,
// including near-misses that must NOT be swallowed.
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
