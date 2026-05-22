// `evolve phase <name>` is the universal phase-subprocess entrypoint.
//
// Protocol (pkg/phaseproto, plan §1 decision #6):
//
//	stdin:  PhaseRequest JSON (one object)
//	stdout: PhaseResponse JSON (one object)
//	stderr: human-readable diagnostics
//	exit:   0 on success, 1 on internal error
//
// The same binary serves as both the in-process runner (called by the
// orchestrator) AND as the per-phase subprocess override via
// EVOLVE_PHASE_<NAME>_BIN. The subprocess override lets third parties
// implement a phase in any language as long as it speaks this protocol.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mickeyyaya/evolve-loop/go/internal/adapters/bridge"
	"github.com/mickeyyaya/evolve-loop/go/internal/core"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/build"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/intent"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/scout"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/tdd"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"
)

// phaseFactories resolves phase name → core.PhaseRunner constructor.
// Indexed for testability; cmd_phase_test patches it in.
var phaseFactories = map[string]func(req core.PhaseRequest) core.PhaseRunner{
	string(core.PhaseIntent):  newIntent,
	string(core.PhaseScout):   newScout,
	string(core.PhaseTriage):  newTriage,
	string(core.PhaseTDD):     newTDD,
	string(core.PhaseBuild):   newBuild,
	string(core.PhaseAudit):   newAudit,
	string(core.PhaseShip):    newShip,
	string(core.PhaseRetro):   newRetro,
}

// runPhase implements `evolve phase <name>`.
func runPhase(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "evolve phase: missing phase name (intent|scout|triage|tdd|build|audit|ship|retro)")
		return 10
	}
	name := strings.ToLower(args[0])
	factory, ok := phaseFactories[name]
	if !ok {
		fmt.Fprintf(stderr, "evolve phase: unknown phase %q\n", name)
		return 10
	}

	var req core.PhaseRequest
	dec := json.NewDecoder(stdin)
	if err := dec.Decode(&req); err != nil {
		fmt.Fprintf(stderr, "evolve phase: parse stdin JSON: %v\n", err)
		return 11
	}

	runner := factory(req)
	resp, err := runner.Run(context.Background(), req)
	if err != nil {
		// Still emit the partial response so the parent can read
		// diagnostics; exit 1 marks the error.
		buf, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Fprintln(stdout, string(buf))
		fmt.Fprintf(stderr, "evolve phase: %s: %v\n", name, err)
		return 1
	}
	buf, mErr := json.MarshalIndent(resp, "", "  ")
	if mErr != nil {
		fmt.Fprintf(stderr, "evolve phase: marshal response: %v\n", mErr)
		return 1
	}
	fmt.Fprintln(stdout, string(buf))
	return 0
}

// newPromptsLoader resolves the prompts source:
//   - $EVOLVE_PROMPTS_DIR if set (dev override)
//   - <project_root> otherwise (loads from agents/, skills/ in the repo)
func newPromptsLoader(projectRoot string) *prompts.Loader {
	if d := os.Getenv("EVOLVE_PROMPTS_DIR"); d != "" {
		return prompts.NewFromDir(d)
	}
	return prompts.NewFromDir(projectRoot)
}

func newIntent(req core.PhaseRequest) core.PhaseRunner {
	return intent.New(intent.Config{
		Bridge:  bridge.NewDefault(req.ProjectRoot),
		Prompts: newPromptsLoader(req.ProjectRoot),
	})
}
func newScout(req core.PhaseRequest) core.PhaseRunner {
	return scout.New(scout.Config{
		Bridge:  bridge.NewDefault(req.ProjectRoot),
		Prompts: newPromptsLoader(req.ProjectRoot),
	})
}
func newTriage(req core.PhaseRequest) core.PhaseRunner {
	return triage.New(triage.Config{
		Bridge:  bridge.NewDefault(req.ProjectRoot),
		Prompts: newPromptsLoader(req.ProjectRoot),
	})
}
func newTDD(req core.PhaseRequest) core.PhaseRunner {
	return tdd.New(tdd.Config{
		Bridge:  bridge.NewDefault(req.ProjectRoot),
		Prompts: newPromptsLoader(req.ProjectRoot),
	})
}
func newBuild(req core.PhaseRequest) core.PhaseRunner {
	return build.New(build.Config{
		Bridge:  bridge.NewDefault(req.ProjectRoot),
		Prompts: newPromptsLoader(req.ProjectRoot),
	})
}
func newAudit(req core.PhaseRequest) core.PhaseRunner {
	return audit.New(audit.Config{
		Bridge:  bridge.NewDefault(req.ProjectRoot),
		Prompts: newPromptsLoader(req.ProjectRoot),
	})
}
func newShip(_ core.PhaseRequest) core.PhaseRunner {
	return ship.NewWithDefaultRunner()
}
func newRetro(req core.PhaseRequest) core.PhaseRunner {
	return retro.New(retro.Config{
		Bridge:  bridge.NewDefault(req.ProjectRoot),
		Prompts: newPromptsLoader(req.ProjectRoot),
	})
}
