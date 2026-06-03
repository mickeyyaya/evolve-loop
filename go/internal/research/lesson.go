// Package research is the programmatic knowledge-base port: it gives the
// orchestrator and advisor a typed, deterministic way to query the lessons
// corpus (.evolve/instincts/lessons/*.yaml) when interpreting a failure — the
// "recall memory" tier of the three-tier memory model. KB search was previously
// shell-only (scripts/research/kb-search.sh, agent-invoked); this package makes
// it a first-class Go capability with no LLM in the loop (ranking is pure code).
package research

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Lesson is the typed view of one entry in a .evolve/instincts/lessons/*.yaml
// file (schema: skills/evolve-loop/lesson-template.yaml). The flattened
// failureContext fields are the ones relevant to taxonomy-shaped lookup.
type Lesson struct {
	ID               string
	Pattern          string
	Description      string
	Confidence       float64
	Source           string
	Type             string
	Category         string
	PreventiveAction string
	FailedStep       string // failureContext.failedStep — matches Query.Source
	ErrorCategory    string // failureContext.errorCategory
	AuditVerdict     string // failureContext.auditVerdict
	Path             string // source file (forensic; not part of equality)
}

// Digest renders a one-line summary for the advisor's recall-memory prompt
// section. Deterministic and compact.
func (l Lesson) Digest() string {
	action := l.PreventiveAction
	if action == "" {
		action = l.Description
	}
	return l.ID + " (" + l.Pattern + "): " + firstSentence(action)
}

// lessonYAML mirrors the on-disk schema for unmarshalling. Each file is a YAML
// list of these (one root cause per entry).
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

// parseLessonFile reads one YAML file and returns its lessons. A malformed file
// yields an error (the caller decides whether to skip or fail) — never a silent
// empty result, so corpus rot surfaces.
func parseLessonFile(path string) ([]Lesson, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []lessonYAML
	if err := yaml.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	out := make([]Lesson, 0, len(entries))
	for _, e := range entries {
		out = append(out, Lesson{
			ID:               e.ID,
			Pattern:          e.Pattern,
			Description:      e.Description,
			Confidence:       e.Confidence,
			Source:           e.Source,
			Type:             e.Type,
			Category:         e.Category,
			PreventiveAction: e.PreventiveAction,
			FailedStep:       e.FailureContext.FailedStep,
			ErrorCategory:    e.FailureContext.ErrorCategory,
			AuditVerdict:     e.FailureContext.AuditVerdict,
			Path:             path,
		})
	}
	return out, nil
}

// firstSentence trims a body to its first sentence (or the whole thing if short)
// so digests stay one line.
func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, ".\n"); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// listLessonFiles returns every *.yaml under root (non-recursive walk of the
// lessons dir; lessons are flat by convention). Missing root ⇒ empty, not error
// (an absent corpus is normal, not a failure).
func listLessonFiles(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".yaml") || strings.HasSuffix(e.Name(), ".yml") {
			files = append(files, filepath.Join(root, e.Name()))
		}
	}
	return files
}
