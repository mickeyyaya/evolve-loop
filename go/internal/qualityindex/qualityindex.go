// Package qualityindex is the one home of the quality index the code-review loop and the audit share: its dimensions, thresholds, grammars and qualification test.
// See docs/architecture/packages/internal-qualityindex.md.
package qualityindex

import (
	"fmt"
	"slices"
	"sort"
)

const (
	DefaultThreshold = 4
	minScore         = 1
	maxScore         = 5
	allDimensions    = "*"
)

type dimension struct {
	key     string
	neverNA bool
}

var index = []dimension{
	{"correctness", true},
	{"architecture", true},
	{"maintainability", true},
	{"test-quality", true},
	{"robustness", false},
	{"concurrency", false},
	{"performance", false},
	{"debuggability", false},
	{"security", false},
	{"docs-consistency", true},
}

type Score struct {
	Value     int
	NA        bool
	Rationale string
}

type Scores map[string]Score

type Thresholds map[string]int

type Gap struct {
	Dimension string
	Score     int
	Threshold int
	Reason    string
}

func Keys() []string {
	keys := make([]string, len(index))
	for i, d := range index {
		keys[i] = d.key
	}
	return keys
}

func NeverNA(key string) bool {
	i := slices.IndexFunc(index, func(d dimension) bool { return d.key == key })
	return i >= 0 && index[i].neverNA
}

func Known(key string) bool {
	return slices.ContainsFunc(index, func(d dimension) bool { return d.key == key })
}

func Qualifies(scores Scores, t Thresholds) (bool, []Gap) {
	var gaps []Gap
	for _, key := range Keys() {
		s, scored := scores[key]
		switch {
		case !scored:
			gaps = append(gaps, Gap{Dimension: key, Threshold: t[key], Reason: "missing"})
		case s.NA && NeverNA(key):
			gaps = append(gaps, Gap{Dimension: key, Threshold: t[key], Reason: "N/A not allowed"})
		case s.NA:
		case s.Value < t[key]:
			gaps = append(gaps, Gap{Dimension: key, Score: s.Value, Threshold: t[key]})
		}
	}
	return len(gaps) == 0, gaps
}

func ResolveThresholds(raw map[string]int) (Thresholds, []string) {
	t := Thresholds{}
	for _, key := range Keys() {
		t[key] = DefaultThreshold
	}
	var warnings []string
	if all, ok := raw[allDimensions]; ok {
		if inRange(all) {
			for key := range t {
				t[key] = all
			}
		} else {
			warnings = append(warnings, outOfRange(allDimensions, all))
		}
	}
	for _, key := range sortedKeys(raw) {
		switch v := raw[key]; {
		case key == allDimensions:
		case !Known(key):
			warnings = append(warnings, fmt.Sprintf("workflow.quality_index.thresholds: unknown dimension %q ignored; the dimensions are %v", key, Keys()))
		case !inRange(v):
			warnings = append(warnings, outOfRange(key, v))
		default:
			t[key] = v
		}
	}
	return t, warnings
}

func inRange(score int) bool { return score >= minScore && score <= maxScore }

func outOfRange(key string, v int) string {
	return fmt.Sprintf("workflow.quality_index.thresholds.%s: %d is outside %d–%d; the default stands", key, v, minScore, maxScore)
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
