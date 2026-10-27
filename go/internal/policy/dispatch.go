package policy

import (
	"slices"
	"time"
)

type FanoutPolicy struct {
	Concurrency        int    `json:"concurrency,omitempty"`
	TimeoutSecs        int    `json:"timeout_secs,omitempty"`
	CancelOnConsensus  bool   `json:"cancel_on_consensus,omitempty"`
	ConsensusK         int    `json:"consensus_k,omitempty"`
	ConsensusPollSecs  int    `json:"consensus_poll_secs,omitempty"`
	TrackWorkers       *bool  `json:"track_workers,omitempty"`
	CachePrefixEnabled *bool  `json:"cache_prefix_enabled,omitempty"`
	TestExecutor       string `json:"test_executor,omitempty"`
}

func (p Policy) FanoutConfig() FanoutPolicy {
	tw, cp := true, true
	out := FanoutPolicy{
		Concurrency:        2,
		TrackWorkers:       &tw,
		CachePrefixEnabled: &cp,
	}
	f := p.Fanout
	if f == nil {
		return out
	}
	if f.Concurrency >= 1 {
		out.Concurrency = f.Concurrency
	}
	out.TimeoutSecs = f.TimeoutSecs
	out.CancelOnConsensus = f.CancelOnConsensus
	out.ConsensusK = f.ConsensusK
	out.ConsensusPollSecs = f.ConsensusPollSecs
	if f.TrackWorkers != nil {
		out.TrackWorkers = f.TrackWorkers
	}
	if f.CachePrefixEnabled != nil {
		out.CachePrefixEnabled = f.CachePrefixEnabled
	}
	out.TestExecutor = f.TestExecutor
	return out
}

type ObserverPolicy struct {
	Autospawn        *bool  `json:"autospawn,omitempty"`
	PollS            *int   `json:"poll_s,omitempty"`
	StallS           *int   `json:"stall_s,omitempty"`
	NudgeS           *int   `json:"nudge_s,omitempty"`
	NudgeBody        string `json:"nudge_body,omitempty"`
	EOFGraceS        int    `json:"eof_grace_s,omitempty"`
	WatchdogPollS    *int   `json:"watchdog_poll_s,omitempty"`
	WatchdogWarnPct  *int   `json:"watchdog_warn_pct,omitempty"`
	WatchdogGraceS   *int   `json:"watchdog_grace_s,omitempty"`
	WatchdogDisabled bool   `json:"watchdog_disabled,omitempty"`
}

func (p Policy) ObserverConfig() ObserverPolicy {
	autospawn, pollS, stallS, nudgeS := true, 5, 600, 300
	watchdogPollS, watchdogWarnPct, watchdogGraceS := 15, 75, 10
	out := ObserverPolicy{
		Autospawn:       &autospawn,
		PollS:           &pollS,
		StallS:          &stallS,
		NudgeS:          &nudgeS,
		WatchdogPollS:   &watchdogPollS,
		WatchdogWarnPct: &watchdogWarnPct,
		WatchdogGraceS:  &watchdogGraceS,
	}
	if p.Observer == nil {
		return out
	}
	o := p.Observer
	if o.Autospawn != nil {
		out.Autospawn = o.Autospawn
	}
	if o.PollS != nil {
		out.PollS = o.PollS
	}
	if o.StallS != nil {
		out.StallS = o.StallS
	}
	if o.NudgeS != nil {
		out.NudgeS = o.NudgeS
	}
	out.NudgeBody = o.NudgeBody
	out.EOFGraceS = o.EOFGraceS
	if o.WatchdogPollS != nil {
		out.WatchdogPollS = o.WatchdogPollS
	}
	if o.WatchdogWarnPct != nil {
		out.WatchdogWarnPct = o.WatchdogWarnPct
	}
	if o.WatchdogGraceS != nil {
		out.WatchdogGraceS = o.WatchdogGraceS
	}
	out.WatchdogDisabled = o.WatchdogDisabled
	return out
}

type BridgePolicy struct {
	ManifestDir           string         `json:"manifest_dir,omitempty"`
	CatalogDir            string         `json:"catalog_dir,omitempty"`
	RecipeDir             string         `json:"recipe_dir,omitempty"`
	BootTimeoutS          int            `json:"boot_timeout_s,omitempty"`
	ArtifactTimeoutS      int            `json:"artifact_timeout_s,omitempty"`
	ArtifactMaxExtends    int            `json:"artifact_max_extends,omitempty"`
	ScrollbackLines       int            `json:"scrollback_lines,omitempty"`
	PhaseArtifactTimeoutS map[string]int `json:"phase_artifact_timeout_s,omitempty"`
	AnthropicBaseURL      string         `json:"anthropic_base_url,omitempty"`
	ModelFailover         string         `json:"model_failover,omitempty"`
	MaxConcurrent         map[string]int `json:"max_concurrent,omitempty"`
}

const ModelFailoverStageOn = "on"

func (b BridgePolicy) ModelFailoverOn() bool { return b.ModelFailover == ModelFailoverStageOn }

const (
	effortLow    = "low"
	effortMedium = "medium"
	effortHigh   = "high"
	effortXHigh  = "xhigh"
	effortMax    = "max"
)

var effortLevels = []string{effortLow, effortMedium, effortHigh, effortXHigh, effortMax}

func EffortLevels() []string {
	return slices.Clone(effortLevels)
}

var defaultTierEffort = map[string]string{
	"fast":     effortLow,
	"balanced": effortMedium,
	"deep":     effortMedium,
	"top":      effortMedium,
}

var defaultPhaseArtifactTimeoutS = map[string]int{
	"retrospective":      900,
	"retro":              900,
	"tdd":                1200,
	"tdd-engineer":       1200,
	"build":              1200,
	"builder":            1200,
	"audit":              1200,
	"auditor":            1200,
	"adversarial-review": 1200,
}

func (p BridgePolicy) PhaseArtifactTimeouts() map[string]int {
	out := make(map[string]int, len(defaultPhaseArtifactTimeoutS)+len(p.PhaseArtifactTimeoutS))
	for phase, budget := range defaultPhaseArtifactTimeoutS {
		out[phase] = budget
	}
	for phase, budget := range p.PhaseArtifactTimeoutS {
		if budget > 0 {
			out[phase] = budget
		}
	}
	for label := range legacyAgentLabels {
		if p.PhaseArtifactTimeoutS[label] > 0 {
			continue
		}
		if budget, ok := legacyLabelValue(p.PhaseArtifactTimeoutS, "phase_artifact_timeout_s", label, func(b int) bool { return b > 0 }); ok {
			out[label] = budget
		}
	}
	return out
}

func (p Policy) BridgeConfig() BridgePolicy {
	if p.Bridge == nil {
		return BridgePolicy{}
	}
	return *p.Bridge
}

type QuotaResetConfig struct {
	ResetAt      string  `json:"reset_at,omitempty"`
	DefaultHours float64 `json:"default_hours,omitempty"`
}

func (p Policy) QuotaResetConfig() QuotaResetConfig {
	if p.QuotaReset == nil {
		return QuotaResetConfig{}
	}
	return *p.QuotaReset
}

type CLIHealthConfig struct {
	ProactiveProbe        bool  `json:"proactive_probe,omitempty"`
	UsageEvidenceTimeoutS int   `json:"usage_evidence_timeout_s,omitempty"`
	UsageEvidenceTTLS     int   `json:"usage_evidence_ttl_s,omitempty"`
	Enabled               *bool `json:"enabled,omitempty"`
}

func (c CLIHealthConfig) On() bool { return c.Enabled == nil || *c.Enabled }

const (
	defaultUsageEvidenceTimeout = 2 * time.Minute
	defaultUsageEvidenceTTL     = 10 * time.Minute
)

func (c CLIHealthConfig) UsageEvidenceTimeout() time.Duration {
	return secondsOr(c.UsageEvidenceTimeoutS, defaultUsageEvidenceTimeout)
}

func (c CLIHealthConfig) UsageEvidenceTTL() time.Duration {
	return secondsOr(c.UsageEvidenceTTLS, defaultUsageEvidenceTTL)
}

func secondsOr(seconds int, fallback time.Duration) time.Duration {
	if seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func (p Policy) CLIHealthConfig() CLIHealthConfig {
	if p.CLIHealth == nil {
		return CLIHealthConfig{}
	}
	return *p.CLIHealth
}

type DispatchConfig struct {
	Policy          string `json:"policy,omitempty"`
	RepeatThreshold int    `json:"repeat_threshold,omitempty"`
}

const defaultDispatchRepeatThreshold = 5

func (p Policy) DispatchConfig() DispatchConfig {
	c := DispatchConfig{Policy: "verify", RepeatThreshold: defaultDispatchRepeatThreshold}
	if p.Dispatch == nil {
		return c
	}
	if p.Dispatch.Policy != "" {
		c.Policy = p.Dispatch.Policy
	}
	if p.Dispatch.RepeatThreshold > 0 {
		c.RepeatThreshold = p.Dispatch.RepeatThreshold
	}
	return c
}
