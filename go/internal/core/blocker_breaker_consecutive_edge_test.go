package core

// blocker_breaker_consecutive_edge_test.go — edge-case pins for the #423
// consecutive-failures rule, added with the 2026-08-09 zero-ship batch
// postmortem (docs/incidents/2026-08-09-zero-ship-batch.md). The base suite
// pins the core semantics; these close the reviewer-flagged corners: rule
// precedence against guard-class, ceiling-1 hair trigger, duplicate digests
// for one cycle, and an ack that splits a long run into two sub-ceiling
// halves versus one that leaves an independently-tripping half.

import "testing"

func TestConsecutiveFailures_GuardClassRuleWinsOnOverlap(t *testing.T) {
	cfg := consecCfg(3)
	cfg.GuardClassCeiling = 2
	digests := []FailureDigest{
		{Cycle: 5, Fingerprint: "guard|abort|a", PreClass: guardAbortClass},
		{Cycle: 6, Fingerprint: "guard|abort|b", PreClass: guardAbortClass},
		{Cycle: 7, Fingerprint: "audit|verdict-fail|c", PreClass: "verdict-fail"},
	}
	v := EvaluateBlockerBreaker(digests, cfg)
	if !v.Halt || v.Rule != "guard-class" {
		t.Errorf("halt=%v rule=%q — guard-class carries the specific diagnosis and must name the halt when both rules trip", v.Halt, v.Rule)
	}
}

func TestConsecutiveFailures_CeilingOneHaltsOnSingleFailure(t *testing.T) {
	digests := []FailureDigest{{Cycle: 9, Fingerprint: "a|x|1", PreClass: "x"}}
	v := EvaluateBlockerBreaker(digests, consecCfg(1))
	if !v.Halt || v.Count != 1 {
		t.Errorf("halt=%v count=%d — ceiling 1 is a valid hair-trigger configuration and must halt on the first failure", v.Halt, v.Count)
	}
}

func TestConsecutiveFailures_DuplicateDigestsForOneCycleCountOnce(t *testing.T) {
	// Defense-in-depth: two digests claiming the same cycle must not
	// fabricate a streak of three out of two real cycles.
	digests := []FailureDigest{
		{Cycle: 5, Fingerprint: "a|x|1", PreClass: "x"},
		{Cycle: 5, Fingerprint: "a|x|1-dup", PreClass: "x"},
		{Cycle: 6, Fingerprint: "b|y|2", PreClass: "y"},
	}
	if v := EvaluateBlockerBreaker(digests, consecCfg(3)); v.Halt {
		t.Fatalf("two cycles (one duplicated) must not trip a ceiling of 3: %s", v.Reason)
	}
}

func TestConsecutiveFailures_AckSplittingLongRunBelowCeilingNoHalt(t *testing.T) {
	// 5-run with the middle acked → two 2-runs, ceiling 3 not reached.
	cfg := consecCfg(3)
	cfg.AckedFingerprints = map[string]bool{"mid|fixed|x": true}
	digests := []FailureDigest{
		{Cycle: 5, Fingerprint: "a|x|1", PreClass: "x"},
		{Cycle: 6, Fingerprint: "b|y|2", PreClass: "y"},
		{Cycle: 7, Fingerprint: "mid|fixed|x", PreClass: "x"},
		{Cycle: 8, Fingerprint: "c|z|3", PreClass: "z"},
		{Cycle: 9, Fingerprint: "d|w|4", PreClass: "w"},
	}
	if v := EvaluateBlockerBreaker(digests, cfg); v.Halt {
		t.Fatalf("acking the middle of a 5-run leaves two 2-runs — no halt at ceiling 3: %s", v.Reason)
	}
}

func TestConsecutiveFailures_AckCannotMaskAnIndependentlyTrippingHalf(t *testing.T) {
	// 7-run with one ack still leaves a 3-run on one side — must halt.
	cfg := consecCfg(3)
	cfg.AckedFingerprints = map[string]bool{"mid|fixed|x": true}
	digests := []FailureDigest{
		{Cycle: 5, Fingerprint: "a|x|1", PreClass: "x"},
		{Cycle: 6, Fingerprint: "b|y|2", PreClass: "y"},
		{Cycle: 7, Fingerprint: "mid|fixed|x", PreClass: "x"},
		{Cycle: 8, Fingerprint: "c|z|3", PreClass: "z"},
		{Cycle: 9, Fingerprint: "d|w|4", PreClass: "w"},
		{Cycle: 10, Fingerprint: "e|v|5", PreClass: "v"},
	}
	v := EvaluateBlockerBreaker(digests, cfg)
	if !v.Halt || v.Count != 3 {
		t.Errorf("halt=%v count=%d — cycles 8–10 form an unacked 3-run; one ack must not mask an independently-tripping half", v.Halt, v.Count)
	}
}
