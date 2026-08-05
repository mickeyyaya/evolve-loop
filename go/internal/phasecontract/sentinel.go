package phasecontract

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// SentinelSchemaVersion is the baseline sentinel payload version. Readers
// tolerate unknown future versions (they only need the verdict field).
const SentinelSchemaVersion = 1

// SentinelSchemaVersionFailure is the version emitted when a failure block is
// present (ADR-0039 §7). Version 1 lines (no failure block) stay legal FOREVER
// — for PASS verdicts and for every artifact written before v2.
const SentinelSchemaVersionFailure = 2

// FailureBlock is the structured failure context a FAIL/WARN verdict may carry
// (ADR-0039 §7): the producing agent's own classification, defect list, and
// machine-readable evidence pointers. Crash-class failures cannot self-report —
// the deterministic floor synthesizes those; this block is for phases healthy
// enough to describe their own failure.
type FailureBlock struct {
	Class         string   `json:"class"`
	Defects       []string `json:"defects,omitempty"`
	EvidencePaths []string `json:"evidence_paths,omitempty"`
	// Prescription carries a WARN's named remediation as structured content
	// (F3, docs/operations/batch-integrity-review-2026-08-04.md:215-238):
	// distinct from Defects because a prescription describes a fix for a
	// foreseen risk, not something itself wrong. Kept as a separate field
	// (not merged into Defects) so downstream consumers can tag entries
	// distinguishably rather than losing that distinction.
	Prescription []string `json:"prescription,omitempty"`
}

// VerdictSentinel is the full parsed sentinel payload.
type VerdictSentinel struct {
	Phase         string        `json:"phase"`
	Verdict       string        `json:"verdict"`
	SchemaVersion int           `json:"schema_version"`
	Failure       *FailureBlock `json:"failure,omitempty"`
}

// sentinelRE captures the JSON payload between the evolve-verdict markers. The
// payload is matched non-greedily so a line with trailing content still parses.
var sentinelRE = regexp.MustCompile(`<!--\s*evolve-verdict:\s*(\{.*?\})\s*-->`)

// placeholderRE matches a failure-block entry that is wholly an angle-bracket
// placeholder token (e.g. `<one line per defect>`, `<artifact path>`). Such a
// value is never genuine agent output — only the Deliverable Contract's own
// printed example echoed into captured scrollback (cycle-603). A real defect
// string is prose and never wholly bracketed, so this is a precise guard with
// no false positives against real failure blocks.
var placeholderRE = regexp.MustCompile(`^\s*<[^<>]+>\s*$`)

// isPlaceholderEcho reports whether the failure block carries a contract-example
// placeholder token in any Defects/EvidencePaths/Prescription entry.
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

// ParseVerdictSentinelFull returns the complete sentinel payload and whether a
// well-formed one was found. v1 and v2 lines both parse (an absent failure
// block is legal); a missing/malformed/verdict-less sentinel yields ok=false
// so the caller falls back to its legacy parser (tolerant reader).
//
// Selection is TAIL-ANCHORED: candidates are walked from the END of the
// document and the LAST valid one wins. A producer emits its real verdict at
// the tail, while prose earlier in the same report routinely QUOTES the
// sentinel shape (contract examples, review commentary). First-match selection
// let those decoys win — or, when a decoy's JSON was elided, blanked the read
// entirely (cycle-1298: five quoted decoys buried a real verdict=FAIL and
// circuit-opened the contract gate). Invalid candidates are skipped, not fatal,
// so a placeholder echo trailing the real sentinel no longer destroys it.
// Single-sentinel documents — the overwhelmingly common case — are unchanged.
func ParseVerdictSentinelFull(content string) (VerdictSentinel, bool) {
	matches := sentinelRE.FindAllStringSubmatch(content, -1)
	for i := len(matches) - 1; i >= 0; i-- {
		if s, ok := parseSentinelPayload(matches[i][1]); ok {
			return s, true
		}
	}
	return VerdictSentinel{}, false
}

// parseSentinelPayload validates ONE candidate payload. A candidate counts only
// when it unmarshals, carries a verdict, and is not a prompt-echoed contract
// example: a sentinel whose failure block still holds literal placeholder
// tokens can only be the Deliverable Contract's own printed example captured
// from scrollback, never a real agent verdict (cycle-603).
func parseSentinelPayload(payload string) (VerdictSentinel, bool) {
	var s VerdictSentinel
	if err := json.Unmarshal([]byte(payload), &s); err != nil {
		return VerdictSentinel{}, false
	}
	if s.Verdict == "" || isPlaceholderEcho(s.Failure) {
		return VerdictSentinel{}, false
	}
	return s, true
}

// ParseVerdictSentinel returns just the verdict — a thin view over
// ParseVerdictSentinelFull (ONE parse, no dual parsers to drift). Kept for the
// many callers that only need the verdict.
func ParseVerdictSentinel(content string) (string, bool) {
	s, ok := ParseVerdictSentinelFull(content)
	return s.Verdict, ok
}

// RenderVerdictSentinelWithFailure produces the sentinel line a producer should
// emit: schema_version 2 when a failure block is present, byte-identical to the
// v1 line when failure is nil (prompt/golden stability for happy paths).
func RenderVerdictSentinelWithFailure(phase, verdict string, failure *FailureBlock) string {
	v := SentinelSchemaVersion
	if failure != nil {
		v = SentinelSchemaVersionFailure
	}
	payload, _ := json.Marshal(VerdictSentinel{Phase: phase, Verdict: verdict, SchemaVersion: v, Failure: failure})
	return "<!-- evolve-verdict: " + string(payload) + " -->"
}

// RenderVerdictSentinel is the no-failure (v1) render — a thin view over
// RenderVerdictSentinelWithFailure so producer and parser stay in lockstep.
func RenderVerdictSentinel(phase, verdict string) string {
	return RenderVerdictSentinelWithFailure(phase, verdict, nil)
}

// ReadFailureBlock reads phase's report in workspace and returns its
// self-reported failure block, when present. This is the ONE reader every
// consumer (router signal lifting, orchestrator faillearn, classifier Pass 0)
// goes through. Artifact-name candidates: the registered contract's name
// (single source — tdd writes test-report.md), then the <phase>-report.md
// convention for user phases. Fail-open: an absent report, sentinel, or
// failure block returns (nil, false) — crash-class failures are the
// supervisor's to synthesize.
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
		// A readable report without a block is NOT authoritative — keep
		// scanning: a user phase may write its registered artifact clean
		// and carry the block in the conventional <phase>-report.md.
	}
	return nil, false
}
