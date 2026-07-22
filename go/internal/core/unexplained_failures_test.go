package core

// Cycle-1044/1045/1047 (batch-6 live-fire): three DIFFERENT failures each
// wrote no failure-reason artifact, collapsed to the identical empty-evidence
// fingerprint "|unknown|e30d…", and tripped the identical-fingerprint breaker
// rule with a wrong diagnosis. Two contracts pinned here:
//  (1) every retro path supplies fallback evidence, so distinct failures get
//      DISTINCT fingerprints (the F8 principle: a failure mode must emit its
//      reason into an artifact);
//  (2) the breaker names the degenerate empty-evidence case honestly as an
//      unexplained-failures diagnosability halt, never "identical defects".

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureFailureDigest_FallbackWritesReasonArtifactAndDistinctFingerprints(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	root := t.TempDir()
	wsA, wsB := t.TempDir(), t.TempDir()
	o.ensureFailureDigest(1, root, wsA, "build", "build handoff floor: 1 deterministic check failure(s)")
	o.ensureFailureDigest(2, root, wsB, "audit", "phase audit verdict FAIL routed to retro")

	if _, err := os.Stat(filepath.Join(wsA, "audit-fail-reason.json")); err != nil {
		t.Fatalf("fallback must write the reason artifact (F8: humans + digest share one evidence trail): %v", err)
	}
	da, _ := os.ReadFile(filepath.Join(wsA, "failure-digest.json"))
	db, _ := os.ReadFile(filepath.Join(wsB, "failure-digest.json"))
	if strings.Contains(string(da), "|unknown|") || strings.Contains(string(db), "|unknown|") {
		t.Fatalf("fallback evidence must escape the unknown bucket:\nA=%s\nB=%s", da, db)
	}
	if string(da) == "" || string(db) == "" {
		t.Fatal("digests must be written")
	}
	var fa, fb FailureDigest
	mustUnmarshal(t, da, &fa)
	mustUnmarshal(t, db, &fb)
	if fa.Fingerprint == fb.Fingerprint {
		t.Fatalf("DIFFERENT failures must produce DIFFERENT fingerprints (batch-6 collision): %q", fa.Fingerprint)
	}
}

func TestEnsureFailureDigest_ExistingArtifactBeatsFallback(t *testing.T) {
	o := NewOrchestrator(&fakeStorage{}, &fakeLedger{}, buildRunners(nil))
	root, ws := t.TempDir(), t.TempDir()
	floor := []byte(`{"schema_version":1,"phase":"audit","reasons":["EGPS: red_count=1 (cycle ships only when red_count==0)"]}`)
	if err := os.WriteFile(filepath.Join(ws, "audit-fail-reason.json"), floor, 0o644); err != nil {
		t.Fatal(err)
	}
	o.ensureFailureDigest(3, root, ws, "build", "unrelated fallback")
	raw, _ := os.ReadFile(filepath.Join(ws, "audit-fail-reason.json"))
	if string(raw) != string(floor) {
		t.Fatal("a floor-written reason artifact must never be overwritten by the fallback")
	}
	dg, _ := os.ReadFile(filepath.Join(ws, "failure-digest.json"))
	if !strings.Contains(string(dg), "gate-block") {
		t.Fatalf("digest must classify from the FLOOR artifact, got %s", dg)
	}
}

func TestBlockerBreaker_UnexplainedRuleHonestReason(t *testing.T) {
	empty := "|unknown|e30d249242ac"
	v := EvaluateBlockerBreaker([]FailureDigest{
		{Cycle: 1, Fingerprint: empty, PreClass: "unknown"},
		{Cycle: 2, Fingerprint: empty, PreClass: "unknown"},
		{Cycle: 3, Fingerprint: empty, PreClass: "unknown"},
	}, BlockerBreakerConfig{IdenticalFingerprintCeiling: 3, UnexplainedCeiling: 3})
	if !v.Halt || v.Rule != "unexplained-failures" {
		t.Fatalf("3 empty-evidence digests must halt via unexplained-failures, got %+v", v)
	}
	if !strings.Contains(v.Reason, "no machine-readable failure reason") {
		t.Errorf("reason must state the diagnosability breakdown honestly, got %q", v.Reason)
	}
	if strings.Contains(v.Reason, "identical failure identities") {
		t.Errorf("degenerate bucket must NOT be described as identical defects, got %q", v.Reason)
	}
}

func TestBlockerBreaker_IdenticalRuleSkipsUnexplained(t *testing.T) {
	empty := "|unknown|e30d249242ac"
	v := EvaluateBlockerBreaker([]FailureDigest{
		{Cycle: 1, Fingerprint: empty, PreClass: "unknown"},
		{Cycle: 2, Fingerprint: empty, PreClass: "unknown"},
		{Cycle: 3, Fingerprint: empty, PreClass: "unknown"},
	}, BlockerBreakerConfig{IdenticalFingerprintCeiling: 3}) // unexplained disabled
	if v.Halt {
		t.Fatalf("identical-fingerprint rule must skip the degenerate unknown bucket, got %+v", v)
	}
}

func mustUnmarshal(t *testing.T, raw []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(raw, v); err != nil {
		t.Fatalf("unmarshal: %v (%s)", err, raw)
	}
}
