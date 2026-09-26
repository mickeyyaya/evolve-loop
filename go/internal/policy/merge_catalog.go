package policy

// ParallelEvaluatePolicy is the "parallel_evaluate" block for post-build evaluate-phase parallelism.
type ParallelEvaluatePolicy struct {
	// Stage is "off" (default), "shadow", "advisory" or "enforce"; any other value maps to "off".
	Stage string `json:"stage,omitempty"`
	// Concurrency bounds the runner pool under "enforce"; non-positive means 3.
	Concurrency int `json:"concurrency,omitempty"`
}

// ParallelEvaluateConfig is the resolved parallel-evaluate configuration with defaults applied.
type ParallelEvaluateConfig struct {
	Stage       string
	Concurrency int
}

// ParallelEvaluateConfig returns parallel-evaluate configuration, defaulting to stage "off" and concurrency 3.
func (p Policy) ParallelEvaluateConfig() ParallelEvaluateConfig {
	c := ParallelEvaluateConfig{Stage: "off", Concurrency: 3}
	if p.ParallelEvaluate == nil {
		return c
	}
	if s := p.ParallelEvaluate.Stage; s != "" {
		switch s {
		case "off", "shadow", "advisory", "enforce":
			c.Stage = s
		default:
			c.Stage = "off"
		}
	}
	if p.ParallelEvaluate.Concurrency > 0 {
		c.Concurrency = p.ParallelEvaluate.Concurrency
	}
	return c
}

// RetryConfig returns retry configuration with defaults and safety bounds.
func (p Policy) RetryConfig() RetryConfig {
	c := RetryConfig{
		PhaseMaxAttempts:          defaultPhaseMaxAttempts,
		RetryBackoffBaseS:         defaultRetryBackoffBaseS,
		PhaseLatencyCeilingS:      defaultPhaseLatencyCeilingS,
		ContractCorrectionRetries: defaultContractCorrectionRetries,
	}
	if p.Retry == nil {
		return c
	}
	if p.Retry.PhaseMaxAttempts > 0 {
		c.PhaseMaxAttempts = min(p.Retry.PhaseMaxAttempts, maxPhaseMaxAttempts)
	}
	if p.Retry.retryBackoffBaseSSet {
		c.RetryBackoffBaseS = max(p.Retry.RetryBackoffBaseS, 0)
	} else if p.Retry.RetryBackoffBaseS > 0 {
		c.RetryBackoffBaseS = p.Retry.RetryBackoffBaseS
	}
	if p.Retry.PhaseLatencyCeilingS > 0 {
		c.PhaseLatencyCeilingS = p.Retry.PhaseLatencyCeilingS
	}
	if p.Retry.contractCorrectionRetriesSet && p.Retry.ContractCorrectionRetries >= 0 {
		c.ContractCorrectionRetries = min(p.Retry.ContractCorrectionRetries, maxContractCorrectionRetries)
	} else if p.Retry.ContractCorrectionRetries > 0 {
		c.ContractCorrectionRetries = min(p.Retry.ContractCorrectionRetries, maxContractCorrectionRetries)
	}
	return c
}

// ClassifyPolicy is the "classify" block for the cycle-failure classifier.
type ClassifyPolicy struct {
	// HangClassifier (opt-in) reclassifies an apparent integrity breach as an
	// exit-transport hang when a SHIPPED report and a matching commit both exist.
	HangClassifier bool `json:"hang_classifier,omitempty"`
}

// ClassifyConfig returns the classify block, or the zero value (classifier off) when absent.
func (p Policy) ClassifyConfig() ClassifyPolicy {
	if p.Classify == nil {
		return ClassifyPolicy{}
	}
	return *p.Classify
}

// CatalogPolicy is the "catalog" block for the model catalog.
type CatalogPolicy struct {
	// AutoRefresh runs the cycle-start live refresh; nil means true.
	AutoRefresh *bool `json:"auto_refresh,omitempty"`

	// AllowedFamilies restricts each CLI's live candidate ids to these model families
	// before classification; nil means no constraint.
	AllowedFamilies map[string][]string `json:"allowed_families,omitempty"`

	// RefreshStage is "off", "shadow" (write model-catalog.shadow.json only) or
	// "enforce"; absent derives from AutoRefresh and an unknown value is "off".
	RefreshStage string `json:"refresh_stage,omitempty"`
}

// CatalogConfig returns the catalog block with defaults resolved; AutoRefresh is never nil.
func (p Policy) CatalogConfig() CatalogPolicy {
	enabled := true
	out := CatalogPolicy{AutoRefresh: &enabled}
	if p.Catalog == nil {
		out.RefreshStage = resolveRefreshStage("", enabled)
		return out
	}
	if p.Catalog.AutoRefresh != nil {
		out.AutoRefresh = p.Catalog.AutoRefresh
	}
	// Callers distinguish nil ("no constraint") from an empty map, so nil is passed through.
	out.AllowedFamilies = p.Catalog.AllowedFamilies
	out.RefreshStage = resolveRefreshStage(p.Catalog.RefreshStage, *out.AutoRefresh)
	return out
}

func resolveRefreshStage(raw string, autoRefresh bool) string {
	switch raw {
	case "off", "shadow", "enforce":
		return raw
	case "":
		if autoRefresh {
			return "enforce"
		}
		return "off"
	default:
		return "off"
	}
}
