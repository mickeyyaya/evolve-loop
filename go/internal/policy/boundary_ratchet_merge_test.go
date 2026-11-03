package policy_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestBoundaryRatchetMergeStage_DefaultsToShadowAndReadsTheKey(t *testing.T) {
	cases := []struct {
		name string
		body string
		want config.Stage
	}{
		{"no key", `{}`, config.StageShadow},
		{"an empty boundary block", `{"boundary":{}}`, config.StageShadow},
		{"an empty stage", `{"boundary":{"ratchet_merge":{}}}`, config.StageShadow},
		{"off", `{"boundary":{"ratchet_merge":{"stage":"off"}}}`, config.StageOff},
		{"enforce", `{"boundary":{"ratchet_merge":{"stage":"enforce"}}}`, config.StageEnforce},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, err := policy.Parse([]byte(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			got, err := p.BoundaryRatchetMergeStage()
			if err != nil || got != tc.want {
				t.Errorf("BoundaryRatchetMergeStage = %v, %v, want %v", got, err, tc.want)
			}
		})
	}
}

func TestBoundaryRatchetMergeStage_AnUnknownWordFailsClosed(t *testing.T) {
	p, err := policy.Parse([]byte(`{"boundary":{"ratchet_merge":{"stage":"sometimes"}}}`))
	if err != nil {
		t.Fatal(err)
	}
	got, err := p.BoundaryRatchetMergeStage()
	if err == nil || got != config.StageOff || !strings.Contains(err.Error(), "boundary.ratchet_merge.stage") {
		t.Errorf("BoundaryRatchetMergeStage = %v, %v, want off and an error naming the key", got, err)
	}
}

func TestBoundaryRatchetMergePolicy_RefusesAnUnknownKey(t *testing.T) {
	_, err := policy.Parse([]byte(`{"boundary":{"ratchet_merge":{"stag":"enforce"}}}`))
	if err == nil || !strings.Contains(err.Error(), "boundary.ratchet_merge.stag") {
		t.Errorf("Parse = %v, want an unknown-key error naming boundary.ratchet_merge.stag", err)
	}
}

func TestTheCheckedInPolicy_HoldsTheShadowStagesThatWaitForEvidence(t *testing.T) {
	p, err := policy.Load(filepath.Join("..", "..", "..", ".evolve", "policy.json"))
	if err != nil {
		t.Fatalf("the checked-in policy must pass the strict decoder: %v", err)
	}
	if p.Boundary == nil || p.Boundary.RatchetMerge == nil || p.Boundary.RatchetMerge.Stage != "shadow" {
		t.Fatalf("boundary.ratchet_merge = %+v, want the key checked in at shadow", p.Boundary)
	}
	if got, err := p.BoundaryRatchetMergeStage(); err != nil || got != config.StageShadow {
		t.Errorf("boundary.ratchet_merge.stage = %v, %v, want shadow", got, err)
	}
}

func TestBoundaryPolicy_ABuiltBoundaryRatchetMergePolicyHoldsItsStage(t *testing.T) {
	p := policy.Policy{Boundary: &policy.BoundaryPolicy{RatchetMerge: &policy.BoundaryRatchetMergePolicy{Stage: "enforce"}}}
	if got, err := p.BoundaryRatchetMergeStage(); err != nil || got != config.StageEnforce {
		t.Errorf("BoundaryRatchetMergeStage = %v, %v, want enforce from a built policy", got, err)
	}
}
