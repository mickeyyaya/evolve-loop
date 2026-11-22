# Build Explanation — Cycle 2603

## Build Binding
- Cycle: 2603
- Base SHA: f34d4bc1d52cc4b4b7d5eb13d2306889ade61b33

## Summary
The restore path for untriaged paused lane scopes is in place; spec recording in `go/internal/core/cyclerun_dispatch.go` waits for the console follow-up. A granted paused scope whose inbox item left the inbox stays out of the front of the backlog.

## Rationale
`pausedPlan.lead` built an untriaged lead todo with only its ID and files, so the pool lost dependencies, priority, and output contract after a quota pause. Storing `LaneSpec` inside the lineage record lets `PausedLaneScopes` and `pausedPlan.lead` restore those fields without a triage entry.

## Changed Areas
- `docs/architecture/checkpoint-resume.md` — updates the untriaged paused-scope rule to name `LaneSpec`, `EVOLVE_FLEET_SPEC`, `PausedLaneScopes`, and `pausedPlan.lead`.
- `go/cmd/evolve/cmd_loop_pool_paused.go` — copies `p.Spec` dependencies, priority, and output contract into an untriaged lead todo inside `pausedPlan.lead` while keeping triage values when present.
- `go/internal/committedset/lanerecord.go` — embeds `LaneSpec` in `Record` so lineage JSON persists and loads the spec.
- `go/internal/committedset/lanespec.go` — defines `LaneSpec`, `Encode`, `DecodeLaneSpec`, and `IsZero` for compact JSON serialization.
- `go/internal/core/input_index.go` — adds `spec` to `indexTarget` and keeps a recorded non-zero `LaneSpec` across subsequent runs of the same cycle in `cycleRecord`.
- `go/internal/core/lane_resume_all.go` — adds `Spec` to `PausedScope` and populates it from `committedset.ReadRecord` in `PausedLaneScopes`.
- `go/internal/fleet/pool.go` — encodes non-zero todo specs into `ipcenv.FleetSpecKey` inside `poolSpec`.
- `go/internal/ipcenv/ipcenv.go` — adds `FleetSpecKey` (`EVOLVE_FLEET_SPEC`) and registers it in `ProtocolKeys`.
- `go/internal/ipcenv/markers.go` — registers `FleetSpecKey` in the pipeline marker list with cell propagation disabled.

## Design Decisions
Embedding `LaneSpec` in `committedset.Record` reuses the existing host-owned lineage file instead of adding a new state file. `pausedPlan.lead` still checks `adoptable` before reading `itemFiles` or building a lead todo, so a scope with a removed inbox item never leads. `cycleRun.dispatch` decoding of `EVOLVE_FLEET_SPEC` in protected `go/internal/core/cyclerun_dispatch.go` remains a console follow-up.

## Verification
Cycle 2603 ACS predicates (`TestC2603_001` through `TestC2603_003`), package unit tests, `go vet`, `gofmt`, and `evolve docs ste-lint --strict` verify the behavior, 100% statement coverage of changed functions, and documentation.

## Compatibility
`LaneSpec` uses `omitempty` JSON fields, so existing lineage records without a spec decode to the zero `LaneSpec` with no schema break.

## Limitations
A lane builder cannot edit `go/internal/core/cyclerun_dispatch.go`, so `cycleRun.dispatch` still needs the console follow-up to decode `ipcenv.FleetSpecKey` into `indexTarget.spec` at `writeInputIndex`.
