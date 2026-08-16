package faillearn

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// novelty.go — near-duplicate suppression on the lesson-write seam
// (cycle-1494, `sleep-time-kb-consolidation`).
//
// writeIfAbsent already dedupes by exact PATH, but the lesson id is
// "cycle-N-<scope>-<slug>": the SAME observation recurring on a later cycle
// lands under a different filename, so an install that fails the same way for
// twenty cycles accumulates twenty near-identical lessons. Recall then ranks a
// corpus that is mostly one repeated failure, which is the growth the inbox
// item ("identical observation twice → one write") asks to bound.
//
// The gate is deliberately conservative in ONE direction: suppressing a lesson
// destroys failure evidence and cannot be undone, so anything short of an
// almost token-identical match is written. Two hard rules follow from that:
//
//   - a materially different failure is never suppressed (that is the whole
//     value of the corpus), and
//   - corpus rot is inert — an unparseable neighbour is skipped, never treated
//     as a reason to drop the incoming lesson, and never rewritten or deleted.

// defaultNoveltyThreshold mirrors policy.ResearchConfig().NoveltyThreshold's
// built-in. faillearn is a leaf (stdlib + yaml.v3) and must not import policy,
// so the resolved value arrives via WithNoveltyThreshold and this constant only
// covers the option-free call sites.
const defaultNoveltyThreshold = 0.9

// WithNoveltyThreshold sets the similarity at or above which an incoming lesson
// counts as a near-duplicate of one already in the corpus and is skipped. A
// threshold outside (0,1] falls back to the built-in 0.9 — 0 would suppress
// every write and >1 would disarm the gate, and neither is an intent an
// operator can express by accident. Callers resolve the value from
// policy.ResearchConfig().NoveltyThreshold.
func WithNoveltyThreshold(t float64) Option {
	return func(c *writeConfig) { c.noveltyThreshold = t }
}

// resolvedNoveltyThreshold applies the built-in default to an unset or
// out-of-range option value. Pure.
func (c writeConfig) resolvedNoveltyThreshold() float64 {
	if c.noveltyThreshold > 0 && c.noveltyThreshold <= 1 {
		return c.noveltyThreshold
	}
	return defaultNoveltyThreshold
}

// isNearDuplicate reports whether lessonsDir already holds a lesson whose
// observation is at least threshold-similar to body's. Best-effort by
// construction: any read/parse failure yields false (write the lesson), because
// the failure mode to avoid is losing evidence, not writing one lesson twice.
func isNearDuplicate(lessonsDir string, body []byte, threshold float64) bool {
	incoming := observationTokens(body)
	if len(incoming) == 0 {
		return false // nothing to compare on — never suppress
	}
	entries, err := os.ReadDir(lessonsDir)
	if err != nil {
		return false // absent/unreadable corpus: the lesson is novel by definition
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		existing, err := os.ReadFile(filepath.Join(lessonsDir, e.Name()))
		if err != nil {
			continue // unreadable neighbour must not suppress the incoming lesson
		}
		if jaccard(incoming, observationTokens(existing)) >= threshold {
			return true
		}
	}
	return false
}

// observationTokens reduces a rendered lesson file to the token set that
// identifies WHAT was observed. Only the observation-bearing fields
// participate: id, source (evidence paths) and preventiveAction all embed the
// cycle number, so including them would make every recurrence look novel and
// the gate would never fire — the exact defect it exists to close.
//
// A file that does not parse as the corpus schema yields no tokens, so it can
// neither match nor be mutated: corpus rot is inert here, by construction.
func observationTokens(body []byte) map[string]struct{} {
	var lessons []lessonYAML
	if err := yaml.Unmarshal(body, &lessons); err != nil {
		return nil
	}
	set := map[string]struct{}{}
	for _, l := range lessons {
		fields := []string{
			l.Pattern,
			l.Description,
			l.FailureContext.FailedStep,
			l.FailureContext.ErrorCategory,
			l.FailureContext.AuditVerdict,
			strings.Join(l.Defects, " "),
		}
		for _, tok := range tokenizeObservation(strings.Join(fields, " ")) {
			set[tok] = struct{}{}
		}
	}
	return set
}

// tokenizeObservation lowercases and splits on every non-alphanumeric rune, so
// "cycle-mid-execution-fail" and "cycle mid execution fail" compare equal.
// Digits are dropped: a bare number in a summary is almost always a cycle or
// count, and letting it differ would keep a recurrence from matching.
func tokenizeObservation(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if strings.IndexFunc(f, func(r rune) bool { return r >= 'a' && r <= 'z' }) < 0 {
			continue // pure-digit token (cycle number, count)
		}
		out = append(out, f)
	}
	return out
}

// jaccard is |a∩b| / |a∪b| over token sets — deterministic, dependency-free,
// and symmetric, so "is this a duplicate of that" cannot depend on write order.
// Two empty sets are NOT similar (returns 0): an empty comparison must never
// suppress a write.
func jaccard(a, b map[string]struct{}) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	inter := 0
	for t := range a {
		if _, ok := b[t]; ok {
			inter++
		}
	}
	union := len(a) + len(b) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}
