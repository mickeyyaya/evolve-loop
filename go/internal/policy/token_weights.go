package policy

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
)

var defaultTokenWeight = TokenWeight{Input: 1, Output: 5, CacheRead: 0.1, CacheWrite: 1.25}

type TokenWeight struct {
	Input      float64
	Output     float64
	CacheRead  float64
	CacheWrite float64
}

type TokenWeightOverride struct {
	Input      *float64 `json:"input,omitempty"`
	Output     *float64 `json:"output,omitempty"`
	CacheRead  *float64 `json:"cache_read,omitempty"`
	CacheWrite *float64 `json:"cache_write,omitempty"`
}

type TokenWeightsPolicy struct {
	Claude *TokenWeightOverride `json:"claude,omitempty"`
	Agy    *TokenWeightOverride `json:"agy,omitempty"`
}

type TelemetryPolicy struct {
	TokenWeights *TokenWeightsPolicy `json:"token_weights,omitempty"`
}

func (t *TokenWeightsPolicy) overrides() map[string]*TokenWeightOverride {
	out := map[string]*TokenWeightOverride{}
	fields := reflect.TypeFor[TokenWeightsPolicy]()
	for i := range fields.NumField() {
		family, _, _ := strings.Cut(fields.Field(i).Tag.Get("json"), ",")
		out[family] = nil
		if t != nil {
			out[family] = reflect.ValueOf(*t).Field(i).Interface().(*TokenWeightOverride)
		}
	}
	return out
}

func tokenWeightFamilies() []string {
	return slices.Sorted(maps.Keys((*TokenWeightsPolicy)(nil).overrides()))
}

type weightField struct {
	name string
	set  *float64
	into *float64
}

func (o *TokenWeightOverride) fields(resolved *TokenWeight) []weightField {
	return []weightField{
		{"input", o.Input, &resolved.Input},
		{"output", o.Output, &resolved.Output},
		{"cache_read", o.CacheRead, &resolved.CacheRead},
		{"cache_write", o.CacheWrite, &resolved.CacheWrite},
	}
}

type TokenWeightKeyError struct {
	Key string
}

func (e *TokenWeightKeyError) Error() string {
	return e.Key + " must not be negative"
}

func (o *TokenWeightOverride) validate(family string) error {
	if o == nil {
		return nil
	}
	for _, w := range o.fields(&TokenWeight{}) {
		if w.set != nil && *w.set < 0 {
			return &TokenWeightKeyError{Key: "telemetry.token_weights." + family + "." + w.name}
		}
	}
	return nil
}

func (t *TelemetryPolicy) validate() error {
	if t == nil {
		return nil
	}
	overrides := t.TokenWeights.overrides()
	for _, family := range tokenWeightFamilies() {
		if err := overrides[family].validate(family); err != nil {
			return err
		}
	}
	return nil
}

func (o *TokenWeightOverride) resolve() TokenWeight {
	weight := defaultTokenWeight
	if o == nil {
		return weight
	}
	for _, w := range o.fields(&weight) {
		if w.set != nil {
			*w.into = *w.set
		}
	}
	return weight
}

func (p Policy) TokenWeightsFor(family string) (TokenWeight, error) {
	var weights *TokenWeightsPolicy
	if p.Telemetry != nil {
		weights = p.Telemetry.TokenWeights
	}
	override, known := weights.overrides()[family]
	if !known {
		return TokenWeight{}, fmt.Errorf("%w %q: no token weight is defined for this family; it comes from the bridge manifest, so a newer binary may know it", ErrUnknownKey, "telemetry.token_weights."+family)
	}
	return override.resolve(), nil
}
