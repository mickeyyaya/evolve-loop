package core

// failure_digest_identity_test.go — the identical-fingerprint breaker's
// identity must be CONTENT-BEARING (batch-14 halt, 2026-07-28). Cycles
// 1137/1139/1143 were three DISTINCT, progressing auditor findings on one
// task (zero coverage → grace never defaulted → gc.mode=off ignored), yet all
// three digests hashed the same content-free router line
// ("phase audit verdict FAIL routed to retro …") to one fingerprint and
// false-tripped the breaker: verdictFailDistinguisher's task-id layer cannot
// separate same-task retries, and its defect layer only matched "- D" bullet
// formatting while these auditors emit the schema-v2 sentinel defects list.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// identityWorkspace materializes the minimal artifacts the distinguisher
// consults: an audit-report.md carrying a schema-v2 verdict sentinel with a
// defects list, and a triage-decision.json committing taskID.
func identityWorkspace(t *testing.T, taskID string, defects ...string) string {
	t.Helper()
	ws := t.TempDir()
	sentinel := map[string]any{
		"phase": "audit", "verdict": "FAIL", "schema_version": 2,
		"failure": map[string]any{"class": "code-audit-fail", "defects": defects},
	}
	sj, err := json.Marshal(sentinel)
	if err != nil {
		t.Fatal(err)
	}
	report := "# Audit Report\n\nprose\n\n<!-- evolve-verdict: " + string(sj) + " -->\n"
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	if taskID != "" {
		td, err := json.Marshal(map[string]any{"top_n": []map[string]string{{"id": taskID}}})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), td, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return ws
}

// digestFor runs the REAL dispatch-composed reason through the REAL assembler
// — the same two calls cyclerun_dispatch.go makes — so the test covers the
// production path, not a reimplementation.
func digestFor(t *testing.T, ws string) FailureDigest {
	t.Helper()
	reason := agentGradedFailReason("audit", ws)
	b, err := json.Marshal(auditFailReason{SchemaVersion: 1, Phase: "audit", Reasons: []string{reason}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "audit-fail-reason.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := AssembleFailureDigest(1, ws, nil)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// TestFailureDigest_SameTaskDistinctDefectsGetDistinctFingerprints is the
// batch-14 live pin: the three real first-defect heads, same committed task,
// must mint three DIFFERENT fingerprints.
func TestFailureDigest_SameTaskDistinctDefectsGetDistinctFingerprints(t *testing.T) {
	heads := []string{
		"CRITICAL: the gc.mode=enforce apply path (cmd_gc.go:147-157) has zero covering tests at any level",
		"CRITICAL: gc.Policy.Worktrees grace is never defaulted (withDefaults skips it), so KeepRecent=0 ships",
		"cmd_gc.go:106-127 gcWorkspaceSweep has no `case \"off\": return` — an explicit gc.mode=off is ignored",
	}
	seen := map[string]string{}
	for _, h := range heads {
		d := digestFor(t, identityWorkspace(t, "workspace-hygiene-s5-wiring-shadow-default", h))
		if prev, dup := seen[d.Fingerprint]; dup {
			t.Fatalf("distinct defects share fingerprint %s:\n  %q\n  %q\n— the 1137/1139/1143 false-identity that halted batch-14", d.Fingerprint, prev, h)
		}
		seen[d.Fingerprint] = h
		if d.Unexplained {
			t.Errorf("defect-bearing digest marked Unexplained: %q", h)
		}
	}
}

// TestFailureDigest_SameDefectAcrossRetryCyclesCollides — the breaker must
// still catch REAL recurrence: the same defect re-audited next cycle carries
// new cycle-numbered tokens in its text, which must normalize out.
func TestFailureDigest_SameDefectAcrossRetryCyclesCollides(t *testing.T) {
	a := digestFor(t, identityWorkspace(t, "task-x",
		"acs/cycle1141 predicates never compile; TestC1141_004 drives the enforce path (cycle-1141)"))
	b := digestFor(t, identityWorkspace(t, "task-x",
		"acs/cycle1142 predicates never compile; TestC1142_004 drives the enforce path (cycle-1142)"))
	if a.Fingerprint != b.Fingerprint {
		t.Fatalf("the SAME defect on consecutive retry cycles minted different fingerprints:\n  %s\n  %s\n— cycle-numbered tokens in defect text must normalize out or the breaker never catches real recurrence", a.Fingerprint, b.Fingerprint)
	}
}

// TestVerdictFailDistinguisher_SentinelDefectsOutrankTaskIDs — task identity
// is the WEAKEST layer (same-task retries with different defects are the
// common case, and same-task repeats are S5 quarantine's job, not the
// breaker's). With both available, the defect wins.
func TestVerdictFailDistinguisher_SentinelDefectsOutrankTaskIDs(t *testing.T) {
	ws := identityWorkspace(t, "task-x", "CRITICAL: the one true defect")
	got := verdictFailDistinguisher("audit", ws)
	if !strings.Contains(got, "defect=") || !strings.Contains(got, "one true defect") {
		t.Fatalf("distinguisher = %q, want the sentinel defect head", got)
	}
	if strings.Contains(got, "tasks=") {
		t.Errorf("distinguisher %q leads with task identity — it cannot separate same-task retries", got)
	}
}

// TestVerdictFailDistinguisher_FallsBackTasksWhenNoDefects — no sentinel, no
// bullets → the task layer still beats nothing.
func TestVerdictFailDistinguisher_FallsBackTasksWhenNoDefects(t *testing.T) {
	ws := t.TempDir()
	td, _ := json.Marshal(map[string]any{"top_n": []map[string]string{{"id": "task-y"}}})
	if err := os.WriteFile(filepath.Join(ws, "triage-decision.json"), td, 0o644); err != nil {
		t.Fatal(err)
	}
	if got := verdictFailDistinguisher("audit", ws); got != "tasks=task-y" {
		t.Fatalf("distinguisher = %q, want tasks=task-y", got)
	}
}

// TestVerdictFailDistinguisher_ClasslessSentinelFallsSoftToBullets pins the
// ReadFailureBlock narrowing (re-review LOW): a hand-written sentinel carrying
// defects but NO failure.class is not authoritative — the distinguisher must
// fall through to the bullet layer, never mint a defect head from it.
func TestVerdictFailDistinguisher_ClasslessSentinelFallsSoftToBullets(t *testing.T) {
	ws := t.TempDir()
	report := "# Audit Report\n\n- D1 CRITICAL: the bullet-layer defect\n\n" +
		`<!-- evolve-verdict: {"phase":"audit","verdict":"FAIL","schema_version":2,"failure":{"defects":["classless sentinel defect"]}} -->` + "\n"
	if err := os.WriteFile(filepath.Join(ws, "audit-report.md"), []byte(report), 0o644); err != nil {
		t.Fatal(err)
	}
	got := verdictFailDistinguisher("audit", ws)
	if strings.Contains(got, "classless sentinel defect") {
		t.Fatalf("distinguisher = %q — a class-less sentinel is not authoritative and must not source identity", got)
	}
	if !strings.Contains(got, "bullet-layer defect") {
		t.Fatalf("distinguisher = %q, want the bullet-layer fallback", got)
	}
}

// TestAssembleFailureDigest_BoilerplateOnlyReasonIsUnexplained — a reason set
// that is EXACTLY the content-free router line asserts no identity: the
// digest must self-mark so the breaker routes it to the diagnosability rule.
func TestAssembleFailureDigest_BoilerplateOnlyReasonIsUnexplained(t *testing.T) {
	ws := t.TempDir()
	b, _ := json.Marshal(auditFailReason{SchemaVersion: 1, Phase: "audit",
		Reasons: []string{agentGradedRouterReason("audit")}})
	if err := os.WriteFile(filepath.Join(ws, "audit-fail-reason.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := AssembleFailureDigest(1, ws, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Unexplained {
		t.Fatalf("boilerplate-only digest not marked Unexplained (fingerprint %s) — three of these false-tripped the identical-fingerprint breaker on batch-14", d.Fingerprint)
	}
	// The negative pin (re-review LOW): a MIXED set — boilerplate plus one
	// real reason — is content-bearing; an "any boilerplate ⇒ unexplained"
	// refactor must fail here.
	b, _ = json.Marshal(auditFailReason{SchemaVersion: 1, Phase: "audit",
		Reasons: []string{agentGradedRouterReason("audit"), "EGPS: red_count=1 [GCHookRunsAfterFinalize]"}})
	if err := os.WriteFile(filepath.Join(ws, "audit-fail-reason.json"), b, 0o644); err != nil {
		t.Fatal(err)
	}
	d, err = AssembleFailureDigest(1, ws, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.Unexplained {
		t.Fatalf("a mixed reason set (boilerplate + real gate detail) marked Unexplained — its real reason IS a defect identity the breaker must keep counting")
	}
}

// TestEvaluateBlockerBreaker_UnexplainedDigestsNeverAssertIdentity — the
// breaker half: content-free digests may halt as a DIAGNOSABILITY breakdown
// (their honest name) but never as "identical defects".
func TestEvaluateBlockerBreaker_UnexplainedDigestsNeverAssertIdentity(t *testing.T) {
	shared := []FailureDigest{
		{Cycle: 1, Fingerprint: "audit|verdict-fail|deadbeef0000", PreClass: "verdict-fail", Unexplained: true},
		{Cycle: 2, Fingerprint: "audit|verdict-fail|deadbeef0000", PreClass: "verdict-fail", Unexplained: true},
		{Cycle: 3, Fingerprint: "audit|verdict-fail|deadbeef0000", PreClass: "verdict-fail", Unexplained: true},
	}
	v := EvaluateBlockerBreaker(shared, BlockerBreakerConfig{IdenticalFingerprintCeiling: 3})
	if v.Halt {
		t.Fatalf("identical-fingerprint rule asserted identity over UNEXPLAINED digests: %+v — distinct failures collapse into the content-free bucket by construction", v)
	}
	v = EvaluateBlockerBreaker(shared, BlockerBreakerConfig{IdenticalFingerprintCeiling: 3, UnexplainedCeiling: 3})
	if !v.Halt || v.Rule != "unexplained-failures" {
		t.Fatalf("unexplained rule did not claim the content-free digests: %+v — the diagnosability breakdown must stay visible under its honest name", v)
	}
}

// TestAgentGradedRouterReason_MatchesBoilerplateDetector pins the writers and
// the detector to shared templates — if a fallback wording drifts, the
// detector must fail here rather than silently reclassifying boilerplate as
// content.
func TestAgentGradedRouterReason_MatchesBoilerplateDetector(t *testing.T) {
	for _, phase := range []string{"audit", "adversarial-review"} {
		for _, r := range []string{agentGradedRouterReason(phase), abnormalEpilogueReason(phase)} {
			if !isBoilerplateRouterReason(r) {
				t.Fatalf("the fallback line %q does not match its own boilerplate detector", r)
			}
			if isBoilerplateRouterReason(r + " defect=something real") {
				t.Fatalf("a distinguisher-bearing reason must NOT read as boilerplate: %q", r)
			}
		}
	}
	if isBoilerplateRouterReason("bridge: launch exit=81: core: bridge artifact timeout") {
		t.Fatal("a real error string must never read as boilerplate")
	}
}

// TestAbnormalEpilogue_DigestIsUnexplained drives the REAL abnormal-exit
// production caller end-to-end: an aborted shell with no floor-written
// fail-reason must self-mark Unexplained — three same-phase aborts (an
// operator bounce cancels a whole wave) must never read as one recurring
// defect to the identical-fingerprint rule.
func TestAbnormalEpilogue_DigestIsUnexplained(t *testing.T) {
	cr, _ := epilogueRun(t, false)
	cr.abnormalEpilogue()
	raw, err := os.ReadFile(filepath.Join(cr.cs.WorkspacePath, "failure-digest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var d FailureDigest
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatal(err)
	}
	if !d.Unexplained {
		t.Fatalf("abnormal-epilogue digest not marked Unexplained: %+v — its template is constant per phase, so identical fingerprints across distinct aborts are guaranteed", d)
	}
}
