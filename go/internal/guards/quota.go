package guards

import (
	"context"
	"strings"
	"sync"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

// Quota caps research tool calls per agent and bucket, counting in memory for the life of the value.
type Quota struct {
	cfg      QuotaConfig
	mu       sync.Mutex
	counters map[string]int // key = "agent|bucket"
}

// QuotaConfig sets per-bucket caps: zero takes the default (3, 5, 20) and a negative cap always denies.
type QuotaConfig struct {
	WebSearch         int
	WebFetch          int
	KbSearch          int
	AllowDeepResearch bool
}

// NewQuota returns a Quota guard with zero caps replaced by their defaults.
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

// Name reports "quota".
func (q *Quota) Name() string { return "quota" }

// Decide counts a research call against its agent's bucket and denies once the cap is reached.
func (q *Quota) Decide(_ context.Context, in core.GuardInput) core.GuardDecision {
	if q.cfg.AllowDeepResearch {
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
				"; set workflow.allow_deep_research=true to lift",
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
