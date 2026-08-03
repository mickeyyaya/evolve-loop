package router

import (
	"reflect"
	"testing"
)

// TestDigest_PayloadWrapped_EquivalentToFlat is the golden-equivalence anchor
// for ADR-0050 Phase 3.3: Digest must read the new payload-wrapped handoff
// envelope and yield byte-identical RoutingSignals to the legacy flat envelope
// (Postel-compatible). Until digest.go unwraps `payload`, the wrapped workspace
// extracts nothing and this fails RED.
func TestDigest_PayloadWrapped_EquivalentToFlat(t *testing.T) {
	roles := []string{"scout", "triage", "build", "audit"}
	triage := `{"cycle_size_estimate":"medium","phase_skip":["retrospective"]}`

	flat := t.TempDir()
	writeFile(t, flat, "handoff-build.json", buildHandoff)
	writeFile(t, flat, "handoff-auditor.json", auditHandoff)
	writeFile(t, flat, "handoff-scout.json", scoutHandoff)
	writeFile(t, flat, "handoff-triage.json", triage)

	// wrap embeds the exact flat bytes as the `payload` of the canonical
	// envelope (schema_version 2 + promoted top-level fields).
	wrap := func(phase, payload string) string {
		return `{"schema_version":2,"phase":"` + phase + `","payload":` + payload + `,"verdict":"PASS","signals":{}}`
	}
	wrapped := t.TempDir()
	writeFile(t, wrapped, "handoff-build.json", wrap("build", buildHandoff))
	writeFile(t, wrapped, "handoff-auditor.json", wrap("audit", auditHandoff))
	writeFile(t, wrapped, "handoff-scout.json", wrap("scout", scoutHandoff))
	writeFile(t, wrapped, "handoff-triage.json", wrap("triage", triage))

	flatSig, err := Digest(flat, roles)
	if err != nil {
		t.Fatalf("flat digest: %v", err)
	}
	wrappedSig, err := Digest(wrapped, roles)
	if err != nil {
		t.Fatalf("wrapped digest: %v", err)
	}

	if !reflect.DeepEqual(flatSig, wrappedSig) {
		t.Fatalf("payload-wrapped digest != flat digest:\n flat   =%+v\n wrapped=%+v", flatSig, wrappedSig)
	}
	// Guard against a vacuous pass (both all-zero): the wrapped digest must
	// have actually extracted the build content.
	if !wrappedSig.Build.Present || wrappedSig.Build.SeverityMax != SevCritical {
		t.Fatalf("wrapped build not extracted (unwrap missing?): %+v", wrappedSig.Build)
	}
}

// TestDigest_PayloadWrapped_FoldsInnerSignals makes the wrapped signal-fold path
// load-bearing (the other wrapped tests use empty signals, so signal folding
// through the wrapper was never actually exercised). A signal living in the
// inner payload's top-level "signals" object must surface in the generic plane
// identically whether the handoff is flat or payload-wrapped — pinning the
// authority contract that Digest folds from the UNWRAPPED payload, and that the
// wrapper's promoted top-level signals are a copy, not a separate source.
func TestDigest_PayloadWrapped_FoldsInnerSignals(t *testing.T) {
	// A build handoff whose body carries top-level signals (a bare key the
	// router namespaces as build.*, and an already-dotted cross-namespace key).
	body := `{"verdict":"PASS","signals":{"files_touched":4,"security.precheck":"clean"}}`
	wrapped := `{"schema_version":2,"phase":"build","payload":` + body + `,"verdict":"PASS","signals":{}}`

	flatWS := t.TempDir()
	writeFile(t, flatWS, "handoff-build.json", body)
	wrapWS := t.TempDir()
	writeFile(t, wrapWS, "handoff-build.json", wrapped)

	flatSig, err := Digest(flatWS, []string{"build"})
	if err != nil {
		t.Fatalf("flat digest: %v", err)
	}
	wrapSig, err := Digest(wrapWS, []string{"build"})
	if err != nil {
		t.Fatalf("wrapped digest: %v", err)
	}
	if !reflect.DeepEqual(flatSig, wrapSig) {
		t.Fatalf("inner-signal fold not flat-equivalent:\n flat=%+v\n wrap=%+v", flatSig, wrapSig)
	}
	// Non-vacuous: the inner-payload signals were actually folded through the wrapper.
	if v, ok := wrapSig.GenericValue("build.files_touched"); !ok || v != float64(4) {
		t.Fatalf("inner bare signal not folded through wrapper: (%v, %v)", v, ok)
	}
	if v, ok := wrapSig.GenericValue("security.precheck"); !ok || v != "clean" {
		t.Fatalf("inner dotted signal not folded through wrapper: (%v, %v)", v, ok)
	}
}

// TestDigest_FlatStillWorks pins the fallback half: a legacy flat envelope (no
// payload key) must keep extracting exactly as before the unwrap step.
func TestDigest_FlatStillWorks(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "handoff-build.json", buildHandoff)
	sig, err := Digest(ws, []string{"build"})
	if err != nil {
		t.Fatalf("digest: %v", err)
	}
	if !sig.Build.Present || sig.Build.SeverityMax != SevCritical || sig.Build.FilesTouched != 3 {
		t.Fatalf("flat build extraction regressed: %+v", sig.Build)
	}
}
