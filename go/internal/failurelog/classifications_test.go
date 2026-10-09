package failurelog

import (
	"testing"
	"time"
)

func TestNormalizeLegacy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in   string
		want Classification
	}{
		{"infrastructure-transient", InfrastructureTransient},
		{"code-audit-warn", CodeAuditWarn},
		{"ship-gate-config", ShipGateConfig},
		{"operator-reset", OperatorReset},
		{"loop-fatal", LoopFatal},

		{"infrastructure", InfrastructureTransient},
		{"audit-fail", CodeAuditFail},
		{"build-fail", CodeBuildFail},
		{"ship-gate-rejection", ShipGateConfig},

		{"FAIL", CodeAuditFail},
		{"WARN", CodeAuditWarn},
		{"SHIP_GATE_DENIED", ShipGateConfig},
		{"WARN-NO-AUDIT", InfrastructureSystemic},
		{"BLOCKED-RECURRING-AUDIT-FAIL", CodeAuditFail},
		{"BLOCKED-RECURRING-BUILD-FAIL", CodeBuildFail},
		{"BLOCKED-SYSTEMIC", InfrastructureSystemic},
		{"SCOPE-REJECTED", IntentRejected},

		{"EXIT_TRANSPORT_HANG", ExitTransportHang},
		{"exit_transport_hang", ExitTransportHang},

		{"", UnknownClassification},
		{"null", UnknownClassification},
		{"completely-made-up", UnknownClassification},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			if got := NormalizeLegacy(tc.in); got != tc.want {
				t.Fatalf("NormalizeLegacy(%q)=%q want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestAgeOutSeconds(t *testing.T) {
	t.Parallel()
	tests := []struct {
		c    Classification
		want int64
	}{
		{InfrastructureTransient, 86400},
		{InfrastructureSystemic, 604800},
		{IntentMalformed, 86400},
		{IntentRejected, 999999999},
		{CodeBuildFail, 2592000},
		{CodeAuditFail, 2592000},
		{CodeAuditWarn, 86400},
		{ShipGateConfig, 86400},
		{HumanAbort, 3600},
		{ExitTransportHang, 3600},
		{IntegrityBreach, 604800},
		{OperatorReset, 3600},
		{LoopFatal, 604800},
		{Classification("nonexistent"), 86400},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(string(tc.c), func(t *testing.T) {
			t.Parallel()
			if got := AgeOutSeconds(tc.c); got != tc.want {
				t.Fatalf("AgeOutSeconds(%q)=%d want %d", tc.c, got, tc.want)
			}
		})
	}
}

func TestSeverityOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		c    Classification
		want Severity
	}{
		{InfrastructureTransient, SeverityLow},
		{IntentMalformed, SeverityLow},
		{HumanAbort, SeverityLow},
		{ShipGateConfig, SeverityLow},
		{CodeAuditWarn, SeverityLow},
		{ExitTransportHang, SeverityLow},
		{OperatorReset, SeverityLow},

		{InfrastructureSystemic, SeverityHigh},
		{CodeBuildFail, SeverityHigh},
		{CodeAuditFail, SeverityHigh},
		{IntegrityBreach, SeverityHigh},
		{LoopFatal, SeverityHigh},

		{IntentRejected, SeverityTerminal},

		{Classification("???"), SeverityUnknown},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(string(tc.c), func(t *testing.T) {
			t.Parallel()
			if got := SeverityOf(tc.c); got != tc.want {
				t.Fatalf("SeverityOf(%q)=%s want %s", tc.c, got, tc.want)
			}
		})
	}
}

func TestRetryPolicyOf(t *testing.T) {
	t.Parallel()
	tests := []struct {
		c    Classification
		want RetryPolicy
	}{
		{InfrastructureTransient, RetryYes},
		{HumanAbort, RetryYes},
		{ShipGateConfig, RetryYes},
		{CodeAuditWarn, RetryYes},
		{ExitTransportHang, RetryYes},
		{IntentMalformed, RetryYes},
		{OperatorReset, RetryYes},
		{LoopFatal, RetryNeedsOp},

		{InfrastructureSystemic, RetryNeedsOp},
		{IntegrityBreach, RetryNeedsOp},
		{CodeBuildFail, RetryNeedsOp},
		{CodeAuditFail, RetryNeedsOp},

		{IntentRejected, RetryNo},

		{Classification("???"), RetryUnknown},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(string(tc.c), func(t *testing.T) {
			t.Parallel()
			if got := RetryPolicyOf(tc.c); got != tc.want {
				t.Fatalf("RetryPolicyOf(%q)=%s want %s", tc.c, got, tc.want)
			}
		})
	}
}

func TestComputeExpiresAt(t *testing.T) {
	t.Parallel()
	base := time.Date(2026, 5, 23, 12, 0, 0, 0, time.UTC)
	got := ComputeExpiresAt(InfrastructureTransient, base)
	want := "2026-05-24T12:00:00Z"
	if got != want {
		t.Fatalf("expiresAt=%q want %q (1d after base)", got, want)
	}
	got = ComputeExpiresAt(CodeAuditFail, base)
	want = "2026-06-22T12:00:00Z"
	if got != want {
		t.Fatalf("expiresAt=%q want %q (30d after base)", got, want)
	}
	got = ComputeExpiresAt(InfrastructureTransient, time.Time{})
	parsed, err := time.Parse(time.RFC3339, got)
	if err != nil {
		t.Fatalf("expiresAt=%q unparseable: %v", got, err)
	}
	if !parsed.After(time.Now()) {
		t.Fatalf("expiresAt=%s not in future (now=%s)", parsed, time.Now())
	}
}

func TestKnownClassifications(t *testing.T) {
	t.Parallel()
	got := KnownClassifications()
	if len(got) != 13 {
		t.Fatalf("KnownClassifications len=%d want 13", len(got))
	}
	for _, want := range []Classification{OperatorReset, LoopFatal} {
		found := false
		for _, c := range got {
			if c == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("KnownClassifications missing %q", want)
		}
	}
	for _, c := range got {
		if c == UnknownClassification {
			t.Fatalf("UnknownClassification should not be in KnownClassifications")
		}
	}
}
