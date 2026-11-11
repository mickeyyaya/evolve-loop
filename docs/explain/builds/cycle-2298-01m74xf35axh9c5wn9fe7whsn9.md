# Build Explanation — Cycle 2298

## Build Binding
- Cycle: 2298
- Base SHA: 956cb2a56357dd3acbf5cfe9f48b47ded0c599c2

## Summary
A lane cycle that an operator stops during a foreground resume stays listed as resumable. The verb `evolve cycle resume --cycle N --grant` grants one named paused cycle and starts no run. This cycle continues cycle 2284, which failed its build floor on an unrelated integration test.

## Rationale
The stop writer rewrites the checkpoint reason to `operator-requested`. The lane filter accepted only `quota-likely`, so the wave lost the cycle. The filter now accepts both reasons. The operator also had only `--all` to grant a budget. One named grant is the smallest addition, and it reuses the grant function of `--all`.

## Changed Areas
- `go/internal/core/lane_resume.go` — `laneQuotaPaused` accepts the reason `operator-requested` beside `quota-likely`. A lane stopped by SIGTERM stays in the wave listing.
- `go/cmd/evolve/cmd_cycle_resume.go` — the new `--grant` flag. `--grant` needs `--cycle` and refuses `--all`. The grant path shares `runCycleResumeAll`.
- `go/cmd/evolve/cmd_cycle_resume_all.go` — `runCycleResumeAll` takes an `only` cycle. It keeps only that cycle, and exits 11 when the cycle is not paused.
- `go/internal/core/lane_resume_test.go` — the "other reason" case uses `batch-cap-near`, because `operator-requested` is now resumable.
- `go/internal/core/lane_resume_all_test.go` — the "non-quota cycle" case uses `batch-cap-near` for the same reason.
- `go/cmd/evolve/cmd_cycle_resume_all_test.go` — in-process tests of the named grant: only the named cycle is granted, and an unpaused cycle exits 11.
- `go/internal/core/lane_resume_stopped_repro_test.go` — the reproducer of the stopped lane.
- `go/acs/cycle2298/predicates_test.go` — the predicates of the cycle.
- `.evolve/evals/stopped-lane-resume-keeps-lane-checkpoint.md` — the eval graders of the task.

## Design Decisions
- Both reasons share one filter. A lane with a non-lane run never enters the listing, because the lane scope file is still required.
- The named grant reuses `cycleGrantResume`, so it cannot start a run.
- Rejected: a new core function for the named grant. The `--all` path already lists, grants and prints.

## Verification
The predicates `TestC2298_001` to `TestC2298_006` pass. The ACS suite for the cycle has red=0. The packages `internal/core`, `internal/checkpoint` and `cmd/evolve` pass.

## Compatibility
Existing `--all` and `--cycle` calls behave as before. A wave now also lists a lane that an operator stopped.

## Limitations
A lane that an operator stopped on purpose, outside a resume, is also listed. The operator can leave it, because the wave grants nothing without a command.
