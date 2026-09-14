package loopwave

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/fleet"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/ipcenv"
	"github.com/mickeyyaya/evolve-loop/go/internal/signalcenter"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// lifecycleItem plants an inbox todo where inboxmover resolves `state`:
// pending at the inbox root, processing under processing/cycle-9/, every
// other state under inbox/<state>/.
func lifecycleItem(t *testing.T, evolveDir, state, id string, deps ...string) {
	t.Helper()
	dir := filepath.Join(evolveDir, "inbox")
	switch state {
	case inboxmover.StatePending:
	case inboxmover.StateProcessing, inboxmover.StateProcessed, inboxmover.StateRejected:
		dir = filepath.Join(dir, state, "cycle-9") // the promoter nests these by cycle (lifecycle.promoteDestPath)
	default:
		dir = filepath.Join(dir, state)
	}
	doc := map[string]any{"id": id, "weight": 0.5, "files": []string{"pkg/" + id + ".go"}}
	if len(deps) > 0 {
		doc["deps"] = deps
	}
	writeJSON(t, filepath.Join(dir, id+".json"), doc)
}

// --- 23. the freshness-gated launcher ---

func TestLauncher_AllStaleLaunchesNothingAndSignals(t *testing.T) {
	h := newHarness(t)
	lifecycleItem(t, h.evolveDir, inboxmover.StateProcessed, "a")
	lifecycleItem(t, h.evolveDir, inboxmover.StateProcessed, "b")
	inner := &recorder{}
	l := h.e.Launcher(7, 2, nil).(gatedLauncher)
	l.inner = inner
	results := l.Run(context.Background(), []fleet.CycleSpec{{Scope: []string{"a"}}, {Scope: []string{"b"}}})
	if results != nil || len(inner.calls) != 0 {
		t.Errorf("an all-stale wave launches nothing: %v %d", results, len(inner.calls))
	}
	ev := h.only(t, CodeWaveAllLanesStale)
	if ev.Kind != signalcenter.KindLoopWave || ev.Origin != "gatedLauncher.Run" || ev.Reason != "freshness gate: all 2 planned lane(s) stale (2 skip(s)), nothing to launch" ||
		ev.Fields["planned"] != "2" || ev.Fields["skipped"] != "2" || ev.Fields["wave"] != "7" {
		t.Errorf("one loop.wave WARN naming the counts and the wave: %+v", ev)
	}
	sections := strings.Split(golden(t, "stderr_wave.golden.txt"), "== launcher_all_stale\n")
	if want := strings.TrimSuffix(sections[1], "\n"); !strings.HasPrefix(want, h.stderr.String()) || strings.Count(h.stderr.String(), "\n") != 2 {
		t.Errorf("fleet's own two skip lines stay on stderr; the third became the signal:\n got %q\nwant prefix of %q", h.stderr.String(), want)
	}
}

func TestLauncher_PartiallyFreshLaunchesTheKeptSpecsWithTheSameCtx(t *testing.T) {
	h := newHarness(t)
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "fresh")
	lifecycleItem(t, h.evolveDir, inboxmover.StateRejected, "stale")
	var gotCtx context.Context
	inner := &ctxRecorder{}
	l := h.e.Launcher(1, 1, nil).(gatedLauncher)
	l.inner = inner
	ctx := context.WithValue(context.Background(), ctxKey{}, "same")
	results := l.Run(ctx, []fleet.CycleSpec{{Scope: []string{"fresh"}}, {Scope: []string{"stale"}}})
	gotCtx = inner.ctx
	if len(results) != 1 || len(inner.specs) != 1 || inner.specs[0].Scope[0] != "fresh" || gotCtx.Value(ctxKey{}) != "same" || len(*h.events) != 0 {
		t.Errorf("only the kept spec launches, with the caller's ctx, no event: %v %v %v", results, inner.specs, h.codes())
	}
	if sup, ok := h.e.Launcher(1, 3, nil).(gatedLauncher).inner.(*fleet.Supervisor); !ok || sup.Concurrency != 3 {
		t.Errorf("the production inner is a Supervisor at the given concurrency: %+v", h.e.Launcher(1, 3, nil))
	}
}

type ctxRecorder struct {
	ctx   context.Context
	specs []fleet.CycleSpec
}

func (r *ctxRecorder) Run(ctx context.Context, specs []fleet.CycleSpec) []fleet.Result {
	r.ctx, r.specs = ctx, specs
	return make([]fleet.Result, len(specs))
}

// --- 24. the probe ---

func TestFreshnessProbe_ResolvesTheInboxLifecycle(t *testing.T) {
	h := newHarness(t)
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "dep-pending")
	lifecycleItem(t, h.evolveDir, inboxmover.StateProcessing, "dep-processing")
	lifecycleItem(t, h.evolveDir, inboxmover.StateRetry, "dep-retry")
	lifecycleItem(t, h.evolveDir, inboxmover.StateProcessed, "dep-done")
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "blocked-pending", "dep-done", "dep-pending")
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "blocked-processing", "dep-processing")
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "blocked-retry", "dep-retry")
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "ready", "dep-done")
	lifecycleItem(t, h.evolveDir, inboxmover.StateRejected, "rej")
	lifecycleItem(t, h.evolveDir, inboxmover.StateQuarantine, "quar")
	probe := h.e.probe()
	cases := map[string]fleet.TaskFreshness{
		"blocked-pending":    {Fresh: false, Reason: "deps unmet: needs dep-pending"},
		"blocked-processing": {Fresh: false, Reason: "deps unmet: needs dep-processing"},
		"blocked-retry":      {Fresh: false, Reason: "deps unmet: needs dep-retry"},
		"ready":              {Fresh: true},
		"never-seen":         {Fresh: true},
		"dep-done":           {Fresh: false, Reason: "consumed: processed cycle-9"},
		"rej":                {Fresh: false, Reason: "consumed: rejected cycle-9"},
		"dep-retry":          {Fresh: false, Reason: "consumed: retry"},
		"quar":               {Fresh: false, Reason: "consumed: quarantine"},
		"dep-processing":     {Fresh: false, Reason: "consumed: processing cycle-9"},
	}
	for id, want := range cases {
		if got := probe(id); got != want {
			t.Errorf("%s: %+v, want %+v", id, got, want)
		}
	}
}

// --- 25. the refill ---

func TestRefill_PicksHighestWeightNotExcludedWithFleetScopeEnv(t *testing.T) {
	h := newHarness(t)
	// Q-W8: the refill reads <ProjectRoot>/.evolve/inbox — pinned by a
	// harness whose EvolveDir is that path and a decoy elsewhere.
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "low.json"), map[string]any{"id": "low", "weight": 0.2, "files": []string{"l.go"}})
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "high.json"), map[string]any{"id": "high", "weight": 0.9, "files": []string{"h.go"}})
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "mid.json"), map[string]any{"id": "mid", "weight": 0.5, "files": []string{"m.go"}})
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "guarded.json"), map[string]any{"id": "guarded", "weight": 0.95, "files": []string{"go/protected/x.go"}})
	refill := h.e.refill()
	spec, ok := refill(map[string]bool{})
	if !ok || spec.Scope[0] != "high" || spec.Env[ipcenv.FleetScopeKey] != "high" || len(spec.Scope) != 1 {
		t.Errorf("the highest-weight dispatchable item, shaped as a lane: %+v %v", spec, ok)
	}
	if spec, ok := refill(map[string]bool{"high": true}); !ok || spec.Scope[0] != "mid" {
		t.Errorf("exclusion honoured: %+v", spec)
	}
	if _, ok := refill(map[string]bool{"high": true, "mid": true, "low": true}); ok {
		t.Error("an exhausted backlog reports no candidate")
	}
	other := New(Roots{ProjectRoot: t.TempDir(), EvolveDir: h.evolveDir}, h.ports, h.stderr)
	if _, ok := other.refill()(map[string]bool{}); ok {
		t.Error("the refill reads ProjectRoot/.evolve, not EvolveDir (Q-W8 held)")
	}
}

// --- 27. the three "consumed" beliefs, characterised ---

func TestConsumedHasThreeBeliefs(t *testing.T) {
	if !isConsumed(inboxmover.StateRetry) || isConsumed(inboxmover.StateQuarantine) || isConsumed(inboxmover.StateProcessing) || isConsumed(inboxmover.StatePending) || isConsumed(inboxmover.StateUnknown) {
		t.Error("plan-time prune: processed|rejected|retry are consumed; quarantine and processing are not (belief 1)")
	}
	if !isConsumed(inboxmover.StateProcessed) || !isConsumed(inboxmover.StateRejected) || !isConsumed(inboxmover.StateConsumed) {
		t.Error("processed, rejected and consumed (the in-commit landing consumption) are consumed")
	}
	h := newHarness(t)
	lifecycleItem(t, h.evolveDir, inboxmover.StateQuarantine, "q")
	lifecycleItem(t, h.evolveDir, inboxmover.StateRetry, "r")
	lifecycleItem(t, h.evolveDir, inboxmover.StateConsumed, "c")
	lifecycleItem(t, h.evolveDir, inboxmover.StateProcessed, "s")
	kept := triagecap.PruneConsumed(h.evolveDir, []triagecap.FleetCandidate{{ID: "q"}, {ID: "r"}, {ID: "c"}, {ID: "s"}})
	if len(kept) != 1 || kept[0].ID != "r" {
		t.Errorf("widen's PruneConsumed drops quarantine, consumed and processed (nested by cycle) and keeps retry (belief 2): %+v", kept)
	}
	lifecycleItem(t, h.evolveDir, inboxmover.StateProcessing, "p")
	if f := h.e.probe()("p"); f.Fresh {
		t.Error("the dispatch probe marks processing stale (belief 3)")
	}
}

// --- Preflight ---

func TestPreflight_RefusesANonGitRoot(t *testing.T) {
	if err := Preflight(t.TempDir())(); err == nil || !strings.Contains(err.Error(), "control-plane preflight") {
		t.Errorf("a non-git root is unverifiable and refuses: %v", err)
	}
}
