// Package panetrust is the single trust boundary for pane-derived text
// (ADR-0045 I5). Anything captured from a tmux pane is attacker-influenceable
// agent output (OWASP LLM Top-10: segregate untrusted content): a manipulated
// or compromised agent can print text crafted to steer the supervisor — fake
// verdict sentinels, fake channel breadcrumbs, ANSI tricks. Every path that
// carries pane text toward an LLM prompt, a privileged decision, or a
// persisted ledger MUST traverse this package.
//
// Slice 1 (shipped with I1 telemetry) provides Digest: the neutralized,
// length-capped data block used for quarantined LLM consumption and for the
// interaction ledger (threat S10 — a stored-injection vector one hop removed).
// The full typed-extraction surface (Extract, untrusted framing) ships in the
// I5-full slice.
//
// Leaf constraints: imports stdlib only, so bridge, core, and interaction can
// all depend on it without cycles.
package panetrust

import (
	"regexp"
	"strings"
)

// ansiRE matches the CSI / OSC escape sequences stripped from captured panes.
// Copied verbatim from bridge/tmux.go (the panestream precedent: the leaf
// duplicates the two-alternative regex rather than importing bridge).
var ansiRE = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]|\x1b\\][^\x07]*\x07")

// defangTag is inserted inside a house control marker so the strict parsers
// (phasecontract.ParseVerdictSentinelFull, the channel-breadcrumb correlator)
// can no longer parse it, while a human reading the digest still sees what
// the agent printed and that it was defused.
const defangTag = "[untrusted]"

// markerBreaks defangs the house control markers an agent could spoof to
// steer the supervisor (threat S1) or to poison the persisted ledger one hop
// removed (S10). Each entry: the marker as the strict parser requires it →
// the broken form. Substring replacement (not regex) keeps the neutralizer
// trivially auditable; defanging runs AFTER ANSI stripping so an escape-split
// marker cannot reassemble into a parseable form.
var markerBreaks = [...][2]string{
	// phasecontract sentinel: `<!-- evolve-verdict: {...} -->`.
	{"evolve-verdict:", "evolve-verdict" + defangTag + ":"},
	// ADR-0037 channel breadcrumb: `{"evolve_channel":...,"corr_id":...}`.
	{`"evolve_channel"`, `"evolve_channel` + defangTag + `"`},
}

// Digest returns a neutralized digest of pane text, safe by construction to
// persist or to embed in an LLM prompt (under the caller's untrusted-content
// framing): ANSI/OSC stripped, house markers defanged, capped at maxLines
// from the TAIL (recency beats volume) and maxCols runes per line (rune-safe
// truncation). maxLines <= 0 requests nothing; maxCols <= 0 means no column
// cap. Digest never joins pane text with anything else — no env, no
// templates — so nothing beyond what the agent printed can leak out (S6).
func Digest(pane string, maxLines, maxCols int) string {
	if pane == "" || maxLines <= 0 {
		return ""
	}
	s := ansiRE.ReplaceAllString(pane, "")
	for _, mb := range markerBreaks {
		s = strings.ReplaceAll(s, mb[0], mb[1])
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	if maxCols > 0 {
		for i, ln := range lines {
			lines[i] = truncateRunes(ln, maxCols)
		}
	}
	return strings.Join(lines, "\n")
}

// truncateRunes caps s at max runes without splitting a multibyte sequence.
func truncateRunes(s string, max int) string {
	if len(s) <= max { // fast path: byte length is an upper bound on rune count
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}
