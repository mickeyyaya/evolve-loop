package panestream

import (
	"regexp"
	"regexp/syntax"
	"strconv"
	"strings"
)

func TokenLinePeak(rendered, pattern string) int {
	re := compiledPattern(pattern)
	if re == nil {
		return 0
	}
	count, scale := re.SubexpIndex("count"), re.SubexpIndex("scale")
	if count < 0 {
		return 0
	}
	peak := 0
	for _, m := range re.FindAllStringSubmatch(stripANSI(rendered), -1) {
		if n := tokenCount(m[count], scaleOf(m, scale)); n > peak {
			peak = n
		}
	}
	return peak
}

func scaleOf(m []string, idx int) string {
	if idx < 0 {
		return ""
	}
	return m[idx]
}

func tokenCount(digits, scale string) int {
	v, err := strconv.ParseFloat(strings.ReplaceAll(digits, ",", ""), 64)
	if err != nil {
		return 0
	}
	if strings.EqualFold(scale, "k") {
		v *= 1000
	}
	return int(v + 0.5)
}

func LastTokenLine(rendered, pattern string) string {
	re := compiledPattern(pattern)
	if re == nil {
		return ""
	}
	matches := re.FindAllString(stripANSI(rendered), -1)
	if len(matches) == 0 {
		return ""
	}
	return strings.TrimSpace(matches[len(matches)-1])
}

func ModelLabel(rendered, pattern string) string {
	lines := lastNonBlankLines(stripANSI(rendered), 1)
	if len(lines) == 0 {
		return ""
	}
	return labelOn(lines[0], pattern)
}

func FooterModelLabel(rendered, pattern string, tail int) string {
	prefix := footerPrefix(pattern)
	if prefix == nil {
		return ""
	}
	for _, line := range lastNonBlankLines(stripANSI(rendered), tail) {
		if prefix.MatchString(line) {
			return labelOn(line, pattern)
		}
	}
	return ""
}

func labelOn(line, pattern string) string {
	re := compiledPattern(pattern)
	if re == nil || re.SubexpIndex("model") < 0 {
		return ""
	}
	if m := re.FindStringSubmatch(line); m != nil {
		return strings.TrimSpace(m[re.SubexpIndex("model")])
	}
	return ""
}

func footerPrefix(pattern string) *regexp.Regexp {
	tree, err := syntax.Parse(pattern, syntax.Perl)
	if err != nil {
		return nil
	}
	footer := namedGroup(tree, "footer")
	if footer == nil {
		return nil
	}
	return compiledPattern(`^(?:` + footer.Sub[0].String() + `)`)
}

func namedGroup(re *syntax.Regexp, name string) *syntax.Regexp {
	if re.Op == syntax.OpCapture && re.Name == name {
		return re
	}
	for _, sub := range re.Sub {
		if group := namedGroup(sub, name); group != nil {
			return group
		}
	}
	return nil
}

func lastNonBlankLines(s string, n int) []string {
	lines := strings.Split(s, "\n")
	var out []string
	for i := len(lines) - 1; i >= 0 && len(out) < n; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			out = append(out, strings.TrimRight(lines[i], " \t"))
		}
	}
	return out
}
