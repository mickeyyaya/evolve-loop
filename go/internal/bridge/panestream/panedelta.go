// Package panestream reads tmux capture-pane snapshots of an interactive LLM REPL: it separates
// agent content from CLI chrome, extracts newly stable content lines and judges liveness.
package panestream

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

// ansiRE duplicates bridge/tmux.go's pattern so panestream stays a stdlib-only leaf.
var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]|\x1b\\][^\x07]*\x07")

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// volatileFallbackRows is the input box, separator and footer, treated as volatile when no boundary marker is found.
const volatileFallbackRows = 3

// PaneProfile describes one CLI's rendered pane: where content ends and how busy and exhaustion show.
type PaneProfile struct {
	Name string
	// BoundaryMarker starts the input line; content is everything above the last line whose
	// left-trimmed text starts with it, so a marker inside prose never matches.
	BoundaryMarker string
	// BoundaryExact requires the trimmed line to equal BoundaryMarker, for an ASCII marker that can
	// start content (agy's ">" vs a markdown blockquote).
	BoundaryExact bool
	// IdlePlaceholder appears only while the REPL is idle at its prompt; PaneBusy reads its absence as busy.
	IdlePlaceholder string
	// ExhaustedRegex is the CLI's quota-wall pattern from the bridge manifest; empty disables exhaustion detection.
	ExhaustedRegex  string
	BusyLineRegex   string
	TokenLineRegex  string
	ModelLabelRegex string
}

// Profiles holds the tuned PaneProfile for each supported tmux LLM CLI.
var Profiles = map[string]PaneProfile{
	"claude": {Name: "claude", BoundaryMarker: "❯"},
	"codex":  {Name: "codex", BoundaryMarker: "›"},
	// The empty input box is exactly ">", while a blockquote or the echoed prompt has text after it.
	"agy": {Name: "agy", BoundaryMarker: ">", BoundaryExact: true},
	// ">>>" prefixes both the echoed prompt and the idle input, and "Thinking…" persists into the
	// answer, so busy is the placeholder's absence.
	"ollama": {Name: "ollama", BoundaryMarker: ">>>", IdlePlaceholder: "Send a message"},
}

// Spinner words are deliberately not matched: the esc affordance already covers claude and agy,
// and a bare word can match an idle answer.
var busyAffordanceRE = regexp.MustCompile(`esc to interrupt|esc to cancel`)

// claude ≥2.1.173 shows only this stats line while busy. The duration run accepts every shape a
// turn passes through ("4s", "12m 34s", "1h 5m").
var busySpinnerStatsRE = regexp.MustCompile(`\(\s*\d[\d hms]*·\s*[↑↓]\s*[\d.,]+k?\s*tokens`)

// PaneBusy reports whether the CLI is generating a turn: an affordance line shows, or IdlePlaceholder is set and absent.
func PaneBusy(rendered string, p PaneProfile) bool {
	clean := stripANSI(rendered)
	lines := strings.Split(clean, "\n")
	if hasBusyLine(lines, p) {
		return true
	}
	for _, line := range lines {
		if IsAffordanceLine(line) {
			return true
		}
	}
	if p.IdlePlaceholder != "" && !strings.Contains(clean, p.IdlePlaceholder) {
		return true
	}
	return false
}

// PaneHasSubstantiveChange reports whether prev and cur differ once chrome is stripped from both.
func PaneHasSubstantiveChange(prev, cur string) bool {
	return cleanPane(prev) != cleanPane(cur)
}

// cleanPane drops affordance lines too, so a ticking spinner-stats clock never reads as progress.
func cleanPane(pane string) string {
	return cleanPaneFor(pane, PaneProfile{})
}

func cleanPaneFor(pane string, p PaneProfile) string {
	var lines []string
	for _, line := range normalizedLines(pane, p) {
		if IsContentLine(line) {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

func ProgressHash(rendered string, p PaneProfile) string {
	sum := sha256.Sum256([]byte(cleanPaneFor(rendered, p)))
	return hex.EncodeToString(sum[:])
}

// PaneDelta yields only the content lines that are new in each successive snapshot.
//
// The cursor anchors on the last emitted line's text, not its index, so content pushed down by a
// prepended prelude is not re-emitted.
type PaneDelta struct {
	emitted int // stable lines already emitted; the positional fallback cursor
	anchor  string
	primed  bool
}

// Next returns the content lines new since the previous call; the first call primes the baseline and returns nil.
func (d *PaneDelta) Next(rendered string, p PaneProfile) []string {
	stable := stableLines(rendered, p)

	if !d.primed {
		d.primed = true
		d.emitted = len(stable)
		d.anchor = lastOf(stable)
		return nil
	}

	if d.anchor != "" {
		if idx := lastIndexOf(stable, d.anchor); idx >= 0 {
			out := append([]string(nil), stable[idx+1:]...)
			d.emitted = len(stable)
			if last := lastOf(stable); last != "" {
				d.anchor = last
			}
			return out
		}
	}

	// Positional fallback: no anchor yet, or the anchor scrolled out of the pane.
	if d.emitted > len(stable) {
		// The stable region shrank: re-anchor and emit nothing.
		d.emitted = len(stable)
		d.anchor = lastOf(stable)
		return nil
	}
	out := append([]string(nil), stable[d.emitted:]...)
	d.emitted = len(stable)
	if last := lastOf(stable); last != "" {
		d.anchor = last
	}
	// A blank last line keeps the old anchor, so the positional cursor drives the next call.
	return out
}

func lastOf(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[len(s)-1]
}

// lastIndexOf searches from the bottom so a repeated line anchors on its freshest copy.
func lastIndexOf(s []string, want string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == want {
			return i
		}
	}
	return -1
}

// stableLines returns the region above the last boundary line minus a trailing volatile run; with
// no boundary, the last volatileFallbackRows rows are volatile.
func stableLines(rendered string, p PaneProfile) []string {
	lines := normalizedLines(rendered, p)

	// capture-pane pads the pane to full height with blank rows.
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	lines = lines[:end]
	if len(lines) == 0 {
		return nil
	}

	lastMarker := LastMarkerLine(lines, p.BoundaryMarker)
	if p.BoundaryExact {
		lastMarker = lastLineWhere(lines, func(trimmed string) bool { return strings.TrimSpace(trimmed) == p.BoundaryMarker })
	}

	if lastMarker < 0 {
		cut := len(lines) - volatileFallbackRows
		if cut < 0 {
			cut = 0
		}
		return trimVolatileTail(lines[:cut])
	}
	return trimVolatileTail(lines[:lastMarker])
}

func LastMarkerLine(lines []string, marker string) int {
	return lastLineWhere(lines, func(trimmed string) bool { return strings.HasPrefix(trimmed, marker) })
}

func lastLineWhere(lines []string, match func(trimmed string) bool) int {
	for i := len(lines) - 1; i >= 0; i-- {
		if match(strings.TrimLeft(lines[i], " \t")) {
			return i
		}
	}
	return -1
}

// trimVolatileTail makes the stable region end at the last content line in both the thinking and
// the answer frame, whose volatile zones have the same height.
func trimVolatileTail(lines []string) []string {
	end := len(lines)
	for end > 0 && isVolatileTailRow(lines[end-1]) {
		end--
	}
	return lines[:end]
}

// statusRE is the frame-derived union of status, spinner and footer fragments across all four CLIs.
var statusRE = regexp.MustCompile(
	`esc to interrupt|esc to cancel|Worked for|Thought for|Generating|Inferring|done thinking|\btokens?$|· /|\? for shortcuts|Send a message`,
)

func isVolatileTailRow(line string) bool {
	return ClassifyLine(line) != LayerContent
}
