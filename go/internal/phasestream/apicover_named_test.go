package phasestream

import (
	"testing"
)

func TestKindError_Defined(t *testing.T) {
	if KindError != "error" {
		t.Fatalf("KindError = %q, want %q (ADR-0020 wire token consumers filter on)", KindError, "error")
	}
}

func TestClassifier_Emit_MonotonicSeqAndSource(t *testing.T) {
	c := newTestClassifier()

	first := c.Emit(KindError, SeverityIncident, map[string]any{"reason": "boom"})
	second := c.Emit(KindStall, SeverityWarn, nil)

	if first.Kind != KindError || first.Severity != SeverityIncident {
		t.Errorf("Emit kind/sev = (%q,%q), want (error,INCIDENT)", first.Kind, first.Severity)
	}
	if first.Data["reason"] != "boom" {
		t.Errorf("Emit dropped data: %#v", first.Data)
	}
	if first.Source.CLI != "claude-p" || first.Source.Cycle != 12 || first.Source.Phase != "build" {
		t.Errorf("Emit source not stamped from Classifier: %+v", first.Source)
	}
	if first.SchemaVersion != SchemaVersion {
		t.Errorf("Emit schema_version = %q, want %q", first.SchemaVersion, SchemaVersion)
	}
	if first.Seq != 1 || second.Seq != 2 {
		t.Errorf("Emit seq = (%d,%d), want (1,2) — must be monotonic gap-free", first.Seq, second.Seq)
	}
}
