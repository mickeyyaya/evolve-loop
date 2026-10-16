package policy

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGatesConfig_RepoContractGateDefaultEnforce(t *testing.T) {
	if got := (Policy{}).GatesConfig().RepoContractGate; got != "enforce" {
		t.Fatalf("default = %q, want enforce", got)
	}
	p := Policy{Gates: &GatesPolicy{RepoContractGate: "off"}}
	if got := p.GatesConfig().RepoContractGate; got != "off" {
		t.Fatalf("operator off = %q, want off", got)
	}
}

func TestGatesConfig_ScanCacheGateDefaultsToShadow(t *testing.T) {
	t.Parallel()
	if got := (Policy{}).GatesConfig().ScanCacheGate; got != "shadow" {
		t.Fatalf("default = %q, want shadow", got)
	}
	p := Policy{Gates: &GatesPolicy{ScanCacheGate: "off"}}
	if got := p.GatesConfig().ScanCacheGate; got != "off" {
		t.Fatalf("operator off = %q, want off", got)
	}
}

func TestScanCacheFor_ReadsTheProjectPolicy(t *testing.T) {
	t.Parallel()
	write := func(body string) string {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".evolve"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".evolve", "policy.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return root
	}
	def := ScanCacheConfig{Gate: "shadow", MaxBytes: 2048 << 20, Budget: 60 * time.Second}
	cases := map[string]struct {
		root string
		want ScanCacheConfig
	}{
		"no policy file":   {t.TempDir(), def},
		"explicit off":     {write(`{"gates":{"scan_cache_gate":"off"}}`), ScanCacheConfig{Gate: "off", MaxBytes: def.MaxBytes, Budget: def.Budget}},
		"other gates only": {write(`{"gates":{"repo_contract_gate":"off"}}`), def},
		"malformed policy": {write(`{not json`), def},
		"explicit budget":  {write(`{"gates":{"scan_cache_max_mb":10,"scan_cache_budget_seconds":5}}`), ScanCacheConfig{Gate: "shadow", MaxBytes: 10 << 20, Budget: 5 * time.Second}},
		"non-positive":     {write(`{"gates":{"scan_cache_max_mb":-1,"scan_cache_budget_seconds":0}}`), def},
	}
	for name, c := range cases {
		if got := ScanCacheFor(c.root); got != c.want {
			t.Errorf("%s: got %+v, want %+v", name, got, c.want)
		}
	}
}

func TestGatesConfig_EveryGateStageIsOverridable(t *testing.T) {
	t.Parallel()
	got := Policy{Gates: &GatesPolicy{
		ContractGate: "a", EvalGate: "b", TriageCapGate: "c", ReviewGate: "d", ReportSizeGate: "e",
		TopNGate: "f", ManifestGate: "g", RepoContractGate: "h", ScanCacheGate: "i", ScanCacheMaxMB: 1, ScanCacheBudgetSeconds: 2,
	}}.GatesConfig()
	want := GatesConfig{
		ContractGate: "a", EvalGate: "b", TriageCapGate: "c", ReviewGate: "d", ReportSizeGate: "e",
		TopNGate: "f", ManifestGate: "g", RepoContractGate: "h", ScanCacheGate: "i", ScanCacheMaxMB: 1, ScanCacheBudgetSeconds: 2,
	}
	if got != want {
		t.Fatalf("every dial an operator sets must win over its default;\n got %+v\nwant %+v", got, want)
	}
}
