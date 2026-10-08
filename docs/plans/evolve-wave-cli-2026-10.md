# `evolve wave`: a public CLI to run and watch waves (2026-10)

> **Status:** lane `cl-wave-cli`, stacked on PR #816 (`fix/loop-log-retention`). Written in ASD-STE100.
>
> **Owner of this document:** lane `cl-wave-cli`.

## 1. Request (verbatim)

On 2026-10-08 the operator wrote:

> Instead of writing python script to trigger the next wave, build the public cli or API command allow LLM to trigger the next wave / cycle through this new cli / api

> I don't want to see any python code like mk_wave81_goal.py , build the cli / api to cover this usage

## 2. The usage to cover

For each wave, the console did these steps by hand:

1. It copied a Python script. The script read the last goal text (36 KB) and replaced its header and one section. The header had the wave number, the main SHA and an ordinal word. The section was "Wave N and this boundary". The console wrote it by hand: cycles, verdicts, ships, pauses, landed PRs and things to watch.
2. It ran `evolve boundary run --goal-text-file <file> --max-cycles 1 --project-root runtime`.
3. Before that, it ran `make -C go build`, `evolve reset-sha --operator`, `evolve checkpoint save --all`, `evolve doctor live claude-tmux` and `evolve gc --dry-run`.
4. It watched the wave with a bash monitor on the loop log, `git ls-remote` for ships, and `cycle-state.json` for phase changes. The text log is not reliable, because tests print event lines into it.

Most of the 36 KB goal is an old narrative ("wave 21 shipped 1721-1724 …"). That wastes agent context. The rule in [logging-and-process-hygiene-2026-10.md](logging-and-process-hygiene-2026-10.md) section 6 says that agents read only bounded, relevant text.

## 3. Goals and non-goals

Goals:

- One public verb, `evolve wave`, with four sub-verbs: `next`, `status`, `watch` and `note`. Each has a JSON form for an LLM.
- The goal text is made by code, from facts. No script, no hand-written section.
- The goal size is bounded: a standing goal file, one "last wave" section, the queued notes and the last K wave summaries.

Non-goals:

- No new loop or cycle logic. `wave next` calls the verbs that exist.
- No new way to run one cycle or to resume. Section 7 names the verbs that do this.
- `wave watch` does not read the text log.

## 4. Decisions

| ID | Decision | Reason |
|---|---|---|
| D1 | The logic is in a new package, `go/internal/wave`: the records, the notes, the facts reader, the goal composer and the event diff. The verbs are in `go/cmd/evolve/cmd_wave*.go`. | The package is pure and has no subprocess, so tests use temp dirs only. The cmd layer is the composition root. |
| D2 | `wave next` reuses the boundary. `boundarySteps` is split into `boundaryHaltSteps` (loop-stop --wait, pr merge, sync-main, gc) and `boundaryLaunchSteps` (loop-stop --release, boundary-log, loop --detach). `wave next` puts its pre-flight steps between the two halves and runs all steps with `runBoundarySteps`. Its dispatcher adds four verbs and then calls `dispatchBoundaryVerb`. | The brief says "reuse them; do not copy them". `boundary run` keeps the same steps in the same order. |
| D3 | Step order: `checkpoint save --all`, then the halt half, then `wave-build`, `reset-sha --operator`, one `doctor live <driver>` for each configured driver, then the launch half. | The checkpoint must come before gc, because gc removes worktrees. The build must come after `sync-main` and the PR merges, so that the loop runs the merged code. |
| D4 | `wave-build` always runs `make -C <root>/go build`. It does not try to find out if a build is necessary. | The Go build cache makes a build with no change fast. A staleness check is more code and can be wrong. This is the one new subprocess site. It uses `sysexec.DefaultRunner`. |
| D5 | `reset-sha --operator` runs in the same process. | `selfsha.Running` hashes the file at `os.Executable()`. `loop --detach` starts that same file. So the pin and the launched loop always agree. |
| D6 | The standing goal is a tracked file, `.evolve/wave-goal.md`, with a `.gitignore` re-include beside `.evolve/policy.json`. | It is operator configuration, like `policy.json`. The plane reads it by a relative path. Because it is tracked, `sync-main` brings each change to the plane. The policy key `wave.standing_goal` can name a different path. |
| D7 | The wave records are `.evolve/waves/wave-<N>.json`. The composed goal is `.evolve/waves/wave-<N>-goal.md`. The notes are `.evolve/waves/notes/<time>-<seq>.md`. All are runtime state and stay ignored. | Each wave is one small file, so a reader opens only the files it needs. |
| D8 | A cycle belongs to wave N when its id is more than the `cycle_floor` of wave N. Its id is also not more than the `cycle_floor` of wave N+1. A missing `state.json` gives the floor 0. A cycle can pause in wave N and resume in wave N+1. It stays in wave N, because its id is not more than the floor of wave N+1. `cycle_floor` is `state.json:lastCycleNumber` at launch. | The loop increments `lastCycleNumber` when it claims a cycle. So the floor is a fact, not a guess from time stamps. |
| D9 | The facts come from structured state only. The phase comes from the run workspace `cycle-state.json` (or `run.json`). The final verdict comes from the `cycle.sealed` event in `signals.ndjson`. Pauses are `quota.paused` events. A ship is `.evolve/landing/cycle-N.json` with the status `complete`. The committed dossier covers a cycle whose workspace gc removed. | The brief says "from cycle-state, not the text log". These are the files the pipeline writes as its record. |
| D10 | The wave number is the last recorded number plus 1. `--number N` sets it when no record exists yet (the plane is at wave 81 before the first record). A number that is not more than the last one is refused. | No policy key for a value that is used one time. |
| D11 | The PRs merged at the boundary are the `--merge` numbers plus each `Merge pull request #N` subject that `git log --first-parent --merges` shows between the last record's `main_sha` and HEAD. Git runs through `gitexec`, the existing constructor. | The console also merges PRs with other verbs. Git is the record of what landed. |
| D12 | `wave watch` polls. It has no long-lived child process. It prints one line for each phase change, seal, ship, quota pause and the loop exit, and it exits when the loop exits. | It is safe under a monitor and replaces the bash monitor. |
| D13 | The `wave` policy block holds `max_cycles` (default 1), `history_k` (default 3), `standing_goal` (default `.evolve/wave-goal.md`) and `health_drivers` (default `["claude-tmux"]`). The defaults are in one Go place, `policy.DefaultWave*`. | Config, not code. No feature flags. |
| D14 | The wave record is written only after the launch step passes. The notes are removed only then. The notes go with the launched goal: after a launch they are removed even when the record save fails. A failed save is tried one more time, then the envelope says `launched: true, recorded: false` and stderr says `loop launched, wave record NOT written`. | A failed boundary does not use a wave number and does not lose notes. A launched loop already has the notes in its goal, so a second wave must not repeat them. |
| D15 | `wave next` refuses while a run lease is live (exit 1). `--dry-run` does not refuse. It shows a warning, because it changes nothing. | The brief asks for a clear refusal. A dry run is a safe read. |
| D16 | The loop of a wave is live when a run lease is live. It is also live when three facts are true. The recorded pid is alive. The log `.writer-pid` file (from #816) names that pid. The process started at or before the wave start (`ps -o lstart=`; an unknown start passes). | A pid can be used again by a different process after the loop exits. The writer pid file and the start time show that the pid is still the loop. |
| D17 | The operator runs the plane's own binary: `<plane>/go/bin/evolve wave next`. When the running binary is a different file, `wave next` gives a warning that names both paths. It does not refuse. | `wave-build` rebuilds `<plane>/go/bin/evolve`. `reset-sha` pins `os.Executable()`, and `loop --detach` starts `os.Executable()`. Only the plane binary makes the build, the pin and the loop one binary. To make the launch start the built file, `loop --detach` must change, which `boundary run` shares. |
| D18 | The facts section shows at most `wave.facts_max_cycles` cycles (default 20), the newest, and a line that counts the older ones. The outcome counts every cycle. | The goal stays bounded when a wave runs many cycles. |
| D19 | `boundary run` stays the low-level primitive: it takes any goal file, and its `loop-stop --wait` stops a live loop first. `wave next` is the operator and LLM path: it refuses a live loop, composes the goal and records the wave. | Both have users. `wave next` reuses the boundary steps, so they cannot drift. |
| D20 | The `cycle_floor` of a new wave is the highest of `state.json:lastCycleNumber` and `state.json:lastAllocatedCycleNumber` (the cycle lease). This changes D8. The rule has one home, `cyclestate.State.HighestCycleNumber`. `wave.CycleFloor` (it decodes `state.json` into `cyclestate.State`), the batch window of the loop (`readBatchWindowFloor`), the allocator (`core.allocateCycleNumberAbove`) and the resume bootstrap read it. | Live case 2026-10-09: `wave watch` of wave 82 showed cycle 1838, a fleet lane of wave 81 that failed in triage. The loop advances `lastCycleNumber` only when a cycle finalizes (`core` cycle finalize, `failurelog.Record`). A lane that stops before it finalizes does not advance it. Each lane that the loop starts advances the lease first (`core.allocateCycleNumberAbove`). So the lease is the highest cycle number of the previous run. |

## 5. The verb contract

### `evolve wave next`

```
evolve wave next [--max-cycles N] [--merge n,...] [--note "<text>"] [--number N] [--dry-run] [--json] [--project-root P]
```

- It composes the goal, prints the steps, runs them, records the wave and removes the consumed notes.
- `--note` queues one more note before the goal is composed.
- `--max-cycles` replaces `wave.max_cycles` for this wave.
- `--dry-run` prints the composed goal and the planned steps. It writes nothing.
- `--json` prints one envelope on stdout on every exit. This includes a refusal, a usage error, an I/O error and a failed step. The step output goes to stderr.

Exit codes and the envelope fields that go with them:

| Exit | Case | `refused` | `failed_step` | `launched` | `recorded` | `error` |
|---|---|---|---|---|---|---|
| 0 | launched and recorded, or a dry run | | | true (dry run: false) | true (dry run: false) | |
| 1 | a run lease is live | `loop_live` | | false | false | names the live run |
| 1 | `--number` is not more than the last wave | `number_taken` | | false | false | names both numbers |
| 2 | policy, standing goal, records, notes or goal file cannot be read or written | | | false | false | the cause |
| 2 | the loop launched, but the record save failed two times | | | true | false | `loop launched, wave record NOT written: …` |
| 10 | usage | | | false | false | `usage: …` |
| the step's own code | a step failed | | the step name | false | false | `step failed with exit code N: <step>` |

The caller can always tell three cases apart. `refused` is set for a refusal. `failed_step` is set for a failed step. Only `error` is set for an error before the steps.

The envelope (schema `evolve.wave.next/1`):

```json
{
  "schema": "evolve.wave.next/1",
  "wave": 82,
  "run_id": "20261008T120000Z",
  "pid": 4242,
  "goal_path": "/plane/.evolve/waves/wave-82-goal.md",
  "goal_bytes": 3120,
  "dry_run": false,
  "launched": true,
  "recorded": true,
  "refused": "",
  "failed_step": "",
  "error": "",
  "steps": [{"name": "evolve checkpoint save --all --project-root /plane", "verb": "checkpoint", "ok": true, "detail": "rc=0"}],
  "warnings": []
}
```

`refused`, `failed_step`, `error` and `goal` (dry run only) are left out when they are empty.

### `evolve wave status`

```
evolve wave status [--json] [--project-root P]
```

Read-only. It shows the current wave, its run id, the loop pid and if the pid is alive (the loop-pid rule in D16). It shows if a run lease is live. It shows the cycles of the wave (id, phase, final verdict, shipped, quota pauses) and the outcome of the last closed wave. It shows at most 20 cycles. Exit `0`, or `2` when `.evolve/` cannot be read.

### `evolve wave watch`

```
evolve wave watch [--json] [--project-root P]
```

It polls the structured state every 5 seconds. It prints one line (or one JSON object with `--json`) for each event. The events are `phase`, `sealed` (with the final verdict), `shipped`, `quota-paused` (with the phase), `wave-started` and `loop-exit`. On each poll it reads the records again, and it follows a newer wave after `wave-started`. It exits `0` after `loop-exit`. It exits `2` when no wave record exists.

### `evolve wave note`

```
evolve wave note add "<text>" | list [--json] | clear [--project-root P]
```

`add` queues one note. `list` prints the queued notes in order. `list --json` prints a JSON array of `{name, text}`. `clear` removes them. `wave next` puts the notes in the goal and then removes them. Exit `10` for an empty note or an unknown sub-verb.

## 6. The composed goal

The goal has four parts, in this order:

1. The standing goal file (section D6), as it is.
2. `Wave <N-1> facts:` from the facts reader. It has the run id, each cycle with its verdict and ship state, the quota pauses and the merged PRs. When no last wave is recorded, this part says so.
3. `Operator notes for wave <N>:` with one bullet for each queued note. No notes, no part.
4. `Earlier waves:` with one line for each of the last `history_k` waves before the last wave, from their records.

All generated text is in STE. A typical goal is about 3 KB, against 36 KB for the hand-written wave 81 goal.

## 7. Cycles and resume

`wave next` runs `wave.max_cycles` (or `--max-cycles`). Other verbs already do the other cases, so this lane adds nothing new:

- To resume a paused cycle: `evolve loop --resume`.
- To run one cycle in the foreground: `evolve cycle run --goal-text "<goal>"`.
- To stop at the next wave boundary: `evolve loop-stop --wait`.

## 8. TDD protocol

1. The tests come first. The red run is kept in the lane scratch file `red.txt`.
2. `go/internal/wave` tests use `t.TempDir()` and fixed clocks only.
3. The cmd tests inject the step dispatcher (`boundaryDispatch`), so no test runs make, git merges, gc or a loop.
4. Each fix gets a mutation check: change the line, see the test fail, put the line back.
5. Coverage is 100%. The `go/internal/wave` floor in `go/.cover-strict` is 100. Each new function in `cmd_wave*.go` and `policy_wave.go` is at 100%.
6. Error paths use real file faults or an injected seam. A file fault is a file at a directory path. It can also be a directory at a file path, or one note name used two times. A seam is the root resolver, the running binary or the process start time.
7. Each error test checks the message, the exit code and the envelope fields.

## 9. Patterns and forces

| Pattern | Force |
|---|---|
| Composition root (the cmd layer wires the stores, the reader and the dispatcher) | The package stays pure and testable without fakes of its own logic. |
| Step list reuse (two halves plus pre-flight) | One boundary sequence. `boundary run` and `wave next` cannot drift. |
| Decorator on the dispatcher (records each step result) | The JSON envelope needs results, and `runBoundarySteps` stays unchanged. |
| Pure diff of two snapshots for `watch` | Events come from state changes, so a test needs two structs and no clock. |

## 10. Seams for other lanes

- The stale-claim release (`evolve inbox release`, from lane cl-inbox-claims) goes into `boundaryHaltSteps`, right after `loop-stop --wait`. This lane does not build it.

## 11. Limits

- gc can remove the workspace of a cycle before its dossier is committed (a fleet lane dossier waits in `dossiers-pending/`). Such a cycle shows only if its landing intent exists.
- `watch` sees a phase only when it polls. A phase shorter than the poll interval does not show.
- `wave-build` rebuilds `go/bin/evolve` in the plane. When the console runs a different binary, `loop --detach` starts that binary, as `boundary run` does today. D17 gives a warning for this case.
- Two ad-hoc decoders (`internal/redteamcheck`, `internal/wave`) read `state.json:lastCycleNumber` beside the typed `cyclestate.State`. Merged PRs come only from merge-commit subjects, so a squash merge is missed. The inbox item `wave-shared-state-reader-and-squash-prs` holds both.
