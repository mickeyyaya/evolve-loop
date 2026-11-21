package core

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/core/defectledger"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

func issuesReport(lines ...string) string {
	return "## Verdict\nFAIL\n\n## Issues\n" + strings.Join(lines, "\n") + "\n"
}

type roundSetup struct {
	ws       string
	worktree string
}

func newRoundSetup(t *testing.T) roundSetup {
	t.Helper()
	wt := t.TempDir()
	writeFile(t, filepath.Join(wt, "b.go"), numberedLines("code", 40))
	return roundSetup{ws: t.TempDir(), worktree: wt}
}

func (r roundSetup) archive(t *testing.T, round int, report string) {
	t.Helper()
	writeFile(t, filepath.Join(r.ws, phasecontract.RoundArchiveFilename("audit-report.md", round)), report)
}

func (r roundSetup) live(t *testing.T, report string) {
	t.Helper()
	writeFile(t, filepath.Join(r.ws, "audit-report.md"), report)
}

func (r roundSetup) scope(t *testing.T, round int, enforce bool, hunks []convergence.Hunk) {
	t.Helper()
	scope := AuditRoundScope{Round: round, Enforce: enforce, Worktree: r.worktree, Hunks: hunks, FixHunks: hunks}
	for k := 1; k < round; k++ {
		raw, err := os.ReadFile(filepath.Join(r.ws, phasecontract.RoundArchiveFilename("audit-report.md", k)))
		if err != nil {
			t.Fatal(err)
		}
		fs := reportdoc.Findings(string(raw))
		for _, f := range fs {
			scope.KnownIDs = append(scope.KnownIDs, HostFindingID(f))
		}
		if k == round-1 {
			for _, f := range fs {
				scope.Previous = append(scope.Previous, ScopeFinding{HostID: HostFindingID(f), Severity: f.Severity, Title: findingTitleKey(f), Location: f.Location})
			}
		}
	}
	if err := writeAuditScope(r.ws, scope); err != nil {
		t.Fatal(err)
	}
}

func (r roundSetup) dispositions(t *testing.T, round int, live bool, entries ...defectledger.Entry) {
	t.Helper()
	var b strings.Builder
	b.WriteString(`{"dispositions":[`)
	for i, e := range entries {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"id":"` + e.ID + `","status":"` + e.Status + `","evidence":"` + e.Evidence + `","reason":"` + e.Reason + `"}`)
	}
	b.WriteString(`]}`)
	name := AuditRoundDispositionsFile
	if !live {
		name = phasecontract.RoundArchiveFilename(name, round)
	}
	writeFile(t, filepath.Join(r.ws, name), b.String())
}

func byTitle(j convergence.Judgment, title string) (convergence.Finding, bool) {
	for _, f := range j.Findings {
		if f.ID == title {
			return f, true
		}
	}
	return convergence.Finding{}, false
}

const (
	oldTitle = "the brief omits the failing predicate at b.go:5"
	newTitle = "brief misses the predicate at b.go:5"
)

var repairHunks = []convergence.Hunk{{File: "b.go", From: 30, To: 35}}

func TestRewordedOldFindingStaysOpen(t *testing.T) {
	t.Parallel()
	r := newRoundSetup(t)
	r.archive(t, 1, issuesReport("### M1 (MEDIUM) — "+oldTitle))
	r.live(t, issuesReport("### M2 (MEDIUM) — "+newTitle))
	r.scope(t, 2, true, repairHunks)

	rounds := auditRepairJudgments(r.ws, 2, &bytes.Buffer{})

	old, ok := byTitle(rounds[1], oldTitle)
	if !ok || old.Status != convergence.StatusOpen {
		t.Fatalf("the reworded-away old finding = %+v (present %v), want it still OPEN", old, ok)
	}
	copyOf, ok := byTitle(rounds[1], newTitle)
	if !ok || copyOf.Late {
		t.Fatalf("the reworded copy = %+v (present %v), want it present and not late: it sits at the location of an open previous finding", copyOf, ok)
	}
	for _, f := range rounds[1].Findings {
		if f.Status == convergence.StatusFixed {
			t.Fatalf("a reword turned %+v into a fixed finding", f)
		}
	}
}

func TestMissingDispositionIsOpen(t *testing.T) {
	t.Parallel()
	id := HostFindingID(reportdoc.Finding{Title: oldTitle})
	cases := map[string]struct {
		entry defectledger.Entry
		want  convergence.Status
	}{
		"no disposition":            {want: convergence.StatusOpen},
		"fixed with evidence":       {entry: defectledger.Entry{ID: id, Status: "FIXED", Evidence: "b.go:12"}, want: convergence.StatusFixed},
		"fixed with no evidence":    {entry: defectledger.Entry{ID: id, Status: "FIXED"}, want: convergence.StatusOpen},
		"fixed with a missing file": {entry: defectledger.Entry{ID: id, Status: "FIXED", Evidence: "nope.go:1"}, want: convergence.StatusOpen},
		"deferred":                  {entry: defectledger.Entry{ID: id, Status: "DEFERRED", Reason: "later"}, want: convergence.StatusOpen},
		"fixed with an unsafe path": {entry: defectledger.Entry{ID: id, Status: "FIXED", Evidence: "../b.go:1"}, want: convergence.StatusOpen},
		"fixed with words only":     {entry: defectledger.Entry{ID: id, Status: "FIXED", Evidence: "I fixed it"}, want: convergence.StatusOpen},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := newRoundSetup(t)
			r.archive(t, 1, issuesReport("### M1 (MEDIUM) — "+oldTitle))
			r.live(t, issuesReport())
			r.scope(t, 2, true, repairHunks)
			if tc.entry.ID != "" {
				r.dispositions(t, 2, true, tc.entry)
			}

			rounds := auditRepairJudgments(r.ws, 2, &bytes.Buffer{})

			got, ok := byTitle(rounds[1], oldTitle)
			if !ok || got.Status != tc.want {
				t.Fatalf("old finding = %+v (present %v), want status %s", got, ok, tc.want)
			}
		})
	}
}

func TestDispositionsOfAnEarlierRoundAreReadFromItsArchive(t *testing.T) {
	t.Parallel()
	id := HostFindingID(reportdoc.Finding{Title: oldTitle})
	r := newRoundSetup(t)
	r.archive(t, 1, issuesReport("### M1 (MEDIUM) — "+oldTitle))
	r.archive(t, 2, issuesReport())
	r.live(t, issuesReport())
	r.scope(t, 2, true, repairHunks)
	r.scope(t, 3, true, repairHunks)
	r.dispositions(t, 2, false, defectledger.Entry{ID: id, Status: "FIXED", Evidence: "b.go:12"})

	rounds := auditRepairJudgments(r.ws, 3, &bytes.Buffer{})

	if got, _ := byTitle(rounds[1], oldTitle); got.Status != convergence.StatusFixed {
		t.Fatalf("round 2 old finding = %+v, want FIXED by the round-2 archive", got)
	}
	if _, ok := byTitle(rounds[2], oldTitle); ok {
		t.Fatalf("a finding fixed in round 2 reappears in round 3: %+v", rounds[2].Findings)
	}
}

func TestShadowAndMissingScopeLeaveTheJudgmentsUnchanged(t *testing.T) {
	t.Parallel()
	for name, setup := range map[string]func(*testing.T, roundSetup){
		"shadow scope": func(t *testing.T, r roundSetup) { r.scope(t, 2, false, repairHunks) },
		"no scope":     func(t *testing.T, r roundSetup) {},
	} {
		t.Run(name, func(t *testing.T) {
			r := newRoundSetup(t)
			r.archive(t, 1, issuesReport("### M1 (MEDIUM) — "+oldTitle))
			r.live(t, issuesReport("### M2 (MEDIUM) — "+newTitle))
			setup(t, r)

			got := auditRepairJudgments(r.ws, 2, &bytes.Buffer{})
			want := judgmentsOf([]string{issuesReport("### M1 (MEDIUM) — " + oldTitle), issuesReport("### M2 (MEDIUM) — " + newTitle)})

			if !sameJudgments(got, want) {
				t.Fatalf("judgments = %+v, want the unscoped %+v", got, want)
			}
		})
	}
}

func sameJudgments(a, b []convergence.Judgment) bool {
	ra, _ := convergenceInputJSON(a)
	rb, _ := convergenceInputJSON(b)
	return ra == rb
}

func convergenceInputJSON(rounds []convergence.Judgment) (string, error) {
	in := auditRepairConvergenceInput(policy.Policy{}.ConvergenceConfig(), rounds, convergence.TierEffort{}, convergence.TierEffort{}, nil)
	raw, err := json.Marshal(in)
	return string(raw), err
}

func TestReplayCycle2035Shape(t *testing.T) {
	t.Parallel()
	r := newRoundSetup(t)
	r.archive(t, 1, issuesReport(
		"### M1 (MEDIUM) — the brief omits the failing predicate at b.go:5",
		"### M2 (MEDIUM) — the retry envelope ignores the attempt count at b.go:8"))
	r.archive(t, 2, issuesReport(
		"### M1 (MEDIUM) — the brief omits the failing predicate at b.go:5",
		"### M2 (MEDIUM) — the retry envelope ignores the attempt count at b.go:8",
		"### M3 (MEDIUM) — the corrected document keeps a stale claim at b.go:31"))
	r.live(t, issuesReport(
		"### M1 (MEDIUM) — the brief omits the failing predicate at b.go:5",
		"### M2 (MEDIUM) — the retry envelope ignores the attempt count at b.go:8",
		"### M3 (MEDIUM) — the corrected document keeps a stale claim at b.go:31",
		"### H1 (HIGH) — a real defect in untouched code at b.go:20",
		"### M4 (MEDIUM) — a log line names the wrong phase at b.go:12"))
	r.scope(t, 2, true, repairHunks)
	noFixThisRound := repairHunks
	r.scope(t, 3, true, noFixThisRound)

	rounds := auditRepairJudgments(r.ws, 3, &bytes.Buffer{})

	round3 := rounds[2]
	for title, wantLate := range map[string]bool{
		"the corrected document keeps a stale claim at b.go:31": false,
		"a real defect in untouched code at b.go:20":            true,
		"a log line names the wrong phase at b.go:12":           true,
		"the brief omits the failing predicate at b.go:5":       false,
	} {
		f, ok := byTitle(round3, title)
		if !ok || f.Late != wantLate {
			t.Fatalf("%q = %+v (present %v), want late=%v", title, f, ok, wantLate)
		}
	}
	d := convergence.Decide(auditRepairConvergenceInput(policy.Policy{}.ConvergenceConfig(), rounds, convergence.TierEffort{}, convergence.TierEffort{}, nil))
	if d.Action == convergence.ActionLand {
		t.Fatalf("the late HIGH and the in-hunk MEDIUM must block, decision = %+v", d)
	}

	onlyLate := newRoundSetup(t)
	onlyLate.archive(t, 1, issuesReport("### M1 (MEDIUM) — a stale claim at b.go:31"))
	onlyLate.live(t, issuesReport("### M4 (MEDIUM) — a log line names the wrong phase at b.go:12"))
	onlyLate.scope(t, 2, true, repairHunks)
	onlyLate.dispositions(t, 2, true, defectledger.Entry{ID: HostFindingID(reportdoc.Finding{Title: "a stale claim at b.go:31"}), Status: "FIXED", Evidence: "b.go:31"})
	pair := auditRepairJudgments(onlyLate.ws, 2, &bytes.Buffer{})
	d = convergence.Decide(auditRepairConvergenceInput(policy.Policy{}.ConvergenceConfig(), pair, convergence.TierEffort{}, convergence.TierEffort{}, nil))
	if d.Action != convergence.ActionLand || len(d.File) != 1 {
		t.Fatalf("a late MEDIUM alone must be filed and land, decision = %+v", d)
	}
}

func TestAnUnreadableDispositionsFileLeavesEveryPreviousFindingOpenAndSaysSo(t *testing.T) {
	t.Parallel()
	r := newRoundSetup(t)
	r.archive(t, 1, issuesReport("### M1 (MEDIUM) — "+oldTitle))
	r.live(t, issuesReport())
	r.scope(t, 2, true, repairHunks)
	writeFile(t, filepath.Join(r.ws, AuditRoundDispositionsFile), "{not json")
	var warn bytes.Buffer

	rounds := auditRepairJudgments(r.ws, 2, &warn)

	if got, _ := byTitle(rounds[1], oldTitle); got.Status != convergence.StatusOpen {
		t.Fatalf("old finding = %+v, want open", got)
	}
	if !strings.Contains(warn.String(), AuditRoundDispositionsFile) {
		t.Fatalf("warning = %q, want it to name the unreadable file", warn.String())
	}
}

func TestAnUnreadableScopeFileIsNoScope(t *testing.T) {
	t.Parallel()
	r := newRoundSetup(t)
	writeFile(t, filepath.Join(r.ws, phasecontract.RoundArchiveFilename("audit-scope.json", 2)), "{not json")
	if got := readAuditScope(r.ws, 2); got != nil {
		t.Fatalf("scope = %+v from a corrupt file", got)
	}
	if got := readAuditScope(r.ws, 3); got != nil {
		t.Fatalf("scope = %+v from a missing file", got)
	}
}

func TestACarriedFindingWithNoDispositionIsListedAgainInTheNextRoundScope(t *testing.T) {
	t.Parallel()
	f := newScopeFixture(t)
	f.build(t)
	f.sealRound(t, 1, issuesReport("### M1 (MEDIUM) — "+oldTitle))
	writeFile(t, filepath.Join(f.wt, "a.go"), numberedLines("fix", 30))
	scope2, reason := f.scope(2)
	if reason != "" {
		t.Fatal(reason)
	}
	scope2.Enforce = true
	scope2.Worktree = f.wt
	if err := writeAuditScope(f.ws, scope2); err != nil {
		t.Fatal(err)
	}
	f.sealRound(t, 2, issuesReport())

	scope3, reason := f.scope(3)
	if reason != "" {
		t.Fatal(reason)
	}

	if len(scope3.Previous) != 1 || scope3.Previous[0].Title != oldTitle || scope3.Previous[0].Location != "b.go:5" {
		t.Fatalf("previous = %+v, want the carried round-1 finding with its location", scope3.Previous)
	}
}

func TestFixHunksOfALaterRoundHoldOnlyItsOwnRepair(t *testing.T) {
	t.Parallel()
	r := newRoundSetup(t)
	r.archive(t, 1, issuesReport("### M1 (MEDIUM) — "+oldTitle))
	r.archive(t, 2, issuesReport("### M1 (MEDIUM) — "+oldTitle))
	r.live(t, issuesReport("### M1 (MEDIUM) — "+oldTitle))
	r.scope(t, 2, true, repairHunks)
	fixTwo := []convergence.Hunk{{File: "b.go", From: 10, To: 12}}
	cumulative := append(append([]convergence.Hunk(nil), repairHunks...), fixTwo...)
	scope := AuditRoundScope{Round: 3, Enforce: true, Worktree: r.worktree, Hunks: cumulative, FixHunks: fixTwo}
	if err := writeAuditScope(r.ws, scope); err != nil {
		t.Fatal(err)
	}

	rounds := auditRepairJudgments(r.ws, 3, &bytes.Buffer{})

	if !reflect.DeepEqual(rounds[2].FixHunks, fixTwo) {
		t.Fatalf("round 3 FixHunks = %+v, want only the hunks of fix 2 %+v: a finding inside fix 1 must not count as damage of fix 2", rounds[2].FixHunks, fixTwo)
	}
}
