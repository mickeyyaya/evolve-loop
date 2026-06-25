// Package main is the evolve CLI entrypoint.
//
// Phase 1 subcommands: version, doctor, guard, ledger, acs.
// Phase 2: loop, cycle, worktree, phase.
package main

import (
	"fmt"
	"io"
	"os"
)

const usage = `evolve — autonomous improvement loop (Go port)

Usage:
  evolve <command> [arguments]

Commands:
  version    Print build version and exit
  doctor     Probe environment ( doctor probe <tool> [--json] [--quiet] )
  setup      Onboarding ( setup detect [--json] | setup complete )
  install    Manual install of evolve-loop agents + the loop skill into
              ~/.claude ( install [--ci] ); --ci validates structure only
  uninstall  Remove the manually-installed agents + loop skill from
              ~/.claude ( uninstall [--ci] ); --ci is a dry-run
  guard      Run a trust-kernel guard ( guard <name> [--evolve-dir DIR] )
              Guards: ship | phase | role | docdelete | quota | chain
  ledger     Verify or tail the ledger ( ledger verify | ledger tail [--n N] )
  dossier    Read and verify cycle dossiers
              ( dossier verify [--project-root P] )
  acs        Run ACS predicates    ( acs run --cycle N <pkg> | acs suite --cycle N )
  names      Guard naming after a rename; scans tracked files for dead tokens
              from .evolve/naming.json ( names check [--project-root P] | names fix )
  phase        Run a single phase in-process; PhaseRequest on stdin,
                PhaseResponse on stdout ( phase <intent|scout|triage|tdd|build|audit|ship|retro> )
  serve-phase  Envelope-framed phase subprocess (phaseproto wire); the binary
                end of phaseproto.SubprocessRunner ( serve-phase <name> )
  cycle      Run one full cycle, or seal an unfinished one
              ( cycle run --goal-hash X | cycle reset [--dry-run] [--force] )
  campaign   Plan and execute dependency-ordered multi-cycle campaigns
              ( campaign study|replan|run )
  worktree   Manage per-cycle git worktrees ( worktree create|list|cleanup )
  gc         Reap orphaned tmux sessions whose creator PID is dead ( gc [--dry-run] )
  loop       Drive the cycle dispatcher loop ( loop --max-cycles N [strategy] "goal" )
  ship       Atomic commit + push (native; v11.3.0)
              ( ship [--class cycle|manual|release|trivial] [--dry-run] "<msg>" )
  bridge     Native-Go multi-CLI agent bridge
              ( bridge launch --cli=NAME ... | bridge probe | bridge version )

Dispatch helpers (Phase 3a + 3b ports):
  detect-cli                Identify which AI CLI is driving the skill
  detect-nested-claude      Detect nested claude -p execution
  phase-order               List phases from phase-registry.json
  routing                   Explain/replay a recorded routing decision (read-only)
  estimate-quota-reset      Predict next quota reset timestamp
  build-invocation-context  Emit subagent bedrock prefix for a role
  resolve-llm               Route phase role → cli + model JSON
  consensus-dispatch        Cross-CLI consensus auditor (env-driven)
  cycle-simulator           No-LLM cycle plumbing simulator
  phase-watchdog            Activity-based stall watchdog
  aggregator                Merge fan-out worker artifacts
  fanout-dispatch           Bounded-concurrency parallel dispatcher
  preflight-environment     Probe host capabilities, emit JSON profile
  phase-observer            Tail stream-json + stall detection + reports
  subagent                  Subagent helpers (cache-prefix, resolve-tier,
                              check-token, check-ctx-advisory,
                              validate-profile, run, dispatch-parallel)
  changelog-gen             Generate Keep-a-Changelog entry from git log
                              ( changelog-gen <from-ref> <to-ref> <version> [--dry-run] )
  version-bump              Atomic version bump across plugin/marketplace/
                              SKILL.md/README.md ( version-bump <version> [--dry-run] )
  marketplace-poll          Post-publish marketplace propagation verifier
                              ( marketplace-poll <version> [--max-wait-s N]
                                [--poll-interval-s N] [--marketplace-dir DIR]
                                [--dry-run] )
  release-preflight         Pre-publish 5-step gate (clean tree, branch,
                              semver bump, recent audit PASS, gate tests)
                              ( release-preflight <version> [--dry-run]
                                [--skip-tests] )
  rollback                  Auto-revert a failed release using a journal
                              ( rollback <journal.json> [--reason "..."]
                                [--dry-run] )
  release                   Self-healing release pipeline orchestrator
                              ( release <version> [--dry-run] [--no-rollback]
                                [--skip-tests] [--require-preflight]
                                [--max-poll-wait-s N] [--from-tag <tag>] )
  prune-ephemeral           TTL retention for .ephemeral/ + dispatch-logs
                              ( prune-ephemeral [--dry-run] [--quiet] )
  postedit-validate         PostToolUse validator (reads payload on stdin)
                              ( postedit-validate )
  inbox-mover               Inbox lifecycle ops (claim/promote/recover-orphans)
                              ( inbox-mover claim <task_id> <cycle>
                              | inbox-mover promote <task_id> <new_state>
                                [<cycle>] [--commit-sha <sha>]
                              | inbox-mover recover-orphans )
  commit-prefix-gate        Conventional-commits prefix vs diff-scope check
                              ( commit-prefix-gate --msg "<msg>"
                                [--repo-dir <path>] [--staged | --diff-ref <ref>]
                                [--manifest <path>] )
  release-consistency       Verify version markers (plugin.json,
                              marketplace.json, SKILL.md, README, CHANGELOG)
                              ( release-consistency [target-version] )

v12.1 utilities + composition:
  skill-inventory           Build .evolve/skill-inventory.json from
                              skills/*/SKILL.md ( skill-inventory build
                              [--ttl 1h] [--force] )
  skills                    Project phase facts into phase skill docs
                              from their SSOTs; drift-checked in CI;
                              publish projects canonical skills to other
                              LLM CLIs ( skills <generate|check> |
                              skills publish [--target codex,agy,ollama]
                              [--dry-run] [--install] [--check]
                              [--ollama-base M] [--codex-home D]
                              [--no-prune] ) — ADR-0040/ADR-0041
  eval                      Eval-quality + verify subcommands
                              ( eval quality-check <eval.md>
                              | eval verify <eval.md> <workspace> )
  cycle-health              11-signal cycle integrity fingerprint
                              ( cycle-health <cycle-N> <workspace> )
  plan-and-execute          Two-pass dispatch: plan mode → execute mode
                              ( plan-and-execute [--plan-output PATH]
                              [--skip-execute] <phase> )
  compose                   Ad-hoc phase composition bypassing the
                              state machine ( compose --phases <p1,p2,...>
                              [--ship-anyway] [--dry-run] )
`

// dispatch is the top-level subcommand router. Extracted so tests can
// drive it without invoking os.Exit. Returns the process exit code.
//
// As of PR-4, the 91-line switch was replaced by a table lookup
// against `commands` defined in registry.go. Adding a subcommand is
// now a one-line registry entry instead of a switch case + import.
func dispatch(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprint(stderr, usage)
		return 2
	}
	if cmd := lookupCommand(args[0]); cmd != nil {
		return cmd.Run(args[1:], stdin, stdout, stderr)
	}
	fmt.Fprintf(stderr, "evolve: unknown command %q\n\n%s", args[0], usage)
	return 2
}

func main() {
	os.Exit(dispatch(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
