package policy_test

// PathsConfig — the paths discovery config that replaced EVOLVE_KB_SEARCH_PATHS
// and EVOLVE_PHASE_ROOTS. Absent block → empty PathsConfig; callers use built-in defaults.

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/policy"
)

func TestPathsConfig_Resolution(t *testing.T) {
	cases := []struct {
		name string
		pol  policy.Policy
		want policy.PathsConfig
	}{
		{
			"absent-defaults-empty",
			policy.Policy{},
			policy.PathsConfig{},
		},
		{
			"empty-block-defaults-empty",
			policy.Policy{Paths: &policy.PathsConfig{}},
			policy.PathsConfig{},
		},
		{
			"kb-search-paths-set",
			policy.Policy{Paths: &policy.PathsConfig{KBSearchPaths: "/a:/b"}},
			policy.PathsConfig{KBSearchPaths: "/a:/b"},
		},
		{
			"phase-roots-set",
			policy.Policy{Paths: &policy.PathsConfig{PhaseRoots: ".evolve/phases:/plugin"}},
			policy.PathsConfig{PhaseRoots: ".evolve/phases:/plugin"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.pol.PathsConfig()
			if got != tc.want {
				t.Errorf("PathsConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestLoad_PathsBlock(t *testing.T) {
	cases := []struct {
		name string
		json string
		want policy.PathsConfig
	}{
		{
			"absent-block-empty",
			`{}`,
			policy.PathsConfig{},
		},
		{
			"kb-search-paths",
			`{"paths":{"kb_search_paths":"/a:/b"}}`,
			policy.PathsConfig{KBSearchPaths: "/a:/b"},
		},
		{
			"phase-roots",
			`{"paths":{"phase_roots":".evolve/phases:/plugin"}}`,
			policy.PathsConfig{PhaseRoots: ".evolve/phases:/plugin"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pol, err := policy.Load(writeTempPolicy(t, tc.json))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := pol.PathsConfig(); got != tc.want {
				t.Errorf("after Load, PathsConfig() = %+v, want %+v", got, tc.want)
			}
		})
	}
}
