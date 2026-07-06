// starvation.go — the L3 leg of the fleet-concurrency-respect architecture:
// detect when the fleet keeps realizing fewer lanes than the operator asked for
// because the WORK SUPPLY (triage plan / inbox backlog) ran dry, not because a
// quota/capacity shrink benched a CLI family. After K consecutive such waves the
// loop self-files one weighted inbox todo naming the cause, so the next batch
// widens the plan instead of silently running under-utilized forever.
//
// This lands INSIDE internal/fleet (already go/.apicover-enforce:124) rather than
// a new leaf package — cycle 542 built the identical logic as a new
// internal/fleethealth package and the ship-gate correctly FAILed it
// (TestApicoverEnforce_CoversEveryInternalPackage: the package was never added to
// the enforce list). Extending an already-enforced package sidesteps that
// completeness gate entirely.
package fleet

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

// starvationWeightFloor is the minimum weight a self-filed starvation todo may
// carry — it is a high-priority self-prioritization signal, never a low-weight
// afterthought. Mirrors the policy-layer floor (policy.FleetConfig clamps the
// resolved StarvationWeight up to the same 0.9); each layer guards it
// independently so neither a bad config nor a bad caller can under-weight it.
const starvationWeightFloor = 0.9

// starvationItemID is the STABLE id (and inbox filename stem) for the fleet
// work-supply-starvation todo. Cause-stable (independent of cycle) so a re-fire
// overwrites the single open todo rather than piling up duplicates.
const starvationItemID = "fleet-work-supply-starvation"

// WaveObservation is one wave's realized-vs-desired lane count.
type WaveObservation struct {
	// DesiredLanes is fleetCfg.Count — the operator-asserted concurrency for the
	// wave (NOT the quota-shrunk waveCfg.Count).
	DesiredLanes int
	// RealizedLanes is how many lanes actually dispatched work this wave.
	RealizedLanes int
	// QuotaShrunk is true iff the wave's capacity was reduced by the quota-aware
	// shrink (waveCfg.Count < fleetCfg.Count) — a capacity shrink, never
	// work-supply starvation.
	QuotaShrunk bool
}

// Starved reports work-supply starvation: fewer lanes realized than desired,
// for a reason OTHER than a quota/capacity shrink. A quota-shrunk under-utilized
// wave is never starvation, no matter how many consecutive such waves occur.
func (o WaveObservation) Starved() bool {
	return o.RealizedLanes < o.DesiredLanes && !o.QuotaShrunk
}

// StarvationTracker counts consecutive work-supply-starved waves. It is held
// OUTSIDE the per-wave loop so the streak spans waves; Observe advances it only
// on the wave-ran path (a single-lane / sequential iteration never calls it).
type StarvationTracker struct {
	streak int
}

// Streak returns the current consecutive-starved-wave count.
func (t *StarvationTracker) Streak() int { return t.streak }

// Observe folds one wave observation into the streak and reports whether this
// wave is the k-th consecutive starved wave (the fire point). A non-starved
// (recovered) wave resets the streak to 0 and returns false. Firing also resets
// the streak to 0, so the NEXT fire needs a fresh k, not k+1. k<1 ⇒ k=1.
func (t *StarvationTracker) Observe(o WaveObservation, k int) bool {
	if k < 1 {
		k = 1
	}
	if !o.Starved() {
		t.streak = 0
		return false
	}
	t.streak++
	if t.streak >= k {
		t.streak = 0
		return true
	}
	return false
}

// StarvationItem is a self-filed weighted inbox todo naming a fleet
// work-supply-starvation cause. Its on-disk JSON is a superset of the fleet
// inbox-candidate schema (id/weight/files, read by triagecap.ReadInboxBacklog)
// plus the human-facing description fields a scout reads.
type StarvationItem struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Weight      float64  `json:"weight"`
	Kind        string   `json:"kind"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
	Source      string   `json:"source"`
	CreatedAt   string   `json:"created_at"`
}

// BuildStarvationItem constructs the weighted inbox todo for a fleet that ran k
// consecutive work-supply-starved waves. weight is clamped UP to the 0.9 floor
// (never silently under-weighted). cycle and nowRFC3339 are recorded for
// provenance; the id is cause-stable (independent of cycle) so re-fires stay
// idempotent by cause.
func BuildStarvationItem(o WaveObservation, k int, weight float64, cycle int, nowRFC3339 string) StarvationItem {
	if weight < starvationWeightFloor {
		weight = starvationWeightFloor
	}
	return StarvationItem{
		ID:     starvationItemID,
		Title:  "Fleet lanes work-supply-starved: realized concurrency < configured",
		Weight: weight,
		Kind:   "feature",
		Description: fmt.Sprintf(
			"The fleet ran %d consecutive waves realizing only %d of %d configured lanes "+
				"for a reason other than a quota/capacity shrink — work-supply starvation, "+
				"not a benched CLI family. Widen the triage plan / inbox backlog so every "+
				"configured lane has file-disjoint work to dispatch, per the "+
				"fleet-concurrency-respect architecture (observed at cycle %d).",
			k, o.RealizedLanes, o.DesiredLanes, cycle),
		Files:     []string{"go/internal/triagecap"},
		Source:    "fleet-starvation-observer",
		CreatedAt: nowRFC3339,
	}
}

// Validate rejects an under-weighted or incompletely-populated item so a
// malformed self-injection fails loud rather than seeding a silent no-op todo.
func (it StarvationItem) Validate() error {
	if it.Weight < starvationWeightFloor {
		return fmt.Errorf("fleet: starvation item weight %v below floor %v", it.Weight, starvationWeightFloor)
	}
	for _, f := range []struct{ name, val string }{
		{"id", it.ID}, {"title", it.Title}, {"kind", it.Kind},
		{"description", it.Description}, {"source", it.Source}, {"created_at", it.CreatedAt},
	} {
		if f.val == "" {
			return fmt.Errorf("fleet: starvation item missing required field %q", f.name)
		}
	}
	return nil
}

// WriteTo validates then atomically writes the item to
// <evolveDir>/inbox/<id>.json and returns the written path. Idempotent by id:
// the filename derives from the cause-stable id, so a second call for the same
// cause overwrites the single open todo rather than accumulating duplicates.
func (it StarvationItem) WriteTo(evolveDir string) (string, error) {
	if err := it.Validate(); err != nil {
		return "", err
	}
	path := filepath.Join(evolveDir, "inbox", it.ID+".json")
	if err := atomicwrite.JSON(path, it); err != nil {
		return "", fmt.Errorf("fleet: write starvation item: %w", err)
	}
	return path, nil
}
