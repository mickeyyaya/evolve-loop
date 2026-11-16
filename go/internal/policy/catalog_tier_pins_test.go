package policy

import (
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestLoad_ParsesCatalogTierPins(t *testing.T) {
	p, err := Load(writePolicy(t, t.TempDir(), `{"catalog":{"tier_pins":{"agy":{"fast":"M1","balanced":"M2"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string]string{"agy": {"fast": "M1", "balanced": "M2"}}
	if got := p.CatalogConfig().TierPins; !reflect.DeepEqual(got, want) {
		t.Errorf("TierPins = %v, want %v", got, want)
	}
}

func TestLoad_RefusesAnUnknownTierInTierPins(t *testing.T) {
	_, err := Load(writePolicy(t, t.TempDir(), `{"catalog":{"tier_pins":{"agy":{"ultra":"M1"}}}}`))
	if err == nil || !strings.Contains(err.Error(), "tier_pins") || !strings.Contains(err.Error(), "ultra") {
		t.Fatalf("err = %v, want a refusal naming tier_pins and the tier", err)
	}
}

func TestLoad_RefusesAnEmptyModelInTierPins(t *testing.T) {
	_, err := Load(writePolicy(t, t.TempDir(), `{"catalog":{"tier_pins":{"agy":{"fast":""}}}}`))
	if err == nil || !strings.Contains(err.Error(), "tier_pins") || !strings.Contains(err.Error(), "fast") {
		t.Fatalf("err = %v, want a refusal naming tier_pins and the tier", err)
	}
}

func TestCatalogConfig_TierPinsAbsentIsNil(t *testing.T) {
	if got := (Policy{}).CatalogConfig().TierPins; got != nil {
		t.Errorf("TierPins = %v, want nil", got)
	}
	if got := (Policy{Catalog: &CatalogPolicy{}}).CatalogConfig().TierPins; got != nil {
		t.Errorf("TierPins = %v, want nil", got)
	}
}

func TestTheCheckedInPolicyPinsTheAgyFastAndBalancedTiers(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	p, err := Load(filepath.Join(filepath.Dir(thisFile), "..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"fast": "Gemini 4 Hydrogen (Medium)", "balanced": "Gemini 4 Hydrogen (High)"}
	if got := p.CatalogConfig().TierPins["agy"]; !reflect.DeepEqual(got, want) {
		t.Errorf("agy tier_pins = %v, want %v", got, want)
	}
}
