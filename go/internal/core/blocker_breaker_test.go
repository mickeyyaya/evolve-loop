package core

// blocker_breaker_test.go — RED contract for the mid-batch pipeline-blocker
// breaker (operator directive 2026-07-22: a pipeline blocker must be fixed
// directly, not passed to following cycles). Batch-5 burned SIX cycles on one
// recurring class with every signal on disk and no mechanism acting mid-batch;
// the 862–899 storm burned 37 with byte-identical defect strings. Two
// deterministic rules over the S1 failure digests:
//
//	Rule A — guard-abort class ≥ ceiling (default 2): guard aborts are
//	         pipeline machinery failures by construction, never task-legit.
//	Rule B — byte-identical fingerprint ≥ ceiling (default 3): three
//	         identical failure identities cannot be three honest defects.
//
// Same-task repeats stay S5 quarantine's job (task_retry_ceiling) — the
// breaker is batch-scoped and task-agnostic.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func dg(cycle int, fp, preClass string) FailureDigest {
	return FailureDigest{Cycle: cycle, Fingerprint: fp, PreClass: preClass}
}

func defaultBreakerCfg() BlockerBreakerConfig {
	return BlockerBreakerConfig{GuardClassCeiling: 2, IdenticalFingerprintCeiling: 3}
}

func TestBlockerBreaker_GuardAbortClassHaltsAtCeiling(t *testing.T) {
	var v BlockerVerdict = EvaluateBlockerBreaker([]FailureDigest{
		dg(10, "build|guard-abort|aaa", "guard-abort"),
		dg(12, "audit|guard-abort|bbb", "guard-abort"),
	}, defaultBreakerCfg())
	if !v.Halt || v.Rule != "guard-class" {
		t.Fatalf("2 guard-abort digests must halt via guard-class, got %+v", v)
	}
	if !strings.Contains(v.Reason, "guard-abort") {
		t.Errorf("reason must name the class, got %q", v.Reason)
	}
}

func TestBlockerBreaker_SingleGuardAbortContinues(t *testing.T) {
	if v := EvaluateBlockerBreaker([]FailureDigest{dg(10, "x", "guard-abort")}, defaultBreakerCfg()); v.Halt {
		t.Fatalf("one guard-abort is below ceiling, got halt: %+v", v)
	}
}

func TestBlockerBreaker_IdenticalFingerprintHaltsAtCeiling(t *testing.T) {
	fp := "audit|gate-block|deadbeef1234"
	v := EvaluateBlockerBreaker([]FailureDigest{
		dg(10, fp, "gate-block"), dg(11, fp, "gate-block"), dg(13, fp, "gate-block"),
	}, defaultBreakerCfg())
	if !v.Halt || v.Rule != "identical-fingerprint" || v.Count != 3 {
		t.Fatalf("3 identical fingerprints must halt, got %+v", v)
	}
	if v.Fingerprint != fp {
		t.Errorf("verdict must carry the fingerprint, got %q", v.Fingerprint)
	}
}

func TestBlockerBreaker_DistinctHonestFailuresContinue(t *testing.T) {
	// Batch-2's healthy shape: many FAILs, all distinct task-level catches —
	// the breaker must never halt a batch of honest, different rejections.
	v := EvaluateBlockerBreaker([]FailureDigest{
		dg(10, "audit|gate-block|aaa", "gate-block"),
		dg(11, "audit|gate-block|bbb", "gate-block"),
		dg(12, "audit|verdict-fail|ccc", "verdict-fail"),
		dg(13, "audit|gate-block|ddd", "gate-block"),
		dg(14, "tdd|verdict-fail|eee", "verdict-fail"),
	}, defaultBreakerCfg())
	if v.Halt {
		t.Fatalf("distinct honest failures must not halt, got %+v", v)
	}
}

func TestBlockerBreaker_ZeroCeilingsDisable(t *testing.T) {
	// Explicit zero = rule disabled (policy escape hatch), mirroring the
	// positive-overrides-win threshold merge.
	fp := "a|b|c"
	v := EvaluateBlockerBreaker([]FailureDigest{
		dg(1, fp, "guard-abort"), dg(2, fp, "guard-abort"), dg(3, fp, "guard-abort"),
	}, BlockerBreakerConfig{})
	if v.Halt {
		t.Fatalf("zero ceilings must disable both rules, got %+v", v)
	}
}

// CollectBatchFailureDigests reads only cycles >= fromCycle and tolerates
// missing/malformed digests (a healthy PASS cycle has none).
func TestCollectBatchFailureDigests_ScopesAndTolerates(t *testing.T) {
	evolveDir := t.TempDir()
	write := func(cycle int, body string) {
		d := filepath.Join(evolveDir, "runs", "cycle-"+itoa(cycle))
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "failure-digest.json"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(5, `{"cycle":5,"fingerprint":"old","pre_class":"gate-block"}`)
	write(10, `{"cycle":10,"fingerprint":"in","pre_class":"guard-abort"}`)
	write(11, `MALFORMED`)
	write(12, `{"cycle":12,"fingerprint":"in2","pre_class":"gate-block"}`)

	got := CollectBatchFailureDigests(evolveDir, 10)
	if len(got) != 2 {
		t.Fatalf("want 2 in-scope digests (5 excluded, 11 malformed skipped), got %d: %+v", len(got), got)
	}
	for _, g := range got {
		if g.Cycle < 10 {
			t.Errorf("out-of-scope cycle %d included", g.Cycle)
		}
	}
}

// --- Cycle-1332: resolved-fingerprints ack ledger ---
//
// Incident: cycle-1329's identical-fingerprint halt (ship|unknown|76d0f4fca190)
// was diagnosed and fixed (#415), consumed twice, and re-tripped the breaker
// on every relaunch — the breaker re-scans disk fresh every call with no
// memory of "already diagnosed and consumed". These tests pin the ack
// ledger (LoadResolvedFingerprints/AppendResolvedFingerprint) and its
// exclusion wiring into EvaluateBlockerBreaker's Rule B.

func TestLoadResolvedFingerprints_ReadsLedgerRecords(t *testing.T) {
	dir := t.TempDir()
	body := `[
		{"fingerprint":"ship|unknown|76d0f4fca190","resolved_at":"2026-08-05T09:30:00Z","resolved_by":"operator-reset"},
		{"fingerprint":"audit|gate-block|deadbeef","resolved_at":"2026-08-05T09:31:00Z","resolved_by":"operator-reset"}
	]`
	if err := os.WriteFile(filepath.Join(dir, "resolved-fingerprints.json"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := LoadResolvedFingerprints(dir)
	if err != nil {
		t.Fatalf("LoadResolvedFingerprints: %v", err)
	}
	if !got["ship|unknown|76d0f4fca190"] || !got["audit|gate-block|deadbeef"] {
		t.Fatalf("want both recorded fingerprints in the set, got %+v", got)
	}
	if len(got) != 2 {
		t.Fatalf("want exactly 2 entries, got %d: %+v", len(got), got)
	}
}

func TestLoadResolvedFingerprints_MissingFileReturnsEmptyNoError(t *testing.T) {
	dir := t.TempDir()
	got, err := LoadResolvedFingerprints(dir)
	if err != nil {
		t.Fatalf("missing ledger must fail-open with no error, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("missing ledger must return an empty set, got %+v", got)
	}
}

func TestEvaluateBlockerBreaker_ExcludesAckedFingerprint(t *testing.T) {
	// Literal cycle-1329 reproduction: 3x identical-fingerprint digests, one
	// of them acked — must NOT halt.
	fp := "ship|unknown|76d0f4fca190"
	cfg := defaultBreakerCfg()
	cfg.AckedFingerprints = map[string]bool{fp: true}
	v := EvaluateBlockerBreaker([]FailureDigest{
		dg(1329, fp, "gate-block"), dg(1330, fp, "gate-block"), dg(1331, fp, "gate-block"),
	}, cfg)
	if v.Halt {
		t.Fatalf("an acked fingerprint must be excluded from Rule B, got halt: %+v", v)
	}
}

func TestEvaluateBlockerBreaker_UnackedIdenticalFingerprintStillHalts(t *testing.T) {
	// Negative/regression: the SAME shape with NO ack for that fingerprint —
	// the ack must be fingerprint-scoped, never a blanket Rule B disable.
	fp := "ship|unknown|76d0f4fca190"
	cfg := defaultBreakerCfg()
	cfg.AckedFingerprints = map[string]bool{"a-different-fingerprint": true}
	v := EvaluateBlockerBreaker([]FailureDigest{
		dg(1329, fp, "gate-block"), dg(1330, fp, "gate-block"), dg(1331, fp, "gate-block"),
	}, cfg)
	if !v.Halt || v.Rule != "identical-fingerprint" {
		t.Fatalf("an unacked identical fingerprint must still halt (ADR-0072 floor unweakened), got %+v", v)
	}
}

func TestAppendResolvedFingerprint_WritesRecord(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 5, 10, 0, 0, 0, time.UTC)
	if err := AppendResolvedFingerprint(dir, "ship|unknown|76d0f4fca190", "operator-reset", now); err != nil {
		t.Fatalf("AppendResolvedFingerprint: %v", err)
	}
	got, err := LoadResolvedFingerprints(dir)
	if err != nil {
		t.Fatalf("LoadResolvedFingerprints after append: %v", err)
	}
	if !got["ship|unknown|76d0f4fca190"] {
		t.Fatalf("appended fingerprint must be readable back, got %+v", got)
	}
	// A second append must accumulate, not clobber.
	if err := AppendResolvedFingerprint(dir, "second-fp", "operator-reset", now); err != nil {
		t.Fatalf("second AppendResolvedFingerprint: %v", err)
	}
	got2, err := LoadResolvedFingerprints(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got2) != 2 {
		t.Fatalf("want 2 accumulated records, got %d: %+v", len(got2), got2)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "resolved-fingerprints.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "resolved_by") || !strings.Contains(string(raw), "resolved_at") {
		t.Fatalf("ledger record must carry resolved_by/resolved_at, got %s", raw)
	}
	// Exercise the ResolvedFingerprint record type directly (its JSON shape is
	// the ledger's on-disk contract, not an implementation detail of the
	// loader/writer pair above).
	rec := ResolvedFingerprint{Fingerprint: "x", ResolvedAt: "2026-08-05T10:00:00Z", ResolvedBy: "operator-reset"}
	buf, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal ResolvedFingerprint: %v", err)
	}
	var decoded ResolvedFingerprint
	if err := json.Unmarshal(buf, &decoded); err != nil {
		t.Fatalf("unmarshal ResolvedFingerprint: %v", err)
	}
	if decoded != rec {
		t.Fatalf("ResolvedFingerprint round-trip mismatch: got %+v want %+v", decoded, rec)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
