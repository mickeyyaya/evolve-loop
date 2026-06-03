package phasecontract

import (
	"encoding/json"
	"regexp"
)

// The machine-readable verdict sentinel (ADR-0034, Layer 5). Producers emit one
// line of the form:
//
//	<!-- evolve-verdict: {"phase":"audit","verdict":"PASS","schema_version":1} -->
//
// Classifiers parse the sentinel FIRST and fall back to legacy regex-on-prose
// (strangler fig). This removes the verdict-drift failure class — a deterministic
// machine token can't be mis-shaped by the prose heading the way "## Verdict"
// vs "**Verdict:**" did (cycle-148).

// SentinelSchemaVersion is the current sentinel payload version. Bump on a
// breaking shape change; readers tolerate unknown future versions (they only
// need the verdict field).
const SentinelSchemaVersion = 1

// sentinelRE captures the JSON payload between the evolve-verdict markers. The
// payload is matched non-greedily so a line with trailing content still parses.
var sentinelRE = regexp.MustCompile(`<!--\s*evolve-verdict:\s*(\{.*?\})\s*-->`)

// ParseVerdictSentinel returns the verdict declared by an evolve-verdict
// sentinel and whether a well-formed one was found. A missing or malformed
// sentinel yields ("", false) so the caller falls back to its legacy parser
// (tolerant reader).
func ParseVerdictSentinel(content string) (string, bool) {
	m := sentinelRE.FindStringSubmatch(content)
	if m == nil {
		return "", false
	}
	var payload struct {
		Verdict string `json:"verdict"`
	}
	if err := json.Unmarshal([]byte(m[1]), &payload); err != nil {
		return "", false
	}
	if payload.Verdict == "" {
		return "", false
	}
	return payload.Verdict, true
}

// RenderVerdictSentinel produces the sentinel line a producer should emit. Used
// by the persona instructions and tests so the producer and parser stay in
// lockstep.
func RenderVerdictSentinel(phase, verdict string) string {
	payload, _ := json.Marshal(struct {
		Phase         string `json:"phase"`
		Verdict       string `json:"verdict"`
		SchemaVersion int    `json:"schema_version"`
	}{Phase: phase, Verdict: verdict, SchemaVersion: SentinelSchemaVersion})
	return "<!-- evolve-verdict: " + string(payload) + " -->"
}
