package main

// cmd_setup_latest_test.go — `evolve setup latest`: the read-only live probe
// behind /evo:setup's "a newer model is available" option. Driven through the
// report builder with a fake lister: the defect classes are a probe that
// quietly skips a family, a failure that kills the whole report instead of
// its own row, and a parallel fan-out that scrambles row order.

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
	"github.com/mickeyyaya/evolve-loop/go/internal/setup"
)

// fakeLister records concurrency and serves per-CLI candidate lists.
type fakeLister struct {
	mu       sync.Mutex
	inflight int
	peak     int
	lists    map[string][]string
	errs     map[string]error
	delay    time.Duration
}

func (f *fakeLister) List(_ context.Context, cli string) ([]string, error) {
	f.mu.Lock()
	f.inflight++
	if f.inflight > f.peak {
		f.peak = f.inflight
	}
	f.mu.Unlock()
	time.Sleep(f.delay)
	f.mu.Lock()
	f.inflight--
	f.mu.Unlock()
	if err := f.errs[cli]; err != nil {
		return nil, err
	}
	return f.lists[cli], nil
}

func latestDetectFixture() setup.DetectReport {
	return setup.DetectReport{CLIs: []setup.CLIStatus{
		{CLI: "claude", Verdict: "ready", TierModels: map[string]string{"deep": "opus-4.6"}},
		{CLI: "codex", Verdict: "ready", TierModels: map[string]string{"deep": "gpt-5.5"}},
		{CLI: "agy", Verdict: "ready", TierModels: map[string]string{"deep": "gemini-3.1-pro"}},
		{CLI: "ollama", Verdict: "blocked", TierModels: map[string]string{"deep": "qwen3"}},
	}}
}

func TestSetupLatestReport_QueriesEveryReadyFamilyInParallel(t *testing.T) {
	t.Parallel()
	fl := &fakeLister{
		delay: 50 * time.Millisecond,
		lists: map[string][]string{
			"claude": {"opus", "opus-4.5"},
			"codex":  {"gpt-5.6", "gpt-5.5", "gpt-4o"},
			"agy":    {"gemini-3.1-pro"},
		},
	}
	fresh := map[string]modelquery.FreshnessPolicy{
		"claude": {PreferAlias: true, AliasIDs: []string{"opus"}},
	}
	rep := setupLatestReport(context.Background(), latestDetectFixture(), nil, fl, fresh)

	if len(rep.CLIs) != 3 {
		t.Fatalf("every READY family gets a row (blocked ollama excluded): %+v", rep.CLIs)
	}
	// Deterministic row order = detection order, regardless of fan-out timing.
	for i, want := range []string{"claude", "codex", "agy"} {
		if rep.CLIs[i].CLI != want {
			t.Fatalf("row %d = %s, want %s (parallel fan-out must not scramble order)", i, rep.CLIs[i].CLI, want)
		}
	}
	if fl.peak < 2 {
		t.Errorf("probes ran sequentially (peak inflight %d) — the operator directive is a PARALLEL collection", fl.peak)
	}
	codex := rep.CLIs[1]
	if codex.LatestModel != "gpt-5.6" || !codex.MapStale || codex.Candidates != 3 || !codex.CurrentSeenLive {
		t.Errorf("codex live gpt-5.6 must mark the gpt-5.5 map stale: %+v", codex)
	}
	if len(codex.StaleTiers) != 1 || codex.StaleTiers[0] != (setup.TierStale{Tier: "deep", Current: "gpt-5.5", Latest: "gpt-5.6"}) {
		t.Errorf("the stale tier must carry its QUOTABLE evidence (tier+current+latest): %+v", codex.StaleTiers)
	}
	claude := rep.CLIs[0]
	if claude.LatestModel != "opus" || !claude.MapStale {
		t.Errorf("claude's alias policy must surface the alias as freshest: %+v", claude)
	}
	if agy := rep.CLIs[2]; agy.MapStale {
		t.Errorf("agy already at its lineage's freshest — not stale: %+v", agy)
	}
}

func TestSetupLatestReport_ProbeFailureIsOneRowNotTheReport(t *testing.T) {
	t.Parallel()
	fl := &fakeLister{
		lists: map[string][]string{"claude": {"opus-4.6"}, "agy": {"gemini-3.1-pro"}},
		errs:  map[string]error{"codex": errors.New("tmux capture timed out")},
	}
	rep := setupLatestReport(context.Background(), latestDetectFixture(), nil, fl, nil)
	if len(rep.CLIs) != 3 {
		t.Fatalf("a failed probe keeps its row: %+v", rep.CLIs)
	}
	codex := rep.CLIs[1]
	if codex.Error == "" || codex.Candidates != 0 || codex.MapStale || codex.CurrentSeenLive {
		t.Errorf("the failed family must carry its error, zero candidates, and never claim staleness or observation: %+v", codex)
	}
	if rep.CLIs[0].Error != "" || rep.CLIs[2].Error != "" {
		t.Errorf("sibling probes must be unaffected: %+v", rep.CLIs)
	}
}

// The catalog is the hot-reloading dispatch authority: when it carries a tier
// map for a family, staleness is judged against IT, not the manifest default.
func TestSetupLatestReport_CatalogOverridesManifestBaseline(t *testing.T) {
	t.Parallel()
	fl := &fakeLister{lists: map[string][]string{
		"claude": {"opus-4.6"}, "codex": {"gpt-5.6"}, "agy": {"gemini-3.1-pro"},
	}}
	catTiers := map[string]map[string]string{"codex": {"deep": "gpt-5.6"}}
	rep := setupLatestReport(context.Background(), latestDetectFixture(), catTiers, fl, nil)
	codex := rep.CLIs[1]
	if codex.CurrentDeepModel != "gpt-5.6" || codex.MapStale {
		t.Errorf("catalog already at the live latest ⇒ not stale (manifest gpt-5.5 must not be the baseline): %+v", codex)
	}
}

// A stale BALANCED tier alone must trip the report: most phases dispatch
// balanced, and deep-only scoping let exactly that staleness escape (review
// finding).
func TestSetupLatestReport_BalancedTierStalenessCounts(t *testing.T) {
	t.Parallel()
	fl := &fakeLister{lists: map[string][]string{
		"codex": {"gpt-5.5", "gpt-5.4-mini", "gpt-5.5-mini"},
	}}
	rep := setup.DetectReport{CLIs: []setup.CLIStatus{
		{CLI: "codex", Verdict: "ready", TierModels: map[string]string{"deep": "gpt-5.5", "balanced": "gpt-5.4-mini"}},
	}}
	row := setupLatestReport(context.Background(), rep, nil, fl, nil).CLIs[0]
	if !row.MapStale || len(row.StaleTiers) != 1 || row.StaleTiers[0] != (setup.TierStale{Tier: "balanced", Current: "gpt-5.4-mini", Latest: "gpt-5.5-mini"}) {
		t.Errorf("a fresher gpt-5.5-mini must mark the BALANCED tier stale WITH its own pair — the deep-anchored fields degenerated to X→X here (review finding): %+v", row)
	}
}

// The identity fallback (tier map missing -> model == tier word) must read
// UNVERIFIED, never "freshest" — the fabricated-verdict class the live smoke
// surfaced.
func TestSetupLatestReport_NeverSeenCurrentIsUnverifiedNotFresh(t *testing.T) {
	t.Parallel()
	fl := &fakeLister{lists: map[string][]string{"ollama": {"qwen3", "llama4"}}}
	rep := setup.DetectReport{CLIs: []setup.CLIStatus{
		{CLI: "ollama", Verdict: "ready", TierModels: map[string]string{"deep": "deep"}},
	}}
	row := setupLatestReport(context.Background(), rep, nil, fl, nil).CLIs[0]
	if row.CurrentSeenLive || row.MapStale {
		t.Errorf("a tier-word fallback must not be judged fresh OR stale: %+v", row)
	}
}

// The per-capture timeout is load-bearing: without it one hung capture stalls
// the entire parallel probe. Pinned by overriding the injectable deadline.
func TestSetupLatestReport_HungCaptureIsBoundedByTheTimeout(t *testing.T) {
	old := setupLatestProbeTimeout
	setupLatestProbeTimeout = 50 * time.Millisecond
	defer func() { setupLatestProbeTimeout = old }()

	rep := setup.DetectReport{CLIs: []setup.CLIStatus{
		{CLI: "codex", Verdict: "ready", TierModels: map[string]string{"deep": "gpt-5.5"}},
	}}
	row := setupLatestReport(context.Background(), rep, nil, hangingLister{}, nil).CLIs[0]
	if row.Error == "" {
		t.Fatalf("a hung capture must surface as the row's error via the deadline: %+v", row)
	}
}

type hangingLister struct{}

func (hangingLister) List(ctx context.Context, _ string) ([]string, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

// A non-deep model that VANISHED from the bridge must surface per-tier — the
// deep-anchored current_seen_live cannot express it (review finding).
func TestSetupLatestReport_VanishedBalancedTierIsNamedUnverified(t *testing.T) {
	t.Parallel()
	fl := &fakeLister{lists: map[string][]string{"codex": {"gpt-5.5"}}}
	rep := setup.DetectReport{CLIs: []setup.CLIStatus{
		{CLI: "codex", Verdict: "ready", TierModels: map[string]string{"deep": "gpt-5.5", "balanced": "o4-mini"}},
	}}
	row := setupLatestReport(context.Background(), rep, nil, fl, nil).CLIs[0]
	if row.MapStale || !row.CurrentSeenLive {
		t.Fatalf("deep is fresh and observed; the vanished balanced model is not a staleness: %+v", row)
	}
	if len(row.UnverifiedTiers) != 1 || row.UnverifiedTiers[0] != "balanced" {
		t.Errorf("the vanished balanced model must be NAMED unverified: %+v", row)
	}
}
