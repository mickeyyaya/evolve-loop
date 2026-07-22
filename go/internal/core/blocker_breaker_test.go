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
	"os"
	"path/filepath"
	"strings"
	"testing"
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
