package recovery

// strip_c1123_test.go — cycle-1123 RED tests for `fatalpane-strip-agent-content`,
// SECOND call site.
//
// WHAT CYCLE-1117 CLOSED, AND WHAT IT DID NOT. Cycle-1117 routed the bridge's
// stop-review fast-fail (bridge.fatalPaneVerdict, fatalpane.go:71) through an
// agent-content stripper, so an agent EDITING the fatal registry — its diff
// view literally rendering `+ Substr: "There's an issue with the selected
// model"` — is no longer fast-failed on its own edit buffer. It left the
// registry's OTHER consumer raw: core.adviseOnUnclassifiedFailure
// (failure_hook.go:87) still calls det.Detect(report.FinalPane) on the
// unstripped escalation pane. (scout-report.md named this second site
// "fatalpane_persistence.go:73"; no such file exists in the tree — the second
// raw Detect consumer is failure_hook.go:87. Same defect class, real file.)
//
// THE DEFECT IS INVERTED, NOT MILDER. In the bridge the raw read causes a false
// KILL; in the hook it causes a false SKIP. "Pane already classified" is the
// deterministic-first short-circuit that suppresses the C3 LLM consultation, so
// an agent-authored diff line quoting a seeded signature makes a GENUINELY
// NOVEL fatal pane look known: no advisor call, no promotion, and every future
// occurrence of that novel wedge burns the ~20 min maxExtends backstop again.
// ADR-0044's whole learning loop is silently disabled by the agent's own text.
//
// THE CONTRACT THESE TESTS PIN (production API Builder must create — RED today
// is a compile failure, which is the correct RED for a missing API):
//
//	recovery: func StripAgentContent(pane, injectedPrompt string, protected []string) string
//	    in go/internal/recovery/strip.go, whose ONLY import is "strings"
//	    (go/acs/cycle1123 mutates this function by overlay; an extra import
//	    would make the mutant fail to compile and the mutation predicates
//	    would report a build error instead of a verdict).
//
// It is the SINGLE source of the fatal-pane pane treatment, exported from the
// package that OWNS the registry, so both consumers strip identically:
// bridge.strippedForFatalPaneScan must DELEGATE to it (it may not keep a second
// copy of the rules — see the acs mutation predicates), and failure_hook.go
// must call it before Detect. Semantics are cycle-1117's, verbatim:
//
//	D1 — matched lines are BLANKED IN PLACE (content -> ""), never deleted, so
//	     no survivor loses its leading "\n" and the four newline-ANCHORED
//	     dead-shell seeds ("\nquote>" &c., cycle-274/277) keep matching.
//	D2 — a line carrying any entry of protected is left untouched by the
//	     prompt-echo half, so a prompt QUOTING a seed can never silence that
//	     seed's real banner (cycle-641/642). The diff half is deliberately NOT
//	     protect-listed: diff-prefixing is proof of agent authorship
//	     (cycle-314), and suppressing agent-authored seed text is the point.
//	Empty prompt strips no echoes (fail-open: never suppress a genuine signal
//	on missing context). Nil/blank protected entries protect nothing.
//
// DO NOT MODIFY THESE TESTS to make them pass — they are the acceptance
// criteria. The function name, parameter names and file path above are part of
// the contract (the acs overlay mutants are Go source compiled against them).

import (
	"strings"
	"testing"
)

const (
	// c1123ModelSeed is a literal-English seed: exactly the shape an agent
	// quotes in a diff, a note, or a prompt.
	c1123ModelSeed = "There's an issue with the selected model"
	// c1123AnchoredSeed is newline-anchored; the anchor IS the defence
	// against a bare-word false positive, and is what a delete-based strip
	// destroys (D1).
	c1123AnchoredSeed = "\nquote>"
)

// TestC1123_StripAgentContentBlanksAgentDiffLines is the core AC1 behaviour:
// agent-authored diff content carrying a seeded signature must not survive into
// the detector's view, and the pane's line POSITIONS must be preserved while
// doing it.
func TestC1123_StripAgentContentBlanksAgentDiffLines(t *testing.T) {
	pane := "⏺ Editing go/internal/recovery/detector.go\n" +
		"    72 +\t\t\tSubstr: \"" + c1123ModelSeed + "\",\n" +
		"    73 +\t\t\tCause:  CauseModelInvalid,\n" +
		"⏺ Working… (esc to interrupt)"

	got := StripAgentContent(pane, "", SeedDetector().Signatures())

	if strings.Contains(got, c1123ModelSeed) {
		t.Errorf("agent-authored diff line survived the strip — the detector would classify the agent's own edit buffer as a fatal pane\nstripped:\n%s", got)
	}
	if _, _, ok := SeedDetector().Detect(got); ok {
		t.Errorf("Detect fires on a pane whose only signature came from agent diff content\nstripped:\n%s", got)
	}
	// D1: blank-in-place, never delete. Line count is the observable.
	if want, have := strings.Count(pane, "\n"), strings.Count(got, "\n"); have != want {
		t.Errorf("line count changed: %d newlines, want %d — lines were DELETED, not blanked; every survivor below the strip lost its position and the newline-anchored seeds stop matching (D1)", have, want)
	}
	// Non-diff chrome passes through untouched.
	if !strings.Contains(got, "Working… (esc to interrupt)") {
		t.Errorf("CLI chrome was stripped — only agent-authored content may be removed\nstripped:\n%s", got)
	}
}

// TestC1123_StripAgentContentPreservesNewlineAnchor is D1's discriminating case:
// a diff line directly ABOVE a continuation prompt. Blanked in place, the
// survivor keeps its leading "\n" and "\nquote>" still matches; deleted, it
// becomes line one and the dead-shell fast-fail silently reverts (cycle-274).
func TestC1123_StripAgentContentPreservesNewlineAnchor(t *testing.T) {
	pane := "    41 +\tpane := captureTail(sess)\nquote>"

	got := StripAgentContent(pane, "", SeedDetector().Signatures())

	if !strings.Contains(got, c1123AnchoredSeed) {
		t.Fatalf("the newline anchor of %q was destroyed by the strip — a real zsh continuation prompt would no longer be classified (D1)\nstripped:%q", c1123AnchoredSeed, got)
	}
	cause, _, ok := SeedDetector().Detect(got)
	if !ok || cause != CauseDeadShell {
		t.Errorf("Detect(stripped) = (%q, %v), want a dead_shell match — a genuinely wedged shell must still be classified after stripping", cause, ok)
	}
}

// TestC1123_StripAgentContentProtectsSeededSignatureFromEchoStrip is D2: the
// prompt for a detector-hardening cycle (this one included) quotes the seeds
// verbatim, and echo-stripping is substring-keyed. Without the protect-list the
// CLI's REAL banner becomes indistinguishable from an echo and the cycle-262
// fast-fail goes silent. The second half proves the protect-list is what saves
// it — not some incidental property of the input.
func TestC1123_StripAgentContentProtectsSeededSignatureFromEchoStrip(t *testing.T) {
	banner := "⏺ " + c1123ModelSeed + " (auto). It may not exist."
	pane := "boot\n" + banner + "\ntail"
	prompt := "Your task: strip agent content before matching. " + banner + " is a seeded signature."

	protected := StripAgentContent(pane, prompt, SeedDetector().Signatures())
	if !strings.Contains(protected, c1123ModelSeed) {
		t.Errorf("a line carrying a SEEDED signature was echo-stripped because the prompt quoted it — a prompt mentioning the banner now silences the banner (D2)\nstripped:\n%s", protected)
	}
	if _, _, ok := SeedDetector().Detect(protected); !ok {
		t.Error("Detect no longer fires on the real CLI banner after echo-stripping against a prompt that quotes it (D2)")
	}

	// Same pane, same prompt, EMPTY protect-list: the echo half must remove it.
	// If this line survives too, the test above proves nothing about protection.
	unprotected := StripAgentContent(pane, prompt, nil)
	if strings.Contains(unprotected, c1123ModelSeed) {
		t.Error("with no protect-list the prompt-echo half did not strip a verbatim echoed line — the protect-list assertion above is therefore vacuous (the echo half may be inert)")
	}
}

// TestC1123_StripAgentContentEdgeCases pins the fail-open and boundary
// behaviour every caller relies on: the hook passes an empty prompt, and a nil
// detector yields a nil protect-list.
func TestC1123_StripAgentContentEdgeCases(t *testing.T) {
	echoed := "boot\nsome ordinary agent sentence\ntail"
	if got := StripAgentContent(echoed, "", nil); got != echoed {
		t.Errorf("empty prompt must strip no echoes (fail-open); got:\n%s", got)
	}
	if got := StripAgentContent(echoed, "   \n\t ", nil); got != echoed {
		t.Errorf("whitespace-only prompt must strip no echoes (fail-open); got:\n%s", got)
	}
	if got := StripAgentContent("", "prompt", []string{""}); got != "" {
		t.Errorf("empty pane must yield an empty pane; got %q", got)
	}
	// A blank protected entry is contained in every line — honouring it would
	// protect the whole pane and defeat the strip entirely.
	blankProtected := StripAgentContent("    9 +\t"+c1123ModelSeed, "", []string{"", "   "})
	if strings.Contains(blankProtected, c1123ModelSeed) {
		t.Errorf("a blank protect-list entry suppressed the diff strip — blank entries must be ignored; got %q", blankProtected)
	}
}
