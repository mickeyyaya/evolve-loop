package guards

import (
	"context"
	"strings"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Quota enforces per-agent web-research caps. Phase-1: in-memory
// counters; Phase 2 persists into cycle-state.json:research_usage.
// Port of scripts/hooks/research-quota-gate.sh.
type Quota struct {
	cfg      QuotaConfig
	mu       sync.Mutex
	counters map[string]int // key = "agent|bucket"
}

// QuotaConfig defines per-bucket caps. Zero means use the bash defaults
// (web_search=3, web_fetch=5, kb_search=20). A negative value disables
// the bucket (always deny).
type QuotaConfig struct {
	WebSearch int
	WebFetch  int
	KbSearch  int
}

func NewQuota(cfg QuotaConfig) *Quota {
	if cfg.WebSearch == 0 {
		cfg.WebSearch = 3
	}
	if cfg.WebFetch == 0 {
		cfg.WebFetch = 5
	}
	if cfg.KbSearch == 0 {
		cfg.KbSearch = 20
	}
	return &Quota{cfg: cfg, counters: map[string]int{}}
}

func (q *Quota) Name() string { return "quota" }

func (q *Quota) Decide(_ context.Context, in core.GuardInput) core.GuardDecision {
	if envEnabled("EVOLVE_ALLOW_DEEP_RESEARCH") {
		return core.GuardDecision{Allow: true}
	}
	bucket, cap := q.bucketFor(in)
	if bucket == "" {
		return core.GuardDecision{Allow: true}
	}
	agent := strField(in, "agent")
	if agent == "" {
		agent = "unknown"
	}
	key := agent + "|" + bucket

	q.mu.Lock()
	defer q.mu.Unlock()
	if q.counters[key] >= cap {
		return core.GuardDecision{
			Allow: false,
			Reason: "research quota exceeded: agent=" + agent + " bucket=" + bucket +
				"; EVOLVE_ALLOW_DEEP_RESEARCH=1 to lift",
		}
	}
	q.counters[key]++
	return core.GuardDecision{Allow: true}
}

func (q *Quota) bucketFor(in core.GuardInput) (string, int) {
	switch in.ToolName {
	case "WebSearch":
		return "web_search", q.cfg.WebSearch
	case "WebFetch":
		return "web_fetch", q.cfg.WebFetch
	case "Bash":
		cmd := cmdString(in)
		if strings.Contains(cmd, "kb-search.sh") {
			return "kb_search", q.cfg.KbSearch
		}
	}
	return "", 0
}
