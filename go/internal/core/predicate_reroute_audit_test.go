package core

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/core/outcome"
	"github.com/mickeyyaya/evolve-loop/go/internal/testaudit"
)

const builderFileFinding = "gofmt: 1 file(s) unformatted"

func constantFloor(lines ...string) *changingFloor {
	return &changingFloor{fixed: func(int) []string { return lines }}
}

func TestBuildFloor_ATestAuditFindingInACyclePredicateFileRerunsTDDWithTheFindingAtOnce(t *testing.T) {
	t.Parallel()
	line := auditFinding(testaudit.RowMissing, "acs/cycle10")

	run := runPredicateCycle(t, constantFloor(line))

	builds, tdds := phaseRuns(run.seq, PhaseBuild), phaseRuns(run.seq, PhaseTDD)
	if len(tdds) < 2 || tdds[1] != builds[0]+1 {
		t.Fatalf("sequence = %v; want TDD right after the first rejected Build, with no Build correction first", run.seq)
	}
	if brief := run.tdd.requests[1].Context[CtxKeyPredicateRepair]; brief != line {
		t.Errorf("TDD brief = %q, want the test-audit line verbatim", brief)
	}
}

func TestBuildFloor_AFindingInABuilderFileStillGetsABuilderCorrection(t *testing.T) {
	t.Parallel()
	floor := &changingFloor{fixed: func(call int) []string {
		if call > 1 {
			return nil
		}
		return []string{auditFinding(testaudit.RowMissing, "internal/core"), builderFileFinding}
	}}

	run := runPredicateCycle(t, floor)

	if tdds := phaseRuns(run.seq, PhaseTDD); len(tdds) != 1 {
		t.Errorf("sequence = %v; a finding in a Builder file must not reroute to TDD", run.seq)
	}
	if builds := phaseRuns(run.seq, PhaseBuild); len(builds) != 2 {
		t.Errorf("Build dispatched %d times, want the first and one correction", len(builds))
	}
}

func TestBuildFloor_AMixedRejectionBriefsTDDWithOnlyItsOwnLines(t *testing.T) {
	t.Parallel()
	own := auditFinding(testaudit.RowMissing, "acs/cycle10")
	builder := auditFinding(testaudit.RowMissing, "internal/core")

	run := runPredicateCycle(t, constantFloor(builder, own))

	if len(run.tdd.requests) < 2 {
		t.Fatalf("sequence = %v; want a TDD reroute", run.seq)
	}
	brief := run.tdd.requests[1].Context[CtxKeyPredicateRepair]
	if !strings.Contains(brief, own) || strings.Contains(brief, builder) {
		t.Errorf("TDD brief = %q, want the cycle predicate line and not the Builder line", brief)
	}
}

func TestBuildFloor_TheTestAuditRerouteIsSpentOncePerCycle(t *testing.T) {
	t.Parallel()

	run := runPredicateCycle(t, constantFloor(auditFinding(testaudit.RowMissing, "acs/cycle10")))

	if tdds := phaseRuns(run.seq, PhaseTDD); len(tdds) != 2 {
		t.Errorf("sequence = %v; the reroute happens once, then Build corrections end the cycle", run.seq)
	}
	if builds := phaseRuns(run.seq, PhaseBuild); len(builds) < 3 {
		t.Errorf("Build dispatched %d times; after the spent reroute the finding must fall back to Build corrections", len(builds))
	}
}

func TestRerouteOpen_OnlyTheMainBuildDispatchWithAFreeSlot(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		next        Phase
		remediation bool
		reroutes    int
		want        bool
	}{
		{"the main Build dispatch", PhaseBuild, false, 0, true},
		{"a remediation builder fix", PhaseBuild, true, 0, false},
		{"the slot of this cycle is spent", PhaseBuild, false, maxPredicateReroutes, false},
		{"another phase", PhaseAudit, false, 0, false},
	}
	for _, c := range cases {
		cr := &cycleRun{cs: CycleState{PredicateReroutes: c.reroutes}}
		if got := cr.rerouteOpen(c.next, &dispatchResult{remediation: c.remediation}); got != c.want {
			t.Errorf("%s: rerouteOpen = %t, want %t", c.name, got, c.want)
		}
	}
}

func TestTddReroute_NamesTheCauseThatFired(t *testing.T) {
	t.Parallel()
	cr := &cycleRun{}
	dr := &dispatchResult{}
	audit := tddOwned{Audit: []string{auditFinding(testaudit.RowMissing, "acs/cycle10")}}
	repeat := tddOwned{Predicates: []string{cycle1761Failure}}

	byAudit, ok := cr.tddReroute(PhaseBuild, dr, audit, nil)
	if !ok || byAudit.Brief[0] != audit.Audit[0] || !strings.Contains(byAudit.Reason, "test-audit") || strings.Contains(byAudit.Reason, "byte-identically") {
		t.Errorf("audit route = %+v, %t; want the test-audit reason alone", byAudit, ok)
	}
	byRepeat, ok := cr.tddReroute(PhaseBuild, dr, repeat, repeat.Predicates)
	if !ok || !strings.Contains(byRepeat.Reason, "byte-identically") || strings.Contains(byRepeat.Reason, "test-audit") {
		t.Errorf("repeat route = %+v, %t; want the repeat reason alone", byRepeat, ok)
	}
	if _, ok := cr.tddReroute(PhaseBuild, dr, repeat, nil); ok {
		t.Error("a first predicate failure with nothing to compare must not reroute")
	}
	if _, ok := cr.tddReroute(PhaseAudit, dr, audit, nil); ok {
		t.Error("another phase must not reroute")
	}
}

func TestTestAuditFailures_TheRealRatchetFindingInACyclePredicatePackageIsTDDOwned(t *testing.T) {
	t.Parallel()
	repo, base := auditRepo(t, map[string]string{"go/acs/cycle10/f.go": "package cycle10\n\nfunc F() error { return nil }\n"})
	writeFile(t, filepath.Join(repo.Dir, "go", "acs", "cycle10", "p_test.go"), strings.Replace(auditTextTest, "package q", "package cycle10", 1))

	lines := testAuditFailures(context.Background(), auditInput(t, repo, base))

	owned := tddOwnedFailures(lines, 10)
	if len(lines) != 1 || len(owned.Audit) != 1 || owned.Audit[0] != lines[0] {
		t.Fatalf("producer lines = %q, owned = %+v; want the real ratchet line classified as TDD-owned", lines, owned)
	}
	if other := tddOwnedFailures(lines, 11); len(other.Audit) != 0 {
		t.Errorf("another cycle's classifier claimed %q", other.Audit)
	}
}

func TestRoutePredicateDefectToTDD_RecordsTheReasonOfTheCauseThatFired(t *testing.T) {
	t.Parallel()
	for name, reason := range map[string]string{"test-audit": auditRerouteReason, "repeat": repeatRerouteReason} {
		workspace := t.TempDir()
		var sink injectedStderr
		cr := &cycleRun{o: NewOrchestrator(&fakeStorage{}, &fakeLedger{}, nil, WithStderr(&sink)), cs: CycleState{WorkspacePath: workspace}}

		cr.routePredicateDefectToTDD(&dispatchResult{resp: PhaseResponse{Verdict: VerdictFAIL}}, tddRoute{Brief: []string{"line"}, Reason: reason})

		if got := sink.String(); !strings.Contains(got, reason) {
			t.Errorf("%s: stderr = %q, want the reason %q", name, got, reason)
		}
		data, err := os.ReadFile(outcome.UsageSidecarPath(workspace, string(PhaseBuild)))
		if err != nil {
			t.Fatalf("%s: build usage sidecar: %v", name, err)
		}
		var sidecar outcome.UsageSidecar
		if err := json.Unmarshal(data, &sidecar); err != nil || sidecar.AbortReason != reason {
			t.Errorf("%s: recorded abort reason = %q (err %v), want %q", name, sidecar.AbortReason, err, reason)
		}
	}
}
