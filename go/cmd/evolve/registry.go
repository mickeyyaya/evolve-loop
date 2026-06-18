// registry.go defines the subcommand table that drives `evolve <cmd>`
// dispatch. Replaces the 91-line switch that previously lived in
// dispatch(), which forced contributors to update two places when
// adding a subcommand (the switch case and the const usage string).
//
// The const usage in main.go remains hand-maintained — auto-generating
// it from Summary fields would lose the multi-line flag detail that
// `evolve help` users rely on. Adding a subcommand still requires two
// edits (this table + the usage block), but the table is now the
// authoritative source for routing.
package main

import (
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

// subcommand is one row in the dispatcher table.
type subcommand struct {
	// Name is the canonical command name as the user types it.
	Name string
	// Aliases are alternate spellings that route to the same Run.
	// `version` aliases include `--version` and `-v` for parity with
	// common CLI conventions.
	Aliases []string
	// Summary is a one-line description; not currently rendered (the
	// detailed const usage in main.go is shown to users), but kept on
	// the struct so future tooling can derive a short listing.
	Summary string
	// Run is the handler with the standard signature used by every
	// existing cmd_*.go file in this package.
	Run func(args []string, stdin io.Reader, stdout, stderr io.Writer) int
}

// commands is the canonical dispatcher table — single source of truth
// for routing. Lookups by Name or any Alias hit the same row.
//
// Order matches the historical const-usage layout in main.go so a
// reader scanning both side-by-side stays oriented.
var commands = []subcommand{
	// Built-in informational commands.
	{Name: "version", Aliases: []string{"--version", "-v"}, Summary: "Print build version", Run: runVersion},
	{Name: "help", Aliases: []string{"--help", "-h"}, Summary: "Show usage", Run: runHelp},

	// Phase 1 + Phase 2 core surface.
	{Name: "doctor", Summary: "Probe environment", Run: runDoctor},
	{Name: "setup", Summary: "Onboarding: detect CLIs, validate per-phase models, mark first-run", Run: runSetup},
	{Name: "guard", Summary: "Run a trust-kernel guard", Run: runGuard},
	{Name: "ledger", Summary: "Verify or tail the ledger", Run: runLedger},
	{Name: "soak-report", Summary: "Render the EVOLVE_PHASE_RECOVERY soak evidence table (read-only)", Run: runSoakReport},
	{Name: "acs", Summary: "Run ACS predicates", Run: runACS},
	{Name: "phase", Summary: "Run a single phase in-process", Run: runPhase},
	{Name: "phases", Summary: "List/validate/scaffold phase definitions (the phase catalog)", Run: runPhases},
	{Name: "serve-phase", Summary: "Envelope-framed phase subprocess", Run: runServePhase},
	{Name: "cycle", Summary: "Run one full cycle", Run: runCycle},
	{Name: "fleet", Summary: "Launch N concurrent cycles (ADR-0049 S6)", Run: runFleet},
	{Name: "worktree", Summary: "Manage per-cycle worktrees", Run: runWorktree},
	{Name: "swarm", Summary: "Inspect/reap swarm worker sessions (ADR-0032)", Run: runSwarm},
	{Name: "loop", Summary: "Drive the dispatcher loop", Run: runLoop},
	{Name: "ship", Summary: "Atomic commit + push", Run: runShipCmd},
	{Name: "commit-gate", Summary: "Pre-commit quality gate (lint + targeted tests + attestation)", Run: runCommitGate},
	{Name: "bridge", Summary: "Native-Go multi-CLI agent bridge (launch|probe)", Run: runBridge},

	// Phase 3a + 3b dispatch helpers (ported from bash).
	{Name: "detect-cli", Summary: "Identify driving AI CLI", Run: runDetectCLI},
	{Name: "detect-nested-claude", Summary: "Detect nested claude -p", Run: runDetectNested},
	{Name: "phase-order", Summary: "List phases from registry", Run: runPhaseOrder},
	{Name: "routing", Summary: "Explain a recorded routing decision (read-only)", Run: runRouting},
	{Name: "estimate-quota-reset", Summary: "Predict quota reset timestamp", Run: runQuotaReset},
	{Name: "build-invocation-context", Summary: "Emit subagent bedrock prefix", Run: runBedrock},
	{Name: "resolve-llm", Summary: "Route phase role → cli + model", Run: runResolveLLM},
	{Name: "consensus-dispatch", Summary: "Cross-CLI consensus auditor", Run: runConsensusDispatch},
	{Name: "cycle-simulator", Summary: "No-LLM cycle plumbing simulator", Run: runCycleSimulator},
	{Name: "phase-watchdog", Summary: "Activity-based stall watchdog", Run: runPhaseWatchdog},
	{Name: "aggregator", Summary: "Merge fan-out worker artifacts", Run: runAggregator},
	{Name: "fanout-dispatch", Summary: "Bounded-concurrency parallel dispatcher", Run: runFanoutDispatch},
	{Name: "preflight-environment", Summary: "Probe host capabilities", Run: runPreflight},
	{Name: "phase-observer", Summary: "Stream-json tail + stall detect", Run: runPhaseObserver},
	{Name: "subagent", Summary: "Subagent helpers", Run: runSubagent},
	{Name: "changelog-gen", Summary: "Generate changelog from git log", Run: runChangelogGen},
	{Name: "version-bump", Summary: "Atomic version bump", Run: runVersionBump},
	{Name: "marketplace-poll", Summary: "Verify marketplace propagation", Run: runMarketplacePoll},
	{Name: "release-preflight", Summary: "Pre-publish 5-step gate", Run: runReleasePreflight},
	{Name: "rollback", Summary: "Auto-revert failed release", Run: runRollback},
	{Name: "release", Aliases: []string{"release-pipeline"}, Summary: "Self-healing release pipeline", Run: runReleasePipeline},
	{Name: "prune-ephemeral", Summary: "TTL retention for .ephemeral/", Run: runPruneEphemeral},
	{Name: "postedit-validate", Summary: "PostToolUse validator", Run: runPostEditValidate},
	{Name: "inbox-mover", Summary: "Inbox lifecycle ops", Run: runInboxMover},
	{Name: "commit-prefix-gate", Summary: "Conventional-commits prefix check", Run: runCommitPrefixGate},
	{Name: "release-consistency", Summary: "Verify version markers", Run: runReleaseConsistency},

	// v12.1 utilities + composition.
	{Name: "skill-inventory", Summary: "Build skill inventory cache", Run: runSkillInventory},
	{Name: "skills", Summary: "Project phase facts into skill docs from SSOT (generate|check); publish skills to other LLM CLIs (publish) — ADR-0040/0041", Run: runSkills},
	{Name: "flags", Summary: "Project the EVOLVE_* flag registry into control-flags.md (generate|check; check exits 2 on drift) — L2 flag SSOT", Run: runFlags},
	{Name: "phase-inventory", Summary: "Build phase inventory cache (the advisor's phase index)", Run: runPhaseInventory},
	{Name: "eval", Summary: "Eval-quality + verify subcommands", Run: runEval},
	{Name: "cycle-health", Summary: "11-signal cycle integrity fingerprint", Run: runCycleHealth},
	{Name: "plan-and-execute", Summary: "Two-pass dispatch: plan → execute", Run: runPlanAndExecute},
	{Name: "compose", Summary: "Ad-hoc phase composition", Run: runCompose},
	{Name: "models", Summary: "Live tier→model catalog: refresh | list", Run: runModels},
}

// lookupCommand returns the subcommand matching name or any of its
// aliases. Linear scan is fine — the table has ~40 entries and
// lookups happen once per process at startup.
func lookupCommand(name string) *subcommand {
	for i := range commands {
		if commands[i].Name == name {
			return &commands[i]
		}
		for _, a := range commands[i].Aliases {
			if a == name {
				return &commands[i]
			}
		}
	}
	return nil
}

// runVersion handles `evolve version` (and `--version` / `-v`). Adapter
// to match the standard subcommand signature.
func runVersion(_ []string, _ io.Reader, stdout, _ io.Writer) int {
	fmt.Fprintln(stdout, version.Get())
	return 0
}

// runHelp handles `evolve help` (and `--help` / `-h`). Prints the
// hand-maintained const usage from main.go.
func runHelp(_ []string, _ io.Reader, stdout, _ io.Writer) int {
	fmt.Fprint(stdout, usage)
	return 0
}
