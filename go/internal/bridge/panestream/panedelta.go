// Package panestream reads tmux capture-pane snapshots of an interactive LLM REPL: it separates
// agent content from CLI chrome, extracts newly stable content lines and judges liveness.
package panestream

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strings"
)

var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]|\x1b\\][^\x07]*\x07")

func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

const volatileFallbackRows = 3

type PaneProfile struct {
	Name            string
	BoundaryMarker  string
	BoundaryExact   bool
	IdlePlaceholder string
	ExhaustedRegex  string
	BusyLineRegex   string
	TokenLineRegex  string
	ModelLabelRegex string
}

var Profiles = map[string]PaneProfile{
	"claude": {Name: "claude", BoundaryMarker: "❯"},
	"codex":  {Name: "codex", BoundaryMarker: "›"},
	"agy":    {Name: "agy", BoundaryMarker: ">", BoundaryExact: true},
	"ollama": {Name: "ollama", BoundaryMarker: ">>>", IdlePlaceholder: "Send a message"},
}

var busyAffordanceRE = regexp.MustCompile(`esc to interrupt|esc to cancel`)

var busySpinnerStatsRE = regexp.MustCompile(`\(\s*\d[\d hms]*·\s*[↑↓]\s*[\d.,]+k?\s*tokens`)

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

func PaneHasSubstantiveChange(prev, cur string) bool {
	return cleanPane(prev) != cleanPane(cur)
}

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

type PaneDelta struct {
	emitted int
	anchor  string
	primed  bool
}

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

	if d.emitted > len(stable) {
		d.emitted = len(stable)
		d.anchor = lastOf(stable)
		return nil
	}
	out := append([]string(nil), stable[d.emitted:]...)
	d.emitted = len(stable)
	if last := lastOf(stable); last != "" {
		d.anchor = last
	}
	return out
}

func lastOf(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[len(s)-1]
}

func lastIndexOf(s []string, want string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == want {
			return i
		}
	}
	return -1
}

func stableLines(rendered string, p PaneProfile) []string {
	lines := normalizedLines(rendered, p)

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

func trimVolatileTail(lines []string) []string {
	end := len(lines)
	for end > 0 && isVolatileTailRow(lines[end-1]) {
		end--
	}
	return lines[:end]
}

var statusRE = regexp.MustCompile(
	`esc to interrupt|esc to cancel|Worked for|Thought for|Generating|Inferring|done thinking|\btokens?$|· /|\? for shortcuts|Send a message`,
)

func isVolatileTailRow(line string) bool {
	return ClassifyLine(line) != LayerContent
}
