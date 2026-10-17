package trail

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/codeindex"
)

var (
	callLine     = regexp.MustCompile(`^\s+calls [←→] (\S+) \((.+):L(\d+)-L\d+\)$`)
	callSiteLine = regexp.MustCompile(`^\s+(\d+: .*)$`)
	skeletonLine = regexp.MustCompile(`^- L(\d+)-L\d+\s+(\S+)\s+(\S+)\s+(.+)$`)
	askHeader    = regexp.MustCompile(`^\d+\.\s+(?:\[[^\]]*\]\s+)?(\S+) · \S+`)
	askLocation  = regexp.MustCompile(`^\s+(\S+):L(\d+)-L\d+$`)
	mapDirLine   = regexp.MustCompile(`^(\S+/)\s*(\d+ files · \d+ symbols.*)$`)
)

func lineNumber(digits string) int {
	n, _ := strconv.Atoi(digits)
	return n
}

func parseCallers(lines []string) []codeindex.Hit {
	var hits []codeindex.Hit
	for _, line := range lines {
		if m := callLine.FindStringSubmatch(line); m != nil {
			hits = append(hits, codeindex.Hit{File: m[2], Line: lineNumber(m[3]), Symbol: m[1]})
		} else if m := callSiteLine.FindStringSubmatch(line); m != nil && len(hits) > 0 && hits[len(hits)-1].Text == "" {
			hits[len(hits)-1].Text = strings.TrimSpace(m[1])
		}
	}
	return hits
}

func parseSkeleton(file string, lines []string) []codeindex.Hit {
	var hits []codeindex.Hit
	for _, line := range lines {
		if m := skeletonLine.FindStringSubmatch(line); m != nil {
			hits = append(hits, codeindex.Hit{File: file, Line: lineNumber(m[1]), Symbol: m[3], Text: strings.TrimSpace(m[4])})
		}
	}
	return hits
}

func parseAsk(lines []string) []codeindex.Hit {
	var hits []codeindex.Hit
	for i, line := range lines {
		if m := askHeader.FindStringSubmatch(line); m != nil {
			if hit, ok := askHit(m[1], lines[i+1:]); ok {
				hits = append(hits, hit)
			}
		}
	}
	return hits
}

func askHit(symbol string, rest []string) (codeindex.Hit, bool) {
	if len(rest) == 0 {
		return codeindex.Hit{}, false
	}
	loc := askLocation.FindStringSubmatch(rest[0])
	if loc == nil {
		return codeindex.Hit{}, false
	}
	hit := codeindex.Hit{File: loc[1], Line: lineNumber(loc[2]), Symbol: symbol}
	if len(rest) > 1 {
		hit.Text = strings.TrimSpace(rest[1])
	}
	return hit, true
}

func parseMap(lines []string) []codeindex.Hit {
	var hits []codeindex.Hit
	for _, line := range lines {
		if m := mapDirLine.FindStringSubmatch(line); m != nil {
			hits = append(hits, codeindex.Hit{File: m[1], Text: strings.TrimSpace(m[2])})
		}
	}
	return hits
}
