package policy

// research_config.go — the .evolve/policy.json "research" block (cycle-1494,
// task `sleep-time-kb-consolidation`). Shape mirrors ContextFillPolicy, the
// canonical sub-block resolution pattern in this package: a pointer block on
// Policy plus a pure resolver that applies built-in defaults. Config-as-code,
// no flags — see [[phase_settings_from_config_not_code]].

const (
	// defaultRecallK is the built-in KB recall bound — the value
	// research.maxResults has always carried. The default is HELD at 5 on
	// purpose: the sole production consumer of KB recall is the advisor's
	// recallForPlan (core/routing_dispatch.go), so lowering it would silently
	// narrow failure recall for every existing install. This block makes the
	// bound tunable and reproducible, not smaller.
	defaultRecallK = 5
	// maxRecallK bounds operator input from above. A recall far past the
	// corpus's useful head floods the advisor prompt with weak matches, which
	// is the same degradation as no recall at all; values beyond this are typo
	// shapes (100000) rather than tuning.
	maxRecallK = 50
	// defaultNoveltyThreshold is the built-in near-duplicate similarity above
	// which a deterministic failure lesson is treated as already recorded.
	// 0.9 is deliberately strict: suppressing a lesson destroys failure
	// evidence, so only an observation that is almost token-identical to one
	// already on disk is dropped.
	defaultNoveltyThreshold = 0.9
)

// ResearchPolicy is the operator-facing "research" block.
type ResearchPolicy struct {
	// RecallK bounds how many lessons a KB lookup returns. Absent/zero/
	// negative/above maxRecallK ⇒ 5.
	RecallK int `json:"recall_k"`
	// NoveltyThreshold is the similarity (0,1] at or above which an incoming
	// failure lesson counts as a near-duplicate of one already in the corpus.
	// Absent/zero/negative/>1 ⇒ 0.9. The range check is load-bearing: 0 would
	// suppress every write (evidence loss) and 1.5 would disarm the gate
	// permanently, and both are typo shapes an operator would never intend.
	NoveltyThreshold float64 `json:"novelty_threshold"`
}

// ResearchConfig is the resolved research configuration with defaults applied.
type ResearchConfig struct {
	RecallK          int
	NoveltyThreshold float64
}

// ResearchConfig returns research configuration with built-in defaults
// resolved. The zero-value Policy{} yields the safe defaults (RecallK=5,
// NoveltyThreshold=0.9) — i.e. today's compiled behaviour on any install with
// no "research" block. Operator input is never accepted verbatim: only an
// in-range value overrides, anything else falls back to the visible built-in.
// Pure.
func (p Policy) ResearchConfig() ResearchConfig {
	c := ResearchConfig{RecallK: defaultRecallK, NoveltyThreshold: defaultNoveltyThreshold}
	if p.Research == nil {
		return c
	}
	if v := p.Research.RecallK; v > 0 && v <= maxRecallK {
		c.RecallK = v
	}
	if v := p.Research.NoveltyThreshold; v > 0 && v <= 1 {
		c.NoveltyThreshold = v
	}
	return c
}
