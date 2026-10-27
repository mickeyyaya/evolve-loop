package qualitypanel

import "sort"

func scoreMutation(report MutationReport) MutationResult {
	var res MutationResult
	for _, m := range report.Mutants {
		switch m.Status {
		case MutantKilled, MutantTimedOut:
			res.Killed++
		case MutantLived:
			res.Lived++
		case MutantNotCovered:
			res.NotCovered++
		}
	}
	n := res.Killed + res.Lived
	res.TQ1 = Reading{Interval: Wilson(res.Killed, n), Method: MethodWilson95, N: n}
	res.TQ3, res.ZeroUniqueTests = testValue(report)
	return res
}

func testValue(report MutationReport) (Reading, []string) {
	tests := map[string]bool{}
	seconds := 0.0
	for name, s := range report.TestSeconds {
		tests[name] = true
		seconds += s
	}
	unique := map[string]int{}
	uniqueTotal := 0
	for _, m := range report.Mutants {
		if m.Status != MutantKilled && m.Status != MutantTimedOut {
			continue
		}
		for _, name := range m.KilledBy {
			tests[name] = true
		}
		if len(m.KilledBy) == 1 {
			unique[m.KilledBy[0]]++
			uniqueTotal++
		}
	}
	var zero []string
	for name := range tests {
		if unique[name] == 0 {
			zero = append(zero, name)
		}
	}
	sort.Strings(zero)
	reading := Reading{Method: MethodCensus}
	if seconds > 0 {
		reading.Interval, reading.N = Interval{Value: float64(uniqueTotal) / seconds}, len(tests)
	}
	return reading, zero
}
