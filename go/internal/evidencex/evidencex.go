// Package evidencex reduces a source log to a bounded, redacted extract with a whole-source digest.
// See docs/architecture/packages/internal-evidencex.md.
package evidencex

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/redact"
)

type Mode string

const (
	ModeExtract Mode = "extract"
	ModePointer Mode = "pointer"
)

const (
	contextLines = 5
	tailLines    = 40
	minRepeats   = 2
)

var (
	errorLineRE = regexp.MustCompile(`\b(?:ERROR|FATAL|PANIC|panic)\b`)
	digitRunRE  = regexp.MustCompile(`[0-9]+`)
)

type Options struct {
	Mode   Mode
	Budget int
}

type Range struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

type Template struct {
	Pattern string `json:"pattern"`
	Count   int    `json:"count"`
	First   string `json:"first"`
	Last    string `json:"last"`
}

type Result struct {
	Digest    string     `json:"digest"`
	Size      int        `json:"size"`
	Text      string     `json:"text"`
	Ranges    []Range    `json:"ranges"`
	Templates []Template `json:"templates"`
}

//apicover:ignore reason=every unit and predicate test executes Extract; the ACS predicate runs apicover without a -cover profile, where any exported func reads as 0% executed
func Extract(src []byte, opts Options) (Result, error) {
	sum := sha256.Sum256(src)
	res := Result{Digest: hex.EncodeToString(sum[:]), Size: len(src)}
	switch opts.Mode {
	case ModePointer:
		return res, nil
	case ModeExtract:
	default:
		return Result{}, fmt.Errorf("evidencex: unknown mode %q", opts.Mode)
	}
	if opts.Budget <= 0 {
		return Result{}, fmt.Errorf("evidencex: budget must be positive, got %d", opts.Budget)
	}
	lines := redactedLines(src)
	kept := selectKept(lines, opts.Budget)
	res.Ranges = rangesOf(kept)
	res.Templates = fitTemplates(lines, kept, opts.Budget)
	res.Text = render(lines, kept, res.Templates)
	return res, nil
}

func redactedLines(src []byte) []string {
	text := strings.TrimSuffix(string(src), "\n")
	if text == "" {
		return nil
	}
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = redact.Secrets(line)
	}
	return lines
}

func selectKept(lines []string, budget int) []bool {
	kept := make([]bool, len(lines))
	for i, line := range lines {
		if !errorLineRE.MatchString(line) {
			continue
		}
		window := make([]int, 0, 2*contextLines+1)
		for j := max(0, i-contextLines); j <= min(len(lines)-1, i+contextLines); j++ {
			if !kept[j] {
				window = append(window, j)
			}
		}
		tryKeep(lines, kept, window, budget)
	}
	for j := len(lines) - 1; j >= max(0, len(lines)-tailLines); j-- {
		if kept[j] {
			continue
		}
		if !tryKeep(lines, kept, []int{j}, budget) {
			break
		}
	}
	return kept
}

func tryKeep(lines []string, kept []bool, idx []int, budget int) bool {
	for _, j := range idx {
		kept[j] = true
	}
	if len(render(lines, kept, nil)) <= budget {
		return true
	}
	for _, j := range idx {
		kept[j] = false
	}
	return false
}

func rangesOf(kept []bool) []Range {
	var out []Range
	for i := 0; i < len(kept); i++ {
		if !kept[i] {
			continue
		}
		start := i
		for i+1 < len(kept) && kept[i+1] {
			i++
		}
		out = append(out, Range{Start: start + 1, End: i + 1})
	}
	return out
}

func fitTemplates(lines []string, kept []bool, budget int) []Template {
	var fitted []Template
	for _, tpl := range templatesOf(lines, kept) {
		next := append(fitted, tpl)
		if len(render(lines, kept, next)) > budget {
			break
		}
		fitted = next
	}
	return fitted
}

func templatesOf(lines []string, kept []bool) []Template {
	index := map[string]int{}
	var tpls []Template
	for i, line := range lines {
		if kept[i] {
			continue
		}
		pattern := digitRunRE.ReplaceAllString(line, "#")
		at, seen := index[pattern]
		if !seen {
			index[pattern] = len(tpls)
			tpls = append(tpls, Template{Pattern: pattern, First: line})
			at = len(tpls) - 1
		}
		tpls[at].Count++
		tpls[at].Last = line
	}
	var repeated []Template
	for _, tpl := range tpls {
		if tpl.Count >= minRepeats {
			repeated = append(repeated, tpl)
		}
	}
	sort.SliceStable(repeated, func(a, b int) bool { return repeated[a].Count > repeated[b].Count })
	return repeated
}

func render(lines []string, kept []bool, tpls []Template) string {
	var b strings.Builder
	for _, r := range rangesOf(kept) {
		fmt.Fprintf(&b, "@@ %d-%d\n", r.Start, r.End)
		for _, line := range lines[r.Start-1 : r.End] {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	if len(tpls) > 0 {
		b.WriteString("## repeats\n")
		for _, tpl := range tpls {
			fmt.Fprintf(&b, "x%d first=%s last=%s\n", tpl.Count, tpl.First, tpl.Last)
		}
	}
	return b.String()
}
