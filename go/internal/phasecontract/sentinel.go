package phasecontract

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// SentinelSchemaVersion is the baseline sentinel payload version; readers need only the verdict field.
const SentinelSchemaVersion = 1

// SentinelSchemaVersionFailure is the version emitted with a failure block; version 1 lines stay legal forever.
const SentinelSchemaVersionFailure = 2

// FailureBlock is the structured failure context a FAIL/WARN sentinel may carry. See ADR-0039.
type FailureBlock struct {
	Class         string   `json:"class"`
	Defects       []string `json:"defects,omitempty"`
	EvidencePaths []string `json:"evidence_paths,omitempty"`
	// Prescription is a WARN's named remediation for a foreseen risk, kept apart from Defects.
	Prescription []string `json:"prescription,omitempty"`
}

// VerdictSentinel is the full parsed sentinel payload.
type VerdictSentinel struct {
	Phase         string        `json:"phase"`
	Verdict       string        `json:"verdict"`
	SchemaVersion int           `json:"schema_version"`
	Failure       *FailureBlock `json:"failure,omitempty"`
}

var sentinelRE = regexp.MustCompile(`<!--\s*evolve-verdict:\s*(\{.*?\})\s*-->`)

// placeholderRE matches an entry that is wholly an angle-bracket placeholder, which only a
// contract example echoed from scrollback carries; real defects are never wholly bracketed.
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

// ParseVerdictSentinelFull returns the last valid sentinel in content; ok=false sends the caller to its prose parser.
func ParseVerdictSentinelFull(content string) (VerdictSentinel, bool) {
	matches := sentinelRE.FindAllStringSubmatch(content, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		if s, ok := parseSentinelPayload(matches[i][1]); ok {
			return s, true
		}
	}
	return VerdictSentinel{}, false
}

// parseSentinelPayload decodes the payload's leading JSON value, so stray bytes inside the comment
// are tolerated, and rejects a payload with no verdict or a placeholder-echo failure block.
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

// ParseVerdictSentinel returns just the verdict from ParseVerdictSentinelFull.
func ParseVerdictSentinel(content string) (string, bool) {
	s, ok := ParseVerdictSentinelFull(content)
	return s.Verdict, ok
}

// RenderVerdictSentinelWithFailure renders the sentinel line: version 2 with a failure block, the v1 line when failure is nil.
func RenderVerdictSentinelWithFailure(phase, verdict string, failure *FailureBlock) string {
	v := SentinelSchemaVersion
	if failure != nil {
		v = SentinelSchemaVersionFailure
	}
	payload, _ := json.Marshal(VerdictSentinel{Phase: phase, Verdict: verdict, SchemaVersion: v, Failure: failure})
	return "<!-- evolve-verdict: " + string(payload) + " -->"
}

// RenderVerdictSentinel renders the v1 sentinel line with no failure block.
func RenderVerdictSentinel(phase, verdict string) string {
	return RenderVerdictSentinelWithFailure(phase, verdict, nil)
}

// ReadFailureBlock returns the failure block from phase's report, trying the registered artifact, then <phase>-report.md.
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
		// A report without a block is not authoritative: a user phase may carry it in the conventional file.
	}
	return nil, false
}
