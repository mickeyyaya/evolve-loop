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
  - When the run has a census, the line ends with the supply, for example `· supply 3 admissible, 4 waiting, 40 console`.
  - The JSON has `measures` for the current run and `runs` for the last 3 runs. `width_source: policy` tells you that the run has no sizing record, and that `width_sized` is `fleet.count`. The definitions are in [wave-orchestrator.md §11](../architecture/wave-orchestrator.md#11-measures).
- To see the supply of work for the lanes, run `evolve wave supply`. It counts the inbox items in six classes and shows the cause of each refusal. It changes nothing. Add `--json` for the full census.
  - The classes are `admissible`, `waiting_cell`, `waiting_deps`, `console_route`, `console_kind` and `console_protected`. The definitions are in [wave-orchestrator.md §4](../architecture/wave-orchestrator.md#4-the-supply-census).
  - The loop takes the same census at the start of each plan and keeps the last 12 in `.evolve/waves/supply.json`. When the `admissible` count is below the window, the loop publishes `LOOP_ORCH_SUPPLY_SHORT`.
- To see where the time of a wave goes, run `evolve wave latency`. It reads the run dirs and the `LOOP_ORCH_*` signals, and it changes nothing. Add `--wave N` for a recorded wave, `--cycles A-B` for a range of cycles, or `--json` for the full numbers. With no flag it reports the latest wave.
  - The report has the slot use: the average live lanes, and the slot-hours used over the capacity.
  - It has the lane time by phase with its share, and the model and host time of each phase. It has the median gate time after a call.
  - It also has the lane time to a PASS for each CLI family. The measure covers the last 20 builds of the whole history, not only of the wave. A family with no pass shows `none`.
  - The JSON gives each duration in nanoseconds. A wave that is still live is read to its last signal.
  - The definitions are in [internal-lanetime.md](../architecture/packages/internal-lanetime.md). The plan is [wave-latency-2026-10](../plans/wave-latency-2026-10.md).
- To follow the wave, run `evolve wave watch` under a monitor. It prints one line for each phase change, seal, ship and quota pause. It exits when the loop exits. When a newer wave starts, it prints `wave-started` and follows the newer wave.

## 3. At the boundary

Run the plane's own binary, `<plane>/go/bin/evolve`. The build step rebuilds that file, `reset-sha` pins the running binary, and the loop starts the running binary. Only the plane binary makes these three one binary. With another binary, `wave next` gives a warning that names both paths.

1. Run `<plane>/go/bin/evolve wave next --merge <n,...> --dry-run`. Read the goal and the steps.
2. Run `<plane>/go/bin/evolve wave next --merge <n,...> --json`. Read the envelope on stdout. The step output goes to stderr.
3. Use the exit code and the envelope to decide the next action:

| Exit | Envelope | Meaning | Next action |
|---|---|---|---|
| 0 | `launched: true`, `recorded: true` | The wave runs. | Run `evolve wave watch`. |
| 1 | `refused: "loop_live"` | A loop is live. | Wait, or stop it with `evolve loop-stop --wait`. |
| 1 | `refused: "number_taken"` | `--number` is not more than the last wave. | Remove `--number`. |
| 2 | `launched: false`, `error` | A file read or write failed before the steps. | Fix the named file. |
| 2 | `launched: true`, `recorded: false` | The loop runs, but the record save failed twice. The notes are consumed, and the old wave stays open. | Do not run `wave next` again. Fix `.evolve/waves/`; `wave status` shows no record for this wave. |
| 10 | `error: "usage: …"` | A bad argument. | Fix the arguments. |
| other | `failed_step`, `error` | A step failed with this exit code. The wave is not recorded, and the notes stay. | Fix the cause, then run step 2 again. A failure before the brake release leaves the brake on, which is the safe state. |

### The code index step

`wave next` has one step, "refresh the code index", after the build and the `doctor live` checks and before the loop launch. It moves the index worktree (`code_index.index_dir`, `dev/code-index` by default) to `origin/main` and runs the incremental `trail build`. Lanes then start on a fresh index. The step writes only the index worktree.

A failed refresh prints `evolve wave: WARN: code index not refreshed: <reason>` on stderr. It never stops the wave. The agents then use `grep`, as they do when no index exists. To repair the index, fix the cause in the message and run `evolve wave next` again at the next boundary.

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
| 0 | `window` | After P2b, merge the green PR with `evolve pr merge <n>`. Before P2b, merge it at the boundary. Do not use `gh pr merge`. |
| 3 | `boundary` | Merge the PR at the next boundary. Below integration `enforce`, the reason is `merge windows wait for fleet.integration.stage enforce (ADR-0147)`. At integration `enforce` and stage `off`, it is `pr.merge_window.stage is off`. The field `rule` shows the class of the paths. |
| 2 | — | A read or a parse failed (policy, `required.yml`, the file list, `gh`). A routing line in `required.yml` that the rule does not model also exits 2. Fix the named cause. |
| 10 | — | A usage error. |

The path classes come from the CI router of `.github/workflows/required.yml`. A change to that router also changes the merge window.

### The on-demand boundary

Call an early boundary when a green boundary PR waits and the pool runs below its window:

1. Run `evolve wave status` and read the width of the pool.
2. Run `evolve loop-stop --wait`. The lanes in flight finish, and the loop stops.
3. Run `evolve wave next --merge <n,...>`.

Until P2b gives a policy bound, call it when a green code PR has waited 1 h (plan Q2).

## 6. Other cases

| Case | Verb |
|---|---|
| Stop at the next wave boundary | `evolve loop-stop --wait` |
| Resume a paused cycle | `evolve loop --resume` |
| Run one cycle in the foreground | `evolve cycle run --goal-text "<goal>"` |
| Turn off the CLI health check | policy `wave.health_drivers: []` |
| Run more cycles in each wave | policy `wave.max_cycles`, or `--max-cycles N` |
| Show more cycles in the facts section | policy `wave.facts_max_cycles` (default 20) |
