package policy

type ParallelEvaluatePolicy struct {
	Stage       string `json:"stage,omitempty"`
	Concurrency int    `json:"concurrency,omitempty"`
}

type ParallelEvaluateConfig struct {
	Stage       string
	Concurrency int
}

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

type ClassifyPolicy struct {
	HangClassifier bool `json:"hang_classifier,omitempty"`
}

func (p Policy) ClassifyConfig() ClassifyPolicy {
	if p.Classify == nil {
		return ClassifyPolicy{}
	}
	return *p.Classify
}

type CatalogPolicy struct {
	AutoRefresh *bool `json:"auto_refresh,omitempty"`

	AllowedFamilies map[string][]string `json:"allowed_families,omitempty"`

	RefreshStage string `json:"refresh_stage,omitempty"`
}

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
