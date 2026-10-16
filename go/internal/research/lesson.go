// Package research is the typed, deterministic query port over the lessons corpus.
// See docs/architecture/packages/internal-research.md.
package research

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/yamlcodec"
)

type Lesson struct {
	ID               string
	Pattern          string
	Description      string
	Confidence       float64
	Source           string
	Type             string
	Category         string
	PreventiveAction string
	FailedStep       string
	ErrorCategory    string
	AuditVerdict     string
	Path             string
}

func (l Lesson) Digest() string {
	action := l.PreventiveAction
	if action == "" {
		action = l.Description
	}
	return l.ID + " (" + l.Pattern + "): " + firstSentence(action)
}

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

func parseLessonFile(path string) ([]Lesson, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var entries []lessonYAML
	if err := yamlcodec.Unmarshal(raw, &entries); err != nil {
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

func firstSentence(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, ".\n"); i > 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

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
