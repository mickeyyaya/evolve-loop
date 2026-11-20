# Build Explanation — Cycle 2555

## Build Binding
- Cycle: 2555
- Base SHA: f25d49e4ae89c9d1c528d1c4fbc1687743635999

## Summary
Judgment carryover todos of the form `cycle-<N>-judgment-<phase>` now retire during cycle closeout at stage `enforce` only when their published document `docs/retrospectives/<id>.md` exists on `main`.

## Rationale
Retiring judgment todos in `Orchestrator.closeoutCarryover` before `writeShadowRetrospectives` keeps an unpublished todo on the first `enforce` closeout. That closeout emits the pending pair. After publication, the next closeout removes the todo without duplicating pending files or regex copies.

## Changed Areas
- `go/internal/core/carryover/judgment_retire.go` — adds `Lifecycle.RetireDocumentedJudgments`, which uses `recoveryretro.JudgmentLessons` to select only `cycle-<N>-judgment-<phase>` todos, queries the injected `documentOnMain` predicate for those IDs, and passes documented IDs to `Retire`.
- `go/internal/core/carryover_lifecycle.go` — wires `RetireDocumentedJudgments` into `Orchestrator.closeoutCarryover` when `projectRoot` is non-empty and `retrospectivesStage` equals `config.StageEnforce`, checking `<projectRoot>/docs/retrospectives/<id>.md`.
- `go/internal/core/retrospective_shadow.go` — extracts `retrospectivesStage` from `writeShadowRetrospectives` so `closeoutCarryover` and `writeShadowRetrospectives` share one `floorStage` lookup for `retrospectives.stage`.
- `docs/architecture/recovery-retrospective.md` — marks C5 step 4 and its guard tests as built and documents the `closeoutCarryover` to `RetireDocumentedJudgments` ordering.
- `docs/plans/recovery-retrospective-2026-10.md` — updates C5 status to built and records the Q2 decision that judgment todos retain long retention until their document lands on `main`.
- `docs/architecture/packages/internal-core-carryover.md` — documents `RetireDocumentedJudgments` and its `closeoutCarryover` stage `enforce` integration under the retirement design notes.

## Design Decisions
- Reused `recoveryretro.JudgmentLessons` inside `internal/core/carryover` instead of duplicating the `cycle-<N>-judgment-<phase>` regular expression or adding a new exported parser symbol to `internal/recoveryretro`.
- Injected `documentOnMain func(id string) bool` into `RetireDocumentedJudgments` so unit tests in `internal/core/carryover` stay filesystem-free while `Orchestrator.closeoutCarryover` supplies the `os.Stat` check on `docs/retrospectives/<id>.md`.
- Extracted `retrospectivesStage` in `go/internal/core/retrospective_shadow.go` rather than duplicating the `floorStage` arguments in `go/internal/core/carryover_lifecycle.go`.

## Verification
Targeted tests in `./internal/core/carryover` (100.0% statement coverage), `./internal/core`, `./internal/recoveryretro`, and `./internal/testaudit` pass. `./acs/cycle2555` (9/9 PASS), `gofmt -l .`, `go vet ./...`, `ste-lint --strict`, and `evolve acs suite --cycle 2555` (272/272 PASS) pass.

## Compatibility
`Lifecycle.Closeout` retains its existing signature; only `Orchestrator.closeoutCarryover` triggers judgment-todo retirement and only when `retrospectives.stage` is `enforce`. Below `enforce` (`shadow` and `off`), behavior is unchanged.

## Limitations
Judgment-todo retirement checks `<projectRoot>/docs/retrospectives/<id>.md` on disk rather than running git queries, assuming `PublishPending` writes files into that directory only when publishing to `main`.
