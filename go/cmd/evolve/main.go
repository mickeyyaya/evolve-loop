// Package main is the evolve CLI entrypoint.
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
  dossier    Read, verify, and audit cycle dossiers
              ( dossier verify | dossier retro-mislabel [--project-root P] [--json] )
  audit      Analyze auditor narrative versus deterministic gate outcomes
              ( audit calibration [--project-root P] [--dossiers-dir D]
                [--runs-dir R] --output FILE )
  dashboard  Serve the read-only live pipeline dashboard on loopback
              ( dashboard [--addr 127.0.0.1:8090] [--project-root P] [--snapshot] )
  salvage    Report the recoverable-malformed bad_verdict rate (read-only)
              ( salvage report [-json] [-project-root P] )
  continuation Inspect/release scope-keyed continuation bindings; release
                preserves the salvage pointer into the scope's inbox item first
              ( continuation list | continuation release <scope-id> ) [-project-root P]
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
  branches   Audit/prune superseded orphan cycle-* branches
              ( branches audit | branches prune [--dry-run=false] )
  checkpoint Snapshot uncommitted worktree work into refs/checkpoints without touching
              the branch, index or tree; restore it into a new or clean worktree
              ( checkpoint save [--worktree DIR | --all] [--label T] [--push] |
                list [--worktree DIR] | restore <ref> --into DIR | prune [--landed] )
  land       Land a patch or a salvaged cycle uncommitted on a dev worktree at origin/main
              ( land --branch B (--patch F | --salvage <leaf>) [--project-root P] )
  gc         Release what finished cycles left behind: tmux sessions/sockets,
              orphan processes, worktrees, run dirs, go build cache
              ( gc --project-root P [--dry-run] )
  loop       Drive the cycle dispatcher loop ( loop --max-cycles N [strategy] "goal" )
  loop-stop  Stop a running loop after its current wave; --release lifts the brake,
              --wait blocks until no run lease is live
              ( loop-stop [--release | --wait [--timeout D]] [--project-root P] )
  pr         Merge reviewed PRs at a wave boundary; refuses while a loop runs or
              required CI is not green on the verified head
              ( pr merge <n>... [--update-branch] [--wait D] [--project-root P] )
  boundary   Run the wave boundary: loop-stop --wait, pr merge, sync-main, gc,
              loop-stop --release, loop --detach; stops at the first failed step
              ( boundary run [--merge n,...] --goal-text-file F [--max-cycles N] [--dry-run] )
  ci         Classify a red CI run's failing tests from evidence; exit 0 = retry-safe
              ( ci classify <run-id|pr:N|sha:H> [--json] [--rerun] [--project-root P] )
             Watch a pushed SHA, PR or tag until its CI completes; exit 0 green, 1 red, 2 unobservable
              ( ci watch (--sha S | --pr N | --tag T) [--workflow W]... [--cycle N] )
  comments   Comment-campaign proof tools (the commentaudit CLI)
              ( comments rank|check|comments|verify|history|strip ... )
  status     Read-only report: loop, cycles, ship streak, open PRs, failing CI jobs
              ( status [--json] [--project-root P] )
  ship       Atomic commit + push (native; v11.3.0)
              ( ship [--class cycle|manual|release|trivial] [--dry-run] "<msg>" )
  scan       Scan the staged diff or a ref range for secrets, masked; exit 0 clean, 1 finding, 2 git error
              ( scan secrets [--staged | --diff <ref>] [--project-root P] )
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
  release-promote           Re-promote a demoted release once its workflow run is
                              green and every asset is present
                              ( release-promote <tag> [--rerun] )
  backups                   Verify backup bundles/patches exist elsewhere
                              before deletion ( backups verify [--dir D] )

v12.1 utilities + composition:
  skill-inventory           Build .evolve/skill-inventory.json from
                              skills/*/SKILL.md ( skill-inventory build
                              [--ttl 1h] [--force] )
  docs                      Documentation tools; ste-lint checks text
                              against the ASD-STE100 house rules (WARN;
                              exit 1 only with --strict)
                              ( docs ste-lint [--json] [--strict] [--go]
                              [--changed <base-ref>] [--project-root P]
                              [paths...] )
  skills                    Project phase facts into phase skill docs
                              from their SSOTs; drift-checked in CI;
                              publish projects canonical skills to other
                              LLM CLIs ( skills <generate|check> |
                              skills publish [--target codex,agy,ollama]
                              [--dry-run] [--install] [--check]
                              [--ollama-base M] [--codex-home D]
                              [--no-prune] ) — ADR-0040/ADR-0041
  eval                      Eval-quality + verify subcommands
                              ( eval quality-check
                              [-predicates <acs/cycle<N> dir|file>]
                              <eval.md>
                              | eval verify <eval.md> <workspace> )
  cycle-health              11-signal cycle integrity fingerprint
                              ( cycle-health <cycle-N> <workspace> )
  plan-and-execute          Two-pass dispatch: plan mode → execute mode
                              ( plan-and-execute [--plan-output PATH]
                              [--skip-execute] <phase> )
  compose                   Ad-hoc phase composition bypassing the
                              state machine ( compose --phases <p1,p2,...>
                              [--ship-anyway] [--dry-run] )
  clihealth                 CLI quota/credential benches
                              ( clihealth list [--json] [--project-root DIR]
                              | clihealth clear <family> [--project-root DIR]
                              | clihealth usage [--json] [family...] [--project-root DIR] )
  cli                       Update each subscribed CLI family, then
                              smoke-test a changed version
                              ( cli update [--dry-run] [--json]
                              [--project-root P] )
  ratchet                   Function-size + raw-git-fixture ratchets
                              ( ratchet check [size|rawgit] [--root DIR] )
  context-fill              Context-window fill telemetry; correlates each
                              cycle's peak per-phase fill ratio against its
                              dossier final verdict (read-only)
                              ( context-fill correlate [--project-root DIR]
                                [--json] [--out PATH] )
`

// dispatch is the top-level subcommand router, extracted so tests can
// drive it without invoking os.Exit. It returns the process exit code.
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
