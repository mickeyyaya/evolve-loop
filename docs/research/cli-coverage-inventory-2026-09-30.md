# Core functions and the `evolve` CLI: coverage inventory (2026-09-30)

This inventory answers one question: which of evolve-loop's core functions can an operator run through the published `evolve` CLI today, and which still need a hand-written procedure, a script or a JSON edit? It exists because of two operator rules:

- "All controls should go through the CLI or API we published, not building the script for specific purpose" (2026-09-28).
- "Add a new request for each core function that can be executed by CLIs" (2026-09-30).

The 50 requests it produced were filed through `evolve inbox add` and merged in PR #750.

## Method

Each row is one core function: something an operator or the loop does as a unit of work. The rows came from four sources: the verbs `go/cmd/evolve` registers, the internal entry points phases and gates expose, the operator procedures in `docs/operations/`, and the procedures the console ran by hand in recent sessions. Every row carries file:line evidence for how the function runs today.

| Coverage | Meaning |
|---|---|
| **FULL** | One `evolve` command does the whole function, with its inputs derived rather than hand-written. |
| **PARTIAL** | A command exists, but the operator must hand-write its input (for example a `PhaseRequest` on stdin), or finish the function with manual steps. |
| **NONE** | No command. The function is a script, a sequence of git and `gh` calls, a JSON edit, or code reachable only from inside another command. |

## Summary

**147 core functions: 64 FULL, 36 PARTIAL, 47 NONE.**

| Area | FULL | PARTIAL | NONE | Total |
|---|---|---|---|---|
| Phases run on their own | 6 | 10 | 11 | 27 |
| Cycle operations | 21 | 7 | 3 | 31 |
| Operator controls | 12 | 5 | 5 | 22 |
| Wave-boundary procedures | 0 | 4 | 5 | 9 |
| Console procedures | 0 | 6 | 7 | 13 |
| Inbox | 5 | 1 | 4 | 10 |
| Release | 6 | 1 | 2 | 9 |
| Maintenance | 7 | 0 | 0 | 7 |
| Observability | 7 | 0 | 0 | 7 |
| Internal entry points | 0 | 2 | 10 | 12 |
| **All** | **64** | **36** | **47** | **147** |

## What the gaps have in common

- **Phases need a hand-written request.** `evolve phase <name>` exists for every registered phase, but it reads a raw `PhaseRequest` from stdin. Nothing derives it from a cycle's `cycle-state.json`, and the orchestrator's floors and transitions do not run.
- **Wave-boundary and console procedures are sequences of git and `gh` calls.** Syncing the plane, landing a patch onto current main, running the full test floor, and watching and classifying CI are all done by hand.
- **Some capabilities are compiled but unreachable.** Several internal packages (for example `ciwatch.Watch`, `secretleakscan.ScanDiff`, `recurrence.BackfillFromLessons`, `dossier.SweepOrphans`) implement a function no command exposes.
- **Protected surfaces.** Rows marked protected touch files only the console may change. Their requests are console-owned, so a loop lane will not take them.

## The requests filed (PR #750)

Each request names the verb to add, where its code would live and acceptance criteria a test can check. Dependencies between them are declared in each item's `deps`.

| Inbox id | Request |
|---|---|
| `cli-backups-verify` | Checking that backup bundles and patches are already on main before deleting them is `git bundle` and merge-base by hand; add `evolve backups verify` |
| `cli-boundary-run` | The wave boundary (stop, merge, sync, gc, launch) is sequenced by hand across evolve, gh and nohup; add `evolve boundary run` |
| `cli-bridge-promote-rule` | Promoting a learned auto-responder rule after checking it against the healthy-pane corpus is unreachable; add `evolve bridge promote-rule` |
| `cli-carryover-list` | carryoverTodos can be reviewed only with jq on state.json; add `evolve carryover list` |
| `cli-ci-classify` | Classifying a CI red as flake or real before a retry means reading `gh run view --log-failed`; add `evolve ci classify` |
| `cli-ci-watch` | Watching CI for a pushed commit, PR or release tag is `gh run watch` by hand, and ciwatch.Watch has no production caller; add `evolve ci watch` |
| `cli-clihealth-list-clear` | A benched CLI family can be inspected or lifted only by reading the cli-health state file or waiting for a launch's canary; add `evolve cli-health list\|clear` |
| `cli-comments` | The comment campaign's proof tools (rank, check, verify) run only as `go run ./cmd/commentaudit`, a separate binary; add `evolve comments` |
| `cli-cycle-carry` | The ADR-0105 carry has no operator view; add `evolve cycle carry --cycle N` to report eligibility and, with --apply, run it |
| `cli-cycle-models` | Which model actually served each phase of a cycle is read from CLI transcripts by hand; add `evolve cycle models --cycle N` |
| `cli-cycle-reaudit` | No verb re-audits one cycle on the current base; the operator resumes and hopes, or hand-builds an audit PhaseRequest |
| `cli-cycle-scope-delta` | No command compares a cycle's touched files with its declared scope, and scopedelta has no caller; add `evolve cycle scope-delta --cycle N` |
| `cli-doctor-live-served-model` | Which model a claude tier alias resolves to is checked with a hand-run `--model sonnet` probe; make `evolve doctor live` report the serving model |
| `cli-doctor-plane-repair` | After the hub moves, worktree links are repaired with raw `git worktree repair`; add `evolve doctor plane --repair` |
| `cli-doctor-versions` | Checking whether an installed LLM CLI lags its latest release is manual; add `evolve doctor versions` |
| `cli-dossier-publish` | Pending dossier closeouts publish only when a loop run exits; after a killed loop they wait for the next launch or raw git, so add `evolve dossier publish` |
| `cli-dossier-sweep` | Orphan dossier pairs left dirty in the tree are committed with raw git, while SweepOrphans exists with no caller; add `evolve dossier sweep` |
| `cli-failures` | Acking a failure fingerprint or pruning failedApproaches happens only as a side effect of launching a loop; add `evolve failures list\|reset\|prune` |
| `cli-floor-run` | The local CI floor (gofmt on CI's Go, vet, integration, e2e, acs-durable, apicover, cover-strict) is make targets by hand; add `evolve floor run` |
| `cli-inbox-edit` | Curating an inbox item (weight, files, deps, a missing id) is a hand edit of its JSON plus a PR; add `evolve inbox edit` |
| `cli-inbox-show-list` | Reading one inbox item, or listing items by route or kind, means opening .evolve/inbox/*.json by hand; add `evolve inbox show` and `evolve inbox list` |
| `cli-inbox-verify` | A console premise re-verification cannot be recorded on an item, so its premise drift never resets; add `evolve inbox verify` |
| `cli-inbox-withdraw` | No verb withdraws a just-filed inbox item, so a mistaken filing is deleted by hand and the ledger records two filings |
| `cli-land-patch` | Landing a patch or a salvaged cycle onto current main is raw git (branch, apply, untar); add `evolve land` |
| `cli-lessons-backfill` | The recurrence ledger cannot be rebuilt from the lesson corpus, and BackfillFromLessons exists with no caller; add `evolve lessons backfill` |
| `cli-loop-detach` | Launching the loop detached and confirming it booted is nohup, a log redirect and a pid check by hand; add `evolve loop --detach` |
| `cli-loop-dry-run-wave-plan` | Which items and lanes the next wave will dispatch is unknown until it launches; make `evolve loop --dry-run` print the wave plan |
| `cli-loop-goal-from-last` | Each wave's goal text is retyped or pasted by hand; add `--goal-file` and `--goal-from-last` to `evolve loop` |
| `cli-loop-preflight-only` | The loop's pre-flight readiness gate runs only at launch; add `evolve loop --preflight-only` to check readiness without starting a batch |
| `cli-loop-resume-target` | `evolve loop --resume` always takes the newest checkpoint; add `--cycle N` and `--from-phase P` so the operator resumes the cycle they mean |
| `cli-loop-status` | `evolve loop status` launches a loop because the word is parsed as goal text; make it a read-only subcommand and refuse reserved words as goals |
| `cli-loop-stop-wait` | `evolve loop-stop` cannot wait for the brake to take effect, so before merging the operator tails the loop log or polls the pid |
| `cli-loop-zero-ship-breaker` | Two consecutive zero-ship waves should halt the loop, but no code counts them: the operator watches every wave and stops the loop by hand |
| `cli-phase-cycle-request` | `evolve phase <name>` and `evolve compose` take only a hand-written PhaseRequest on stdin; add `--cycle N` so a built-in phase reruns from the cycle's own state |
| `cli-phase-spec-phases` | `evolve phase` runs only the nine built-in phases; spec and catalog phases (plan-review, tester, doc-sync, memo) run only inside a cycle |
| `cli-phases-check-provenance` | The ledger provenance check of a cycle's phase artifacts is unreachable; add `evolve phases check-provenance --cycle N` |
| `cli-pr-merge` | Merging reviewed PRs at a boundary is gh checks, update-branch and merge by hand, with no loop-stopped check; add `evolve pr merge` |
| `cli-ratchet-check` | The size and raw-git ratchets run only as `go test` on their packages; add `evolve ratchet check` so a landing runs the same check CI runs |
| `cli-rebuild` | Rebuilding an evolve binary outside the loop's boot refresh is `make -C go build` by hand; add `evolve rebuild` |
| `cli-release-preflight-no-wip` | `evolve release-preflight` lacks the no-WIP-commit check that the /evo:release skill runs with git log; add it to the preflight |
| `cli-release-promote` | Re-promoting an auto-demoted release, or rerunning its failed release workflow, is raw `gh api` and `gh run rerun`; add `evolve release-promote` |
| `cli-routing-plan` | `evolve routing` only explains a recorded decision; add `routing plan --cycle N` to ask the phase advisor for a plan without a cycle |
| `cli-salvage-list` | `evolve salvage` reports only the bad-verdict rate; add `salvage list` to inventory what gc salvaged |
| `cli-scan-secrets` | There is no secret scan of a diff outside a cycle, and secretleakscan.ScanDiff exists with no caller; add `evolve scan secrets` |
| `cli-ship-force-with-lease` | Re-shipping a diverged feature branch needs a raw force push under a guard bypass; add `--force-with-lease` to the manual ship |
| `cli-ship-manual-paths` | `evolve ship --class manual` stages the whole tree; add `--paths` to ship only the named files, and make --dry-run print the staged set |
| `cli-ship-pr` | `evolve ship --class manual` pushes a branch but cannot open its PR; add `--pr` |
| `cli-signals-tail` | Watching a running wave (phases, ship verdicts, halts, Signal Center codes) means grepping each lane's signals.ndjson; add `evolve signals tail` |
| `cli-status` | There is no single status verb, so the operator combines the dashboard snapshot, the dossiers and gh to report cycles, the ship streak, open PRs and failing CI |
| `cli-worktree-dev` | Console dev worktrees are created and removed with raw git; add `evolve worktree create --dev` and `evolve worktree cleanup --dev` |

## Full inventory

### Phases run on their own

| Function | How it runs today | Coverage | Evidence | Gap | Suggested verb | Protected |
|---|---|---|---|---|---|---|
| ACS predicate suite for a cycle | evolve acs suite --cycle N \| acs run | FULL | cmd_acs.go:71-73; writes runs/cycle-N/acs-verdict.json (runtime-reference.md operator commands) | — | — | no |
| build handoff floor pre-flight | evolve selfcheck build [--worktree DIR] | FULL | registry.go selfcheck: the build handoff floor checks in-session | — | — | no |
| document deliverable contract check | evolve solution check <solutions/slug> | FULL | registry.go solution: same engine as the build floor and audit gate (ADR-0099) | — | — | no |
| eval quality / diversity / independent re-verify | evolve eval quality-check\|diversity-check\|verify | FULL | internal/cli/guardcmd/eval.go:30-34 | — | — | no |
| phase deliverable self-check / persona lint | evolve phase verify\|lint | FULL | phasecmd/phase.go:31-36 routes verify/lint; phase_verify.go:22 shares the host gate verifier | — | — | no |
| ship a PASS cycle | evolve ship --class cycle "<msg>" | FULL | cmd_ship.go:25 --class cycle = full audit binding (runtime-reference.md Ship classes); also evolve phase ship | — | — | no |
| ad-hoc phase composition | evolve compose --phases p1,p2 | PARTIAL | cmd_compose.go:18-70 runs registered phases in order; same stdin PhaseRequest; built-ins only | hand-written PhaseRequest JSON on stdin (cycle, workspace, worktree, run_id, goal_hash); no derivation from cycle-state.json; orchestrator floors/transitions not applied; spec phases refused | evolve compose --phases ... --cycle N | no |
| audit phase (run standalone) | evolve phase audit; evolve acs suite --cycle N | PARTIAL | phasecmd/phase.go:38-50; audit registered; acs suite writes acs-verdict.json independently | hand-written PhaseRequest JSON on stdin; no derivation from cycle-state.json; orchestrator floors/transitions and CI-parity host seams not applied | evolve phase <name> --cycle N (derive PhaseRequest from runs/cycle-N/cycle-state.json) | no |
| build phase (run standalone) | evolve phase build; evolve selfcheck build | PARTIAL | phasecmd/phase.go:38-50; build registered; selfcheck build runs the handoff floor in-session | hand-written PhaseRequest JSON on stdin (cycle, workspace, worktree, run_id, goal_hash); no derivation from cycle-state.json; orchestrator floors/transitions not applied; worktree must pre-exist | evolve phase <name> --cycle N (derive PhaseRequest from runs/cycle-N/cycle-state.json) | no |
| debugger phase (run standalone) | evolve phase debugger | PARTIAL | phasecmd/phase.go:38-50; debugger registered (phases/debugger/debugger.go:256) | hand-written PhaseRequest JSON on stdin (cycle, workspace, worktree, run_id, goal_hash); no derivation from cycle-state.json; orchestrator floors/transitions not applied | evolve phase <name> --cycle N (derive PhaseRequest from runs/cycle-N/cycle-state.json) | no |
| intent phase (run standalone) | evolve phase intent | PARTIAL | internal/cli/phasecmd/phase.go:38-50 decodes core.PhaseRequest from stdin; intent is registered | hand-written PhaseRequest JSON on stdin (cycle, workspace, worktree, run_id, goal_hash); no derivation from cycle-state.json; orchestrator floors/transitions not applied | evolve phase <name> --cycle N (derive PhaseRequest from runs/cycle-N/cycle-state.json) | no |
| phase advisor plan for a cycle (routing decision) | evolve routing (explain recorded); resolve-llm | PARTIAL | routing explains/replays a RECORDED decision (read-only); resolve-llm maps role->cli; no fresh advisor plan on demand | cannot ask the advisor to plan a given cycle/goal outside a live cycle | evolve routing plan --cycle N --dry-run | yes |
| retrospective phase (run standalone) | evolve phase retro | PARTIAL | phasecmd/phase.go:38-50; retro registered (phases/retro/retro.go:408) | hand-written PhaseRequest JSON on stdin (cycle, workspace, worktree, run_id, goal_hash); no derivation from cycle-state.json; orchestrator floors/transitions not applied | evolve phase <name> --cycle N (derive PhaseRequest from runs/cycle-N/cycle-state.json) | no |
| scout phase (run standalone) | evolve phase scout | PARTIAL | phasecmd/phase.go:38-50; scout registered (phases/scout/scout.go:192) | hand-written PhaseRequest JSON on stdin (cycle, workspace, worktree, run_id, goal_hash); no derivation from cycle-state.json; orchestrator floors/transitions not applied | evolve phase <name> --cycle N (derive PhaseRequest from runs/cycle-N/cycle-state.json) | no |
| tdd phase (run standalone) | evolve phase tdd | PARTIAL | phasecmd/phase.go:38-50; tdd registered; phase verify tdd --worktree adds the frozen-pin gate | hand-written PhaseRequest JSON on stdin (cycle, workspace, worktree, run_id, goal_hash); no derivation from cycle-state.json; orchestrator floors/transitions not applied | evolve phase <name> --cycle N (derive PhaseRequest from runs/cycle-N/cycle-state.json) | no |
| triage phase (run standalone) | evolve phase triage | PARTIAL | phasecmd/phase.go:38-50; triage registered; host triage-decision derivation only runs inside the cycle | hand-written PhaseRequest JSON on stdin (cycle, workspace, worktree, run_id, goal_hash); no derivation from cycle-state.json; orchestrator floors/transitions not applied | evolve phase <name> --cycle N (derive PhaseRequest from runs/cycle-N/cycle-state.json) | no |
| architecture-design phase (run standalone) | none (only inside cycle run) | NONE | evolve phase architecture-design -> "unknown phase" (registry: audit,build,debugger,intent,retro,scout,ship,tdd,triage); spec phases minted only inside cycle run (cmd_cycle.go:522) | runs only inside evolve cycle run/loop; subagent run bypasses the spec runner contract and verdict; operator must hand-compose a prompt | evolve phase <spec-phase> --cycle N (register spec phases via phaseregistrar in phasecmd) | no |
| build-planner phase (run standalone) | none (only inside cycle run) | NONE | evolve phase build-planner -> "unknown phase" (registry: audit,build,debugger,intent,retro,scout,ship,tdd,triage); spec phases minted only inside cycle run (cmd_cycle.go:522) | runs only inside evolve cycle run/loop; subagent run bypasses the spec runner contract and verdict; operator must hand-compose a prompt | evolve phase <spec-phase> --cycle N (register spec phases via phaseregistrar in phasecmd) | no |
| catalog/user phases (72 in .evolve/phases: bug-reproduction, test-amplification, fault-localization, security-scan...) | none (advisor inserts them inside a cycle) | NONE | evolve phase/compose accept only the 9 registered built-ins; catalog phases are minted by phaseregistrar in cycle run only | runs only inside evolve cycle run/loop; subagent run bypasses the spec runner contract and verdict; operator must hand-compose a prompt | evolve phase <spec-phase> --cycle N (register spec phases via phaseregistrar in phasecmd) | no |
| deliverable recovery rung (ADR-0106 recovery agent) | none | NONE | recovery agent dispatch runs only inside the cycle behind internal/recoveryguard | no standalone recover-deliverable for a failed cycle; operator re-runs the whole cycle | evolve cycle recover --cycle N --phase P | yes |
| doc-sync phase (run standalone) | none (only inside cycle run) | NONE | evolve phase doc-sync -> "unknown phase" (registry: audit,build,debugger,intent,retro,scout,ship,tdd,triage); spec phases minted only inside cycle run (cmd_cycle.go:522) | runs only inside evolve cycle run/loop; subagent run bypasses the spec runner contract and verdict; operator must hand-compose a prompt | evolve phase <spec-phase> --cycle N (register spec phases via phaseregistrar in phasecmd) | no |
| flake-rerun-scan phase (run standalone) | none (only inside cycle run) | NONE | evolve phase flake-rerun-scan -> "unknown phase" (registry: audit,build,debugger,intent,retro,scout,ship,tdd,triage); spec phases minted only inside cycle run (cmd_cycle.go:522) | runs only inside evolve cycle run/loop; subagent run bypasses the spec runner contract and verdict; operator must hand-compose a prompt | evolve phase <spec-phase> --cycle N (register spec phases via phaseregistrar in phasecmd) | no |
| memo phase (run standalone) | none (only inside cycle run) | NONE | evolve phase memo -> unknown phase "memo" (verified on runtime binary c8faa4ec) | runs only inside evolve cycle run/loop; subagent run bypasses the spec runner contract and verdict; operator must hand-compose a prompt | evolve phase <spec-phase> --cycle N (register spec phases via phaseregistrar in phasecmd) | no |
| plan-review phase (run standalone) | none (only inside cycle run) | NONE | evolve phase plan-review -> "unknown phase" (registry: audit,build,debugger,intent,retro,scout,ship,tdd,triage); spec phases minted only inside cycle run (cmd_cycle.go:522) | runs only inside evolve cycle run/loop; subagent run bypasses the spec runner contract and verdict; operator must hand-compose a prompt | evolve phase <spec-phase> --cycle N (register spec phases via phaseregistrar in phasecmd) | no |
| secret-leak-scan phase (run standalone) | none (only inside cycle run) | NONE | evolve phase secret-leak-scan -> "unknown phase" (registry: audit,build,debugger,intent,retro,scout,ship,tdd,triage); spec phases minted only inside cycle run (cmd_cycle.go:522) | runs only inside evolve cycle run/loop; subagent run bypasses the spec runner contract and verdict; operator must hand-compose a prompt | evolve phase <spec-phase> --cycle N (register spec phases via phaseregistrar in phasecmd) | no |
| spec-verify phase (run standalone) | none (only inside cycle run) | NONE | evolve phase spec-verify -> "unknown phase" (registry: audit,build,debugger,intent,retro,scout,ship,tdd,triage); spec phases minted only inside cycle run (cmd_cycle.go:522) | runs only inside evolve cycle run/loop; subagent run bypasses the spec runner contract and verdict; operator must hand-compose a prompt | evolve phase <spec-phase> --cycle N (register spec phases via phaseregistrar in phasecmd) | no |
| tester phase (run standalone) | none (only inside cycle run) | NONE | evolve phase tester -> "unknown phase" (registry: audit,build,debugger,intent,retro,scout,ship,tdd,triage); spec phases minted only inside cycle run (cmd_cycle.go:522) | runs only inside evolve cycle run/loop; subagent run bypasses the spec runner contract and verdict; operator must hand-compose a prompt | evolve phase <spec-phase> --cycle N (register spec phases via phaseregistrar in phasecmd) | no |

### Cycle operations

| Function | How it runs today | Coverage | Evidence | Gap | Suggested verb | Protected |
|---|---|---|---|---|---|---|
| apply carryover keep/drop/cluster decisions | evolve carryover apply-decisions | FULL | cmd_carryover.go:45 (locked RMW on state.json:carryoverTodos) | — | — | no |
| campaign study/replan/run/status | evolve campaign study\|replan\|run\|status | FULL | cmd_campaign.go:44-50 | — | — | no |
| chain batches until the inbox drains | evolve loop --until-inbox-empty | FULL | cmd_loop_args.go:52 | — | — | no |
| complete a stranded lane ship (push-only) | evolve ship --push-only | FULL | cmd_ship.go:26 push an already-committed provenance-verified ahead set after sync-main | — | — | no |
| cycle integrity fingerprint / timing / outputs | evolve cycle-health \| cycle timing\|outputs | FULL | registry.go cycle-health; cmd_cycle.go:74-76 | — | — | no |
| inspect/release continuation bindings | evolve continuation list\|release <scope> | FULL | cmd_continuation.go:23-25; runtime-reference.md:168 | — | — | no |
| ledger verify/seal/tail/anchor/rebaseline | evolve ledger verify\|seal\|tail\|anchor\|rebaseline | FULL | cmd_ledger.go:36-44 (plain verify fails after a seal: inbox ledger-plain-verify-fails-after-seal) | — | — | no |
| list audit failures in retention | evolve guard list-audit-fails | FULL | runtime-reference.md operator commands (read-only) | — | — | no |
| per-cycle worktree create/list/cleanup | evolve worktree create\|list\|cleanup --cycle N | FULL | cmd_worktree.go:43-47 | — | — | no |
| preview resolved loop config | evolve loop --dry-run | FULL | cmd_loop.go:108 exits before maintenance/launch | — | — | no |
| recover orphaned inbox claims | evolve inbox-mover recover-orphans | FULL | main.go usage inbox-mover | — | — | no |
| report bad-verdict salvage rate | evolve salvage report | FULL | cmd_salvage.go:21 (read-only) | — | — | no |
| run N concurrent cycles (fleet) | evolve fleet --count N --goal-hash X [--plan F] | FULL | cmd_fleet.go:50-54 | — | — | no |
| run one full cycle | evolve cycle run --goal-hash X | FULL | cmd_cycle.go:70 run; flags :166-171 | — | — | no |
| run the wave loop | evolve loop --goal-text T --max-cycles N | FULL | cmd_loop_args.go:37-53 | — | — | no |
| seal/reset an unfinished cycle | evolve cycle reset [--dry-run] [--force] | FULL | cmd_cycle.go:86-120 core.SealCycle | — | — | no |
| simulate a cycle without LLM calls | evolve cycle run --simulate \| cycle-simulator | FULL | cmd_cycle.go:170 --simulate; registry cycle-simulator | — | — | no |
| stale cycle worktree cleanup | evolve gc --project-root P | FULL | gc step 3: merged clean dead trees removed, dirty ones salvaged then removed (runtime-reference.md gc); supersedes most of inbox worktree-gc-respects-continuations | — | — | no |
| start fresh past an unfinished cycle | evolve loop --force-fresh | FULL | cmd_loop_args.go:49 (history not sealed) | — | — | no |
| swarm worker inspect/reap | evolve swarm status\|reap\|reap-orphans | FULL | cmd_swarm.go:23-27 | — | — | no |
| verify dossiers / retro mislabels | evolve dossier verify\|retro-mislabel | FULL | cmd_dossier.go:21-23 | — | — | no |
| ack a failure fingerprint / prune failedApproaches | evolve loop --reset --fingerprint fp | PARTIAL | cmd_loop_maintenance.go:20 runs the reset only as part of a loop launch; inbox ack-fingerprint needs an item | ack/prune without launching a batch is impossible | evolve failures reset [--fingerprint fp] \| failures list | no |
| dispatch a specific inbox item to a lane | evolve fleet --plan F (cycle-level only) | PARTIAL | goal-text only reaches LLM prompts; wave lanes come from triage/fleet.Partition (goal-blind) | loop waves cannot be told to work or exclude named items; operator edits goal text and hopes | evolve loop --items id1,id2 [--exclude id] | yes |
| prune expired failedApproaches/carryoverTodos | evolve loop (auto-prune at launch) | PARTIAL | cmd_loop_maintenance.go:25-45 failurelog.PruneExpired* runs only at loop launch | no standalone prune between batches | evolve failures prune [--dry-run] | no |
| re-audit a cycle on the current base | evolve loop --resume (needs-reaudit chain) or hand-built phase audit | PARTIAL | re-audit happens only inside the recovery chain (runtime-reference.md Ship self-healing: needs-reaudit); no verb | no targeted re-audit of cycle N; operator resumes blind or hand-builds a PhaseRequest | evolve cycle reaudit --cycle N | yes |
| resume an unfinished cycle | evolve loop --resume | PARTIAL | cmd_loop_resume.go:48 RunCycleFromPhase on the auto-located most-recent checkpoint; no --cycle/--from-phase | cannot target cycle N or a phase; after a fleet signal stop it resumes an unrelated stale checkpoint | evolve loop --resume --cycle N [--from-phase P] | yes |
| salvage a failed cycle worktree (inventory + land) | evolve gc (salvage step); evolve worktree list | PARTIAL | gc step 3 writes .evolve/operator-salvage/<leaf>/{HEAD,uncommitted.patch,untracked.tgz}; landing is git apply + ship (operating-policy.md:85) | inventory + git apply the patch into a dev worktree + ship manual + PR by hand | evolve salvage list \| salvage apply <leaf> --into <worktree> | no |
| superseded cycle-* branch audit/prune | evolve branches audit\|prune | PARTIAL | cmd_branches.go:22-24 | audit reports live lanes' (commit-less) branches as superseded; operator cross-checks leases by hand | evolve branches audit (skip branches with a live lease) | no |
| carry a passed audit across a fleet rebase (ADR-0105 carry) | none (inside ship/resume only) | NONE | carry runs only in the ship/resume recovery path; no test drives it end to end | no way to check carry eligibility or trigger the carry for cycle N | evolve cycle carry --cycle N [--dry-run] | yes |
| list/review carryoverTodos | none | NONE | cmd_carryover.go has only apply-decisions; dashboard snapshot omits carryover | operator reads .evolve/state.json with jq to review the queue | evolve carryover list [--json] | no |
| publish pending dossier closeouts | none (loop exit only) | NONE | cmd_loop_dossiers.go:30 publishPendingDossiers runs only when a loop run exits | after a crashed/killed loop the closeouts wait for the next launch or raw git | evolve dossier publish [--dry-run] | no |

### Operator controls

| Function | How it runs today | Coverage | Evidence | Gap | Suggested verb | Protected |
|---|---|---|---|---|---|---|
| emergency guard bypass | evolve guard phase\|ship --bypass; ship --bypass-commit-gate | FULL | AGENTS.md invariants 1 and 3 | — | — | no |
| model catalog refresh/list | evolve models refresh\|list | FULL | cmd_models.go:17-24 | — | — | no |
| onboarding and per-phase model pins | evolve setup detect\|recommend\|latest\|apply\|complete | FULL | cmd_setup.go:23-31 | — | — | no |
| operator lease for runtime-tree paths | evolve console-lease <path>... --ttl | FULL | registry.go console-lease (ADR-0080 S4) | — | — | no |
| probe CLI availability / boot / live quota | evolve doctor probe\|boot\|live; bridge probe\|doctor | FULL | opscmd/doctor.go:22-28; doctor_live.go:49 | — | — | no |
| publish skills to other LLM CLIs | evolve skills publish | FULL | runtime-reference.md operator commands | — | — | no |
| re-pin the ship-gate binary SHA | evolve reset-sha [--operator] | FULL | cmd_resetsha.go:23-24 | — | — | no |
| release finished-cycle resources | evolve gc --project-root P [--dry-run] | FULL | runtime-reference.md gc section (7 steps) | — | — | no |
| release the brake | evolve loop-stop --release | FULL | cmd_loop_stop.go:20 | — | — | no |
| report plane layout / loop state location | evolve doctor plane [root] | FULL | opscmd/doctor.go:28 | — | — | no |
| stop the loop after the current wave (brake) | evolve loop-stop | FULL | runtime-reference.md:174; cmd_loop_stop.go | — | — | no |
| sync the plane main with origin (merge only) | evolve sync-main | FULL | runtime-reference.md:176 (doc drift: :183 still tells the operator to run git merge origin/main) | — | — | no |
| loop pre-flight readiness check standalone | none standalone (inside evolve loop) | PARTIAL | looppreflight runs only at loop launch; preflight-environment and doctor boot/live cover pieces | no dry pre-flight without launching | evolve loop --preflight-only | no |
| manual commit with review attestation | evolve commit-gate run + evolve ship --class manual | PARTIAL | /commit skill stages with raw git add (skills/commit/SKILL.md:15-34); ship stages git add -A; reviewer fleet is CLI-agent side | explicit-path staging is raw git (memory rule forbids git add -A) | evolve ship --class manual --paths p1,p2 | yes |
| rebuild the plane binary | none standalone (auto at loop boot/boundary) | PARTIAL | boot/boundary refresh runs make -C go build inside evolve loop (runtime-reference.md:74); console uses make -C go build (workspace-layout.md:38) | fresh worktree / console must run make -C go build by hand | evolve rebuild [--repin] | yes |
| ship-streak tracking (6 consecutive ships) | evolve dashboard --snapshot (ship-rate trend) | PARTIAL | dashboard model has trend + per-cycle verdicts; no consecutive-ship counter | operator counts the streak from dossiers/snapshot | evolve status --streak | no |
| status report (cycles, ships/lanes, open PRs, failing CI jobs) | evolve dashboard --snapshot | PARTIAL | CLAUDE.md Status Reporting; snapshot covers loop/cycles/verdicts only | open PR numbers and failing CI job names need gh | evolve status [--json] (snapshot + PRs + CI) | no |
| disk-space check before launch | none (gc reports disk free after a run) | NONE | no preflight halts on low free space (inbox disk-space-preflight); gc prints disk free A->B | operator checks df by hand | preflight check in looppreflight (halt below threshold) | yes |
| inspect/lift a CLI family bench | none | NONE | clihealth.Store.Clear (clihealth.go:230) is called only by the loop canary (cli_health_canary.go:52) | operator reads/edits .evolve cli-health state or waits for a launch | evolve cli-health list \| cli-health clear <family> | no |
| rebuild + ship the tracked go/evolve binary | none | NONE | runtime-reference.md:274-285: raw go build -ldflags, jq del(.expected_ship_sha), ship with EVOLVE_BYPASS_COMMIT_GATE | whole chore is raw go/jq plus a gate bypass | untrack go/evolve (item) or evolve rebuild --tracked | yes |
| wait for the brake to take effect | none | NONE | cmd_loop_stop.go:19-20 has no --wait; runtime-reference.md:174 says watch the loop log for the brake line or the process to exit | operator tails the loop log / polls the pid before merging | evolve loop-stop --wait [--timeout D] | no |
| zero-ship halt (2 consecutive zero-ship cycles) | none | NONE | operating-policy.md:177 "No Go code counts zero-ship cycles"; operator applies it by hand | operator counts ships per wave and stops the loop manually | compiled breaker in the wave engine (policy failure_policy.thresholds.zero_ship_halt) | yes |

### Wave-boundary procedures

| Function | How it runs today | Coverage | Evidence | Gap | Suggested verb | Protected |
|---|---|---|---|---|---|---|
| full boundary sequence (stop, merge, sync, build, reset-sha, gc, launch) | separate verbs only | PARTIAL | runtime-reference.md:174-177 lists 4 steps across evolve + gh; build/merge/launch-verify are outside | operator sequences evolve + gh + make + nohup by hand | evolve boundary run --merge <prs> --goal-text-file F | no |
| launch the loop detached and verify the launch | evolve loop (foreground only) | PARTIAL | cmd_loop_args.go:37-53 has no --detach; a sync launch from a session hangs (memory); verification is by log/pid | nohup + log redirect + pid check by hand | evolve loop --detach --log F (prints pid + boot verdict) | no |
| monitor a running wave: ship verdict per lane | evolve dashboard --snapshot | PARTIAL | snapshot cycles[].verdict/commit_sha; loop log has no ship-ok line; session read runs/cycle-N/signals.ndjson | operator greps signals.ndjson per cycle for ship verdicts | evolve loop watch (per-lane phase/verdict/ship lines) | no |
| read-only loop status | evolve dashboard --snapshot | PARTIAL | dashboard model LoopStatus{running,brake_engaged,cycle_id,phase}; evolve loop status is NOT a verb (parsed as goal text) | no loop status verb, and the natural spelling launches a loop | evolve loop status (read-only) | no |
| classify a CI red (flake vs real) before retry | none | NONE | CLAUDE.md:47; session used gh run view --log-failed; ciwatch.Watch has no caller; wavesync only detects red (cmd_loop_wavesync.go:36) | read logs with gh and classify by hand | evolve ci classify <run-id\|pr\|sha> | no |
| generate the next wave goal text from the previous goal and outcomes | none | NONE | cmd_loop_args.go:39-40 only accepts --goal-text/--goal-hash; no goal derivation | operator hand-writes the next goal from the last goal + outcomes | evolve loop goal --from-last [--outcomes] | no |
| land a plane-side inbox stamp as a data PR and discard the plane copy | none (route-console/route-lane/consume write the stamp only) | NONE | runtime-reference.md:167 "Steps (2) and (3) are plain git: no evolve verb lands or discards a plane-side stamp yet" | copy item to a branch, PR, merge, git checkout the plane copy, then sync-main | evolve sync-main treats inbox dirt equal to origin as clean + evolve inbox land <id> | yes |
| merge reviewed PRs at a wave boundary only when all checks pass | none | NONE | runtime-reference.md:175 gh pr update-branch + gh pr merge; memory merge_only_all_green | gh pr checks/update-branch/merge by hand; no lease check that the loop is stopped | evolve pr merge <n>... --require-green --update-branch (refuse on live lease) | no |
| tail/relay Signal Center coded lines | none | NONE | cmd_signals.go:15-27 only signals codes generate\|check | operator tails the loop log / signals.ndjson by hand | evolve signals tail [--cycle N] [--code X] | no |

### Console procedures

| Function | How it runs today | Coverage | Evidence | Gap | Suggested verb | Protected |
|---|---|---|---|---|---|---|
| check Claude Code version and lag vs latest | evolve bridge doctor (installed --version only) | PARTIAL | internal/bridge/doctor.go:225 and looppreflight/versioninventory.go:20 read --version; no latest-release comparison | compare with latest by hand; upgrade with claude update | evolve doctor versions [--latest] | no |
| comment campaign proof (rank/check/comments/verify) | go run ./cmd/commentaudit (separate binary) | PARTIAL | internal/commentaudit/cli.go:23-35; commit-gate only uses commentaudit.Equivalent for its waiver (commitgate/comment_only.go:73) | rank/check/verify need go run ./cmd/commentaudit | evolve comments rank\|check\|verify\|list | no |
| model currency (what a tier alias resolves to now) | evolve setup latest; models refresh\|list | PARTIAL | setup latest probes catalog tiers; claude tiers are aliases resolved inside Claude Code (memory claude_code_version_drift) | verified by a --model sonnet probe by hand | evolve models verify --cli claude (live probe prints served model id) | no |
| push a feature branch and open a PR | evolve ship --class manual (push only) | PARTIAL | ship pushes the current branch (phases/ship/gitops.go:344, landing/push.go:72); PR creation is gh | gh pr create by hand | evolve ship --class manual --pr | yes |
| run the local floor (gofmt go1.23, vet, integration, e2e, acs-durable, apicover-enforce, cover-strict) | evolve commit-gate run; evolve apicover -enforce | PARTIAL | commitgate/lanes.go:70 host gofmt -s + vet + golangci + targeted tests; tiers are make -C go test-integration\|test-e2e\|test-acs-durable\|cover-strict | make targets + GOTOOLCHAIN=go1.23 gofmt by hand | evolve floor run [--tier fmt,vet,integration,e2e,acs-durable,apicover,cover] | yes |
| verify which model a loop phase actually used | evolve models performance | PARTIAL | cmd_models_performance.go:97 shows the dispatched selector from llm-calls.ndjson, not the serving model | operator reads CLI transcripts message.model by hand | evolve cycle models --cycle N (serving model per phase) | yes |
| create a dev worktree for console work | none | NONE | workspace-layout.md:18 git worktree add dev/<task> -b <branch> origin/main; worktree create is per-cycle only | raw git worktree add + make -C go build | evolve worktree create --dev <task> --branch <b> | no |
| land a change as a patch onto current main | none | NONE | session: diff -> patch -> fresh branch from origin/main -> git apply -> ship | raw git diff/apply on a new branch | evolve land --patch F --onto origin/main --branch B | no |
| re-ship a branch whose remote diverged (force-with-lease) | none (ship refuses force-push) | NONE | guards/ship.go:22 denies git push; ship repair ladder never force-pushes (runtime-reference.md Ship self-healing) | raw git push --force-with-lease under a guard bypass | evolve ship --class manual --force-with-lease (feature branches only) | yes |
| remove a dev worktree after merge | none | NONE | workspace-layout.md:36 git worktree remove + git branch -D | raw git; memory warns to verify the remote PR first | evolve worktree cleanup --dev <task> --merged | no |
| repair hub worktree links after moving the hub | none | NONE | workspace-layout.md:41 git --git-dir=.repo.git worktree repair | raw git | evolve doctor plane --repair | no |
| verify backups (bundle refs merged to main and pushed) | none | NONE | no backup/bundle support in go/ (grep); backups/ holds .bundle/.patch files | git bundle list-heads + merge-base checks by hand | evolve backups verify [--dir backups/] | no |
| watch CI for a pushed commit/PR | none | NONE | /commit uses gh pr checks --watch / gh run watch (skills/commit/SKILL.md:38); ciwatch.Watch exists with no caller | gh watch + log read by hand | evolve ci watch --sha HEAD [--pr N] | no |

### Inbox

| Function | How it runs today | Coverage | Evidence | Gap | Suggested verb | Protected |
|---|---|---|---|---|---|---|
| claim/promote inbox lifecycle | evolve inbox-mover claim\|promote | FULL | main.go usage inbox-mover | — | — | no |
| consume/retire an item with evidence | evolve inbox consume <path> --resolution T --cycle N\|console | FULL | runtime-reference.md operator commands (consume stamps + acks fingerprints) | — | — | no |
| inbox classification (lane vs console, dependency-blocked) | evolve inbox batches [--json] | FULL | cmd_inbox.go:38-62 emits console_routed and dependency_blocked with reasons | — | — | no |
| quarantine list/release | evolve inbox quarantine list\|release <id> | FULL | cmd_inbox_quarantine.go:19-61 | — | — | no |
| route an item to the console | evolve inbox route-console <id> <reason> <cycle> | FULL | runtime-reference.md operator commands (route-console, 2026-09-29); cmd_inbox_route_console.go | — | — | no |
| route an item to the lanes | evolve inbox route-lane <id> <reason> | PARTIAL | verb exists on branch feat/inbox-route-lane (36f9c1f62, unmerged); runtime binary c8faa4ec prints no route-lane | not usable from the published binary until the branch lands | merge feat/inbox-route-lane | no |
| edit/curate an inbox item (weight, files, depends_on, id stamp) | none | NONE | no edit verb; 2026-09-27 curation stamped ids by hand (inbox-item-identity-one-rule) | hand-edit JSON + PR | evolve inbox edit <id> --set k=v | yes |
| file a new inbox item | none | NONE | runtime-reference.md:179 file as a tracked file through a PR; no inbox add verb | hand-written JSON + PR | evolve inbox add --id --title --files --depends-on | yes |
| show/list/filter single inbox items | none | NONE | cmd_inbox.go:15-35 verbs: batches\|quarantine\|ack-fingerprint\|consume\|route-console\|route-lane; no list/show | operator reads .evolve/inbox/*.json by hand | evolve inbox show <id> \| inbox list [--route R --kind K] | no |
| stamp a console premise re-verification on an item | none | NONE | premise drift never resets; stale-drop evidence is free text (inbox premise-verification-anchor) | hand-edit the item | evolve inbox verify <id> --evidence T | yes |

### Release

| Function | How it runs today | Coverage | Evidence | Gap | Suggested verb | Protected |
|---|---|---|---|---|---|---|
| publish a release | evolve release X.Y.Z [--dry-run] | FULL | opscmd/release_pipeline.go; operating-policy.md section 5 | — | — | no |
| roll back a failed release | evolve rollback <journal> | FULL | registry.go rollback | — | — | no |
| verify marketplace propagation | evolve marketplace-poll <v> | FULL | registry.go marketplace-poll (verify-release skill still cites deleted legacy/scripts: skills/verify-release/SKILL.md:18) | — | — | no |
| verify release assets | evolve release-verify-binaries | FULL | registry.go release-verify-binaries | — | — | no |
| verify release installs per LLM CLI | evolve release-verify-clis | FULL | registry.go release-verify-clis | — | — | no |
| version bump / changelog / consistency | evolve version-bump \| changelog-gen \| release-consistency | FULL | registry.go opscmd rows | — | — | no |
| release readiness gate | evolve release-preflight; release-consistency | PARTIAL | /evo:release adds CI-green-on-main (gh run list) and no-WIP (git log) checks by hand (skills/release/SKILL.md:33,38) | gh run list + git log checks by hand | evolve release-preflight <v> --ci --no-wip | no |
| re-promote an auto-demoted release / rerun the release workflow | none | NONE | runtime-reference.md:327 gh api -X PATCH ... prerelease=false; /evo:publish gh run rerun | raw gh api / gh run rerun | evolve release promote <tag> \| release rerun <tag> | no |
| watch post-release CI (required.yml, release.yml) | none | NONE | releasepipeline/release_run.go:328 "GitHub CI is NOT verified by this pipeline"; /evo:publish uses gh run watch (skills/publish/SKILL.md:57) | gh run list/watch by hand | evolve release-verify-ci <version> | no |

### Maintenance

| Function | How it runs today | Coverage | Evidence | Gap | Suggested verb | Protected |
|---|---|---|---|---|---|---|
| flag / signal-code / skill doc projections | evolve flags\|signals codes\|skills generate\|check | FULL | registry.go flags, signals, skills | — | — | no |
| install/uninstall agents + loop skill | evolve install\|uninstall | FULL | registry.go | — | — | no |
| naming guard | evolve names check\|fix | FULL | registry.go names | — | — | no |
| phase / skill inventory caches | evolve phase-inventory build \| skill-inventory build | FULL | registry.go | — | — | no |
| phase catalog list/validate/add/create/coherence | evolve phases ... | FULL | phasecmd/phases.go:35-45 | — | — | no |
| prune ephemeral artifacts | evolve prune-ephemeral | FULL | registry.go prune-ephemeral | — | — | no |
| public-API coverage | evolve apicover [-enforce] | FULL | registry.go apicover | — | — | no |

### Observability

| Function | How it runs today | Coverage | Evidence | Gap | Suggested verb | Protected |
|---|---|---|---|---|---|---|
| audit calibration / context-fill / soak report | evolve audit calibration \| context-fill correlate \| soak-report | FULL | registry.go | — | — | no |
| estimate quota reset | evolve estimate-quota-reset | FULL | registry.go | — | — | no |
| explain a recorded routing decision | evolve routing | FULL | registry.go routing (read-only) | — | — | no |
| lesson recurrence ledger | evolve lessons recurrence | FULL | cmd_lessons.go:21 | — | — | no |
| live pipeline dashboard | evolve dashboard [--snapshot] | FULL | runtime-reference.md operator commands | — | — | no |
| model attempt performance | evolve models performance | FULL | cmd_models.go:35 | — | — | no |
| token usage report | evolve tokens report | FULL | cmd_tokens.go:54 | — | — | no |

### Internal entry points

| Function | How it runs today | Coverage | Evidence | Gap | Suggested verb | Protected |
|---|---|---|---|---|---|---|
| interaction.PromoteRule: promote a learned auto-responder rule | evolve bridge add-rule (manual add only) | PARTIAL | internal/interaction/rulepromote.go:77; test-only callers | corpus-validated promotion at stage shadow is unreachable | evolve bridge promote-rule --regex R --corpus F | no |
| ship.PlanLanding/LandPrefixes: preview what a ship would land | evolve ship --dry-run (checks only) | PARTIAL | internal/phases/ship/planlanding.go:10, landprefixes.go:10; test-only callers | no landing-plan preview listing paths | evolve ship --dry-run --plan | yes |
| ciwatch.Watch: watch CI for a pushed SHA, file a fix-forward item on red | none | NONE | internal/ciwatch/ciwatch.go:92; importers are only go/acs/cycle748 and cycle1769 predicates (no production caller) | CI feedback edge is dead code; operator watches CI with gh | evolve ci watch --sha S [--cycle N] | no |
| deliverable.SalvageVerdict: repair a malformed verdict | none (salvage report is rate-only) | NONE | internal/deliverable/salvage_extract.go:132; test-only callers | no operator verb to salvage one artifact verdict | evolve salvage verdict <artifact> | no |
| dossier.SweepOrphans: recommit orphan dossier pairs | none | NONE | internal/dossier/sweep.go:30; only sweep_test.go calls it | orphan dossier pairs are fixed by raw git | evolve dossier sweep [--dry-run] | no |
| phaseblock.Verify: per-phase integrity block verify | none (ledger verify / guard chain cover the ledger) | NONE | internal/phaseblock/verify.go:40; zero callers | per-phase integrity chain for a cycle is not checkable | evolve guard chain --cycle N --phases | yes |
| phasecoherence.CheckProvenance: ledger provenance of phase artifacts | none (phases check-coherence covers other checks) | NONE | internal/phasecoherence/provenance.go:28; test-only caller | provenance check unreachable | evolve phases check-provenance --cycle N | no |
| recurrence.BackfillFromLessons: rebuild the recurrence ledger | none | NONE | internal/recurrence/backfill.go:41; test-only callers | no way to rebuild the ledger from lessons | evolve lessons backfill | no |
| scopedelta.Classify/Summarize: declared vs touched files | none | NONE | internal/scopedelta/scopedelta.go:256,416; zero callers | no scope-delta check for a cycle | evolve cycle scope-delta --cycle N (or delete the package) | no |
| secretleakscan.ScanDiff: secret scan of a diff | none | NONE | internal/phases/secretleakscan/secretleakscan.go:46; test-only caller | no standalone secret scan of a diff | evolve scan secrets --diff <ref> | no |
| size / raw-git ratchet checks | none (go test ./internal/sizeratchet) | NONE | sizeratchet.Check (sizeratchet.go:141) and rawgitratchet.Check (:204) are called only from tests | landing runs go test on the ratchet packages by hand | evolve ratchet check [size\|rawgit] | no |
| wave plan preview (triagecap.SelectWaveSeedTopN, fleet.PartitionGraph, fleetbudget.Plan) | none (loop --dry-run prints config only) | NONE | triagecap/wave_seed.go:14 and fleet/packagegraph.go:68 test-only; fleetbudget.Plan only via loopwave | cannot preview which items/lanes the next wave would dispatch and why | evolve loop plan (lanes, items, exclusion census) | yes |

## Sources

- Inventory rows with file:line evidence: compiled 2026-09-30 from the tree at main `d7ac8f609`.
- Requests: `.evolve/inbox/` items filed 2026-09-30 through `evolve inbox add` (PR #750).
