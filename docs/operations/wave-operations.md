# Run a wave with `evolve wave`

This page tells the console (an operator or an LLM) how to run each wave through the CLI. Do not write a goal script. Do not watch the text log. The full contract is in [runtime-reference.md](runtime-reference.md) ("The wave as one verb") and in the [plan](../plans/evolve-wave-cli-2026-10.md).

`evolve wave next` is the operator and LLM path. `evolve boundary run` is the low-level primitive: it takes any goal file, and it stops a live loop first. Use it only when you must give a goal by hand.

## 1. Before the first wave

- The plane must have the tracked standing goal `.evolve/wave-goal.md`. Change that file (in a PR) to change the instructions that each wave repeats.
- When the plane has no wave record yet, give the number of the next wave one time: `<plane>/go/bin/evolve wave next --number 82 --dry-run`. After that, the number counts up from the records.

## 2. During a wave

- To queue a fact for the next goal, run `evolve wave note add "<text>"`. Use one note for each fact. Write it in ASD-STE100. `evolve wave note list --json` shows the queue.
- To see the state, run `evolve wave status --json`. It reads cycle state, signals and landing intents. It does not change anything.
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
| 2 | `launched: true`, `recorded: false` | The loop runs, but the record save failed. The notes are consumed. | Do not run `wave next` again. Fix `.evolve/waves/`; `wave status` shows no record for this wave. |
| 10 | `error: "usage: …"` | A bad argument. | Fix the arguments. |
| other | `failed_step`, `error` | A step failed with this exit code. The wave is not recorded, and the notes stay. | Fix the cause, then run step 2 again. A failure before the brake release leaves the brake on, which is the safe state. |

## 4. Other cases

| Case | Verb |
|---|---|
| Stop at the next wave boundary | `evolve loop-stop --wait` |
| Resume a paused cycle | `evolve loop --resume` |
| Run one cycle in the foreground | `evolve cycle run --goal-text "<goal>"` |
| Turn off the CLI health check | policy `wave.health_drivers: []` |
| Run more cycles in each wave | policy `wave.max_cycles`, or `--max-cycles N` |
| Show more cycles in the facts section | policy `wave.facts_max_cycles` (default 20) |
