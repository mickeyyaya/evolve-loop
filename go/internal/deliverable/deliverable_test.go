package deliverable

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// Layer 3 of the deliverable-contract feature (ADR-0034): the shared verifier
// both the `evolve phase verify` self-check AND the host-side contract gate
// call. The fail-open/fail-closed contract is encoded in the return signature:
//
//	err != nil          → ambiguity/infra (unknown phase, unreadable dir) → caller fails OPEN
//	err == nil, !OK     → confirmed agent violation                      → caller fails CLOSED
//	err == nil, OK      → well-formed deliverable
//
// Verify checks WELL-FORMEDNESS ONLY (location, sections, verdict parseable,
// JSON keys). Semantic correctness stays the auditor's job (anti-Goodhart).

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestVerify_UnknownPhase_FailsOpen(t *testing.T) {
	_, err := Verify("nope", phasecontract.Roots{Workspace: t.TempDir()})
	if err == nil {
		t.Fatal("unknown phase: want error (fail-open signal), got nil")
	}
}

func TestVerify_ValidMarkdown_OK(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", "# Build Report\n\n## Changes\n- foo.go\n\nVerdict: PASS\n")
	res, err := Verify("build", phasecontract.Roots{Workspace: ws})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK {
		t.Errorf("want OK, got violations: %+v", res.Violations)
	}
}

func TestVerify_MissingArtifact_ConfirmedViolation(t *testing.T) {
	ws := t.TempDir()
	res, err := Verify("build", phasecontract.Roots{Workspace: ws})
	if err != nil {
		t.Fatalf("missing file is a confirmed violation, not ambiguity; got err=%v", err)
	}
	if res.OK {
		t.Fatal("want !OK for missing artifact")
	}
	if !hasCode(res, "missing_artifact") {
		t.Errorf("want missing_artifact violation, got %+v", res.Violations)
	}
	// Actionable: the message must name the expected path.
	if msg := firstMsg(res, "missing_artifact"); msg == "" ||
		!filepathContains(msg, filepath.Join(ws, "build-report.md")) {
		t.Errorf("missing_artifact message must name the expected path; got %q", msg)
	}
}

func TestVerify_MissingSection_NamesIt(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", "# Build Report\n\nno changes section here\nVerdict: PASS\n")
	res, _ := Verify("build", phasecontract.Roots{Workspace: ws})
	if res.OK {
		t.Fatal("want !OK when required section absent")
	}
	if !hasCode(res, "missing_section") {
		t.Errorf("want missing_section, got %+v", res.Violations)
	}
}

func TestVerify_StrayInWorktree(t *testing.T) {
	ws, wt := t.TempDir(), t.TempDir()
	// Agent wrote the report into the worktree root instead of the workspace.
	writeFile(t, wt, "build-report.md", "## Changes\n- x\nVerdict: PASS\n")
	res, _ := Verify("build", phasecontract.Roots{Workspace: ws, Worktree: wt})
	if res.OK {
		t.Fatal("want !OK: report is stray in worktree, missing from workspace")
	}
	if !hasCode(res, "stray_in_worktree") {
		t.Errorf("want stray_in_worktree, got %+v", res.Violations)
	}
}

func TestVerify_EmptyArtifact(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "build-report.md", "   \n\t\n")
	res, _ := Verify("build", phasecontract.Roots{Workspace: ws})
	if res.OK || !hasCode(res, CodeEmptyArtifact) {
		t.Errorf("want empty_artifact, got %+v", res.Violations)
	}
}

func TestVerify_BadVerdict(t *testing.T) {
	ws := t.TempDir()
	// audit is the only phase with a required verdict; a report with the Verdict
	// section heading but no PASS/FAIL/WARN/SKIPPED token must flag bad_verdict.
	writeFile(t, ws, "audit-report.md", "## Verdict\ninconclusive musings, no token\n")
	res, _ := Verify("audit", phasecontract.Roots{Workspace: ws})
	if res.OK || !hasCode(res, CodeBadVerdict) {
		t.Errorf("want bad_verdict, got %+v", res.Violations)
	}
}

func TestCheckStray_SkipsNonWorkspaceTarget(t *testing.T) {
	// Defensive guard: checkStray is a no-op for a non-workspace-target contract
	// even if a worktree is supplied.
	var res Result
	c := phasecontract.Contract{ArtifactName: "x.json", WriteTarget: phasecontract.TargetEvolveDir}
	checkStray(&res, c, phasecontract.Roots{Workspace: "/ws", Worktree: "/wt"})
	if len(res.Violations) != 0 {
		t.Errorf("want no violations for non-workspace target, got %+v", res.Violations)
	}
}

func TestVerify_ValidJSON_OK(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "routing-plan.json", `{"plan":[{"phase":"build"}],"extra":"ignored"}`)
	res, err := Verify("advisor", phasecontract.Roots{Workspace: ws})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK {
		t.Errorf("want OK (tolerant reader ignores unknown 'extra'), got %+v", res.Violations)
	}
}

func TestVerify_InvalidJSON(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "routing-plan.json", `{not json`)
	res, _ := Verify("advisor", phasecontract.Roots{Workspace: ws})
	if res.OK || !hasCode(res, "invalid_json") {
		t.Errorf("want invalid_json violation, got %+v", res.Violations)
	}
}

// TestVerify_MissingJSONKey: a contract that DOES declare RequiredKeys still
// requires a JSON object containing them. Uses orchestrator (cycle_id+phase) —
// the router contract no longer has required keys (it is a bare array now).
func TestVerify_MissingJSONKey(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "cycle-state.json", `{"phase":"build"}`) // missing cycle_id
	res, _ := Verify("orchestrator", phasecontract.Roots{EvolveDir: ws})
	if res.OK {
		t.Fatal("want !OK: orchestrator cycle-state.json missing required cycle_id")
	}
	if !hasCode(res, "missing_key") {
		t.Errorf("want missing_key, got %+v", res.Violations)
	}
}

func TestVerify_Orchestrator_EvolveDir(t *testing.T) {
	ev := t.TempDir()
	writeFile(t, ev, "cycle-state.json", `{"cycle_id":213,"phase":"tdd"}`)
	res, err := Verify("orchestrator", phasecontract.Roots{EvolveDir: ev})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK {
		t.Errorf("want OK for valid cycle-state.json, got %+v", res.Violations)
	}
}

// ---- helpers ----

func hasCode(r Result, code string) bool {
	for _, v := range r.Violations {
		if v.Code == code {
			return true
		}
	}
	return false
}

func firstMsg(r Result, code string) string {
	for _, v := range r.Violations {
		if v.Code == code {
			return v.Message
		}
	}
	return ""
}

func filepathContains(haystack, needle string) bool {
	return len(haystack) > 0 && len(needle) > 0 &&
		(haystack == needle || contains(haystack, needle))
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// --- ADR-0039 §7: failure-context conditionality ---

// A FAIL/WARN sentinel on a RequireFailureContext contract must carry the
// structured failure block; its absence is a confirmed violation whose message
// is the correction directive (the retry machinery injects it verbatim).
func TestVerify_AuditFailWithoutFailureBlock_Violation(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "audit-report.md",
		"## Verdict\nFAIL\n"+phasecontract.RenderVerdictSentinel("audit", "FAIL")+"\n")
	res, err := Verify("audit", phasecontract.Roots{Workspace: ws})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.OK || !hasCode(res, "failure_context_missing") {
		t.Fatalf("want failure_context_missing violation, got %+v", res.Violations)
	}
	for _, v := range res.Violations {
		if v.Code == "failure_context_missing" &&
			(!contains(v.Message, "failure") || !contains(v.Message, "schema_version")) {
			t.Errorf("violation message must tell the agent HOW to fix (failure block, schema_version 2); got %q", v.Message)
		}
	}
}

func TestVerify_AuditFailWithFailureBlock_OK(t *testing.T) {
	ws := t.TempDir()
	line := phasecontract.RenderVerdictSentinelWithFailure("audit", "FAIL",
		&phasecontract.FailureBlock{Class: "code-audit-fail", Defects: []string{"d1"}})
	writeFile(t, ws, "audit-report.md", "## Verdict\nFAIL\n"+line+"\n")
	res, err := Verify("audit", phasecontract.Roots{Workspace: ws})
	if err != nil || !res.OK {
		t.Fatalf("structured FAIL must verify clean; err=%v violations=%+v", err, res.Violations)
	}
}

// PASS needs no failure block; legacy prose-only FAIL (no sentinel) stays
// legal forever (v1 artifacts predate the failure contract).
func TestVerify_FailureContextNotRequiredOnPassOrLegacy(t *testing.T) {
	for name, content := range map[string]string{
		"pass":   "## Verdict\nPASS\n" + phasecontract.RenderVerdictSentinel("audit", "PASS") + "\n",
		"legacy": "## Verdict\nFAIL\n",
	} {
		ws := t.TempDir()
		writeFile(t, ws, "audit-report.md", content)
		res, err := Verify("audit", phasecontract.Roots{Workspace: ws})
		if err != nil || !res.OK {
			t.Errorf("%s: must verify clean; err=%v violations=%+v", name, err, res.Violations)
		}
	}
}

// TestVerify_RouterBareArray_OK pins the inbox defect
// router-contract-bare-array-vs-plan-key: PhaseAdvisor.Plan writes
// routing-plan.json as a BARE JSON ARRAY ("write your whole-cycle plan as a
// strict JSON array", phase_advisor.go) and the consumer parses an array —
// producer and consumer agree. The contract wrongly required a {"plan":...}
// OBJECT, so `evolve phase verify router` failed every fresh cycle.
func TestVerify_RouterBareArray_OK(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "routing-plan.json", `[{"phase":"build"},{"phase":"audit"}]`)
	res, err := Verify("router", phasecontract.Roots{Workspace: ws})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.OK {
		t.Errorf("router bare-array routing-plan.json must verify OK; got %+v", res.Violations)
	}
}

// A NoArtifact contract (ship — its deliverable is the pushed commit, not a
// file) must verify OK without looking for any file, and WITHOUT the
// "no contract registered" fail-open error that logged every cycle.
func TestVerify_NoArtifactContract_OK(t *testing.T) {
	res, err := Verify("ship", phasecontract.Roots{Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("ship verify: must not fail-open on a registered no-artifact phase: %v", err)
	}
	if !res.OK {
		t.Errorf("ship NoArtifact contract: want OK=true, got violations %+v", res.Violations)
	}
}
