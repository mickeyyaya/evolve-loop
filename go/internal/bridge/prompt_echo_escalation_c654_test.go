package bridge

// RED regression test for cycle-654 top_n task `infra-classifier-echo-veto`
// at the ESCALATION / agy-tmux auto-responder layer (lesson cycle-641
// preventiveAction #4, cycle-642 defect D2): the exhaustion/escalation matcher
// MUST NOT fire on captured pane text that is a verbatim echo of the injected
// prompt/instruction body. decideAutoRespond's signature is pinned (autorespond.go),
// so the fix pre-strips echoed prompt lines from the pane in the caller — a
// stripPromptEchoLines helper mirroring the existing stripAgentDiffLines. This
// test drives that helper and asserts on matchExhausted over its output.
//
// RED today: stripPromptEchoLines does not exist (compile failure). GREEN once
// Builder adds it (drop pane lines that appear verbatim in the injected prompt)
// and wires it into the tick pane-cleaning ahead of the exhaustion / escalate scan.
//
// Ported (renamed C653→C654) from the fix-of-record RED suite preserved in
// .evolve/worktrees/cycle-21f9f7ae-653; do not author duplicate C653 copies.

import "testing"

// TestC654_004_EchoedExhaustionStrippedGenuineSurvives — prompt-echo exclusion
// for the exhaustion matcher. An echoed Deliverable-Contract instruction line
// containing the exhaustion phrase is removed (so it no longer matches), while a
// genuine CLI quota banner absent from the prompt survives and still matches.
func TestC654_004_EchoedExhaustionStrippedGenuineSurvives(t *testing.T) {
	const pattern = `(?i)reached your usage limit`

	// The agent's OWN echoed instructions: identical line present in the prompt.
	const promptEcho = "Deliverable-Contract: If you have reached your usage limit, stop and hand off."
	echoedPane := "thinking...\n" + promptEcho + "\nwriting report..."
	stripped := stripPromptEchoLines(echoedPane, "Instructions.\n"+promptEcho+"\nProceed.")
	if matchExhausted(pattern, stripped) {
		t.Errorf("echoed prompt exhaustion text still matches after strip: %q", stripped)
	}

	// A real CLI quota wall banner NOT present in the injected prompt must survive.
	const genuineBanner = "You have reached your usage limit. Resets in 4h."
	survived := stripPromptEchoLines(genuineBanner, "Instructions: do the task and report.")
	if !matchExhausted(pattern, survived) {
		t.Errorf("genuine CLI exhaustion banner was wrongly stripped as a prompt echo: %q", survived)
	}
}
