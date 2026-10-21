package policy_test

import (
	"bytes"
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

var checkedInPolicyPath = filepath.Join("..", "..", "..", ".evolve", "policy.json")

func TestLoad_TheCheckedInPolicyRunsTheSTEFloorAtEnforce(t *testing.T) {
	t.Parallel()
	p, err := policy.Load(checkedInPolicyPath)
	if err != nil {
		t.Fatalf("the checked-in policy must load: %v", err)
	}

	if got := p.DocsFloorConfig(); p.DocsFloor == nil || got.SteStage != "enforce" {
		t.Errorf("checked-in docs_floor = %+v (block %+v); the operator approved ste_stage enforce on 2026-10-21, so a lane doc that fails the STE lint goes back to the build", got, p.DocsFloor)
	}
}

func TestLoad_TheCheckedInPolicyRecordsTheAuditCascadeInShadow(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(checkedInPolicyPath)
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		AuditCascade json.RawMessage `json:"audit_cascade"`
	}
	if err := json.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	strict := json.NewDecoder(bytes.NewReader(file.AuditCascade))
	strict.DisallowUnknownFields()
	var block policy.AuditCascadePolicy
	decodeErr := strict.Decode(&block)
	p, loadErr := policy.Load(checkedInPolicyPath)

	got := p.AuditCascadeConfig()
	if decodeErr != nil || loadErr != nil || got.Stage != policy.AuditCascadeShadow || len(got.RiskPaths) == 0 || got.MaxChangedFiles == 0 || len(got.EscalateVerdicts) == 0 {
		t.Errorf("audit_cascade = %+v (strict decode %v, load %v); the operator approved the shadow stage on 2026-10-21 with every threshold in config", got, decodeErr, loadErr)
	}
}
