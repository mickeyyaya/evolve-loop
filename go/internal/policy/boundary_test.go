package policy_test

import (
	"encoding/json"
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

func TestBoundaryPolicy_RatchetMergeIsTheOnlyKeyAndSurvivesAJSONRoundTrip(t *testing.T) {
	t.Parallel()
	p, err := policy.Parse([]byte(`{"boundary":{"ratchet_merge":{"stage":"enforce"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if p.Boundary == nil || p.Boundary.RatchetMerge == nil {
		t.Fatalf("boundary = %+v, want a parsed ratchet_merge", p.Boundary)
	}
	out, err := json.Marshal(p.Boundary)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"ratchet_merge":{"stage":"enforce"}}` {
		t.Errorf("boundary JSON = %s", out)
	}
	if _, err := policy.Parse([]byte(`{"boundary":{"other":{}}}`)); err == nil {
		t.Error("an unknown boundary key must be refused")
	}
}

func TestRatchetMergePolicy_StageWordDrivesTheReadStageAndAnEmptyStageIsOmitted(t *testing.T) {
	t.Parallel()
	enforce := policy.Policy{Boundary: &policy.BoundaryPolicy{RatchetMerge: &policy.RatchetMergePolicy{Stage: policy.RatchetMergeEnforce}}}
	if got := enforce.RatchetMergeStage(); got != policy.RatchetMergeEnforce {
		t.Errorf("stage %q, want enforce", got)
	}
	out, err := json.Marshal(policy.RatchetMergePolicy{})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{}` {
		t.Errorf("empty RatchetMergePolicy JSON = %s, want {}", out)
	}
}
