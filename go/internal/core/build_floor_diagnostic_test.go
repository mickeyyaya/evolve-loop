package core

// build_floor_diagnostic_test.go — cycle-1270 blocker (B-5).
//
// Cycle-1268 died with this recorded failure reason:
//
//	./cmd/evolve: unit tests FAIL
//	[engine] WARN: Deps.TokenResolver is nil — token telemetry disabled …
//	[engine] WARN: Deps.TokenResolver is nil — token telemetry disabled …
//	[engine] WARN: Deps.TokenResolver is nil — token telemetry disabled …
//
// 400 bytes of a benign, repeated warning and not one word about what failed.
// The floor kept output[:400], but `go test` writes its `--- FAIL` lines,
// panics and stack traces at the END. The truncation was pointed at exactly the
// region where the diagnosis was not — the difference between a cycle that is
// broken and a cycle that is undiagnosable.

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
	// Short output must pass through untouched — the elision marker is a signal,
	// not decoration, and a marker on complete output would train the reader to
	// ignore it.
	if short := "FAIL\tpkg\t0.1s\n"; floorFailureDiagnostic(short) != short {
		t.Errorf("short output was altered: %q", floorFailureDiagnostic(short))
	}
}
