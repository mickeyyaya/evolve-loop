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

// PlanFn is the wave's single-writer plan source. Preferred: the immediately
// prior cycle's committed triage-decision.json (LastCycle → Workspace), pruned
// of consumed and console-routed ids and THEN widened to fleet width from the
// inbox backlog (an id consumed during an earlier wave, or one the plan-time
// gate would refuse, is dead work; dropping it first frees the slot for the
// widening to refill — pruning after would leave the wave a lane short). A
// prior decision absent (fresh start, a sealed run dir, a cycle that
// failed before triage) or a failed last-cycle read falls through to the inbox
// seed, so 2-wide starts on the FIRST cycle; a seed too narrow errors and the
// caller falls back to sequential. cardPackages is always nil (no dedicated
// card-package reader exists; top_n[].id cards flow through PlanFromTriage's
// own fallback). Only LastCycle observes ctx — the decision read is a plain
// os.ReadFile (Q-W10).
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

// SeedWavePlanFromInbox synthesizes a triage-decision.json (top_n[].id+files)
// from the inbox backlog so PlanFromTriage can partition it into disjoint
// lanes without a prior cycle's decision. count is the caller's resolved wave
// width, clamped to >= 2; fewer than 2 disjoint LANES (menus, not flattened
// ids — a single 4-item cluster still fails to fill a 2-lane wave) returns an
// error so the caller falls back to sequential. Each lane is deepened with
// its same-file cluster mates up to the compiled inboxbatch.DefaultMaxItems —
// the batching cap's single source today.
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

// decision is the wire shape both the prune and the widen read — ONE
// declaration.
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

// cardOf is the ONE top_n card builder: id, plus files only when declared.
func cardOf(id string, files []string) map[string]any {
	c := map[string]any{"id": id}
	if len(files) > 0 {
		c["files"] = files
	}
	return c
}

// cards flattens lane menus into top_n cards, files preserved — the shape
// PlanFromTriage re-partitions into the same lanes.
func cards(menus [][]triagecap.FleetCandidate) []map[string]any {
	var topN []map[string]any
	for _, menu := range menus {
		for _, c := range menu {
			topN = append(topN, cardOf(c.ID, c.Files))
		}
	}
	return topN
}

// pruneConsumed drops every top_n id whose inbox lifecycle resolves to a
// CONSUMED state (isConsumed) before the prior decision is widened. FAIL
// OPEN: only positively-consumed ids are dropped — pending ids and ids with
// no lifecycle evidence are kept (over-pruning would starve the wave). It
// resolves through inboxmover's ProjectRoot-derived inbox (Q-W7) and keeps
// EVERY other key of the decision (remarshalFull) — the first of the two
// re-marshal fidelities. Best-effort: an unparseable decision, one carrying
// committed_floors (dispatched ahead of top_n, which is ignored), an empty
// top_n or nothing consumed returns the original bytes. Note the three
// "consumed" beliefs (TestConsumedHasThreeBeliefs): this prune's set, widen's
// triagecap.PruneConsumed and the dispatch probe's differ deliberately.
func (e *Engine) pruneConsumed(data []byte) []byte {
	opts := inboxmover.Options{ProjectRoot: e.roots.ProjectRoot, Stderr: io.Discard, Signals: e.center()}
	return e.pruneTopN(data, func(id string) (string, bool) {
		return fmt.Sprintf("pruned consumed top_n id %q from prior decision", id),
			isConsumed(inboxmover.ResolveDispatchState(opts, id).State)
	})
}

// pruneRouted drops every top_n id the plan-time gate would refuse — the
// ADR-0074 classifier NOW routes it to the console (an operator stamp or a
// protected surface landed after the prior cycle's triage committed it) —
// before the widen, for the consumed prune's reason: kept, a routed id holds a
// lane slot the widen will not refill, and the gate refuses it only after the
// lanes are cut (F34: wave 7 ran 1 of 2 lanes, 2026-09-26). Same fidelity and
// passthroughs as pruneConsumed; routedBase is the gate's own authority.
func (e *Engine) pruneRouted(data []byte) []byte {
	routed := e.routedBase()
	return e.pruneTopN(data, func(id string) (string, bool) {
		r, reason := routed(id)
		return fmt.Sprintf("pruned console-routed top_n id %q from prior decision before widening (%s)", id, reason), r
	})
}

// pruneTopN is the prunes' one skeleton: drop every non-empty top_n id dead
// reports, one WARN line each, keeping every other key of the decision. An
// unparseable decision, one carrying committed_floors (dispatched ahead of
// top_n, which is ignored), an empty top_n or nothing dropped returns the
// original bytes.
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

// remarshalFull rewrites top_n inside the decision's full map — every other
// key survives (the prune's fidelity).
func remarshalFull(data []byte, topN []map[string]any) []byte {
	var full map[string]any
	if err := json.Unmarshal(data, &full); err != nil {
		return data
	}
	full["top_n"] = topN
	return marshalOr(data, full)
}

// remarshalTopN emits {"top_n": …} only — every other key dropped (the
// widen's fidelity; Q-W3).
func remarshalTopN(data []byte, topN []map[string]any) []byte {
	return marshalOr(data, map[string]any{"top_n": topN})
}

// marshalOr marshals v, returning the original bytes on the (structurally
// unreachable for parsed JSON) marshal failure — widening and pruning are
// optimizations, never a correctness dependency.
func marshalOr(data []byte, v any) []byte {
	out, err := json.Marshal(v)
	if err != nil {
		return data
	}
	return out
}

// isConsumed reports whether a lifecycle state means the item is done with
// at plan time: processed, consumed (the in-commit landing consumption),
// rejected or retry. StateProcessing is NOT
// consumed: it is in flight, and its own claim keeps a second lane off it;
// quarantine is not in this set either (belief 1 of 3 — Q-W5).
func isConsumed(state string) bool {
	switch state {
	case inboxmover.StateProcessed, inboxmover.StateConsumed, inboxmover.StateRejected, inboxmover.StateRetry:
		return true
	}
	return false
}
