package policy

type ACSConfig struct {
	// GoTimeoutS overrides the whole-Go-lane timeout in seconds. 0 = use DefaultTimeout.
	GoTimeoutS int `json:"go_timeout_s,omitempty"`
}

// ACSTimeoutConfig returns the ACS timeout configuration.
// An absent block returns ACSConfig{GoTimeoutS:0} — callers must treat 0 as
// "use DefaultTimeout" to avoid a zero-duration timeout.
func (p Policy) ACSTimeoutConfig() ACSConfig {
	if p.ACS == nil {
		return ACSConfig{}
	}
	return *p.ACS
}

// PathsConfig configures path-discovery overrides.
// Loaded from .evolve/policy.json "paths" block; absent block ⇒ built-in
// defaults apply. Replaces EVOLVE_KB_SEARCH_PATHS and EVOLVE_PHASE_ROOTS reads.
type PathsConfig struct {
	// KBSearchPaths is a colon-separated list of KB search roots.
	// Empty ⇒ built-in default (knowledge-base/research/:.evolve/instincts/lessons/:docs/research/).
	// Replaces EVOLVE_KB_SEARCH_PATHS.
	KBSearchPaths string `json:"kb_search_paths,omitempty"`
	// PhaseRoots is a colon-separated list of phase discovery roots.
	// Empty ⇒ built-in default (.evolve/phases). Replaces EVOLVE_PHASE_ROOTS.
	PhaseRoots string `json:"phase_roots,omitempty"`
}

// PathsConfig returns the paths configuration.
// An absent block returns PathsConfig{} — callers fall back to built-in defaults.
func (p Policy) PathsConfig() PathsConfig {
	if p.Paths == nil {
		return PathsConfig{}
	}
	return *p.Paths
}

// MaxContractCorrectionRetries is the hard ceiling on build correction
// re-dispatches — exported for the ADR-0076 size-budget scaler, which must
// clamp its scaled limit to the same bound the resolver enforces.
const MaxContractCorrectionRetries = maxContractCorrectionRetries
