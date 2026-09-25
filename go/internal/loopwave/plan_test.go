package loopwave

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/budgethistory"
	"github.com/mickeyyaya/evolve-loop/go/internal/clihealth"
	"github.com/mickeyyaya/evolve-loop/go/internal/inboxmover"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
	"github.com/mickeyyaya/evolve-loop/go/internal/quotastate"
	"github.com/mickeyyaya/evolve-loop/go/internal/triagecap"
)

// The fold-0 sizing fixtures (cmd_loop_wave_budget_test.go's tightQuota /
// fastPace / tokenFastPace, copied verbatim so the golden replays).
func tightQuota(now time.Time) []quotastate.QuotaState {
	return []quotastate.QuotaState{{
		Family:     "claude",
		Source:     quotastate.SourceProbed,
		Buckets:    []quotastate.Bucket{{Name: "week", UsedFraction: 0.9, ResetAt: now.Add(time.Hour)}},
		ObservedAt: now,
	}}
}

func fastPace() budgethistory.Throughput {
	return budgethistory.Throughput{SampleCount: 5, MedianCycleDurationMS: 360000, CyclesPerHour: 10}
}

func tokenFastPace(medianTokens int64) budgethistory.Throughput {
	tp := fastPace()
	tp.MedianTokensPerCycle = medianTokens
	return tp
}

func stderrSection(t *testing.T, name string) string {
	t.Helper()
	for _, s := range strings.Split(golden(t, "stderr_wave.golden.txt"), "== ") {
		n, body, _ := strings.Cut(s, "\n")
		if n == name {
			return body
		}
	}
	t.Fatalf("no golden section %q", name)
	return ""
}

// --- 28-29. Size ---

func TestSize_NilBudgetByteIdentical(t *testing.T) {
	h := newHarness(t)
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	fc := policy.FleetConfig{Count: 3, Concurrency: 3, MinLanes: 1}
	got, pace := h.e.Size(fc, tightQuota(now), fastPace(), now)
	if got.Count != 3 || pace != 0 || h.stderr.String() != stderrSection(t, "size_nil_budget") || len(*h.events) != 0 {
		t.Errorf("no budget: no resize, no pace, no line, no event: %+v %v %q", got, pace, h.stderr.String())
	}
}

func TestSize_ShadowLogsAndHolds(t *testing.T) {
	h := newHarness(t)
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	fc := policy.FleetConfig{Count: 3, Concurrency: 3, MinLanes: 1, Budget: &policy.FleetBudgetConfig{Stage: "shadow", CapacityCycles: 10, Safety: 0.5, HistoryWindow: 10}}
	got, pace := h.e.Size(fc, tightQuota(now), tokenFastPace(1110), now)
	if got.Count != 3 || pace != 0 || fc.Count != 3 {
		t.Errorf("shadow holds the count and never paces; the caller's fc is a copy: %+v %v %+v", got, pace, fc)
	}
	if h.stderr.String() != stderrSection(t, "size_shadow") {
		t.Errorf("the two [budget] lines verbatim:\n got %q\nwant %q", h.stderr.String(), stderrSection(t, "size_shadow"))
	}
}

func TestSize_EnforceResizesAndPaces(t *testing.T) {
	h := newHarness(t)
	now := time.Date(2026, time.July, 3, 12, 0, 0, 0, time.UTC)
	fc := policy.FleetConfig{Count: 3, Concurrency: 3, MinLanes: 1, Budget: &policy.FleetBudgetConfig{Stage: "enforce", CapacityCycles: 10, Safety: 0.5, HistoryWindow: 10}}
	got, _ := h.e.Size(fc, tightQuota(now), fastPace(), now)
	if got.Count != 1 || fc.Count != 3 || h.stderr.String() != stderrSection(t, "size_enforce") {
		t.Errorf("enforce applies the plan: %+v %+v %q", got, fc, h.stderr.String())
	}
	// Floor-forced: the surplus is paced.
	fc = policy.FleetConfig{Count: 4, Concurrency: 4, MinLanes: 2, Budget: &policy.FleetBudgetConfig{Stage: "enforce", CapacityCycles: 10, Safety: 0.5, HistoryWindow: 10}}
	states := tightQuota(now)
	states[0].Buckets[0].UsedFraction = 0.8
	got, pace := h.e.Size(fc, states, budgethistory.Throughput{SampleCount: 5, MedianCycleDurationMS: 360000, CyclesPerHour: 2}, now)
	if got.Count != 2 || pace <= 0 {
		t.Errorf("the floor forces two lanes and paces: %+v %v", got, pace)
	}
}

func TestSize_BenchedFamiliesReadFromTheStore(t *testing.T) {
	h := newHarness(t)
	until := time.Now().Add(time.Hour)
	if err := clihealth.NewStore(h.root, nil).Bench(clihealth.Entry{Family: "codex", Reason: "rate_limit", BenchedAt: time.Now(), BenchedUntil: until}); err != nil {
		t.Fatal(err)
	}
	var seen map[string]string
	var seenCount, seenMin int
	h.ports.Shrink = func(count int, benched map[string]string, minLanes int, _ io.Writer) int {
		seen, seenCount, seenMin = benched, count, minLanes
		return count - 1
	}
	e := New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr)
	got, _ := e.Size(policy.FleetConfig{Count: 3, MinLanes: 2}, nil, budgethistory.Throughput{}, time.Now())
	if got.Count != 2 || seenCount != 3 || seenMin != 2 || seen["codex"] != "rate_limit" || len(seen) != 1 {
		t.Errorf("the shrink port sees the bench map and the floor: %+v %v %d %d", got, seen, seenCount, seenMin)
	}
	// The real shrink prints fleet's own line through the engine's writer.
	got, _ = h.e.Size(policy.FleetConfig{Count: 3, MinLanes: 1}, nil, budgethistory.Throughput{}, time.Now())
	if got.Count != 2 || !strings.Contains(h.stderr.String(), `[loop] WARN: fleet: quota bench on CLI family "codex" (rate_limit): wave count 3 -> 2 (min 1)`) {
		t.Errorf("fleet.QuotaAwareCount shrinks and warns: %+v %q", got, h.stderr.String())
	}
	if h.e.benchedFamilies()["codex"] != "rate_limit" || New(Roots{ProjectRoot: t.TempDir()}, h.ports, io.Discard).benchedFamilies() != nil {
		t.Error("benchedFamilies reads the store; no benches is nil")
	}
}

// --- 30-31. PlanFn ---

func TestPlanFn_PriorDecisionIsPrunedThenWidened(t *testing.T) {
	h := newHarness(t)
	var gotCtx context.Context
	h.ports.LastCycle = func(ctx context.Context) (int, error) { gotCtx = ctx; return 3, nil }
	e := New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr)
	writeJSON(t, filepath.Join(h.ports.Workspace(3), triagecap.TriageDecisionName()),
		map[string]any{"top_n": []map[string]any{{"id": "alpha", "files": []string{"a.go"}}, {"id": "gamma", "files": []string{"g.go"}}}})
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "alpha")
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "beta")
	lifecycleItem(t, h.evolveDir, inboxmover.StateProcessed, "gamma")
	ctx := context.WithValue(context.Background(), ctxKey{}, "caller")
	data, packages, err := e.PlanFn(2)(ctx, 1)
	if err != nil || packages != nil || gotCtx.Value(ctxKey{}) != "caller" {
		t.Fatalf("the plan reads the prior cycle with the caller's ctx: %v %v", err, packages)
	}
	if s := string(data); !strings.Contains(s, `"alpha"`) || strings.Contains(s, `"gamma"`) || !strings.Contains(s, `"beta"`) {
		t.Errorf("gamma pruned, the slot refilled from the backlog (prune BEFORE widen): %s", s)
	}
	if !strings.Contains(h.stderr.String(), `pruned consumed top_n id "gamma"`) {
		t.Errorf("the :577 line on stderr: %q", h.stderr.String())
	}
	// A LastCycle error falls through to the seed, never a missing workspace.
	h.ports.LastCycle = func(context.Context) (int, error) { return 0, errors.New("state unreadable") }
	e = New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, io.Discard)
	if data, _, err := e.PlanFn(2)(context.Background(), 1); err != nil || !strings.Contains(string(data), `"beta"`) {
		t.Errorf("a failed last-cycle read seeds from the inbox: %s %v", data, err)
	}
}

// TestPlanFn_ConsoleRoutedPriorIDsArePrunedBeforeWidening (F34, wave 7,
// 2026-09-26): the prior cycle's triage committed ids the classifier NOW
// routes to the console — an operator stamp landed after that triage, or the
// declared surface is protected. Kept, they filled the fleet width, the widen
// short-circuited, and the plan-time gate refused them only after the lanes
// were cut: the wave ran 1 of 2 lanes. Pruned BEFORE the widen (the same
// ordering the consumed prune keeps), their slots refill from the backlog.
func TestPlanFn_ConsoleRoutedPriorIDsArePrunedBeforeWidening(t *testing.T) {
	h := newHarness(t)
	h.ports.LastCycle = func(context.Context) (int, error) { return 3, nil }
	e := New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr)
	writeJSON(t, filepath.Join(h.ports.Workspace(3), triagecap.TriageDecisionName()),
		map[string]any{"top_n": []map[string]any{{"id": "stamped", "files": []string{"s.go"}}, {"id": "guarded", "files": []string{"g.go"}}, {"id": "alpha", "files": []string{"a.go"}}}})
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "stamped.json"), map[string]any{"id": "stamped", "route": "console-manual", "files": []string{"pkg/stamped.go"}})
	writeJSON(t, filepath.Join(h.evolveDir, "inbox", "guarded.json"), map[string]any{"id": "guarded", "files": []string{"pkg/protected/guarded.go"}})
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "alpha")
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "beta")
	data, _, err := e.PlanFn(2)(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(data); strings.Contains(s, `"stamped"`) || strings.Contains(s, `"guarded"`) || !strings.Contains(s, `"alpha"`) || !strings.Contains(s, `"beta"`) {
		t.Errorf("both console-routed ids pruned, their slots refilled from the backlog: %s", s)
	}
	for _, id := range []string{"stamped", "guarded"} {
		if !strings.Contains(h.stderr.String(), `pruned console-routed top_n id "`+id+`"`) {
			t.Errorf("the %s prune is loud: %q", id, h.stderr.String())
		}
	}
}

func TestPlanFn_AbsentDecisionSeedsFromInbox(t *testing.T) {
	h := newHarness(t)
	h.ports.LastCycle = func(context.Context) (int, error) { return 5, nil } // no workspace on disk
	e := New(Roots{ProjectRoot: h.root, EvolveDir: h.evolveDir}, h.ports, h.stderr)
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "one")
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "two")
	data, _, err := e.PlanFn(2)(context.Background(), 1)
	if err != nil || !strings.Contains(string(data), `"one"`) || !strings.Contains(string(data), `"two"`) {
		t.Errorf("seeded from the inbox: %s %v", data, err)
	}
}

func TestPlanFn_TooNarrowSeedErrorsWithTheWrappedText(t *testing.T) {
	h := newHarness(t)
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "only")
	_, _, err := h.e.PlanFn(2)(context.Background(), 4)
	want := "wave 4: no prior triage decision and inbox seed: 1 disjoint lane(s) — need >= 2 file-disjoint inbox todos to fill a wave"
	if err == nil || err.Error() != want {
		t.Errorf("the wrapped seed error: %v", err)
	}
}

// --- 3/31. the seed ---

func TestSeedWavePlanFromInbox_ClampsAndRefuses(t *testing.T) {
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	for i, id := range []string{"todo-a", "todo-b", "todo-c"} {
		writeJSON(t, filepath.Join(evolveDir, "inbox", id+".json"), map[string]any{"id": id, "weight": 0.9 - float64(i)/10, "files": []string{"pkg/" + id + "/" + id + ".go"}})
	}
	data, err := SeedWavePlanFromInbox(evolveDir, 1, nil)
	if err != nil || string(data) != golden(t, "decision_seed.golden.json") {
		t.Errorf("count<2 clamps to 2 and the bytes match the golden: %s %v", data, err)
	}
	_, err = SeedWavePlanFromInbox(filepath.Join(t.TempDir(), ".evolve"), 2, nil)
	if err == nil || err.Error() != "inbox seed: 0 disjoint lane(s) — need >= 2 file-disjoint inbox todos to fill a wave" {
		t.Errorf("fewer than two menus refuse with the exact text: %v", err)
	}
	protected := func(p string) bool { return strings.HasPrefix(p, "pkg/todo-a") }
	if data, err := SeedWavePlanFromInbox(evolveDir, 2, protected); err != nil || strings.Contains(string(data), "todo-a") {
		t.Errorf("the predicate is threaded into the seed: %s %v", data, err)
	}
}

// --- 32-33. prune and widen ---

func TestPruneConsumed_PreservesKeysAndReturnsOriginalWhenNothingDropped(t *testing.T) {
	h := newHarness(t)
	lifecycleItem(t, h.evolveDir, inboxmover.StatePending, "alpha")
	lifecycleItem(t, h.evolveDir, inboxmover.StateProcessed, "gamma")
	data := []byte(`{"note":"keep","top_n":[{"id":"alpha","files":["a.go"]},{"id":"gamma","files":["g.go"]}]}`)
	if got := string(h.e.pruneConsumed(data)); got != golden(t, "decision_prune.golden.json") {
		t.Errorf("prune keeps every key and rewrites top_n: %s", got)
	}
	if h.stderr.String() != stderrSection(t, "prune_dropped") {
		t.Errorf("the :577 line once: %q", h.stderr.String())
	}
	for _, same := range [][]byte{
		[]byte(`{"note":"keep","top_n":[{"id":"alpha","files":["a.go"]}]}`),
		[]byte(`{"committed_floors":["core"],"top_n":[{"id":"gamma"}]}`),
		[]byte(`{"top_n":[]}`),
		[]byte(`{not json`),
	} {
		if got := h.e.pruneConsumed(same); !bytes.Equal(got, same) {
			t.Errorf("passthrough: %s -> %s", same, got)
		}
	}
	if got := marshalOr([]byte("orig"), make(chan int)); string(got) != "orig" {
		t.Errorf("an unmarshalable value returns the original bytes: %s", got)
	}
	if got := remarshalFull([]byte("{not json"), nil); string(got) != "{not json" {
		t.Errorf("an unparseable decision returns the original bytes: %s", got)
	}
}

func TestWidenNarrow_PrunedDisarmsBothShortcutsAndRemarshalsTopNOnly(t *testing.T) {
	evolveDir := filepath.Join(t.TempDir(), ".evolve")
	lifecycleItem(t, evolveDir, inboxmover.StatePending, "alpha")
	lifecycleItem(t, evolveDir, inboxmover.StatePending, "beta")
	writeJSON(t, filepath.Join(evolveDir, "inbox", "alpha.json"), map[string]any{"id": "alpha", "weight": 0.9, "files": []string{"a.go"}})
	writeJSON(t, filepath.Join(evolveDir, "inbox", "beta.json"), map[string]any{"id": "beta", "weight": 0.8, "files": []string{"b.go"}})
	narrow := []byte(`{"note":"dropped","top_n":[{"id":"alpha","files":["a.go"]}]}`)
	if got := string(WidenNarrowDecision(narrow, evolveDir, 2, nil)); got != golden(t, "decision_widen.golden.json") {
		t.Errorf("widened to fleet width, top_n only: %s", got)
	}
	wide := []byte(`{"top_n":[{"id":"alpha","files":["a.go"]},{"id":"beta","files":["b.go"]}]}`)
	if got := WidenNarrowDecision(wide, evolveDir, 2, nil); !bytes.Equal(got, wide) {
		t.Errorf("already fleet-width with nothing consumed returns the original: %s", got)
	}
	for _, same := range [][]byte{narrow, []byte(`{"committed_floors":["core"]}`), []byte(`{not json`)} {
		if got := WidenNarrowDecision(same, evolveDir, 1, nil); !bytes.Equal(got, same) {
			t.Errorf("count<2 passes through: %s", got)
		}
	}
	if got := WidenNarrowDecision([]byte(`{"committed_floors":["core"],"top_n":[]}`), evolveDir, 2, nil); string(got) != `{"committed_floors":["core"],"top_n":[]}` {
		t.Errorf("floors pass through: %s", got)
	}
	if got := WidenNarrowDecision([]byte(`{not json`), evolveDir, 2, nil); string(got) != `{not json` {
		t.Errorf("unparseable passes through: %s", got)
	}
	// A consumed committed id at fleet width: pruned disarms the short-circuit.
	lifecycleItem(t, evolveDir, inboxmover.StateProcessed, "gone")
	stale := []byte(`{"top_n":[{"id":"alpha","files":["a.go"]},{"id":"gone","files":["z.go"]}]}`)
	if got := string(WidenNarrowDecision(stale, evolveDir, 2, nil)); strings.Contains(got, "gone") || !strings.Contains(got, "beta") {
		t.Errorf("the consumed id is dropped and the lane refilled: %s", got)
	}
	// Still fleet-width AFTER the prune (three committed, one consumed): the
	// prune disarms the "already wide" shortcut too (Q-W2).
	wideStale := []byte(`{"top_n":[{"id":"alpha","files":["a.go"]},{"id":"beta","files":["b.go"]},{"id":"gone","files":["z.go"]}]}`)
	if got := string(WidenNarrowDecision(wideStale, evolveDir, 2, nil)); strings.Contains(got, "gone") {
		t.Errorf("a fleet-width decision carrying a consumed id is rewritten, never returned verbatim: %s", got)
	}
	// Nothing to add and nothing pruned: original bytes.
	lonely := filepath.Join(t.TempDir(), ".evolve")
	if got := WidenNarrowDecision(narrow, lonely, 2, nil); !bytes.Equal(got, narrow) {
		t.Errorf("no backlog to widen with returns the original: %s", got)
	}
	// Empty ids are skipped when building the committed list.
	if got := string(WidenNarrowDecision([]byte(`{"top_n":[{"id":""}]}`), evolveDir, 2, nil)); !strings.Contains(got, "alpha") {
		t.Errorf("an empty id is not a committed lane: %s", got)
	}
	// The predicate excludes protected backlog items from the widening. It
	// judges the DECLARED surface — path-shaped files[] tokens (F29) — so the
	// protected item declares a path; the heavier gamma would win the lane
	// without the predicate, so the exclusion is observable.
	writeJSON(t, filepath.Join(evolveDir, "inbox", "gamma.json"), map[string]any{"id": "gamma", "weight": 0.95, "files": []string{"pkg/c.go"}})
	protected := func(p string) bool { return p == "pkg/c.go" }
	if got := string(WidenNarrowDecision(narrow, evolveDir, 2, protected)); strings.Contains(got, "gamma") || !strings.Contains(got, "beta") {
		t.Errorf("a protected backlog item never widens a lane: %s", got)
	}
	if got := string(WidenNarrowDecision(narrow, evolveDir, 2, nil)); !strings.Contains(got, "gamma") {
		t.Errorf("without the predicate the heavier gamma widens the lane: %s", got)
	}
}
