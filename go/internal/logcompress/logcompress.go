// Package logcompress is the deterministic filter that elides bulky, low-signal lines of a test log, build output or diff before a prompt carries it.
// See docs/architecture/packages/internal-logcompress.md.
package logcompress

import (
	"fmt"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

const (
	reasonPass   = "passing tests"
	reasonNoise  = "download noise"
	reasonRepeat = "repeated lines"
	reasonDiff   = "diff body"
	diffHeader   = "diff --git "
	hunkHeader   = "@@"
	diffBodyHead = "+- @\\"
	diffAdd      = "+"
	diffDel      = "-"
)

type Result struct {
	Text      string
	Applied   bool
	BytesIn   int
	BytesOut  int
	TokensIn  int
	TokensOut int
}

//apicover:ignore reason=every unit and predicate test executes Compress; the ACS predicate runs apicover without a -cover profile, where any exported func reads as 0% executed
func Compress(in string, cfg policy.LogCompressionSettings) Result {
	if len(in) < cfg.MinInputBytes || in == "" {
		return passthrough(in, cfg)
	}
	trailing := strings.HasSuffix(in, "\n")
	lines := strings.Split(strings.TrimSuffix(in, "\n"), "\n")
	reasons := classify(lines, cfg)
	out, elided := render(lines, reasons)
	if trailing {
		out += "\n"
	}
	if elided == 0 {
		return passthrough(in, cfg)
	}
	res := measured(out, in, cfg)
	if float64(res.TokensIn-res.TokensOut) < cfg.MinReductionRatio*float64(res.TokensIn) {
		return passthrough(in, cfg)
	}
	res.Applied = true
	return res
}

func passthrough(in string, cfg policy.LogCompressionSettings) Result {
	return measured(in, in, cfg)
}

func measured(out, in string, cfg policy.LogCompressionSettings) Result {
	return Result{
		Text:      out,
		BytesIn:   len(in),
		BytesOut:  len(out),
		TokensIn:  tokens(len(in), cfg.BytesPerToken),
		TokensOut: tokens(len(out), cfg.BytesPerToken),
	}
}

func tokens(bytes, bytesPerToken int) int {
	return (bytes + bytesPerToken - 1) / bytesPerToken
}

func classify(lines []string, cfg policy.LogCompressionSettings) []string {
	reasons := make([]string, len(lines))
	protected := protectedLines(lines, cfg.NeverCompress)
	collapseDiffs(lines, protected, reasons, cfg.DiffFileMaxLines)
	free := func(i int) bool {
		return !protected[i] && reasons[i] == "" && !containsAny(lines[i], cfg.FailureMarkers)
	}
	for i := 0; i < len(lines); {
		switch {
		case !free(i):
			i++
		case hasAnyPrefix(lines[i], cfg.NoisePrefixes):
			i = markRun(reasons, i, reasonNoise, 1, func(j int) bool { return free(j) && hasAnyPrefix(lines[j], cfg.NoisePrefixes) })
		case hasAnyPrefix(lines[i], cfg.PassPrefixes):
			i = markRun(reasons, i, reasonPass, cfg.PassRunMin, func(j int) bool { return free(j) && hasAnyPrefix(lines[j], cfg.PassPrefixes) })
		default:
			i = markRepeats(lines, reasons, i, cfg.RepeatRunMin, free)
		}
	}
	return reasons
}

func markRun(reasons []string, start int, reason string, minRun int, member func(int) bool) int {
	end := start
	for end < len(reasons) && member(end) {
		end++
	}
	if end-start >= minRun {
		for j := start; j < end; j++ {
			reasons[j] = reason
		}
	}
	return end
}

func markRepeats(lines, reasons []string, start, minRun int, free func(int) bool) int {
	end := start + 1
	for end < len(lines) && lines[end] == lines[start] && free(end) {
		end++
	}
	if end-start >= minRun {
		for j := start + 1; j < end; j++ {
			reasons[j] = reasonRepeat
		}
	}
	return end
}

func protectedLines(lines, markers []string) []bool {
	protected := make([]bool, len(lines))
	inRegion := false
	for i, line := range lines {
		switch {
		case strings.TrimSpace(line) == "":
			inRegion = false
		case !inRegion && containsAny(line, markers):
			inRegion = true
		}
		protected[i] = inRegion
	}
	return protected
}

func collapseDiffs(lines []string, protected []bool, reasons []string, maxLines int) {
	for start := 0; start < len(lines); start++ {
		if !strings.HasPrefix(lines[start], diffHeader) {
			continue
		}
		hunk, end := diffExtent(lines, start)
		if hunk < 0 || end-start <= maxLines || anyProtected(protected, start, end) {
			continue
		}
		adds, dels := 0, 0
		for _, line := range lines[hunk:end] {
			switch {
			case strings.HasPrefix(line, diffAdd):
				adds++
			case strings.HasPrefix(line, diffDel):
				dels++
			}
		}
		stat := fmt.Sprintf("%s +%d -%d", reasonDiff, adds, dels)
		for j := hunk; j < end; j++ {
			reasons[j] = stat
		}
	}
}

func diffExtent(lines []string, start int) (hunk, end int) {
	hunk = -1
	end = start + 1
	for end < len(lines) && !strings.HasPrefix(lines[end], diffHeader) {
		if hunk < 0 && strings.HasPrefix(lines[end], hunkHeader) {
			hunk = end
		}
		if hunk >= 0 && !isDiffLine(lines[end]) {
			break
		}
		end++
	}
	return hunk, end
}

func isDiffLine(line string) bool {
	return line != "" && strings.ContainsRune(diffBodyHead, rune(line[0]))
}

func anyProtected(protected []bool, from, to int) bool {
	for _, p := range protected[from:to] {
		if p {
			return true
		}
	}
	return false
}

func render(lines, reasons []string) (string, int) {
	var out []string
	elided := 0
	for i := 0; i < len(lines); {
		end := i + 1
		for end < len(lines) && reasons[end] == reasons[i] && reasons[i] != "" {
			end++
		}
		group := lines[i:end]
		if reasons[i] != "" {
			if marker := marked(len(group), reasons[i]); len(marker) < joinedLen(group) {
				out = append(out, marker)
				elided += len(group)
				i = end
				continue
			}
		}
		out = append(out, group...)
		i = end
	}
	return strings.Join(out, "\n"), elided
}

func marked(count int, reason string) string {
	return fmt.Sprintf("[logcompress: %d lines elided: %s]", count, reason)
}

func joinedLen(group []string) int {
	total := len(group) - 1
	for _, line := range group {
		total += len(line)
	}
	return total
}

func containsAny(line string, needles []string) bool {
	for _, needle := range needles {
		if strings.Contains(line, needle) {
			return true
		}
	}
	return false
}

func hasAnyPrefix(line string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}
