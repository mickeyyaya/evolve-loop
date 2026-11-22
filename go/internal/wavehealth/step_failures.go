package wavehealth

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/core"
)

func failuresStep(s Snapshot) Step {
	if !s.Failures.Known {
		return notGathered("A3", "no blocker breaker rule matches")
	}
	f := s.Failures.Value
	cfg := f.Breaker
	st := Step{ID: "A3", Result: ResultOK, Threshold: fmt.Sprintf("breaker ceilings: identical fingerprint %d, guard-class %d, unexplained %d, consecutive %d",
		cfg.IdenticalFingerprintCeiling, cfg.GuardClassCeiling, cfg.UnexplainedCeiling, cfg.ConsecutiveFailuresCeiling)}
	digests, classes := digestsUpTo(f.Digests, s)
	reading := fmt.Sprintf("%d failed cycle(s); classes %s", len(digests), classText(classes))
	verdict := core.EvaluateBlockerBreaker(digests, cfg)
	if !verdict.Halt {
		st.Reading = reading + "; no breaker rule matched"
		return st
	}
	st.Reading = fmt.Sprintf("%s; REPEAT: rule %s, fingerprint %q, count %d", reading, verdict.Rule, verdict.Fingerprint, verdict.Count)
	st.Result, st.Action = ResultBrake, brakeAction
	return st
}

func digestsUpTo(all []DigestAt, s Snapshot) ([]core.FailureDigest, map[string]int) {
	var digests []core.FailureDigest
	classes := map[string]int{}
	for _, d := range all {
		if d.At.After(s.Now) {
			continue
		}
		digests = append(digests, d.Digest)
		classes[classOrUnclassified(d.Class)]++
	}
	return digests, classes
}

func classOrUnclassified(class string) string {
	if class == "" {
		return "unclassified"
	}
	return class
}

func classText(classes map[string]int) string {
	if len(classes) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(classes))
	for _, name := range slices.Sorted(maps.Keys(classes)) {
		parts = append(parts, fmt.Sprintf("%s %d", name, classes[name]))
	}
	return strings.Join(parts, ", ")
}
