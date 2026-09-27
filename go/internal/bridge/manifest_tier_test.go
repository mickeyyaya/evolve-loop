package bridge

import "testing"

func TestTranslateV1TierKey_HighAliasesToDeep(t *testing.T) {
	if got := translateV1TierKey("high"); got != "deep" {
		t.Errorf(`translateV1TierKey("high") = %q, want "deep"`, got)
	}
}

func TestTranslateV1TierKey_TopIsNotConflatedWithHigh(t *testing.T) {
	if got := translateV1TierKey("top"); got != "top" {
		t.Errorf(`translateV1TierKey("top") = %q, want "top" (pass-through — "top" must stay distinct from "deep")`, got)
	}
}

func TestTranslateV1TierKey_PreexistingMappingsUnchanged(t *testing.T) {
	cases := map[string]string{
		"haiku":       "fast",
		"sonnet":      "balanced",
		"opus":        "deep",
		"some-custom": "some-custom",
	}
	for in, want := range cases {
		if got := translateV1TierKey(in); got != want {
			t.Errorf("translateV1TierKey(%q) = %q, want %q", in, got, want)
		}
	}
}
