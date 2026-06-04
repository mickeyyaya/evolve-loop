// Package panestream extracts newly-stabilized content lines from successive
// tmux `capture-pane` snapshots of an interactive LLM REPL.
//
// Background (see knowledge-base/research/tmux-live-capture-2026-06-04/NOTES.md):
// the live content source for the tmux bridge is polling `capture-pane` (tmux
// renders the pane to clean text) rather than raw `pipe-pane` (which needs a
// full terminal emulator to linearize). Each rendered snapshot is mostly stable
// scrollback with a volatile bottom UI (the empty input box, the footer, and
// the thinking spinner) re-painted every tick. This package emits only the NEW
// stable content above that volatile region.
//
// # Delta-boundary rule (tuned against the real frames in testdata/)
//
// A claude REPL pane has TWO `❯` prompt-marker lines: the echoed submitted
// prompt (e.g. `❯ List 3 short bullet points …`) high in the pane, and the
// EMPTY input box near the bottom. The bottom input box, the `────` separators
// around it, the footer (`⏵⏵ bypass permissions …`), and the status/spinner
// line (`✽ Inferring…` / `✻ Worked for Ns`) are all VOLATILE and must never be
// emitted as content.
//
// The boundary is computed in two trims:
//
//  1. Drop everything at/below the LAST line containing promptMarker. That
//     removes the empty input box, the `────` separator below it, the footer,
//     and any spinner text on the footer line.
//  2. From the BOTTOM of what remains, drop a trailing run of volatile rows —
//     `────` separator lines and status/spinner lines (`✽ …`, `✻ …`) and the
//     blank lines interleaved with them. This is the load-bearing tuning the
//     real frames forced: between the submitted prompt and the empty input box
//     sits a volatile zone that, mid-thinking, is the spinner+separator and,
//     post-answer, is the `✻ Worked for Ns`+separator. Because that zone is the
//     SAME height in the thinking frame as in the answer frame, a naive
//     append-only index cursor primed on the thinking frame would skip the
//     answer bullets (they land where the spinner used to be) and emit only the
//     `✻ Worked`/separator tail. Trimming the volatile tail makes the stable
//     region end exactly at the last real content line in BOTH frames, so the
//     index cursor diffs correctly.
//
// Because the FIRST snapshot primes the baseline (records the current stable
// length and emits nothing), the boot banner, the setup-warning, AND the echoed
// submitted-prompt line are counted at prime time and never re-emitted. So the
// submitted-prompt echo line is NOT emitted as content: it is pre-existing
// chrome absorbed by the priming baseline, exactly like the boot box.
//
// When the answer appears in a later frame it grows the stable region, so only
// the freshly-appended assistant lines (the `⏺ …` bullets) are returned.
// Residual blank/border noise is dropped downstream by the Normalizer's
// plaintext classifier, so this extractor stays simple.
//
// Fallback: if NO line contains promptMarker (REPL not yet drawn / unusual
// pane), the last volatileFallbackRows rows are treated as volatile so a
// half-painted footer/spinner is not mistaken for content.
package panestream

import (
	"regexp"
	"strings"
)

// ansiRE matches the CSI / OSC escape sequences to strip from a rendered
// capture. Copied verbatim from bridge/tmux.go to keep panestream a leaf
// package (stdlib only — no import of the whole bridge package).
var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]|\x1b\\][^\x07]*\x07")

// stripANSI removes terminal escape sequences from a captured pane snapshot.
func stripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// volatileFallbackRows is how many bottom rows are treated as volatile when no
// prompt marker is found in the snapshot (best-effort: input box + separator +
// footer ≈ 3 rows).
const volatileFallbackRows = 3

// PaneDelta tracks how much stable content has already been emitted so each
// capture-pane snapshot yields only NEW content lines (the assistant output),
// excluding the volatile bottom UI (input box / footer / spinner).
type PaneDelta struct {
	emitted int
	primed  bool
}

// Next takes one rendered capture-pane snapshot and the REPL prompt marker
// (e.g. "❯"). It returns content lines that are new since the last call.
//
//   - stable region = lines ABOVE the LAST line containing promptMarker
//     (everything at/below that line is the volatile input box + footer +
//     spinner and is never emitted).
//   - the FIRST call PRIMES the baseline (records the current stable size) and
//     returns nil, so pre-existing boot chrome / prior history is NOT emitted —
//     only content that appears AFTER the extractor starts.
//   - subsequent calls return stable[emitted:].
//   - if the pane scrolled and the stable region shrank (emitted > len), re-anchor
//     to the new length (best-effort; emit nothing that tick).
func (d *PaneDelta) Next(rendered, promptMarker string) []string {
	stable := stableLines(rendered, promptMarker)

	if !d.primed {
		d.emitted = len(stable)
		d.primed = true
		return nil
	}

	if d.emitted > len(stable) {
		// Pane scrolled / stable region shrank — re-anchor, emit nothing.
		d.emitted = len(stable)
		return nil
	}

	out := append([]string(nil), stable[d.emitted:]...)
	d.emitted = len(stable)
	return out
}

// stableLines renders the snapshot to the stable above-prompt region: ANSI is
// stripped, trailing blank rows are trimmed, and everything at/below the LAST
// promptMarker line is dropped. With no marker, the last volatileFallbackRows
// rows are treated as volatile.
func stableLines(rendered, promptMarker string) []string {
	clean := stripANSI(rendered)
	lines := strings.Split(clean, "\n")

	// Right-trim trailing empty (whitespace-only) rows — capture-pane pads the
	// pane to full height with blank lines.
	end := len(lines)
	for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	lines = lines[:end]
	if len(lines) == 0 {
		return nil
	}

	lastMarker := -1
	for i, ln := range lines {
		if strings.Contains(ln, promptMarker) {
			lastMarker = i
		}
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

// trimVolatileTail drops a trailing run of volatile rows (separators, spinner /
// status lines, and the blank rows interleaved with them) from the bottom of
// the above-prompt region so the stable region ends at the last real content
// line. See the package boundary-rule comment for why this is load-bearing.
func trimVolatileTail(lines []string) []string {
	end := len(lines)
	for end > 0 && isVolatileTailRow(lines[end-1]) {
		end--
	}
	return lines[:end]
}

// isVolatileTailRow reports whether a row is part of the volatile zone that
// hugs the input box: a `────` separator, a spinner/status line (claude renders
// these with the `✽`/`✻` leaders), or a blank line.
func isVolatileTailRow(line string) bool {
	t := strings.TrimSpace(line)
	if t == "" {
		return true
	}
	if strings.HasPrefix(t, "✽") || strings.HasPrefix(t, "✻") {
		return true
	}
	// A run of box-drawing horizontal rules ("─" U+2500) is a separator row.
	for _, r := range t {
		if r != '─' {
			return false
		}
	}
	return true
}
