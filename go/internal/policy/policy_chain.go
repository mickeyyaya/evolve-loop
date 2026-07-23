package policy

// policy_chain.go — the `chain` block: batch-chaining defaults for
// `evolve loop --until-inbox-empty` (cycle 1075, inbox item
// loop-batch-chaining). Chaining is an operator CAPABILITY, not a feature
// flag: the CLI parameter is the opt-in and this block supplies its bound, in
// the same compiled-default-then-policy-override shape as `workflow`/`gates`.

// DefaultChainMaxBatches is the compiled-in ceiling on chained batches when
// policy.json supplies no (or a non-positive) chain.max_batches. It is a
// runaway backstop, not a target: a healthy chain stops because the inbox
// drained, not because it hit the cap. Positive by construction — a 0 cap
// would silently disable chaining for an operator who asked for it.
const DefaultChainMaxBatches = 20

// ChainPolicy is the raw `chain` block as written in .evolve/policy.json.
// Absent ⇒ ChainConfig's compiled defaults (chaining off, default cap).
type ChainPolicy struct {
	// Enabled turns chain mode on without the CLI parameter. Pointer so an
	// explicit `false` is distinguishable from an absent key.
	Enabled *bool `json:"enabled,omitempty"`
	// MaxBatches caps how many batches one chained invocation may run.
	// Non-positive ⇒ DefaultChainMaxBatches.
	MaxBatches int `json:"max_batches,omitempty"`
}

// ChainConfig is the resolved chain configuration with defaults applied.
type ChainConfig struct {
	// Enabled is the policy-side chain opt-in. The CLI parameter
	// (--until-inbox-empty) is ORed with this at the dispatcher; chaining is
	// never on unless one of the two explicitly asked for it.
	Enabled bool
	// MaxBatches is the resolved, always-positive batch ceiling.
	MaxBatches int
}

// ChainConfig returns the chain configuration with built-in defaults resolved,
// mirroring WorkflowConfig: an absent block yields chaining OFF with the
// positive compiled cap, and a non-positive max_batches falls back to that cap
// rather than resolving to a 0 that would disable chaining outright.
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
