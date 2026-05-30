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
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/registry"
	"github.com/mickeyyaya/evolve-loop/go/internal/prompts"

	// Blank imports drive the init()-time registry.Register calls for
	// every built-in phase. Adding a new phase = new package + import
	// line here; no edit to a dispatch switch (OCP).
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/audit"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/build"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/debugger"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/intent"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/retro"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/scout"
	"github.com/mickeyyaya/evolve-loop/go/internal/phases/ship"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/tdd"
	_ "github.com/mickeyyaya/evolve-loop/go/internal/phases/triage"
)

// Ship and retro are not yet migrated to the registry — they keep
// manual factory wiring here until Phase 2.5 commit 2 follow-up.
func init() {
	registry.Register(string(core.PhaseShip), newShip)
	registry.Register(string(core.PhaseRetro), newRetro)
}

// runPhase implements `evolve phase <name>`.
func runPhase(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintf(stderr, "evolve phase: missing phase name (%s)\n", strings.Join(registry.Names(), "|"))
		return 10
	}
	name := strings.ToLower(args[0])
	factory, ok := registry.For(name)
	if !ok {
		fmt.Fprintf(stderr, "evolve phase: unknown phase %q (known: %s)\n", name, strings.Join(registry.Names(), ", "))
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

// Most phase factories (intent/scout/triage/tdd/build/audit) self-
// register in their package init() and don't appear here. Ship and
// retro keep manual factories because their construction shape differs
// (ship has no bridge/prompts; retro is bridge-dispatching but predates
// the BaseRunner migration).

func newShip(_ core.PhaseRequest) core.PhaseRunner {
	return ship.NewWithDefaultRunner()
}

func newRetro(req core.PhaseRequest) core.PhaseRunner {
	return retro.New(retro.Config{
		Bridge:  bridge.NewDefault(req.ProjectRoot),
		Prompts: newPromptsLoader(req.ProjectRoot),
	})
}
