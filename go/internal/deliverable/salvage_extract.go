package deliverable

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/config"
	evolvelog "github.com/mickeyyaya/evolve-loop/go/internal/log"
	"github.com/mickeyyaya/evolve-loop/go/internal/phasecontract"
)

// salvageAppliedFile records only successful salvages, apart from the unconditional bad-verdict baseline.
const salvageAppliedFile = "salvage-applied.jsonl"

// salvageAppliedEventType is the one event_type the writer stamps and the readers filter on.
const salvageAppliedEventType = "salvage_applied"

// verdictKeyRE matches a "verdict" key case-insensitively, as encoding/json resolves it on the re-verify pass.
var verdictKeyRE = regexp.MustCompile(`(?i)"verdict"\s*:`)

// verdictSpan is a half-open byte range of a verdict-bearing object; end == -1 marks an unterminated one.
type verdictSpan struct{ start, end int }

// verdictCandidates returns every top-level balanced object carrying a verdict key, counting objects, not fences.
// stringAware decides whether a brace inside a JSON string moves the depth. An unterminated candidate still counts (fail closed).
func verdictCandidates(content string, stringAware bool) []verdictSpan {
	var out []verdictSpan
	depth, start := 0, -1
	inStr, esc := false, false
	for i := 0; i < len(content); i++ {
		ch := content[i]
		if !stringAware {
			switch ch {
			case '{':
				if depth == 0 {
					start = i
				}
				depth++
			case '}':
				if depth == 0 {
					continue
				}
				depth--
				if depth == 0 && start >= 0 {
					if verdictKeyRE.MatchString(content[start : i+1]) {
						out = append(out, verdictSpan{start: start, end: i + 1})
					}
					start = -1
				}
			}
			continue
		}
		if inStr {
			switch {
			case esc:
				esc = false
			case ch == '\\':
				esc = true
			case ch == '"':
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{':
			if depth == 0 {
				start = i
			}
			depth++
		case '}':
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 && start >= 0 {
				if verdictKeyRE.MatchString(content[start : i+1]) {
					out = append(out, verdictSpan{start: start, end: i + 1})
				}
				start = -1
			}
		}
	}
	if depth > 0 && start >= 0 && verdictKeyRE.MatchString(content[start:]) {
		out = append(out, verdictSpan{start: start, end: -1})
	}
	return out
}

// candidateCount is the max of three readings (string-aware, brace-only, stateless key count): each stateful scan
// has a one-character silencer in prose, and over-counting costs a salvage while under-counting costs the gate.
func candidateCount(content string) int {
	counts := []int{
		len(verdictCandidates(content, true)),
		len(verdictCandidates(content, false)),
		verdictKeyCount(content),
	}
	max := 0
	for _, n := range counts {
		if n > max {
			max = n
		}
	}
	return max
}

// verdictKeyCount counts "verdict" keys with no scan state, as the decoder sees them: case-insensitive, \uXXXX decoded.
func verdictKeyCount(content string) int {
	return len(verdictKeyRE.FindAllStringIndex(unescapeJSONShort(content), -1))
}

// unescapeJSONShort decodes \uXXXX escapes of ASCII characters; non-ASCII escapes cannot spell "verdict".
func unescapeJSONShort(s string) string {
	if !strings.Contains(s, `\u`) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if i+6 <= len(s) && s[i] == '\\' && s[i+1] == 'u' {
			if n, err := strconv.ParseUint(s[i+2:i+6], 16, 32); err == nil && n < 0x80 {
				b.WriteByte(byte(n))
				i += 6
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

// SalvageVerdict repairs a sole, recoverable, unambiguous bad_verdict whose repaired bytes re-verify clean; otherwise it returns res unchanged.
func SalvageVerdict(res Result) (Result, bool) {
	return salvageVerdictWith(res, phasecontract.BuiltinResolver{}, phasecontract.Roots{}, config.StageOff)
}

// RepairedVerdictContent returns res.Content with its sole recoverable bad_verdict repaired to a canonical sentinel; it writes nothing.
func RepairedVerdictContent(res Result) (string, bool) {
	// Sole violation, never membership: salvage must not erase a co-occurring violation such as the proof-of-read check.
	if res.OK || !res.onlyViolation(CodeBadVerdict) {
		return "", false
	}
	cls := ClassifyBadVerdict(res.Content)
	if !cls.Recoverable {
		return "", false
	}
	if candidateCount(res.Content) > 1 {
		// Genuine ambiguity: refuse rather than pick one candidate.
		return "", false
	}
	return repairVerdict(res.Content, cls)
}

// salvageVerdictWith is SalvageVerdict with the caller's resolver, roots and stage, so the re-verify runs the contract that rejected it.
func salvageVerdictWith(res Result, resolver phasecontract.Resolver, roots phasecontract.Roots, phaseIO config.Stage) (Result, bool) {
	repaired, ok := RepairedVerdictContent(res)
	if !ok {
		return res, false
	}
	c, ok := resolver.Resolve(res.Phase)
	if !ok {
		return res, false
	}
	// Re-verify the repaired bytes rather than set OK, so every content check still applies; the caller's
	// roots keep the path-dependent checks off the process CWD.
	var check Result
	verifyMarkdown(&check, c, repaired, roots, phaseIO)
	check.finish()
	if !check.OK {
		return res, false
	}
	salvaged := res
	salvaged.OK = true
	salvaged.Violations = nil
	// The approval covers exactly these bytes, so they are what the Result carries.
	salvaged.Content = repaired
	return salvaged, true
}

// repairVerdict re-emits the classifier's qualified payload in place as a canonical sentinel, dropping only trailing
// commas. It never searches on its own, so it cannot repair a span the classifier did not qualify.
func repairVerdict(content string, cls BadVerdictClassification) (string, bool) {
	if !cls.Recoverable || !cls.span.addresses(len(content)) || !cls.payload.addresses(len(content)) {
		return "", false
	}
	payload := content[cls.payload.start:cls.payload.end]
	if cls.Pattern == SalvagePatternTrailingComma {
		// Drop the comma, keep the closing brace/bracket the match consumed.
		payload = trailingCommaRE.ReplaceAllStringFunc(payload, func(s string) string { return s[1:] })
	}
	if !json.Valid([]byte(payload)) {
		return "", false
	}
	return content[:cls.span.start] + sentinelLine(payload) + content[cls.span.end:], true
}

// addresses reports whether v is a resolvable half-open range over n bytes, rejecting the unterminated marker.
func (v verdictSpan) addresses(n int) bool {
	return v.start >= 0 && v.end >= v.start && v.end <= n
}

// sentinelLine wraps payload in sentinel markers; RenderVerdictSentinel would drop the agent's other keys.
func sentinelLine(payload string) string { return "<!-- evolve-verdict: " + payload + " -->" }

// recordSalvageApplied appends one salvage-applied record; a write failure is logged and never affects the decision.
func recordSalvageApplied(roots phasecontract.Roots, phase string, pattern SalvagePattern, logf func(string, ...any)) {
	if roots.EvolveDir == "" {
		return
	}
	w := evolvelog.NewSidecarWriter(filepath.Join(roots.EvolveDir, salvageAppliedFile))
	err := w.EmitAbnormal(evolvelog.Event{
		EventType: salvageAppliedEventType,
		Severity:  "info",
		Fields: map[string]any{
			"phase":   phase,
			"pattern": string(pattern),
			"run":     salvageRunID,
		},
	})
	if err != nil && logf != nil {
		logf("[contract-gate] WARN could not append salvage-applied record: %v", err)
	}
}

// SalvageSummaryLine renders "Salvaged verdicts: N (breakdown)" for this run from the salvage-applied sidecar, or "" at zero.
func SalvageSummaryLine(evolveDir string) string {
	// Streamed, not slurped: this runs on every salvage over a never-rotated sidecar.
	f, err := os.Open(filepath.Join(evolveDir, salvageAppliedFile))
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	type appliedRec struct{ pattern, run string }
	recs := make([]appliedRec, 0, 8)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m struct {
			Pattern string `json:"pattern"`
			Run     string `json:"run"`
		}
		if json.Unmarshal([]byte(line), &m) != nil {
			// A torn record is skipped by the total and the breakdown alike, since both come from recs.
			continue
		}
		recs = append(recs, appliedRec{pattern: m.Pattern, run: m.Run})
	}
	// An unread tail would make the count a partial total, so the line is omitted.
	if sc.Err() != nil {
		return ""
	}
	// Scope to this run's records; a legacy sidecar with no run ids reports the whole file.
	scoped := make([]appliedRec, 0, len(recs))
	for _, r := range recs {
		if r.run == salvageRunID {
			scoped = append(scoped, r)
		}
	}
	if len(scoped) > 0 {
		recs = scoped
	}
	if len(recs) == 0 {
		return ""
	}
	counts := map[string]int{}
	order := make([]string, 0, len(recs))
	for _, r := range recs {
		p := operatorPatternLabel(r.pattern)
		if _, seen := counts[p]; !seen {
			order = append(order, p)
		}
		counts[p]++
	}
	parts := make([]string, 0, len(order))
	for _, p := range order {
		parts = append(parts, fmt.Sprintf("%s=%d", p, counts[p]))
	}
	return fmt.Sprintf("Salvaged verdicts: %d (%s)", len(recs), strings.Join(parts, ", "))
}

// salvageRunID tags this process's records; the renderer runs in the process that appended them.
var salvageRunID = strconv.Itoa(os.Getpid())

// unknownPatternBucket is the one label an unrecognised pattern collapses into, bounding what a forged record can say.
const unknownPatternBucket = "unknown"

// operatorPatternLabel renders only minted SalvagePattern values, so a forged pattern in the untrusted sidecar
// cannot pose as gate state in the operator log; anything else still counts, as "unknown".
func operatorPatternLabel(pattern string) string {
	switch SalvagePattern(pattern) {
	case SalvagePatternFencedJSON, SalvagePatternTrailingComma, SalvagePatternDisplaced:
		return pattern
	}
	return unknownPatternBucket
}
