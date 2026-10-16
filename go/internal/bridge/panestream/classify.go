package panestream

import (
	"regexp"
	"strings"
)

type Layer int

const (
	LayerContent Layer = iota
	LayerChrome
	LayerAffordance
)

var (
	chromeBrailleRE      = regexp.MustCompile(`[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]`)
	chromeAsciiSpinnerRE = regexp.MustCompile(`^[-/|\\]\s*$`)
	chromeDeliberatingRE = regexp.MustCompile(`Deliberating.*[0-9]+[ms]`)
	chromeTokenCounterRE = regexp.MustCompile(`[↑↓]\s*[0-9]+(?:\.[0-9]+)?k?\s+tokens`)
	chromeSpinnerLeaders = []string{"✽", "✻", "⣯", "▸"}
)

func ClassifyLine(line string) Layer {
	t := strings.TrimSpace(stripANSI(line))
	if t == "" {
		return LayerChrome
	}
	if busyAffordanceRE.MatchString(t) || busySpinnerStatsRE.MatchString(t) {
		return LayerAffordance
	}
	if isChromeRow(t) {
		return LayerChrome
	}
	return LayerContent
}

func isChromeRow(t string) bool {
	for _, lead := range chromeSpinnerLeaders {
		if strings.HasPrefix(t, lead) {
			return true
		}
	}
	if chromeBrailleRE.MatchString(t) ||
		chromeAsciiSpinnerRE.MatchString(t) ||
		chromeDeliberatingRE.MatchString(t) ||
		chromeTokenCounterRE.MatchString(t) ||
		statusRE.MatchString(t) {
		return true
	}
	return strings.IndexFunc(t, func(r rune) bool { return r != '─' }) == -1
}

func IsContentLine(line string) bool { return ClassifyLine(line) == LayerContent }

func IsAffordanceLine(line string) bool { return ClassifyLine(line) == LayerAffordance }
