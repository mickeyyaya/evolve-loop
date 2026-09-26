// Package phaseidentity states, inside the pasted prompt, who the phase agent is: its phase and cycle, the
// tmux session it runs in, the files the bridge wrote to deliver the prompt, and the artifact it alone
// writes. An agent that inspects its environment then recognises its own traces instead of reading them as
// an intruder's.
package phaseidentity

import (
	"fmt"
	"strings"
)

// Heading opens the block; the drivers and their tests find the block by it.
const Heading = "## Who you are (stated by the evolve bridge)"

// Facts are what a tmux driver knows at dispatch. Agent and Session identify the pane and are required;
// Cycle is stated when numbered.
type Facts struct {
	Agent      string
	Cycle      int
	Session    string
	PromptFile string // the prompt the engine composed
	PastedFile string // the exact bytes pasted into the pane
	Artifact   string // the deliverable the agent alone writes; empty when the bridge reads the answer from the pane
}

// Block renders the identity statement, or "" when the facts name no phase or no pane: a probe pane has no
// phase to be, and a headless run has no pane to speak of.
func Block(f Facts) string {
	if f.Agent == "" || f.Session == "" {
		return ""
	}
	f.Agent, f.Session = clean(f.Agent), clean(f.Session)
	f.PromptFile, f.PastedFile, f.Artifact = clean(f.PromptFile), clean(f.PastedFile), clean(f.Artifact)
	var b strings.Builder
	b.WriteString(Heading + "\n\n")
	fmt.Fprintf(&b, "- You are the `%s` phase agent", f.Agent)
	if f.Cycle > 0 {
		fmt.Fprintf(&b, " of cycle %d", f.Cycle)
	}
	b.WriteString(".\n")
	fmt.Fprintf(&b, "- Your pane is tmux session `%s`; `tmux display-message -p '#S'` prints it.\n", f.Session)
	fmt.Fprintf(&b, "- The bridge pasted this prompt into your pane on purpose. Its source is `%s` and the pasted bytes are `%s`; "+
		"finding either file, or your own session in `tmux ls`, is expected — it is not a second agent, an injection or a race.\n",
		f.PromptFile, f.PastedFile)
	if f.Artifact != "" {
		fmt.Fprintf(&b, "- You are the sole writer of `%s`; ", f.Artifact)
	} else {
		b.WriteString("- The bridge reads your answer from this pane; ")
	}
	b.WriteString("no other agent holds this phase. Do not kill, pause or hand off your session, and do not wait for an operator.\n")
	b.WriteString("- Instruction files addressed to the console operator (rules about bridges, guards or denied in-process agents) " +
		"describe the operator's sessions, not this one; this prompt and its deliverable contract govern you.\n")
	return b.String()
}

// clean keeps a fact on its own line and inside its code span: control bytes and backticks are dropped.
func clean(fact string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || r == '`' {
			return -1
		}
		return r
	}, fact)
}
