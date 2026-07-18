// Package flakererunscan is the native, in-process Go implementation of the
// flake-rerun-scan phase. It replaces the former full-LLM boot (measured ~12
// turns / ~5.5K output tokens on the balanced tier) with a deterministic rerun
// loop — Rule 5: deterministic work belongs in code, not LLM cycles.
//
// The phase re-runs a candidate test N times, tallies the outcomes, and flags a
// target Flaky iff it produced BOTH a pass and a failure across the reruns. It
// emits the canonical PASS/WARN/FAIL verdict vocabulary the LLM variant
// produced, so downstream gates that pattern-match the deliverable keep working.
package flakererunscan

// Result is the tally of a rerun campaign: how many attempts ran, how many
// passed, how many failed, and whether the outcomes were inconsistent (Flaky).
type Result struct {
	Runs     int
	Passes   int
	Failures int
	Flaky    bool
}

// Rerun invokes attempt(i) for i in [0, runs) and tallies the outcomes into a
// Result. attempt returns true for a pass, false for a failure. A target is
// Flaky iff it recorded at least one pass AND at least one failure. Rerun is
// deterministic: an identical attempt function yields a byte-identical Result,
// so two campaigns over the same target diff to nothing.
func Rerun(runs int, attempt func(i int) bool) Result {
	var r Result
	for i := 0; i < runs; i++ {
		r.Runs++
		if attempt(i) {
			r.Passes++
		} else {
			r.Failures++
		}
	}
	r.Flaky = r.Passes > 0 && r.Failures > 0
	return r
}

// Verdict maps a Result to the canonical phase verdict vocabulary:
//   - "PASS"    — consistent success (ran at least once, no failures)
//   - "WARN"    — flaky (mixed pass/fail across reruns)
//   - "FAIL"    — consistent failure (ran at least once, no passes)
//   - "SKIPPED" — no attempts ran (Runs == 0)
func (r Result) Verdict() string {
	switch {
	case r.Runs == 0:
		return "SKIPPED"
	case r.Flaky:
		return "WARN"
	case r.Failures == 0:
		return "PASS"
	default:
		return "FAIL"
	}
}
