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

// gate_defect_chain_test.go — cycle-459 TDD contract for the triage-cap gate
// defect chain F1–F5 (inbox triagecap-prose-counter-defect; post-mortem of
// cycles 448/449, which both died at this gate after complying with its
// correction twice). These tests are authored RED-FIRST by the TDD engineer;
// the Builder makes them GREEN without modifying them.
//
//   F1 — the prose counter counts packages named in EVIDENCE citations as
//        committed floors (cycle 449: 3 true floors counted as 7).
//   F2 — the reject reason states neither the counting rule nor the
//        declaration-primary escape (triage-decision.json committed_floors[]).
//   F3 — nothing checks that a floor-bearing report carries the declaration
//        companion (the declaration-primary design has no producer check).
//   F4 — ShouldDemote requires rejections at currentCycle-1 AND -2, so an
//        operator reset that seals a cycle without a rejection record breaks
//        the identical-rejection demotion chain (cycle 450 reset ⇒ 451/452
//        cannot demote).
//   F5 — the reject reason shows the count but not WHICH packages were
//        counted, so corrections are not self-explanatory.
//
// The goldens are the EXACT preserved cycle-448/449 triage artifacts
// (testdata/triage-cycle44{8,9}-golden.md, byte-identical to
// .evolve/runs/cycle-44{8,9}/triage-report.md).

// gapGoldenPkgs mirrors the production package vocabulary relevant to the
// cycle-448/449 goldens: the three true floor targets (core, bridge, audit)
// plus the phantom sources the defective counter attributed (scout via kept
// evidence= values, sysexec via "sysexec.RunFunc" in item prose) and
// distractors that must never count. With this vocabulary the unfixed
// counter yields 7 on the cycle-449 golden — the exact incident count.
var gapGoldenPkgs = []string{
	"core", "bridge", "audit",
	"scout", "sysexec",
	"config", "router", "llmroute", "recovery", "evidence", "paths",
}

// TestCountCommittedFloors_Cycle449GoldenReplay (F1, golden) — the normative
// acceptance of the fix: cycle 449 committed exactly three coverage floors
// (core 85.0 / bridge 94.5 / audit 96.0, one per top_n item) and was killed
// because evidence citations ("core 83.1%", "bridge 93.5%; matchExhausted
// 66.7%", "audit 92.6%", "scout fresh cover-func", "sysexec.RunFunc")
// inflated the count to 7. The EXACT report must count 3 after the fix.
func TestCountCommittedFloors_Cycle449GoldenReplay(t *testing.T) {
	artifact := readFixture(t, "triage-cycle449-golden.md")
	got := CountCommittedFloors(artifact, gapGoldenPkgs)
	if got != 3 {
		t.Errorf("cycle-449 golden committed floors = %d, want 3 (core/bridge/audit floor targets; evidence citations must not count)", got)
	}
}

// TestCountCommittedFloors_Cycle448GoldenReplay (F1, anti-weakening golden) —
// cycle 448 genuinely committed FOUR floor targets (item 1: "coverage floors
// core ≥85.0%, audit ≥96.0%, bridge ≥94.5%" = 3; item 2: "core total
// coverage ... to ≥86.0%" = 1). The fix must scope out evidence citations
// WITHOUT collapsing true multi-package floor commitments: this golden counts
// 4 before AND after the fix. The gate's purpose (cycles 280/282/283 real
// overpacking) stays intact.
func TestCountCommittedFloors_Cycle448GoldenReplay(t *testing.T) {
	artifact := readFixture(t, "triage-cycle448-golden.md")
	got := CountCommittedFloors(artifact, gapGoldenPkgs)
	if got != 4 {
		t.Errorf("cycle-448 golden committed floors = %d, want 4 (core+audit+bridge targets in item 1, core target in item 2)", got)
	}
}

// TestCountCommittedFloors_EvidenceCitationDoesNotCount (F1, semantic) — a
// single-target floor item whose contract-mandated evidence sentence names
// OTHER packages with percentages counts 1, not 4. The pipeline's own
// eval-quality rules REQUIRE rich numeric evidence; the gate must not punish
// what another gate demands.
func TestCountCommittedFloors_EvidenceCitationDoesNotCount(t *testing.T) {
	artifact := "## top_n (commit to THIS cycle)\n" +
		"- salvage-core-coverage: raise core coverage floor to ≥85.0% — priority=H, evidence=scout fresh cover-func (bridge 93.5%; matchExhausted 66.7% in audit), source=scout\n"
	got := CountCommittedFloors(artifact, gapGoldenPkgs)
	if got != 1 {
		t.Errorf("evidence-citation item floors = %d, want 1 (core is the only floor TARGET; bridge/audit/scout are evidence prose)", got)
	}
}

// newGateDefectReviewer wires the clamp with seam overrides and a FORMATTED
// log capture (newTestReviewer captures only format strings, which would
// make warning-content assertions depend on how the implementation splits
// format vs args).
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

// TestCapReviewer_RejectReasonStatesDeclarationEscape (F2) — the corrective
// must be actionable: an agent that cannot see the counting rule cannot
// comply with it (cycles 448/449 complied with the natural reading twice and
// were killed twice). The reject reason must name the declaration-primary
// escape: emit triage-decision.json with committed_floors[].
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

// TestCapReviewer_RejectReasonListsCountedPackages (F5) — observability: the
// reason must name WHICH packages the counter attributed, so a correction is
// self-explanatory ("counted: bridge, evalgate, swarmrunner") and a counter
// defect is visible in the rejection itself instead of requiring a
// post-mortem against the preserved artifacts.
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

// Same-template rejection summaries for cycles 448/449, shaped like the real
// state.json:failedApproaches records (same digit-run lengths throughout, so
// ReasonTemplateHash collapses them — the incident's determinism artifact).
const (
	gapSummary448 = `cycle 448 failed during triage: review gate: phase "triage" deliverable rejected after 2 correction(s): triage overpacked: 4 committed coverage floors exceed the capacity cap 3 (= ceil(1.25×K), K=2 observed floors/turn over 5 shipped cycles). Re-emit the triage report keeping at most 3 coverage floors in ## top_n and move the remaining floor work to ## deferred — deferred items carry over to the next cycle automatically.`
	gapSummary449 = `cycle 449 failed during triage: review gate: phase "triage" deliverable rejected after 2 correction(s): triage overpacked: 7 committed coverage floors exceed the capacity cap 3 (= ceil(1.25×K), K=2 observed floors/turn over 5 shipped cycles). Re-emit the triage report keeping at most 3 coverage floors in ## top_n and move the remaining floor work to ## deferred — deferred items carry over to the next cycle automatically.`
)

// writeGapWorkspace builds one cycle's review workspace under a SHARED
// project root (unlike newDemotionFixture, which owns its root) so relief
// consumption can be observed across consecutive cycles.
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

// TestCapReviewer_ResetSealedGapStillDemotes (F4) — the incident replay:
// cycles 448 and 449 recorded same-template rejections from this gate; an
// operator SIGINT + `cycle reset --force` sealed cycle 450 WITHOUT a
// rejection record. Reviewing cycle 451, the last two RECORDED same-template
// rejections (448, 449) are within the demotion window, so the gate must
// treat the reset-sealed 450 as a transparent gap: demote to shadow
// (approve) and auto-file exactly one defect. Today ShouldDemote demands
// records at 450 AND 449, so 451 enforces and burns — RED.
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

// TestCapReviewer_ReliefIsOneCycleThenEnforces (F4, bounded relief) — the
// demotion's one-cycle relief semantics survive the gap fix: after cycle 451
// consumes the 448/449 pair's relief (shadow + auto-filed defect), cycle 452
// with the SAME history must enforce again. This pins against the naive
// pure-window implementation, under which the pair would keep granting
// relief to every cycle in the window.
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

// TestCapReviewer_StaleRejectionPairOutsideWindowEnforces (F4, negative) — a
// same-template pair far in the past grants no relief: the demotion window
// is small (the fix's own spec suggests ~3 cycles). A 444/445 pair reviewed
// at cycle 451 must keep enforcing. GREEN today and must stay GREEN — this
// is the anti-overcorrection bound on the gap transparency.
func TestCapReviewer_StaleRejectionPairOutsideWindowEnforces(t *testing.T) {
	root := newGapRoot(t)
	stale := []FailEntry{{Cycle: 444, Summary: gapSummary448}, {Cycle: 445, Summary: gapSummary449}}
	r := newGateDefectReviewer(config.StageEnforce, tightWindow, stale, nil)

	if res := r.Review(context.Background(), writeGapWorkspace(t, root, 451)); res.Approve {
		t.Fatal("a stale same-template pair (444/445) outside the demotion window must not demote cycle 451")
	}
}

// TestCapReviewer_FloorBearingReportWithoutDeclarationWarns (F3) — the
// declaration-primary design finally gets a producer check: reviewing a
// floor-bearing triage report that lacks a committed_floors declaration must
// WARN (log naming triage-decision.json + committed_floors) while still
// approving an under-cap report. Non-floor-bearing reports and reports that
// carry the declaration stay silent — the warning must not become noise.
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
