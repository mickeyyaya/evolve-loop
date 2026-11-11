package policy_test

import (
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestHermeticMode_DefaultsToShadow(t *testing.T) {
	t.Parallel()
	for name, raw := range map[string]string{"no policy": `{}`, "no gates": `{"gates":{}}`, "empty hermetic": `{"gates":{"hermetic":{}}}`} {
		pol, err := policy.Parse([]byte(raw))
		if err != nil {
			t.Fatalf("%s: Parse: %v", name, err)
		}
		if got := pol.HermeticMode(); got != policy.HermeticShadow {
			t.Errorf("%s: HermeticMode() = %q, want shadow", name, got)
		}
	}
	if got := (policy.Policy{}).HermeticMode(); got != policy.HermeticShadow {
		t.Errorf("zero Policy HermeticMode() = %q, want shadow", got)
	}
}

func TestHermeticMode_ReadsEachKnownMode(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{policy.HermeticOff, policy.HermeticShadow, policy.HermeticEnforce} {
		pol, err := policy.Parse([]byte(`{"gates":{"hermetic":{"mode":"` + mode + `"}}}`))
		if err != nil {
			t.Fatalf("Parse(%s): %v", mode, err)
		}
		if got := pol.HermeticMode(); got != mode {
			t.Errorf("HermeticMode() = %q, want %q", got, mode)
		}
	}
}

func TestHermeticMode_AnUnknownModeAndAnUnknownKeyAreValidationErrors(t *testing.T) {
	t.Parallel()
	for raw, want := range map[string]string{
		`{"gates":{"hermetic":{"mode":"strict"}}}`:        `"strict"`,
		`{"gates":{"hermetic":{"mode":"enforce","x":1}}}`: "x",
		`{"gates":{"hermetic":{"mode":3}}}`:               "mode",
	} {
		_, err := policy.Parse([]byte(raw))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Parse(%s) = %v, want an error naming %s", raw, err, want)
		}
	}
}

func TestHermeticModeValue_UnmarshalTextKeepsAKnownModeAndRefusesAnother(t *testing.T) {
	t.Parallel()
	var mode policy.HermeticModeValue
	if err := mode.UnmarshalText([]byte(policy.HermeticEnforce)); err != nil || mode != policy.HermeticEnforce {
		t.Errorf("UnmarshalText(enforce) = %v, mode %q", err, mode)
	}
	if err := mode.UnmarshalText([]byte("strict")); err == nil || mode != policy.HermeticEnforce {
		t.Errorf("UnmarshalText(strict) = %v, mode %q: an unknown mode must fail and keep the old value", err, mode)
	}
	pol := policy.Policy{Gates: &policy.GatesPolicy{Hermetic: &policy.HermeticPolicy{Mode: mode}}}
	if got := pol.HermeticMode(); got != policy.HermeticEnforce {
		t.Errorf("HermeticMode() = %q, want enforce", got)
	}
}
