package policy_test

// ClassifyPolicy — the classifier config that replaced EVOLVE_HANG_CLASSIFIER.
// HangClassifier defaults false (opt-in); absent block is safe (no reclassification).

import (
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestClassifyConfig_Resolution(t *testing.T) {
	cases := []struct {
		name string
		pol  policy.Policy
		want policy.ClassifyPolicy
	}{
		{
			"absent-defaults-false",
			policy.Policy{},
			policy.ClassifyPolicy{HangClassifier: false},
		},
		{
			"empty-block-defaults-false",
			policy.Policy{Classify: &policy.ClassifyPolicy{}},
			policy.ClassifyPolicy{HangClassifier: false},
		},
		{
			"hang-classifier-enabled",
			policy.Policy{Classify: &policy.ClassifyPolicy{HangClassifier: true}},
			policy.ClassifyPolicy{HangClassifier: true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.pol.ClassifyConfig()
			if got != tc.want {
				t.Errorf("ClassifyConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestLoad_ClassifyBlock(t *testing.T) {
	cases := []struct {
		name string
		json string
		want policy.ClassifyPolicy
	}{
		{
			"absent-block-defaults-false",
			`{}`,
			policy.ClassifyPolicy{HangClassifier: false},
		},
		{
			"hang-classifier-true",
			`{"classify":{"hang_classifier":true}}`,
			policy.ClassifyPolicy{HangClassifier: true},
		},
		{
			"hang-classifier-false-explicit",
			`{"classify":{"hang_classifier":false}}`,
			policy.ClassifyPolicy{HangClassifier: false},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pol, err := policy.Load(writeTempPolicy(t, tc.json))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := pol.ClassifyConfig(); got != tc.want {
				t.Errorf("after Load, ClassifyConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
