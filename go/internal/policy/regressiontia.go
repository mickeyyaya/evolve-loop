package policy

import "path/filepath"

// RegressionTIAPolicy is the .evolve/policy.json "regression_tia" block — the
// config-as-code surface for test-impact selection over the EGPS Go regression
// corpus (no flags; the shape mirrors the sibling ParallelEvaluate block).
//
// Stage drives the off → shadow → enforce rollout:
//   - "off"     dormant. Nothing is computed, no artifact is written, and the
//     audit path is byte-identical to its pre-change self. This is
//     the LIVE production value: the checked-in policy.json carries
//     no regression_tia block.
//   - "shadow"  compute the decision and emit it as evidence; skip nothing.
//   - "enforce" reserved for the human-gated cutover that actually narrows what
//     the gate runs. It emits the same evidence today.
type RegressionTIAPolicy struct {
	// Stage selects the rollout stage: "off" / "shadow" / "enforce".
	// Empty/absent/unrecognized ⇒ "off".
	Stage string `json:"stage,omitempty"`
}

// RegressionTIAConfig is the resolved regression-TIA configuration with
// defaults applied.
type RegressionTIAConfig struct {
	Stage string
}

// RegressionTIAConfig returns the regression-TIA configuration with built-in
// defaults resolved. The zero-value Policy{} and an absent block both yield the
// safe default (stage="off"). Only the closed vocabulary {off,shadow,enforce}
// is honored; ANY other value — a typo, a case variant, "on", "true" — falls
// back to "off".
//
// The fallback direction is the whole safety argument: selection that skips a
// predicate can hide a regression class, so a misspelled stage must never
// silently ARM selection. Every unknown resolves toward running everything.
func (p Policy) RegressionTIAConfig() RegressionTIAConfig {
	c := RegressionTIAConfig{Stage: "off"}
	if p.RegressionTIA == nil {
		return c
	}
	switch p.RegressionTIA.Stage {
	case "off", "shadow", "enforce":
		c.Stage = p.RegressionTIA.Stage
	}
	return c
}

// RegressionTIAStageFor resolves the regression-TIA stage from
// <projectRoot>/.evolve/policy.json, mirroring StrictAuditFor. An unreadable or
// unparseable policy file resolves to "off" — the dormant, byte-identical
// default — so a config problem can never arm selection either.
func RegressionTIAStageFor(projectRoot string) string {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		return "off"
	}
	return pol.RegressionTIAConfig().Stage
}
