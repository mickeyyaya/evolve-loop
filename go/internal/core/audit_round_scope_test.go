package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/convergence"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
	"github.com/mickeyyaya/evolve-loop/go/internal/reportdoc"
)

const scopeCycle = 2035

type scopeFixture struct {
	repo, wt, ws string
	led          *fakeLedger
	o            *Orchestrator
}

func numberedLines(prefix string, n int) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		fmt.Fprintf(&b, "%s %d\n", prefix, i)
	}
	return b.String()
}

func newScopeFixture(t *testing.T) *scopeFixture {
	t.Helper()
	repo := stagingScopeRepo(t)
	writeFile(t, filepath.Join(repo, "a.go"), numberedLines("old", 30))
	gitOut(t, repo, "add", "a.go")
	gitOut(t, repo, "commit", "-q", "-m", "a")
	wt := filepath.Join(t.TempDir(), "wt")
	gitOut(t, repo, "worktree", "add", "--detach", "-q", wt, "HEAD")
	ws := filepath.Join(repo, ".evolve", "runs", fmt.Sprintf("cycle-%d", scopeCycle))
	led := &fakeLedger{}
	o := NewOrchestrator(nil, led, nil)
	o.now = func() time.Time { return time.Date(2026, 11, 21, 0, 0, 0, 0, time.UTC) }
	return &scopeFixture{repo: repo, wt: wt, ws: ws, led: led, o: o}
}

func (f *scopeFixture) cycleState(round int) CycleState {
	return CycleState{CycleID: scopeCycle, WorkspacePath: f.ws, ActiveWorktree: f.wt, WorktreeBaseSHA: gitHead(f.repo), AuditDispatches: round}
}

func gitHead(repo string) string {
	out, _, _ := gitCapture(context.Background(), repo, "rev-parse", "HEAD")
	return strings.TrimSpace(out)
}

func (f *scopeFixture) build(t *testing.T, deliverables ...string) {
	t.Helper()
	body := "Wrote"
	for _, d := range deliverables {
		body += " `" + d + "`"
	}
	writeFile(t, filepath.Join(f.ws, "build-report.md"), body+".\n")
}

func (f *scopeFixture) sealRound(t *testing.T, round int, report string) {
	t.Helper()
	writeFile(t, filepath.Join(f.ws, "audit-report.md"), report)
	f.o.emitPhaseBindings(context.Background(), scopeCycle, f.repo, f.cycleState(round), PhaseAudit, VerdictFAIL)
	archive := filepath.Join(f.ws, phasecontract.RoundArchiveFilename("audit-report.md", round))
	if err := os.Rename(filepath.Join(f.ws, "audit-report.md"), archive); err != nil {
		t.Fatal(err)
	}
}

func (f *scopeFixture) scope(round int) (AuditRoundScope, string) {
	return computeAuditScope(context.Background(), scopeInputs{
		ledger: f.led, cycle: scopeCycle, round: round, projectRoot: f.repo, worktree: f.wt, workspace: f.ws,
	})
}

const round1Report = "## Verdict\nFAIL\n\n## Issues\n### H1 (HIGH) — the loop swallows the error at a.go:3\n"

func TestScopeHunksIncludeNewFile(t *testing.T) {
	t.Parallel()
	f := newScopeFixture(t)
	f.build(t)
	f.sealRound(t, 1, round1Report)

	edited := numberedLines("old", 10) + numberedLines("new", 5) + strings.TrimPrefix(numberedLines("old", 30), numberedLines("old", 10))
	writeFile(t, filepath.Join(f.wt, "a.go"), edited)
	writeFile(t, filepath.Join(f.wt, "n.go"), numberedLines("fresh", 4))
	f.build(t, "n.go")

	scope, reason := f.scope(2)
	if reason != "" {
		t.Fatalf("scope unavailable: %s", reason)
	}
	want := []convergence.Hunk{{File: "a.go", From: 11, To: 15}, {File: "n.go", From: 1, To: 4}}
	if !reflect.DeepEqual(scope.Hunks, want) {
		t.Fatalf("hunks = %+v, want post-image ranges %+v", scope.Hunks, want)
	}
	if !reflect.DeepEqual(scope.FixHunks, want) {
		t.Fatalf("fix hunks of the first repair = %+v, want %+v", scope.FixHunks, want)
	}
	for loc, wantLate := range map[string]bool{"a.go:12": false, "n.go:2": false, "a.go:3": false, "a.go:4": true, "a.go:16": true, "a.go:30": true} {
		got := scope.Late(reportdoc.Finding{ID: "M1", Severity: "MEDIUM", Title: "brand new " + loc, Location: loc})
		if got != wantLate {
			t.Errorf("Late(%s) = %v, want %v", loc, got, wantLate)
		}
	}
}

func TestScopeHunks_APureDeletionFailsClosedAndALaterHunkKeepsItsPostImageLines(t *testing.T) {
	t.Parallel()
	f := newScopeFixture(t)
	f.build(t)
	f.sealRound(t, 1, round1Report)
	lines := strings.Split(strings.TrimSuffix(numberedLines("old", 30), "\n"), "\n")
	edited := append(append([]string{}, lines[:4]...), lines[7:]...)
	edited = append(edited[:19], append([]string{"inserted one", "inserted two"}, edited[19:]...)...)
	writeFile(t, filepath.Join(f.wt, "a.go"), strings.Join(edited, "\n")+"\n")

	scope, reason := f.scope(2)
	if reason != "" {
		t.Fatalf("scope unavailable: %s", reason)
	}
	want := []convergence.Hunk{{File: "a.go", From: 4, To: 4}, {File: "a.go", From: 20, To: 21}}
	if !reflect.DeepEqual(scope.Hunks, want) {
		t.Fatalf("hunks = %+v, want %+v", scope.Hunks, want)
	}
}

func TestScopeFixHunksAreTheLatestRepairAlone(t *testing.T) {
	t.Parallel()
	f := newScopeFixture(t)
	f.build(t)
	f.sealRound(t, 1, round1Report)
	writeFile(t, filepath.Join(f.wt, "a.go"), strings.Replace(numberedLines("old", 30), "old 5\n", "fix1 5\n", 1))
	f.sealRound(t, 2, round1Report)
	writeFile(t, filepath.Join(f.wt, "a.go"), strings.Replace(strings.Replace(numberedLines("old", 30), "old 5\n", "fix1 5\n", 1), "old 25\n", "fix2 25\n", 1))

	scope, reason := f.scope(3)
	if reason != "" {
		t.Fatalf("scope unavailable: %s", reason)
	}
	if want := []convergence.Hunk{{File: "a.go", From: 5, To: 5}, {File: "a.go", From: 25, To: 25}}; !reflect.DeepEqual(scope.Hunks, want) {
		t.Fatalf("cumulative hunks = %+v, want %+v", scope.Hunks, want)
	}
	if want := []convergence.Hunk{{File: "a.go", From: 25, To: 25}}; !reflect.DeepEqual(scope.FixHunks, want) {
		t.Fatalf("fix hunks = %+v, want only fix 2's %+v", scope.FixHunks, want)
	}
}

func TestScopeRoundTreeEqualsTheLaterBindingTree(t *testing.T) {
	t.Parallel()
	f := newScopeFixture(t)
	f.build(t)
	f.sealRound(t, 1, round1Report)
	writeFile(t, filepath.Join(f.wt, "a.go"), numberedLines("changed", 30))
	scope, reason := f.scope(2)
	if reason != "" {
		t.Fatalf("scope unavailable: %s", reason)
	}
	f.sealRound(t, 2, round1Report)
	if bound := f.led.entries[1].WorktreeTreeSHA; scope.Tree == "" || scope.Tree != bound {
		t.Fatalf("dispatch tree %q is not the tree the round-2 binding recorded %q", scope.Tree, bound)
	}
}

func TestScopeUnavailableIsRound1(t *testing.T) {
	t.Parallel()
	cases := map[string]func(t *testing.T, f *scopeFixture){
		"binding missing": func(t *testing.T, f *scopeFixture) { f.led.entries = nil },
		"empty tree sha": func(t *testing.T, f *scopeFixture) {
			f.led.entries[0].WorktreeTreeSHA = ""
		},
		"archive lost": func(t *testing.T, f *scopeFixture) {
			if err := os.Remove(filepath.Join(f.ws, phasecontract.RoundArchiveFilename("audit-report.md", 1))); err != nil {
				t.Fatal(err)
			}
		},
		"git error": func(t *testing.T, f *scopeFixture) {
			f.led.entries[0].WorktreeTreeSHA = strings.Repeat("0", 40)
		},
		"in-place worktree": func(t *testing.T, f *scopeFixture) { f.wt = f.repo },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newScopeFixture(t)
			f.build(t)
			f.sealRound(t, 1, round1Report)
			writeFile(t, filepath.Join(f.wt, "a.go"), numberedLines("changed", 30))
			mutate(t, f)
			scope, reason := f.scope(2)
			if reason == "" {
				t.Fatalf("scope %+v built from a %s", scope, name)
			}
			if scope.Late(reportdoc.Finding{ID: "M1", Severity: "MEDIUM", Title: "x", Location: "a.go:3"}) {
				t.Fatalf("an unavailable scope called a finding late")
			}
		})
	}
}

func TestScope_NoRoundOneAndNoHunksFileNothingLate(t *testing.T) {
	t.Parallel()
	f := newScopeFixture(t)
	f.build(t)
	f.sealRound(t, 1, round1Report)
	scope, reason := f.scope(2)
	if reason != "" {
		t.Fatalf("scope unavailable: %s", reason)
	}
	if len(scope.Hunks) != 0 {
		t.Fatalf("an unchanged tree has hunks %+v", scope.Hunks)
	}
	if scope.Late(reportdoc.Finding{ID: "M1", Severity: "MEDIUM", Title: "fresh", Location: "a.go:3"}) {
		t.Fatal("with zero cumulative hunks the scope must not call a finding late")
	}
	if _, reason := f.scope(1); reason == "" {
		t.Fatal("round 1 has a scope")
	}
}

func TestParsePostImageHunks_ReadsPostImageNumbersOfEachShape(t *testing.T) {
	t.Parallel()
	diff := strings.Join([]string{
		"diff --git a/new.go b/new.go", "new file mode 100644", "--- /dev/null", "+++ b/new.go", "@@ -0,0 +1,7 @@",
		"diff --git a/gone.go b/gone.go", "deleted file mode 100644", "--- a/gone.go", "+++ /dev/null", "@@ -1,3 +0,0 @@",
		"diff --git a/m.go b/m.go", "--- a/m.go", "+++ b/m.go", "@@ -10,0 +11,5 @@", "@@ -40 +46 @@", "@@ -50,2 +56,0 @@",
		"diff --git a/sp ace.go b/sp ace.go", "--- a/sp ace.go", "+++ b/sp ace.go\t", "@@ -1 +1,2 @@",
		"diff --git a/q.go b/q.go", "--- a/q.go", "+++ \"b/q\\303\\251.go\"", "@@ -2 +2 @@",
	}, "\n")
	want := []convergence.Hunk{
		{File: "new.go", From: 1, To: 7},
		{File: "m.go", From: 11, To: 15}, {File: "m.go", From: 46, To: 46}, {File: "m.go", From: 56, To: 56},
		{File: "sp ace.go", From: 1, To: 2}, {File: "qé.go", From: 2, To: 2},
	}
	if got := parsePostImageHunks(diff); !reflect.DeepEqual(got, want) {
		t.Fatalf("hunks = %+v, want %+v", got, want)
	}
}

func TestCheckFiledNamesEachBreachOfTheFilingRule(t *testing.T) {
	t.Parallel()
	known := reportdoc.Finding{ID: "M1", Severity: "MEDIUM", Title: "an old defect at a.go:3", Location: "a.go:3"}
	scope := &AuditRoundScope{
		KnownIDs: []string{HostFindingID(known)},
		Previous: []ScopeFinding{{HostID: HostFindingID(known), Severity: "MEDIUM", Title: known.Title, Location: "a.go:3"}},
		Hunks:    []convergence.Hunk{{File: "a.go", From: 10, To: 12}},
	}
	for name, tc := range map[string]struct {
		entry reportdoc.Finding
		want  string
	}{
		"known id":          {known, "known from an earlier round"},
		"no location":       {reportdoc.Finding{ID: "M2", Severity: "MEDIUM", Title: "no place"}, "names no path:line"},
		"inside the hunks":  {reportdoc.Finding{ID: "M2", Severity: "MEDIUM", Title: "inside", Location: "a.go:11"}, "inside the repair hunks"},
		"previous location": {reportdoc.Finding{ID: "M2", Severity: "MEDIUM", Title: "reworded", Location: "a.go:3"}, "location of an open previous finding"},
		"a late HIGH":       {reportdoc.Finding{ID: "H2", Severity: "HIGH", Title: "serious", Location: "a.go:20"}, "blocks and is never filed"},
		"a late CRITICAL":   {reportdoc.Finding{ID: "C2", Severity: "CRITICAL", Title: "severe", Location: "a.go:20"}, "blocks and is never filed"},
	} {
		got := scope.CheckFiled([]reportdoc.Finding{tc.entry})
		if len(got) != 1 || !strings.Contains(got[0], tc.want) {
			t.Errorf("%s: problems = %v, want one naming %q", name, got, tc.want)
		}
	}
	if got := scope.CheckFiled([]reportdoc.Finding{{ID: "M2", Severity: "LOW", Title: "fresh", Location: "a.go:20"}}); len(got) != 0 {
		t.Errorf("a valid late LOW has problems %v", got)
	}
}

type brokenLedger struct {
	iterErr error
	nextErr error
}

func (brokenLedger) Append(context.Context, LedgerEntry) error { return nil }
func (brokenLedger) Verify(context.Context) error              { return nil }
func (l brokenLedger) Iter(context.Context) (LedgerIterator, error) {
	if l.iterErr != nil {
		return nil, l.iterErr
	}
	return brokenIterator{err: l.nextErr}, nil
}

type brokenIterator struct{ err error }

func (i brokenIterator) Next() (LedgerEntry, bool, error) { return LedgerEntry{}, false, i.err }
func (brokenIterator) Close() error                       { return nil }

func TestScopeUnavailableWhenTheLedgerCannotBeRead(t *testing.T) {
	t.Parallel()
	for name, led := range map[string]Ledger{
		"iter": brokenLedger{iterErr: errors.New("no ledger")},
		"next": brokenLedger{nextErr: errors.New("torn line")},
		"nil":  nil,
	} {
		t.Run(name, func(t *testing.T) {
			f := newScopeFixture(t)
			f.build(t)
			f.sealRound(t, 1, round1Report)
			scope, reason := computeAuditScope(context.Background(), scopeInputs{ledger: led, cycle: scopeCycle, round: 2, projectRoot: f.repo, worktree: f.wt, workspace: f.ws})
			if reason == "" || !strings.Contains(reason, "ledger") {
				t.Fatalf("scope %+v reason %q, want a ledger reason", scope, reason)
			}
		})
	}
}

func TestScopeUnavailableWhenTheLatestRepairTreeIsUnknownToGit(t *testing.T) {
	t.Parallel()
	f := newScopeFixture(t)
	f.build(t)
	f.sealRound(t, 1, round1Report)
	writeFile(t, filepath.Join(f.wt, "a.go"), numberedLines("one", 30))
	f.sealRound(t, 2, round1Report)
	writeFile(t, filepath.Join(f.wt, "a.go"), numberedLines("two", 30))
	f.led.entries[1].WorktreeTreeSHA = strings.Repeat("0", 40)

	if _, reason := f.scope(3); !strings.HasPrefix(reason, "git:") {
		t.Fatalf("reason = %q, want a git reason", reason)
	}
}

func TestDiffPostImageHunks_ReportsAGitThatCannotStart(t *testing.T) {
	t.Parallel()
	if _, err := diffPostImageHunks(context.Background(), filepath.Join(t.TempDir(), "missing"), "a", "b"); err == nil {
		t.Fatal("a missing directory produced hunks")
	}
}

func TestAuditScopeFor_ANonWritableScopeFileIsNoScopeAndSignalled(t *testing.T) {
	t.Parallel()
	f := newScopeFixture(t)
	f.build(t)
	f.sealRound(t, 1, round1Report)
	writeFile(t, filepath.Join(f.wt, "a.go"), numberedLines("changed", 30))
	if err := os.MkdirAll(filepath.Join(f.ws, phasecontract.RoundArchiveFilename("audit-scope.json", 2), "blocker"), 0o755); err != nil {
		t.Fatal(err)
	}
	center, events := captureSignals()
	f.o = NewOrchestrator(nil, f.led, nil, WithSignalCenter(center))

	if got := f.o.auditScopeFor(context.Background(), PhaseAudit, f.cycleState(2), f.repo); got != nil {
		t.Fatalf("scope = %+v, want none when it cannot be persisted", got)
	}
	if len(*events) != 1 || !strings.Contains((*events)[0].Fields["reason"], "cannot be written") {
		t.Fatalf("signals = %+v", *events)
	}
}
