package modelquery

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

type countingClassifier struct {
	inner Classifier
	calls int
}

func (c *countingClassifier) Classify(ctx context.Context, cli string, ids []string) (map[string]string, error) {
	c.calls++
	return c.inner.Classify(ctx, cli, ids)
}

func fullTiers(model string) map[string]string {
	out := make(map[string]string, len(modelcatalog.CanonicalTiers))
	for _, tier := range modelcatalog.CanonicalTiers {
		out[tier] = model
	}
	return out
}

func priorFor(cli string, candidates []string, tiers map[string]string, source string) modelcatalog.Catalog {
	return modelcatalog.Catalog{
		FetchedAt: fixedNow(),
		CLIs: map[string]modelcatalog.CLIEntry{
			cli: {
				TierModels: tiers,
				Available:  candidates,
				Source:     source,
				CandidatesHash: Fingerprint(FingerprintInput{
					CLI: cli, Candidates: candidates, Tiers: modelcatalog.CanonicalTiers,
				}),
			},
		},
	}
}

func TestRefresh_UnchangedOfferingSkipsClassifier(t *testing.T) {
	t.Parallel()
	candidates := []string{"gpt-5.5-mini", "gpt-5.5"}
	cls := &countingClassifier{inner: fakeClassifier{}}
	cat, err := Refresh(context.Background(), RefreshDeps{
		CLIs:       []string{"codex"},
		Lister:     fakeLister{ids: map[string][]string{"codex": candidates}},
		Classifier: cls,
		Prior:      priorFor("codex", candidates, fullTiers("gpt-5.5"), modelcatalog.SourceLive),
		Now:        fixedNow,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if cls.calls != 0 {
		t.Errorf("classifier called %d times on an unchanged offering, want 0", cls.calls)
	}
	if m, ok := cat.Lookup("codex", "top"); !ok || m != "gpt-5.5" {
		t.Errorf("reused top = (%q,%v), want the prior mapping", m, ok)
	}
	if got := cat.CLIs["codex"].Source; got != modelcatalog.SourceLive {
		t.Errorf("reused entry Source = %q, want live", got)
	}
}

func TestRefresh_ChangedOfferingClassifies(t *testing.T) {
	t.Parallel()
	prior := priorFor("codex", []string{"gpt-5.5-mini", "gpt-5.5"}, fullTiers("gpt-5.5"), modelcatalog.SourceLive)
	cls := &countingClassifier{inner: fakeClassifier{}}
	_, err := Refresh(context.Background(), RefreshDeps{
		CLIs:       []string{"codex"},
		Lister:     fakeLister{ids: map[string][]string{"codex": {"gpt-5.5-mini", "gpt-5.5", "o4-mini"}}},
		Classifier: cls,
		Prior:      prior,
		Now:        fixedNow,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if cls.calls != 1 {
		t.Errorf("classifier calls = %d, want 1 after the offering changed", cls.calls)
	}
}

func TestRefresh_ReuseRequiresLiveAndFullCoverage(t *testing.T) {
	t.Parallel()
	candidates := []string{"gpt-5.5-mini", "gpt-5.5"}
	topless := map[string]string{"fast": "gpt-5.5-mini", "balanced": "gpt-5.5", "deep": "gpt-5.5"}
	cases := []struct {
		name  string
		prior modelcatalog.Catalog
	}{
		{"detect-sourced prior", priorFor("codex", candidates, fullTiers("gpt-5.5"), modelcatalog.SourceDetect)},
		{"top-less prior", priorFor("codex", candidates, topless, modelcatalog.SourceLive)},
	}
	for _, c := range cases {
		cls := &countingClassifier{inner: fakeClassifier{}}
		_, err := Refresh(context.Background(), RefreshDeps{
			CLIs:       []string{"codex"},
			Lister:     fakeLister{ids: map[string][]string{"codex": candidates}},
			Classifier: cls,
			Prior:      c.prior,
			Now:        fixedNow,
		})
		if err != nil {
			t.Fatalf("%s: Refresh: %v", c.name, err)
		}
		if cls.calls != 1 {
			t.Errorf("%s: classifier calls = %d, want 1 (reuse must be refused)", c.name, cls.calls)
		}
	}
}

func TestRefresh_PromotesCompletesAndStamps(t *testing.T) {
	t.Parallel()
	candidates := []string{"Gemini 3.5 Flash (Medium)", "Gemini 3.5 Pro (High)", "Gemini 3.1 Pro (High)"}
	cls := &countingClassifier{inner: fakeClassifier{}}
	cat, err := Refresh(context.Background(), RefreshDeps{
		CLIs:       []string{"agy"},
		Lister:     fakeLister{ids: map[string][]string{"agy": candidates}},
		Classifier: cls,
		Now:        fixedNow,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	entry := cat.CLIs["agy"]
	if got := entry.TierModels["deep"]; got != "Gemini 3.5 Pro (High)" {
		t.Errorf("deep = %q, want the newest Pro", got)
	}
	if got := entry.TierModels["top"]; got != "Gemini 3.5 Pro (High)" {
		t.Errorf("top = %q, want deep's promoted model", got)
	}
	wantHash := Fingerprint(FingerprintInput{CLI: "agy", Candidates: candidates, Tiers: modelcatalog.CanonicalTiers})
	if entry.CandidatesHash != wantHash {
		t.Errorf("CandidatesHash = %q, want the current decision fingerprint", entry.CandidatesHash)
	}
	cls2 := &countingClassifier{inner: fakeClassifier{}}
	_, err = Refresh(context.Background(), RefreshDeps{
		CLIs:       []string{"agy"},
		Lister:     fakeLister{ids: map[string][]string{"agy": candidates}},
		Classifier: cls2,
		Prior:      cat,
		Now:        fixedNow,
	})
	if err != nil {
		t.Fatalf("second Refresh: %v", err)
	}
	if cls2.calls != 0 {
		t.Errorf("second refresh classifier calls = %d, want 0", cls2.calls)
	}
}

type perCLIClassifier struct {
	inner Classifier
	clis  []string
}

func (r *perCLIClassifier) Classify(ctx context.Context, cli string, ids []string) (map[string]string, error) {
	r.clis = append(r.clis, cli)
	return r.inner.Classify(ctx, cli, ids)
}

func mixedTiers() map[string]string {
	return map[string]string{
		"fast": "gpt-5.5-mini", "balanced": "gpt-5.5", "deep": "gpt-5.5", "top": "gpt-5.5",
	}
}

func codexPrior() modelcatalog.Catalog {
	return priorFor("codex", []string{"gpt-5.5-mini", "gpt-5.5"}, mixedTiers(), modelcatalog.SourceLive)
}

func refreshCodex(t *testing.T, cls Classifier, ids []string, prior modelcatalog.Catalog) modelcatalog.Catalog {
	t.Helper()
	cat, err := Refresh(context.Background(), RefreshDeps{
		CLIs:       []string{"codex"},
		Lister:     fakeLister{ids: map[string][]string{"codex": ids}},
		Classifier: cls,
		Prior:      prior,
		Now:        fixedNow,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return cat
}

func TestRefresh_LineageBump_NoClassifierCall(t *testing.T) {
	t.Parallel()
	ids := []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"}
	cls := &countingClassifier{inner: fakeClassifier{}}
	cat := refreshCodex(t, cls, ids, codexPrior())
	if cls.calls != 0 {
		t.Fatalf("classifier called %d times on a version bump within one lineage, want 0", cls.calls)
	}
	entry := cat.CLIs["codex"]
	want := map[string]string{
		"fast": "gpt-5.5-mini", "balanced": "gpt-5.6", "deep": "gpt-5.6", "top": "gpt-5.6",
	}
	for tier, model := range want {
		if entry.TierModels[tier] != model {
			t.Errorf("tier %s = %q, want %q", tier, entry.TierModels[tier], model)
		}
	}
	if entry.Source != modelcatalog.SourceLive {
		t.Errorf("Source = %q, want live", entry.Source)
	}
	if len(entry.Available) != len(ids) {
		t.Errorf("Available = %v, want the current offering %v", entry.Available, ids)
	}
	wantHash := Fingerprint(FingerprintInput{CLI: "codex", Candidates: ids, Tiers: modelcatalog.CanonicalTiers})
	if entry.CandidatesHash != wantHash {
		t.Errorf("CandidatesHash = %q, want the fingerprint of the bumped offering", entry.CandidatesHash)
	}
}

func TestRefresh_LineageBump_RetiredVersionPromotesReplacement(t *testing.T) {
	t.Parallel()
	cls := &countingClassifier{inner: fakeClassifier{}}
	cat := refreshCodex(t, cls, []string{"gpt-5.5-mini", "gpt-5.6"}, codexPrior())
	if cls.calls != 0 {
		t.Fatalf("classifier called %d times when a version replaced its predecessor, want 0", cls.calls)
	}
	for _, tier := range []string{"balanced", "deep", "top"} {
		if got := cat.CLIs["codex"].TierModels[tier]; got != "gpt-5.6" {
			t.Errorf("tier %s = %q, want the replacement gpt-5.6", tier, got)
		}
	}
}

func TestRefresh_LineageBump_NextRefreshReusesByFingerprint(t *testing.T) {
	t.Parallel()
	ids := []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"}
	first := refreshCodex(t, &countingClassifier{inner: fakeClassifier{}}, ids, codexPrior())
	cls := &countingClassifier{inner: fakeClassifier{}}
	second := refreshCodex(t, cls, ids, first)
	if cls.calls != 0 {
		t.Errorf("classifier called %d times on the refresh after a bump, want 0", cls.calls)
	}
	if got := second.CLIs["codex"].TierModels["balanced"]; got != "gpt-5.6" {
		t.Errorf("balanced after second refresh = %q, want gpt-5.6", got)
	}
}

func TestRefresh_NewLineage_StillClassifies(t *testing.T) {
	t.Parallel()
	cls := &countingClassifier{inner: fakeClassifier{}}
	refreshCodex(t, cls, []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6", "o4-mini"}, codexPrior())
	if cls.calls != 1 {
		t.Errorf("classifier calls = %d, want 1 when a new lineage appeared", cls.calls)
	}
}

func TestRefresh_RemovedLineage_StillClassifies(t *testing.T) {
	t.Parallel()
	cls := &countingClassifier{inner: fakeClassifier{}}
	refreshCodex(t, cls, []string{"gpt-5.5", "gpt-5.6"}, codexPrior())
	if cls.calls != 1 {
		t.Errorf("classifier calls = %d, want 1 when a lineage vanished", cls.calls)
	}
}

func TestRefresh_LineageBump_RefusedForUnsoundPrior(t *testing.T) {
	t.Parallel()
	bumped := []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"}
	topless := map[string]string{"fast": "gpt-5.5-mini", "balanced": "gpt-5.5", "deep": "gpt-5.5"}
	cases := []struct {
		name  string
		prior modelcatalog.Catalog
	}{
		{"detect-sourced prior", priorFor("codex", []string{"gpt-5.5-mini", "gpt-5.5"}, mixedTiers(), modelcatalog.SourceDetect)},
		{"top-less prior", priorFor("codex", []string{"gpt-5.5-mini", "gpt-5.5"}, topless, modelcatalog.SourceLive)},
		{"prior without a recorded offering", priorFor("codex", nil, mixedTiers(), modelcatalog.SourceLive)},
	}
	for _, c := range cases {
		cls := &countingClassifier{inner: fakeClassifier{}}
		refreshCodex(t, cls, bumped, c.prior)
		if cls.calls != 1 {
			t.Errorf("%s: classifier calls = %d, want 1 (lineage reuse must be refused)", c.name, cls.calls)
		}
	}
}

func TestRefresh_LineageBump_ClassifiesOnlyTheCLIWithANewLineage(t *testing.T) {
	t.Parallel()
	agyPrior := priorFor("agy", []string{"gemini-3-flash", "gemini-3-pro"}, map[string]string{
		"fast": "gemini-3-flash", "balanced": "gemini-3-pro", "deep": "gemini-3-pro", "top": "gemini-3-pro",
	}, modelcatalog.SourceLive)
	prior := codexPrior()
	prior.CLIs["agy"] = agyPrior.CLIs["agy"]
	rec := &perCLIClassifier{inner: fakeClassifier{}}
	_, err := Refresh(context.Background(), RefreshDeps{
		CLIs: []string{"codex", "agy"},
		Lister: fakeLister{ids: map[string][]string{
			"codex": {"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"},
			"agy":   {"gemini-3-flash", "gemini-3-pro", "claude-sonnet-4"},
		}},
		Classifier: rec,
		Prior:      prior,
		Now:        fixedNow,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if len(rec.clis) != 1 || rec.clis[0] != "agy" {
		t.Errorf("classified CLIs = %v, want only [agy]", rec.clis)
	}
}
