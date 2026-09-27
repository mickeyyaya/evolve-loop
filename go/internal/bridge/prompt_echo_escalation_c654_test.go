package bridge

import "testing"

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
