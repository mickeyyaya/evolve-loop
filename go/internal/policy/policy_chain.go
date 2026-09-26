package policy

// DefaultChainMaxBatches is the runaway backstop for chained batches; it must stay positive or chaining silently stops.
const DefaultChainMaxBatches = 20

// ChainPolicy is the "chain" block for `evolve loop --until-inbox-empty`.
type ChainPolicy struct {
	// Enabled turns chaining on without the CLI parameter; a pointer keeps explicit false distinct from absent.
	Enabled *bool `json:"enabled,omitempty"`
	// MaxBatches caps one chained invocation; non-positive means DefaultChainMaxBatches.
	MaxBatches int `json:"max_batches,omitempty"`
}

// ChainConfig is the resolved chain configuration with defaults applied.
type ChainConfig struct {
	// Enabled is ORed with --until-inbox-empty at the dispatcher.
	Enabled    bool
	MaxBatches int
}

// ChainConfig returns the chain block, defaulting to chaining off with the positive compiled cap.
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
