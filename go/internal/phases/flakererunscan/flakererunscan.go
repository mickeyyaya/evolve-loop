// Package flakererunscan reruns a test and flags it flaky when its outcomes disagree.
// Its verdicts match the vocabulary that downstream gates pattern-match.
package flakererunscan

type Result struct {
	Runs     int
	Passes   int
	Failures int
	Flaky    bool
}

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
