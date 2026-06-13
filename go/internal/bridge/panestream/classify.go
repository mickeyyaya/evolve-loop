package panestream

import (
	"regexp"
	"strings"
)

// classify.go — the SINGLE channel separator for a tmux pane (ADR-0047). A pane
// co-mingles two logically distinct channels with no structural delimiter: the
// agent's transcript CONTENT and the CLI's volatile CHROME. Every pane detector
// — PaneBusy (liveness), cleanPane/PaneHasSubstantiveChange (progress),
// trimVolatileTail (delta extraction) — is a PROJECTION of ClassifyLine, so the
// chrome vocabulary can never again diverge between detectors. Before this, the
// claude `· Schlepping… (50s · ↑ 3.1k tokens)` line was CHROME to PaneBusy but
// CONTENT to cleanPane — the divergence that let a busy agent's ticking clock
// read as "progress" and a stalled one's read as "stuck" (the 8-instance
// content-vs-chrome disease, ADR-0047).

// Layer is the channel a pane line belongs to.
type Layer int

const (
	// LayerContent is the agent's transcript: tool calls, command output, prose.
	// Only Content counts as progress.
	LayerContent Layer = iota
	// LayerChrome is volatile CLI rendering with no liveness meaning: blank
	// rows, separators, spinner frames, elapsed-clock / token-counter lines,
	// status footers. Stripped from progress; not proof the turn is live.
	LayerChrome
	// LayerAffordance is the subset of chrome that PROVES the turn is live: the
	// interrupt affordance ("esc to interrupt") or the in-turn spinner-stats
	// line ("(<dur> · <arrow> <n> tokens"). Stripped from progress (it is not
	// new work) but read as busy by PaneBusy.
	LayerAffordance
)

// Chrome that carries no liveness signal — consolidated here as the single home
// (these regexes previously lived split across panedelta.go's statusRE and
// bridge/stopreview.go's rxBraille/rxAsciiSpinner/rxDeliberating/rxTokens).
var (
	chromeBrailleRE = regexp.MustCompile(`[⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏]`)
	// chromeAsciiSpinnerRE matches a line that is ONLY an ascii spinner frame
	// (`-` `\` `|` `/` alone, optional trailing space) — NOT a markdown bullet
	// "- real text", which is agent CONTENT. The earlier `^[-/|\\]\s` prefix
	// match trimmed every bullet as chrome (ADR-0047: a closed structural shape,
	// never an open prefix into the content channel).
	chromeAsciiSpinnerRE = regexp.MustCompile(`^[-/|\\]\s*$`)
	chromeDeliberatingRE = regexp.MustCompile(`Deliberating.*[0-9]+[ms]`)
	// chromeTokenCounterRE matches a standalone token-counter line in EITHER
	// stream direction (↓ response / ↑ prompt) — claude renders ↑ during
	// generation, others ↓; matching only one was how a rising ↑ counter leaked
	// into progress.
	chromeTokenCounterRE = regexp.MustCompile(`[↑↓]\s*[0-9]+(?:\.[0-9]+)?k?\s+tokens`)
	// chromeSpinnerLeaders are the reasoning/spinner glyphs that lead a status
	// row (claude ✽/✻, agy ⣯/▸).
	chromeSpinnerLeaders = []string{"✽", "✻", "⣯", "▸"}
)

// ClassifyLine maps one rendered pane line to its channel. Profile-independent:
// the only profile-specific busy signal (ollama's IdlePlaceholder) is a
// whole-pane rule that lives in PaneBusy, not a per-line classification.
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

// isChromeRow reports whether a NON-blank, NON-affordance trimmed line is
// volatile chrome (spinner frame, elapsed clock, token counter, separator,
// status footer). The single home for the no-liveness chrome vocabulary.
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
	// A line of only box-drawing horizontal rules ("─" U+2500) is a separator
	// (t is already non-empty and non-blank here).
	return strings.IndexFunc(t, func(r rune) bool { return r != '─' }) == -1
}

// IsContentLine reports whether a line is agent transcript (the progress
// channel). cleanPane/PaneHasSubstantiveChange keep only these.
func IsContentLine(line string) bool { return ClassifyLine(line) == LayerContent }

// IsAffordanceLine reports whether a line is the live-turn affordance (the
// per-line half of PaneBusy).
func IsAffordanceLine(line string) bool { return ClassifyLine(line) == LayerAffordance }
