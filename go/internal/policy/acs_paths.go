package policy

type ACSConfig struct {
	GoTimeoutS   int      `json:"go_timeout_s,omitempty"`
	PredicateEnv []string `json:"predicate_env,omitempty"`
}

func (p Policy) ACSTimeoutConfig() ACSConfig {
	if p.ACS == nil {
		return ACSConfig{}
	}
	return *p.ACS
}

type PathsConfig struct {
	KBSearchPaths string `json:"kb_search_paths,omitempty"`
	PhaseRoots    string `json:"phase_roots,omitempty"`
}

func (p Policy) PathsConfig() PathsConfig {
	if p.Paths == nil {
		return PathsConfig{}
	}
	return *p.Paths
}

const MaxContractCorrectionRetries = maxContractCorrectionRetries
