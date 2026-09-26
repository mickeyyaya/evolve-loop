package deliverable

import (
	"encoding/json"
	"path/filepath"
	"regexp"

	evolvelog "github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// baselineFile gets one record per bad_verdict block, unconditionally: a sampled baseline would measure the sampler.
const baselineFile = BadVerdictBaselineFile

// recordBadVerdictBaseline records one classification per bad_verdict; a write failure is logged and never reaches the decision.
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

// hasCode reports whether any violation carries code; onlyViolation asks whether it is the sole one.
func (r Result) hasCode(code string) bool {
	for _, v := range r.Violations {
		if v.Code == code {
			return true
		}
	}
	return false
}

// SalvagePattern names a recoverable-malformed verdict shape; the zero value means not recoverable.
type SalvagePattern string

// The recoverable shapes, as stable strings: they key the baseline records aggregated across cycles.
const (
	SalvagePatternNone          SalvagePattern = ""
	SalvagePatternFencedJSON    SalvagePattern = "fenced-json"
	SalvagePatternTrailingComma SalvagePattern = "trailing-comma"
	SalvagePatternDisplaced     SalvagePattern = "displaced-line"
)

// BadVerdictClassification is the classifier's read of one bad_verdict; Reason is always set when Recoverable.
type BadVerdictClassification struct {
	Recoverable bool
	Pattern     SalvagePattern
	Reason      string

	// span (the range to replace) and payload (the JSON inside it) are the offsets the repairer must use,
	// so classifier and repairer can never qualify different spans.
	span    verdictSpan
	payload verdictSpan
}

var (
	// sentinelPayloadRE is looser than phasecontract's sentinel regex: it must see the payloads that parser rejects.
	sentinelPayloadRE = regexp.MustCompile(`(?s)<!--\s*evolve-verdict:\s*(\{.*?\})\s*-->`)
	fencedBlockRE     = regexp.MustCompile("(?s)```[A-Za-z0-9_-]*\\n(.*?)```")
	verdictObjRE      = regexp.MustCompile(`\{[^{}]*"verdict"\s*:[^{}]*\}`)
	trailingCommaRE   = regexp.MustCompile(`,\s*[}\]]`)
)

// ownSentinelPayload selects the report's own sentinel, the last one outside a closed inline-code span, and returns
// content with every quoted echo blanked in place; both rules are needed against quoted decoys.
//
// minimal: "quoted" means CONTAINED IN A CLOSED inline-code span, computed by
// pairing backtick runs (below). Upgrade path if a blockquoted (`> `) echo is
// ever observed: extend inlineCodeSpans' notion of a delimiter — not a new
// parser.
func ownSentinelPayload(content string) (body string, span, payload verdictSpan, ok bool) {
	spans := sentinelPayloadRE.FindAllStringSubmatchIndex(content, -1)
	if len(spans) == 0 {
		return content, verdictSpan{}, verdictSpan{}, false
	}
	quoted := inlineCodeSpans(content)
	var echoed [][2]int
	for _, m := range spans {
		if !sentinelInClosedSpan(quoted, m[0], m[1]) {
			span, payload, ok = verdictSpan{start: m[0], end: m[1]}, verdictSpan{start: m[2], end: m[3]}, true
			continue
		}
		echoed = append(echoed, [2]int{m[0], m[1]})
	}
	return blankSpans(content, echoed), span, payload, ok
}

// blankSpans overwrites each range with spaces, keeping every other byte at its offset and never joining
// fragments into a match the source did not have.
func blankSpans(content string, spans [][2]int) string {
	if len(spans) == 0 {
		return content
	}
	b := []byte(content)
	for _, s := range spans {
		for i := s[0]; i < s[1] && i < len(b); i++ {
			b[i] = ' '
		}
	}
	return string(b)
}

// sentinelInClosedSpan reports whether [start,end) lies inside one closed inline-code span. Containment, not
// adjacency: a lone backtick is prose punctuation and must not excise a report's own verdict.
func sentinelInClosedSpan(quoted [][2]int, start, end int) bool {
	for _, q := range quoted {
		if q[0] <= start && end <= q[1] {
			return true
		}
	}
	return false
}

// inlineCodeSpans returns the closed code spans of content, paired CommonMark-style: a run of N backticks is closed
// only by a later run of exactly N, so an unmatched run quotes nothing. Fences fall out of the same rule.
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
	// Pair through an open-run index, not a nested scan: content is LLM-authored and unbounded.
	var spans [][2]int
	open := make(map[int]int, len(runs)) // run length → index of its pending opener
	for i, r := range runs {
		if o, ok := open[r.length]; ok {
			spans = append(spans, [2]int{runs[o].start, r.start + r.length})
			delete(open, r.length)
			continue
		}
		open[r.length] = i
	}
	return spans
}

// ClassifyBadVerdict purely reports whether a verdict a strict parse rejected was clearly intended, and by which shape (most specific first).
func ClassifyBadVerdict(content string) BadVerdictClassification {
	body, sentinelSpan, payloadSpan, hasSentinel := ownSentinelPayload(content)

	// 1. Only a trailing comma inside a sentinel is claimed recoverable; claiming unknown corruption would inflate the baseline.
	if hasSentinel {
		payload := content[payloadSpan.start:payloadSpan.end]
		if !json.Valid([]byte(payload)) && trailingCommaRE.MatchString(payload) {
			return BadVerdictClassification{
				Recoverable: true,
				Pattern:     SalvagePatternTrailingComma,
				Reason:      "evolve-verdict sentinel payload is JSON with a trailing comma before a closing brace/bracket; a lenient reader recovers it",
				span:        sentinelSpan,
				payload:     payloadSpan,
			}
		}
		return BadVerdictClassification{Reason: "evolve-verdict sentinel present but its payload is not recoverably malformed"}
	}

	// 2. A verdict object in a code fence, searched over body so a quoted echo is never read as the report's own.
	var fences [][2]int
	for _, fb := range fencedBlockRE.FindAllStringSubmatchIndex(body, -1) {
		if loc := verdictObjRE.FindStringIndex(body[fb[2]:fb[3]]); loc != nil {
			return BadVerdictClassification{
				Recoverable: true,
				Pattern:     SalvagePatternFencedJSON,
				Reason:      "a JSON object carrying a \"verdict\" key is wrapped in a markdown code fence instead of the evolve-verdict sentinel comment",
				span:        verdictSpan{start: fb[0], end: fb[1]},
				payload:     verdictSpan{start: fb[2] + loc[0], end: fb[2] + loc[1]},
			}
		}
		fences = append(fences, [2]int{fb[0], fb[1]})
	}
	rest := blankSpans(body, fences)

	// 3. A bare verdict object in prose (a displaced sentinel), searched with every fence blanked.
	if loc := verdictObjRE.FindStringIndex(rest); loc != nil {
		return BadVerdictClassification{
			Recoverable: true,
			Pattern:     SalvagePatternDisplaced,
			Reason:      "a bare JSON object carrying a \"verdict\" key sits in prose with no evolve-verdict comment markers (displaced sentinel)",
			span:        verdictSpan{start: loc[0], end: loc[1]},
			payload:     verdictSpan{start: loc[0], end: loc[1]},
		}
	}

	return BadVerdictClassification{Reason: "no JSON object carrying a \"verdict\" key anywhere in the deliverable — genuinely absent, not recoverable"}
}
