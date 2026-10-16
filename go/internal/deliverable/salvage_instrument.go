package deliverable

import (
	"encoding/json"
	"path/filepath"
	"regexp"

	evolvelog "github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

const baselineFile = BadVerdictBaselineFile

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

func (r Result) hasCode(code string) bool {
	for _, v := range r.Violations {
		if v.Code == code {
			return true
		}
	}
	return false
}

type SalvagePattern string

const (
	SalvagePatternNone          SalvagePattern = ""
	SalvagePatternFencedJSON    SalvagePattern = "fenced-json"
	SalvagePatternTrailingComma SalvagePattern = "trailing-comma"
	SalvagePatternDisplaced     SalvagePattern = "displaced-line"
)

type BadVerdictClassification struct {
	Recoverable bool
	Pattern     SalvagePattern
	Reason      string

	span    verdictSpan
	payload verdictSpan
}

var (
	sentinelPayloadRE = regexp.MustCompile(`(?s)<!--\s*evolve-verdict:\s*(\{.*?\})\s*-->`)
	fencedBlockRE     = regexp.MustCompile("(?s)```[A-Za-z0-9_-]*\\n(.*?)```")
	verdictObjRE      = regexp.MustCompile(`\{[^{}]*"verdict"\s*:[^{}]*\}`)
	trailingCommaRE   = regexp.MustCompile(`,\s*[}\]]`)
)

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

func sentinelInClosedSpan(quoted [][2]int, start, end int) bool {
	for _, q := range quoted {
		if q[0] <= start && end <= q[1] {
			return true
		}
	}
	return false
}

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
	open := make(map[int]int, len(runs))
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

func ClassifyBadVerdict(content string) BadVerdictClassification {
	body, sentinelSpan, payloadSpan, hasSentinel := ownSentinelPayload(content)

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
