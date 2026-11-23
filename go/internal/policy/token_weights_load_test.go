package policy

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func refusedKey(t *testing.T, body string) string {
	t.Helper()
	_, err := Parse([]byte(body))
	var keyErr *TokenWeightKeyError
	if !errors.As(err, &keyErr) {
		t.Fatalf("%s: err = %v, want a TokenWeightKeyError", body, err)
	}
	if err.Error() != keyErr.Key+" must not be negative" {
		t.Errorf("%s: message = %q", body, err.Error())
	}
	return keyErr.Key
}

func namesKey(err error, key string) bool {
	named, _, _ := strings.Cut(err.Error(), ":")
	return named == ErrUnknownKey.Error()+" "+strconv.Quote(key)
}

func TestTokenWeights_ClaudeFamilyLoads(t *testing.T) {
	t.Parallel()
	if _, err := Parse([]byte(`{"telemetry":{"token_weights":{"claude":{"output":7}}}}`)); err != nil {
		t.Fatalf("Parse = %v", err)
	}
}

func TestTokenWeights_UnknownKeysFailNamingTheKey(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		`{"telemetry":{"token_weights":{"gemini":{"input":1}}}}`:     "telemetry.token_weights.gemini",
		`{"telemetry":{"token_weights":{"claude":{"cache_hit":1}}}}`: "telemetry.token_weights.claude.cache_hit",
		`{"telemetry":{"token_weights":{"claude":{}},"other":1}}`:    "telemetry.other",
	}
	for body, key := range cases {
		_, err := Parse([]byte(body))
		if !errors.Is(err, ErrUnknownKey) || !namesKey(err, key) {
			t.Errorf("%s: err = %v, want ErrUnknownKey naming %q", body, err, key)
		}
	}
}

func TestTokenWeights_NegativeWeightFailsNamingTheFullKey(t *testing.T) {
	t.Parallel()
	for _, family := range []string{"claude", "agy"} {
		for _, name := range []string{"input", "output", "cache_read", "cache_write"} {
			body := `{"telemetry":{"token_weights":{"` + family + `":{"` + name + `":-0.5}}}}`
			key := "telemetry.token_weights." + family + "." + name
			if got := refusedKey(t, body); got != key {
				t.Errorf("%s: refused key = %q, want %q", body, got, key)
			}
		}
	}
}

func TestTokenWeights_TwoBadFamiliesNameTheFirstInSortedOrder(t *testing.T) {
	t.Parallel()
	body := `{"telemetry":{"token_weights":{"claude":{"input":-1},"agy":{"input":-1}}}}`
	if got := refusedKey(t, body); got != "telemetry.token_weights.agy.input" {
		t.Errorf("refused key = %q, want the agy key first", got)
	}
}

func TestTokenWeights_WrongShapesFail(t *testing.T) {
	t.Parallel()
	for _, body := range []string{
		`{"telemetry":[]}`,
		`{"telemetry":{"token_weights":3}}`,
		`{"telemetry":{"token_weights":{"claude":{"output":"5"}}}}`,
	} {
		if _, err := Parse([]byte(body)); err == nil {
			t.Errorf("%s: want a refusal", body)
		}
	}
}
