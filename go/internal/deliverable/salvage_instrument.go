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
//
// Single source: the reader (salvage_report.go) and the CLI open this same
// name via the exported BadVerdictBaselineFile.
const baselineFile = BadVerdictBaselineFile

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
		EventType: badVerdictEventType,
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

// ownSentinelPayload selects the payload of the report's OWN verdict sentinel
// and returns the document with every ECHOED sentinel span excised.
//
// Two rules, both load-bearing, and neither sufficient alone (cycle-1407):
//
//   - Quote-awareness. A sentinel span delimited by a backtick is prose quoting
//     the sentinel SHAPE — a contract example, or another phase's verdict the
//     auditor pasted while describing it. Keying off such a span is the
//     cycle-641 lesson verbatim ("classifiers MUST exclude any span that is a
//     verbatim echo of injected prompt/instruction text"), and it is exactly
//     how the cycle-1298 corpus buried a real FAIL behind five quoted decoys.
//   - Tail anchoring. Among the spans that survive, the LAST wins: a producer
//     emits its real verdict at the tail, while examples accumulate above it.
//     Same selection rule as phasecontract.ParseVerdictSentinelFull, reached
//     independently here because this classifier must see the payloads that
//     parser REJECTED (it cannot reuse a parser that only returns valid ones).
//
// Last-match-wins alone is not decoy immunity — a decoy quoted BELOW the real
// sentinel would win — and quote-awareness alone is not enough either, since a
// document's own sentinel is routinely preceded by unquoted-looking examples.
//
// minimal: "quoted" means CONTAINED IN A CLOSED inline-code span, computed by
// pairing backtick runs (below). Upgrade path if a blockquoted (`> `) echo is
// ever observed: extend inlineCodeSpans' notion of a delimiter — not a new
// parser.
func ownSentinelPayload(content string) (body, payload string, ok bool) {
	spans := sentinelPayloadRE.FindAllStringSubmatchIndex(content, -1)
	if len(spans) == 0 {
		return content, "", false
	}
	quoted := inlineCodeSpans(content)
	var kept strings.Builder
	prev := 0
	for _, m := range spans {
		if !sentinelInClosedSpan(quoted, m[0], m[1]) {
			payload, ok = content[m[2]:m[3]], true
			continue
		}
		kept.WriteString(content[prev:m[0]])
		prev = m[1]
	}
	kept.WriteString(content[prev:])
	return kept.String(), payload, ok
}

// sentinelInClosedSpan reports whether the sentinel span [start,end) lies
// entirely inside one of the CLOSED inline-code spans — the signature of a
// sentinel being DISCUSSED rather than emitted. (Named to stay clear of the
// TestNoQuotedEchoRegression symbol tripwire: the historical helper of the
// old name proved adjacency, not containment, and the ban on that name is
// deliberately kept armed — the cross-lane merge of cycles 1438/1439 landed
// this correct containment logic under the banned name and red'd main.)
//
// Containment, not adjacency (cycle-1407 finding F1): a lone backtick that
// never closes is ordinary prose punctuation, and reading it as a delimiter
// excised reports' own genuine verdicts, inflating the not-recoverable count
// this instrumentation exists to measure honestly.
func sentinelInClosedSpan(quoted [][2]int, start, end int) bool {
	for _, q := range quoted {
		if q[0] <= start && end <= q[1] {
			return true
		}
	}
	return false
}

// inlineCodeSpans returns the half-open byte ranges of the CLOSED markdown
// code spans in content, paired CommonMark-style: a run of N backticks opens a
// span that only a later run of exactly N backticks closes. A run with no such
// partner opens nothing and is literal text, so the next run may open instead —
// which is precisely why an unmatched backtick cannot quote anything.
//
// Fenced blocks fall out of the same rule (a ``` run closed by a later ```), so
// a sentinel inside a fence is excised here too; branch 2 of ClassifyBadVerdict
// still owns the fenced-JSON shape, which is a verdict OBJECT, not a sentinel.
func inlineCodeSpans(content string) [][2]int {
	type run struct{ start, length int }
	var runs []run
	for i := 0; i < len(content); {
		if content[i] != '`' {
			i++
			continue
		}
		j := i
		for j < len(content) && content[j] == '`' {
			j++
		}
		runs = append(runs, run{start: i, length: j - i})
		i = j
	}
	var spans [][2]int
	for i := 0; i < len(runs); i++ {
		for j := i + 1; j < len(runs); j++ {
			if runs[j].length != runs[i].length {
				continue
			}
			spans = append(spans, [2]int{runs[i].start, runs[j].start + runs[j].length})
			i = j
			break
		}
	}
	return spans
}

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
	body, payload, hasSentinel := ownSentinelPayload(content)

	// 1. The sentinel comment shape matched but the payload did not parse. The
	//    only shape we claim as recoverable here is a trailing comma: everything
	//    else that fails json.Unmarshal inside a sentinel is an unknown
	//    corruption, and claiming recoverability we cannot justify would inflate
	//    the very baseline this instrumentation exists to measure honestly.
	if hasSentinel {
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
	//    Searched over `body` (quoted sentinel echoes already excised) so an
	//    echoed sentinel's payload cannot be re-read here as the report's own.
	rest := body
	for _, fb := range fencedBlockRE.FindAllStringSubmatch(body, -1) {
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
