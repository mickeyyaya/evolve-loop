package modelquery

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

// countingClassifier wraps fakeClassifier and counts Classify invocations —
// the headline stability assertion is calls == 0 on an unchanged offering.
type countingClassifier struct {
	inner Classifier
	calls int
}

func (c *countingClassifier) Classify(ctx context.Context, cli string, ids []string) (map[string]string, error) {
	c.calls++
	return c.inner.Classify(ctx, cli, ids)
}

// fullTiers is a prior tier map covering every canonical tier (the reuse
// gate's coverage condition).
func fullTiers(model string) map[string]string {
	out := make(map[string]string, len(modelcatalog.CanonicalTiers))
	for _, tier := range modelcatalog.CanonicalTiers {
		out[tier] = model
	}
	return out
}

// priorFor builds a prior catalog whose CLI entry matches what the current
// pipeline would have written for candidates: live-sourced, full tier
// coverage, and the CURRENT fingerprint of the decision inputs.
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

// TestRefresh_UnchangedOfferingSkipsClassifier: an identical candidate list
// with a live, fully-covered prior reuses the prior tier map with ZERO
// classifier LLM calls — the stability fix for the documented flap where an
// identical agy list reclassified Sonnet-4.6 → GPT-OSS-120B.
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

// TestRefresh_ChangedOfferingClassifies is the anti-no-op twin: one added id
// invalidates the fingerprint and the classifier runs exactly once.
func TestRefresh_ChangedOfferingClassifies(t *testing.T) {
	t.Parallel()
	prior := priorFor("codex", []string{"gpt-5.5-mini", "gpt-5.5"}, fullTiers("gpt-5.5"), modelcatalog.SourceLive)
	cls := &countingClassifier{inner: fakeClassifier{}}
	_, err := Refresh(context.Background(), RefreshDeps{
		CLIs:       []string{"codex"},
		Lister:     fakeLister{ids: map[string][]string{"codex": {"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"}}},
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

// TestRefresh_ReuseRequiresLiveAndFullCoverage: a hash match alone is not
// enough. A detect-sourced prior must never be laundered into an
// authoritative entry, and a prior missing a canonical tier (the pre-fix
// top-less shape) must reclassify once rather than stay sticky forever.
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

// TestRefresh_PromotesCompletesAndStamps: a fresh classification is promoted
// within lineage (stale Pro → newest Pro, never Flash), completed to full
// canonical-tier coverage, and stamped with the decision fingerprint so the
// NEXT refresh can reuse it.
func TestRefresh_PromotesCompletesAndStamps(t *testing.T) {
	t.Parallel()
	// fakeClassifier: fast=first, balanced=middle, deep=last (no top).
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
	// deep was classified as the stale "Gemini 3.1 Pro (High)" (last id) and
	// must be promoted to the 3.5 Pro — never the higher-versioned Flash.
	if got := entry.TierModels["deep"]; got != "Gemini 3.5 Pro (High)" {
		t.Errorf("deep = %q, want the newest Pro", got)
	}
	// top was absent from the classifier reply and fills from deep.
	if got := entry.TierModels["top"]; got != "Gemini 3.5 Pro (High)" {
		t.Errorf("top = %q, want deep's promoted model", got)
	}
	wantHash := Fingerprint(FingerprintInput{CLI: "agy", Candidates: candidates, Tiers: modelcatalog.CanonicalTiers})
	if entry.CandidatesHash != wantHash {
		t.Errorf("CandidatesHash = %q, want the current decision fingerprint", entry.CandidatesHash)
	}
	// A second refresh over the identical offering now reuses.
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
