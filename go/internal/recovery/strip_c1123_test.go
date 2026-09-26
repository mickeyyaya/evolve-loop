package recovery

import (
	"strings"
	"testing"
)

const (
	// c1123ModelSeed is an English-sentence seed, the shape an agent quotes.
	c1123ModelSeed = "There's an issue with the selected model"
	// c1123AnchoredSeed is newline-anchored, which a delete-based strip would break.
	c1123AnchoredSeed = "\nquote>"
)

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
	// Line count is the observable for blank-in-place.
	if want, have := strings.Count(pane, "\n"), strings.Count(got, "\n"); have != want {
		t.Errorf("line count changed: %d newlines, want %d — lines were DELETED, not blanked; every survivor below the strip lost its position and the newline-anchored seeds stop matching (D1)", have, want)
	}
	if !strings.Contains(got, "Working… (esc to interrupt)") {
		t.Errorf("CLI chrome was stripped — only agent-authored content may be removed\nstripped:\n%s", got)
	}
}

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

	unprotected := StripAgentContent(pane, prompt, nil)
	if strings.Contains(unprotected, c1123ModelSeed) {
		t.Error("with no protect-list the prompt-echo half did not strip a verbatim echoed line — the protect-list assertion above is therefore vacuous (the echo half may be inert)")
	}
}

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
	blankProtected := StripAgentContent("    9 +\t"+c1123ModelSeed, "", []string{"", "   "})
	if strings.Contains(blankProtected, c1123ModelSeed) {
		t.Errorf("a blank protect-list entry suppressed the diff strip — blank entries must be ignored; got %q", blankProtected)
	}
}
