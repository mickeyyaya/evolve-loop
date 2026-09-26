package core

import (
	"strings"
	"testing"
)

func TestBuildFloorSelfCheckFailures_KeepsTailDiagnostic(t *testing.T) {
	noise := strings.Repeat("[engine] WARN: Deps.TokenResolver is nil — token telemetry disabled (fail-open)\n", 40)
	verdict := "--- FAIL: TestLoop_MaxCyclesExit_ClearsCompletedMarker (6.75s)\n    loop_test.go:88: want 0 got 1\nFAIL\n"
	got := floorFailureDiagnostic(noise + verdict)

	if len(got) > floorFailureDiagnosticMax+len("…") {
		t.Fatalf("diagnostic is %d bytes, want <= %d — the cap must still hold", len(got), floorFailureDiagnosticMax+len("…"))
	}
	if !strings.Contains(got, "--- FAIL: TestLoop_MaxCyclesExit_ClearsCompletedMarker") {
		t.Errorf("the FAIL line was truncated away:\n%s\n\ngo test writes its verdict at the TAIL, so keeping the head hands the operator the noise and drops the diagnosis (the literal cycle-1268 failure reason)", got)
	}
	if !strings.HasPrefix(got, "…") {
		t.Errorf("a trimmed diagnostic must be marked as trimmed; got prefix %q", got[:1])
	}
	// Complete output carries no elision marker, so the marker stays a signal the reader trusts.
	if short := "FAIL\tpkg\t0.1s\n"; floorFailureDiagnostic(short) != short {
		t.Errorf("short output was altered: %q", floorFailureDiagnostic(short))
	}
}
