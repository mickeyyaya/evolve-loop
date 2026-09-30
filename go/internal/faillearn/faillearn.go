// Package faillearn renders the deterministic failure floor beneath the LLM
// retrospective: a durable retrospective, a failure lesson and inbox remediation.
// See docs/architecture/packages/internal-faillearn.md.
package faillearn

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Scope string

const (
	ScopePhase Scope = "phase"
	ScopeReset Scope = "reset"
	ScopeLoop  Scope = "loop"
)

const summaryMaxRunes = 500

const deterministicConfidence = 0.5

type FailureEvent struct {
	Cycle          int
	FailedPhase    string
	Scope          Scope
	Classification string
	Verdict        string
	Summary        string
	Defects        []string
	EvidencePaths  []string
	GitHead        string
	Now            time.Time
}

type lessonYAML struct {
	ID               string   `yaml:"id"`
	Pattern          string   `yaml:"pattern"`
	Description      string   `yaml:"description"`
	Confidence       float64  `yaml:"confidence"`
	Source           string   `yaml:"source"`
	Type             string   `yaml:"type"`
	Category         string   `yaml:"category"`
	PreventiveAction string   `yaml:"preventiveAction"`
	Defects          []string `yaml:"defects,omitempty"`
	FailureContext   struct {
		FailedStep    string `yaml:"failedStep"`
		ErrorCategory string `yaml:"errorCategory"`
		AuditVerdict  string `yaml:"auditVerdict"`
	} `yaml:"failureContext"`
}

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
		Defects:          StructuredDefects(ev),
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
		panic("faillearn: yaml.Marshal of lesson entry must not fail: " + err.Error())
	}
	return id, body
}

func StructuredDefects(ev FailureEvent) []string {
	defectsOnlyEchoSummary := len(ev.Defects) == 1 && ev.Defects[0] == ev.Summary
	if defectsOnlyEchoSummary {
		return nil
	}
	return ev.Defects
}

func lessonID(ev FailureEvent) string {
	src := ev.FailedPhase
	if src == "" {
		src = ev.Classification
	}
	return fmt.Sprintf("cycle-%d-%s-%s", ev.Cycle, ev.Scope, slugify(src))
}

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

func truncateRunes(s string, max int) string {
	if len(s) <= max {
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
