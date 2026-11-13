# Build Explanation — Cycle 2353

## Build Binding
- Cycle: 2353
- Base SHA: f869d73e084d4e1f3bcccd23d51fdde772bd0337

## Summary
A paused cycle is now adopted by one fleet lane only. The lane records its resume through `checkpoint.RecordLaneResume`, which takes the resume lease of the paused workspace.

## Rationale
Two lanes of one scope can both admit the resume of the same paused cycle. The lease check and the try count now run under one path lock. When both lanes reach the lease together, the second lane gets a typed refusal (`*core.ResumeHeldError`) and starts a fresh cycle. A lane that arrives after the winner took the lease never reaches the record step, because discovery skips the leased cycle. That lane also starts a fresh cycle, with no typed refusal.

## Changed Areas
- `go/internal/core/lane_resume_lease.go` — adds `ResumeHeldError`, `TakeResumeLease` and `ReleaseResumeLease`.
- `go/internal/core/lane_resume_lease_test.go` — tests the take, the refusal, the takeover and the release.
- `go/internal/checkpoint/autoresume.go` — adds `RecordLaneResume`. `RecordResume` now shares the `recordResume` core and takes no lease.
- `go/internal/checkpoint/lane_resume_exclusive_test.go` — tests that two concurrent lanes admit exactly one.
- `go/cmd/evolve/cmd_cycle.go` — wires the lane of `evolve cycle run` to `RecordLaneResume`.
- `go/cmd/evolve/cmd_cycle_lane_resume.go` — releases the admission lease when the run ends or the reclaim fails.
- `go/cmd/evolve/cmd_cycle_lane_resume_exclusive_test.go` — tests the lane adoption, the lease-free operator resume and the release warning.
- `docs/architecture/checkpoint-resume.md` — documents the lease step.

## Design Decisions
The lease is taken only after the try is admitted, so a spent budget leaves no lease. A fresh unreadable lease file counts as held, because the holder is unknown.

## Verification
The new tests pass in core, checkpoint and cmd/evolve. The ACS suite reports red=0.

## Compatibility
`evolve loop --resume` keeps `RecordResume` and its behavior.

## Limitations
This change does not include F2, the lane identity for the project lock. F2 edits the protected files `cyclerun.go`, `orchestrator.go` and `resume.go`, so the console owns it.
