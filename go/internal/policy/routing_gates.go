package policy

import (
	"maps"
	"path/filepath"
	"time"
)

const defaultObservationMaskWindow = 10

type ObservationMaskPolicy struct {
	WindowTurns int `json:"window_turns,omitempty"`
}

func (p Policy) ObservationMaskConfig() ObservationMaskPolicy {
	c := ObservationMaskPolicy{WindowTurns: defaultObservationMaskWindow}
	if p.ObservationMask != nil && p.ObservationMask.WindowTurns > 0 {
		c.WindowTurns = p.ObservationMask.WindowTurns
	}
	return c
}

type RouterPolicy struct {
	RouterReplan string `json:"router_replan,omitempty"`
	RoutingJudge bool   `json:"routing_judge,omitempty"`
	ReconDigest  bool   `json:"recon_digest,omitempty"`
	ReplanDepth  int    `json:"replan_depth,omitempty"`
	PlanModel    string `json:"plan_model,omitempty"`
	ProposeModel string `json:"propose_model,omitempty"`
	CLI          string `json:"cli,omitempty"`
	Model        string `json:"model,omitempty"`

	DynamicRouting        string            `json:"dynamic_routing,omitempty"`
	RoutingMode           string            `json:"routing_mode,omitempty"`
	ConditionalMandatory  map[string]string `json:"conditional_mandatory,omitempty"`
	MaxOptionalInsertions *int              `json:"max_optional_insertions,omitempty"`
}

func (p Policy) RouterConfig() RouterPolicy {
	c := RouterPolicy{RouterReplan: "shadow", ReplanDepth: 1}
	if p.Router == nil {
		return c
	}
	if p.Router.RouterReplan != "" {
		c.RouterReplan = p.Router.RouterReplan
	}
	c.RoutingJudge = p.Router.RoutingJudge
	c.ReconDigest = p.Router.ReconDigest
	if p.Router.ReplanDepth > 0 {
		c.ReplanDepth = p.Router.ReplanDepth
	}
	c.PlanModel = p.Router.PlanModel
	c.ProposeModel = p.Router.ProposeModel
	c.CLI = p.Router.CLI
	c.Model = p.Router.Model
	c.DynamicRouting = p.Router.DynamicRouting
	c.RoutingMode = p.Router.RoutingMode
	c.ConditionalMandatory = maps.Clone(p.Router.ConditionalMandatory)
	c.MaxOptionalInsertions = p.Router.MaxOptionalInsertions
	return c
}

type GatesPolicy struct {
	ContractGate   string `json:"contract_gate,omitempty"`
	EvalGate       string `json:"eval_gate,omitempty"`
	TriageCapGate  string `json:"triage_cap_gate,omitempty"`
	ReviewGate     string `json:"review_gate,omitempty"`
	TopNGate       string `json:"topn_gate,omitempty"`
	ReportSizeGate string `json:"report_size_gate,omitempty"`
	ManifestGate   string `json:"manifest_gate,omitempty"`

	RepoContractGate string `json:"repo_contract_gate,omitempty"`

	ScanCacheGate          string `json:"scan_cache_gate,omitempty"`
	ScanCacheMaxMB         int    `json:"scan_cache_max_mb,omitempty"`
	ScanCacheBudgetSeconds int    `json:"scan_cache_budget_seconds,omitempty"`

	PhaseIO string `json:"phase_io,omitempty"`
}

type GatesConfig struct {
	ContractGate     string
	EvalGate         string
	TriageCapGate    string
	ReviewGate       string
	ReportSizeGate   string
	TopNGate         string
	ManifestGate     string
	RepoContractGate string
	ScanCacheGate    string

	ScanCacheMaxMB         int
	ScanCacheBudgetSeconds int

	PhaseIO string
}

func (p Policy) GatesConfig() GatesConfig {
	c := GatesConfig{
		ContractGate:     "enforce",
		EvalGate:         "enforce",
		TriageCapGate:    "enforce",
		ReviewGate:       "off",
		ReportSizeGate:   "shadow",
		TopNGate:         "enforce",
		ManifestGate:     "shadow",
		RepoContractGate: "enforce",
		ScanCacheGate:    "shadow",

		ScanCacheMaxMB:         defaultScanCacheMaxMB,
		ScanCacheBudgetSeconds: defaultScanCacheBudgetSeconds,
	}
	if p.Gates == nil {
		return c
	}
	if p.Gates.ContractGate != "" {
		c.ContractGate = p.Gates.ContractGate
	}
	if p.Gates.EvalGate != "" {
		c.EvalGate = p.Gates.EvalGate
	}
	if p.Gates.TriageCapGate != "" {
		c.TriageCapGate = p.Gates.TriageCapGate
	}
	if p.Gates.ReviewGate != "" {
		c.ReviewGate = p.Gates.ReviewGate
	}
	if p.Gates.ReportSizeGate != "" {
		c.ReportSizeGate = p.Gates.ReportSizeGate
	}
	if p.Gates.TopNGate != "" {
		c.TopNGate = p.Gates.TopNGate
	}
	if p.Gates.ManifestGate != "" {
		c.ManifestGate = p.Gates.ManifestGate
	}
	if p.Gates.RepoContractGate != "" {
		c.RepoContractGate = p.Gates.RepoContractGate
	}
	c.applyScanCache(p.Gates)
	c.PhaseIO = p.Gates.PhaseIO
	return c
}

func (c *GatesConfig) applyScanCache(g *GatesPolicy) {
	if g.ScanCacheGate != "" {
		c.ScanCacheGate = g.ScanCacheGate
	}
	if g.ScanCacheMaxMB > 0 {
		c.ScanCacheMaxMB = g.ScanCacheMaxMB
	}
	if g.ScanCacheBudgetSeconds > 0 {
		c.ScanCacheBudgetSeconds = g.ScanCacheBudgetSeconds
	}
}

const (
	defaultScanCacheMaxMB         = 2048
	defaultScanCacheBudgetSeconds = 60
)

type ScanCacheConfig struct {
	Gate     string
	MaxBytes int64
	Budget   time.Duration
}

func ScanCacheFor(projectRoot string) ScanCacheConfig {
	pol, err := Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	if err != nil {
		pol = Policy{}
	}
	g := pol.GatesConfig()
	return ScanCacheConfig{Gate: g.ScanCacheGate, MaxBytes: int64(g.ScanCacheMaxMB) << 20, Budget: time.Duration(g.ScanCacheBudgetSeconds) * time.Second}
}
