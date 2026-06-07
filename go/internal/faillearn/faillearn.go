// Package faillearn renders deterministic learning artifacts from
// structured failure data — the kernel-owned "failure floor" beneath the
// LLM retrospective (mirrors the integrity-floor pattern: LLM proposes,
// kernel disposes). When the LLM retro cannot run (bridge failure,
// operator reset, loop fatal), these renders guarantee a durable
// retrospective record + failure lesson instead of a stderr WARN.
//
// Leaf package: stdlib + yaml.v3 only. Callers own where artifacts land
// (writer.go) and state.json records (failurelog).
package faillearn

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Scope classifies which layer of the loop terminated abnormally.
type Scope string

const (
	ScopePhase Scope = "phase" // mid-cycle phase failure
	ScopeReset Scope = "reset" // operator `evolve cycle reset`
	ScopeLoop  Scope = "loop"  // loop-runner fatal exit
)

// summaryMaxRunes caps the rendered summary/description. Rune-based so
// multi-byte text truncates cleanly.
const summaryMaxRunes = 500

// deterministicConfidence marks fallback lessons below LLM-authored ones
// (corpus norm ≥0.9) so KB recall can weight them accordingly.
const deterministicConfidence = 0.5

// FailureEvent is the structured failure data the floor renders from.
// All fields come from data the caller already holds — no LLM, no I/O.
type FailureEvent struct {
	Cycle          int
	FailedPhase    string
	Scope          Scope
	Classification string // "cycle-mid-execution-fail" | "operator-reset" | "loop-fatal"
	Verdict        string
	Summary        string // truncated to 500 runes at render time
	Defects        []string
	EvidencePaths  []string
	GitHead        string
	Now            time.Time
}

// lessonYAML mirrors the on-disk corpus schema read by
// research.parseLessonFile. The schema-parity contract test in
// internal/research pins the round trip — change shape there first.
type lessonYAML struct {
	ID               string  `yaml:"id"`
	Pattern          string  `yaml:"pattern"`
	Description      string  `yaml:"description"`
	Confidence       float64 `yaml:"confidence"`
	Source           string  `yaml:"source"`
	Type             string  `yaml:"type"`
	Category         string  `yaml:"category"`
	PreventiveAction string  `yaml:"preventiveAction"`
	FailureContext   struct {
		FailedStep    string `yaml:"failedStep"`
		ErrorCategory string `yaml:"errorCategory"`
		AuditVerdict  string `yaml:"auditVerdict"`
	} `yaml:"failureContext"`
}

// RenderRetrospectiveMarkdown renders the deterministic fallback
// retrospective-report.md. Byte-deterministic for a given event.
func RenderRetrospectiveMarkdown(ev FailureEvent) []byte {
	var b bytes.Buffer
	b.WriteString("<!-- deterministic-fallback: rendered by faillearn (LLM retrospective unavailable) -->\n\n")
	fmt.Fprintf(&b, "# Cycle %d Retrospective Report (deterministic fallback)\n\n", ev.Cycle)
	fmt.Fprintf(&b, "**Cycle:** %d\n", ev.Cycle)
	fmt.Fprintf(&b, "**Scope:** %s\n", ev.Scope)
	if ev.FailedPhase != "" {
		fmt.Fprintf(&b, "**Failed phase:** %s\n", ev.FailedPhase)
	}
	fmt.Fprintf(&b, "**Verdict:** %s\n", ev.Verdict)
	fmt.Fprintf(&b, "**Classification:** %s\n", ev.Classification)
	if ev.GitHead != "" {
		fmt.Fprintf(&b, "**Git head:** %s\n", ev.GitHead)
	}
	fmt.Fprintf(&b, "**Recorded at:** %s\n\n", ev.Now.UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "## What Happened\n\n%s\n", truncateRunes(ev.Summary, summaryMaxRunes))
	writeBulletSection(&b, "Defects", ev.Defects)
	writeBulletSection(&b, "Evidence", ev.EvidencePaths)
	return b.Bytes()
}

// RenderLessonYAML renders one failure-lesson as a YAML LIST (the corpus
// parser unmarshals []lessonYAML — a bare mapping would be invisible to
// KB recall). Returns the stable lesson id "cycle-N-<scope>-<slug>" and
// the body. Byte-deterministic for a given event.
func RenderLessonYAML(ev FailureEvent) (id string, body []byte) {
	id = lessonID(ev)
	entry := lessonYAML{
		ID:               id,
		Pattern:          ev.Classification,
		Description:      truncateRunes(ev.Summary, summaryMaxRunes),
		Confidence:       deterministicConfidence,
		Source:           strings.Join(ev.EvidencePaths, ", "),
		Type:             "failure-lesson",
		Category:         "episodic",
		PreventiveAction: preventiveAction(ev),
		FailureContext: struct {
			FailedStep    string `yaml:"failedStep"`
			ErrorCategory string `yaml:"errorCategory"`
			AuditVerdict  string `yaml:"auditVerdict"`
		}{
			FailedStep:    ev.FailedPhase,
			ErrorCategory: ev.Classification,
			AuditVerdict:  ev.Verdict,
		},
	}
	body, err := yaml.Marshal([]lessonYAML{entry})
	if err != nil {
		// yaml.Marshal of a plain struct slice cannot fail; a silent
		// fallback here would emit a contract-violating artifact, so
		// make the invariant breach loud instead.
		panic("faillearn: yaml.Marshal of lesson entry must not fail: " + err.Error())
	}
	return id, body
}

// lessonID derives the stable artifact id "cycle-N-<scope>-<slug>".
// Slug prefers the failed phase (most specific) over the classification.
func lessonID(ev FailureEvent) string {
	src := ev.FailedPhase
	if src == "" {
		src = ev.Classification
	}
	return fmt.Sprintf("cycle-%d-%s-%s", ev.Cycle, ev.Scope, slugify(src))
}

// slugify lowercases and maps runs of non-alphanumerics to single
// hyphens: "stop_reason=circuit_breaker" → "stop-reason-circuit-breaker".
func slugify(s string) string {
	var b strings.Builder
	prevHyphen := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevHyphen = false
		default:
			if !prevHyphen && b.Len() > 0 {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	return strings.TrimSuffix(b.String(), "-")
}

func preventiveAction(ev FailureEvent) string {
	return fmt.Sprintf(
		"Deterministic fallback lesson — the LLM retrospective was unavailable for cycle %d (%s). "+
			"Review the evidence paths and defects, then re-run a full retrospective if deeper analysis is needed.",
		ev.Cycle, ev.Classification)
}

func writeBulletSection(b *bytes.Buffer, title string, items []string) {
	if len(items) == 0 {
		return
	}
	fmt.Fprintf(b, "\n## %s\n\n", title)
	for _, it := range items {
		fmt.Fprintf(b, "- %s\n", it)
	}
}

// truncateRunes caps s at max runes without allocating in the common
// short-string case (range over a string iterates runes).
func truncateRunes(s string, max int) string {
	if len(s) <= max { // byte length bounds rune count from above
		return s
	}
	n := 0
	for i := range s {
		if n == max {
			return s[:i]
		}
		n++
	}
	return s
}
