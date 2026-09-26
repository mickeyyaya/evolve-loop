package router

import (
	"reflect"
	"testing"
)

func TestDigest_PayloadWrapped_EquivalentToFlat(t *testing.T) {
	roles := []string{"scout", "triage", "build", "audit"}
	triage := `{"cycle_size_estimate":"medium","phase_skip":["retrospective"]}`

	flat := t.TempDir()
	writeFile(t, flat, "handoff-build.json", buildHandoff)
	writeFile(t, flat, "handoff-auditor.json", auditHandoff)
	writeFile(t, flat, "handoff-scout.json", scoutHandoff)
	writeFile(t, flat, "handoff-triage.json", triage)

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
	// Guards against a vacuous pass where both digests are empty.
	if !wrappedSig.Build.Present || wrappedSig.Build.SeverityMax != SevCritical {
		t.Fatalf("wrapped build not extracted (unwrap missing?): %+v", wrappedSig.Build)
	}
}

func TestDigest_PayloadWrapped_FoldsInnerSignals(t *testing.T) {
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
	if v, ok := wrapSig.GenericValue("build.files_touched"); !ok || v != float64(4) {
		t.Fatalf("inner bare signal not folded through wrapper: (%v, %v)", v, ok)
	}
	if v, ok := wrapSig.GenericValue("security.precheck"); !ok || v != "clean" {
		t.Fatalf("inner dotted signal not folded through wrapper: (%v, %v)", v, ok)
	}
}

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
