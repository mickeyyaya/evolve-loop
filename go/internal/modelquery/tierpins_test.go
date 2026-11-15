package modelquery

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

var hydrogenOffer = []string{
	"Gemini 3.8 Flash (Low)",
	"Gemini 4 Hydrogen (Low)",
	"Gemini 4 Hydrogen (Medium)",
	"Gemini 4 Hydrogen (High)",
	"Gemini 4 Hydrogen (XHigh)",
}

var hydrogenPicks = map[string]string{
	"fast":     "Gemini 3.8 Flash (Low)",
	"balanced": "Gemini 4 Hydrogen (Medium)",
	"deep":     "Gemini 4 Hydrogen (High)",
	"top":      "Gemini 4 Hydrogen (XHigh)",
}

var hydrogenPins = map[string]map[string]string{
	"agy": {"fast": "Gemini 4 Hydrogen (Medium)", "balanced": "Gemini 4 Hydrogen (High)"},
}

type fixedClassifier struct {
	picks map[string]string
	calls int
}

func (f *fixedClassifier) Classify(_ context.Context, _ string, _ []string) (map[string]string, error) {
	f.calls++
	out := make(map[string]string, len(f.picks))
	for k, v := range f.picks {
		out[k] = v
	}
	return out, nil
}

func refreshAgyPinned(t *testing.T, cls Classifier, prior modelcatalog.Catalog, pins map[string]map[string]string, ids []string, log *bytes.Buffer) modelcatalog.CLIEntry {
	t.Helper()
	cat, err := Refresh(context.Background(), RefreshDeps{
		CLIs:       []string{"agy"},
		Lister:     fakeLister{ids: map[string][]string{"agy": ids}},
		Classifier: cls,
		TierPins:   pins,
		Prior:      prior,
		Now:        fixedNow,
		Log:        log,
	})
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return cat.CLIs["agy"]
}

func agyPrior(pins map[string]string, tiers map[string]string, offered []string) modelcatalog.Catalog {
	return modelcatalog.Catalog{
		FetchedAt: fixedNow(),
		CLIs: map[string]modelcatalog.CLIEntry{"agy": {
			TierModels: tiers, Available: offered, Source: modelcatalog.SourceLive,
			CandidatesHash: Fingerprint(FingerprintInput{
				CLI: "agy", Candidates: offered, Tiers: modelcatalog.CanonicalTiers, Pins: PinList(pins),
			}),
		}},
	}
}

func TestTierPins_ClassifierPathHoldsThePinnedModels(t *testing.T) {
	t.Parallel()
	entry := refreshAgyPinned(t, &fixedClassifier{picks: hydrogenPicks}, modelcatalog.Catalog{}, hydrogenPins, hydrogenOffer, &bytes.Buffer{})
	want := map[string]string{
		"fast": "Gemini 4 Hydrogen (Medium)", "balanced": "Gemini 4 Hydrogen (High)",
		"deep": "Gemini 4 Hydrogen (High)", "top": "Gemini 4 Hydrogen (XHigh)",
	}
	for tier, model := range want {
		if entry.TierModels[tier] != model {
			t.Errorf("tier %s = %q, want %q", tier, entry.TierModels[tier], model)
		}
	}
}

func TestTierPins_PinNotOfferedKeepsTheLivePickAndWarns(t *testing.T) {
	t.Parallel()
	var log bytes.Buffer
	pins := map[string]map[string]string{"agy": {"fast": "Gemini 9 Nonesuch (Medium)"}}
	entry := refreshAgyPinned(t, &fixedClassifier{picks: hydrogenPicks}, modelcatalog.Catalog{}, pins, hydrogenOffer, &log)
	if entry.TierModels["fast"] != "Gemini 3.8 Flash (Low)" {
		t.Errorf("fast = %q, want the live pick", entry.TierModels["fast"])
	}
	out := log.String()
	if !strings.Contains(out, "WARN") || !strings.Contains(out, "tier pin") || !strings.Contains(out, "Gemini 9 Nonesuch (Medium)") {
		t.Errorf("log = %q, want a WARN naming the tier pin and its model", out)
	}
}

func TestTierPins_ReusablePriorPathAlsoGetsThePins(t *testing.T) {
	t.Parallel()
	cls := &fixedClassifier{picks: hydrogenPicks}
	prior := agyPrior(hydrogenPins["agy"], hydrogenPicks, hydrogenOffer)
	entry := refreshAgyPinned(t, cls, prior, hydrogenPins, hydrogenOffer, &bytes.Buffer{})
	if cls.calls != 0 {
		t.Fatalf("classifier calls = %d, want 0 (prior reusable under unchanged pins)", cls.calls)
	}
	if entry.TierModels["fast"] != "Gemini 4 Hydrogen (Medium)" || entry.TierModels["balanced"] != "Gemini 4 Hydrogen (High)" {
		t.Errorf("tiers = %v, want the pinned models", entry.TierModels)
	}
}

func TestTierPins_RemovedPinClassifiesAgain(t *testing.T) {
	t.Parallel()
	pinned := map[string]string{
		"fast": "Gemini 4 Hydrogen (Medium)", "balanced": "Gemini 4 Hydrogen (High)",
		"deep": "Gemini 4 Hydrogen (High)", "top": "Gemini 4 Hydrogen (XHigh)",
	}
	cls := &fixedClassifier{picks: hydrogenPicks}
	prior := agyPrior(hydrogenPins["agy"], pinned, hydrogenOffer)
	entry := refreshAgyPinned(t, cls, prior, nil, hydrogenOffer, &bytes.Buffer{})
	if cls.calls != 1 {
		t.Fatalf("classifier calls = %d, want 1 after the pins were removed", cls.calls)
	}
	if entry.TierModels["fast"] != "Gemini 3.8 Flash (Low)" || entry.TierModels["balanced"] != "Gemini 4 Hydrogen (Medium)" {
		t.Errorf("tiers = %v, want the classifier picks", entry.TierModels)
	}
}

var gptOffer = []string{"gpt-5.5-mini", "gpt-5.5"}

var gptBumped = []string{"gpt-5.5-mini", "gpt-5.5", "gpt-5.6"}

func TestTierPins_RemovedPinOnCarryAcrossBumpClassifiesAgain(t *testing.T) {
	t.Parallel()
	pins := map[string]string{"fast": "gpt-5.5"}
	tiers := mixedTiers()
	tiers["fast"] = "gpt-5.5"
	cls := &countingClassifier{inner: fakeClassifier{}}
	prior := agyPrior(pins, tiers, gptOffer)
	entry := refreshAgyPinned(t, cls, prior, nil, gptBumped, &bytes.Buffer{})
	if cls.calls != 1 {
		t.Fatalf("classifier calls = %d, want 1: a removed pin must not ride the carry path", cls.calls)
	}
	if entry.TierModels["fast"] != "gpt-5.5-mini" {
		t.Errorf("fast = %q, want the classifier pick", entry.TierModels["fast"])
	}
}

func TestTierPins_CarryAcrossBumpKeepsPinsApplied(t *testing.T) {
	t.Parallel()
	pins := map[string]map[string]string{"agy": {"fast": "gpt-5.5"}}
	cls := &countingClassifier{inner: fakeClassifier{}}
	prior := agyPrior(pins["agy"], mixedTiers(), gptOffer)
	entry := refreshAgyPinned(t, cls, prior, pins, gptBumped, &bytes.Buffer{})
	if cls.calls != 0 {
		t.Fatalf("classifier calls = %d, want 0 on the carry path", cls.calls)
	}
	if entry.TierModels["fast"] != "gpt-5.5" {
		t.Errorf("fast = %q, want the pin", entry.TierModels["fast"])
	}
}

func TestTierPins_DetectFallbackIsUnchanged(t *testing.T) {
	t.Parallel()
	cat, err := Refresh(context.Background(), RefreshDeps{
		CLIs:       []string{"agy"},
		Lister:     fakeLister{errOn: map[string]bool{"agy": true}},
		Classifier: fakeClassifier{},
		Fallback:   map[string]map[string]string{"agy": {"fast": "detect-model"}},
		TierPins:   hydrogenPins,
		Now:        fixedNow,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := cat.CLIs["agy"].TierModels["fast"]; got != "detect-model" {
		t.Errorf("fast = %q, want the detect fallback", got)
	}
}

func TestPinList_SortedTierEqualsModelAndEmptyForNone(t *testing.T) {
	t.Parallel()
	got := PinList(map[string]string{"balanced": "b", "fast": "a"})
	if len(got) != 2 || got[0] != "balanced=b" || got[1] != "fast=a" {
		t.Errorf("PinList = %v", got)
	}
	if PinList(nil) != nil {
		t.Error("PinList(nil) must be nil")
	}
}

func TestFingerprint_PinsChangeTheHashOnlyWhenPresent(t *testing.T) {
	t.Parallel()
	base := FingerprintInput{CLI: "agy", Candidates: []string{"a"}, Tiers: []string{"fast"}}
	const wantUnpinned = "sha256:a6d61015362f8f20a2ac4f894ee2ed401b229ac766ee7b9d9f1eaa592e1fc8f1"
	if got := Fingerprint(base); got != wantUnpinned {
		t.Errorf("unpinned hash = %s, want %s", got, wantUnpinned)
	}
	withEmpty := base
	withEmpty.Pins = []string{}
	if Fingerprint(base) != Fingerprint(withEmpty) {
		t.Error("an empty pin list moved the hash")
	}
	pinned := base
	pinned.Pins = []string{"fast=a"}
	if Fingerprint(base) == Fingerprint(pinned) {
		t.Error("a pin did not change the hash")
	}
}
