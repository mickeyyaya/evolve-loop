package setup

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
)

func TestComputeLatest_NewerVersionInLineageIsStale(t *testing.T) {
	t.Parallel()
	latest, stale, observed := ComputeLatest("gpt-5.4",
		[]string{"gpt-5.5", "gpt-5.4", "gpt-4o", "o4-mini"}, modelquery.FreshnessPolicy{})
	if latest != "gpt-5.5" || !stale || !observed {
		t.Errorf("live gpt-5.5 in the gpt-5 lineage must mark the gpt-5.4 map stale; got latest=%q stale=%v observed=%v", latest, stale, observed)
	}
}

func TestComputeLatest_CurrentAlreadyFreshestIsNotStale(t *testing.T) {
	t.Parallel()
	latest, stale, observed := ComputeLatest("gpt-5.5",
		[]string{"gpt-5.4", "gpt-5.5", "gpt-4o"}, modelquery.FreshnessPolicy{})
	if latest != "gpt-5.5" || stale || !observed {
		t.Errorf("nothing fresher live ⇒ not stale; got latest=%q stale=%v observed=%v", latest, stale, observed)
	}
}

func TestComputeLatest_OtherLineageNeverPromotes(t *testing.T) {
	t.Parallel()
	if latest, stale, observed := ComputeLatest("gpt-5.5", []string{"o5", "claude-x"}, modelquery.FreshnessPolicy{}); stale || latest != "gpt-5.5" || observed {
		t.Errorf("cross-lineage candidates must not promote — and a never-seen current is NOT observed; got latest=%q stale=%v observed=%v", latest, stale, observed)
	}
}

func TestComputeLatest_AliasPreferredWhenPolicySaysSo(t *testing.T) {
	t.Parallel()
	fp := modelquery.FreshnessPolicy{PreferAlias: true, AliasIDs: []string{"opus"}}
	latest, stale, observed := ComputeLatest("opus-4.6", []string{"opus", "opus-4.5"}, fp)
	if latest != "opus" || !stale || !observed {
		t.Errorf("alias must win under PreferAlias; got latest=%q stale=%v observed=%v", latest, stale, observed)
	}
}

func TestComputeLatest_EmptyCurrentIsInert(t *testing.T) {
	t.Parallel()
	if latest, stale, observed := ComputeLatest("", []string{"gpt-5.5"}, modelquery.FreshnessPolicy{}); latest != "" || stale || observed {
		t.Errorf("no current model ⇒ nothing to compare; got latest=%q stale=%v observed=%v", latest, stale, observed)
	}
}

func TestComputeLatest_TierWordFallbackIsNotObserved(t *testing.T) {
	t.Parallel()
	if _, stale, observed := ComputeLatest("deep", []string{"qwen3", "llama4"}, modelquery.FreshnessPolicy{}); stale || observed {
		t.Errorf("a tier word matches no live model and must not be judged; stale=%v observed=%v", stale, observed)
	}
}

func TestFamilyLatestAndLatestReport_AreTheWireShape(t *testing.T) {
	t.Parallel()
	rep := LatestReport{Source: "live", CLIs: []FamilyLatest{{
		CLI: "codex", CurrentDeepModel: "gpt-5.5", LatestModel: "gpt-5.6",
		CurrentSeenLive: true, StaleTiers: []TierStale{{Tier: "deep", Current: "gpt-5.5", Latest: "gpt-5.6"}},
		UnverifiedTiers: []string{"balanced"},
		MapStale:        true, Candidates: 7,
	}}}
	raw, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"clis"`, `"source"`, `"cli"`, `"current_deep_model"`, `"latest_model"`, `"map_stale"`, `"candidates"`, `"current_seen_live"`, `"stale_tiers"`, `"tier"`, `"current"`, `"latest"`, `"unverified_tiers"`} {
		if !strings.Contains(string(raw), key) {
			t.Errorf("wire lost key %s: %s", key, raw)
		}
	}
	if errRow, _ := json.Marshal(FamilyLatest{CLI: "agy", Error: "probe failed"}); !strings.Contains(string(errRow), `"error"`) {
		t.Errorf("a failed probe must carry its error on the wire: %s", errRow)
	}
}
