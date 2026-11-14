# Run a wave with `evolve wave`

This page tells the console (an operator or an LLM) how to run each wave through the CLI. Do not write a goal script. Do not watch the text log. The full contract is in [runtime-reference.md](runtime-reference.md) ("The wave as one verb") and in the [plan](../plans/evolve-wave-cli-2026-10.md).

`evolve wave next` is the operator and LLM path. `evolve boundary run` is the low-level primitive: it takes any goal file, and it stops a live loop first. Use it only when you must give a goal by hand.

## 1. Before the first wave

- The plane must have the tracked standing goal `.evolve/wave-goal.md`. Change that file (in a PR) to change the instructions that each wave repeats.
- When the plane has no wave record yet, give the number of the next wave one time: `<plane>/go/bin/evolve wave next --number 82 --dry-run`. After that, the number counts up from the records.

## 2. During a wave

- To queue a fact for the next goal, run `evolve wave note add "<text>"`. Use one note for each fact. Write it in ASD-STE100. `evolve wave note list --json` shows the queue.
- To see the state, run `evolve wave status --json`. It reads cycle state, signals and landing intents. It does not change anything.
  - The text output has one measures line, for example `measures: width 2/3 · utilization 0.71 · conflicts 0/2`. It gives the realized width over the sized width and the utilization. Then it gives the landed lanes with a rebase or a re-audit over all landed lanes.
  - The next line is the health line, for example `health:  wave 7 · PQ6 0.143/day · top idle cause run_cap · top fail class spec · PQ7 0.500 · PQ8 1.000 · PQ9 0.000` (JSON `health`). It shows the PQ6 to PQ9 numbers of the latest wave. It also shows the idle cause with the largest share. Last, it shows the most frequent fail class of the unshipped cycles. `evolve quality report --scope wave` prints the same line, because both verbs read it from `pipelinerates.ReadWaveView`. A wave with no idle cause or no fail class shows `none`.
  - A reading that was never taken shows `n/a`. This happens when the wave has no slot window or no lane time.
  - The realized width and the utilization come from the pool signals in `.evolve/signals.ndjson`: `LOOP_ORCH_LANE_LAUNCHED`, `LOOP_ORCH_LANE_BACKFILLED` and `LOOP_ORCH_LANE_EXITED`. The cycle state does not give them. A run with no pool signals uses the cycle signals.
  - The pool signals of the run come from the wave loop. They have the `pid` of the wave record, and they start at the run ID time. The pool launches its first lanes before `wave next` writes the record, so a cutoff at `started_at` loses them.
  - The width is the time-weighted median over the run. It is not the count of lanes that are live now. For that count, read the line `pool:    live N` (JSON `pool_live`). It is `fields.live` of the latest pool signal of the run. The line shows only while the wave loop is live.
  - A cycle whose lane got `LOOP_ORCH_LANE_DEADLINE` with `action: stopped` shows `cycle N: stopped: lane deadline (phase build)`. It is not a live lane. The JSON gives `termination: stopped: lane deadline` and `todo` for that cycle.
  - The loop seals that outcome in `.evolve/runs/cycle-N/signals.ndjson`: one `cycle.sealed` with `termination_reason: stopped: lane deadline` and no final verdict. A run from before 2026-10-21 has no seal, so the commands read the stop from the deadline signal. `wave watch` and the facts of `wave next` use the same rule ([wave-orchestrator.md §11](../architecture/wave-orchestrator.md#11-measures)).
  - When the run has a census, the line ends with the supply, for example `· supply 3 admissible, 4 waiting, 40 console`.
  - The JSON has `measures` for the current run and `runs` for the last 3 runs. `width_source: policy` tells you that the run has no sizing record, and that `width_sized` is `fleet.count`. The definitions are in [wave-orchestrator.md §11](../architecture/wave-orchestrator.md#11-measures).
- The benches at launch: before the pool admits its first lane, the loop runs the usage probe on each installed CLI. The probe benches each exhausted family. `wave status` shows one line for each active bench, for example `benched: agy usage_probe until 2026-10-28T18:53:39+08:00` (JSON `benched`).
  - `cli_health.usage_evidence_timeout_s` bounds the probe. Its default is in [runtime-reference.md](runtime-reference.md). A probe that fails or does not answer is one `WARN` line in the loop log, and it benches nothing.
  - The probe reads the quota at launch only. When the lanes use up a quota later, the reactive bench and the next wave boundary find it ([internal-usageprobe.md](../architecture/packages/internal-usageprobe.md)).
- To see the supply of work for the lanes, run `evolve wave supply`. It counts the inbox items in six classes and shows the cause of each refusal. It changes nothing. Add `--json` for the full census.
  - The classes are `admissible`, `waiting_cell`, `waiting_deps`, `console_route`, `console_kind` and `console_protected`. The definitions are in [wave-orchestrator.md §4](../architecture/wave-orchestrator.md#4-the-supply-census).
  - The loop takes the same census at the start of each plan and keeps the last 12 in `.evolve/waves/supply.json`. When the `admissible` count is below the window, the loop publishes `LOOP_ORCH_SUPPLY_SHORT`.
- To see where the time of a wave goes, run `evolve wave latency`. It reads the run dirs and the `LOOP_ORCH_*` signals, and it changes nothing. Add `--wave N` for a recorded wave, `--cycles A-B` for a range of cycles, or `--json` for the full numbers. With no flag it reports the latest wave.
  - The report has the slot use: the average live lanes, and the slot-hours used over the capacity.
  - It has the lane time by phase with its share, and the model and host time of each phase. It has the median gate time after a call.
  - It also has the lane time to a PASS for each CLI family. The measure covers the last 20 builds of the whole history, not only of the wave. A family with no pass shows `none`.
  - The JSON gives each duration in nanoseconds. A wave that is still live is read to its last signal.
  - The definitions are in [internal-lanetime.md](../architecture/packages/internal-lanetime.md). The plan is [wave-latency-2026-10](../plans/wave-latency-2026-10.md).
- To follow the wave, run `evolve wave watch` under a monitor. It prints one line for each phase change, seal, deadline stop, ship and quota pause. It exits when the loop exits. When a newer wave starts, it prints `wave-started` and follows the newer wave.

## 3. At the boundary

Run the plane's own binary, `<plane>/go/bin/evolve`. The build step rebuilds that file, `reset-sha` pins the running binary, and the loop starts the running binary. Only the plane binary makes these three one binary. With another binary, `wave next` gives a warning that names both paths.

A `wave next` that is not a dry run first heals a stale plane binary. When the binary is behind `HEAD` with a `go/` change, it rebuilds the binary and starts again in the new binary. `evolve loop` does the same at boot (`boot.binary_refresh`). A batch that adds a policy key thus does not stop the next boundary. If the policy still has a key that the binary does not know, the error tells you to run `make -C <plane>/go build`. It also gives the `wave next` command to run again, with your arguments.

The heal does not check for a dirty plane tree. The build uses the working tree, as at the boot of `evolve loop`.

1. Run `<plane>/go/bin/evolve wave next --merge <n,...> --dry-run`. Read the goal and the steps.
2. Run `<plane>/go/bin/evolve wave next --merge <n,...> --json`. Read the envelope `evolve.wave.next/2` on stdout ([cli-contract.md](../architecture/cli-contract.md) §2). Its `data` holds the fields of the table below. The step output goes to stderr.
3. Use the exit code and the envelope to decide the next action:

| Exit | `data` | Meaning | Next action |
|---|---|---|---|
| 0 | `launched: true`, `recorded: true` | The wave runs. | Run `evolve wave watch`. |
| 3 | `refused: "loop_live"` | A loop is live. | Wait, or stop it with `evolve loop-stop --wait`. |
| 3 | `refused: "number_taken"` | `--number` is not more than the last wave. | Remove `--number`. |
| 3 | `refused: "host_disk_low"` | The free space under `.evolve` is below the floor `preflight.min_free_gib`. The error names the free MiB and the floor. No step ran. | Run `evolve gc`, then run step 2 again. |
| 2 | `launched: false`, `error` | A file read or write failed before the steps. | Fix the named file. |
| 2 | `launched: true`, `recorded: false` | The loop runs, but the record save failed twice. The notes are consumed, and the old wave stays open. | Do not run `wave next` again. Fix `.evolve/waves/`; `wave status` shows no record for this wave. |
| 10 | `error: "usage: …"` | A bad argument. | Fix the arguments. |
| other | `failed_step`, `error` | A step failed with this exit code. The wave is not recorded, and the notes stay. | Fix the cause, then run step 2 again. A failure before the brake release leaves the brake on, which is the safe state. |

### Resume the paused cycles

Quota pauses can leave more than one cycle with a checkpoint. Use `evolve cycle resume --all` before `evolve wave next` to give each paused cycle an auto-resume budget. The verb starts no cycle.

| Step | Command | Result |
|---|---|---|
| 1 | `evolve cycle resume --all --dry-run` | It lists each paused cycle and changes nothing. |
| 2 | `evolve cycle resume --all` | It grants one auto-resume budget to each paused cycle. |
| 3 | `evolve wave next` | The pool puts the granted scopes first, and the lanes continue the paused cycles. |

See [checkpoint-resume.md](../architecture/checkpoint-resume.md#resume-every-paused-cycle-at-once) for the rules.

### The code index refresh

The code index refresh is part of `sync-main`. It is not a separate step, and the console does not run `git` in `dev/code-index` by hand. After `sync-main` merges `main`, it moves the index worktree (`code_index.index_dir`, `dev/code-index` by default) to `origin/main`, detached. Then it runs the incremental `trail build`. Lanes thus start on a fresh index. The refresh writes only the index worktree ([code-index-cli.md](../architecture/code-index-cli.md) §4).

| `sync-main` exit | Cause | Next action |
|---|---|---|
| 0, with `sync-main: moved the code index …` | The index is at `origin/main`. | none |
| 0, with `sync-main: no code index to refresh: …` | The host has no index worktree. | none. The agents use `grep`. |
| 22 | The index worktree has a change that is not `.gitignore` or `.ignore`. Nothing was reset. | Read the named paths. Keep or remove each change by hand, then run `evolve code refresh`. |
| 23 | `git fetch origin main` failed in the index worktree. | Fix the network or the remote, then run `evolve code refresh`. |
| 1, after the line `sync-main: reconciled …` | A guard, a policy error, a failed checkout or rebuild, or the deadline of 5 minutes. | Fix the cause in the message, then run `evolve code refresh`. |

When the refresh fails, the merge is already done. The step fails, and the brake stays on. After `evolve code refresh` exits 0, run the boundary again without `--merge`.

`evolve code refresh [--json]` runs the same refresh at any time. Each `evolve code` query prints a WARN on stderr when the index is behind `origin/main`.

### What the loop reads at the boundary

After the loop runs `evolve cli update`, it runs the practice watch. The watch reads the changelogs and guides of Claude Code and agy. It files one inbox research item for each source that changed.

The `[loop] practice-watch:` lines show the result. A `WARN` line does not stop the wave. Run `evolve practice watch --dry-run` to see the result with no record and no filing.

The item has class `hygiene`, so it ranks low. Read it at the next boundary. Then decide which prompt or persona to change under [ADR-0133](../architecture/adr/0133-the-loop-watches-cli-practice-and-trims-its-scaffolding.md).

## 4. The orchestrator stage

The policy key `fleet.orchestrator.stage` sets how the loop runs its lanes ([wave-orchestrator §7, §8](../architecture/wave-orchestrator.md#7-the-rolling-pool)). The default is `shadow`.

| What the console sees | `shadow` | `enforce` |
|---|---|---|
| How the lanes run | the barrier wave: all lanes start, and the wave ends when the last lane ends | the rolling pool: a free slot takes the next admissible todo at once |
| Which todo takes a slot | the file partition | admission by package cell; `LOOP_ORCH_LANE_BACKFILLED` names it |
| The end line | `[loop] wave N: X/Y lanes ok` | `[loop] pool N: X/Y lanes ok (orchestrator, target=W)` |
| How many lanes run at once | `fleet.count`, after the quota shrink | at most the adaptive window ([wave-orchestrator §10](../architecture/wave-orchestrator.md#10-the-adaptive-window)). It starts at `window.start` lanes (default 4). It adds 1 for each clean landing, and it halves on each landing conflict, down to `window.min`. |
| When a run ends | when the wave ends | when the supply is empty and no lane runs, at the operator stop, or after `run_max_lanes` launches (default 4 × `fleet.count`) |
| A lane past its deadline | `LOOP_ORCH_LANE_DEADLINE`, action `recorded`. The lane runs on. | the loop stops the lane, reaps its dispatches and releases its inbox claim. The todo does not launch again in the same run; a later run can take it. A second stop routes the todo to the console with the reason `deadline twice`. |
| A lane with no live signal (cycle event, pane-watch, events or stdout file) for `stall_minutes` (default 30) | `LOOP_ORCH_LANE_STALLED` once. The lane runs on. | the same |

At `enforce`:

- Merge at the boundary as before. The run is one boundary-to-boundary unit, and the boundary work runs after the run.
- `evolve loop-stop --wait` stops new launches. The lanes in flight finish, and then the loop stops at the boundary.
- A todo with route `console-manual` and the reason `deadline twice` needs the console. Read its last two cycles before you route it back to a lane.
- Remove `fleet.scheduling: pool`. The orchestrator always runs the pool, and the loop gives a WARN for the key.
- Read the line `[loop] orchestrator: adaptive window` at the start of a run. It names the lanes that the pool admits first. The window grows only when the oversight loop (plan O6) reports a clean landing. Until O6 is built, it stays at `window.start`. To run all `fleet.count` lanes before then, set `fleet.orchestrator.window.start` to `fleet.count`.
- `LOOP_ORCH_WINDOW` shows each change of the window, with the old size, the new size and the cause. The window bounds conflicts. It does not lift the serial landing limit of about 6 lanes.
- `LOOP_ORCH_LANE_RECOVERY_FAILED` means that a claim release or a park failed. Release or route the named todo by hand.

At `shadow`, count the `LOOP_ORCH_LANE_LATE_EXIT` events with `status` `ok`. Each one is a lane that passed after its deadline. A deadline stop at `enforce` stops such a lane. The exit to `enforce` needs none over 3 runs ([wave-orchestrator §14](../architecture/wave-orchestrator.md#14-stages)).

## 5. When a PR merges

The merge rule is in [ADR-0148](../architecture/adr/0148-docs-inbox-and-ci-changes-merge-in-a-window-and-code-merges-at-a-boundary-that-the-console-can-call-early.md). The plan is [pr-merge-latency-2026-10](../plans/pr-merge-latency-2026-10.md).

- A code PR merges only at a boundary. Use `evolve wave next --merge <n,...>`.
- A window PR is a PR that CI routes to no Go suite and no landing suite, after its `.github/` paths are dropped. It can merge when green, also in a wave, but only at stage `on` and `fleet.integration.stage: enforce`.
- The policy key `pr.merge_window.stage` is `off` by default. Below `fleet.integration.stage: enforce` (ADR-0147) the verb gives `boundary` at each stage: before that stage, a merge in a wave strands the next lane push.

To classify a PR, run `evolve pr window <n>`. Add `--json` for the envelope `{pr, stage, integration_stage, verdict, rule, reason, boundary_paths}`. The field `pr` is present only for a PR number.

| Exit | Verdict | Next action |
|---|---|---|
| 0 | `window` | Merge the green PR with `evolve pr merge <n>`, also while a loop runs. Do not use `gh pr merge`. |
| 3 | `boundary` | Merge the PR at the next boundary. Below integration `enforce`, the reason is `merge windows wait for fleet.integration.stage enforce (ADR-0147)`. At integration `enforce` and stage `off`, it is `pr.merge_window.stage is off`. The field `rule` shows the class of the paths. |
| 2 | — | A read or a parse failed (policy, `required.yml`, the file list, `gh`). A routing line in `required.yml` that the rule does not model also exits 2. Fix the named cause. |
| 10 | — | A usage error. |

The path classes come from the CI router of `.github/workflows/required.yml`. A change to that router also changes the merge window.

### Rebase a PR

- Open each PR from `origin/main`. Do not stack a PR on another open PR: a squash merge rewrites the commits of the first PR, so the stacked PR conflicts.
- After a rebase onto `origin/main`, run `evolve pr refresh`. It writes `docs/architecture/signal-codes.md`, `go/.darwin-set` and the generated region of `docs/research-index.md` again, and commits only those files. In a merge or a rebase that stopped on a conflict, it stages the files, and you continue the merge or the rebase.
- After a merge of `origin/main`, push the branch with `evolve ship --class manual --push-only`. It leaves out each commit that `origin/main` holds, and it checks each other commit.
- Do not merge a generated file by hand.

### The on-demand boundary

Call an early boundary when a green boundary PR waits longer than the policy bound and the pool runs below its width:

1. Run `evolve pr boundary`. It prints one line that names the green boundary PR that waited longest, its wait, the bound and the pool.
2. If it exits 0, run `evolve loop-stop --wait`. The lanes in flight finish, and the loop stops.
3. Run `evolve wave next --merge <n,...>`.

The bound is `pr.merge_window.early_boundary_after_s` (compiled default 3600, plan Q2). The pool width is `fleet.count`, and the live lanes come from the pool signal of the live wave. The operator decided this on 2026-10-20: "Combine them to fleet count". The width is not `fleet.min_lanes`.

```
$ evolve pr boundary
early boundary: #1012 waited 1h35m0s (bound 1h0m0s); pool 2/4 lanes: run evolve loop-stop --wait, then evolve wave next --merge 1012
```

| Exit | Meaning |
|---|---|
| 0 | Call the early boundary now. |
| 3 | No early boundary: no loop runs, no green boundary PR waits past the bound, or the pool is full. |
| 2 | A read failed: the policy, `gh`, `required.yml` or the pool signal of the live wave. |
| 10 | A usage error. |

`evolve pr merge` also merges a window PR while a loop runs, but only at stage `on` and integration `enforce`. A boundary PR keeps the refusal `a loop is running`, with the reason of `evolve pr window` added. Below that gate, `evolve pr merge` refuses each PR while a loop runs, as before.

## 6. Other cases

| Case | Verb |
|---|---|
| Stop at the next wave boundary | `evolve loop-stop --wait` |
| Resume a paused cycle | `evolve loop --resume` |
| Run one cycle in the foreground | `evolve cycle run --goal-text "<goal>"` |
| Turn off the CLI health check | policy `wave.health_drivers: []` |
| Run more cycles in each wave | policy `wave.max_cycles`, or `--max-cycles N` |
| Show more cycles in the facts section | policy `wave.facts_max_cycles` (default 20) |

## The console PR train

The design is in [§10 of the integration plan](../plans/wave-integration-2026-10.md#10-amendment-of-2026-11-04-console-prs-enter-the-train).

The console opens many PRs by hand. Each PR waits for a merge. The train puts the ready PRs into one batch and lands the batch with the engine of this plan.

- **Record.** A console PR has a record with the source `console-pr`. The record holds the PR number, the commits of the PR and a review binding. It holds no cycle number. A lane record keeps its cycle number and its trailer, and it validates as before.
- **Review binding.** The command refuses a binding that does not cover the PR. A reviewer name that you declare yourself never binds. The **Class** rule is apart: a class checks only the paths, and it proves no run.
- **Attestation.** The commit gate writes one record for each review, in the directory `evolve/reviews` of the git common directory. The name of the file is the `tree_state_sha` of the change. The command `record-pr --reviewed` reads the file that has the digest of the PR diff.
- **Diff of a PR.** The diff runs from the parent of the first commit to the last commit. A PR of many commits needs one review of the whole change. A record with a waiver does not bind.
- **Review runs.** The record must name a review run, and it holds a copy of the launch record of each run. Each launch record must show a reviewer that did not write source, a PASS or WARN verdict, a report and the same tree.
- **Review proof.** The profiles of the copied launch records must prove a simplify review and a correctness review. A change of only Markdown files needs a docs review. A reviewer name with no launch record does not bind.
- **Class.** A class comes from a closed list. Each class has a rule on the changed paths. The class `docs-review` uses the rule of the commit gate: every changed path ends in `.md`. A class that is not in the list does not bind.
- **Instruction files.** The class never covers an instruction file. These are `AGENTS.md`, `CLAUDE.md`, `GEMINI.md`, and each path under `agents/`, `skills/`, `.claude/`, `commands/` and `guidance/`. Use a review record for these paths. The **Review proof** rule gives the review that each record needs.
- **Record command.** `evolve integration record-pr --pr N` with `--reviewed` or `--class NAME` writes the record. The record is in the wave `console`.
- **Head check.** After the fetch, the command compares the head of each PR with the last commit in its record. If they differ, the command refuses the PR. The refusal says to record the review again. A PR with a new commit is never landed with its old commits.
- **Head check in the landing.** The command reads the head again before each CI run and before the fast-forward. A PR whose head moved is ejected. It stays open, and its comment names the commits that did not land.
- **Head check after the fast-forward.** The command reads the head one more time before it closes a PR. If the head moved, the PR stays open and its record is spent. The comment names the new commits.
- **Batch command.** `evolve integration batch [--dry-run] [--project-root P] [PR...]` takes the PRs that you name. Without a name, it takes the open PRs that are not drafts and are not batch PRs of the train. A PR with no record is refused, and the refusal names the PR. The command pushes nothing in that case.
- **Order and ejection.** The command orders the PRs with the order of the engine and composes them one by one. A PR that conflicts is ejected. The PR gets a brief with the paths and stays open. The same rule holds when `main` moves during the landing: the recompose ejects the PR that now conflicts, and the clean PRs still land.
- **Dry run.** The dry run prints the order, the ejections and the size of the batch. It does not push and does not touch a PR.
- **Landing.** `Merger.Land` opens one batch PR, watches the CI and moves `main` with one fast-forward (plan decision D11). A red batch is split as in plan decision D10. A run that has not started yet is not an error. The command waits for the run to register, up to five tries. Then it stops with an error.
- **Red and gh errors.** The command reads the verdict from the conclusion of the run. The conclusion `failure` is red. A run that was cancelled, a run that is not finished, and any error of `gh` stop the batch with an error. They never brief a PR.
- **Landed PRs.** A PR that is on `main` does not land again. A rerun after a failed close closes the PR and does not move `main`. A record is spent when its PR closes.
- **Main-red stop.** The batch reads the stop of the project root (`.evolve/main-red-stop.json`). The root is `--project-root`, then `EVOLVE_PROJECT_ROOT`, then the working directory. Name the root when you run the batch from a linked worktree.
  - While a stop holds, the command runs the keys of the stop on the batch tip before it moves `main`. A batch that turns a key green is a fix: it lands and releases those keys.
  - At `gates.main_red_stop` `enforce`, a batch that is red in CI, or that turns no key green, does not land. The command exits 1, and the text names the keys and the fix item. Land one batch that has each fix and no other change.
  - At `shadow`, the batch lands, and one `GATE_REPO_MAIN_RED_STOP_WOULD_REFUSE` event records the refusal of `enforce`.
- **Landed identity.** The command composes each commit of a console PR as its own commit on `main`, in order. Each composed commit has the `Source-PR` line. A PR is landed when each of its commits has a composed commit with the same patch-id. The patch-id is read with zero lines of context. A peer PR in the same file does not change the result.
- **Many commits.** The train does not squash a PR. A PR of two commits lands as two commits on `main`, in order. A PR of N commits lands as N commits. Each landed commit has the `Source-PR` line.
- **Landed check error.** If git fails during the landed check, the command stops with an error. It lands nothing.
- **Exit code.** The command exits with 1 when it ejects a PR. This holds for an ejection at the plan, at the landing and on a red CI. It also holds when a PR head moved after the landing. The error names each PR. The command prints a line when the CI was red once and green on the rerun.
- **Member PRs.** The command never pushes to the branch of a member PR. After a green landing, each landed PR closes with a comment that links the batch PR.
- **Inbox.** The composed commit keeps each `Closes-Inbox` id of the member commits, one id on each line. One parser reads the marker. It is in the package `closesinbox`, and the lane ship path uses the same parser.
- **Inbox move.** After the landing, the command moves each named inbox item to `processed` in the checkout where it runs. This move changes the working tree only. The command does not commit or push it, because only `evolve ship` commits and pushes.
- **Inbox command.** The command prints the `evolve ship --class manual --paths` command that puts the move on `main`. Run that command. An item that is not in the checkout is not moved, and the output does not name it.
