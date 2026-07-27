package recovery

// strip.go — the SINGLE source of the fatal-pane pane treatment, owned by the
// package that owns the registry (ADR-0044).
//
// A captured pane carries two kinds of text: the CLI's own chrome, and content
// the AGENT rendered into it (edit/diff views, patch scrollback, echoes of the
// injected prompt). Only the former is evidence about the CLI's state. Matching
// the fatal registry against the latter means classifying the agent's own edit
// buffer as a fatal pane — and the registry's two consumers fail in OPPOSITE
// directions when that happens:
//
//	bridge.fatalPaneVerdict (C2, fatalpane.go)  — false KILL: an agent editing
//	    the registry, its diff view rendering `+ Substr: "There's an issue with
//	    the selected model"`, is fast-failed on its own work (cycle-1117).
//	core.adviseOnUnclassifiedFailure (C3, failure_hook.go) — false SKIP: the
//	    deterministic-first short-circuit ("pane already classified") reads a
//	    GENUINELY NOVEL wedge as known, so the advise→promote path never runs
//	    and every recurrence burns the ~20 min maxExtends backstop again. The
//	    learning loop switched off by the agent's own text (cycle-1123).
//
// Both call this. Keeping one copy of the rules is the point: cycle-1117 fixed
// C2 alone and the two seams drifted for six cycles.
//
// The rules, and why they are NOT the exhaustion scan's (the cycle-1115 auditor
// rejected reusing bridge.strippedForExhaustionScan on both counts):
//
//	D1 — every matched line is BLANKED IN PLACE (content -> "", the "\n" kept),
//	     never deleted. Four dead-shell seeds are newline-ANCHORED ("\nquote>",
//	     "\nbquote>", "\ndquote>", "\nheredoc>") and the anchor is the whole
//	     defence against a bare-word false positive (cycle-274/277). Deleting
//	     the line ABOVE a continuation prompt strips that survivor's leading
//	     "\n" and silently reverts the cycle-274 fast-fail.
//	D2 — a line carrying any protected signature is exempt from the ECHO half.
//	     Echo-stripping is substring-keyed and two seeds are literal English
//	     sentences, so any prompt QUOTING one (a detector-hardening cycle, a
//	     retro) would make the CLI's real banner indistinguishable from an echo
//	     and silence the cycle-262 fast-fail. Callers pass det.Signatures(), so
//	     the protect-list comes FROM the live registry and can never suppress a
//	     signature the detector is looking for.
//
// The asymmetry is deliberate: protected applies to the echo half ONLY. Diff
// prefixing is proof of agent authorship by construction (cycle-314), and
// suppressing agent-authored seed text is the entire point.
//
// An empty (or whitespace-only) injectedPrompt strips no echoes — fail-open:
// never suppress a genuine signal on missing context. The C3 hook passes ""
// deliberately: D2 neuters the echo half for a fatal-pane scan anyway, so
// plumbing the phase prompt in would add I/O and zero behaviour.
//
// Deliberately depends on "strings" alone: go/acs/cycle1123 mutates
// StripAgentContent's body by `go test -overlay`, and an import used solely by
// this function would make the mutant fail to COMPILE — turning a mutation
// verdict into a build error.

import "strings"

// StripAgentContent removes agent-rendered content — diff/edit lines and
// prompt echoes — from a captured pane so recovery.FatalPaneDetector sees the
// CLI's own chrome. protected (the caller's det.Signatures()) exempts a line
// from the echo half only. See the file header for D1/D2 and the fail-open
// posture.
func StripAgentContent(pane, injectedPrompt string, protected []string) string {
	echo := strings.TrimSpace(injectedPrompt) != ""
	lines := strings.Split(pane, "\n")
	for i, ln := range lines {
		if isAgentDiffLine(ln) {
			lines[i] = "" // D1: blank in place, never delete
			continue
		}
		if !echo {
			continue
		}
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || !strings.Contains(injectedPrompt, trimmed) {
			continue
		}
		if !carriesProtectedSignature(ln, protected) {
			lines[i] = ""
		}
	}
	return strings.Join(lines, "\n")
}

// isAgentDiffLine reports whether a pane line is agent-rendered diff content:
// an optional line number, then a "+"/"-" marker. Both the editor's numbered
// view ("    72 +\t...") and bare unified-diff scrollback ("+\t...") count.
// "+++"/"---" file HEADERS are not content and pass through.
//
// Equivalent to bridge's `^[ \t]*(?:\d+[ \t]+)?[+-]` test, hand-rolled so this
// file's only import stays "strings" (see the file header on the acs mutants).
func isAgentDiffLine(ln string) bool {
	rest := strings.TrimLeft(ln, " \t")
	if strings.HasPrefix(rest, "+++") || strings.HasPrefix(rest, "---") {
		return false
	}
	// An optional line number counts only when separated by whitespace — a bare
	// "72+" is arithmetic in agent prose, not a diff marker.
	if digits := len(rest) - len(strings.TrimLeft(rest, "0123456789")); digits > 0 {
		if after := rest[digits:]; strings.TrimLeft(after, " \t") != after {
			rest = strings.TrimLeft(after, " \t")
		}
	}
	return strings.HasPrefix(rest, "+") || strings.HasPrefix(rest, "-")
}

// carriesProtectedSignature reports whether a pane line contains any protected
// fatal signature. Entries are compared TrimSpace'd because the anchored seeds
// carry a leading newline the pane line does not. Blank entries are skipped: a
// "" signature is contained in every line and would protect the whole pane,
// defeating the strip entirely.
func carriesProtectedSignature(line string, protected []string) bool {
	for _, sig := range protected {
		if s := strings.TrimSpace(sig); s != "" && strings.Contains(line, s) {
			return true
		}
	}
	return false
}
