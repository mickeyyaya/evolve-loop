package faillearn

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultNoveltyThreshold = 0.9

func WithNoveltyThreshold(t float64) Option {
	return func(c *writeConfig) { c.noveltyThreshold = t }
}

func (c writeConfig) resolvedNoveltyThreshold() float64 {
	if c.noveltyThreshold > 0 && c.noveltyThreshold <= 1 {
		return c.noveltyThreshold
	}
	return defaultNoveltyThreshold
}

func isNearDuplicate(lessonsDir string, body []byte, threshold float64) bool {
	incoming := observationTokens(body)
	if len(incoming) == 0 {
		return false
	}
	entries, err := os.ReadDir(lessonsDir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		existing, err := os.ReadFile(filepath.Join(lessonsDir, e.Name()))
		if err != nil {
			continue
		}
		if jaccard(incoming, observationTokens(existing)) >= threshold {
			return true
		}
	}
	return false
}

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

func tokenizeObservation(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return (r < 'a' || r > 'z') && (r < '0' || r > '9')
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if strings.IndexFunc(f, func(r rune) bool { return r >= 'a' && r <= 'z' }) < 0 {
			continue
		}
		out = append(out, f)
	}
	return out
}

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
