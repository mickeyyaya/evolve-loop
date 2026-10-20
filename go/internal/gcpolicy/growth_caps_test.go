package gcpolicy_test

import (
	"encoding/json"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/gcpolicy"
)

func TestGrowthCaps_AZeroValuePolicyTakesTheCompiledDefaults(t *testing.T) {
	var p gcpolicy.Policy
	if got := p.GuardsLogMaxBytes(); got != 10<<20 {
		t.Errorf("GuardsLogMaxBytes() = %d, want the 10 MB default", got)
	}
	if got := p.VerdictCacheKeep(); got != 50 {
		t.Errorf("VerdictCacheKeep() = %d, want the default of 50 entries", got)
	}
	if got := p.LedgerSealKeep(); got != 20000 {
		t.Errorf("LedgerSealKeep() = %d, want the 20000-line default (about 285 cycles at 70 lines a cycle)", got)
	}
	if p.LedgerSealEnforced() {
		t.Error("the ledger seal must stay in shadow until the policy names enforce")
	}
}

func TestGrowthCaps_ThePolicyKeysSetEachCap(t *testing.T) {
	var p gcpolicy.Policy
	if err := json.Unmarshal([]byte(`{"guards_log_max_mb":2,"verdict_cache_keep":7,"ledger_seal_keep":40,"ledger_seal":"enforce"}`), &p); err != nil {
		t.Fatal(err)
	}
	if p.GuardsLogMaxBytes() != 2<<20 || p.VerdictCacheKeep() != 7 || p.LedgerSealKeep() != 40 || !p.LedgerSealEnforced() {
		t.Errorf("caps = %d, %d, %d; want 2 MB, 7, 40", p.GuardsLogMaxBytes(), p.VerdictCacheKeep(), p.LedgerSealKeep())
	}
}
