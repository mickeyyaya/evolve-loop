package phasestream

import (
	"testing"
)

// TestKindError_Defined names the KindError taxonomy member (envelope.go) and
// pins its wire token. KindError is part of the public ADR-0020 normalized-event
// Kind enum; downstream consumers (cycleclassify, the observer rules) filter on
// these exact string values, so the token is a contract — mirrors the existing
// TestKindCorrelation_Defined idiom in envelope_test.go.
func TestKindError_Defined(t *testing.T) {
	if KindError != "error" {
		t.Fatalf("KindError = %q, want %q (ADR-0020 wire token consumers filter on)", KindError, "error")
	}
}

// TestClassifier_Emit_MonotonicSeqAndSource names and calls Classifier.Emit —
// the normalizer-origin event seam (used for stall/correlation events) that is
// exercised internally but never asserted directly. It pins the contract Emit
// promises in its doc comment: the emitted envelope carries the supplied kind,
// severity, and data, stamps the Classifier's source + trace_id + schema, and
// advances the SAME monotonic seq the line-event path uses — so the unified
// stream stays gap-free across both line- and rule-originated events.
func TestClassifier_Emit_MonotonicSeqAndSource(t *testing.T) {
	c := newTestClassifier() // Source{CLI:"claude-p",Cycle:12,Phase:"build",Agent:"build"}, fixed clock

	first := c.Emit(KindError, SeverityIncident, map[string]any{"reason": "boom"})
	second := c.Emit(KindStall, SeverityWarn, nil)

	// Fields are propagated verbatim.
	if first.Kind != KindError || first.Severity != SeverityIncident {
		t.Errorf("Emit kind/sev = (%q,%q), want (error,INCIDENT)", first.Kind, first.Severity)
	}
	if first.Data["reason"] != "boom" {
		t.Errorf("Emit dropped data: %#v", first.Data)
	}
	// Classifier identity is stamped from its Source/traceID/schema.
	if first.Source.CLI != "claude-p" || first.Source.Cycle != 12 || first.Source.Phase != "build" {
		t.Errorf("Emit source not stamped from Classifier: %+v", first.Source)
	}
	if first.SchemaVersion != SchemaVersion {
		t.Errorf("Emit schema_version = %q, want %q", first.SchemaVersion, SchemaVersion)
	}
	// seq is monotonic and gap-free across successive Emits (the whole point).
	if first.Seq != 1 || second.Seq != 2 {
		t.Errorf("Emit seq = (%d,%d), want (1,2) — must be monotonic gap-free", first.Seq, second.Seq)
	}
}

// TestClassifier_SetInjectedPrompt names and covers Classifier.SetInjectedPrompt
// — the cycle-641/642 fix-of-record seam that threads the phase's own prompt
// text into the Classifier so an infra-marker line that merely echoes it (the
// agent quoting its OWN instructions) is suppressed rather than emitted as a
// runtime infra_failure. Pins the contract: an echoed prompt substring emits
// nothing; a genuine runtime line absent from the prompt still emits.
func TestClassifier_SetInjectedPrompt(t *testing.T) {
	const prompt = "Adversarial Reviewer checklist: TOCTOU / race windows; missing rate limits."
	c := NewClassifier(Source{Producer: "normalizer", Phase: "adversarial-review"}, "trace-cover", nil)
	c.SetInjectedPrompt(prompt)

	if hasInfraFailure(c.Stderr([]byte("missing rate limits."))) {
		t.Errorf("SetInjectedPrompt failed to suppress a verbatim prompt echo")
	}
	if !hasInfraFailure(c.Stderr([]byte("Error: 429 Too Many Requests (rate limit hit)"))) {
		t.Errorf("SetInjectedPrompt wrongly suppressed a genuine runtime infra signal")
	}
}
