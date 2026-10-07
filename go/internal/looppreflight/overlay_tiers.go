package looppreflight

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

func checkOverlayTierSelectors(o resolved) CheckResult {
	const name = "overlay-tier-selectors"
	pol, err := policy.Load(filepath.Join(o.evolveDir, "policy.json"))
	if err != nil {
		return CheckResult{Name: name, Level: LevelWarn,
			Message: "overlay rule tier selectors not checked: policy unreadable",
			Detail:  err.Error()}
	}
	bad := pol.NonCanonicalOverlayTierSelectors()
	if len(bad) == 0 {
		return CheckResult{Name: name, Level: LevelPass,
			Message: "every overlay rule tier selector names a canonical tier"}
	}
	return CheckResult{Name: name, Level: LevelWarn,
		Message: fmt.Sprintf("%d overlay rule tier selector(s) name no canonical tier and match only a dispatch token spelled the same way", len(bad)),
		Detail: fmt.Sprintf("selectors: %s; canonical tiers: %s",
			strings.Join(bad, ", "), strings.Join(policy.TierNames(), ", ")),
	}
}
