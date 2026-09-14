package core

// crossartifact_invariants_wiring_test.go — THE WIRING HALF of the cycle-1676
// RED contract for inbox item `crossartifact-invariant-stack`.
//
// internal/coherence/crossartifact_test.go pins WHAT the aggregate computes.
// This file pins that it FIRES from the real cycle-close path (finalizeCycle,
// the terminal segment RunCycle always reaches) and that firing it changes
// nothing about whether the cycle blocks. A checker nothing calls is the same
// defect class the stack was written to catch (#373: wired into one path only
// is the same defect); an advisory that quietly gained teeth is the 1054/1060
// breaker lesson the inbox record explicitly forbids repeating.
//
// THE CONTRACT (Builder implements; this file is frozen):
//  1. finalizeCycle evaluates coherence.CheckCrossArtifactInvariants over the
//     cycle's workspace and its LANE worktree (cs.ActiveWorktree) — never the
//     projectRoot argument, which in fleet mode names a tree this lane did not
//     write (#612).
//  2. It records the result at <workspace>/crossartifact-invariants.json:
//     {"advisory":true,"invariants":[{"name","status","evidence"},...]} — always,
//     including an all-ok cycle, because a false-positive rate that is never
//     recorded can never be evidenced, and that evidence is the only door to
//     graduating any of these invariants to blocking.
//  3. It NEVER mutates result.FinalVerdict or result.SystemFailure, and never
//     disturbs the ADR-0072 verdict-incoherence floor that runs beside it.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Wiring/positive: the artifact appears on the real path (anti-no-op: a
//     helper that exists but is never called fails here and nowhere else).
//   - NEGATIVE       : four violations must NOT change the verdict, and the
//     lane-binding case must stay violated when the cited path exists only
//     under the project root.
//   - Semantic       : the pre-existing forgery halt is byte-for-byte unchanged.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// crossArtifactArtifact is the on-disk advisory record this cycle adds.
const crossArtifactArtifact = "crossartifact-invariants.json"

// xaWireReport is the wire view of that artifact — decoded by field name, so
// the Builder may add fields but may not rename these.
type xaWireReport struct {
	Advisory   bool `json:"advisory"`
	Invariants []struct {
		Name     string `json:"name"`
		Status   string `json:"status"`
		Evidence string `json:"evidence"`
	} `json:"invariants"`
}

// xaWireSentinel writes an audit-report.md whose canonical evolve-verdict
// sentinel carries verdict and (optionally) cited evidence paths.
func xaWireSentinel(t *testing.T, dir, verdict string, evidencePaths []string) {
	t.Helper()
	payload := map[string]any{"phase": "audit", "verdict": verdict, "schema_version": 1}
	if len(evidencePaths) > 0 {
		payload["schema_version"] = 2
		payload["failure"] = map[string]any{
			"class":          "code-audit-fail",
			"defects":        []string{"H1: cited artifact missing"},
			"evidence_paths": evidencePaths,
		}
	}
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal sentinel: %v", err)
	}
	body := "# Audit Report\n\n## Verdict\n**" + verdict + "**\n<!-- evolve-verdict: " + string(b) + " -->\n"
	if err := os.WriteFile(filepath.Join(dir, "audit-report.md"), []byte(body), 0o644); err != nil {
		t.Fatalf("write audit-report.md: %v", err)
	}
}

// xaIncoherentWorkspace builds a workspace that violates all four invariants:
// the sentinel says FAIL while acs-verdict.json says PASS, the claimed counts
// contradict the results beside them, a cited evidence path exists nowhere, and
// the phase chain runs backwards.
func xaIncoherentWorkspace(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	xaWireSentinel(t, ws, "FAIL", []string{"docs/never-written.md"})
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("acs-verdict.json", `{"schema_version":"1.0","cycle":1676,`+
		`"predicate_suite":{"total":9},`+
		`"results":[{"ac_id":"a","result":"green"},{"ac_id":"b","result":"red"}],`+
		`"green_count":2,"red_count":0,"skip_count":0,"verdict":"PASS","ship_eligible":true}`)
	write("phase-timing.json", `[`+
		`{"phase":"audit","duration_ms":1,"verdict":"PASS","cost_usd":0,"started_at":"2026-09-14T03:10:00Z","ended_at":"2026-09-14T03:15:00Z","attempt_count":1},`+
		`{"phase":"build","duration_ms":1,"verdict":"PASS","cost_usd":0,"started_at":"2026-09-14T03:00:00Z","ended_at":"2026-09-14T03:05:00Z","attempt_count":1}]`)
	return ws
}

// xaReadWireReport reads and decodes the advisory artifact, failing loudly when
// the real path never produced it.
func xaReadWireReport(t *testing.T, ws string) xaWireReport {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(ws, crossArtifactArtifact))
	if err != nil {
		t.Fatalf("RED: finalizeCycle did not record %s in the workspace (%v) — the aggregate is not wired into the real cycle-close path", crossArtifactArtifact, err)
	}
	var got xaWireReport
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("%s is not decodable as the advisory report: %v\n%s", crossArtifactArtifact, err, b)
	}
	return got
}

func xaOrchestrator() *Orchestrator {
	return &Orchestrator{
		storage:       &fakeUpdaterStorage{},
		gitHEAD:       func() (string, error) { return "same-head", nil },
		failurePolicy: policy.DefaultSystemFailurePolicy(),
	}
}

// xaStatusOf returns the reported status of one invariant from the artifact.
func xaStatusOf(t *testing.T, r xaWireReport, name string) string {
	t.Helper()
	for _, inv := range r.Invariants {
		if inv.Name == name {
			return inv.Status
		}
	}
	t.Fatalf("invariant %q absent from %s: %+v", name, crossArtifactArtifact, r.Invariants)
	return ""
}

// TestFinalizeCycle_EmitsAdvisoryCrossArtifactInvariantsArtifact — the wiring
// proof. Through the REAL finalizeCycle, a workspace whose artifacts contradict
// each other leaves a decodable advisory record naming all four invariants,
// with the violated ones carrying evidence.
func TestFinalizeCycle_EmitsAdvisoryCrossArtifactInvariantsArtifact(t *testing.T) {
	ws := xaIncoherentWorkspace(t)
	result := &CycleResult{FinalVerdict: VerdictPASS}
	cs := CycleState{CycleID: 1676, WorkspacePath: ws, ActiveWorktree: t.TempDir()}

	if _, err := xaOrchestrator().finalizeCycle(context.Background(), cs, 1676, "same-head", "", result, &State{}, nil); err != nil {
		t.Fatalf("finalizeCycle: %v", err)
	}

	got := xaReadWireReport(t, ws)
	if !got.Advisory {
		t.Error("the recorded report is not marked advisory — every invariant ships advisory until its false-positive rate is evidenced")
	}
	if len(got.Invariants) != 4 {
		t.Fatalf("recorded %d invariants, want the four named by the inbox record: %+v", len(got.Invariants), got.Invariants)
	}
	violated := 0
	for _, inv := range got.Invariants {
		if inv.Status == "violated" {
			violated++
			if inv.Evidence == "" {
				t.Errorf("invariant %q is violated with no evidence", inv.Name)
			}
		}
	}
	if violated == 0 {
		t.Errorf("a workspace whose artifacts contradict each other produced no violation: %+v", got.Invariants)
	}
	if s := xaStatusOf(t, got, "verdict-agreement"); s != "violated" {
		t.Errorf("verdict-agreement = %q through the real path, want violated (sentinel FAIL vs acs-verdict PASS)", s)
	}
}

// TestFinalizeCycle_CrossArtifactViolationsNeverBlockTheCycle — the NEGATIVE.
// Four violations on a PASS cycle change nothing: the verdict stays PASS and no
// system-failure signal is raised. This is the inbox record's advisory ratchet,
// and it is what a later graduation must deliberately and separately undo.
func TestFinalizeCycle_CrossArtifactViolationsNeverBlockTheCycle(t *testing.T) {
	ws := xaIncoherentWorkspace(t)
	result := &CycleResult{FinalVerdict: VerdictPASS}
	cs := CycleState{CycleID: 1676, WorkspacePath: ws, ActiveWorktree: t.TempDir()}

	if _, err := xaOrchestrator().finalizeCycle(context.Background(), cs, 1676, "same-head", "", result, &State{}, nil); err != nil {
		t.Fatalf("finalizeCycle: %v", err)
	}

	if result.FinalVerdict != VerdictPASS {
		t.Errorf("cross-artifact violations changed the verdict to %q — the stack is advisory this cycle", result.FinalVerdict)
	}
	if result.SystemFailure != nil {
		t.Errorf("cross-artifact violations raised a system failure (%+v) — advisory findings must not halt or escalate", result.SystemFailure)
	}
	xaReadWireReport(t, ws) // the finding is still RECORDED; it just does not bite
}

// TestFinalizeCycle_CrossArtifactBindsTheLaneWorktreeNotTheProjectRoot — AC3 at
// the seam. The cited evidence path exists ONLY under the projectRoot argument,
// so an implementation that resolves against the project root reports ok while
// the lane's own tree never had the file. Binding cs.ActiveWorktree is the
// #612 lesson: a cross-artifact check must read the tree the lane actually
// wrote. The second half proves the check is not simply always-violated.
func TestFinalizeCycle_CrossArtifactBindsTheLaneWorktreeNotTheProjectRoot(t *testing.T) {
	projectRoot, lane := t.TempDir(), t.TempDir()
	for _, root := range []string{projectRoot} {
		if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
			t.Fatalf("mkdir %s/docs: %v", root, err)
		}
		if err := os.WriteFile(filepath.Join(root, "docs", "evidence.md"), []byte("# only in the project root\n"), 0o644); err != nil {
			t.Fatalf("write evidence: %v", err)
		}
	}

	run := func(t *testing.T, worktree string) xaWireReport {
		t.Helper()
		ws := t.TempDir()
		xaWireSentinel(t, ws, "PASS", []string{"docs/evidence.md"})
		if err := os.WriteFile(filepath.Join(ws, "acs-verdict.json"), []byte(`{"verdict":"PASS"}`), 0o644); err != nil {
			t.Fatalf("write acs-verdict.json: %v", err)
		}
		result := &CycleResult{FinalVerdict: VerdictPASS}
		cs := CycleState{CycleID: 1676, WorkspacePath: ws, ActiveWorktree: worktree}
		if _, err := xaOrchestrator().finalizeCycle(context.Background(), cs, 1676, "same-head", projectRoot, result, &State{}, nil); err != nil {
			t.Fatalf("finalizeCycle: %v", err)
		}
		return xaReadWireReport(t, ws)
	}

	if s := xaStatusOf(t, run(t, lane), "referenced-paths-exist"); s != "violated" {
		t.Errorf("referenced-paths-exist = %q; a path that exists only under the project root is NOT evidence this lane produced (#612)", s)
	}

	if err := os.MkdirAll(filepath.Join(lane, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir lane/docs: %v", err)
	}
	if err := os.WriteFile(filepath.Join(lane, "docs", "evidence.md"), []byte("# now in the lane worktree\n"), 0o644); err != nil {
		t.Fatalf("write lane evidence: %v", err)
	}
	if s := xaStatusOf(t, run(t, lane), "referenced-paths-exist"); s != "ok" {
		t.Errorf("referenced-paths-exist = %q after the cited path landed in the lane worktree, want ok", s)
	}
}

// TestFinalizeCycle_VerdictIncoherenceFloorSurvivesTheAdvisory — AC4's
// no-regression half at the seam. The ADR-0072 forgery halt keeps firing
// exactly as before with the advisory aggregate running beside it, and the
// advisory record is still written on that path.
func TestFinalizeCycle_VerdictIncoherenceFloorSurvivesTheAdvisory(t *testing.T) {
	ws := t.TempDir()
	writeVerdicts(t, ws, "PASS", "PASS") // green artifacts, no contract verifier configured
	result := &CycleResult{FinalVerdict: VerdictFAIL}
	cs := CycleState{CycleID: 1676, WorkspacePath: ws, ActiveWorktree: t.TempDir()}

	if _, err := xaOrchestrator().finalizeCycle(context.Background(), cs, 1676, "same-head", "", result, &State{}, nil); err != nil {
		t.Fatalf("finalizeCycle: %v", err)
	}

	if result.SystemFailure == nil || result.SystemFailure.Category != "verdict-incoherence" {
		t.Fatalf("the recorded-FAIL-with-green-artifacts forgery halt regressed; got %+v", result.SystemFailure)
	}
	if !result.SystemFailure.Halt {
		t.Error("verdict-incoherence must remain a floor HALT")
	}
	xaReadWireReport(t, ws)
}
