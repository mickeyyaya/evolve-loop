package panestream

import (
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
	re := compiledPattern(pattern)
	if re == nil || re.SubexpIndex("model") < 0 {
		return ""
	}
	m := re.FindStringSubmatch(lastNonBlankLine(stripANSI(rendered)))
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m[re.SubexpIndex("model")])
}

func lastNonBlankLine(s string) string {
	lines := strings.Split(s, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return strings.TrimRight(lines[i], " \t")
		}
	}
	return ""
}
