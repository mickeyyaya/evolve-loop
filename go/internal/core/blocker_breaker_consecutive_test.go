package core

import "testing"

func consecCfg(ceiling int) BlockerBreakerConfig {
	return BlockerBreakerConfig{ConsecutiveFailuresCeiling: ceiling}
}

func TestConsecutiveFailures_ThreeDistinctFingerprintsHalt(t *testing.T) {
	digests := []FailureDigest{
		{Cycle: 5, Fingerprint: "audit|verdict-fail|aaa", PreClass: "verdict-fail"},
		{Cycle: 6, Fingerprint: "ship|gate-block|bbb", PreClass: "gate-block"},
		{Cycle: 7, Fingerprint: "audit|verdict-fail|ccc", PreClass: "verdict-fail"},
	}
	v := EvaluateBlockerBreaker(digests, consecCfg(3))
	if !v.Halt {
		t.Fatal("3 consecutive failed cycles with distinct fingerprints must halt — identity-keyed rules alone let the 2026-08-09 batch burn 10 cycles")
	}
	if v.Rule != "consecutive-failures" {
		t.Errorf("rule = %q, want consecutive-failures", v.Rule)
	}
	if v.Count != 3 {
		t.Errorf("count = %d, want 3", v.Count)
	}
}

func TestConsecutiveFailures_PassCycleBreaksTheStreak(t *testing.T) {
	// The missing digest between 6 and 8 is a PASS, so 5,6 and 8,9 are two separate 2-streaks.
	digests := []FailureDigest{
		{Cycle: 5, Fingerprint: "a|x|1", PreClass: "x"},
		{Cycle: 6, Fingerprint: "b|y|2", PreClass: "y"},
		{Cycle: 8, Fingerprint: "c|z|3", PreClass: "z"},
		{Cycle: 9, Fingerprint: "d|w|4", PreClass: "w"},
	}
	if v := EvaluateBlockerBreaker(digests, consecCfg(3)); v.Halt {
		t.Fatalf("a passing cycle must reset the consecutive count, got halt: %s", v.Reason)
	}
}

func TestConsecutiveFailures_ZeroCeilingDisables(t *testing.T) {
	digests := []FailureDigest{
		{Cycle: 1, Fingerprint: "a|x|1", PreClass: "x"},
		{Cycle: 2, Fingerprint: "b|y|2", PreClass: "y"},
		{Cycle: 3, Fingerprint: "c|z|3", PreClass: "z"},
	}
	if v := EvaluateBlockerBreaker(digests, consecCfg(0)); v.Halt {
		t.Fatal("ceiling 0 must disable the rule (per-threshold merge semantics)")
	}
}

func TestConsecutiveFailures_AckedFingerprintBreaksTheStreak(t *testing.T) {
	// The middle digest is acked; counting it would re-halt a batch resumed to verify the fix.
	cfg := consecCfg(3)
	cfg.AckedFingerprints = map[string]bool{"ship|gate-block|fixed": true}
	digests := []FailureDigest{
		{Cycle: 5, Fingerprint: "a|x|1", PreClass: "x"},
		{Cycle: 6, Fingerprint: "ship|gate-block|fixed", PreClass: "gate-block"},
		{Cycle: 7, Fingerprint: "c|z|3", PreClass: "z"},
	}
	if v := EvaluateBlockerBreaker(digests, cfg); v.Halt {
		t.Fatalf("an acked digest must not count toward the streak, got halt: %s", v.Reason)
	}
}

func TestConsecutiveFailures_IdenticalFingerprintRuleWinsOnOverlap(t *testing.T) {
	cfg := consecCfg(3)
	cfg.IdenticalFingerprintCeiling = 3
	digests := []FailureDigest{
		{Cycle: 5, Fingerprint: "ship|gate-block|same", PreClass: "gate-block"},
		{Cycle: 6, Fingerprint: "ship|gate-block|same", PreClass: "gate-block"},
		{Cycle: 7, Fingerprint: "ship|gate-block|same", PreClass: "gate-block"},
	}
	v := EvaluateBlockerBreaker(digests, cfg)
	if !v.Halt || v.Rule != "identical-fingerprint" {
		t.Errorf("halt=%v rule=%q — the specific identity rule must outrank the broad consecutive rule", v.Halt, v.Rule)
	}
}

func TestConsecutiveFailures_UnexplainedDigestsCountTowardTheStreak(t *testing.T) {
	digests := []FailureDigest{
		{Cycle: 5, Fingerprint: "|unknown|deg1", PreClass: "unknown", Unexplained: true},
		{Cycle: 6, Fingerprint: "|unknown|deg2", PreClass: "unknown", Unexplained: true},
		{Cycle: 7, Fingerprint: "audit|verdict-fail|x", PreClass: "verdict-fail"},
	}
	v := EvaluateBlockerBreaker(digests, consecCfg(3))
	if !v.Halt || v.Rule != "consecutive-failures" {
		t.Errorf("halt=%v rule=%q — unexplained failures must count toward the consecutive streak", v.Halt, v.Rule)
	}
}

func TestConsecutiveFailures_InputOrderDoesNotMatter(t *testing.T) {
	digests := []FailureDigest{
		{Cycle: 7, Fingerprint: "c|z|3", PreClass: "z"},
		{Cycle: 5, Fingerprint: "a|x|1", PreClass: "x"},
		{Cycle: 6, Fingerprint: "b|y|2", PreClass: "y"},
	}
	v := EvaluateBlockerBreaker(digests, consecCfg(3))
	if !v.Halt || v.Rule != "consecutive-failures" || v.Count != 3 {
		t.Errorf("halt=%v rule=%q count=%d — verdict must be order-independent", v.Halt, v.Rule, v.Count)
	}
}

func TestConsecutiveFailures_TwoLanesOneWaveCountAsTwo(t *testing.T) {
	// Cycle numbers are batch-global, so a fully failed width-2 wave contributes 2 to the streak.
	digests := []FailureDigest{
		{Cycle: 10, Fingerprint: "a|x|1", PreClass: "x"},
		{Cycle: 11, Fingerprint: "b|y|2", PreClass: "y"},
		{Cycle: 12, Fingerprint: "c|z|3", PreClass: "z"},
		{Cycle: 13, Fingerprint: "d|w|4", PreClass: "w"},
	}
	v := EvaluateBlockerBreaker(digests, consecCfg(3))
	if !v.Halt || v.Count < 3 {
		t.Errorf("halt=%v count=%d — 4 consecutive failed cycles must trip a ceiling of 3", v.Halt, v.Count)
	}
}
