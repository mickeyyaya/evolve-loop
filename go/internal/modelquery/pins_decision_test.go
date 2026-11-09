package modelquery

import (
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

const unpinnedClaudeHash = "sha256:34dadccd4350c2f8e94f0e47f33195feb8218ba4d343456326dac6345753293a"

var (
	gptOffering       = []string{"gpt-5.5-mini", "gpt-5.5"}
	gptBumpedOffering = []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"}
	gptClassifier     = fixedClassifier{"fast": "gpt-5.5-mini", "balanced": "gpt-5.5", "deep": "gpt-5.5", "top": "gpt-5.5"}
)

func refreshGPT(t *testing.T, cls Classifier, ids []string, prior modelcatalog.Catalog, pins map[string]string) modelcatalog.Catalog {
	t.Helper()
	cat, err := Refresh(context.Background(), RefreshDeps{
		CLIs:       []string{"codex"},
		Lister:     fakeLister{ids: map[string][]string{"codex": ids}},
		Classifier: cls,
		Prior:      prior,
		Pins:       map[string]map[string]string{"codex": pins},
		Now:        fixedNow,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return cat
}

func TestRefresh_APinnedPriorCarriesAcrossAVersionBump(t *testing.T) {
	t.Parallel()
	pins := map[string]string{"fast": "gpt-5.5-mini"}
	prior := refreshGPT(t, gptClassifier, gptOffering, modelcatalog.Catalog{}, pins)
	cls := &countingClassifier{inner: gptClassifier}

	bumped := refreshGPT(t, cls, gptBumpedOffering, prior, pins)

	if cls.calls != 0 {
		t.Fatalf("classifier calls = %d, want 0: a prior with the same pins carries across a version bump", cls.calls)
	}
	if model, _ := bumped.DispatchModel("codex", "deep"); model != "gpt-5.6" {
		t.Errorf("deep = %q, want the promoted gpt-5.6", model)
	}
}

func TestRefresh_AChangedPinDoesNotCarryAPriorAcrossAVersionBump(t *testing.T) {
	t.Parallel()
	prior := refreshGPT(t, gptClassifier, gptOffering, modelcatalog.Catalog{}, map[string]string{"fast": "gpt-5.5-mini"})
	cls := &countingClassifier{inner: gptClassifier}

	refreshGPT(t, cls, gptBumpedOffering, prior, map[string]string{"balanced": "gpt-5.5-mini"})

	if cls.calls != 1 {
		t.Fatalf("classifier calls = %d, want 1: a prior from other pins is not carried", cls.calls)
	}
}

func TestRefresh_AReusedPriorStillWarnsOfItsUnmetPin(t *testing.T) {
	t.Parallel()
	deps := RefreshDeps{
		Lister:     fakeLister{ids: map[string][]string{"claude": {"opus", "sonnet", "haiku"}}},
		Classifier: opusDeepClassifier,
		Pins:       map[string]map[string]string{"claude": {"deep": "fable"}},
	}
	prior, _ := refreshClaude(t, deps)
	cls := &countingClassifier{inner: opusDeepClassifier}
	deps.Classifier, deps.Prior = cls, prior

	_, log := refreshClaude(t, deps)

	if cls.calls != 0 {
		t.Fatalf("classifier calls = %d, want 0 as the precondition: the prior is reused", cls.calls)
	}
	if !strings.Contains(log, "WARN") || !strings.Contains(log, "deep=fable") {
		t.Errorf("a reused prior hid the unmet pin:\n%s", log)
	}
}

func TestPinTiers_DoesNotChangeTheMapOfTheCaller(t *testing.T) {
	t.Parallel()
	tiers := map[string]string{"deep": "opus", "fast": "haiku"}

	pinned, unmet := pinTiers(tiers, map[string]string{"deep": "fable"}, claudeOffering)

	if tiers["deep"] != "opus" {
		t.Errorf("the input map moved to %q; a reused prior holds this map", tiers["deep"])
	}
	if pinned["deep"] != "fable" || pinned["fast"] != "haiku" || len(unmet) != 0 {
		t.Errorf("pinTiers = %v, %v; want deep on fable and fast kept", pinned, unmet)
	}
}

func TestFingerprint_TheModelOfAPinReachesTheHash(t *testing.T) {
	t.Parallel()
	in := FingerprintInput{CLI: "claude", Candidates: claudeOffering, Tiers: modelcatalog.CanonicalTiers}
	onOpus, onFable := in, in
	onOpus.Pins = map[string]string{"deep": "opus"}
	onFable.Pins = map[string]string{"deep": "fable"}

	if Fingerprint(onOpus) == Fingerprint(onFable) {
		t.Error("a pin that moves from opus to fable kept the hash; the stale prior is then reused")
	}
}

func TestFingerprint_AnUnpinnedDecisionKeepsItsHashFromBeforeThePins(t *testing.T) {
	t.Parallel()
	in := FingerprintInput{CLI: "claude", Candidates: claudeOffering, Tiers: modelcatalog.CanonicalTiers}

	if got := Fingerprint(in); got != unpinnedClaudeHash {
		t.Errorf("Fingerprint = %s, want %s: a moved hash makes each unpinned CLI reclassify", got, unpinnedClaudeHash)
	}
}
