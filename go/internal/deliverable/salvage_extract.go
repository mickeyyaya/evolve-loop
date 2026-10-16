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

const salvageAppliedFile = "salvage-applied.jsonl"

const salvageAppliedEventType = "salvage_applied"

var verdictKeyRE = regexp.MustCompile(`(?i)"verdict"\s*:`)

type verdictSpan struct{ start, end int }

func verdictCandidates(content string, stringAware bool) []verdictSpan {
	var out []verdictSpan
	depth, start := 0, -1
	inStr, esc := false, false
	for i := 0; i < len(content); i++ {
		ch := content[i]
		if !stringAware {
			depth, start, out = balanceBrace(content, i, depth, start, out)
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
		default:
			depth, start, out = balanceBrace(content, i, depth, start, out)
		}
	}
	if depth > 0 && start >= 0 && verdictKeyRE.MatchString(content[start:]) {
		out = append(out, verdictSpan{start: start, end: -1})
	}
	return out
}

func balanceBrace(content string, i, depth, start int, out []verdictSpan) (int, int, []verdictSpan) {
	switch content[i] {
	case '{':
		if depth == 0 {
			start = i
		}
		depth++
	case '}':
		if depth == 0 {
			return depth, start, out
		}
		depth--
		if depth == 0 && start >= 0 {
			if verdictKeyRE.MatchString(content[start : i+1]) {
				out = append(out, verdictSpan{start: start, end: i + 1})
			}
			start = -1
		}
	}
	return depth, start, out
}

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

func verdictKeyCount(content string) int {
	return len(verdictKeyRE.FindAllStringIndex(unescapeJSONShort(content), -1))
}

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

func SalvageVerdict(res Result) (Result, bool) {
	return salvageVerdictWith(res, phasecontract.BuiltinResolver{}, phasecontract.Roots{}, config.StageOff)
}

func RepairedVerdictContent(res Result) (string, bool) {
	if res.OK || !res.onlyViolation(CodeBadVerdict) {
		return "", false
	}
	cls := ClassifyBadVerdict(res.Content)
	if !cls.Recoverable {
		return "", false
	}
	if candidateCount(res.Content) > 1 {
		return "", false
	}
	return repairVerdict(res.Content, cls)
}

func salvageVerdictWith(res Result, resolver phasecontract.Resolver, roots phasecontract.Roots, phaseIO config.Stage) (Result, bool) {
	repaired, ok := RepairedVerdictContent(res)
	if !ok {
		return res, false
	}
	c, ok := resolver.Resolve(res.Phase)
	if !ok {
		return res, false
	}
	var check Result
	if err := verifyMarkdown(&check, c, repaired, roots, phaseIO); err != nil {
		return res, false
	}
	check.finish()
	if !check.OK {
		return res, false
	}
	salvaged := res
	salvaged.OK = true
	salvaged.Violations = nil
	salvaged.Content = repaired
	return salvaged, true
}

func repairVerdict(content string, cls BadVerdictClassification) (string, bool) {
	if !cls.Recoverable || !cls.span.addresses(len(content)) || !cls.payload.addresses(len(content)) {
		return "", false
	}
	payload := content[cls.payload.start:cls.payload.end]
	if cls.Pattern == SalvagePatternTrailingComma {
		payload = trailingCommaRE.ReplaceAllStringFunc(payload, func(s string) string { return s[1:] })
	}
	if !json.Valid([]byte(payload)) {
		return "", false
	}
	return content[:cls.span.start] + sentinelLine(payload) + content[cls.span.end:], true
}

func (v verdictSpan) addresses(n int) bool {
	return v.start >= 0 && v.end >= v.start && v.end <= n
}

func sentinelLine(payload string) string { return "<!-- evolve-verdict: " + payload + " -->" }

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

func SalvageSummaryLine(evolveDir string) string {
	recs, ok := readSalvageApplied(evolveDir)
	if !ok {
		return ""
	}
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

type appliedRec struct{ pattern, run string }

func readSalvageApplied(evolveDir string) ([]appliedRec, bool) {
	f, err := os.Open(filepath.Join(evolveDir, salvageAppliedFile))
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()
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
			continue
		}
		recs = append(recs, appliedRec{pattern: m.Pattern, run: m.Run})
	}
	if sc.Err() != nil {
		return nil, false
	}
	return recs, true
}

var salvageRunID = strconv.Itoa(os.Getpid())

const unknownPatternBucket = "unknown"

func operatorPatternLabel(pattern string) string {
	switch SalvagePattern(pattern) {
	case SalvagePatternFencedJSON, SalvagePatternTrailingComma, SalvagePatternDisplaced:
		return pattern
	}
	return unknownPatternBucket
}
