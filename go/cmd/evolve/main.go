// Package main is the evolve CLI entrypoint.
//
// Phase 1 subcommands: version, doctor, guard, ledger, acs.
// Phase 2: loop, cycle, worktree, phase.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

const usage = `evolve — autonomous improvement loop (Go port)

Usage:
  evolve <command> [arguments]

Commands:
  version    Print build version and exit
  doctor     Probe environment ( doctor probe <tool> [--json] [--quiet] )
  guard      Run a trust-kernel guard ( guard <name> [--evolve-dir DIR] )
              Guards: ship | phase | role | docdelete | quota | chain
  ledger     Verify or tail the ledger ( ledger verify | ledger tail [--n N] )
  acs        Run ACS predicates    ( acs run --cycle N <pkg> [--json=false] )
  phase        Run a single phase in-process; PhaseRequest on stdin,
                PhaseResponse on stdout ( phase <intent|scout|triage|tdd|build|audit|ship|retro> )
  serve-phase  Envelope-framed phase subprocess (phaseproto wire); the binary
                end of phaseproto.SubprocessRunner ( serve-phase <name> )
  cycle      Run one full cycle through the orchestrator ( cycle run --goal-hash X )
  worktree   Manage per-cycle git worktrees ( worktree create|list|cleanup )
  loop       Drive the cycle dispatcher loop ( loop --max-cycles N --budget-usd X )
  ship       Atomic commit + push (native; v11.3.0)
              ( ship [--class cycle|manual|release|trivial] [--dry-run] "<msg>" )

Dispatch helpers (Phase 3a + 3b ports):
  detect-cli                Identify which AI CLI is driving the skill
  detect-nested-claude      Detect nested claude -p execution
  phase-order               List phases from phase-registry.json
  estimate-quota-reset      Predict next quota reset timestamp
  build-invocation-context  Emit subagent bedrock prefix for a role
  resolve-llm               Route phase role → cli + model JSON
  consensus-dispatch        Cross-CLI consensus auditor (env-driven)
  cycle-simulator           No-LLM cycle plumbing simulator
  phase-watchdog            Activity-based stall watchdog
  aggregator                Merge fan-out worker artifacts
  fanout-dispatch           Bounded-concurrency parallel dispatcher
`

// dispatch is the top-level subcommand router. Extracted so tests can
// drive it without invoking os.Exit. Returns the process exit code.
func dispatch(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, version.Get())
		return 0
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return 0
	case "doctor":
		return runDoctor(args[1:], stdin, stdout, stderr)
	case "guard":
		return runGuard(args[1:], stdin, stdout, stderr)
	case "ledger":
		return runLedger(args[1:], stdin, stdout, stderr)
	case "acs":
		return runACS(args[1:], stdin, stdout, stderr)
	case "phase":
		return runPhase(args[1:], stdin, stdout, stderr)
	case "serve-phase":
		return runServePhase(args[1:], stdin, stdout, stderr)
	case "cycle":
		return runCycle(args[1:], stdin, stdout, stderr)
	case "worktree":
		return runWorktree(args[1:], stdin, stdout, stderr)
	case "loop":
		return runLoop(args[1:], stdin, stdout, stderr)
	case "ship":
		return runShipCmd(args[1:], stdin, stdout, stderr)
	case "detect-cli":
		return runDetectCLI(args[1:], stdin, stdout, stderr)
	case "detect-nested-claude":
		return runDetectNested(args[1:], stdin, stdout, stderr)
	case "phase-order":
		return runPhaseOrder(args[1:], stdin, stdout, stderr)
	case "estimate-quota-reset":
		return runQuotaReset(args[1:], stdin, stdout, stderr)
	case "build-invocation-context":
		return runBedrock(args[1:], stdin, stdout, stderr)
	case "resolve-llm":
		return runResolveLLM(args[1:], stdin, stdout, stderr)
	case "consensus-dispatch":
		return runConsensusDispatch(args[1:], stdin, stdout, stderr)
	case "cycle-simulator":
		return runCycleSimulator(args[1:], stdin, stdout, stderr)
	case "phase-watchdog":
		return runPhaseWatchdog(args[1:], stdin, stdout, stderr)
	case "aggregator":
		return runAggregator(args[1:], stdin, stdout, stderr)
	case "fanout-dispatch":
		return runFanoutDispatch(args[1:], stdin, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "evolve: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}

func main() {
	os.Exit(dispatch(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
