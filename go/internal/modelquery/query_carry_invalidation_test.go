package modelquery

import (
	"context"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

var aliasPreferringPolicy = FreshnessPolicy{PreferAlias: true, AliasIDs: []string{"gpt-5.6"}}

func priorUnderPolicy(p FreshnessPolicy) modelcatalog.Catalog {
	prior := codexPrior()
	entry := prior.CLIs["codex"]
	entry.CandidatesHash = Fingerprint(FingerprintInput{
		CLI: "codex", Candidates: entry.Available, Policy: p, Tiers: modelcatalog.CanonicalTiers,
	})
	prior.CLIs["codex"] = entry
	return prior
}

func refreshCodexUnderPolicy(t *testing.T, cls Classifier, ids []string, prior modelcatalog.Catalog, p FreshnessPolicy) modelcatalog.Catalog {
	t.Helper()
	cat, err := Refresh(context.Background(), RefreshDeps{
		CLIs:       []string{"codex"},
		Lister:     fakeLister{ids: map[string][]string{"codex": ids}},
		Classifier: cls,
		Prior:      prior,
		Freshness:  map[string]FreshnessPolicy{"codex": p},
		Now:        fixedNow,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return cat
}

func TestRefresh_LineageBump_FreshnessPolicyChangeStillClassifies(t *testing.T) {
	t.Parallel()
	ids := []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"}
	cls := &countingClassifier{inner: fakeClassifier{}}
	cat := refreshCodexUnderPolicy(t, cls, ids, priorUnderPolicy(FreshnessPolicy{}), aliasPreferringPolicy)
	if cls.calls != 1 {
		t.Fatalf("classifier calls = %d, want 1 when the freshness policy changed together with a version bump", cls.calls)
	}
	wantHash := Fingerprint(FingerprintInput{
		CLI: "codex", Candidates: ids, Policy: aliasPreferringPolicy, Tiers: modelcatalog.CanonicalTiers,
	})
	if got := cat.CLIs["codex"].CandidatesHash; got != wantHash {
		t.Errorf("CandidatesHash = %q, want the fingerprint under the new policy", got)
	}
}

func TestRefresh_LineageBump_UnchangedNonDefaultPolicyStillCarries(t *testing.T) {
	t.Parallel()
	ids := []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"}
	cls := &countingClassifier{inner: fakeClassifier{}}
	cat := refreshCodexUnderPolicy(t, cls, ids, priorUnderPolicy(aliasPreferringPolicy), aliasPreferringPolicy)
	if cls.calls != 0 {
		t.Fatalf("classifier calls = %d, want 0 when only the candidates changed under an unchanged policy", cls.calls)
	}
	if got := cat.CLIs["codex"].TierModels["balanced"]; got != "gpt-5.6" {
		t.Errorf("balanced = %q, want gpt-5.6", got)
	}
}

func TestRefresh_LineageBump_PriorHashNotDerivedFromItsOfferingStillClassifies(t *testing.T) {
	t.Parallel()
	ids := []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"}
	cases := map[string]string{
		"hash of an older decision version": "sha256:hash-written-under-an-older-decision-version",
		"empty hash":                        "",
	}
	for name, hash := range cases {
		prior := codexPrior()
		entry := prior.CLIs["codex"]
		entry.CandidatesHash = hash
		prior.CLIs["codex"] = entry
		cls := &countingClassifier{inner: fakeClassifier{}}
		cat := refreshCodex(t, cls, ids, prior)
		if cls.calls != 1 {
			t.Errorf("%s: classifier calls = %d, want 1", name, cls.calls)
		}
		wantHash := Fingerprint(FingerprintInput{CLI: "codex", Candidates: ids, Tiers: modelcatalog.CanonicalTiers})
		if got := cat.CLIs["codex"].CandidatesHash; got != wantHash {
			t.Errorf("%s: CandidatesHash = %q, want the current fingerprint", name, got)
		}
	}
}
