package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mickeyyaya/evolve-loop/go/internal/consensusdispatch"
	"github.com/mickeyyaya/evolve-loop/go/internal/policy"
)

// runConsensusDispatch is the `evolve consensus-dispatch` subcommand. Its
// inputs are env vars, the bash contract; --help is the only flag.
func runConsensusDispatch(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	for _, a := range args {
		switch a {
		case "--help", "-h":
			fmt.Fprintln(stdout, "Usage: evolve consensus-dispatch")
			fmt.Fprintln(stdout, "Inputs (env): CYCLE WORKSPACE_PATH PROFILE_PATH PROMPT_FILE")
			fmt.Fprintln(stdout, "Optional:    policy.json workflow.consensus_audit_enabled=false refuses to dispatch")
			fmt.Fprintln(stdout, "Exit:        0 PASS/WARN  |  1 FAIL  |  2 runtime  |  10 profile")
			return 0
		}
	}
	// envOrCwd absolutizes a relative $EVOLVE_PROJECT_ROOT and falls back to cwd.
	projectRoot := envOrCwd("EVOLVE_PROJECT_ROOT")
	pol, _ := policy.Load(filepath.Join(projectRoot, ".evolve", "policy.json"))
	in := consensusdispatch.Inputs{
		Cycle:           os.Getenv("CYCLE"),
		WorkspacePath:   os.Getenv("WORKSPACE_PATH"),
		ProfilePath:     os.Getenv("PROFILE_PATH"),
		PromptFile:      os.Getenv("PROMPT_FILE"),
		ConsensusEnvOff: !pol.WorkflowConfig().ConsensusAuditEnabled,
	}
	in.AdaptersDir = filepath.Join(projectRoot, "adapters")
	in.DispatchDir = filepath.Join(projectRoot, "legacy", "scripts", "dispatch")

	return consensusdispatch.Run(in, stdout, stderr)
}
