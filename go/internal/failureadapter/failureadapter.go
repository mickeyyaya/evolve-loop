// Package failureadapter decides how the pipeline adapts to recorded failures.
// See docs/architecture/packages/internal-failureadapter.md.
package failureadapter

import (
	"fmt"
	"strings"
	"time"
)

type Classification string

const (
	InfraTransient    Classification = "infrastructure-transient"
	InfraSystemic     Classification = "infrastructure-systemic"
	IntentMalformed   Classification = "intent-malformed"
	IntentRejected    Classification = "intent-rejected"
	CodeBuildFail     Classification = "code-build-fail"
	CodeAuditFail     Classification = "code-audit-fail"
	CodeAuditWarn     Classification = "code-audit-warn"
	ShipGateConfig    Classification = "ship-gate-config"
	HumanAbort        Classification = "human-abort"
	ExitTransportHang Classification = "exit-transport-hang"
	IntegrityBreach   Classification = "integrity-breach"
)

type Action string

const (
	ActionProceed             Action = "PROCEED"
	ActionRetryWithFallback   Action = "RETRY-WITH-FALLBACK"
	ActionBlockCode           Action = "BLOCK-CODE"
	ActionBlockOperatorAction Action = "BLOCK-OPERATOR-ACTION"
)

type Verdict string

const (
	VerdictNone                      Verdict = ""
	VerdictScopeRejected             Verdict = "SCOPE-REJECTED"
	VerdictBlockedSystemic           Verdict = "BLOCKED-SYSTEMIC"
	VerdictBlockedRecurringAuditFail Verdict = "BLOCKED-RECURRING-AUDIT-FAIL"
	VerdictBlockedRecurringBuildFail Verdict = "BLOCKED-RECURRING-BUILD-FAIL"
)

type Entry struct {
	TS                string         `json:"ts,omitempty"`
	Cycle             int            `json:"cycle,omitempty"`
	Verdict           string         `json:"verdict,omitempty"`
	Classification    Classification `json:"classification,omitempty"`
	RecordedAt        string         `json:"recordedAt,omitempty"`
	ExpiresAt         string         `json:"expiresAt,omitempty"`
	AuditReportPath   string         `json:"auditReportPath,omitempty"`
	AuditReportSHA256 string         `json:"auditReportSha256,omitempty"`
	GitHead           string         `json:"gitHead,omitempty"`
	TreeStateSHA      string         `json:"treeStateSha,omitempty"`
	Defects           []string       `json:"defects,omitempty"`
	Retrospected      bool           `json:"retrospected,omitempty"`
	Summary           string         `json:"summary,omitempty"`
}

type Decision struct {
	Action          Action            `json:"action"`
	Reason          string            `json:"reason"`
	Remediation     string            `json:"remediation,omitempty"`
	SetEnv          map[string]string `json:"set_env,omitempty"`
	SkipPhases      []string          `json:"skip_phases,omitempty"`
	VerdictForBlock Verdict           `json:"verdict_for_block,omitempty"`
	Evidence        Evidence          `json:"evidence"`
}

type Evidence struct {
	NonExpiredCount                 int            `json:"non_expired_count"`
	ByClass                         map[string]int `json:"by_class"`
	ConsecutiveInfraTransientStreak int            `json:"consecutive_infra_transient_streak"`
}

type Options struct {
	Strict bool
	Now    time.Time
}

func AgeOutSeconds(c Classification) int64 {
	switch c {
	case InfraTransient, IntentMalformed, CodeAuditWarn, ShipGateConfig:
		return 86400
	case InfraSystemic, IntegrityBreach:
		return 604800
	case IntentRejected:
		return 999999999
	case CodeBuildFail, CodeAuditFail:
		return 2592000
	case HumanAbort, ExitTransportHang:
		return 3600
	default:
		return 86400
	}
}

func Decide(entries []Entry, opts Options) Decision {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	nonExpired := make([]Entry, 0, len(entries))
	for _, e := range entries {
		if isNonExpired(e, now) {
			nonExpired = append(nonExpired, e)
		}
	}

	ev := Evidence{
		NonExpiredCount:                 len(nonExpired),
		ByClass:                         countByClass(nonExpired),
		ConsecutiveInfraTransientStreak: tailInfraTransientStreak(nonExpired),
	}

	auditFailDistinct := distinctCyclesByClass(nonExpired, CodeAuditFail)
	buildFailDistinct := distinctCyclesByClass(nonExpired, CodeBuildFail)

	awareness := make([]string, 0)

	if ev.ByClass[string(IntentRejected)] > 0 {
		reason := fmt.Sprintf("%d prior intent-rejected (out-of-scope IBTC)",
			ev.ByClass[string(IntentRejected)])
		remediation := "Refine the goal description to be in-scope, then re-run /evo:loop."
		if opts.Strict {
			return Decision{
				Action: ActionBlockCode, Reason: reason, Remediation: remediation,
				VerdictForBlock: VerdictScopeRejected, Evidence: ev,
			}
		}
		awareness = append(awareness, "would-have-blocked: BLOCK-CODE — "+reason)
	}

	if n := ev.ByClass[string(InfraSystemic)]; n > 0 {
		summary := lastSummaryOf(nonExpired, InfraSystemic)
		if summary == "" {
			summary = "(no summary)"
		}
		reason := fmt.Sprintf("%d non-expired infrastructure-systemic failure(s); last summary: %s", n, summary)
		remediation := "Investigate the systemic infrastructure issue (tooling, host, claude-cli). Use scripts/failure/state-prune.sh --classification infrastructure-systemic after fixing."
		if opts.Strict {
			return Decision{
				Action: ActionBlockOperatorAction, Reason: reason, Remediation: remediation,
				VerdictForBlock: VerdictBlockedSystemic, Evidence: ev,
			}
		}
		awareness = append(awareness, "would-have-blocked: BLOCK-OPERATOR-ACTION — "+reason)
	}

	if auditFailDistinct >= 2 {
		reason := fmt.Sprintf("%d non-expired code-audit-fail entries (within 30d retention)", auditFailDistinct)
		remediation := "Auditor has rejected code N times. Pick a materially different task or prune via scripts/failure/state-prune.sh --classification code-audit-fail after addressing root cause."
		if opts.Strict {
			return Decision{
				Action: ActionBlockCode, Reason: reason, Remediation: remediation,
				VerdictForBlock: VerdictBlockedRecurringAuditFail, Evidence: ev,
			}
		}
		awareness = append(awareness, "would-have-blocked: BLOCK-CODE — "+reason)
	}

	if buildFailDistinct >= 2 {
		reason := fmt.Sprintf("%d non-expired code-build-fail entries (within 30d retention)", buildFailDistinct)
		remediation := "Builder has failed to compile/test N times. Pick a materially different task or prune via scripts/failure/state-prune.sh --classification code-build-fail."
		if opts.Strict {
			return Decision{
				Action: ActionBlockCode, Reason: reason, Remediation: remediation,
				VerdictForBlock: VerdictBlockedRecurringBuildFail, Evidence: ev,
			}
		}
		awareness = append(awareness, "would-have-blocked: BLOCK-CODE — "+reason)
	}

	if ev.ConsecutiveInfraTransientStreak >= 3 {
		reason := fmt.Sprintf("%d consecutive infrastructure-transient failures despite EPERM-fallback.",
			ev.ConsecutiveInfraTransientStreak)
		remediation := "Either: (1) run /evo:loop from a non-sandboxed terminal, OR (2) run scripts/failure/state-prune.sh --classification infrastructure-transient after confirming the underlying issue is resolved, OR (3) file an issue with cycle ledger entry."
		if opts.Strict {
			return Decision{
				Action: ActionBlockOperatorAction, Reason: reason, Remediation: remediation,
				VerdictForBlock: VerdictBlockedSystemic, Evidence: ev,
			}
		}
		awareness = append(awareness, "would-have-blocked: BLOCK-OPERATOR-ACTION — "+reason)
	}

	if n := ev.ByClass[string(InfraTransient)]; n > 0 {
		if opts.Strict {
			return Decision{
				Action:   ActionRetryWithFallback,
				Reason:   fmt.Sprintf("%d prior infrastructure-transient (within 1d retention); attempting with EPERM fallback enabled", n),
				Evidence: ev,
			}
		}
		awareness = append(awareness, fmt.Sprintf("infra-transient: %d prior; EPERM fallback enabled", n))
	}

	if len(awareness) > 0 {
		reason := "fluent mode (set workflow.strict_audit in policy.json for legacy blocking): " + strings.Join(awareness, "; ")
		return Decision{
			Action:      ActionProceed,
			Reason:      reason,
			Remediation: "Awareness only — orchestrator should consider the prior failures when planning. Set workflow.strict_audit in .evolve/policy.json to restore legacy block-on-recurring-failure behavior.",
			Evidence:    ev,
		}
	}
	return Decision{
		Action:   ActionProceed,
		Reason:   fmt.Sprintf("no recent failures requiring adaptation (non-expired count=%d)", ev.NonExpiredCount),
		Evidence: ev,
	}
}

func isNonExpired(e Entry, now time.Time) bool {
	if e.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, e.ExpiresAt)
		if err != nil {
			return true
		}
		return t.After(now)
	}
	if e.RecordedAt == "" {
		return true
	}
	rt, err := time.Parse(time.RFC3339, e.RecordedAt)
	if err != nil {
		return false
	}
	return rt.Add(24 * time.Hour).After(now)
}

func countByClass(entries []Entry) map[string]int {
	m := make(map[string]int, len(entries))
	for _, e := range entries {
		c := string(e.Classification)
		if c == "" {
			c = "unknown-classification"
		}
		m[c]++
	}
	return m
}

func distinctCyclesByClass(entries []Entry, target Classification) int {
	seen := map[int]struct{}{}
	for _, e := range entries {
		if e.Classification == target {
			seen[e.Cycle] = struct{}{}
		}
	}
	return len(seen)
}

func tailInfraTransientStreak(entries []Entry) int {
	n := 0
	for i := len(entries) - 1; i >= 0; i-- {
		c := entries[i].Classification
		if c == InfraTransient || c == "infrastructure" {
			n++
		} else {
			break
		}
	}
	return n
}

func lastSummaryOf(entries []Entry, class Classification) string {
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Classification == class {
			s := entries[i].Summary
			if len(s) > 200 {
				s = s[:200]
			}
			return s
		}
	}
	return ""
}
