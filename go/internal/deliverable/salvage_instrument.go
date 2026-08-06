package deliverable

// salvage_instrument.go — the FIRST deliverable of the schema-aligned salvage
// layer (docs/research/deliverable-alignment-2026-08/README.md, portfolio item
// `schema-aligned-salvage-layer`): MEASUREMENT, not extraction.
//
// The inbox item is explicit that instrumentation ships before any coercion
// logic, because the recoverable-malformed rate on our own CLIs was recorded in
// README §6 as "not yet instrumented". Building an extraction/coercion stage
// against an unmeasured rate is exactly the speculative-generality this repo's
// minimalism discipline rejects.
//
// So this file adds a PURE, log-only classifier over the exact bytes a
// CodeBadVerdict verdict was computed from (Result.Content — the single-read
// seam, deliverable.go:44-64). It never mutates Result.OK or Result.Violations,
// never invents a verdict, and is invoked strictly as an observability side
// effect from Reviewer.Review AFTER the res.OK branch. A cycle's block/approve
// decision is byte-identical with and without it.

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"

	evolvelog "github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// baselineFile is the JSONL sidecar the recoverable-malformed BASELINE is
// counted from (README §7). One append-only record per bad_verdict block, under
// .evolve alongside the gate's own breaker state — no dial, no flag: an
// unconditional measurement is the whole point of the instrumentation-first
// mandate, and a sampled baseline would be a baseline of the sampler.
const baselineFile = "bad-verdict-baseline.jsonl"

// recordBadVerdictBaseline appends one classification record when (and only
// when) the verdict was the thing that failed. Failure to write is logged and
// swallowed: instrumentation must never influence, delay, or brick the gate's
// decision — the contract gate's fail-safe posture outranks its own telemetry.
func recordBadVerdictBaseline(roots phasecontract.Roots, phase string, res Result, logf func(string, ...any)) {
	if !res.hasCode(CodeBadVerdict) || roots.EvolveDir == "" {
		return
	}
	c := ClassifyBadVerdict(res.Content)
	w := evolvelog.NewSidecarWriter(filepath.Join(roots.EvolveDir, baselineFile))
	err := w.EmitAbnormal(evolvelog.Event{
		EventType: "bad_verdict_classified",
		Severity:  "info",
		Fields: map[string]any{
			"phase":         phase,
			"artifact_path": res.ArtifactPath,
			"recoverable":   c.Recoverable,
			"pattern":       string(c.Pattern),
			"reason":        c.Reason,
		},
	})
	if err != nil && logf != nil {
		logf("[contract-gate] WARN could not append bad_verdict baseline record: %v", err)
	}
}

// hasCode reports whether the result carries at least one violation with the
// given code — the "is this failure about the verdict at all" test, kept
// separate from onlyViolation (which asks whether it is the SOLE reason).
func (r Result) hasCode(code string) bool {
	for _, v := range r.Violations {
		if v.Code == code {
			return true
		}
	}
	return false
}

// SalvagePattern names one recoverable-malformed verdict shape. The zero value
// (SalvagePatternNone) means "not recoverable" — a genuinely absent verdict.
type SalvagePattern string

// The three shapes the inbox item names ("fenced/mislabeled JSON, trailing
// commas, displaced sentinel"). Stable strings: they are the `pattern` key of
// the baseline JSONL records and are aggregated across cycles.
const (
	SalvagePatternNone          SalvagePattern = ""
	SalvagePatternFencedJSON    SalvagePattern = "fenced-json"
	SalvagePatternTrailingComma SalvagePattern = "trailing-comma"
	SalvagePatternDisplaced     SalvagePattern = "displaced-line"
)

// BadVerdictClassification is the classifier's read of one bad_verdict
// deliverable. Reason is always non-empty for a recoverable classification:
// a silent classification is not observability — the baseline record has to
// say WHY a future salvage stage would have recovered this report.
type BadVerdictClassification struct {
	Recoverable bool
	Pattern     SalvagePattern
	Reason      string
}

var (
	// sentinelPayloadRE matches the sentinel COMMENT shape and captures its JSON
	// payload. Deliberately looser than phasecontract's own sentinel regex: this
	// classifier's job is to see the shapes that regex+json.Unmarshal REJECTED.
	sentinelPayloadRE = regexp.MustCompile(`(?s)<!--\s*evolve-verdict:\s*(\{.*?\})\s*-->`)
	// fencedBlockRE matches a markdown fenced block and captures its body.
	fencedBlockRE = regexp.MustCompile("(?s)```[A-Za-z0-9_-]*\\n(.*?)```")
	// verdictObjRE matches a flat JSON object carrying a "verdict" key — the
	// clearly-intended-but-misplaced payload a lenient reader could recover.
	verdictObjRE = regexp.MustCompile(`\{[^{}]*"verdict"\s*:[^{}]*\}`)
	// trailingCommaRE matches a comma immediately before a closing brace or
	// bracket — JSON's single most common LLM-emitted syntax error.
	trailingCommaRE = regexp.MustCompile(`,\s*[}\]]`)
)

// ClassifyBadVerdict inspects a deliverable's raw bytes and reports whether the
// verdict a strict parse rejected was nevertheless clearly intended — and by
// which shape. It is a pure function: no I/O, no mutation, no coercion. A
// caller learns only "a salvage stage WOULD have recovered this, via pattern P".
//
// Precedence is deliberate and mutually exclusive, from most specific shape to
// least: a sentinel comment is neither fenced nor displaced, and a fenced block
// is not displaced. Anything with no recoverable JSON verdict object at all —
// prose musings, an empty report — is SalvagePatternNone, Recoverable=false.
func ClassifyBadVerdict(content string) BadVerdictClassification {
	// 1. The sentinel comment shape matched but the payload did not parse. The
	//    only shape we claim as recoverable here is a trailing comma: everything
	//    else that fails json.Unmarshal inside a sentinel is an unknown
	//    corruption, and claiming recoverability we cannot justify would inflate
	//    the very baseline this instrumentation exists to measure honestly.
	if m := sentinelPayloadRE.FindStringSubmatch(content); m != nil {
		payload := m[1]
		if !json.Valid([]byte(payload)) && trailingCommaRE.MatchString(payload) {
			return BadVerdictClassification{
				Recoverable: true,
				Pattern:     SalvagePatternTrailingComma,
				Reason:      "evolve-verdict sentinel payload is JSON with a trailing comma before a closing brace/bracket; a lenient reader recovers it",
			}
		}
		return BadVerdictClassification{Reason: "evolve-verdict sentinel present but its payload is not recoverably malformed"}
	}

	// 2. A verdict object wrapped in a markdown code fence — the agent rendered
	//    the payload as displayable JSON instead of the sentinel comment.
	rest := content
	for _, fb := range fencedBlockRE.FindAllStringSubmatch(content, -1) {
		if verdictObjRE.MatchString(fb[1]) {
			return BadVerdictClassification{
				Recoverable: true,
				Pattern:     SalvagePatternFencedJSON,
				Reason:      "a JSON object carrying a \"verdict\" key is wrapped in a markdown code fence instead of the evolve-verdict sentinel comment",
			}
		}
		rest = strings.Replace(rest, fb[0], "", 1)
	}

	// 3. A bare, uncommented, unfenced verdict object sitting in prose — the
	//    displaced sentinel. Searched over `rest` (fenced blocks removed) so a
	//    fence whose body has no verdict key cannot masquerade as displaced.
	if verdictObjRE.MatchString(rest) {
		return BadVerdictClassification{
			Recoverable: true,
			Pattern:     SalvagePatternDisplaced,
			Reason:      "a bare JSON object carrying a \"verdict\" key sits in prose with no evolve-verdict comment markers (displaced sentinel)",
		}
	}

	return BadVerdictClassification{Reason: "no JSON object carrying a \"verdict\" key anywhere in the deliverable — genuinely absent, not recoverable"}
}
