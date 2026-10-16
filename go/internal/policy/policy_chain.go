package policy

const DefaultChainMaxBatches = 20

type ChainPolicy struct {
	Enabled    *bool `json:"enabled,omitempty"`
	MaxBatches int   `json:"max_batches,omitempty"`
}

type ChainConfig struct {
	Enabled    bool
	MaxBatches int
}

func (p Policy) ChainConfig() ChainConfig {
	c := ChainConfig{MaxBatches: DefaultChainMaxBatches}
	if p.Chain == nil {
		return c
	}
	if p.Chain.Enabled != nil {
		c.Enabled = *p.Chain.Enabled
	}
	if p.Chain.MaxBatches > 0 {
		c.MaxBatches = p.Chain.MaxBatches
	}
	return c
}
