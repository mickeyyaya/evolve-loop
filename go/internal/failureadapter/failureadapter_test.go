package failureadapter

import (
	"testing"
	"time"
)

func fixedTime() time.Time {
	return time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
}

func iso(t time.Time) string { return t.UTC().Format(time.RFC3339) }

func TestDecide_NoEntries(t *testing.T) {
	d := Decide(nil, Options{Now: fixedTime()})
	if d.Action != ActionProceed {
		t.Errorf("Action=%q, want PROCEED", d.Action)
	}
	if d.Evidence.NonExpiredCount != 0 {
		t.Errorf("NonExpiredCount=%d, want 0", d.Evidence.NonExpiredCount)
	}
}

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

func TestDecide_LegacyMissingExpiresAt_OneDayTTL(t *testing.T) {
	now := fixedTime()
	fresh := iso(now.Add(-12 * time.Hour))
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

func TestDecide_TrulyLegacy_BothMissingKept(t *testing.T) {
	now := fixedTime()
	entries := []Entry{{Cycle: 1, Classification: ""}}
	d := Decide(entries, Options{Now: now})
	if d.Evidence.NonExpiredCount != 1 {
		t.Errorf("NonExpiredCount=%d, want 1 (truly legacy kept)", d.Evidence.NonExpiredCount)
	}
}

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

func TestDecide_InfraTransientTailStreak_BrokenByOtherClass(t *testing.T) {
	now := fixedTime()
	exp := iso(now.Add(12 * time.Hour))
	codeExp := iso(now.Add(7 * 24 * time.Hour))
	entries := []Entry{
		{Cycle: 1, Classification: InfraTransient, RecordedAt: iso(now.Add(-4 * time.Hour)), ExpiresAt: exp},
		{Cycle: 2, Classification: InfraTransient, RecordedAt: iso(now.Add(-3 * time.Hour)), ExpiresAt: exp},
		{Cycle: 3, Classification: CodeAuditFail, RecordedAt: iso(now.Add(-2 * time.Hour)), ExpiresAt: codeExp},
		{Cycle: 4, Classification: InfraTransient, RecordedAt: iso(now.Add(-1 * time.Hour)), ExpiresAt: exp},
	}
	d := Decide(entries, Options{Now: now, Strict: true})
	if d.Evidence.ConsecutiveInfraTransientStreak != 1 {
		t.Errorf("ConsecutiveStreak=%d, want 1 (broken at cycle 3)",
			d.Evidence.ConsecutiveInfraTransientStreak)
	}
}

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
}

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
}

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
		{"some-unknown-class", 86400},
	}
	for _, tc := range cases {
		t.Run(string(tc.class), func(t *testing.T) {
			if got := AgeOutSeconds(tc.class); got != tc.want {
				t.Errorf("AgeOutSeconds(%q)=%d, want %d", tc.class, got, tc.want)
			}
		})
	}
}

func TestDecide_DefaultClock(t *testing.T) {
	farFuture := time.Now().Add(365 * 24 * time.Hour).UTC().Format(time.RFC3339)
	entries := []Entry{
		{Cycle: 1, Classification: CodeAuditFail, RecordedAt: time.Now().UTC().Format(time.RFC3339), ExpiresAt: farFuture},
	}
	d := Decide(entries, Options{})
	if d.Evidence.NonExpiredCount != 1 {
		t.Errorf("NonExpiredCount=%d, want 1 (default clock)", d.Evidence.NonExpiredCount)
	}
}

func TestDecide_InfraSystemic_FluentAwareness(t *testing.T) {
	now := fixedTime()
	entries := []Entry{
		{Cycle: 5, Classification: InfraSystemic, RecordedAt: iso(now), ExpiresAt: iso(now.Add(7 * 24 * time.Hour)), Summary: "host broken"},
	}
	d := Decide(entries, Options{Now: now, Strict: false})
	if d.Action != ActionProceed {
		t.Errorf("Action=%q fluent, want PROCEED", d.Action)
	}
	if !containsStr(d.Reason, "would-have-blocked: BLOCK-OPERATOR-ACTION") {
		t.Errorf("Reason=%q missing fluent infra-systemic awareness", d.Reason)
	}
}

func TestDecide_InfraSystemic_NoSummary(t *testing.T) {
	now := fixedTime()
	entries := []Entry{
		{Cycle: 5, Classification: InfraSystemic, RecordedAt: iso(now), ExpiresAt: iso(now.Add(7 * 24 * time.Hour))},
	}
	d := Decide(entries, Options{Now: now, Strict: true})
	if !containsStr(d.Reason, "(no summary)") {
		t.Errorf("Reason=%q missing '(no summary)' fallback", d.Reason)
	}
}

func TestDecide_LongSummaryTruncatedAt200(t *testing.T) {
	now := fixedTime()
	longSummary := ""
	for i := 0; i < 300; i++ {
		longSummary += "Z"
	}
	entries := []Entry{
		{Cycle: 5, Classification: InfraSystemic, RecordedAt: iso(now), ExpiresAt: iso(now.Add(7 * 24 * time.Hour)), Summary: longSummary},
	}
	d := Decide(entries, Options{Now: now, Strict: true})
	count := 0
	for i := range d.Reason {
		if d.Reason[i] == 'Z' {
			count++
		}
	}
	if count != 200 {
		t.Errorf("summary in Reason has %d 'Z' chars, want 200 (bash head -c 200)", count)
	}
}

func TestDecide_TwoAuditFail_Fluent(t *testing.T) {
	now := fixedTime()
	exp := iso(now.Add(7 * 24 * time.Hour))
	entries := []Entry{
		{Cycle: 32, Classification: CodeAuditFail, RecordedAt: iso(now), ExpiresAt: exp},
		{Cycle: 33, Classification: CodeAuditFail, RecordedAt: iso(now), ExpiresAt: exp},
	}
	d := Decide(entries, Options{Now: now, Strict: false})
	if d.Action != ActionProceed {
		t.Errorf("Action=%q, want PROCEED fluent", d.Action)
	}
	if !containsStr(d.Reason, "code-audit-fail") {
		t.Errorf("Reason=%q missing audit-fail awareness", d.Reason)
	}
}

func TestDecide_TwoBuildFail_Fluent(t *testing.T) {
	now := fixedTime()
	exp := iso(now.Add(7 * 24 * time.Hour))
	entries := []Entry{
		{Cycle: 10, Classification: CodeBuildFail, RecordedAt: iso(now), ExpiresAt: exp},
		{Cycle: 11, Classification: CodeBuildFail, RecordedAt: iso(now), ExpiresAt: exp},
	}
	d := Decide(entries, Options{Now: now, Strict: false})
	if d.Action != ActionProceed {
		t.Errorf("Action=%q, want PROCEED fluent", d.Action)
	}
	if !containsStr(d.Reason, "code-build-fail") {
		t.Errorf("Reason=%q missing build-fail awareness", d.Reason)
	}
}

func TestDecide_InfraTailStreak_Fluent(t *testing.T) {
	now := fixedTime()
	exp := iso(now.Add(12 * time.Hour))
	entries := []Entry{
		{Cycle: 1, Classification: InfraTransient, RecordedAt: iso(now.Add(-3 * time.Hour)), ExpiresAt: exp},
		{Cycle: 2, Classification: InfraTransient, RecordedAt: iso(now.Add(-2 * time.Hour)), ExpiresAt: exp},
		{Cycle: 3, Classification: InfraTransient, RecordedAt: iso(now.Add(-1 * time.Hour)), ExpiresAt: exp},
	}
	d := Decide(entries, Options{Now: now, Strict: false})
	if d.Action != ActionProceed {
		t.Errorf("Action=%q, want PROCEED fluent", d.Action)
	}
	if !containsStr(d.Reason, "consecutive infrastructure-transient") {
		t.Errorf("Reason=%q missing tail-streak awareness", d.Reason)
	}
}

func TestIsNonExpired_UnparseableExpiresAt_KeptAsLive(t *testing.T) {
	now := fixedTime()
	entries := []Entry{
		{Cycle: 1, Classification: CodeAuditFail, RecordedAt: iso(now), ExpiresAt: "not-a-date"},
	}
	d := Decide(entries, Options{Now: now})
	if d.Evidence.NonExpiredCount != 1 {
		t.Errorf("NonExpiredCount=%d, want 1 (unparseable expiresAt → kept)", d.Evidence.NonExpiredCount)
	}
}

func TestIsNonExpired_UnparseableRecordedAt_NoExpiresAt_Drops(t *testing.T) {
	now := fixedTime()
	entries := []Entry{
		{Cycle: 1, Classification: CodeAuditFail, RecordedAt: "bogus"},
	}
	d := Decide(entries, Options{Now: now})
	if d.Evidence.NonExpiredCount != 0 {
		t.Errorf("NonExpiredCount=%d, want 0 (unparseable recordedAt → dropped)", d.Evidence.NonExpiredCount)
	}
}

func TestLastSummaryOf_NoMatch_ReturnsEmpty(t *testing.T) {
	entries := []Entry{
		{Cycle: 1, Classification: CodeAuditFail, Summary: "x"},
	}
	if got := lastSummaryOf(entries, InfraSystemic); got != "" {
		t.Errorf("lastSummaryOf=%q, want empty", got)
	}
}

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
