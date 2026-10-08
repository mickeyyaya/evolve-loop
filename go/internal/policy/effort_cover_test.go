package policy_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func TestEfforts_APolicyWithoutARoutingBlockGivesOnlyTheCompiledDefaults(t *testing.T) {
	t.Parallel()
	table := policy.Policy{}.Efforts()

	if e, src := table.Resolve("deep", "auditor"); e != "medium" || src == "cli_routing.agents.auditor" || src == "cli_routing.tiers.deep" {
		t.Errorf("Resolve(deep, auditor) = %q, %q, want the compiled medium", e, src)
	}
	if e, src := table.Resolve("galactic"); e != "" || src != "" {
		t.Errorf("Resolve(galactic) = %q, %q, want no effort", e, src)
	}
}

func TestTierRule_AChainWithANonStringIsRefused(t *testing.T) {
	t.Parallel()
	var r policy.TierRule

	err := json.Unmarshal([]byte(`["claude", 7]`), &r)

	if err == nil || !strings.HasPrefix(err.Error(), "tier rule: want an array of CLIs") || r.CLIs != nil {
		t.Errorf("Unmarshal = %+v, %v, want the tier-rule error and no chain", r, err)
	}
}
