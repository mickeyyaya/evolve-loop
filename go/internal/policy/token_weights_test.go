package policy

import (
	"errors"
	"os"
	"strings"
	"testing"
)

var defaultWeights = TokenWeight{Input: 1, Output: 5, CacheRead: 0.1, CacheWrite: 1.25}

func TestTokenWeights_ClaudeFamilyKeepsTheConfiguredWeight(t *testing.T) {
	t.Parallel()
	p, err := Parse([]byte(`{"telemetry":{"token_weights":{"claude":{"output":7}}}}`))
	if err != nil {
		t.Fatalf("Parse = %v", err)
	}
	var block *TelemetryPolicy = p.Telemetry
	var weights *TokenWeightsPolicy = block.TokenWeights
	var claude *TokenWeightOverride = weights.Claude
	if *claude.Output != 7 || claude.Input != nil {
		t.Fatalf("the claude block = %+v, want only output 7", claude)
	}
}

func TestTokenWeights_NoKeyGivesTheDefaults(t *testing.T) {
	t.Parallel()
	for _, body := range []string{`{}`, `{"telemetry":{}}`, `{"telemetry":{"token_weights":{}}}`} {
		p, err := Parse([]byte(body))
		if err != nil {
			t.Fatalf("%s: Parse = %v", body, err)
		}
		for _, family := range []string{"claude", "agy"} {
			if got, err := p.TokenWeightsFor(family); err != nil || got != defaultWeights {
				t.Errorf("%s %s: TokenWeightsFor = %+v, %v; want %+v", body, family, got, err, defaultWeights)
			}
		}
	}
}

func TestTokenWeights_PartialFamilyKeepsTheOtherDefaults(t *testing.T) {
	t.Parallel()
	p, err := Parse([]byte(`{"telemetry":{"token_weights":{"claude":{"output":7,"cache_read":0}}}}`))
	if err != nil {
		t.Fatalf("Parse = %v", err)
	}
	want := TokenWeight{Input: 1, Output: 7, CacheRead: 0, CacheWrite: 1.25}
	if got, err := p.TokenWeightsFor("claude"); err != nil || got != want {
		t.Errorf("claude = %+v, %v; want %+v", got, err, want)
	}
	if got, err := p.TokenWeightsFor("agy"); err != nil || got != defaultWeights {
		t.Errorf("agy = %+v, %v; want the defaults", got, err)
	}
}

func TestTokenWeights_EachOverrideLandsInItsOwnField(t *testing.T) {
	t.Parallel()
	p, err := Parse([]byte(`{"telemetry":{"token_weights":{"agy":{"input":2,"output":7,"cache_read":0.5,"cache_write":3}}}}`))
	if err != nil {
		t.Fatalf("Parse = %v", err)
	}
	want := TokenWeight{Input: 2, Output: 7, CacheRead: 0.5, CacheWrite: 3}
	if got, err := p.TokenWeightsFor("agy"); err != nil || got != want {
		t.Errorf("agy = %+v, %v; want %+v", got, err, want)
	}
	if got, err := p.TokenWeightsFor("claude"); err != nil || got != defaultWeights {
		t.Errorf("claude = %+v, %v; want the defaults", got, err)
	}
}

func TestTokenWeightsFor_UnknownFamilyErrors(t *testing.T) {
	t.Parallel()
	_, err := Policy{}.TokenWeightsFor("agy-claude")
	if !errors.Is(err, ErrUnknownKey) {
		t.Errorf("err = %v, want ErrUnknownKey", err)
	}
	if _, known := (Policy{}).TokenWeightsFor("claude"); known != nil {
		t.Errorf("claude must stay a known family, got %v", known)
	}
}

func TestTokenWeights_PolicyConfigDocListsTheDefaultsAndFamilies(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("../../../docs/architecture/policy-config.md")
	if err != nil {
		t.Fatal(err)
	}
	_, section, ok := strings.Cut(string(raw), "## Token weights")
	if !ok {
		t.Fatal("policy-config.md has no Token weights section")
	}
	section, _, _ = strings.Cut(section, "\n## ")
	for _, want := range append(tokenWeightFamilies(), "1.25", "0.1", ".input`", ".output`", ".cache_read`", ".cache_write`") {
		if !strings.Contains(section, want) {
			t.Errorf("Token weights section does not mention %q", want)
		}
	}
}
