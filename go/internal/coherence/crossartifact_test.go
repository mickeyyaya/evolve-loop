package coherence

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// xaWriteAudit marshals the sentinel with encoding/json, never hand-spelled, so
// the fixture is the shape phasecontract actually parses.
func xaWriteAudit(t *testing.T, dir, verdict string, evidencePaths []string) {
	t.Helper()
	payload := map[string]any{"phase": "audit", "verdict": verdict, "schema_version": 1}
	if len(evidencePaths) > 0 {
		payload["schema_version"] = 2
		payload["failure"] = map[string]any{
			"class":          "code-audit-fail",
			"defects":        []string{"H1: the cited artifact does not exist"},
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

func xaWriteRaw(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// xaACS takes the claimed total and counts explicitly so a test can make the
// claim disagree with its own results.
func xaACS(t *testing.T, verdict string, total, green, red, skip int, results []string) string {
	t.Helper()
	rows := make([]map[string]any, 0, len(results))
	for i, r := range results {
		rows = append(rows, map[string]any{
			"ac_id":     "cycle1676/TestC1676_" + string(rune('a'+i)),
			"predicate": "go/acs/cycle1676/...",
			"exit_code": map[bool]int{true: 1, false: 0}[r == "red"],
			"result":    r,
		})
	}
	b, err := json.Marshal(map[string]any{
		"schema_version":  "1.0",
		"cycle":           1676,
		"predicate_suite": map[string]any{"this_cycle_count": len(results), "total": total},
		"results":         rows,
		"green_count":     green,
		"red_count":       red,
		"skip_count":      skip,
		"verdict":         verdict,
		"ship_eligible":   red == 0,
	})
	if err != nil {
		t.Fatalf("marshal acs-verdict: %v", err)
	}
	return string(b)
}

type xaPhase struct{ phase, started, ended string }

func xaWriteTiming(t *testing.T, dir string, entries ...xaPhase) {
	t.Helper()
	rows := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		rows = append(rows, map[string]any{
			"phase": e.phase, "duration_ms": 1000, "verdict": "PASS", "cost_usd": 0,
			"started_at": e.started, "ended_at": e.ended, "attempt_count": 1,
		})
	}
	b, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("marshal phase-timing: %v", err)
	}
	xaWriteRaw(t, dir, "phase-timing.json", string(b))
}

func xaCoherentWorkspace(t *testing.T) (workspace, worktree string) {
	t.Helper()
	workspace, worktree = t.TempDir(), t.TempDir()
	xaWriteAudit(t, workspace, "PASS", []string{"acs-verdict.json", "go/internal/coherence/coherence.go"})
	xaWriteRaw(t, workspace, "acs-verdict.json", xaACS(t, "PASS", 3, 2, 0, 1, []string{"green", "green", "skip"}))
	if err := os.MkdirAll(filepath.Join(worktree, "go", "internal", "coherence"), 0o755); err != nil {
		t.Fatalf("mkdir worktree tree: %v", err)
	}
	xaWriteRaw(t, filepath.Join(worktree, "go", "internal", "coherence"), "coherence.go", "package coherence\n")
	xaWriteTiming(t, workspace,
		xaPhase{"scout", "2026-09-14T03:41:45Z", "2026-09-14T03:45:45Z"},
		xaPhase{"tdd", "2026-09-14T03:46:23Z", "2026-09-14T03:48:15Z"},
		xaPhase{"build", "2026-09-14T03:48:15Z", "2026-09-14T03:59:00Z"},
		xaPhase{"audit", "2026-09-14T04:00:00Z", "2026-09-14T04:10:00Z"},
	)
	return workspace, worktree
}

func xaFind(t *testing.T, r InvariantReport, name string) Invariant {
	t.Helper()
	for _, inv := range r.Invariants {
		if inv.Name == name {
			return inv
		}
	}
	t.Fatalf("invariant %q absent from the report; got %+v", name, r.Invariants)
	return Invariant{}
}

func xaWantStatus(t *testing.T, r InvariantReport, name string, want InvariantStatus) Invariant {
	t.Helper()
	got := xaFind(t, r, name)
	if got.Status != want {
		t.Errorf("%s: status = %q, want %q (evidence: %s)", name, got.Status, want, got.Evidence)
	}
	if got.Status != InvariantOK && strings.TrimSpace(got.Evidence) == "" {
		t.Errorf("%s: status %q carries no evidence — an unactionable finding", name, got.Status)
	}
	return got
}

var xaAllFour = []string{
	InvariantVerdictAgreement,
	InvariantTestCounts,
	InvariantReferencedPaths,
	InvariantPhaseOrder,
}

func TestCrossArtifactInvariants_CoherentWorkspaceIsAllOK(t *testing.T) {
	ws, wt := xaCoherentWorkspace(t)
	got := CheckCrossArtifactInvariants(ws, wt)
	for _, name := range xaAllFour {
		xaWantStatus(t, got, name, InvariantOK)
	}
	if v := got.Violations(); len(v) != 0 {
		t.Errorf("a coherent workspace reported %d violation(s): %+v", len(v), v)
	}
}

func TestCrossArtifactInvariants_ReportShapeIsDeterministic(t *testing.T) {
	ws, wt := xaCoherentWorkspace(t)
	first := CheckCrossArtifactInvariants(ws, wt)

	if !first.Advisory {
		t.Error("Advisory = false; every invariant ships advisory until its false-positive rate is evidenced (inbox rule)")
	}
	if len(first.Invariants) != len(xaAllFour) {
		t.Fatalf("reported %d invariants, want exactly %d: %+v", len(first.Invariants), len(xaAllFour), first.Invariants)
	}
	for i, name := range xaAllFour {
		if first.Invariants[i].Name != name {
			t.Errorf("invariant[%d] = %q, want %q (the report order is part of the contract)", i, first.Invariants[i].Name, name)
		}
	}
	if second := CheckCrossArtifactInvariants(ws, wt); !reflect.DeepEqual(first, second) {
		t.Errorf("two evaluations of the same workspace differ — the aggregate is not deterministic\nfirst:  %+v\nsecond: %+v", first, second)
	}
}

func TestCrossArtifactInvariants_VerdictDisagreementIsViolatedWithBothSides(t *testing.T) {
	ws, wt := xaCoherentWorkspace(t)
	xaWriteAudit(t, ws, "FAIL", nil)

	got := xaWantStatus(t, CheckCrossArtifactInvariants(ws, wt), InvariantVerdictAgreement, InvariantViolated)
	if !strings.Contains(got.Evidence, "FAIL") || !strings.Contains(got.Evidence, "PASS") {
		t.Errorf("evidence must name both sides of the disagreement; got %q", got.Evidence)
	}
}

func TestCrossArtifactInvariants_ProseVerdictCannotSatisfyTheSentinelInvariant(t *testing.T) {
	ws, wt := xaCoherentWorkspace(t)
	xaWriteRaw(t, ws, "audit-report.md",
		"# Audit Report\n\n## Verdict\n**PASS**\n\nAll gates PASS. Emit `<!-- evolve-verdict: "+
			`{"phase":"audit","verdict":"PASS","schema_version":2,"failure":{"class":"<failure class>",`+
			`"defects":["<one line per defect>"],"evidence_paths":["<artifact>"]}}`+" -->` when you fail.\n")

	got := xaWantStatus(t, CheckCrossArtifactInvariants(ws, wt), InvariantVerdictAgreement, InvariantIndeterminate)
	if got.Status == InvariantOK {
		t.Fatal("an arbitrary PASS string satisfied the verdict invariant — the suite is greppable")
	}
}

func TestCrossArtifactInvariants_TestCountDisagreementNamesClaimedAndCounted(t *testing.T) {
	ws, wt := xaCoherentWorkspace(t)
	// claims 3 green / 0 red over a 3-entry suite; the results say 2 green, 1 red.
	xaWriteRaw(t, ws, "acs-verdict.json", xaACS(t, "PASS", 3, 3, 0, 0, []string{"green", "green", "red"}))

	got := xaWantStatus(t, CheckCrossArtifactInvariants(ws, wt), InvariantTestCounts, InvariantViolated)
	if !strings.Contains(got.Evidence, "red") {
		t.Errorf("evidence must name the disagreeing count; got %q", got.Evidence)
	}
	if !strings.Contains(got.Evidence, "0") || !strings.Contains(got.Evidence, "1") {
		t.Errorf("evidence must carry the claimed (0) and the independently counted (1) numbers; got %q", got.Evidence)
	}
}

func TestCrossArtifactInvariants_TestCountTotalDisagreementIsViolated(t *testing.T) {
	ws, wt := xaCoherentWorkspace(t)
	xaWriteRaw(t, ws, "acs-verdict.json", xaACS(t, "PASS", 230, 2, 0, 0, []string{"green", "green"}))

	got := xaWantStatus(t, CheckCrossArtifactInvariants(ws, wt), InvariantTestCounts, InvariantViolated)
	if !strings.Contains(got.Evidence, "230") {
		t.Errorf("evidence must carry the claimed total 230; got %q", got.Evidence)
	}
}

func TestCrossArtifactInvariants_MissingReferencedPathIsViolatedAndNamed(t *testing.T) {
	ws, wt := xaCoherentWorkspace(t)
	xaWriteAudit(t, ws, "PASS", []string{"acs-verdict.json", "docs/explain/builds/cycle-1676-never-written.md"})

	got := xaWantStatus(t, CheckCrossArtifactInvariants(ws, wt), InvariantReferencedPaths, InvariantViolated)
	if !strings.Contains(got.Evidence, "docs/explain/builds/cycle-1676-never-written.md") {
		t.Errorf("evidence must name the missing path; got %q", got.Evidence)
	}
	if strings.Contains(got.Evidence, "acs-verdict.json") {
		t.Errorf("evidence must name ONLY the missing path, not the resolved one; got %q", got.Evidence)
	}
}

func TestCrossArtifactInvariants_ReferencedPathsResolveUnderWorkspaceThenWorktree(t *testing.T) {
	ws, wt := xaCoherentWorkspace(t)
	decoy := t.TempDir() // an unrelated root the aggregate is never handed
	if err := os.MkdirAll(filepath.Join(decoy, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir decoy: %v", err)
	}
	xaWriteRaw(t, filepath.Join(decoy, "docs"), "note.md", "# only in the decoy root\n")
	xaWriteAudit(t, ws, "PASS", []string{"docs/note.md"})

	xaWantStatus(t, CheckCrossArtifactInvariants(ws, wt), InvariantReferencedPaths, InvariantViolated)

	if err := os.MkdirAll(filepath.Join(wt, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir worktree docs: %v", err)
	}
	xaWriteRaw(t, filepath.Join(wt, "docs"), "note.md", "# now in the lane worktree\n")
	xaWantStatus(t, CheckCrossArtifactInvariants(ws, wt), InvariantReferencedPaths, InvariantOK)
}

func TestCrossArtifactInvariants_EscapingReferencedPathIsViolatedNotResolved(t *testing.T) {
	ws, wt := xaCoherentWorkspace(t)
	escape := strings.Repeat("../", 12) + "etc/hosts"
	if _, err := os.Stat(filepath.Join(ws, escape)); err != nil {
		t.Skipf("host has no /etc/hosts to escape onto (%v) — the escape cannot be demonstrated here", err)
	}
	xaWriteAudit(t, ws, "PASS", []string{escape})

	inv := xaWantStatus(t, CheckCrossArtifactInvariants(ws, wt), InvariantReferencedPaths, InvariantViolated)
	if !strings.Contains(inv.Evidence, escape) {
		t.Errorf("evidence does not name the escaping citation: %q", inv.Evidence)
	}
}

func TestCrossArtifactInvariants_PhaseOrderViolationsAreReported(t *testing.T) {
	cases := []struct {
		name    string
		entries []xaPhase
		want    InvariantStatus
	}{
		{
			name: "chain runs backwards",
			entries: []xaPhase{
				{"scout", "2026-09-14T03:41:45Z", "2026-09-14T03:45:45Z"},
				{"build", "2026-09-14T03:20:00Z", "2026-09-14T03:30:00Z"},
				{"audit", "2026-09-14T04:00:00Z", "2026-09-14T04:10:00Z"},
			},
			want: InvariantViolated,
		},
		{
			name: "audit recorded before the build it audits",
			entries: []xaPhase{
				{"scout", "2026-09-14T03:41:45Z", "2026-09-14T03:45:45Z"},
				{"audit", "2026-09-14T03:50:00Z", "2026-09-14T03:55:00Z"},
				{"build", "2026-09-14T03:56:00Z", "2026-09-14T03:59:00Z"},
			},
			want: InvariantViolated,
		},
		{
			name: "re-dispatched build/audit rounds stay in order",
			entries: []xaPhase{
				{"scout", "2026-09-14T03:41:45Z", "2026-09-14T03:45:45Z"},
				{"tdd", "2026-09-14T03:46:00Z", "2026-09-14T03:47:00Z"},
				{"build", "2026-09-14T03:48:00Z", "2026-09-14T03:55:00Z"},
				{"audit", "2026-09-14T03:56:00Z", "2026-09-14T03:58:00Z"},
				{"build", "2026-09-14T03:59:00Z", "2026-09-14T04:05:00Z"},
				{"audit", "2026-09-14T04:06:00Z", "2026-09-14T04:09:00Z"},
			},
			want: InvariantOK,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws, wt := xaCoherentWorkspace(t)
			xaWriteTiming(t, ws, tc.entries...)
			xaWantStatus(t, CheckCrossArtifactInvariants(ws, wt), InvariantPhaseOrder, tc.want)
		})
	}
}

func TestCrossArtifactInvariants_AbsentArtifactsAreIndeterminateNotOK(t *testing.T) {
	got := CheckCrossArtifactInvariants(t.TempDir(), t.TempDir())
	for _, name := range xaAllFour {
		xaWantStatus(t, got, name, InvariantIndeterminate)
	}
	if v := got.Violations(); len(v) != 0 {
		t.Errorf("an empty workspace produced %d violation(s) — an advisory that fires on absence is a false-positive generator: %+v", len(v), v)
	}
}

func TestCrossArtifactInvariants_MalformedArtifactsAreIndeterminateNotOK(t *testing.T) {
	cases := []struct {
		name      string
		mutate    func(t *testing.T, ws string)
		invariant string
	}{
		{
			name:      "truncated acs-verdict.json",
			mutate:    func(t *testing.T, ws string) { xaWriteRaw(t, ws, "acs-verdict.json", `{"verdict":"PA`) },
			invariant: InvariantVerdictAgreement,
		},
		{
			name: "wrong-typed counts",
			mutate: func(t *testing.T, ws string) {
				xaWriteRaw(t, ws, "acs-verdict.json", `{"verdict":"PASS","red_count":"none","results":[]}`)
			},
			invariant: InvariantTestCounts,
		},
		{
			name: "no results to recount",
			mutate: func(t *testing.T, ws string) {
				xaWriteRaw(t, ws, "acs-verdict.json", xaACS(t, "PASS", 0, 0, 0, 0, nil))
			},
			invariant: InvariantTestCounts,
		},
		{
			name:      "timing log is an object, not an array",
			mutate:    func(t *testing.T, ws string) { xaWriteRaw(t, ws, "phase-timing.json", `{"phase":"build"}`) },
			invariant: InvariantPhaseOrder,
		},
		{
			name: "timestamps do not parse",
			mutate: func(t *testing.T, ws string) {
				xaWriteTiming(t, ws, xaPhase{"build", "yesterday", "later"}, xaPhase{"audit", "soon", "eventually"})
			},
			invariant: InvariantPhaseOrder,
		},
		{
			name:      "audit report is empty",
			mutate:    func(t *testing.T, ws string) { xaWriteRaw(t, ws, "audit-report.md", "") },
			invariant: InvariantReferencedPaths,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws, wt := xaCoherentWorkspace(t)
			tc.mutate(t, ws)
			got := CheckCrossArtifactInvariants(ws, wt)
			inv := xaWantStatus(t, got, tc.invariant, InvariantIndeterminate)
			if inv.Status == InvariantViolated {
				t.Errorf("%s: malformed input was reported as a VIOLATION — fail-safe means indeterminate", tc.invariant)
			}
		})
	}
}

func TestCrossArtifactInvariants_ViolationsReturnsOnlyTheViolated(t *testing.T) {
	ws, wt := xaCoherentWorkspace(t)
	xaWriteAudit(t, ws, "FAIL", nil)                                          // verdict-agreement → violated
	if err := os.Remove(filepath.Join(ws, "phase-timing.json")); err != nil { // phase-order → indeterminate
		t.Fatalf("remove phase-timing.json: %v", err)
	}

	got := CheckCrossArtifactInvariants(ws, wt)
	violations := got.Violations()
	if len(violations) != 1 {
		t.Fatalf("Violations() = %d entries, want exactly the 1 violated invariant: %+v", len(violations), violations)
	}
	if violations[0].Name != InvariantVerdictAgreement {
		t.Errorf("Violations()[0].Name = %q, want %q", violations[0].Name, InvariantVerdictAgreement)
	}
	if violations[0].Status != InvariantViolated {
		t.Errorf("Violations() returned a %q invariant", violations[0].Status)
	}
	if strings.TrimSpace(violations[0].Evidence) == "" {
		t.Error("a violation reached the advisory surface with no evidence")
	}
}

func TestCrossArtifactInvariants_ExistingVerdictCoherenceIsUntouched(t *testing.T) {
	forged := CheckVerdictCoherence(VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "PASS", AuditRan: true})
	if !forged.Incoherent || forged.Category != "verdict-incoherence" {
		t.Errorf("the forgery signature changed: %+v", forged)
	}
	explained := CheckVerdictCoherence(VerdictInputs{Recorded: "FAIL", Audit: "PASS", ACS: "PASS", AuditRan: true, SubstantiveError: true})
	if explained.Incoherent || explained.Reconciled {
		t.Errorf("a diagnosed negative must stay coherent: %+v", explained)
	}
}
