package core

import (
	"context"
	"strconv"
	"strings"
	"testing"
)

func TestCorrectionLadder_EachReDispatchAsksForItsOwnNumberedSection(t *testing.T) {
	t.Parallel()
	rev := &sequencedReviewer{phase: "build", results: []ReviewResult{
		{Approve: false, Reason: "[missing_what_why] Changed Areas path lacks a what/why line"},
		{Approve: false, Reason: "[missing_section] build-report.md lacks ## Task"},
		{Approve: true},
	}}
	runners := buildRunners(nil)
	buildR := runners[PhaseBuild].(*fakeRunner)
	o := NewOrchestrator(&fakeStorage{state: State{LastCycleNumber: 0}}, &fakeLedger{}, runners, WithReviewer(rev))

	if _, err := o.RunCycle(context.Background(), CycleRequest{ProjectRoot: t.TempDir(), GoalHash: "g"}); err != nil {
		t.Fatalf("RunCycle should pass after two corrections; got %v", err)
	}
	if len(buildR.requests) != 3 {
		t.Fatalf("build dispatched %d times, want 3 (initial + 2 corrections)", len(buildR.requests))
	}
	if first := buildR.requests[0].CorrectionDirective; strings.Contains(first, "## Correction") {
		t.Errorf("the first dispatch is not a correction and must not ask for a correction section: %q", first)
	}
	for round, req := range buildR.requests[1:] {
		want := "append a `## Correction " + strconv.Itoa(round+1) + "` section to the deliverable"
		if !strings.Contains(req.CorrectionDirective, want) {
			t.Errorf("correction %d directive must ask for %q\n  got: %s", round+1, want, req.CorrectionDirective)
		}
	}
}
