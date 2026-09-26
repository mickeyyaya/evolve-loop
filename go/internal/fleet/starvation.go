package fleet

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
)

// starvationWeightFloor mirrors policy.FleetConfig's clamp, so neither a bad config nor a bad caller under-weights the todo.
const starvationWeightFloor = 0.9

// starvationItemID is independent of the cycle, so a re-fire overwrites the single open todo.
const starvationItemID = "fleet-work-supply-starvation"

// WaveObservation is one wave's realized-vs-desired lane count.
type WaveObservation struct {
	DesiredLanes  int  // the configured fleet count, not the quota-shrunk wave count
	RealizedLanes int  // lanes that actually dispatched work this wave
	QuotaShrunk   bool // the quota-aware shrink reduced this wave's capacity
}

// Starved reports fewer lanes realized than desired for a reason other than a quota shrink.
func (o WaveObservation) Starved() bool {
	return o.RealizedLanes < o.DesiredLanes && !o.QuotaShrunk
}

// StarvationTracker counts consecutive starved waves; hold one across waves so the streak spans them.
type StarvationTracker struct {
	streak int
}

// Streak returns the current consecutive-starved-wave count.
func (t *StarvationTracker) Streak() int { return t.streak }

// Observe reports whether o is the k-th consecutive starved wave (k<1 means 1); a fire or a recovered wave resets the streak.
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

// StarvationItem is the self-filed inbox todo; its JSON is a superset of what triagecap.ReadInboxBacklog reads.
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

// BuildStarvationItem builds the inbox todo for k consecutive starved waves, clamping weight up to 0.9.
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

// Validate rejects an item below the weight floor or missing a required field.
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

// WriteTo validates then atomically writes the item to <evolveDir>/inbox/<id>.json, returning the path.
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
