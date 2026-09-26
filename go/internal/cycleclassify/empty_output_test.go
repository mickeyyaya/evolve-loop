package cycleclassify

import (
	"os"
	"path/filepath"
	"testing"
)

// emptyOutputWS seeds an unclassifiable orchestrator-report.md so Classify reaches the final passes.
func emptyOutputWS(t *testing.T) string {
	t.Helper()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "orchestrator-report.md"), []byte("## Summary\nnothing classifiable here\n"), 0o644); err != nil {
		t.Fatalf("write report: %v", err)
	}
	return ws
}

func TestClassify_EmptyStdoutLog_QuotaLikely(t *testing.T) {
	ws := emptyOutputWS(t)
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
	ws := emptyOutputWS(t)
	r := Classify(ws)
	if r.Class != ClassIntegrityBreach {
		t.Errorf("class=%q, want integrity-breach (no launched-but-empty phase to excuse the failure)", r.Class)
	}
}

func TestClassify_NonEmptyStdoutLog_StaysBreach(t *testing.T) {
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
	orig := maxScannerBufBytes
	maxScannerBufBytes = 32
	defer func() { maxScannerBufBytes = orig }()

	ws := emptyOutputWS(t)
	if err := os.WriteFile(filepath.Join(ws, "build-stdout.log"), []byte(""), 0o644); err != nil {
		t.Fatalf("write stdout: %v", err)
	}
	// The line must exceed the 1024-byte initial scanner buffer as well as the shrunk cap.
	if err := os.WriteFile(filepath.Join(ws, "build-events.ndjson"),
		[]byte(`{"kind":"tool_use","data":{"text":"`+stringRepeat("x", 2000)+`"}}`+"\n"), 0o644); err != nil {
		t.Fatalf("write events: %v", err)
	}
	r := Classify(ws)
	if r.Class != ClassIntegrityBreach {
		t.Errorf("class=%q, want integrity-breach (truncated scan must NOT trigger false quota-pause)", r.Class)
	}
}

func stringRepeat(s string, n int) string {
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}

func TestClassify_EmptyOutputNeverBeatsClassifiableMarker(t *testing.T) {
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
