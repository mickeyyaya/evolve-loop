package phasecontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const SentinelSchemaVersion = 1

const SentinelSchemaVersionFailure = 2

type FailureBlock struct {
	Class         string   `json:"class"`
	Defects       []string `json:"defects,omitempty"`
	EvidencePaths []string `json:"evidence_paths,omitempty"`
	Prescription  []string `json:"prescription,omitempty"`
}

type VerdictSentinel struct {
	Phase         string        `json:"phase"`
	Verdict       string        `json:"verdict"`
	SchemaVersion int           `json:"schema_version"`
	Failure       *FailureBlock `json:"failure,omitempty"`
}

var sentinelRE = regexp.MustCompile(`<!--\s*evolve-verdict:\s*(\{.*?\})\s*-->`)

var placeholderRE = regexp.MustCompile(`^\s*<[^<>]+>\s*$`)

func isPlaceholderEcho(f *FailureBlock) bool {
	if f == nil {
		return false
	}
	for _, entries := range [][]string{f.Defects, f.EvidencePaths, f.Prescription} {
		for _, e := range entries {
			if placeholderRE.MatchString(e) {
				return true
			}
		}
	}
	return false
}

func ParseVerdictSentinelFull(content string) (VerdictSentinel, bool) {
	matches := sentinelRE.FindAllStringSubmatch(content, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		if s, ok := parseSentinelPayload(matches[i][1]); ok {
			return s, true
		}
	}
	return VerdictSentinel{}, false
}

func parseSentinelPayload(payload string) (VerdictSentinel, bool) {
	var s VerdictSentinel
	if err := json.NewDecoder(strings.NewReader(payload)).Decode(&s); err != nil {
		return VerdictSentinel{}, false
	}
	if s.Verdict == "" || isPlaceholderEcho(s.Failure) {
		return VerdictSentinel{}, false
	}
	return s, true
}

func ParseVerdictSentinel(content string) (string, bool) {
	s, ok := ParseVerdictSentinelFull(content)
	return s.Verdict, ok
}

func RenderVerdictSentinelWithFailure(phase, verdict string, failure *FailureBlock) string {
	v := SentinelSchemaVersion
	if failure != nil {
		v = SentinelSchemaVersionFailure
	}
	payload, _ := json.Marshal(VerdictSentinel{Phase: phase, Verdict: verdict, SchemaVersion: v, Failure: failure})
	return "<!-- evolve-verdict: " + string(payload) + " -->"
}

func RenderVerdictSentinel(phase, verdict string) string {
	return RenderVerdictSentinelWithFailure(phase, verdict, nil)
}

func ReadFailureBlock(workspace, phase string) (*FailureBlock, bool) {
	conventional := phase + "-report.md"
	candidates := []string{conventional}
	if c, ok := For(phase); ok && c.Kind == KindMarkdown && c.ArtifactName != conventional {
		candidates = []string{c.ArtifactName, conventional}
	}
	for _, name := range candidates {
		raw, err := os.ReadFile(filepath.Join(workspace, name))
		if err != nil {
			continue
		}
		if s, ok := ParseVerdictSentinelFull(string(raw)); ok && s.Failure != nil && s.Failure.Class != "" {
			return s.Failure, true
		}
	}
	return nil, false
}
