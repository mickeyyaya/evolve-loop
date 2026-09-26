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

// CapReviewer is the triage capacity clamp; construct it with NewReviewer.
type CapReviewer struct {
	stage    config.Stage
	logf     func(format string, args ...any)
	pkgsFn   func(projectRoot string) []string
	windowFn func(projectRoot string) []core.TriageThroughputEntry
	failsFn  func(projectRoot string) []FailEntry
}

// NewReviewer builds the capacity clamp for a stage; it approves every non-triage phase.
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

// readWindow skips the project lock: the orchestrator's lock prevents concurrent runs and WriteState renames
// atomically. A read failure yields an empty window, which K turns into the seed.
func readWindow(projectRoot string) []core.TriageThroughputEntry {
	st, err := storage.New(filepath.Join(projectRoot, ".evolve")).ReadState(context.Background())
	if err != nil {
		return nil
	}
	return st.TriageThroughput
}

// Review rejects a triage deliverable whose committed floors exceed Cap(K) at enforce; it only logs at shadow.
func (r *CapReviewer) Review(_ context.Context, in core.ReviewInput) core.ReviewResult {
	if r.stage == config.StageOff || in.Phase != string(core.PhaseTriage) {
		return core.ReviewResult{Approve: true}
	}
	data, err := os.ReadFile(filepath.Join(in.Workspace, TriageArtifactName()))
	if err != nil {
		// Fail open: the contract gate owns presence checks.
		r.logf("[triage-cap] ambiguity, failing open: %v", err)
		return core.ReviewResult{Approve: true}
	}
	// Footprint provenance only warns: refusing a deliverable over it would turn an overlap risk into a failed cycle.
	pkgs := r.pkgsFn(in.ProjectRoot)
	companionPath := filepath.Join(in.Workspace, TriageDecisionName())
	if msg := MissingCardFilesWarning(string(data), companionPath); msg != "" {
		r.logf("[triage-cap] WARN: %s", msg)
	}
	floors := CommittedFloorCount(string(data), companionPath, pkgs)
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
	// The reason states the rule, the counted packages and the declaration escape; an agent cannot comply with an unseen rule.
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
	// Demotion is consulted only at rejection time, so the approve path never reads state.json.
	if cycle, ok := workspaceCycleID(in.Workspace); ok {
		if older, newer, why, demote := demotionDecision(r.failsFn(in.ProjectRoot), cycle); demote {
			// Relief another cycle already consumed keeps enforcing, so gap transparency never becomes free passes.
			if by, consumed := reliefConsumedBy(in.ProjectRoot, older, newer); consumed && by != cycle {
				r.logf("[triage-cap] demotion relief for the c%d/c%d pair already consumed by cycle %d — enforcing (one-cycle relief)", older, newer, by)
			} else {
				r.logf("[triage-cap] DEMOTED to shadow for cycle %d — %s; gate defect suspected (ADR-0046 L2). Would-block: %s", cycle, why, reason)
				autoFileDemotionDefect(in.ProjectRoot, cycle, older, newer, why, RemedyPending)
				return core.ReviewResult{Approve: true}
			}
		}
	}
	r.logf("[triage-cap] %s (stage=enforce, BLOCK)", reason)
	return core.ReviewResult{Approve: false, Reason: reason}
}
