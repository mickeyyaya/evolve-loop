package main

import (
	"fmt"
	"io"

	"github.com/mickeyyaya/evolve-loop/go/internal/cli/guardcmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/opscmd"
	"github.com/mickeyyaya/evolve-loop/go/internal/cli/phasecmd"
	"github.com/mickeyyaya/evolve-loop/go/pkg/version"
)

// subcommand is one row in the dispatcher table.
type subcommand struct {
	// Name is the canonical command name as the user types it.
	Name string
	// Aliases are alternate spellings that route to the same Run.
	Aliases []string
	// Summary is a one-line description, not currently rendered, but
	// kept on the struct so future tooling can derive a short listing.
	Summary string
	// Run is the handler with the standard signature used by every
	// existing cmd_*.go file in this package.
	Run func(args []string, stdin io.Reader, stdout, stderr io.Writer) int
}

// commands is the canonical dispatcher table, single source of truth for
// routing. Its order matches main.go's usage listing so a reader scanning
// both stays oriented.
var commands = []subcommand{
	{Name: "version", Aliases: []string{"--version", "-v"}, Summary: "Print build version", Run: runVersion},
	{Name: "help", Aliases: []string{"--help", "-h"}, Summary: "Show usage", Run: runHelp},

	{Name: "doctor", Summary: "Probe environment", Run: opscmd.RunDoctor},
	{Name: "console-lease", Summary: "Time-bounded operator lease for runtime-tree paths (ADR-0080 S4)", Run: opscmd.RunConsoleLease},
	{Name: "setup", Summary: "Onboarding: detect CLIs, validate per-phase models, mark first-run", Run: runSetup},
	{Name: "install", Summary: "Manual install of agents + loop skill into ~/.claude (install [--ci])", Run: runInstall},
	{Name: "uninstall", Summary: "Remove manually-installed agents + loop skill from ~/.claude (uninstall [--ci])", Run: runUninstall},
	{Name: "guard", Summary: "Run a trust-kernel guard", Run: guardcmd.RunGuard},
	{Name: "ledger", Summary: "Verify or tail the ledger", Run: runLedger},
	{Name: "dossier", Summary: "Read and verify cycle dossiers (dossier verify)", Run: runDossier},
	{Name: "audit", Summary: "Analyze auditor calibration evidence (audit calibration)", Run: runAudit},
	{Name: "salvage", Summary: "Report the recoverable-malformed bad_verdict rate from the baseline sidecar (salvage report [-json])", Run: runSalvage},
	{Name: "soak-report", Summary: "Render the EVOLVE_PHASE_RECOVERY soak evidence table (read-only)", Run: runSoakReport},
	{Name: "dashboard", Summary: "Serve the read-only live pipeline dashboard (dashboard [--addr A] [--project-root P] [--snapshot])", Run: runDashboard},
	{Name: "names", Summary: "Guard naming after a rename: names check (scan) | names fix (rewrite dead tokens)", Run: runNames},
	{Name: "acs", Summary: "Run ACS predicates", Run: runACS},
	{Name: "apicover", Summary: "Measure public-API coverage (apicover [-cover f] [-require-doc] [-enforce] <pkgdir>...)", Run: runApicover},
	{Name: "inbox", Summary: "The backlog: batches (the grouping triage consumes) | list | show | add | edit | verify | withdraw | route-console | route-lane | consume | quarantine | ack-fingerprint (evolve inbox --help)", Run: runInbox},
	{Name: "phase", Summary: "Run a single phase in-process", Run: phasecmd.RunPhase},
	{Name: "phases", Summary: "List/validate/scaffold phase definitions (the phase catalog)", Run: phasecmd.RunPhases},
	{Name: "serve-phase", Summary: "Envelope-framed phase subprocess", Run: phasecmd.RunServePhase},
	{Name: "cycle", Summary: "Run one full cycle", Run: runCycle},
	{Name: "fleet", Summary: "Launch N concurrent cycles (ADR-0049 S6)", Run: runFleet},
	{Name: "campaign", Summary: "Multi-cycle campaign planner (study|replan|run|status)", Run: runCampaign},
	{Name: "worktree", Summary: "Manage per-cycle worktrees", Run: runWorktree},
	{Name: "branches", Summary: "Audit/prune superseded orphan cycle-* branches (branches audit|prune)", Run: runBranches},
	{Name: "continuation", Summary: "Inspect/release scope-keyed continuation bindings (continuation list | continuation release <scope-id>)", Run: runContinuation},
	{Name: "carryover", Summary: "Apply a reviewed keep/drop/cluster decisions file to state.json:carryoverTodos via the sanctioned locked RMW path (carryover apply-decisions)", Run: runCarryover},
	{Name: "swarm", Summary: "Inspect/reap swarm worker sessions (ADR-0032)", Run: runSwarm},
	{Name: "gc", Summary: "Release what finished cycles left behind: tmux sessions/sockets, orphan processes, worktrees, run dirs, go build cache (gc --project-root <dir> [--dry-run])", Run: runGC},
	{Name: "loop", Summary: "Drive the dispatcher loop", Run: runLoop},
	{Name: "loop-stop", Summary: "Stop a running loop after its current wave: engage the .evolve/loop-stop brake (loop-stop [--release] [--project-root P])", Run: runLoopStop},
	{Name: "status", Summary: "Read-only report: loop, cycles, ship streak, open PRs with check state, failing CI jobs on main (status [--json] [--project-root P])", Run: runStatus},
	{Name: "ship", Summary: "Atomic commit + push", Run: runShipCmd},
	{Name: "reset-sha", Summary: "Re-pin the ship-gate binary SHA to the running binary (provenance-gated; --operator to override)", Run: runResetSHA},
	{Name: "sync-main", Summary: "Reconcile a locally-diverged main with origin via merge only (never rebase/force-push/push); refuses on live lease or dirty tree", Run: runSyncMain},
	{Name: "commit-gate", Summary: "Pre-commit quality gate (lint + targeted tests + attestation)", Run: guardcmd.RunCommitGate},
	{Name: "bridge", Summary: "Native-Go multi-CLI agent bridge (launch|probe)", Run: runBridge},

	{Name: "detect-cli", Summary: "Identify driving AI CLI", Run: runDetectCLI},
	{Name: "detect-nested-claude", Summary: "Detect nested claude -p", Run: runDetectNested},
	{Name: "phase-order", Summary: "List phases from registry", Run: phasecmd.RunPhaseOrder},
	{Name: "routing", Summary: "Explain a recorded routing decision (read-only)", Run: runRouting},
	{Name: "estimate-quota-reset", Summary: "Predict quota reset timestamp", Run: runQuotaReset},
	{Name: "build-invocation-context", Summary: "Emit subagent bedrock prefix", Run: runBedrock},
	{Name: "resolve-llm", Summary: "Route phase role → cli + model", Run: runResolveLLM},
	{Name: "consensus-dispatch", Summary: "Cross-CLI consensus auditor", Run: runConsensusDispatch},
	{Name: "cycle-simulator", Summary: "No-LLM cycle plumbing simulator", Run: runCycleSimulator},
	{Name: "phase-watchdog", Summary: "Activity-based stall watchdog", Run: phasecmd.RunPhaseWatchdog},
	{Name: "aggregator", Summary: "Merge fan-out worker artifacts", Run: runAggregator},
	{Name: "fanout-dispatch", Summary: "Bounded-concurrency parallel dispatcher", Run: runFanoutDispatch},
	{Name: "preflight-environment", Summary: "Probe host capabilities", Run: guardcmd.RunPreflight},
	{Name: "phase-observer", Summary: "Stream-json tail + stall detect", Run: phasecmd.RunPhaseObserver},
	{Name: "subagent", Summary: "Subagent helpers", Run: runSubagent},
	{Name: "changelog-gen", Summary: "Generate changelog from git log", Run: opscmd.RunChangelogGen},
	{Name: "version-bump", Summary: "Atomic version bump", Run: opscmd.RunVersionBump},
	{Name: "marketplace-poll", Summary: "Verify marketplace propagation", Run: opscmd.RunMarketplacePoll},
	{Name: "release-preflight", Summary: "Pre-publish 5-step gate", Run: opscmd.RunReleasePreflight},
	{Name: "rollback", Summary: "Auto-revert failed release", Run: opscmd.RunRollback},
	{Name: "release", Aliases: []string{"release-pipeline"}, Summary: "Self-healing release pipeline", Run: opscmd.RunReleasePipeline},
	{Name: "prune-ephemeral", Summary: "TTL retention for .ephemeral/", Run: runPruneEphemeral},
	{Name: "postedit-validate", Summary: "PostToolUse validator", Run: guardcmd.RunPostEditValidate},
	{Name: "inbox-mover", Summary: "Inbox lifecycle ops", Run: runInboxMover},
	{Name: "commit-prefix-gate", Summary: "Conventional-commits prefix check", Run: guardcmd.RunCommitPrefixGate},
	{Name: "release-consistency", Summary: "Verify version markers", Run: opscmd.RunReleaseConsistency},
	{Name: "release-verify-clis", Summary: "Verify the release installs + performs for every LLM CLI", Run: runReleaseVerifyCLIs},
	{Name: "release-verify-binaries", Summary: "Verify every prebuilt binary + checksums is published on a release tag", Run: runReleaseVerifyBinaries},

	{Name: "skill-inventory", Summary: "Build skill inventory cache", Run: runSkillInventory},
	{Name: "skills", Summary: "Project phase facts into skill docs from SSOT (generate|check); publish skills to other LLM CLIs (publish) — ADR-0040/0041", Run: runSkills},
	{Name: "flags", Summary: "Project the EVOLVE_* flag registry into control-flags.md (generate|check; check exits 2 on drift) — L2 flag SSOT", Run: runFlags},
	{Name: "signals", Summary: "Signal Center (ADR-0101): `signals codes generate|check` projects the code registry into signal-codes.md (check exits 2 on drift)", Run: runSignals},
	{Name: "phase-inventory", Summary: "Build phase inventory cache (the advisor's phase index)", Run: phasecmd.RunPhaseInventory},
	{Name: "eval", Summary: "Eval-quality + verify subcommands", Run: guardcmd.RunEval},
	{Name: "solution", Summary: "Document deliverable contract (ADR-0099): check <solutions/slug> [--project-root DIR] — the same engine as the build floor and the audit gate", Run: runSolution},
	{Name: "cycle-health", Summary: "11-signal cycle integrity fingerprint", Run: runCycleHealth},
	{Name: "selfcheck", Summary: "Builder pre-flight: the build handoff floor checks in-session ( selfcheck build [--worktree DIR] )", Run: runSelfcheck},
	{Name: "plan-and-execute", Summary: "Two-pass dispatch: plan → execute", Run: runPlanAndExecute},
	{Name: "compose", Summary: "Ad-hoc phase composition", Run: runCompose},
	{Name: "models", Summary: "Model catalog and attempt performance: refresh | list | performance", Run: runModels},
	{Name: "tokens", Summary: "Token-usage telemetry: report [--last N] (ranked per-phase consumers)", Run: runTokens},
	{Name: "lessons", Summary: "Lesson analytics: recurrence (deterministic recurrence ledger, patterns by count + fix status)", Run: runLessons},
	{Name: "reachability", Summary: "Import-cycle-safety probe for structural test pins (reachability check-pin)", Run: runReachability},
	{Name: "clihealth", Aliases: []string{"cli-health"}, Summary: "CLI quota/credential benches: list [--json] | clear <family> [--project-root DIR]", Run: runClihealth},
	{Name: "ratchet", Summary: "Function-size + raw-git-fixture ratchets over a module: check [size|rawgit] [--root DIR]", Run: runRatchet},
	{Name: "context-fill", Summary: "Context-window fill telemetry: correlate (peak fill band vs cycle final verdict)", Run: runContextFill},
}

// lookupCommand returns the subcommand matching name or any alias. A linear
// scan is fine: the table has ~40 entries and lookups happen once at startup.
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

func runVersion(_ []string, _ io.Reader, stdout, _ io.Writer) int {
	fmt.Fprintln(stdout, version.Get())
	return 0
}

func runHelp(_ []string, _ io.Reader, stdout, _ io.Writer) int {
	fmt.Fprint(stdout, usage)
	return 0
}
