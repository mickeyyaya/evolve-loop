package policy

const (
	defaultRecallK          = 5
	maxRecallK              = 50
	defaultNoveltyThreshold = 0.9
)

type ResearchPolicy struct {
	RecallK          int     `json:"recall_k"`
	NoveltyThreshold float64 `json:"novelty_threshold"`
}

type ResearchConfig struct {
	RecallK          int
	NoveltyThreshold float64
}

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
