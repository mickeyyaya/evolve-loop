package policy

const (
	// defaultRecallK stays at 5 because lowering it would narrow the advisor's failure recall on every install.
	defaultRecallK = 5
	// maxRecallK rejects typo-sized values that would flood the advisor prompt with weak matches.
	maxRecallK = 50
	// defaultNoveltyThreshold is strict because suppressing a lesson destroys failure evidence.
	defaultNoveltyThreshold = 0.9
)

// ResearchPolicy is the "research" block. See docs/architecture/policy-config.md.
type ResearchPolicy struct {
	// RecallK bounds how many lessons a KB lookup returns; outside 1..maxRecallK means 5.
	RecallK int `json:"recall_k"`
	// NoveltyThreshold is the similarity in (0,1] at which an incoming lesson is a
	// near-duplicate; out of range means 0.9.
	NoveltyThreshold float64 `json:"novelty_threshold"`
}

// ResearchConfig is the resolved research configuration with defaults applied.
type ResearchConfig struct {
	RecallK          int
	NoveltyThreshold float64
}

// ResearchConfig returns the research block; only in-range values override the defaults (5, 0.9).
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
