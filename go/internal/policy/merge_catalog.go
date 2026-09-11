package policy

type ParallelEvaluatePolicy struct {
	// Stage selects the rollout stage: "off" / "shadow" / "enforce".
	// Empty/absent ⇒ "off" (dispatcher dormant; byte-identical baseline).
	// Any unknown value (e.g. a typo) maps to "off" — a fail-safe so a
	// misspelling can never silently arm the parallel dispatcher.
	Stage string `json:"stage,omitempty"`
	// Concurrency bounds the parallel runner pool when Stage="enforce".
	// Zero/negative/absent ⇒ 3 (the soak sweet spot: ~11% saving, diminishing
	// past it). Applies only; use cases where an explicit cap is needed.
	Concurrency int `json:"concurrency,omitempty"`
}

// ParallelEvaluateConfig is the resolved parallel-evaluate configuration with
// defaults applied.
type ParallelEvaluateConfig struct {
	Stage       string
	Concurrency int
}

// ParallelEvaluateConfig returns parallel-evaluate configuration with built-in
// defaults resolved. The zero-value Policy{} yields safe defaults (stage="off",
// concurrency=3). Stage overrides apply only for the closed vocabulary
// {"off","shadow","advisory","enforce"}; any other value falls back to "off"
// (fail-safe). Concurrency overrides apply only when > 0. Pure.
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

// ClassifyPolicy configures the cycle-failure classifier (internal/cycleclassify).
// Loaded from .evolve/policy.json "classify" block; absent block ⇒ built-in
// defaults apply. Replaces EVOLVE_HANG_CLASSIFIER env read.
type ClassifyPolicy struct {
	// HangClassifier enables the two-factor exit-transport-hang reclassification:
	// when true, a SHIPPED-verdict report + matching git commit reclassifies an
	// apparent integrity-breach as ClassExitTransportHang (1h retention vs 7d).
	// Default false — hang detection is opt-in so a misconfigured git log never
	// silently masks a real breach.
	HangClassifier bool `json:"hang_classifier,omitempty"`
}

// ClassifyConfig returns the classifier configuration with defaults resolved.
// An absent block yields ClassifyPolicy{HangClassifier: false} — safe default.
func (p Policy) ClassifyConfig() ClassifyPolicy {
	if p.Classify == nil {
		return ClassifyPolicy{}
	}
	return *p.Classify
}

// CatalogPolicy configures the model catalog subsystem.
// Loaded from .evolve/policy.json "catalog" block; absent block ⇒ built-in
// defaults apply. Replaces EVOLVE_MODELCATALOG_AUTOREFRESH env read.
type CatalogPolicy struct {
	// AutoRefresh controls whether the cycle-start live model-catalog refresh
	// runs. Nil/absent ⇒ true (opt-out semantics: default on, set false to
	// disable). Replaces EVOLVE_MODELCATALOG_AUTOREFRESH=0.
	AutoRefresh *bool `json:"auto_refresh,omitempty"`

	// AllowedFamilies is a per-CLI model-family allow-list (e.g. agy:[gemini]).
	// A CLI's live-queried candidate ids are filtered to these families BEFORE
	// classification (modelquery.RefreshDeps threads this through FilterByFamily),
	// so a cross-family id can never reach the classifier (D7: "agy must not have
	// Claude models"). Nil/absent ⇒ no constraint — byte-identical to today for
	// every CLI that does not opt in.
	AllowedFamilies map[string][]string `json:"allowed_families,omitempty"`

	// RefreshStage stages the cycle-start refresh WRITE path: "off" (no
	// refresh), "shadow" (full live pipeline, writes model-catalog.shadow.json
	// + would-change diff lines, never the live catalog — dispatch is
	// byte-identical to off), "enforce" (writes the live catalog via Commit —
	// today's behavior when auto_refresh is on). Absent ⇒ derived from
	// AutoRefresh (true ⇒ enforce, false ⇒ off) so existing deployments keep
	// their exact behavior. Unknown values resolve to "off" — the closed-
	// vocabulary fail-safe (merge_gate precedent): a typo disables the write,
	// it never silently arms one.
	RefreshStage string `json:"refresh_stage,omitempty"`
}

// CatalogConfig returns the catalog configuration with defaults resolved.
// AutoRefresh defaults to true (opt-out); the returned pointer is never nil.
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
	// Pass AllowedFamilies through unchanged — nil stays nil ("no constraint");
	// never default to an empty map (callers/tests distinguish nil from empty).
	out.AllowedFamilies = p.Catalog.AllowedFamilies
	out.RefreshStage = resolveRefreshStage(p.Catalog.RefreshStage, *out.AutoRefresh)
	return out
}

// resolveRefreshStage maps the raw refresh_stage string to the resolved stage.
// Explicit known values pass through; absent derives from autoRefresh
// (back-compat: true ⇒ enforce, false ⇒ off); unknown fails safe to off.
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

// ACSConfig configures the ACS Go lane timeout.
// Loaded from .evolve/policy.json "acs" block; absent block ⇒ built-in
// defaults apply (DefaultTimeout=60s). Replaces EVOLVE_ACS_GO_TIMEOUT_S env read.
