package policy_test

import (
	"reflect"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestRatchetMergeStage_DefaultsToShadowWhenAbsent(t *testing.T) {
	t.Parallel()
	for name, raw := range map[string]string{
		"no policy":        `{}`,
		"empty boundary":   `{"boundary":{}}`,
		"empty ratchet":    `{"boundary":{"ratchet_merge":{}}}`,
		"empty stage word": `{"boundary":{"ratchet_merge":{"stage":""}}}`,
	} {
		p, err := policy.Parse([]byte(raw))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if got := p.RatchetMergeStage(); got != policy.RatchetMergeShadow {
			t.Errorf("%s: stage %q, want shadow", name, got)
		}
	}
}

func TestRatchetMergeStage_ReadsEachWord(t *testing.T) {
	t.Parallel()
	for word, want := range map[string]policy.RatchetMergeStage{
		"off": policy.RatchetMergeOff, "shadow": policy.RatchetMergeShadow, "enforce": policy.RatchetMergeEnforce,
	} {
		p, err := policy.Parse([]byte(`{"boundary":{"ratchet_merge":{"stage":"` + word + `"}}}`))
		if err != nil {
			t.Fatalf("%s: %v", word, err)
		}
		if got := p.RatchetMergeStage(); got != want || string(got) != word {
			t.Errorf("%s: stage %q, want %q", word, got, want)
		}
	}
}

func TestRatchetMergeStage_RefusesAnyOtherWord(t *testing.T) {
	t.Parallel()
	for _, word := range []string{"on", "0", "advisory", "Enforce"} {
		_, err := policy.Parse([]byte(`{"boundary":{"ratchet_merge":{"stage":"` + word + `"}}}`))
		if err == nil {
			t.Errorf("%q: want a refusal", word)
		}
	}
}

func TestRatchetMergeStage_RefusesAnUnknownKey(t *testing.T) {
	t.Parallel()
	if _, err := policy.Parse([]byte(`{"boundary":{"ratchet_merge":{"stag":"shadow"}}}`)); err == nil {
		t.Error("a misspelled key must be refused")
	}
}

func TestRatchetMergeStage_IsANamedStringTypeNotAPlainString(t *testing.T) {
	t.Parallel()
	for _, f := range policy.StageFields() {
		if f.Path == "boundary.ratchet_merge.stage" {
			if f.TypeName == "string" || f.TypeName != reflect.TypeOf(policy.RatchetMergeStage("")).String() {
				t.Errorf("type name %q, want the named RatchetMergeStage", f.TypeName)
			}
			return
		}
	}
	t.Error("StageFields lacks boundary.ratchet_merge.stage")
}
