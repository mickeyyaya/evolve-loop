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
// # Per-CLI PaneProfile
//
// The four supported tmux LLM CLIs (claude, codex, agy, ollama) each paint a
// different bottom-UI, so the extractor is configured per CLI via a PaneProfile
// (see Profiles). A profile carries the ONE thing that differs structurally —
// the input-line BoundaryMarker — and otherwise shares a single volatile-tail
// matcher derived from the real frames under testdata/.
//
// # Delta-boundary rule (general, tuned against the real frames in testdata/)
//
// Every CLI echoes the submitted prompt high in the pane on a line that starts
// with the same marker as its empty input box, then re-paints the EMPTY input
// box near the bottom. The bottom input box, the `────` separators around it,
// the footer (claude `⏵⏵ bypass permissions …`, codex `gpt-5.5 medium · /dir`,
// agy `? for shortcuts … Gemini …`, ollama folds its footer INTO the input
// line), and the status/spinner line (`✽ Inferring…`, `⣯ Generating…`,
// `▸ Thought for Ns`) are all VOLATILE and must never be emitted as content.
//
// The boundary is computed in two trims:
//
//  1. Drop everything at/below the LAST line whose TRIMMED text matches the
//     boundary. Match mode depends on PaneProfile.BoundaryExact: when false
//     (default), HasPrefix on the left-trimmed line is used — the marker must
//     be at line start, so an interior `>` never fires, and codex's
//     placeholder-bearing input box (`› Summarize recent commits`) is correctly
//     caught as the boundary. When true (agy only), the line must equal the
//     marker exactly after trimming — so the empty input box `>` matches but a
//     markdown blockquote `> quoted text` does not.
//     That removes the empty input box, any separator/footer below it, and any
//     spinner text painted on those rows. For codex this also drops the
//     next-prompt echo (`› Summarize recent commits`) that the REPL paints as
//     the new bottom prompt after an answer.
//  2. From the BOTTOM of what remains, drop a trailing run of volatile rows —
//     `────` separators, status/spinner lines, footers, and the blank lines
//     interleaved with them (isVolatileTailRow). This is the load-bearing
//     tuning the real frames forced: between the submitted prompt and the empty
//     input box sits a volatile zone whose height is the SAME in the thinking
//     frame as in the answer frame, so a naive append-only index cursor primed
//     on the thinking frame would skip the answer (it lands where the spinner
//     used to be) and emit only the spinner/separator tail. Trimming the
//     volatile tail makes the stable region end at the last real content line in
//     BOTH frames, so the index cursor diffs correctly.
//
// Because the FIRST snapshot primes the baseline (records the current stable
// length and emits nothing), the boot banner, the setup-warning, AND the echoed
// submitted-prompt line are counted at prime time and never re-emitted.
//
// # CLI-specific notes (verified against testdata/)
//
//   - claude (BoundaryMarker "❯"): two `❯` lines — echoed prompt + empty input.
//     Answer is `⏺ - …` / `  - …`; status leader is `✻`/`✽`.
//   - codex (BoundaryMarker "›"): three `›`-prefixed lines after an answer —
//     echoed prompt, the empty/next input echo, and the next-prompt line; the
//     LAST is dropped at boundary so the footer `gpt-5.5 medium · /dir` and the
//     next-prompt echo `› Summarize recent commits` never leak. Answer `• - …`.
//   - agy (BoundaryMarker ">"): the driver's BOOT marker is the footer
//     `? for shortcuts`, which sits BELOW the input box — NOT the content
//     boundary. The `>`-prefixed lines are the echoed prompt `> In exactly 3…`
//     and the empty input `>`; the LAST is the empty box → correct boundary.
//     agy emits a visible `▸ Thought for Ns` + reasoning preamble before the
//     `•` bullets; that reasoning IS real model output and is emitted as content
//     (only the `▸ Thought for Ns` status leader itself is volatile-trimmed when
//     it is the trailing row).
//   - ollama (BoundaryMarker ">>>"): the bottom line is
//     `>>> Send a message (/? for help)`. DECISION on gemma's chain-of-thought:
//     while generating, the thinking frame has NO bottom `>>>` input line at all
//     (it is replaced by `Thinking…`/CoT), so the last `>>>` at prime time is
//     the echoed prompt and almost nothing is baselined. When the answer frame
//     arrives the whole region above `>>> Send a message` — the visible CoT, the
//     `…done thinking.` delimiter, AND the `*  …` bullets — becomes stable and is
//     emitted. We treat the visible CoT as legitimate content (it is genuinely
//     streamed model output; ollama chose to surface it), NOT as something this
//     low-level extractor silently deletes from the middle of the transcript.
//     The only volatile rows trimmed are the trailing input line / blanks. This
//     keeps the shared rule honest (no per-CLI middle-of-buffer surgery) and
//     leaves any further CoT suppression to a downstream normalizer.
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
// boundary marker is found in the snapshot (best-effort: input box + separator +
// footer ≈ 3 rows).
const volatileFallbackRows = 3

// PaneProfile describes how to find the content boundary in one CLI's rendered
// pane. The only field that differs structurally between CLIs is the
// input-line BoundaryMarker; the volatile-tail matcher (isVolatileTailRow) is
// shared because the real frames showed the same small family of separator /
// status / footer rows across all four CLIs.
type PaneProfile struct {
	// Name identifies the CLI: "claude" | "codex" | "agy" | "ollama".
	Name string
	// BoundaryMarker is the input-line prefix that marks the bottom of content.
	// The stable region is everything ABOVE the LAST line whose left-trimmed
	// text starts with this marker (claude "❯", codex "›", agy ">", ollama
	// ">>>"). Required at line start so a marker char inside prose never matches.
	BoundaryMarker string
	// BoundaryExact, when true, treats a line as the input-box boundary only
	// when its left+right-trimmed text EQUALS BoundaryMarker (the empty input
	// box) — not merely starts with it. Needed for CLIs whose marker is an
	// ASCII char that can legitimately start a content line (agy's ">" vs a
	// markdown blockquote "> quoted"). CLIs with a unique/non-ASCII marker or a
	// placeholder-bearing input box keep prefix matching (BoundaryExact false).
	BoundaryExact bool
	// IdlePlaceholder is a substring that appears ONLY when the REPL is idle at
	// its input prompt — used by PaneBusy for a CLI whose input box vanishes
	// during generation and whose spinner persists into the answer (ollama's
	// ">>> Send a message"). Empty for CLIs whose busy state is detected by the
	// interrupt/spinner affordance instead (claude/agy) or not at all (codex).
	IdlePlaceholder string
	// ExhaustedRegex is the per-CLI quota/rate-limit wall pattern, projected
	// single-source from the bridge manifest's controls.usage.exhausted_regex
	// (the bridge layer sets it in paneProfileFor; panestream stays
	// manifest-agnostic). When the rendered pane matches it the session is
	// LivenessExhausted regardless of any liveness signal — the ExhaustionProbe
	// override that stops a re-printed quota error from masquerading as
	// Converging. Empty = no exhaustion detection for this CLI (fail-open).
	ExhaustedRegex string
}

// Profiles holds the tuned PaneProfile for each supported tmux LLM CLI.
var Profiles = map[string]PaneProfile{
	"claude": {Name: "claude", BoundaryMarker: "❯"},
	"codex":  {Name: "codex", BoundaryMarker: "›"},
	// agy uses BoundaryExact because ">" is plain ASCII and can start a
	// markdown blockquote line ("> quoted text") inside answer content.
	// The empty input box is EXACTLY ">" (nothing after it), while the echoed
	// prompt is "> In exactly 3…" (has text) — exact-match distinguishes them.
	"agy": {Name: "agy", BoundaryMarker: ">", BoundaryExact: true},
	// ollama's ">>>" prefixes BOTH the echoed prompt and the idle input line, so
	// marker presence cannot signal idle; the full input placeholder appears only
	// when idle, and its "Thinking…" spinner persists into the answer — so busy is
	// detected by the placeholder's ABSENCE (see PaneBusy).
	"ollama": {Name: "ollama", BoundaryMarker: ">>>", IdlePlaceholder: "Send a message"},
}

// busyAffordanceRE matches the interrupt/cancel footer hint a CLI shows for the
// ENTIRE interruptible turn (claude "esc to interrupt", agy "esc to cancel") —
// the most reliable busy signal for CLIs whose input box persists during
// generation. The spinner words ("Inferring"/"Generating") are deliberately NOT
// matched: they're redundant (the esc affordance already covers claude/agy busy)
// and, as bare words, could false-match an idle answer that happens to contain
// them. ollama has no esc affordance — it uses the IdlePlaceholder rule instead.
// Verified against every testdata/<cli>/{thinking,answer}.txt.
var busyAffordanceRE = regexp.MustCompile(`esc to interrupt|esc to cancel`)

// busySpinnerStatsRE matches the in-turn spinner stats line — the ONLY busy
// chrome claude ≥2.1.173 renders (its self-update removed the esc-to-
// interrupt affordance from generating panes; the 2026-06-11 soak-killer,
// cycles 286/288). The shape "(<dur> · <arrow> <n> tokens" is structural —
// duration, middot, stream-direction arrow (↑ prompt / ↓ response), live
// token counter — and is rendered only while a turn runs, so it cannot
// false-match an idle answer the way bare spinner words ("Kneading"/
// "Inferring") could. The duration span is a digit-leading [\d hms]+ run so
// every format a turn passes through matches — "4s", "44s", "12m 34s",
// "1h 5m" (a miss on the hour shapes would re-open the stall wound exactly
// for the longest turns).
var busySpinnerStatsRE = regexp.MustCompile(`\(\s*\d[\d hms]*·\s*[↑↓]\s*[\d.,]+k?\s*tokens`)

// PaneBusy reports whether the rendered pane shows the CLI is actively
// generating a turn (vs idle at the prompt). The driver brackets the
// correlation span's idle_reached on a busy→idle transition; the input-box
// MARKER cannot be that signal because it persists during generation for
// claude/agy (and ollama echoes it on the prompt line), so two complementary
// rules cover the real frames:
//
//  1. an interrupt/spinner affordance is present (claude/agy); OR
//  2. the profile's IdlePlaceholder is set AND absent (ollama, whose input
//     placeholder vanishes mid-turn while its "Thinking…" header persists).
//
// A CLI with neither signal in its capture (codex in the committed fixtures)
// always reads not-busy: its span cannot be bracketed (a documented weak-signal
// degradation), but live monitoring is unaffected.
func PaneBusy(rendered string, p PaneProfile) bool {
	clean := stripANSI(rendered)
	// Rule 1 — a live-turn affordance line is present. Projected from the single
	// channel separator (ClassifyLine, ADR-0047) so the busy vocabulary cannot
	// drift from the progress vocabulary that cleanPane reads.
	for _, line := range strings.Split(clean, "\n") {
		if IsAffordanceLine(line) {
			return true
		}
	}
	// Rule 2 — ollama's input placeholder vanishes mid-turn (it has no affordance
	// line); a whole-pane rule, not a per-line classification.
	if p.IdlePlaceholder != "" && !strings.Contains(clean, p.IdlePlaceholder) {
		return true
	}
	return false
}

// PaneHasSubstantiveChange reports whether prev and cur differ once volatile
// chrome is stripped from both (cycle-432 S4: relocated from
// bridge/stopreview.go into panestream, the single home for pane-chrome
// parsing — panestream.SignalCenter's Changed projection folds this in).
func PaneHasSubstantiveChange(prev, cur string) bool {
	return cleanPane(prev) != cleanPane(cur)
}

// cleanPane keeps only the agent CONTENT lines, dropping every chrome/
// affordance line per the single channel separator (ClassifyLine, ADR-0047).
// This is what makes a ticking spinner-stats line (claude `· Schlepping… (Ns ·
// ↑ Nk tokens)`) NOT count as progress — it is the live-turn affordance, the
// same line PaneBusy reads as busy. A genuinely-working agent still progresses
// via its real transcript (tool calls, output); a stalled one whose only delta
// is the clock no longer reads as progress (closes the ticking-clock hole).
func cleanPane(pane string) string {
	var lines []string
	for _, line := range strings.Split(pane, "\n") {
		if IsContentLine(line) {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}

// PaneDelta tracks how much stable content has already been emitted so each
// capture-pane snapshot yields only NEW content lines (the assistant output),
// excluding the volatile bottom UI (input box / footer / spinner).
//
// The cursor is CONTENT-ANCHORED, not a raw positional index: it remembers the
// last content line it emitted and, on the next frame, re-locates that line
// (searching from the bottom) in the new stable region and emits only the lines
// AFTER it. This survives a top-of-buffer shift — when older scrollback or a
// late prelude is prepended, every content line keeps its text but moves to a
// new index. The real `final.txt` frames exhibit exactly this: codex prepends
// the trust-prompt prelude (stable 24→36), agy/ollama prepend a leading blank
// (18→19, 35→36) while the answer text is unchanged. A purely positional cursor
// misreads that downward shift as fresh bottom growth and re-emits the last
// bullet; the content anchor does not.
type PaneDelta struct {
	// emitted is the count of stable lines already emitted (the positional
	// fallback used at prime time and when the anchor cannot be found).
	emitted int
	// anchor is the text of the last content line emitted, or "" if none yet.
	anchor string
	primed bool
}

// Next takes one rendered capture-pane snapshot and the CLI's PaneProfile. It
// returns content lines that are new since the last call.
//
//   - stable region = lines ABOVE the LAST line whose left-trimmed text starts
//     with p.BoundaryMarker (everything at/below that line is the volatile input
//     box + footer + spinner and is never emitted), minus a trailing volatile
//     run.
//   - the FIRST call PRIMES the baseline (records the current stable tail) and
//     returns nil, so pre-existing boot chrome / prior history is NOT emitted —
//     only content that appears AFTER the extractor starts.
//   - subsequent calls re-locate the anchor (last emitted line) from the bottom
//     of the new stable region and return everything after it. A top-of-buffer
//     shift therefore emits nothing (the anchor is found at a new index with no
//     real content after it).
//   - if the anchor is not found (pane scrolled past it, or stable shrank below
//     the baseline), fall back to the positional cursor and re-anchor.
func (d *PaneDelta) Next(rendered string, p PaneProfile) []string {
	stable := stableLines(rendered, p)

	if !d.primed {
		d.primed = true
		d.emitted = len(stable)
		d.anchor = lastOf(stable)
		return nil
	}

	// Content-anchored path: find the last-emitted line (from the bottom) and
	// emit only what follows it.
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

	// Positional fallback (no anchor yet, or anchor scrolled out of the pane).
	if d.emitted > len(stable) {
		// Pane scrolled / stable region shrank — re-anchor, emit nothing.
		d.emitted = len(stable)
		d.anchor = lastOf(stable)
		return nil
	}
	out := append([]string(nil), stable[d.emitted:]...)
	d.emitted = len(stable)
	if last := lastOf(stable); last != "" {
		d.anchor = last
	}
	// NOTE: when last == "" (answer ends in a stable blank line) the anchor
	// stays unchanged — anti-shift is not applied on the next call and the
	// positional cursor drives instead. Acknowledged gap; a content-line that
	// is genuinely blank is rare enough that the positional path is acceptable.
	return out
}

// lastOf returns the last element of s, or "" if s is empty.
func lastOf(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[len(s)-1]
}

// lastIndexOf returns the index of the last element of s equal to want, or -1.
// Searching from the bottom anchors on the most recent occurrence, so a
// repeated line (e.g. an empty content row) anchors on the freshest copy.
func lastIndexOf(s []string, want string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == want {
			return i
		}
	}
	return -1
}

// stableLines renders the snapshot to the stable above-boundary region: ANSI is
// stripped, trailing blank rows are trimmed, everything at/below the LAST line
// matching the boundary is dropped, and a trailing volatile run is trimmed.
// With no marker, the last volatileFallbackRows rows are treated as volatile.
//
// The boundary match is controlled by p.BoundaryExact:
//   - false (default): left-trimmed line starts with BoundaryMarker (HasPrefix).
//     Safe for non-ASCII / unique markers (❯, ›, >>>) and for markers whose
//     input box carries placeholder text (codex "› Summarize recent commits").
//   - true: left+right-trimmed line equals BoundaryMarker exactly. Used for agy
//     whose marker ">" is plain ASCII and can start a markdown blockquote line
//     ("> quoted text") in answer content — exact-match lets ">" mean "empty
//     input box" while "> quoted" is treated as content.
func stableLines(rendered string, p PaneProfile) []string {
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

	isBoundary := func(ln string) bool {
		trimmed := strings.TrimLeft(ln, " \t")
		if p.BoundaryExact {
			return strings.TrimSpace(trimmed) == p.BoundaryMarker
		}
		return strings.HasPrefix(trimmed, p.BoundaryMarker)
	}

	lastMarker := -1
	for i, ln := range lines {
		if isBoundary(ln) {
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
// status lines, footers, and the blank rows interleaved with them) from the
// bottom of the above-boundary region so the stable region ends at the last
// real content line. See the package boundary-rule comment for why this is
// load-bearing.
func trimVolatileTail(lines []string) []string {
	end := len(lines)
	for end > 0 && isVolatileTailRow(lines[end-1]) {
		end--
	}
	return lines[:end]
}

// statusRE is the small union of status/spinner/footer fragments observed in the
// real frames across all four CLIs. A trailing row matching any of these is
// volatile and trimmed. Kept deliberately small and frame-derived rather than a
// per-CLI list (see knowledge-base/research/tmux-live-capture-2026-06-04/).
var statusRE = regexp.MustCompile(
	`esc to interrupt|esc to cancel|Worked for|Thought for|Generating|Inferring|done thinking|\btokens?$|· /|\? for shortcuts|Send a message`,
)

// isVolatileTailRow reports whether a row is part of the volatile zone that hugs
// the input box. A row is volatile iff it is NOT agent CONTENT — i.e. chrome
// (blank, `────` separator, spinner/status/token line) or a live-turn
// affordance. Projected from the single channel separator (ClassifyLine,
// ADR-0047) so the trim vocabulary cannot drift from PaneBusy and cleanPane.
func isVolatileTailRow(line string) bool {
	return ClassifyLine(line) != LayerContent
}
