---
score_cap:
  - criterion: "evolve boundary run --dry-run --merge 12 --goal-text-file F prints the six steps in order and runs none of them"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1810_006_BoundaryDryRunPrintsSixStepsInOrderWithoutSideEffects$' ./acs/cycle1810"
  - criterion: "a refusal at the real pr merge verb stops the run before sync-main, keeps the brake engaged and returns pr merge's own exit code"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1810_007_BoundaryRefusalAtPRMergeStopsBeforeSyncMainWithItsExitCode$' ./acs/cycle1810"
  - criterion: "with fakes for each step the run stops at the first failure with that step's exit code"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestBoundaryRun_StopsAtFirstFailedStepWithItsExitCode$' ./cmd/evolve"
  - criterion: "a full fake run dispatches loop-stop --wait, pr merge, sync-main, gc, loop-stop --release, loop --detach in order and ends with the detached loop's pid and boot verdict"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestBoundaryRun_FullFakeRunEndsWithDetachedPidAndBootVerdict$' ./cmd/evolve"
  - criterion: "bad arguments (missing or absent goal text file, bad PR numbers, unknown flag or sub-verb) are refused before any step runs"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestBoundaryRun_(BadArgumentsDispatchNoStep|DryRunDispatchesNoStep)$' ./cmd/evolve"
  - criterion: "the launch step's argv is accepted by evolve loop's own argument parser as a detached launch on the resolved plane, carrying the goal text, --max-cycles and a log under the plane's .evolve/"
    max_if_missing: 2
    evidence: "cd go && go test -count=1 -run '^TestBoundaryRun_LaunchStepPassesTheRealLoopArgumentParser$' ./cmd/evolve"
  - criterion: "the dry-run plan's launch step names an absolute --log under the plane's .evolve/ and the dry run creates no log"
    max_if_missing: 3
    evidence: "cd go && go test -tags acs -count=1 -run '^TestC1810_010_BoundaryDryRunLaunchStepLogsUnderThePlanesEvolveDir$' ./acs/cycle1810"
  - criterion: "a launch the loop refuses returns the loop's own exit code after the release and says the brake is no longer engaged"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestBoundaryRun_RefusedLaunchReturnsTheLoopsExitCodeAfterTheRelease$' ./cmd/evolve"
---

# Eval: evolve boundary run — the wave boundary as one verb

> Pins the `cli-boundary-run` inbox item (console, 2026-09-30, core-function CLI inventory),
> first built in cycle 1810. runtime-reference.md described the wave boundary as steps the
> operator ran one at a time: `evolve loop-stop --wait`, merge console PRs, `evolve sync-main`,
> `evolve gc`, `evolve loop-stop --release`, then a nohup launch. A skipped or failed step (a merge
> while a lane still runs, a launch on an unsynced plane) was found later. `evolve boundary run
> [--merge n,…] --goal-text-file F [--max-cycles N] [--dry-run]` runs those verbs in order,
> reimplements none, prints each step's result, and stops at the first failure with that step's
> exit code — the release never runs after a failure, so the brake stays engaged.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| plan-order | dry run prints six steps in order, no side effect | 3/10 | `TestC1810_006_BoundaryDryRunPrintsSixStepsInOrderWithoutSideEffects` |
| real-refusal | real pr merge refusal stops before sync-main, rc preserved, brake kept | 4/10 | `TestC1810_007_BoundaryRefusalAtPRMergeStopsBeforeSyncMainWithItsExitCode` |
| fake-stop | first failing fake step stops the run with its rc | 4/10 | `TestBoundaryRun_StopsAtFirstFailedStepWithItsExitCode` |
| fake-full-run | six fakes in order, ends with pid + boot verdict | 4/10 | `TestBoundaryRun_FullFakeRunEndsWithDetachedPidAndBootVerdict` |
| negative-args | invalid args dispatch nothing | 6/10 | `TestBoundaryRun_BadArgumentsDispatchNoStep`, `TestBoundaryRun_DryRunDispatchesNoStep` |
| real-launch-argv | the planned `loop` argv passes the real `parseLoopArgs` (rc 0, detached, log under `<plane>/.evolve/`) | 2/10 | `TestBoundaryRun_LaunchStepPassesTheRealLoopArgumentParser` |
| plan-log | the dry-run launch line names an absolute `--log` under `<plane>/.evolve/` | 3/10 | `TestC1810_010_BoundaryDryRunLaunchStepLogsUnderThePlanesEvolveDir` |
| refused-launch | a refused launch returns the loop's rc and reports the brake as released | 5/10 | `TestBoundaryRun_RefusedLaunchReturnsTheLoopsExitCodeAfterTheRelease` |

## Adversarial Cases

- Negative: a refused merge must not be followed by sync-main, gc, the brake release or a launch.
- Edge/OOD: an I/O failure (rc 2) at any step is returned verbatim, not collapsed to 1.
- Cheapest gaming fake: a dry run that prints the plan but also runs the steps — `TestC1810_006`
  fails it (brake file, gh calls and plane HEAD are all checked).
- Cheapest gaming fake (cycle 1810 audit H1): a fake dispatcher accepts any argv, so a launch step
  without `--log` passed every fake test while the real `evolve loop` refused it with exit 10 after
  the brake was already released. `TestBoundaryRun_LaunchStepPassesTheRealLoopArgumentParser` sends
  the planned argv through the real parser; a relative `--log` fails it too, because the in-process
  loop resolves it against the operator's cwd, not the plane.
