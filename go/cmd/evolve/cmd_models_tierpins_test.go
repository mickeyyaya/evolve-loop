package main

import (
	"bytes"
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/mickeyyaya/evolve-loop/go/internal/modelcatalog"
	"github.com/mickeyyaya/evolve-loop/go/internal/modelquery"
)

type pinLister struct{ ids []string }

func (l pinLister) List(context.Context, string) ([]string, error) { return l.ids, nil }

type pinClassifier struct{ picks map[string]string }

func (c pinClassifier) Classify(context.Context, string, []string) (map[string]string, error) {
	return c.picks, nil
}

func TestPolicyCatalog_ReadsTheCatalogBlock(t *testing.T) {
	t.Parallel()
	root := strictPolicyRoot(t, `{"catalog": {"tier_pins": {"agy": {"fast": "M"}}}}`)
	if got := policyCatalog(root).TierPins; !reflect.DeepEqual(got, map[string]map[string]string{"agy": {"fast": "M"}}) {
		t.Fatalf("tier pins = %v", got)
	}
}

func TestLiveRefreshDeps_CarriesTheCatalogPolicyIntoTheDeps(t *testing.T) {
	t.Parallel()
	root := strictPolicyRoot(t, `{"catalog": {"allowed_families": {"agy": ["gemini"]}, "tier_pins": {"agy": {"fast": "Gemini 4 Hydrogen (Medium)"}}}}`)
	lister, classifier := pinLister{ids: []string{"x"}}, pinClassifier{}
	prior := modelcatalog.Catalog{FetchedAt: time.Unix(1, 0)}
	fb := map[string]map[string]string{"claude": {"fast": "haiku"}}
	deps := liveRefreshDeps(liveRefreshInput{ProjectRoot: root, CLIs: []string{"agy", "claude"}, Fallback: fb, Prior: prior, Lister: lister, Classifier: classifier, Log: &bytes.Buffer{}})
	if !reflect.DeepEqual(deps.Fallback, fb) || !deps.Freshness["claude"].PreferAlias {
		t.Errorf("deps lost the fallback or the freshness policy: %+v", deps)
	}
	if !reflect.DeepEqual(deps.AllowedFamilies, map[string][]string{"agy": {"gemini"}}) {
		t.Errorf("AllowedFamilies = %v", deps.AllowedFamilies)
	}
	if !reflect.DeepEqual(deps.TierPins, map[string]map[string]string{"agy": {"fast": "Gemini 4 Hydrogen (Medium)"}}) {
		t.Errorf("TierPins = %v", deps.TierPins)
	}
	if !reflect.DeepEqual(deps.Lister, lister) || !reflect.DeepEqual(deps.Classifier, classifier) || !deps.Prior.FetchedAt.Equal(prior.FetchedAt) {
		t.Errorf("deps lost the lister, classifier or prior: %+v", deps)
	}
	if !reflect.DeepEqual(deps.CLIs, []string{"agy", "claude"}) || deps.Now == nil || deps.Log == nil || deps.EffortListers == nil {
		t.Errorf("deps lost the CLIs, clock, log or effort listers: %+v", deps)
	}
}

func TestLiveRefreshDeps_ThePolicyPinsReachTheRefreshedCatalog(t *testing.T) {
	t.Parallel()
	root := strictPolicyRoot(t, `{"catalog": {"tier_pins": {"agy": {"fast": "Gemini 4 Hydrogen (Medium)"}}}}`)
	offer := []string{"Gemini 3.8 Flash (Low)", "Gemini 4 Hydrogen (Medium)"}
	picks := map[string]string{"fast": offer[0], "balanced": offer[1], "deep": offer[1], "top": offer[1]}
	deps := liveRefreshDeps(liveRefreshInput{ProjectRoot: root, CLIs: []string{"agy"}, Lister: pinLister{ids: offer}, Classifier: pinClassifier{picks: picks}, Log: &bytes.Buffer{}})
	cat, err := modelquery.Refresh(context.Background(), deps)
	if err != nil {
		t.Fatal(err)
	}
	if got := cat.CLIs["agy"].TierModels["fast"]; got != "Gemini 4 Hydrogen (Medium)" {
		t.Errorf("fast = %q, want the policy pin", got)
	}
}
