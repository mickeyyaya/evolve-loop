package modelquery

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

func TestRefresh_LineageSwap_StillClassifies(t *testing.T) {
	t.Parallel()
	cls := &countingClassifier{inner: fakeClassifier{}}
	refreshCodex(t, cls, []string{"gpt-5.5", "gpt-5.6", "o4-mini"}, codexPrior())
	if cls.calls != 1 {
		t.Errorf("classifier calls = %d, want 1 when one lineage was swapped for another of the same count", cls.calls)
	}
}

func TestRefresh_LineageBump_TierOutsideOfferingStillClassifies(t *testing.T) {
	t.Parallel()
	tiers := mixedTiers()
	tiers["fast"] = "gpt-5.5-nano"
	prior := priorFor("codex", []string{"gpt-5.5-mini", "gpt-5.5"}, tiers, modelcatalog.SourceLive)
	cls := &countingClassifier{inner: fakeClassifier{}}
	cat, err := Refresh(context.Background(), RefreshDeps{
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
		t.Errorf("classifier calls = %d, want 1 when a carried tier model is not offered", cls.calls)
	}
	if got := cat.CLIs["codex"].TierModels["fast"]; got == "gpt-5.5-nano" {
		t.Errorf("fast = %q, an unoffered model was carried over", got)
	}
}
