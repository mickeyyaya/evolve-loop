// Package failureadapter is the Go port of scripts/failure/failure-adapter.sh
// — the deterministic failure-adaptation kernel introduced in v8.22.0.
//
// It is a PURE FUNCTION over a slice of failure entries. No I/O; no
// process spawning; safe to call from any phase. The orchestrator (or
// CLI subcommand wrapper) reads state.json:failedApproaches, hands the
// entries to Decide(), and follows the resulting Decision verbatim.
//
// The bash source remains canonical until the v11.0 cutover; both
// implementations must agree on every input. See failureadapter_test.go
// for the bash-source line references on each rule.
package failureadapter

import (
	"fmt"
	"strings"
	"time"
)

// Classification mirrors the v8.22.0+ taxonomy at
// scripts/failure/failure-classifications.sh:66-86.
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

// Action is the top-level outcome of Decide.
type Action string

const (
	ActionProceed             Action = "PROCEED"
	ActionRetryWithFallback   Action = "RETRY-WITH-FALLBACK"
	ActionBlockCode           Action = "BLOCK-CODE"
	ActionBlockOperatorAction Action = "BLOCK-OPERATOR-ACTION"
)

// Verdict mirrors the verdict_for_block field of the bash JSON output.
type Verdict string

const (
	VerdictNone                      Verdict = ""
	VerdictScopeRejected             Verdict = "SCOPE-REJECTED"
	VerdictBlockedSystemic           Verdict = "BLOCKED-SYSTEMIC"
	VerdictBlockedRecurringAuditFail Verdict = "BLOCKED-RECURRING-AUDIT-FAIL"
	VerdictBlockedRecurringBuildFail Verdict = "BLOCKED-RECURRING-BUILD-FAIL"
)

// Entry mirrors one element of state.json:failedApproaches. JSON tags
// match the on-disk schema observed at .evolve/state.json (canonical
// since v8.22.0).
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

// Decision is the structured output of Decide. Field shape matches the
// bash JSON output at failure-adapter.sh:26-39.
type Decision struct {
	Action          Action            `json:"action"`
	Reason          string            `json:"reason"`
	Remediation     string            `json:"remediation,omitempty"`
	SetEnv          map[string]string `json:"set_env,omitempty"`
	SkipPhases      []string          `json:"skip_phases,omitempty"`
	VerdictForBlock Verdict           `json:"verdict_for_block,omitempty"`
	Evidence        Evidence          `json:"evidence"`
}

// Evidence carries forensic counts for the decision.
type Evidence struct {
	NonExpiredCount                 int            `json:"non_expired_count"`
	ByClass                         map[string]int `json:"by_class"`
	ConsecutiveInfraTransientStreak int            `json:"consecutive_infra_transient_streak"`
}

// Options controls Decide's behaviour. Now=zero defaults to time.Now().
type Options struct {
	Strict bool      // EVOLVE_STRICT_AUDIT=1 → first matching BLOCK wins.
	Now    time.Time // override clock for tests.
}

// AgeOutSeconds returns the retention window for a Classification.
// Mirrors failure_age_out_seconds at failure-classifications.sh:66-86.
// Unknown classifications default to 1 day (bash:84).
func AgeOutSeconds(c Classification) int64 {
	switch c {
	case InfraTransient, IntentMalformed, CodeAuditWarn, ShipGateConfig:
		return 86400 // 1 day
	case InfraSystemic, IntegrityBreach:
		return 604800 // 7 days
	case IntentRejected:
		return 999999999 // effectively never
	case CodeBuildFail, CodeAuditFail:
		return 2592000 // 30 days
	case HumanAbort, ExitTransportHang:
		return 3600 // 1 hour
	default:
		return 86400 // 1 day for unknown
	}
}

// Decide computes the failure-adaptation outcome for the given entries.
// Pure function: no I/O, deterministic given (entries, opts).
func Decide(entries []Entry, opts Options) Decision {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	// Filter to non-expired entries using the v8.23.1 fallback rules
	// from failure-adapter.sh:120-130.
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

	// Distinct-cycle counts used by recurring-fail rules. bash:162-164.
	auditFailDistinct := distinctCyclesByClass(nonExpired, CodeAuditFail)
	buildFailDistinct := distinctCyclesByClass(nonExpired, CodeBuildFail)

	awareness := make([]string, 0)

	// Rule 1: intent-rejected (any non-expired) — bash:249-254.
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

	// Rule 2: infrastructure-systemic — bash:257-265.
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

	// Rule 3: 2+ distinct cycles with code-audit-fail — bash:268-273.
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

	// Rule 4: 2+ distinct cycles with code-build-fail — bash:276-281.
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

	// Rule 5: 3+ consecutive infra-transient tail streak — bash:284-289.
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

	// Rule 6: 1+ infra-transient — RETRY-WITH-FALLBACK in strict mode,
	// otherwise merge into set_env and continue to PROCEED. bash:294-306.
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

	// Default / fluent terminus.
	if len(awareness) > 0 {
		reason := "fluent mode (set EVOLVE_STRICT_AUDIT=1 for legacy blocking): " + strings.Join(awareness, "; ")
		return Decision{
			Action:      ActionProceed,
			Reason:      reason,
			Remediation: "Awareness only — orchestrator should consider the prior failures when planning. Set EVOLVE_STRICT_AUDIT=1 to restore legacy block-on-recurring-failure behavior.",
			Evidence:    ev,
		}
	}
	return Decision{
		Action:   ActionProceed,
		Reason:   fmt.Sprintf("no recent failures requiring adaptation (non-expired count=%d)", ev.NonExpiredCount),
		Evidence: ev,
	}
}

// isNonExpired implements the bash v8.23.1 expiration logic
// (failure-adapter.sh:120-130). Three cases:
//
//  1. explicit non-null expiresAt in the future  → keep
//  2. both expiresAt and recordedAt missing      → keep (truly legacy)
//  3. expiresAt missing but recordedAt present   → effective 1d TTL
func isNonExpired(e Entry, now time.Time) bool {
	if e.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, e.ExpiresAt)
		if err != nil {
			// bash: try fromdateiso8601 catch ($now + 1) → treat unparseable as future.
			return true
		}
		return t.After(now)
	}
	// expiresAt missing.
	if e.RecordedAt == "" {
		return true // truly legacy, keep.
	}
	rt, err := time.Parse(time.RFC3339, e.RecordedAt)
	if err != nil {
		// bash: try fromdateiso8601 catch 0 → effective 1970 + 1d → expired.
		return false
	}
	return rt.Add(24 * time.Hour).After(now)
}

// countByClass returns raw counts per classification.
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

// distinctCyclesByClass returns the count of unique .cycle values for
// entries matching the given classification. Mirrors
// count_distinct_cycles_by_class at bash:162-164.
func distinctCyclesByClass(entries []Entry, target Classification) int {
	seen := map[int]struct{}{}
	for _, e := range entries {
		if e.Classification == target {
			seen[e.Cycle] = struct{}{}
		}
	}
	return len(seen)
}

// tailInfraTransientStreak walks the entries in reverse and counts
// consecutive infra-transient (or legacy "infrastructure") entries
// from the tail until the streak breaks. bash:142-152.
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

// lastSummaryOf returns the .summary of the last entry matching class.
// Used by rule 2 to surface the most-recent systemic-failure context.
func lastSummaryOf(entries []Entry, class Classification) string {
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i].Classification == class {
			s := entries[i].Summary
			if len(s) > 200 { // bash | head -c 200
				s = s[:200]
			}
			return s
		}
	}
	return ""
}
