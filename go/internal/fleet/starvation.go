package fleet

import (
	"fmt"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/atomicwrite"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
)

const starvationWeightFloor = 0.9

const starvationItemID = "fleet-work-supply-starvation"

type WaveObservation struct {
	SizedLanes    int
	RealizedLanes int
	// minimal: DesiredLanes/QuotaShrunk only keep go/acs/cycle544's superseded observations compiling; an unset SizedLanes falls back to them.
	DesiredLanes int
	QuotaShrunk  bool
}

func (o WaveObservation) Starved() bool {
	return o.RealizedLanes < o.sizedWidth()
}

func (o WaveObservation) sizedWidth() int {
	if o.SizedLanes > 0 || o.QuotaShrunk {
		return o.SizedLanes
	}
	return o.DesiredLanes
}

type StarvationTracker struct {
	streak int
}

func (t *StarvationTracker) Streak() int { return t.streak }

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

type StarvationItem struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Weight        float64  `json:"weight"`
	Kind          string   `json:"kind"`
	PriorityClass string   `json:"priority_class"`
	Origin        string   `json:"origin"`
	Description   string   `json:"description"`
	Files         []string `json:"files"`
	Source        string   `json:"source"`
	CreatedAt     string   `json:"created_at"`
}

func BuildStarvationItem(o WaveObservation, k int, weight float64, cycle int, nowRFC3339 string) StarvationItem {
	if weight < starvationWeightFloor {
		weight = starvationWeightFloor
	}
	return StarvationItem{
		ID:            starvationItemID,
		Title:         "Fleet lanes work-supply-starved: realized concurrency < sized width",
		Weight:        weight,
		Kind:          "feature",
		PriorityClass: inboxbatch.ClassStability,
		Origin:        inboxbatch.OriginCycle,
		Description: fmt.Sprintf(
			"The fleet ran %d consecutive waves realizing only %d of %d sized lanes "+
				"(the width left after any quota/capacity shrink) — work-supply starvation, "+
				"not a benched CLI family. Widen the triage plan / inbox backlog so every "+
				"configured lane has file-disjoint work to dispatch, per the "+
				"fleet-concurrency-respect architecture (observed at cycle %d).",
			k, o.RealizedLanes, o.sizedWidth(), cycle),
		Files:     []string{"go/internal/triagecap"},
		Source:    "fleet-starvation-observer",
		CreatedAt: nowRFC3339,
	}
}

func (it StarvationItem) Validate() error {
	if it.Weight < starvationWeightFloor {
		return fmt.Errorf("fleet: starvation item weight %v below floor %v", it.Weight, starvationWeightFloor)
	}
	for _, f := range []struct{ name, val string }{
		{"id", it.ID}, {"title", it.Title}, {"kind", it.Kind}, {"priority_class", it.PriorityClass},
		{"description", it.Description}, {"source", it.Source}, {"created_at", it.CreatedAt},
	} {
		if f.val == "" {
			return fmt.Errorf("fleet: starvation item missing required field %q", f.name)
		}
	}
	return nil
}

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
