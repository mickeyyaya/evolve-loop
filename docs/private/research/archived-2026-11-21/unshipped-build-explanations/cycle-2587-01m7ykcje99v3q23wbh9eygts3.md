# Build Explanation — Cycle 2587

## Build Binding
- Cycle: 2587
- Base SHA: 23c32f42926c3dcde2c2da08401c897afd66275a

## Summary
The loop tick now polls the CI of origin/main. A red CI opens the main-red stop with no lane reproduction, and ship refuses each landing while that stop holds. A green main closes the stop, and landings resume.

## Rationale
The only CI poll ran at the start of a wave. In the incident of 2026-11-20, three more landings reached a red main. The loop tick runs every minute, so the poll uses it. The stop, its fix item and its release already exist, so a CI failure becomes one more kind of key of that stop.

## Changed Areas
- `go/cmd/evolve/main_ci_poll.go` — adds the `mainCIPoll` type. It reads the origin/main tip with `git ls-remote` and asks the existing check-run query. A red result opens the stop. A green result closes the CI keys. An unknown state opens and closes nothing.
- `go/cmd/evolve/signal_loop.go` — runs the poll on each loop tick at the interval of the policy, in the sequential loop and in the fleet loop.
- `go/internal/core/main_ci_red.go` — adds `OpenMainCIRedStop`, `MainCIRed`, its incident event, the code `GATE_REPO_MAIN_CI_RED`, the CI key helpers and `CIKeysStayRed`.
- `go/internal/core/main_red_stop_open.go` — the fix item names the failed jobs when no lane found the red.
- `go/internal/phases/ship/main_red_stop.go` — runs no CI key in the lane, admits the CI fix lane, and refuses other lanes with `MAIN_CI_RED`.
- `go/internal/phases/ship/consume.go`, `go/internal/phases/ship/postship.go` — keep the CI fix item pending in the inbox while the CI stop holds.
- `go/cmd/evolve/cmd_integration_stop.go` — the train hold wraps its key runner with `CIKeysStayRed`.
- `go/internal/shiperr/shiperr.go` — adds the code `MAIN_CI_RED`.
- `go/internal/shiperr/signalcodes.go` — describes the code `MAIN_CI_RED`.
- `go/internal/policy/routing_gates.go` — adds `gates.main_ci_poll_minutes` with a default of 10 minutes.
- `go/internal/phases/ship/parallel_default_test.go` — lists the seam-swapping ship tests as serial.
- `go/internal/policy/gates_repocontract_test.go` — the expected defaults gain `MainCIPoll`.
- `go/internal/core/main_ci_keys_test.go` — tests the key split and that no CI key runs or turns green.
- `go/internal/core/main_ci_red_incident_test.go` — tests the incident event.
- `go/cmd/evolve/parallel_serial_tests_test.go`, `go/cmd/evolve/cmd_inbox_signals_test.go` — list the poll test as serial and pin the inbox literal of the poll.
- `docs/architecture/signal-codes.md` — two rows for the new codes.
- `docs/architecture/packages/internal-core.md` — documents the CI-only opener.
- `docs/architecture/packages/internal-phases-ship.md` — documents the CI refusal and fix-lane admission.
- `docs/architecture/packages/internal-policy.md` — documents the poll interval.

## Design Decisions
A CI failure is one key of the existing stop, in the package `origin-main-ci`. The fix item, the park, the bound and the release keep one home. The alternative was a second stop record. That record needs a second check in ship and a second release. The poll reads the tip with `ls-remote`, not `fetch`, so it never moves a ref under a running lane.

## Verification
The staged tests for the poll, the opener, the ship refusal, the fix-lane admission, the code and the policy pass. The ACS suite of the cycle covers the same behavior.

## Compatibility
A stop of lane keys behaves as before. A policy file without `main_ci_poll_minutes` keeps the default interval.

## Limitations
A CI-only stop refuses other lanes and integration batches, because no lane can prove a CI job green. Ship admits a lane that carries the stop fix item, and keeps that item pending until the poll sees a green main. The ship stage `gates.main_red_stop` sets if the refusal blocks lanes. Its default is shadow.
