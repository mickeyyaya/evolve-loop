// Package failureadapter ports scripts/failure/failure-adapter.sh —
// the deterministic failure-adaptation kernel introduced in v8.22.0.
//
// The bash source is at scripts/failure/failure-adapter.sh and its
// taxonomy at scripts/failure/failure-classifications.sh. This Go port
// must produce the SAME Decision for the SAME inputs so the trust
// kernel stays host-CLI-agnostic.
package failureadapter

import (
	"testing"
	"time"
)

// fixedTime returns a deterministic clock anchor for tests.
// All RecordedAt/ExpiresAt comparisons use this as "now".
func fixedTime() time.Time {
	return time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
}

// iso formats t as RFC3339 (the schema used by recordedAt/expiresAt).
func iso(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// TestDecide_NoEntries — empty failedApproaches → PROCEED.
// Mirrors the "no state.json present" early-exit at bash line 82-93.
func TestDecide_NoEntries(t *testing.T) {
	d := Decide(nil, Options{Now: fixedTime()})
	if d.Action != ActionProceed {
		t.Errorf("Action=%q, want PROCEED", d.Action)
	}
	if d.Evidence.NonExpiredCount != 0 {
		t.Errorf("NonExpiredCount=%d, want 0", d.Evidence.NonExpiredCount)
	}
}

// TestDecide_AllExpired_Proceed — entries past their expiresAt are
// pruned by the adapter and do not influence the decision. The bash
// auto-prune behavior (line 95-130) is replicated here.
func TestDecide_AllExpired_Proceed(t *testing.T) {
	now := fixedTime()
	past := now.Add(-2 * 24 * time.Hour)
	entries := []Entry{
		{Cycle: 1, Classification: InfraTransient, RecordedAt: iso(past.Add(-24 * time.Hour)), ExpiresAt: iso(past)},
		{Cycle: 2, Classification: CodeAuditFail, RecordedAt: iso(past.Add(-24 * time.Hour)), ExpiresAt: iso(past)},
	}
	d := Decide(entries, Options{Now: now})
	if d.Action != ActionProceed {
		t.Errorf("Action=%q, want PROCEED (all expired)", d.Action)
	}
	if d.Evidence.NonExpiredCount != 0 {
		t.Errorf("NonExpiredCount=%d after filter, want 0", d.Evidence.NonExpiredCount)
	}
}

// TestDecide_LegacyMissingExpiresAt_OneDayTTL — bash v8.23.1 fix:
// when expiresAt is missing but recordedAt is present, effective TTL
// is recordedAt + 1d. Replicates failure-adapter.sh:124-130.
func TestDecide_LegacyMissingExpiresAt_OneDayTTL(t *testing.T) {
	now := fixedTime()
	// Recorded 12h ago, no expiresAt → still within 1d TTL.
	fresh := iso(now.Add(-12 * time.Hour))
	// Recorded 2d ago, no expiresAt → past 1d TTL.
	stale := iso(now.Add(-48 * time.Hour))

	entries := []Entry{
		{Cycle: 1, Classification: CodeAuditFail, RecordedAt: fresh},
		{Cycle: 2, Classification: CodeAuditFail, RecordedAt: stale},
	}
	d := Decide(entries, Options{Now: now})
	if d.Evidence.NonExpiredCount != 1 {
		t.Errorf("NonExpiredCount=%d, want 1 (legacy 1d TTL)", d.Evidence.NonExpiredCount)
	}
}

// TestDecide_TrulyLegacy_BothMissingKept — when BOTH expiresAt AND
// recordedAt are missing, the entry is kept (defensive; rare and
// inert). bash:126.
func TestDecide_TrulyLegacy_BothMissingKept(t *testing.T) {
	now := fixedTime()
	entries := []Entry{{Cycle: 1, Classification: ""}}
	d := Decide(entries, Options{Now: now})
	if d.Evidence.NonExpiredCount != 1 {
		t.Errorf("NonExpiredCount=%d, want 1 (truly legacy kept)", d.Evidence.NonExpiredCount)
	}
}

// TestDecide_IntentRejected_BlocksInStrict — rule 1: any non-expired
// intent-rejected → BLOCK-CODE / SCOPE-REJECTED. bash:249-254.
func TestDecide_IntentRejected_BlocksInStrict(t *testing.T) {
	now := fixedTime()
	entries := []Entry{
		{Cycle: 1, Classification: IntentRejected, RecordedAt: iso(now.Add(-1 * time.Hour)), ExpiresAt: iso(now.Add(24 * time.Hour))},
	}
	d := Decide(entries, Options{Now: now, Strict: true})
	if d.Action != ActionBlockCode {
		t.Errorf("Action=%q, want BLOCK-CODE", d.Action)
	}
	if d.VerdictForBlock != VerdictScopeRejected {
		t.Errorf("Verdict=%q, want SCOPE-REJECTED", d.VerdictForBlock)
	}
	if d.Remediation == "" {
		t.Error("Remediation empty for BLOCK")
	}
}

// TestDecide_IntentRejected_FluentAwareness — fluent mode (default):
// the same input → PROCEED with awareness in the reason. bash:229-242.
func TestDecide_IntentRejected_FluentAwareness(t *testing.T) {
	now := fixedTime()
	entries := []Entry{
		{Cycle: 1, Classification: IntentRejected, RecordedAt: iso(now.Add(-1 * time.Hour)), ExpiresAt: iso(now.Add(24 * time.Hour))},
	}
	d := Decide(entries, Options{Now: now, Strict: false})
	if d.Action != ActionProceed {
		t.Errorf("Action=%q in fluent mode, want PROCEED", d.Action)
	}
	if !containsStr(d.Reason, "would-have-blocked") {
		t.Errorf("Reason=%q missing 'would-have-blocked' awareness marker", d.Reason)
	}
}

// TestDecide_InfraSystemic_BlocksWithLastSummary — rule 2: any
// non-expired infrastructure-systemic → BLOCK-OPERATOR-ACTION /
// BLOCKED-SYSTEMIC; last entry's summary is appended to the reason.
// bash:257-265.
func TestDecide_InfraSystemic_BlocksWithLastSummary(t *testing.T) {
	now := fixedTime()
	entries := []Entry{
		{Cycle: 5, Classification: InfraSystemic, RecordedAt: iso(now.Add(-2 * time.Hour)), ExpiresAt: iso(now.Add(7 * 24 * time.Hour)), Summary: "earlier failure"},
		{Cycle: 6, Classification: InfraSystemic, RecordedAt: iso(now.Add(-1 * time.Hour)), ExpiresAt: iso(now.Add(7 * 24 * time.Hour)), Summary: "tooling missing: gws"},
	}
	d := Decide(entries, Options{Now: now, Strict: true})
	if d.Action != ActionBlockOperatorAction {
		t.Errorf("Action=%q, want BLOCK-OPERATOR-ACTION", d.Action)
	}
	if d.VerdictForBlock != VerdictBlockedSystemic {
		t.Errorf("Verdict=%q, want BLOCKED-SYSTEMIC", d.VerdictForBlock)
	}
	if !containsStr(d.Reason, "tooling missing: gws") {
		t.Errorf("Reason=%q missing last summary", d.Reason)
	}
}

// TestDecide_TwoCodeAuditFail_DistinctCycles_Blocks — rule 3: 2+
// distinct cycles with code-audit-fail → BLOCK-CODE /
// BLOCKED-RECURRING-AUDIT-FAIL. bash:268-273.
// Note: distinct cycles, not raw count — multiple entries from one
// cycle should not count twice. bash:162-164.
func TestDecide_TwoCodeAuditFail_DistinctCycles_Blocks(t *testing.T) {
	now := fixedTime()
	withinRetention := iso(now.Add(7 * 24 * time.Hour))
	entries := []Entry{
		{Cycle: 32, Classification: CodeAuditFail, RecordedAt: iso(now), ExpiresAt: withinRetention},
		{Cycle: 33, Classification: CodeAuditFail, RecordedAt: iso(now), ExpiresAt: withinRetention},
	}
	d := Decide(entries, Options{Now: now, Strict: true})
	if d.Action != ActionBlockCode {
		t.Errorf("Action=%q, want BLOCK-CODE", d.Action)
	}
	if d.VerdictForBlock != VerdictBlockedRecurringAuditFail {
		t.Errorf("Verdict=%q, want BLOCKED-RECURRING-AUDIT-FAIL", d.VerdictForBlock)
	}
}

// TestDecide_OneCycle_TwoCodeAuditFailEntries_DoesNotBlock — same cycle,
// two entries (e.g., retry artifact) → distinct-cycle count is 1, not 2.
// bash:162-164 docstring confirms this is the intended dedup.
func TestDecide_OneCycle_TwoCodeAuditFailEntries_DoesNotBlock(t *testing.T) {
	now := fixedTime()
	exp := iso(now.Add(7 * 24 * time.Hour))
	entries := []Entry{
		{Cycle: 32, Classification: CodeAuditFail, RecordedAt: iso(now), ExpiresAt: exp},
		{Cycle: 32, Classification: CodeAuditFail, RecordedAt: iso(now.Add(1 * time.Hour)), ExpiresAt: exp},
	}
	d := Decide(entries, Options{Now: now, Strict: true})
	if d.Action == ActionBlockCode && d.VerdictForBlock == VerdictBlockedRecurringAuditFail {
		t.Errorf("Action=%q Verdict=%q: same-cycle entries should not trip recurring-audit-fail",
			d.Action, d.VerdictForBlock)
	}
}

// TestDecide_TwoCodeBuildFail_DistinctCycles_Blocks — rule 4: same as
// rule 3, different classification. bash:276-281.
func TestDecide_TwoCodeBuildFail_DistinctCycles_Blocks(t *testing.T) {
	now := fixedTime()
	exp := iso(now.Add(7 * 24 * time.Hour))
	entries := []Entry{
		{Cycle: 10, Classification: CodeBuildFail, RecordedAt: iso(now), ExpiresAt: exp},
		{Cycle: 11, Classification: CodeBuildFail, RecordedAt: iso(now), ExpiresAt: exp},
	}
	d := Decide(entries, Options{Now: now, Strict: true})
	if d.VerdictForBlock != VerdictBlockedRecurringBuildFail {
		t.Errorf("Verdict=%q, want BLOCKED-RECURRING-BUILD-FAIL", d.VerdictForBlock)
	}
}

// TestDecide_InfraTransientTailStreak_BlocksAt3 — rule 5: 3+
// consecutive infrastructure-transient at the TAIL (most recent) →
// BLOCK-OPERATOR-ACTION / BLOCKED-SYSTEMIC. bash:284-289.
func TestDecide_InfraTransientTailStreak_BlocksAt3(t *testing.T) {
	now := fixedTime()
	exp := iso(now.Add(12 * time.Hour))
	entries := []Entry{
		{Cycle: 1, Classification: CodeAuditFail, RecordedAt: iso(now.Add(-4 * time.Hour)), ExpiresAt: iso(now.Add(7 * 24 * time.Hour))},
		{Cycle: 2, Classification: InfraTransient, RecordedAt: iso(now.Add(-3 * time.Hour)), ExpiresAt: exp},
		{Cycle: 3, Classification: InfraTransient, RecordedAt: iso(now.Add(-2 * time.Hour)), ExpiresAt: exp},
		{Cycle: 4, Classification: InfraTransient, RecordedAt: iso(now.Add(-1 * time.Hour)), ExpiresAt: exp},
	}
	d := Decide(entries, Options{Now: now, Strict: true})
	if d.Action != ActionBlockOperatorAction {
		t.Errorf("Action=%q, want BLOCK-OPERATOR-ACTION (tail streak=3)", d.Action)
	}
	if d.Evidence.ConsecutiveInfraTransientStreak != 3 {
		t.Errorf("ConsecutiveStreak=%d, want 3", d.Evidence.ConsecutiveInfraTransientStreak)
	}
}

// TestDecide_InfraTransientTailStreak_BrokenByOtherClass — tail streak
// resets when a non-infra-transient entry is encountered while walking
// from the tail backwards. bash:142-152.
func TestDecide_InfraTransientTailStreak_BrokenByOtherClass(t *testing.T) {
	now := fixedTime()
	exp := iso(now.Add(12 * time.Hour))
	codeExp := iso(now.Add(7 * 24 * time.Hour))
	entries := []Entry{
		{Cycle: 1, Classification: InfraTransient, RecordedAt: iso(now.Add(-4 * time.Hour)), ExpiresAt: exp},
		{Cycle: 2, Classification: InfraTransient, RecordedAt: iso(now.Add(-3 * time.Hour)), ExpiresAt: exp},
		// This breaks the tail streak.
		{Cycle: 3, Classification: CodeAuditFail, RecordedAt: iso(now.Add(-2 * time.Hour)), ExpiresAt: codeExp},
		{Cycle: 4, Classification: InfraTransient, RecordedAt: iso(now.Add(-1 * time.Hour)), ExpiresAt: exp},
	}
	d := Decide(entries, Options{Now: now, Strict: true})
	if d.Evidence.ConsecutiveInfraTransientStreak != 1 {
		t.Errorf("ConsecutiveStreak=%d, want 1 (broken at cycle 3)",
			d.Evidence.ConsecutiveInfraTransientStreak)
	}
}

// TestDecide_OneInfraTransient_RetryWithFallback_BothModes — rule 6:
// any non-expired infrastructure-transient → RETRY-WITH-FALLBACK +
// set_env={"EVOLVE_SANDBOX_FALLBACK_ON_EPERM":"1"}, in BOTH modes.
// bash:291-306.
func TestDecide_OneInfraTransient_RetryWithFallback_StrictMode(t *testing.T) {
	now := fixedTime()
	exp := iso(now.Add(12 * time.Hour))
	entries := []Entry{
		{Cycle: 1, Classification: InfraTransient, RecordedAt: iso(now.Add(-30 * time.Minute)), ExpiresAt: exp},
	}
	d := Decide(entries, Options{Now: now, Strict: true})
	if d.Action != ActionRetryWithFallback {
		t.Errorf("Action=%q strict, want RETRY-WITH-FALLBACK", d.Action)
	}
	if d.SetEnv["EVOLVE_SANDBOX_FALLBACK_ON_EPERM"] != "1" {
		t.Errorf("SetEnv=%v missing EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1", d.SetEnv)
	}
}

// TestDecide_OneInfraTransient_FluentMergesSetEnv — fluent mode keeps
// PROCEED but still merges the EPERM-fallback env into set_env.
// bash:294-306.
func TestDecide_OneInfraTransient_FluentMergesSetEnv(t *testing.T) {
	now := fixedTime()
	exp := iso(now.Add(12 * time.Hour))
	entries := []Entry{
		{Cycle: 1, Classification: InfraTransient, RecordedAt: iso(now.Add(-30 * time.Minute)), ExpiresAt: exp},
	}
	d := Decide(entries, Options{Now: now, Strict: false})
	if d.Action != ActionProceed {
		t.Errorf("Action=%q fluent, want PROCEED", d.Action)
	}
	if d.SetEnv["EVOLVE_SANDBOX_FALLBACK_ON_EPERM"] != "1" {
		t.Errorf("SetEnv=%v missing EPERM fallback in fluent mode", d.SetEnv)
	}
}

// TestDecide_StrictMode_FirstMatchWins — in strict mode, intent-rejected
// (rule 1) wins over infrastructure-systemic (rule 2). bash decision-tree
// priority — first emit_or_advise that fires in strict mode exits.
func TestDecide_StrictMode_FirstMatchWins(t *testing.T) {
	now := fixedTime()
	entries := []Entry{
		{Cycle: 1, Classification: InfraSystemic, RecordedAt: iso(now), ExpiresAt: iso(now.Add(7 * 24 * time.Hour))},
		{Cycle: 2, Classification: IntentRejected, RecordedAt: iso(now), ExpiresAt: iso(now.Add(999 * 24 * time.Hour))},
	}
	d := Decide(entries, Options{Now: now, Strict: true})
	if d.VerdictForBlock != VerdictScopeRejected {
		t.Errorf("Verdict=%q, want SCOPE-REJECTED (rule 1 wins over rule 2)", d.VerdictForBlock)
	}
}

// TestDecide_Evidence_ByClassPopulated — evidence.by_class must report
// raw counts per classification (not distinct cycles — those drive
// thresholds but raw counts go into forensics). bash:133.
func TestDecide_Evidence_ByClassPopulated(t *testing.T) {
	now := fixedTime()
	exp := iso(now.Add(7 * 24 * time.Hour))
	entries := []Entry{
		{Cycle: 1, Classification: CodeAuditFail, RecordedAt: iso(now), ExpiresAt: exp},
		{Cycle: 1, Classification: CodeAuditFail, RecordedAt: iso(now), ExpiresAt: exp},
		{Cycle: 2, Classification: CodeBuildFail, RecordedAt: iso(now), ExpiresAt: exp},
	}
	d := Decide(entries, Options{Now: now, Strict: false})
	if got := d.Evidence.ByClass["code-audit-fail"]; got != 2 {
		t.Errorf("ByClass[code-audit-fail]=%d, want 2 (raw count)", got)
	}
	if got := d.Evidence.ByClass["code-build-fail"]; got != 1 {
		t.Errorf("ByClass[code-build-fail]=%d, want 1", got)
	}
}

// TestAgeOutSeconds_Taxonomy — exercises every branch of the
// classification → retention-window mapping from
// failure-classifications.sh:66-86.
func TestAgeOutSeconds_Taxonomy(t *testing.T) {
	cases := []struct {
		class Classification
		want  int64
	}{
		{InfraTransient, 86400},
		{InfraSystemic, 604800},
		{IntentMalformed, 86400},
		{IntentRejected, 999999999},
		{CodeBuildFail, 2592000},
		{CodeAuditFail, 2592000},
		{CodeAuditWarn, 86400},
		{ShipGateConfig, 86400},
		{HumanAbort, 3600},
		{ExitTransportHang, 3600},
		{IntegrityBreach, 604800},
		{"some-unknown-class", 86400}, // bash default.
	}
	for _, tc := range cases {
		t.Run(string(tc.class), func(t *testing.T) {
			if got := AgeOutSeconds(tc.class); got != tc.want {
				t.Errorf("AgeOutSeconds(%q)=%d, want %d", tc.class, got, tc.want)
			}
		})
	}
}

// TestDecide_DefaultClock — Options.Now zero value defaults to time.Now.
// Verified by passing entries with expiresAt far in the future; we
// only assert NonExpiredCount, not absolute timing.
func TestDecide_DefaultClock(t *testing.T) {
	farFuture := time.Now().Add(365 * 24 * time.Hour).UTC().Format(time.RFC3339)
	entries := []Entry{
		{Cycle: 1, Classification: CodeAuditFail, RecordedAt: time.Now().UTC().Format(time.RFC3339), ExpiresAt: farFuture},
	}
	d := Decide(entries, Options{}) // Now defaults to time.Now()
	if d.Evidence.NonExpiredCount != 1 {
		t.Errorf("NonExpiredCount=%d, want 1 (default clock)", d.Evidence.NonExpiredCount)
	}
}

// containsStr — small helper since strings.Contains would import
// strings just for tests; inlined for clarity (and matches the
// projecthash_test convention of self-contained tests).
func containsStr(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
