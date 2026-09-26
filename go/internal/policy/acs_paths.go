package policy

// ACSConfig is the "acs" block for the ACS Go lane timeout.
type ACSConfig struct {
	// GoTimeoutS overrides the Go lane timeout in seconds; 0 means DefaultTimeout.
	GoTimeoutS int `json:"go_timeout_s,omitempty"`
}

// ACSTimeoutConfig returns the acs block; callers must read GoTimeoutS=0 as DefaultTimeout, never a zero timeout.
func (p Policy) ACSTimeoutConfig() ACSConfig {
	if p.ACS == nil {
		return ACSConfig{}
	}
	return *p.ACS
}

// PathsConfig is the "paths" block of path-discovery overrides.
type PathsConfig struct {
	// KBSearchPaths is a colon-separated list of KB search roots; empty means
	// knowledge-base/research/:.evolve/instincts/lessons/:docs/research/.
	KBSearchPaths string `json:"kb_search_paths,omitempty"`
	// PhaseRoots is a colon-separated list of phase discovery roots; empty means .evolve/phases.
	PhaseRoots string `json:"phase_roots,omitempty"`
}

// PathsConfig returns the paths block, or the zero value (built-in paths) when absent.
func (p Policy) PathsConfig() PathsConfig {
	if p.Paths == nil {
		return PathsConfig{}
	}
	return *p.Paths
}

// MaxContractCorrectionRetries caps build correction re-dispatches; the size-budget scaler clamps to it too.
const MaxContractCorrectionRetries = maxContractCorrectionRetries
