package quotastate

import (
	"strings"
	"testing"
)

func claudeLikeSpec(modelMatch map[string]string) WindowSpec {
	return WindowSpec{
		Labels: []WindowLabel{
			{Regex: `^\s*Current session\s*$`, Kind: KindSession, Scope: "session"},
			{Regex: `^\s*Current week \((?P<scope>[^)]+)\)\s*$`, Kind: KindWeek},
		},
		ValueRegex:     `(?P<pct>\d{1,3}(?:\.\d+)?)%\s*used`,
		ValueDirection: DirectionUsed,
		Scopes:         map[string]ScopeTarget{AnyScope: {Family: "claude", PerModel: true}},
		ModelMatch:     modelMatch,
	}
}

func TestWindowSpec_ModelMatchesFollowsTheDeclaredMapAlone(t *testing.T) {
	t.Parallel()
	spec := claudeLikeSpec(map[string]string{"Fable": "(?i)sonnet"})

	cases := []struct {
		word, model string
		want        bool
	}{
		{"Fable", "claude-sonnet-5-5", true},
		{"Fable", "Claude SONNET 5.5 (High)", true},
		{"Fable", "claude-opus-5-5", false},
		{"Opus", "claude-opus-5-5", false},
		{"", "claude-sonnet-5-5", false},
		{"Fable", "", false},
	}
	for _, tc := range cases {
		if got := spec.ModelMatches(tc.word, tc.model); got != tc.want {
			t.Errorf("ModelMatches(%q, %q) = %v, want %v", tc.word, tc.model, got, tc.want)
		}
	}
}

func TestWindowSpec_ModelMatchesIsFalseWhenTheSpecDeclaresNoMap(t *testing.T) {
	t.Parallel()
	if claudeLikeSpec(nil).ModelMatches("Opus", "claude-opus-5-5") {
		t.Error("no model_match declared, yet Opus matched an Opus model: a built-in word list would hide here")
	}
}

func TestWindowSpec_ValidateRejectsAModelMatchRegexThatDoesNotCompile(t *testing.T) {
	t.Parallel()
	err := claudeLikeSpec(map[string]string{"Opus": "(unclosed"}).Validate()
	if err == nil {
		t.Fatal("Validate accepted an uncompilable model_match regex")
	}
	if !strings.Contains(err.Error(), "model_match") || !strings.Contains(err.Error(), "Opus") {
		t.Errorf("error = %q, want it to name model_match and the scope word", err)
	}
}

func TestWindowSpec_ValidateAcceptsAWellFormedModelMatch(t *testing.T) {
	t.Parallel()
	if err := claudeLikeSpec(map[string]string{"Opus": "(?i)opus"}).Validate(); err != nil {
		t.Errorf("Validate rejected a good model_match: %v", err)
	}
}

func TestWindowSpec_ValidateRejectsAnEmptyModelMatchRegex(t *testing.T) {
	t.Parallel()
	err := claudeLikeSpec(map[string]string{"Opus": ""}).Validate()
	if err == nil || !strings.Contains(err.Error(), "empty regex") {
		t.Fatalf("Validate error = %v, want an empty-regex refusal", err)
	}
}
