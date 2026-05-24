package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/consensusdispatch"
)

// runConsensusDispatch is the `evolve consensus-dispatch` subcommand. Ports
// legacy/scripts/dispatch/consensus-dispatch.sh. Inputs are env-vars to
// preserve the bash contract; flags are accepted as overrides.
func runConsensusDispatch(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	for _, a := range args {
		switch a {
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve consensus-dispatch")
			fmt.Fprintln(stdout, "Inputs (env): CYCLE WORKSPACE_PATH PROFILE_PATH PROMPT_FILE")
			fmt.Fprintln(stdout, "Optional:    EVOLVE_CONSENSUS_AUDIT=0 refuses to dispatch")
			fmt.Fprintln(stdout, "Exit:        0 PASS/WARN  |  1 FAIL  |  2 runtime  |  10 profile")
			return 0
		}
	}
	in := consensusdispatch.Inputs{
		Cycle:           os.Getenv("CYCLE"),
		WorkspacePath:   os.Getenv("WORKSPACE_PATH"),
		ProfilePath:     os.Getenv("PROFILE_PATH"),
		PromptFile:      os.Getenv("PROMPT_FILE"),
		ConsensusEnvOff: os.Getenv("EVOLVE_CONSENSUS_AUDIT") == "0",
	}
	// Resolve script-relative defaults from the legacy/scripts/ tree.
	projectRoot := os.Getenv("EVOLVE_PROJECT_ROOT")
	if projectRoot == "" {
		projectRoot, _ = os.Getwd()
	}
	in.AdaptersDir = filepath.Join(projectRoot, "adapters")
	in.DispatchDir = filepath.Join(projectRoot, "legacy", "scripts", "dispatch")

	return consensusdispatch.Run(in, stdout, stderr)
}
