package policy_test

// CatalogPolicy — the model-catalog config that replaced EVOLVE_MODELCATALOG_AUTOREFRESH.
// AutoRefresh defaults true (on); absent block keeps catalog refresh enabled.

import (
	"testing"

	"github.com/mickeyyaya/evolveloop/go/internal/policy"
)

func TestCatalogConfig_Resolution(t *testing.T) {
	truePtr := func() *bool { v := true; return &v }()
	falsePtr := func() *bool { v := false; return &v }()

	cases := []struct {
		name        string
		pol         policy.Policy
		wantRefresh bool
	}{
		{"absent-defaults-true", policy.Policy{}, true},
		{"empty-block-defaults-true", policy.Policy{Catalog: &policy.CatalogPolicy{}}, true},
		{"explicit-true", policy.Policy{Catalog: &policy.CatalogPolicy{AutoRefresh: truePtr}}, true},
		{"explicit-false", policy.Policy{Catalog: &policy.CatalogPolicy{AutoRefresh: falsePtr}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.pol.CatalogConfig()
			if got.AutoRefresh == nil {
				t.Fatalf("CatalogConfig().AutoRefresh is nil, want non-nil")
			}
			if *got.AutoRefresh != tc.wantRefresh {
				t.Errorf("CatalogConfig().AutoRefresh = %v, want %v", *got.AutoRefresh, tc.wantRefresh)
			}
		})
	}
}

func TestLoad_CatalogBlock(t *testing.T) {
	cases := []struct {
		name        string
		json        string
		wantRefresh bool
	}{
		{"absent-block-defaults-true", `{}`, true},
		{"auto-refresh-false", `{"catalog":{"auto_refresh":false}}`, false},
		{"auto-refresh-true-explicit", `{"catalog":{"auto_refresh":true}}`, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			pol, err := policy.Load(writeTempPolicy(t, tc.json))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			got := pol.CatalogConfig()
			if got.AutoRefresh == nil {
				t.Fatalf("CatalogConfig().AutoRefresh is nil after Load, want non-nil")
			}
			if *got.AutoRefresh != tc.wantRefresh {
				t.Errorf("after Load, CatalogConfig().AutoRefresh = %v, want %v", *got.AutoRefresh, tc.wantRefresh)
			}
		})
	}
}
