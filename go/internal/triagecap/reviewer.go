package triagecap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/storage"
	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// reviewer.go — the R9.2 capacity clamp at the orchestrator's per-phase
// deliverable-review seam (chained with the evalgate + contract gates via
// core.ChainReviewers). A reject here enters the existing correction ladder:
// triage is re-dispatched with the cap directive injected as a
// "## Correction" block, so the agent re-shapes top_n instead of the cycle
// burning on an overpacked commitment (inbox coverage-floor-overpacking —
// cycles 280/282/283 all failed on floors the builder demonstrably cannot
// clear in one turn).
//
// Posture (matches the contract gate):
//   - Only the triage phase is in scope; everything else is approved.
//   - Ambiguity (missing/unreadable artifact) → fail OPEN.
//   - StageShadow → log-only.
//   - StageEnforce → reject with an actionable cap directive. The ladder
//     bounds retries, so a miscalibrated clamp costs corrections, not a
//     bricked loop.

// CapReviewer is the triage capacity clamp. Construct with NewReviewer;
// tests override pkgsFn/windowFn/failsFn/logf directly.
type CapReviewer struct {
	stage    config.Stage
	logf     func(format string, args ...any)
	pkgsFn   func(projectRoot string) []string
	windowFn func(projectRoot string) []core.TriageThroughputEntry
	failsFn  func(projectRoot string) []FailEntry
}

// NewReviewer builds the capacity clamp for a stage. Callers wire it via
// core.WithReviewer (chained after the eval + contract gates) only when
// stage != StageOff.
func NewReviewer(stage config.Stage) core.DeliverableReviewer {
	return newCapReviewer(stage)
}

func newCapReviewer(stage config.Stage) *CapReviewer {
	return &CapReviewer{
		stage:    stage,
		logf:     func(f string, a ...any) { fmt.Fprintf(os.Stderr, f+"\n", a...) },
		pkgsFn:   KnownPackages,
		windowFn: readWindow,
		failsFn:  readFailedApproaches,
	}
}

// readWindow loads the rolling throughput window from state.json. The read
// does NOT acquire the project lock — safe because the orchestrator's own
// lock prevents concurrent runs, and WriteState's atomic rename prevents
// torn reads; we deliberately see the window as of the last completed
// cycle. Any read failure yields an empty window — i.e. the cycle-281 seed.
func readWindow(projectRoot string) []core.TriageThroughputEntry {
	st, err := storage.New(filepath.Join(projectRoot, ".evolve")).ReadState(context.Background())
	if err != nil {
		return nil
	}
	return st.TriageThroughput
}

// Review adjudicates one finished triage deliverable against observed
// builder throughput.
func (r *CapReviewer) Review(_ context.Context, in core.ReviewInput) core.ReviewResult {
	if r.stage == config.StageOff || in.Phase != string(core.PhaseTriage) {
		return core.ReviewResult{Approve: true}
	}
	data, err := os.ReadFile(filepath.Join(in.Workspace, TriageArtifactName()))
	if err != nil {
		// Ambiguity / infra — fail OPEN (the contract gate owns presence checks).
		r.logf("[triage-cap] ambiguity, failing open: %v", err)
		return core.ReviewResult{Approve: true}
	}
	pkgs := r.pkgsFn(in.ProjectRoot)
	companionPath := filepath.Join(in.Workspace, TriageDecisionName())
	floors := CommittedFloorCount(string(data), companionPath, pkgs)
	// F3 producer check: the declaration-primary design needs its producer.
	// A floor-bearing report still governed by the prose fallback gets a
	// WARN naming the companion, so a missing declaration is visible in the
	// logs instead of silently leaving the count to prose semantics.
	if floors > 0 {
		if _, declared, err := ReadDeclaredFloors(companionPath); err == nil && !declared {
			r.logf("[triage-cap] WARN: floor-bearing triage report has no %s companion declaring committed_floors — prose counting is the fallback; declare {\"committed_floors\":[...]} to make the commitment authoritative", TriageDecisionName())
		}
	}
	window := r.windowFn(in.ProjectRoot)
	k := K(window)
	capacity := Cap(k)
	if floors <= capacity {
		return core.ReviewResult{Approve: true}
	}

	corrective := FloorDivergenceCorrective(string(data), companionPath, pkgs)
	// F2+F5: the correction must be actionable and self-explanatory — state
	// the counting rule, WHICH packages the counter attributed, and the
	// declaration-primary escape. Cycles 448/449 complied with the natural
	// reading of the bare cap directive twice and were killed twice because
	// they could not see either the rule or the escape.
	counted := CommittedFloorPackages(string(data), companionPath, pkgs)
	countedList := "none package-resolved (aggregate items count 1 each)"
	if len(counted) > 0 {
		countedList = strings.Join(counted, ", ")
	}
	reason := fmt.Sprintf(
		"triage overpacked: %d committed coverage floors exceed the capacity cap %d (= ceil(1.25×K), K=%d observed floors/turn over %d shipped cycles). Counting rule: each floor-bearing ## top_n item counts one floor per distinct package in floor-TARGET position (named before its ≥N%% target), minimum one; packages counted: %s. Preferred fix: declare the true commitment by writing %s with {\"committed_floors\":[...]} beside the report — the declaration overrides prose counting. Otherwise re-emit the triage report keeping at most %d coverage floors in ## top_n and move the remaining floor work to ## deferred — deferred items carry over to the next cycle automatically.",
		floors, capacity, k, len(window), countedList, TriageDecisionName(), capacity)
	if corrective != "" {
		reason += " " + corrective
	}
	if r.stage != config.StageEnforce {
		r.logf("[triage-cap] %s (stage=%s, would-block)", reason, r.stage)
		return core.ReviewResult{Approve: true}
	}
	// ADR-0046 Layer 2: before enforcing, consult the identical-rejection
	// demotion (demotion.go). The last two RECORDED cycles rejected with
	// this exact template (reset-sealed cycles are transparent gaps, F4) ⇒
	// the gate is the suspect ⇒ shadow for ONE cycle, with an auto-filed
	// defect. Consulted at rejection time so a healthy approve path never
	// pays the state.json read.
	if cycle, ok := workspaceCycleID(in.Workspace); ok {
		if older, newer, why, demote := demotionDecision(r.failsFn(in.ProjectRoot), cycle); demote {
			// One-cycle relief: the pair's auto-filed defect doubles as the
			// relief-consumption marker. A pair whose relief another cycle
			// already consumed keeps enforcing — gap transparency must not
			// widen into a window of free passes.
			if by, consumed := reliefConsumedBy(in.ProjectRoot, older, newer); consumed && by != cycle {
				r.logf("[triage-cap] demotion relief for the c%d/c%d pair already consumed by cycle %d — enforcing (one-cycle relief)", older, newer, by)
			} else {
				r.logf("[triage-cap] DEMOTED to shadow for cycle %d — %s; gate defect suspected (ADR-0046 L2). Would-block: %s", cycle, why, reason)
				autoFileDemotionDefect(in.ProjectRoot, cycle, older, newer, why)
				return core.ReviewResult{Approve: true}
			}
		}
	}
	r.logf("[triage-cap] %s (stage=enforce, BLOCK)", reason)
	return core.ReviewResult{Approve: false, Reason: reason}
}
