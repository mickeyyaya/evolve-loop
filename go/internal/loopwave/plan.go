package loopwave

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/inboxbatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

func (e *Engine) PlanFn(count int) PlanFn {
	return func(ctx context.Context, wave int) ([]byte, []string, error) {
		e.releaseStaleClaims(wave)
		if lastCycle, err := e.ports.LastCycle(ctx); err == nil && lastCycle > 0 {
			companion := filepath.Join(e.ports.Workspace(lastCycle), triagecap.TriageDecisionName())
			if data, rerr := os.ReadFile(companion); rerr == nil {
				data = e.pruneRouted(e.pruneUndispatchable(data))
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

type card struct {
	ID    string   `json:"id"`
	Files []string `json:"files"`
}

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

func cards(menus [][]triagecap.FleetCandidate) []map[string]any {
	var topN []map[string]any
	for _, menu := range menus {
		for _, c := range menu {
			topN = append(topN, cardOf(c.ID, c.Files))
		}
	}
	return topN
}

func (e *Engine) releaseStaleClaims(wave int) {
	opts := e.lifecycle()
	opts.CurrentGoal = e.goal
	released, err := inboxmover.ReleaseStaleClaims(opts, fmt.Sprintf("wave %d plan", wave))
	for _, r := range released {
		fmt.Fprintf(e.stderr, "[loop] wave plan: released the claim of %q held by cycle-%d (%s): %s\n", r.ID, r.Cycle, r.Outcome, r.Holder.Reason)
	}
	if err != nil {
		fmt.Fprintf(e.stderr, "[loop] WARN: wave plan: a stale claim stays held: %v\n", err)
	}
}

func (e *Engine) pruneUndispatchable(data []byte) []byte {
	lifecycle := e.lifecycle()
	return e.pruneTopN(data, func(id string) (string, bool) {
		d := inboxmover.ResolveDispatchability(lifecycle, id)
		return fmt.Sprintf("pruned undispatchable top_n id %q from prior decision (%s)", id, d.Reason), !d.Dispatchable
	})
}

func (e *Engine) pruneRouted(data []byte) []byte {
	routed := e.routedBase()
	return e.pruneTopN(data, func(id string) (string, bool) {
		r, reason := routed(id)
		return fmt.Sprintf("pruned console-routed top_n id %q from prior decision before widening (%s)", id, reason), r
	})
}

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

func remarshalFull(data []byte, topN []map[string]any) []byte {
	var full map[string]any
	if err := json.Unmarshal(data, &full); err != nil {
		return data
	}
	full["top_n"] = topN
	return marshalOr(data, full)
}

func remarshalTopN(data []byte, topN []map[string]any) []byte {
	return marshalOr(data, map[string]any{"top_n": topN})
}

func marshalOr(data []byte, v any) []byte {
	out, err := json.Marshal(v)
	if err != nil {
		return data
	}
	return out
}
