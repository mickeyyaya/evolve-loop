package modelquery

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
)

var claudeOffering = []string{"fable", "opus", "sonnet", "haiku"}

type fixedClassifier map[string]string

func (f fixedClassifier) Classify(context.Context, string, []string) (map[string]string, error) {
	out := make(map[string]string, len(f))
	for tier, model := range f {
		out[tier] = model
	}
	return out, nil
}

var opusDeepClassifier = fixedClassifier{"fast": "haiku", "balanced": "sonnet", "deep": "opus", "top": "opus"}

func refreshClaude(t *testing.T, deps RefreshDeps) (modelcatalog.Catalog, string) {
	t.Helper()
	var log bytes.Buffer
	deps.CLIs = []string{"claude"}
	deps.Now = fixedNow
	deps.Log = &log
	cat, err := Refresh(context.Background(), deps)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	return cat, log.String()
}

func TestParseClaudePicker_AFableRowIsOfferedAsFable(t *testing.T) {
	t.Parallel()
	pane := strings.Join([]string{
		"  Select model",
		"  ❯ 1. Default (recommended) ✔  Fable 5.2 · Most capable for the hardest work",
		"    2. Opus                     Opus 5.5 · Most capable for complex work",
		"    3. Sonnet                   Sonnet 5.5 · Best for everyday tasks",
		"    4. Haiku                    Haiku 5.5 · Fastest for quick answers",
	}, "\n")

	got := parseClaudePicker(pane)

	assertIDs(t, got, claudeOffering)
}

func TestRefresh_APinnedTierOverridesTheClassifierWhenItsModelIsOffered(t *testing.T) {
	t.Parallel()
	cat, _ := refreshClaude(t, RefreshDeps{
		Lister:     fakeLister{ids: map[string][]string{"claude": claudeOffering}},
		Classifier: opusDeepClassifier,
		Pins:       map[string]map[string]string{"claude": {"deep": "fable", "top": "fable"}},
	})

	for _, tier := range []string{"deep", "top"} {
		if model, ok := cat.DispatchModel("claude", tier); !ok || model != "fable" {
			t.Errorf("DispatchModel(claude, %s) = (%q, %v), want the pinned fable", tier, model, ok)
		}
	}
	if model, _ := cat.DispatchModel("claude", "balanced"); model != "sonnet" {
		t.Errorf("balanced = %q, want the classifier's sonnet: a pin moves only its own tier", model)
	}
}

func TestRefresh_APinWhoseModelIsNotOfferedKeepsTheClassifierPickAndWarns(t *testing.T) {
	t.Parallel()
	cat, log := refreshClaude(t, RefreshDeps{
		Lister:     fakeLister{ids: map[string][]string{"claude": {"opus", "sonnet", "haiku"}}},
		Classifier: opusDeepClassifier,
		Pins:       map[string]map[string]string{"claude": {"deep": "fable"}},
	})

	if model, ok := cat.DispatchModel("claude", "deep"); !ok || model != "opus" {
		t.Errorf("DispatchModel(claude, deep) = (%q, %v), want opus: an unoffered pin never launches a model the CLI does not list", model, ok)
	}
	for _, want := range []string{"WARN", "claude", "deep", "fable"} {
		if !strings.Contains(log, want) {
			t.Errorf("the unmet pin lost %q from its WARN:\n%s", want, log)
		}
	}
}

func TestRefresh_AnAddedPinReplacesAReusedPrior(t *testing.T) {
	t.Parallel()
	unpinned, _ := refreshClaude(t, RefreshDeps{
		Lister:     fakeLister{ids: map[string][]string{"claude": claudeOffering}},
		Classifier: opusDeepClassifier,
	})
	if model, _ := unpinned.DispatchModel("claude", "deep"); model != "opus" {
		t.Fatalf("unpinned deep = %q, want opus as the precondition", model)
	}

	pinned, _ := refreshClaude(t, RefreshDeps{
		Lister:     fakeLister{ids: map[string][]string{"claude": claudeOffering}},
		Classifier: opusDeepClassifier,
		Prior:      unpinned,
		Pins:       map[string]map[string]string{"claude": {"deep": "fable"}},
	})

	if model, _ := pinned.DispatchModel("claude", "deep"); model != "fable" {
		t.Errorf("deep = %q, want fable: a prior from before the pin never hides it", model)
	}
	if pinned.CLIs["claude"].CandidatesHash == unpinned.CLIs["claude"].CandidatesHash {
		t.Error("CandidatesHash is unchanged; the pins are a decision input")
	}
}

func TestRefresh_ARemovedPinReclassifies(t *testing.T) {
	t.Parallel()
	pinned, _ := refreshClaude(t, RefreshDeps{
		Lister:     fakeLister{ids: map[string][]string{"claude": claudeOffering}},
		Classifier: opusDeepClassifier,
		Pins:       map[string]map[string]string{"claude": {"deep": "fable"}},
	})
	cls := &countingClassifier{inner: opusDeepClassifier}

	unpinned, _ := refreshClaude(t, RefreshDeps{
		Lister:     fakeLister{ids: map[string][]string{"claude": claudeOffering}},
		Classifier: cls,
		Prior:      pinned,
	})

	if cls.calls != 1 {
		t.Fatalf("classifier calls = %d, want 1: the pinned prior is not reusable without the pin", cls.calls)
	}
	if model, _ := unpinned.DispatchModel("claude", "deep"); model != "opus" {
		t.Errorf("deep = %q, want the classifier's opus after the pin is removed", model)
	}
}

func TestFingerprint_NoPinsKeepsTheHashOfAnUnpinnedDecision(t *testing.T) {
	t.Parallel()
	base := FingerprintInput{CLI: "claude", Candidates: claudeOffering, Tiers: modelcatalog.CanonicalTiers}
	empty := base
	empty.Pins = map[string]string{}
	pinned := base
	pinned.Pins = map[string]string{"deep": "fable"}
	other := base
	other.Pins = map[string]string{"top": "fable"}

	if Fingerprint(base) != Fingerprint(empty) {
		t.Error("an empty pin set changed the hash; every unpinned CLI would reclassify")
	}
	if Fingerprint(base) == Fingerprint(pinned) || Fingerprint(pinned) == Fingerprint(other) {
		t.Error("the pinned tier and model must both reach the hash")
	}
}
