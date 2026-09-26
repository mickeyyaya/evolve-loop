package loopwave

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// PlanFn returns the wave's plan source: the prior cycle's triage decision, else an
// inbox seed. It prunes the decision before widening it, so the widen refills the freed slots.
func (e *Engine) PlanFn(count int) PlanFn {
	return func(ctx context.Context, wave int) ([]byte, []string, error) {
		if lastCycle, err := e.ports.LastCycle(ctx); err == nil && lastCycle > 0 {
			companion := filepath.Join(e.ports.Workspace(lastCycle), triagecap.TriageDecisionName())
			if data, rerr := os.ReadFile(companion); rerr == nil {
				data = e.pruneRouted(e.pruneConsumed(data))
				return WidenNarrowDecision(data, e.roots.EvolveDir, count, e.ports.Protected), nil, nil
			}
		}
		data, err := SeedWavePlanFromInbox(e.roots.EvolveDir, count, e.ports.Protected)
		if err != nil {
			return nil, nil, fmt.Errorf("wave %d: no prior triage decision and %w", wave, err)
		}
		return data, nil, nil
	}
}

// SeedWavePlanFromInbox builds a triage decision from the inbox backlog. count is
// clamped to at least 2, and fewer than 2 file-disjoint lanes is an error.
func SeedWavePlanFromInbox(evolveDir string, count int, protected func(string) bool) ([]byte, error) {
	if count < 2 {
		count = 2
	}
	menus := triagecap.SelectWaveSeedMenus(evolveDir, nil, count, inboxbatch.DefaultMaxItems, protected)
	if len(menus) < 2 {
		return nil, fmt.Errorf("inbox seed: %d disjoint lane(s) — need >= 2 file-disjoint inbox todos to fill a wave", len(menus))
	}
	return json.Marshal(map[string]any{"top_n": cards(menus)})
}

// card is one top_n entry as the decision file spells it.
type card struct {
	ID    string   `json:"id"`
	Files []string `json:"files"`
}

// decision is the triage-decision wire shape the prunes and the widen read.
type decision struct {
	CommittedFloors []string `json:"committed_floors"`
	TopN            []card   `json:"top_n"`
}

func parseDecision(data []byte) (decision, bool) {
	var d decision
	if json.Unmarshal(data, &d) != nil {
		return decision{}, false
	}
	return d, true
}

func cardOf(id string, files []string) map[string]any {
	c := map[string]any{"id": id}
	if len(files) > 0 {
		c["files"] = files
	}
	return c
}

// cards flattens lane menus into top_n cards, keeping files so the fan-out re-derives the same lanes.
func cards(menus [][]triagecap.FleetCandidate) []map[string]any {
	var topN []map[string]any
	for _, menu := range menus {
		for _, c := range menu {
			topN = append(topN, cardOf(c.ID, c.Files))
		}
	}
	return topN
}

// pruneConsumed drops top_n ids whose lifecycle is positively consumed. It fails
// open, keeping ids with no lifecycle evidence, because over-pruning starves the wave.
func (e *Engine) pruneConsumed(data []byte) []byte {
	opts := inboxmover.Options{ProjectRoot: e.roots.ProjectRoot, Stderr: io.Discard, Signals: e.center()}
	return e.pruneTopN(data, func(id string) (string, bool) {
		return fmt.Sprintf("pruned consumed top_n id %q from prior decision", id),
			isConsumed(inboxmover.ResolveDispatchState(opts, id).State)
	})
}

// pruneRouted drops top_n ids the plan-time gate would now refuse. Kept, such an
// id holds a lane slot the widen never refills, and the gate refuses it after the lanes are cut.
func (e *Engine) pruneRouted(data []byte) []byte {
	routed := e.routedBase()
	return e.pruneTopN(data, func(id string) (string, bool) {
		r, reason := routed(id)
		return fmt.Sprintf("pruned console-routed top_n id %q from prior decision before widening (%s)", id, reason), r
	})
}

// pruneTopN drops each non-empty top_n id that dead reports, with one WARN line
// per id. A decision with committed_floors passes through, because the fan-out then ignores top_n.
func (e *Engine) pruneTopN(data []byte, dead func(id string) (line string, drop bool)) []byte {
	d, ok := parseDecision(data)
	if !ok || len(d.CommittedFloors) > 0 || len(d.TopN) == 0 {
		return data
	}
	kept := make([]map[string]any, 0, len(d.TopN))
	for _, c := range d.TopN {
		if c.ID != "" {
			if line, drop := dead(c.ID); drop {
				fmt.Fprintf(e.stderr, "[loop] WARN: wave plan: %s\n", line)
				continue
			}
		}
		kept = append(kept, cardOf(c.ID, c.Files))
	}
	if len(kept) == len(d.TopN) {
		return data
	}
	return remarshalFull(data, kept)
}

// remarshalFull rewrites top_n and keeps every other key of the decision.
func remarshalFull(data []byte, topN []map[string]any) []byte {
	var full map[string]any
	if err := json.Unmarshal(data, &full); err != nil {
		return data
	}
	full["top_n"] = topN
	return marshalOr(data, full)
}

// remarshalTopN emits {"top_n": …} only, dropping every other key.
func remarshalTopN(data []byte, topN []map[string]any) []byte {
	return marshalOr(data, map[string]any{"top_n": topN})
}

// marshalOr falls back to the original bytes on a marshal failure: widening and
// pruning are optimizations, never a correctness dependency.
func marshalOr(data []byte, v any) []byte {
	out, err := json.Marshal(v)
	if err != nil {
		return data
	}
	return out
}

// isConsumed reports whether an item is done with at plan time. Processing is
// not: the item is in flight, and its own claim keeps a second lane off it.
func isConsumed(state string) bool {
	switch state {
	case inboxmover.StateProcessed, inboxmover.StateConsumed, inboxmover.StateRejected, inboxmover.StateRetry:
		return true
	}
	return false
}
