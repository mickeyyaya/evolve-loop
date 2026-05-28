package cycleclassify

import (
	"os"
	"path/filepath"
	"testing"
)

// Workstream D2: an empty-output model session (the subscription-quota-wall
// signature — a phase launched but the model returned nothing) must be
// reclassified from integrity-breach to recoverable infrastructure, so the
// dispatcher QUOTA-PAUSEs/retries instead of treating the quota wall as a
// 7-day-retention breach. These tests pin both the positive signal and the
// guards that keep it from masking a genuine silent skip.

// emptyOutputWS seeds an unclassifiable orchestrator-report.md (so Classify
// reaches the final passes) plus the supplied per-phase files.
func emptyOutputWS(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	// A report that matches none of the infra/ship/audit/build patterns →
	// without pass 6 this would be ClassIntegrityBreach.
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte("## Summary\nnothing classifiable here\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	return ws
}

func TestClassify_EmptyStdoutLog_QuotaLikely(t *testing.T) {
	ws := emptyOutputWS(t)
	// build-planner WAS launched (stdout.log exists) but produced nothing.
	if err := os.WriteFile(filepath.Join(ws, "build-planner-stdout.log"), []byte("   \n\n"), 0o644); err != nil {
		t.Fatalf("write stdout: %v", err)
	}
	r := Classify(ws)
	if r.Class != ClassInfrastructure {
		t.Errorf("class=%q, want infrastructure (empty-output is recoverable, not a breach)", r.Class)
	}
	if r.Marker != MarkerQuotaLikelyEmptyOutput {
		t.Errorf("marker=%q, want %q", r.Marker, MarkerQuotaLikelyEmptyOutput)
	}
	if r.Source != "build-planner-stdout.log" {
		t.Errorf("source=%q, want build-planner-stdout.log", r.Source)
	}
}

func TestClassify_NoStdoutLog_StaysBreach(t *testing.T) {
	// No stdout.log at all = a phase that never ran = silent skip = breach.
	// The empty-output pass must NOT fire (the EXISTS guard).
	ws := emptyOutputWS(t)
	r := Classify(ws)
	if r.Class != ClassIntegrityBreach {
		t.Errorf("class=%q, want integrity-breach (no launched-but-empty phase to excuse the failure)", r.Class)
	}
}

func TestClassify_NonEmptyStdoutLog_StaysBreach(t *testing.T) {
	// A phase that DID produce output but the cycle is still unclassifiable is
	// a real breach, not a quota wall — the pass must not fire.
	ws := emptyOutputWS(t)
	if err := os.WriteFile(filepath.Join(ws, "build-stdout.log"), []byte("ran fine, wrote code\n"), 0o644); err != nil {
		t.Fatalf("write stdout: %v", err)
	}
	r := Classify(ws)
	if r.Class != ClassIntegrityBreach {
		t.Errorf("class=%q, want integrity-breach (phase produced output → not a quota wall)", r.Class)
	}
}

func TestClassify_EmptyStdoutButAssistantEvents_StaysBreach(t *testing.T) {
	// Empty stdout.log BUT the events stream captured assistant output → the
	// log was merely truncated, not a quota wall. Guard must keep it a breach.
	ws := emptyOutputWS(t)
	if err := os.WriteFile(filepath.Join(ws, "build-stdout.log"), []byte(""), 0o644); err != nil {
		t.Fatalf("write stdout: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "build-events.ndjson"),
		[]byte(`{"kind":"assistant_text","data":{"text":"working on it"}}`+"\n"), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}
	r := Classify(ws)
	if r.Class != ClassIntegrityBreach {
		t.Errorf("class=%q, want integrity-breach (assistant events present → output happened)", r.Class)
	}
}

func TestClassify_NoReportButEmptyOutput_QuotaLikely(t *testing.T) {
	// The cycle-120 signature: a mid-cycle quota abort never writes an
	// orchestrator-report.md. The pass must still recover, from per-phase
	// artifacts alone, instead of short-circuiting to breach.
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "build-planner-stdout.log"), []byte(""), 0o644); err != nil {
		t.Fatalf("write stdout: %v", err)
	}
	r := Classify(ws)
	if r.Class != ClassInfrastructure || r.Marker != MarkerQuotaLikelyEmptyOutput {
		t.Errorf("class=%q marker=%q, want infrastructure / %q (pass 6 must run even with no orchestrator-report.md)",
			r.Class, r.Marker, MarkerQuotaLikelyEmptyOutput)
	}
}

func TestClassify_EmptyStdoutButEventsTruncated_StaysBreach(t *testing.T) {
	// A single events line exceeds maxScannerBufBytes → scanner.Err() fires →
	// hasAssistantEvents conservatively returns TRUE (assume output present) so
	// the empty-output pass does NOT misfire on a truncation. Shrinking the
	// cap drives the same branch deterministically.
	orig := maxScannerBufBytes
	maxScannerBufBytes = 32
	defer func() { maxScannerBufBytes = orig }()

	ws := emptyOutputWS(t)
	if err := os.WriteFile(filepath.Join(ws, "build-stdout.log"), []byte(""), 0o644); err != nil {
		t.Fatalf("write stdout: %v", err)
	}
	// One line that exceeds both the 1024-byte initial scanner buffer AND
	// the 32-byte cap, forcing bufio.ErrTooLong → scanner.Err() non-nil.
	if err := os.WriteFile(filepath.Join(ws, "build-events.ndjson"),
		[]byte(`{"kind":"tool_use","data":{"text":"`+stringRepeat("x", 2000)+`"}}`+"\n"), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}
	r := Classify(ws)
	if r.Class != ClassIntegrityBreach {
		t.Errorf("class=%q, want integrity-breach (truncated scan must NOT trigger false quota-pause)", r.Class)
	}
}

// stringRepeat is a local helper since this package doesn't already import
// strings just for tests (avoids touching the import list).
func stringRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

func TestClassify_EmptyOutputNeverBeatsClassifiableMarker(t *testing.T) {
	// The pass runs LAST. A report with a real infra/build marker must keep its
	// specific classification even if an empty stdout.log is also present.
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"),
		[]byte("Build status: FAIL — tests RED\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "build-stdout.log"), []byte(""), 0o644); err != nil {
		t.Fatalf("write stdout: %v", err)
	}
	r := Classify(ws)
	if r.Class != ClassBuildFail {
		t.Errorf("class=%q, want build-fail (classifiable marker must win over the last-resort quota pass)", r.Class)
	}
}
