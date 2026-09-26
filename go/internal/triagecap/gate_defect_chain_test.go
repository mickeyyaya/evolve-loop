package triagecap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// gapGoldenPkgs holds the goldens' true targets, the phantom sources (scout, sysexec) and distractors;
// with it, a counter that reads evidence citations counts 7 on triage-cycle449-golden.md.
var gapGoldenPkgs = []string{
	"core", "bridge", "audit",
	"scout", "sysexec",
	"config", "router", "llmroute", "recovery", "evidence", "paths",
}

func TestCountCommittedFloors_Cycle449GoldenReplay(t *testing.T) {
	artifact := readFixture(t, "triage-cycle449-golden.md")
	got := CountCommittedFloors(artifact, gapGoldenPkgs)
	if got != 3 {
		t.Errorf("cycle-449 golden committed floors = %d, want 3 (core/bridge/audit floor targets; evidence citations must not count)", got)
	}
}

func TestCountCommittedFloors_Cycle448GoldenReplay(t *testing.T) {
	artifact := readFixture(t, "triage-cycle448-golden.md")
	got := CountCommittedFloors(artifact, gapGoldenPkgs)
	if got != 4 {
		t.Errorf("cycle-448 golden committed floors = %d, want 4 (core+audit+bridge targets in item 1, core target in item 2)", got)
	}
}

func TestCountCommittedFloors_EvidenceCitationDoesNotCount(t *testing.T) {
	artifact := "## top_n (commit to THIS cycle)\n" +
		"- salvage-core-coverage: raise core coverage floor to ≥85.0% — priority=H, evidence=scout fresh cover-func (bridge 93.5%; matchExhausted 66.7% in audit), source=scout\n"
	got := CountCommittedFloors(artifact, gapGoldenPkgs)
	if got != 1 {
		t.Errorf("evidence-citation item floors = %d, want 1 (core is the only floor TARGET; bridge/audit/scout are evidence prose)", got)
	}
}

// newGateDefectReviewer captures formatted log lines; newTestReviewer keeps only format strings, which hide args.
func newGateDefectReviewer(stage config.Stage, window []core.TriageThroughputEntry, fails []FailEntry, logs *[]string) *CapReviewer {
	r := newCapReviewer(stage)
	r.pkgsFn = func(string) []string { return []string{"swarmrunner", "bridge", "evalgate"} }
	r.windowFn = func(string) []core.TriageThroughputEntry { return window }
	r.failsFn = func(string) []FailEntry { return fails }
	r.logf = func(f string, a ...any) {
		if logs != nil {
			*logs = append(*logs, fmt.Sprintf(f, a...))
		}
	}
	return r
}

// tightWindow yields K=1 ⇒ cap 2, so the 3-floor overpackedArtifact rejects.
var tightWindow = []core.TriageThroughputEntry{{Cycle: 300, Floors: 1}}

func TestCapReviewer_RejectReasonStatesDeclarationEscape(t *testing.T) {
	ws := writeTriageWorkspace(t, overpackedArtifact)
	r := newGateDefectReviewer(config.StageEnforce, tightWindow, nil, nil)
	rr := r.Review(context.Background(), reviewIn(ws))
	if rr.Approve {
		t.Fatal("3 floors > cap 2 must reject at enforce")
	}
	for _, want := range []string{"triage-decision.json", "committed_floors"} {
		if !strings.Contains(rr.Reason, want) {
			t.Errorf("reject reason must state the declaration-primary escape; missing %q:\n%s", want, rr.Reason)
		}
	}
}

func TestCapReviewer_RejectReasonListsCountedPackages(t *testing.T) {
	ws := writeTriageWorkspace(t, overpackedArtifact)
	r := newGateDefectReviewer(config.StageEnforce, tightWindow, nil, nil)
	rr := r.Review(context.Background(), reviewIn(ws))
	if rr.Approve {
		t.Fatal("3 floors > cap 2 must reject at enforce")
	}
	if !strings.Contains(rr.Reason, "counted") {
		t.Errorf("reject reason must state the counted-package attribution; missing %q:\n%s", "counted", rr.Reason)
	}
	for _, pkg := range []string{"swarmrunner", "bridge", "evalgate"} {
		if !strings.Contains(rr.Reason, pkg) {
			t.Errorf("reject reason must list counted package %q:\n%s", pkg, rr.Reason)
		}
	}
}

// These summaries share every digit-run length, so ReasonTemplateHash collapses them.
const (
	gapSummary448 = `cycle 448 failed during triage: review gate: phase "triage" deliverable rejected after 2 correction(s): triage overpacked: 4 committed coverage floors exceed the capacity cap 3 (= ceil(1.25×K), K=2 observed floors/turn over 5 shipped cycles). Re-emit the triage report keeping at most 3 coverage floors in ## top_n and move the remaining floor work to ## deferred — deferred items carry over to the next cycle automatically.`
	gapSummary449 = `cycle 449 failed during triage: review gate: phase "triage" deliverable rejected after 2 correction(s): triage overpacked: 7 committed coverage floors exceed the capacity cap 3 (= ceil(1.25×K), K=2 observed floors/turn over 5 shipped cycles). Re-emit the triage report keeping at most 3 coverage floors in ## top_n and move the remaining floor work to ## deferred — deferred items carry over to the next cycle automatically.`
)

// writeGapWorkspace shares one project root across cycles so relief consumption is observable.
func writeGapWorkspace(t *testing.T, root string, cycle int) core.ReviewInput {
	t.Helper()
	ws := filepath.Join(root, ".evolve", "runs", fmt.Sprintf("cycle-%d", cycle))
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, TriageArtifactName()), []byte(overpackedArtifact), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "run.json"), []byte(fmt.Sprintf(`{"cycle_id":%d}`, cycle)), 0o644); err != nil {
		t.Fatal(err)
	}
	return core.ReviewInput{Phase: "triage", Workspace: ws, ProjectRoot: root}
}

func newGapRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".evolve", "inbox"), 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestCapReviewer_ResetSealedGapStillDemotes(t *testing.T) {
	root := newGapRoot(t)
	pair := []FailEntry{{Cycle: 448, Summary: gapSummary448}, {Cycle: 449, Summary: gapSummary449}}
	r := newGateDefectReviewer(config.StageEnforce, tightWindow, pair, nil)

	res := r.Review(context.Background(), writeGapWorkspace(t, root, 451))
	if !res.Approve {
		t.Fatalf("reset-sealed cycle 450 is a transparent gap — the 448/449 identical-template pair must demote cycle 451 to shadow, got reject: %s", res.Reason)
	}
	matches, _ := filepath.Glob(filepath.Join(root, ".evolve", "inbox", "auto-heuristic-demotion-*.json"))
	if len(matches) != 1 {
		t.Errorf("gap demotion must auto-file exactly one inbox defect, found %d", len(matches))
	}
}

func TestCapReviewer_ReliefIsOneCycleThenEnforces(t *testing.T) {
	root := newGapRoot(t)
	pair := []FailEntry{{Cycle: 448, Summary: gapSummary448}, {Cycle: 449, Summary: gapSummary449}}
	r := newGateDefectReviewer(config.StageEnforce, tightWindow, pair, nil)

	if res := r.Review(context.Background(), writeGapWorkspace(t, root, 451)); !res.Approve {
		t.Fatalf("cycle 451 must consume the pair's relief (shadow), got reject: %s", res.Reason)
	}
	if res := r.Review(context.Background(), writeGapWorkspace(t, root, 452)); res.Approve {
		t.Fatal("relief is one cycle: cycle 452 must enforce again against the same 448/449 history")
	}
}

func TestCapReviewer_StaleRejectionPairOutsideWindowEnforces(t *testing.T) {
	root := newGapRoot(t)
	stale := []FailEntry{{Cycle: 444, Summary: gapSummary448}, {Cycle: 445, Summary: gapSummary449}}
	r := newGateDefectReviewer(config.StageEnforce, tightWindow, stale, nil)

	if res := r.Review(context.Background(), writeGapWorkspace(t, root, 451)); res.Approve {
		t.Fatal("a stale same-template pair (444/445) outside the demotion window must not demote cycle 451")
	}
}

func TestCapReviewer_FloorBearingReportWithoutDeclarationWarns(t *testing.T) {
	floorArtifact := "## top_n (commit to THIS cycle)\n- coverage-one: Push bridge coverage to ≥98%\n"

	hasDeclarationWarn := func(logs []string) bool {
		for _, l := range logs {
			if strings.Contains(l, "triage-decision.json") && strings.Contains(l, "committed_floors") {
				return true
			}
		}
		return false
	}

	t.Run("warns when floor-bearing report lacks the companion", func(t *testing.T) {
		ws := writeTriageWorkspace(t, floorArtifact)
		var logs []string
		r := newGateDefectReviewer(config.StageEnforce, nil, nil, &logs)
		if rr := r.Review(context.Background(), reviewIn(ws)); !rr.Approve {
			t.Fatalf("1 floor under cap must approve: %s", rr.Reason)
		}
		if !hasDeclarationWarn(logs) {
			t.Errorf("floor-bearing report without triage-decision.json committed_floors must log a warning naming the companion and the field; logs:\n%s", strings.Join(logs, "\n"))
		}
	})

	t.Run("silent when the companion declares committed_floors", func(t *testing.T) {
		ws := writeTriageWorkspace(t, floorArtifact)
		if err := os.WriteFile(filepath.Join(ws, TriageDecisionName()), []byte(`{"committed_floors":["bridge"]}`), 0o644); err != nil {
			t.Fatal(err)
		}
		var logs []string
		r := newGateDefectReviewer(config.StageEnforce, nil, nil, &logs)
		if rr := r.Review(context.Background(), reviewIn(ws)); !rr.Approve {
			t.Fatalf("declared 1 floor under cap must approve: %s", rr.Reason)
		}
		if hasDeclarationWarn(logs) {
			t.Errorf("declaration present — the missing-declaration warning must not fire; logs:\n%s", strings.Join(logs, "\n"))
		}
	})

	t.Run("silent on a non-floor-bearing report", func(t *testing.T) {
		ws := writeTriageWorkspace(t, "## top_n (commit to THIS cycle)\n- fix-bug: Fix the dispatch worktree bug — priority=H\n")
		var logs []string
		r := newGateDefectReviewer(config.StageEnforce, nil, nil, &logs)
		if rr := r.Review(context.Background(), reviewIn(ws)); !rr.Approve {
			t.Fatalf("non-floor report must approve: %s", rr.Reason)
		}
		if hasDeclarationWarn(logs) {
			t.Errorf("no floors committed — the missing-declaration warning must not fire; logs:\n%s", strings.Join(logs, "\n"))
		}
	})
}
