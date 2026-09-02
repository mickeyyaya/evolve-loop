package dashboard

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const round1 = "# Audit Report — Cycle 1605 (round 1)\n\n## Verdict\n\n**FAIL**\n\n## Issues\n\n" +
	"### H1 (HIGH) — caller-proof hard floor violated\nb\n\n### H2 (HIGH) — slug has no grading contract\nb\n\n### M1 (MEDIUM) — explanation names an area not in the diff\nb\n"
const round2 = "# Audit Report — Cycle 1605 (round 2)\n\n**Verdict: FAIL**\n\n## Issues\n\n" +
	"### H1 (HIGH) — caller-proof hard floor violated for the second consecutive round\nb\n\n### H2 (HIGH) — slug has no grading contract\nb\n"
const final = "# Audit Report — Cycle 1605 (round 3)\n\n## Verdict\n\n**FAIL**\n\n## Issues\n\n" +
	"### H1 (HIGH) — caller-proof hard floor violated for the third consecutive round\nb\n\n### L1 (LOW, provenance drift)\nb\n"

func seedFailure(t *testing.T, ws string) {
	t.Helper()
	writeFile(t, filepath.Join(ws, "audit-report.round1.md"), round1)
	writeFile(t, filepath.Join(ws, "audit-report.round2.md"), round2)
	writeFile(t, filepath.Join(ws, "audit-report.md"), final)
	writeFile(t, filepath.Join(ws, "failure-decision.json"),
		`{"category":"code-audit-fail","level":"task","action":"retry-with-fix","fix_type":"address-audit-findings","evidence":"e","justification":"j","schema_version":1}`)
	writeFile(t, filepath.Join(ws, "disposition.json"),
		`{"cycle":1605,"fingerprint":"audit|gate-block|62e800da6d1f","recurrence":0,"legitimacy":"legit-rejection","root_cause":{"layer":"task-code","summary":"repair rounds proved adjacent gates"},"salvage":{"worktree_has_value":true,"pointer":"/wt/cycle-1605 (base 1b9b53ea)"},"urgency":"P1","routing":"carryover"}`)
	writeFile(t, filepath.Join(ws, "audit-fail-reason.json"),
		`{"schema_version":1,"phase":"audit","reasons":["EGPS: acs-verdict.json ship_eligible=false","apicover -enforce flagged 2 line(s)"]}`)
	writeFile(t, filepath.Join(ws, "failure-digest.json"),
		`{"cycle":1605,"phase":"audit","pre_class":"gate-block","fingerprint":"audit|gate-block|62e800da6d1f"}`)
}

func TestReadFailure_AllPanelsAndRoundHistory(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	ws := core.RunWorkspacePath(root, 1605)
	seedFailure(t, ws)

	f, warns := readFailure(ws, nil)
	if len(warns) != 0 {
		t.Fatalf("warnings: %v", warns)
	}
	if f == nil {
		t.Fatal("readFailure = nil")
	}
	if f.Category != "code-audit-fail" || f.Level != "task" || f.Action != "retry-with-fix" || f.FixType != "address-audit-findings" {
		t.Fatalf("decision = %+v", f)
	}
	if f.Fingerprint != "audit|gate-block|62e800da6d1f" || f.PreClass != "gate-block" {
		t.Fatalf("identity = %q/%q", f.Fingerprint, f.PreClass)
	}
	if f.Legitimacy != "legit-rejection" || f.Layer != "task-code" || f.RootCause != "repair rounds proved adjacent gates" ||
		f.Salvage != "/wt/cycle-1605 (base 1b9b53ea)" || f.Urgency != "P1" {
		t.Fatalf("disposition = %+v", f)
	}
	if len(f.GateReasons) != 2 {
		t.Fatalf("gate reasons = %v", f.GateReasons)
	}
	if len(f.Findings) != 2 || f.Findings[0].ID != "H1" || f.Findings[1].ID != "L1" {
		t.Fatalf("final findings (severity-sorted) = %+v", f.Findings)
	}
	if len(f.Rounds) != 3 {
		t.Fatalf("rounds = %+v", f.Rounds)
	}
	r1, r2, r3 := f.Rounds[0], f.Rounds[1], f.Rounds[2]
	if r1.Round != 1 || r1.Verdict != "FAIL" || r1.New != 3 || r1.Resolved != 0 {
		t.Fatalf("r1 = %+v", r1)
	}
	if r2.Round != 2 || r2.Carried != 2 || r2.Resolved != 1 || r2.New != 0 {
		t.Fatalf("r2 = %+v (H1+H2 carried, M1 resolved)", r2)
	}
	if r3.Round != 3 || r3.Carried != 1 || r3.Resolved != 1 || r3.New != 1 {
		t.Fatalf("r3 = %+v (H1 carried, H2 resolved, L1 new)", r3)
	}
}

func TestReadFailure_DossierFallbackWhenWorkspaceGone(t *testing.T) {
	t.Parallel()
	d := failDossier(1580, "audit|infra-error|4dfa0ca6ff87")
	f, _ := readFailure(filepath.Join(t.TempDir(), "missing"), &d)
	if f == nil || f.Fingerprint != "audit|infra-error|4dfa0ca6ff87" || f.PreClass != "gate-block" || len(f.GateReasons) != 1 {
		t.Fatalf("dossier-only failure = %+v", f)
	}
}

func TestReadFailure_NothingToReportIsNil(t *testing.T) {
	t.Parallel()
	if f, _ := readFailure(filepath.Join(t.TempDir(), "missing"), nil); f != nil {
		t.Fatalf("want nil, got %+v", f)
	}
	d := passDossier(1)
	if f, _ := readFailure(filepath.Join(t.TempDir(), "missing"), &d); f != nil {
		t.Fatalf("PASS dossier must yield nil, got %+v", f)
	}
}

func TestReadFailure_SingleRoundReportOnly(t *testing.T) {
	t.Parallel()
	ws := core.RunWorkspacePath(t.TempDir(), 3)
	writeFile(t, filepath.Join(ws, "audit-report.md"), round1)
	f, _ := readFailure(ws, nil)
	if f == nil || len(f.Rounds) != 1 || f.Rounds[0].Round != 1 || len(f.Findings) != 3 {
		t.Fatalf("single round = %+v", f)
	}
}

// A torn or corrupt failure artifact must surface as a warning, never as
// "nothing recorded yet" — the panel is the operator's 30-second triage.
func TestReadFailure_TornArtifactIsWarnedNotSilent(t *testing.T) {
	t.Parallel()
	ws := core.RunWorkspacePath(t.TempDir(), 9)
	writeFile(t, filepath.Join(ws, dispositionFile), `{"cycle":9,"legit`)
	writeFile(t, filepath.Join(ws, failureDecisionFile), `{"category":"code-audit-fail","level":"task","action":"retry-with-fix"}`)
	f, warns := readFailure(ws, nil)
	if f == nil || f.Category != "code-audit-fail" {
		t.Fatalf("the readable artifact must still render: %+v", f)
	}
	if len(warns) != 1 || !strings.Contains(warns[0], dispositionFile) {
		t.Fatalf("torn disposition.json must be a warning naming the file, got %v", warns)
	}
}

// The round archives are read by the registry-derived name the writer uses.
func TestReadAuditRounds_UsesRegistryArchiveNames(t *testing.T) {
	t.Parallel()
	ws := core.RunWorkspacePath(t.TempDir(), 4)
	writeFile(t, filepath.Join(ws, phasecontract.RoundArchiveFilename(auditReportName, 1)), round1)
	writeFile(t, filepath.Join(ws, auditReportName), final)
	rounds, _ := readAuditRounds(ws)
	if len(rounds) != 2 || rounds[0].Verdict != "FAIL" || len(rounds[1].Findings) != 2 {
		t.Fatalf("rounds = %+v", rounds)
	}
	// A verdict sentinel outranks the prose grammar, as in the audit gate.
	writeFile(t, filepath.Join(ws, auditReportName), final+"\n"+phasecontract.RenderVerdictSentinel("audit", "WARN")+"\n")
	if got, _ := readAuditRounds(ws); got[1].Verdict != "WARN" {
		t.Fatalf("sentinel must win: %+v", got[1])
	}
}

// Archive indices are not contiguous when a dispatch died before writing its
// report: round2 + live with no round1 must label the rounds 2 and 3 and diff
// against the round that exists, never collapse to a single "r1".
func TestReadAuditRounds_NonContiguousArchiveIndices(t *testing.T) {
	t.Parallel()
	ws := core.RunWorkspacePath(t.TempDir(), 8)
	writeFile(t, filepath.Join(ws, phasecontract.RoundArchiveFilename(auditReportName, 2)), round2)
	writeFile(t, filepath.Join(ws, auditReportName), final)
	rounds, warns := readAuditRounds(ws)
	if len(warns) != 0 {
		t.Fatalf("warnings: %v", warns)
	}
	if len(rounds) != 2 || rounds[0].Round != 2 || rounds[1].Round != 3 {
		t.Fatalf("rounds = %+v, want labels 2 then 3", rounds)
	}
	if rounds[1].Carried != 1 || rounds[1].Resolved != 1 || rounds[1].New != 1 {
		t.Fatalf("final round delta vs round 2 = %+v (H1 carried, H2 resolved, L1 new)", rounds[1])
	}
}
