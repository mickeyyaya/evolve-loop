package policy_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestDocsFloorConfigDefaults(t *testing.T) {
	cases := []struct {
		name string
		p    policy.Policy
		want policy.DocsFloorPolicy
	}{
		{
			name: "absent block ⇒ compiled default enforce",
			p:    policy.Policy{},
			want: policy.DocsFloorPolicy{Stage: "enforce", SteStage: "shadow"},
		},
		{
			name: "empty block ⇒ compiled default enforce",
			p:    policy.Policy{DocsFloor: &policy.DocsFloorPolicy{}},
			want: policy.DocsFloorPolicy{Stage: "enforce", SteStage: "shadow"},
		},
		{
			name: "an explicit ste_stage wins and keeps the stage default",
			p:    policy.Policy{DocsFloor: &policy.DocsFloorPolicy{SteStage: "off"}},
			want: policy.DocsFloorPolicy{Stage: "enforce", SteStage: "off"},
		},
		{
			name: "explicit off wins",
			p:    policy.Policy{DocsFloor: &policy.DocsFloorPolicy{Stage: "off"}},
			want: policy.DocsFloorPolicy{Stage: "off", SteStage: "shadow"},
		},
		{
			name: "explicit shadow wins",
			p:    policy.Policy{DocsFloor: &policy.DocsFloorPolicy{Stage: "shadow"}},
			want: policy.DocsFloorPolicy{Stage: "shadow", SteStage: "shadow"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.p.DocsFloorConfig(); got != tc.want {
				t.Errorf("DocsFloorConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestDocsFloorConfigRoundTripsThroughLoad(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name, body, want string
	}{
		{"bare policy keeps the compiled default", `{}`, "enforce"},
		{"docs_floor.stage override reaches the gate", `{"docs_floor":{"stage":"off"}}`, "off"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".json")
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatalf("write policy: %v", err)
			}
			p, err := policy.Load(path)
			if err != nil {
				t.Fatalf("policy.Load: %v", err)
			}
			if got := p.DocsFloorConfig().Stage; got != tc.want {
				t.Errorf("stage = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDocsFloorConfig_SteStageReachesTheFloorFromPolicyJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "policy.json")
	if err := os.WriteFile(path, []byte(`{"docs_floor":{"ste_stage":"enforce"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := policy.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.DocsFloorConfig(); got != (policy.DocsFloorPolicy{Stage: "enforce", SteStage: "enforce"}) {
		t.Errorf("DocsFloorConfig() = %+v, want ste_stage enforce beside the stage default", got)
	}
}

func TestDocsFloorPolicy_DecodesStrictly(t *testing.T) {
	for name, tc := range map[string]struct {
		body string
		want string
	}{
		"an unknown key":                   {`{"docs_floor":{"ste_stag":"off"}}`, "ste_stag"},
		"an unknown stage word":            {`{"docs_floor":{"ste_stage":"warn"}}`, `"warn"`},
		"an unknown docs floor stage word": {`{"docs_floor":{"stage":"warn"}}`, `"warn"`},
		"a stage of the wrong type":        {`{"docs_floor":{"ste_stage":1}}`, "ste_stage"},
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "policy.json")
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := policy.Load(path)
			if err == nil || !strings.Contains(err.Error(), "docs_floor") || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load(%s) err = %v, want a docs_floor error naming %s", tc.body, err, tc.want)
			}
		})
	}
}

func TestDocsFloorPolicy_AcceptsEveryStageWord(t *testing.T) {
	for _, word := range []string{"off", "shadow", "enforce"} {
		var d policy.DocsFloorPolicy
		if err := json.Unmarshal([]byte(`{"stage":"`+word+`","ste_stage":"`+word+`"}`), &d); err != nil || d.SteStage != word || d.Stage != word {
			t.Errorf("stage and ste_stage %q: got %+v, %v", word, d, err)
		}
	}
}
