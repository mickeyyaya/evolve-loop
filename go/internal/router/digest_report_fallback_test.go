package router

// digest_report_fallback_test.go — RED contract for the L1 spine-floor fix
// (static/dynamic boundary review 2026-07-16, spine-floor-enforce-flip).
//
// TODAY: Digest's "scout" and "audit" branches read ONLY handoff-*.json via
// readFirstTracked. Handoff files have been extinct since ~cycle 215 (see
// buildFromGitFallback, which closed the same gap for build) — so on EVERY
// real cycle sig.Scout/sig.Audit stay Present:false, the spine floor
// (SpineSatisfiedUpTo) reports unsatisfied for next=triage/build/audit/ship,
// and the WARN clamp fires on healthy PASSing cycles (1914 clamped routing
// decisions across the surviving run history; 7 in the 2026-07-16 run alone).
// That WARN storm is the ONLY thing blocking the PhaseRecovery shadow→enforce
// flip — the floor cannot be armed while it cries wolf on every cycle.
//
// FIX CONTRACT (fails today — that failure is the RED evidence):
//   - scout: handoff absent → fall back to the artifact scout actually writes
//     every cycle: a non-empty scout-report.md ⇒ Present:true (signals beyond
//     presence stay zero — presence is what the floor gates on). No report at
//     all stays a CLEAN absence (Present:false, DigestDegraded empty).
//   - audit: handoff absent → fall back to acs-verdict.json — the
//     DETERMINISTIC, Go-generated verdict artifact (generateACSVerdict), whose
//     top-level verdict/red_count keys extractAudit already reads. A FAIL
//     verdict is carried through honestly (the audit anchor then correctly
//     refuses ship). A corrupt acs-verdict.json degrades LOUDLY via
//     DigestDegraded (fail-open at enforce), never silently Present:false.

import (
	"os"
	"strings"
	"testing"
)

func mkWorkspace(t *testing.T) string {
	t.Helper()
	ws := runWorkspacePath(t.TempDir(), 900)
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatalf("mkdir workspace: %v", err)
	}
	return ws
}

func writeWorkspaceFile(t *testing.T, ws, name, content string) {
	t.Helper()
	if err := os.WriteFile(ws+"/"+name, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestDigest_Scout_FallsBackToReportWhenHandoffAbsent: the artifact scout
// actually writes (scout-report.md) proves the phase ran — Present:true.
func TestDigest_Scout_FallsBackToReportWhenHandoffAbsent(t *testing.T) {
	ws := mkWorkspace(t)
	writeWorkspaceFile(t, ws, "scout-report.md", "# Scout Report\n\nbacklog scanned; task selected.\n")

	sig, err := Digest(ws, []string{"scout"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	if !sig.Scout.Present {
		t.Fatal("Digest(no handoff, scout-report.md present).Scout.Present = false, want true (report fallback)")
	}
	if len(sig.DigestDegraded) != 0 {
		t.Errorf("healthy report fallback must not degrade; DigestDegraded = %v", sig.DigestDegraded)
	}
}

// TestDigest_Scout_NoArtifactsStaysCleanAbsence: nothing on disk = an honest
// gap — Present:false with an EMPTY DigestDegraded (the enforce gate's
// clean-absence signal; this is what makes the armed floor fail-CLOSED only
// on real gaps).
func TestDigest_Scout_NoArtifactsStaysCleanAbsence(t *testing.T) {
	ws := mkWorkspace(t)

	sig, err := Digest(ws, []string{"scout"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	if sig.Scout.Present {
		t.Fatal("Digest(no artifacts).Scout.Present = true, want false")
	}
	if len(sig.DigestDegraded) != 0 {
		t.Errorf("clean absence must not degrade; DigestDegraded = %v", sig.DigestDegraded)
	}
}

// TestDigest_Audit_FallsBackToACSVerdictWhenHandoffAbsent: acs-verdict.json is
// generateACSVerdict's deterministic output and carries the same top-level
// verdict/red_count keys the handoff would — the objective source the audit
// anchor should trust.
func TestDigest_Audit_FallsBackToACSVerdictWhenHandoffAbsent(t *testing.T) {
	ws := mkWorkspace(t)
	writeWorkspaceFile(t, ws, "acs-verdict.json",
		`{"schema_version":"1.0","cycle":900,"green_count":10,"red_count":0,"verdict":"PASS","ship_eligible":true}`)

	sig, err := Digest(ws, []string{"audit"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	if !sig.Audit.Present {
		t.Fatal("Digest(no handoff, acs-verdict.json present).Audit.Present = false, want true (acs-verdict fallback)")
	}
	if sig.Audit.Verdict != "PASS" {
		t.Errorf("Audit.Verdict = %q, want PASS (from acs-verdict.json)", sig.Audit.Verdict)
	}
	if sig.Audit.RedCount != 0 {
		t.Errorf("Audit.RedCount = %d, want 0", sig.Audit.RedCount)
	}
}

// TestDigest_Audit_FAILVerdictCarriedHonestly: a red acs-verdict flows through
// unchanged — the spine's audit anchor (PASS|WARN required) then correctly
// refuses the ship transition. The fallback must never launder a FAIL.
func TestDigest_Audit_FAILVerdictCarriedHonestly(t *testing.T) {
	ws := mkWorkspace(t)
	writeWorkspaceFile(t, ws, "acs-verdict.json",
		`{"schema_version":"1.0","cycle":900,"green_count":8,"red_count":2,"verdict":"FAIL","ship_eligible":false}`)

	sig, err := Digest(ws, []string{"audit"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	if !sig.Audit.Present {
		t.Fatal("a FAIL acs-verdict still proves the audit RAN — Present must be true")
	}
	if sig.Audit.Verdict != "FAIL" || sig.Audit.RedCount != 2 {
		t.Errorf("Audit = {Verdict:%q RedCount:%d}, want {FAIL 2} — the fallback must carry red results honestly", sig.Audit.Verdict, sig.Audit.RedCount)
	}
}

// TestDigest_Audit_VerdictlessACSVerdictDegradesLoudly — the e2e-tier red at
// 2c0559a5 (cycles blocked at ship on CI): an acs-verdict.json that PARSES but
// carries no top-level verdict field (a legacy/schema-drifted artifact — the
// e2e fake emitted exactly this) yielded Present:true + Verdict:"" — an
// UNSATISFIABLE audit anchor (needs PASS|WARN) that read as a CLEAN absence,
// so the armed spine floor hard-blocked. A schema-drifted artifact is a
// DEGRADED read, not a clean gap: mark DigestDegraded (fail-open at enforce),
// never a silent block.
func TestDigest_Audit_VerdictlessACSVerdictDegradesLoudly(t *testing.T) {
	ws := mkWorkspace(t)
	writeWorkspaceFile(t, ws, "acs-verdict.json", `{"red_count": 0, "yellow_count": 0, "green_count": 1}`)

	sig, err := Digest(ws, []string{"audit"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	if sig.Audit.Present {
		t.Error("verdict-less acs-verdict must not report Present:true (an unsatisfiable anchor)")
	}
	found := false
	for _, d := range sig.DigestDegraded {
		if strings.Contains(d, "verdict") {
			found = true
		}
	}
	if !found {
		t.Errorf("schema-drifted acs-verdict must degrade LOUDLY naming the missing verdict; DigestDegraded = %v", sig.DigestDegraded)
	}
}

// TestDigest_Audit_CorruptACSVerdictDegradesLoudly: an unparseable
// acs-verdict.json is a read-miss, not a clean absence — DigestDegraded must
// record it (fail-open at enforce), mirroring the R5 distinction the digest
// applies everywhere else.
func TestDigest_Audit_CorruptACSVerdictDegradesLoudly(t *testing.T) {
	ws := mkWorkspace(t)
	writeWorkspaceFile(t, ws, "acs-verdict.json", `{"verdict": TRUNCATED`)

	sig, err := Digest(ws, []string{"audit"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	if sig.Audit.Present {
		t.Fatal("corrupt acs-verdict.json must not report Present:true")
	}
	found := false
	for _, d := range sig.DigestDegraded {
		if strings.Contains(strings.ToLower(d), "audit") || strings.Contains(d, "acs-verdict") {
			found = true
		}
	}
	if !found {
		t.Errorf("corrupt acs-verdict fallback must degrade loudly; DigestDegraded = %v", sig.DigestDegraded)
	}
}

// TestDigest_Audit_HandoffStillWinsOverFallback: when a real handoff-audit.json
// exists (the planned envelope writer, or a future phase that emits one), it
// remains authoritative — the fallback is reached only on handoff absence.
func TestDigest_Audit_HandoffStillWinsOverFallback(t *testing.T) {
	ws := mkWorkspace(t)
	writeWorkspaceFile(t, ws, "handoff-audit.json", `{"verdict":"WARN","red_count":1,"confidence":0.7}`)
	writeWorkspaceFile(t, ws, "acs-verdict.json", `{"verdict":"PASS","red_count":0}`)

	sig, err := Digest(ws, []string{"audit"})
	if err != nil {
		t.Fatalf("Digest error: %v", err)
	}
	if sig.Audit.Verdict != "WARN" || sig.Audit.RedCount != 1 {
		t.Errorf("handoff must stay authoritative over the fallback; got {Verdict:%q RedCount:%d}, want {WARN 1}", sig.Audit.Verdict, sig.Audit.RedCount)
	}
}
