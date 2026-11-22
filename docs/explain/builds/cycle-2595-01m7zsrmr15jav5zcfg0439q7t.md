# Build Explanation — Cycle 2595

## Build Binding
- Cycle: 2595
- Base SHA: 74a0684210e1ee413c4a852820c06b8f87581270

## Summary
The loop tick polls the CI of origin/main at the policy interval. A red CI opens the main-red stop with no lane reproduction, and ship refuses other lanes while that stop holds. A lane that carries the CI fix item lands at enforce, and a green main closes the stop so normal landings resume.

## Rationale
Before this change, the CI poll ran only at wave start, so waves kept landing onto a red main. Adding a tick poll in `mainCIPoll` and representing CI failures as `origin-main-ci` keys in the shared main-red stop closes that gap. At enforce, ship admits only the lane that carries the stop fix item and keeps that item open until the poll sees a green main.

## Changed Areas
- `go/cmd/evolve/main_ci_poll.go` — adds `mainCIPoll` to poll origin/main with `git ls-remote` and check-run status at the policy interval.
- `go/cmd/evolve/signal_loop.go` — wires `newLoopMainCIPoll` into the loop tick announcer for both sequential and fleet loops.
- `go/cmd/evolve/cmd_integration_stop.go` — wraps integration train key runs with `core.CIKeysStayRed` so lanes cannot clear CI keys.
- `go/internal/core/main_ci_red.go` — adds `MainCIRed`, `OpenMainCIRedStop`, `GATE_REPO_MAIN_CI_RED`, CI key predicates, and `CIKeysStayRed`.
- `go/internal/core/main_red_stop_open.go` — generates CI-specific fix item summary, fix instructions, and empty file list for CI-only stops.
- `go/internal/phases/ship/main_red_stop.go` — refuses other lanes with `MAIN_CI_RED`, admits the CI fix lane in `admitsCIFixLane`, and skips lane runs for CI keys.
- `go/internal/phases/ship/consume.go` — excludes a held CI fix item with `withoutHeldCIFixItem` so it stays pickable until main turns green.
- `go/internal/phases/ship/postship.go` — excludes a held CI fix item from promotion in `PromoteLanded` while the CI stop holds.
- `go/internal/policy/routing_gates.go` — adds `gates.main_ci_poll_minutes` configuration field with a default of 10 minutes.
- `go/internal/shiperr/shiperr.go` — adds `CodeMainCIRed` (`MAIN_CI_RED`) ship error code.
- `go/internal/shiperr/signalcodes.go` — registers `CodeMainCIRed` in signal center metadata.
- `go/cmd/evolve/main_ci_poll_test.go` — tests red open, incident emission, green close, unknown-state safety, policy interval, and production wiring.
- `go/cmd/evolve/cmd_inbox_signals_test.go` — verifies the fix item literal alignment with `mainCIPoll`.
- `go/cmd/evolve/parallel_serial_tests_test.go` — lists `TestNewLoopMainCIPoll_LoadsPolicyAndFallsBackWhenMissingOrInvalid` as serial.
- `go/internal/core/main_ci_keys_test.go` — tests `StopCIKeys`, `StopLaneKeys`, `StopCIOnly`, and `CIKeysStayRed`.
- `go/internal/core/main_ci_red_incident_test.go` — tests `MainCIRed.Incident` fields and message formatting.
- `go/internal/core/main_ci_red_stop_test.go` — tests CI-only stop opening, fix item metadata, and empty failed-list guard.
- `go/internal/phases/ship/main_ci_red_stop_test.go` — tests `MAIN_CI_RED` refusal, shadow mode, and green-main resumption.
- `go/internal/phases/ship/main_ci_fix_lane_test.go` — tests CI fix-lane admission, negative cases, and `heldCIFixItem` with `t.Parallel()`.
- `go/internal/phases/ship/parallel_default_test.go` — lists seam-swapping CI stop ship tests as serial.
- `go/internal/policy/gates_mainci_test.go` — tests `main_ci_poll_minutes` parsing, defaults, and invalid values.
- `go/internal/policy/gates_repocontract_test.go` — updates expected `GatesConfig` defaults with `MainCIPoll`.
- `go/internal/shiperr/mainci_red_vocab_test.go` — verifies `CodeMainCIRed` registration.
- `.evolve/evals/main-ci-red-is-polled-while-the-wave-runs.md` — pins eval graders and score caps for CI polling and fix-lane admission.
- `docs/architecture/signal-codes.md` — documents `GATE_REPO_MAIN_CI_RED` and `MAIN_CI_RED`.
- `docs/architecture/packages/internal-core.md` — documents `OpenMainCIRedStop` and CI key helpers.
- `docs/architecture/packages/internal-phases-ship.md` — documents `MAIN_CI_RED` refusal and CI fix-lane admission.
- `docs/architecture/packages/internal-policy.md` — documents `main_ci_poll_minutes`.

## Design Decisions
CI failures become keys in the shared main-red stop under `origin-main-ci`. This reuses the stop file, bound check, fix item, and release flow. Polling with `git ls-remote` reads the remote main tip without mutating local refs in active worktrees. Admitting the CI fix lane in `admitsCIFixLane` while keeping its fix item open avoids an enforce-mode deadlock.

## Verification
All unit tests in `cmd/evolve`, `internal/core`, `internal/phases/ship`, `internal/policy`, and `internal/shiperr` pass. All `paralleltest` guards and all 8 ACS predicates in `go/acs/cycle2595` pass after the final edit.

## Compatibility
Lane-reproduced stops behave as before. Policy files without `main_ci_poll_minutes` default to 10 minutes.

## Limitations
A CI-only stop refuses non-fix lanes and integration batches until the loop poll observes a green origin/main CI. Ship refusal enforcement depends on `gates.main_red_stop` (shadow by default).
