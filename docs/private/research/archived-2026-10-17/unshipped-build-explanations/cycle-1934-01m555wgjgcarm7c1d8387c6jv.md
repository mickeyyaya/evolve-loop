# Build Explanation — Cycle 1934

## Build Binding
- Cycle: 1934
- Base SHA: 7cce1ae6cf82d4331ae6ef08803fbcf1693dac22

## Summary
The compiled default of `cover_changed.stage` is now `enforce`. The build handoff floor refuses each changed Go function below 100% statement coverage and names it. An explicit `shadow` or `off` in `.evolve/policy.json` still wins.

## Rationale
CI runs `make cover-changed` in enforce, but the loop floor only warned. Cycles 1869 and 1870 shipped functions below 100%, and later PRs went red in CI on functions they did not touch. The floor already runs the same check against the cycle base, so a one-word default change closes the gap.

## Changed Areas
- `go/internal/policy/budgets_recovery.go` — `CoverChangedConfig` defaults the stage to `enforce` instead of `shadow`.
- `go/internal/policy/coverchanged_config_test.go` — the table pins the new default and that an explicit `shadow` still wins.
- `go/internal/core/cover_changed_floor_test.go` — the shadow cases now set `shadow` explicitly, and the no-root stage expects `enforce`.
- `go/internal/core/cover_changed_floor_strict_test.go` — the shadow cases set `shadow` explicitly instead of relying on the default.
- `go/internal/core/cover_changed_floor_tagfilter_test.go` — the shadow case sets `shadow` explicitly instead of relying on the default.
- `go/acs/cycle1934/predicates_test.go` — the TDD phase's predicates drive `core.DefaultBuildFloorChecks` with no policy block.
- `.evolve/evals/cycle-ship-enforces-cover-changed.md` — the TDD phase's eval for the task.
- `docs/architecture/policy-config.md` — the `cover_changed` table names `enforce` as the default.
- `docs/operations/runtime-reference.md` — the wiring note names `enforce` as the compiled default.

## Design Decisions
The triage named `go/internal/phases/ship/repocontract.go`, but the ship pack has no cover-changed check. The build handoff floor runs before ship and already measures the cycle diff against the base. A second copy in the ship pack would duplicate the check, so the change flips the policy default only. `go/internal/core/build_floor_reviewer.go` is a protected surface and stays unchanged.

## Verification
`go test -tags acs -count=1 ./acs/cycle1934` passes 6/6. `go test -count=1 -run 'CoverChanged|FloorStage' ./internal/core ./internal/policy` passes.

## Compatibility
The policy key and its words are unchanged. The checked-in `.evolve/policy.json` sets no `cover_changed` block, so live cycles now enforce. An operator can restore the warning with `{"cover_changed":{"stage":"shadow"}}`.

## Limitations
A plumbing fault in the coverage run still only warns. AC3 (CI on console PRs) needs a manual check after rollout.
