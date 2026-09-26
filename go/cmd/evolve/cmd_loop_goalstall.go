package main

import (
	"fmt"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/dispatchevents"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// goalStallWeightFloor mirrors policy.GoalStallWeight's floor; guarded at both
// layers so neither a bad config nor a bad caller can under-weight a stalled
// goal.
const goalStallWeightFloor = 0.9

const goalStallItemIDPrefix = "goal-stall-"

// nonprogressItemIDPrefix stays distinct from goalStallItemIDPrefix so a
// non-progress escalation cannot overwrite the empty-only goal-stall todo for
// the same goal.
const nonprogressItemIDPrefix = "nonprogress-"

// stallKind is per-breaker data; the two breakers share all other machinery so
// the difference stays data, not a duplicate
// (feedback_never_duplicate_centralize_via_design_patterns).
type stallKind struct {
	idPrefix string
	// outcomes names the counted class in the title/description/event; reporting
	// a FAIL/EMPTY streak as empty-only would send the scout after the wrong
	// evidence, so each kind states its own.
	outcomes string
	source   string
	marker   string
}

var (
	goalStallKind = stallKind{
		idPrefix: goalStallItemIDPrefix,
		outcomes: "empty/blocked",
		source:   "goal-stall-escalation",
		marker:   "GOAL-STALL",
	}
	nonprogressKind = stallKind{
		idPrefix: nonprogressItemIDPrefix,
		outcomes: "non-shipping (fail/empty/blocked). NOTE: a WARN-verdict cycle may have shipped inline — CycleResult carries no HEAD-moved field, so confirm HEAD before treating this streak as true non-progress",
		source:   "nonprogress-escalation",
		marker:   "NONPROGRESS",
	}
)

func nonShippingOutcome(finalVerdict string) bool {
	// A WARN-verdict cycle that shipped inline still counts as non-progress
	// here, since CycleResult carries no HEAD-moved field; that can file one
	// spurious diagnostic todo but never halts.
	return !core.IsShippingVerdict(finalVerdict)
}

type goalStallTracker struct {
	streak  int
	reasons []string
}

type goalStallEscalation struct {
	streak  int
	reasons []string
}

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

// goalStallItem's JSON matches the canonical inbox-item schema, plus the
// human-facing fields a scout reads.
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

func buildGoalStallItem(kind stallKind, goalHash string, esc *goalStallEscalation, weight float64, cycle int, nowRFC3339 string) goalStallItem {
	if weight < goalStallWeightFloor {
		weight = goalStallWeightFloor
	}
	reasons := "none recorded"
	if len(esc.reasons) > 0 {
		reasons = strings.Join(esc.reasons, "; ")
	}
	short := shortGoalHash(goalHash)
	return goalStallItem{
		ID:       kind.idPrefix + short,
		Title:    fmt.Sprintf("Goal %s stalled: %d consecutive %s cycles shipped nothing — re-scope, split, or unblock", short, esc.streak, kind.outcomes),
		Weight:   weight,
		Kind:     "bug",
		Priority: "high",
		Campaign: "pipeline-stability",
		Description: fmt.Sprintf(
			"The goal (hash %s) produced %d CONSECUTIVE %s cycles landing nothing "+
				"(observed through cycle %d). The scheduler stopped blind re-dispatch and filed "+
				"this instead of re-running the identical goal again. Re-scope to a narrower "+
				"reachable slice, split the goal, or address the recurring block reason(s): %s.",
			goalHash, esc.streak, kind.outcomes, cycle, reasons),
		Source:    kind.source,
		CreatedAt: nowRFC3339,
	}
}

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

func shortGoalHash(goalHash string) string {
	if len(goalHash) <= 8 {
		return goalHash
	}
	return goalHash[:8]
}

// goalStallConfig groups both breakers' policy-sourced knobs so the loop's
// single load site cannot pick up one threshold and miss the other.
type goalStallConfig struct {
	threshold            int // empty/blocked-only ceiling
	nonprogressThreshold int // union (any non-shipping outcome) ceiling
	weight               float64
}

// loadGoalStallConfig falls back to the compiled defaults on any policy.json
// read error (feedback_phase_settings_from_config_not_code).
func loadGoalStallConfig(evolveDir string) goalStallConfig {
	pol, err := policy.Load(filepath.Join(evolveDir, "policy.json"))
	if err != nil {
		pol = policy.Policy{}
	}
	return goalStallConfig{
		threshold:            pol.GoalStallThreshold(),
		nonprogressThreshold: pol.GoalStallNonprogressThreshold(),
		weight:               pol.GoalStallWeight(),
	}
}

// handleGoalStall never fails the loop: escalation errors are logged, not
// fatal (never_stop_queue_inject_inbox).
func handleGoalStall(kind stallKind, evolveDir, goalHash, workspace string, cycle int, esc *goalStallEscalation, threshold int, weight float64, stderr io.Writer) {
	item := buildGoalStallItem(kind, goalHash, esc, weight, cycle, time.Now().UTC().Format(time.RFC3339))
	if path, err := item.writeTo(evolveDir); err != nil {
		fmt.Fprintf(stderr, "[loop] WARN: %s: failed to file inbox todo: %v\n", kind.source, err)
	} else {
		fmt.Fprintf(stderr, "[loop] %s: goal %s ran %d consecutive %s cycles — filed %s; re-scope/split instead of re-dispatching\n",
			kind.marker, shortGoalHash(goalHash), esc.streak, kind.outcomes, path)
	}
	if dirExists(workspace) {
		w := dispatchevents.NewWriter(workspace)
		_ = w.EmitGoalStallEscalated(cycle, esc.streak, threshold, goalHash, kind.outcomes)
	}
}
