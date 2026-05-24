package main

import (
	"fmt"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/internal/posteditvalidate"
)

// runPostEditValidate is `evolve postedit-validate` — reads a PostToolUse
// payload from stdin and runs the by-extension validator. Mirrors
// legacy/scripts/verification/postedit-validate.sh.
//
// Always exits 0 (PostToolUse cannot block); WARN goes to stderr where
// Claude Code surfaces it to the LLM as a reminder.
func runPostEditValidate(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	for _, a := range args {
		switch {
		case a == "--help" || a == "-h":
			fmt.Fprintln(stdout, "Usage: evolve postedit-validate < <PostToolUse-payload-json>")
			fmt.Fprintln(stdout, "Validates the just-edited file by extension: .json/.sh/.py")
			fmt.Fprintln(stdout, "Always exits 0. WARN to stderr is surfaced to the LLM.")
			fmt.Fprintln(stdout, "Bypass: EVOLVE_BYPASS_POSTEDIT_VALIDATE=1")
			return 0
		case len(a) >= 2 && a[:2] == "--":
			// Unknown flag — log once, still exit 0 (PostToolUse can't block).
			fmt.Fprintf(stderr, "[postedit-validate] unknown flag: %s (ignored)\n", a)
			return 0
		}
	}

	payload, _ := io.ReadAll(stdin)
	projectRoot := envOrCwd("EVOLVE_PROJECT_ROOT")
	bypass := os.Getenv("EVOLVE_BYPASS_POSTEDIT_VALIDATE") == "1"

	posteditvalidate.Run(posteditvalidate.Options{
		Payload:     payload,
		ProjectRoot: projectRoot,
		LLMStderr:   stderr,
		Bypass:      bypass,
	})
	return 0
}
