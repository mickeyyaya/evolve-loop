package core

import (
	"reflect"
	"testing"
)

func TestEvaluateBatch(t *testing.T) {
	t.Parallel()
	// archetype lookup mirroring the real taxonomy for the test phases.
	arch := map[string]string{
		"bug-reproduction": "evaluate", // PRE-build evaluate — must NOT be batched
		"scout":            "plan",
		"build":            "build",
		"coverage-gate":    "evaluate",
		"behavior-compare": "evaluate",
		"adversarial":      "evaluate",
		"audit":            "evaluate", // the brancher — excluded
		"ship":             "control",
	}
	archetypeOf := func(p string) string { return arch[p] }

	cases := []struct {
		name string
		plan []string
		want []string
	}{
		{
			name: "post-build evaluate run, audit excluded",
			plan: []string{"scout", "build", "coverage-gate", "behavior-compare", "adversarial", "audit", "ship"},
			want: []string{"coverage-gate", "behavior-compare", "adversarial"},
		},
		{
			name: "pre-build evaluate (bug-reproduction) is NOT batched",
			plan: []string{"bug-reproduction", "scout", "build", "coverage-gate", "behavior-compare", "audit"},
			want: []string{"coverage-gate", "behavior-compare"},
		},
		{
			name: "single post-build evaluate ⇒ nothing to batch",
			plan: []string{"scout", "build", "coverage-gate", "audit"},
			want: nil,
		},
		{
			name: "no evaluate phases ⇒ nil",
			plan: []string{"scout", "build", "audit", "ship"},
			want: nil,
		},
		{
			name: "no build phase ⇒ nil (never batch a pre-build run)",
			plan: []string{"bug-reproduction", "scout", "audit"},
			want: nil,
		},
		{
			name: "audit immediately after build ⇒ nil",
			plan: []string{"build", "audit", "ship"},
			want: nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := evaluateBatch(tc.plan, archetypeOf)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("evaluateBatch(%v) = %v, want %v", tc.plan, got, tc.want)
			}
		})
	}
}
