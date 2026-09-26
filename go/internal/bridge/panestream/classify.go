package panestream

import (
	"regexp"
	"strings"
)

// Layer is the channel a pane line belongs to.
type Layer int

const (
	// LayerContent is the agent's transcript (tool calls, output, prose); only content counts as progress.
	LayerContent Layer = iota
	// LayerChrome is volatile CLI rendering with no liveness meaning; it is stripped from progress.
	LayerChrome
	// LayerAffordance is chrome that proves the turn is live; it is stripped from progress but reads as busy.
	LayerAffordance
)

var (
	chromeBrailleRE = regexp.MustCompile(`[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]`)
	// A line that is only a spinner frame; a markdown bullet ("- text") is content.
	chromeAsciiSpinnerRE = regexp.MustCompile(`^[-/|\\]\s*$`)
	chromeDeliberatingRE = regexp.MustCompile(`Deliberating.*[0-9]+[ms]`)
	// Both arrows: claude renders ↑ while generating, the other CLIs ↓.
	chromeTokenCounterRE = regexp.MustCompile(`[↑↓]\s*[0-9]+(?:\.[0-9]+)?k?\s+tokens`)
	// Status-row leaders: claude ✽/✻, agy ⣯/▸.
	chromeSpinnerLeaders = []string{"✽", "✻", "⣯", "▸"}
)

// ClassifyLine maps one rendered pane line to its channel, independent of the CLI profile.
// See ADR-0047.
func ClassifyLine(line string) Layer {
	t := strings.TrimSpace(stripANSI(line))
	if t == "" {
		return LayerChrome
	}
	// Affordance first — it proves liveness and must win over generic chrome.
	if busyAffordanceRE.MatchString(t) || busySpinnerStatsRE.MatchString(t) {
		return LayerAffordance
	}
	if isChromeRow(t) {
		return LayerChrome
	}
	return LayerContent
}

// isChromeRow expects a trimmed, non-blank line that is not an affordance.
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
	// A line of only "─" (U+2500) is a separator; t is non-empty here.
	return strings.IndexFunc(t, func(r rune) bool { return r != '─' }) == -1
}

// IsContentLine reports whether a line is agent transcript, the progress channel.
func IsContentLine(line string) bool { return ClassifyLine(line) == LayerContent }

// IsAffordanceLine reports whether a line is the live-turn affordance.
func IsAffordanceLine(line string) bool { return ClassifyLine(line) == LayerAffordance }
