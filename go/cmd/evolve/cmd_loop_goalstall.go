package main

import (
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// cmd_loop_goalstall.go — goal-stall escalation for the sequential loop.
//
// When a goal keeps producing EMPTY (nothing shipped, no signal —
// CycleOutcomeSkippedUnknown) or BLOCKED (audit would-have-blocked —
// CycleOutcomeSkippedAuditAdvisory) cycles, the scheduler re-dispatched the
// identical goal forever: goal_hash 805f6ced burned 4 full pipelines in one
// batch (cycles 640/642/643/644) landing nothing, because nothing COUNTED the
// non-progress at the goal layer (the consecutive-FAIL breaker misses it — an
// empty/blocked cycle is not a FAIL). This adds that counter: after N
// consecutive non-shipping cycles on one goal, the loop self-files a weighted
// inbox todo naming the stalled goal + the block reasons and emits an
// abnormal-event, instead of blindly re-running it. The counter resets on any
// shipping cycle.
//
// Held in-memory for the loop invocation, mirroring consecutiveFailBreaker
// (cmd_loop_control.go) and fleet.StarvationTracker — a stall burns WITHIN one
// batch, so per-invocation tracking catches the live case; cross-process
// durability would need a dossier schema migration (goal_hash + raw outcome
// preservation) for marginal benefit and is deliberately out of scope.

// goalStallWeightFloor mirrors the policy-layer floor (policy.GoalStallWeight):
// a stalled goal is a high-priority self-prioritization signal, never a
// low-weight afterthought. Guarded at both layers so neither a bad config nor a
// bad caller can under-weight it.
const goalStallWeightFloor = 0.9

// goalStallItemIDPrefix + a short goal-hash forms the STABLE inbox id, so a
// re-fire for the same goal overwrites the single open todo rather than piling
// up duplicates.
const goalStallItemIDPrefix = "goal-stall-"

// goalStallTracker counts consecutive non-shipping cycles for the running goal
// and collects the distinct block reasons seen during the streak.
type goalStallTracker struct {
	streak  int
	reasons []string
}

// goalStallEscalation is the data captured at the fire point (before reset), so
// the caller can build the inbox todo naming the streak length and reasons.
type goalStallEscalation struct {
	streak  int
	reasons []string
}

// observe folds one cycle outcome into the streak and returns a non-nil
// escalation exactly on the threshold-th CONSECUTIVE non-shipping cycle (then
// resets, so the next escalation needs a fresh `threshold` streak). A shipping
// cycle resets the streak and clears the reasons. reason is the cycle's
// non-shipping outcome (recorded, deduped); "" is ignored. threshold<1 ⇒ 1.
func (t *goalStallTracker) observe(nonShipping bool, reason string, threshold int) *goalStallEscalation {
	if threshold < 1 {
		threshold = 1
	}
	if !nonShipping {
		t.streak = 0
		t.reasons = nil
		return nil
	}
	t.streak++
	if reason != "" && !slices.Contains(t.reasons, reason) {
		t.reasons = append(t.reasons, reason)
	}
	if t.streak >= threshold {
		esc := &goalStallEscalation{streak: t.streak, reasons: append([]string(nil), t.reasons...)}
		t.streak = 0
		t.reasons = nil
		return esc
	}
	return nil
}

// goalStallItem is a self-filed weighted inbox todo naming a stalled goal. Its
// on-disk JSON matches the canonical inbox-item schema (id/title/weight/kind/
// priority) plus the human-facing fields a scout reads.
type goalStallItem struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Weight      float64 `json:"weight"`
	Kind        string  `json:"kind"`
	Priority    string  `json:"priority"`
	Campaign    string  `json:"campaign"`
	Description string  `json:"description"`
	Source      string  `json:"source"`
	CreatedAt   string  `json:"created_at"`
}

// buildGoalStallItem constructs the weighted inbox todo for a goal that ran
// `esc.streak` consecutive non-shipping cycles. weight is clamped UP to the 0.9
// floor. The id is goal-stable (independent of cycle) so re-fires stay
// idempotent by goal.
func buildGoalStallItem(goalHash string, esc *goalStallEscalation, weight float64, cycle int, nowRFC3339 string) goalStallItem {
	if weight < goalStallWeightFloor {
		weight = goalStallWeightFloor
	}
	reasons := "none recorded"
	if len(esc.reasons) > 0 {
		reasons = strings.Join(esc.reasons, "; ")
	}
	short := shortGoalHash(goalHash)
	return goalStallItem{
		ID:       goalStallItemIDPrefix + short,
		Title:    fmt.Sprintf("Goal %s stalled: %d consecutive empty/blocked cycles shipped nothing — re-scope, split, or unblock", short, esc.streak),
		Weight:   weight,
		Kind:     "bug",
		Priority: "high",
		Campaign: "pipeline-stability",
		Description: fmt.Sprintf(
			"The goal (hash %s) produced %d CONSECUTIVE empty/blocked cycles landing nothing "+
				"(observed through cycle %d). The scheduler stopped blind re-dispatch and filed "+
				"this instead of re-running the identical goal again. Re-scope to a narrower "+
				"reachable slice, split the goal, or address the recurring block reason(s): %s.",
			goalHash, esc.streak, cycle, reasons),
		Source:    "goal-stall-escalation",
		CreatedAt: nowRFC3339,
	}
}

// validate rejects an under-weighted or incompletely-populated item so a
// malformed self-injection fails loud rather than seeding a silent no-op todo.
func (it goalStallItem) validate() error {
	if it.Weight < goalStallWeightFloor {
		return fmt.Errorf("goalstall: item weight %v below floor %v", it.Weight, goalStallWeightFloor)
	}
	for _, f := range []struct{ name, val string }{
		{"id", it.ID}, {"title", it.Title}, {"kind", it.Kind},
		{"description", it.Description}, {"source", it.Source}, {"created_at", it.CreatedAt},
	} {
		if f.val == "" {
			return fmt.Errorf("goalstall: item missing required field %q", f.name)
		}
	}
	return nil
}

// writeTo validates then atomically writes the item to <evolveDir>/inbox/<id>.json
// and returns the written path. Idempotent by id: the goal-stable filename means a
// second fire for the same goal overwrites the single open todo.
func (it goalStallItem) writeTo(evolveDir string) (string, error) {
	if err := it.validate(); err != nil {
		return "", err
	}
	path := filepath.Join(evolveDir, "inbox", it.ID+".json")
	if err := atomicwrite.JSON(path, it); err != nil {
		return "", fmt.Errorf("goalstall: write item: %w", err)
	}
	return path, nil
}

// shortGoalHash returns the first 8 chars of a goal hash (or the whole thing if
// shorter) — enough to disambiguate goals in an id/title without the full 64.
func shortGoalHash(goalHash string) string {
	if len(goalHash) <= 8 {
		return goalHash
	}
	return goalHash[:8]
}

// loadGoalStallConfig reads the goal-stall threshold + weight from policy.json,
// falling back to the compiled defaults on any read error (sourced from policy,
// never a Go literal at the call site — feedback_phase_settings_from_config_not_code).
func loadGoalStallConfig(evolveDir string) (int, float64) {
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		return policy.Policy{}.GoalStallThreshold(), policy.Policy{}.GoalStallWeight()
	}
	return pol.GoalStallThreshold(), pol.GoalStallWeight()
}

// handleGoalStall self-files the goal-stall inbox todo and emits an
// abnormal-event for the observer. Failures are logged, never fatal — a
// diagnostic escalation must not itself halt the loop.
func handleGoalStall(evolveDir, goalHash, workspace string, cycle int, esc *goalStallEscalation, threshold int, weight float64, stderr io.Writer) {
	item := buildGoalStallItem(goalHash, esc, weight, cycle, time.Now().UTC().Format(time.RFC3339))
	if path, err := item.writeTo(evolveDir); err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: goal-stall: failed to file inbox todo: %v\n", err)
	} else {
		fmt.Fprintf(stderr, "[loop] GOAL-STALL: goal %s ran %d consecutive empty/blocked cycles — filed %s; re-scope/split instead of re-dispatching\n", shortGoalHash(goalHash), esc.streak, path)
	}
	if dirExists(workspace) {
		w := dispatchevents.NewWriter(workspace)
		_ = w.EmitGoalStallEscalated(cycle, esc.streak, threshold, goalHash)
	}
}
