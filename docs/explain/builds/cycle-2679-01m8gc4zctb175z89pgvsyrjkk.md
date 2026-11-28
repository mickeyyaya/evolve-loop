# Build Explanation — Cycle 2679

## Build Binding
- Cycle: 2679
- Base SHA: cfc86a56097563ce4aae9856979ab6844793e910

## Summary
The agy headless driver now passes the routed model and effort to agy. It also logs the real flags. The route data in the agy manifest decides both values.

## Rationale
agy 1.5.17 has the `--model` and `--effort` flags. The old driver said that agy has no `-m` flag and that agy ignores the model. Both statements were wrong, and the argv never carried a model. The manifest already has the route data: the family table `model_tier_map` and the effort ladder. Two `flag` channels in the manifest use that data, so the driver needs no model logic of its own.

## Changed Areas
- `go/internal/bridge/manifests/agy.json` — `model_tier` becomes a `--model` flag fed by `model_tier_map`. A new `effort` param is a `--effort` flag. The note states the real agy flags.
- `go/internal/bridge/driver_agy.go` — the stale model messages become `logAgyRoute`. It names the `--model` and `--effort` pair of the realization, or says that the route has no model flag.
- `go/internal/bridge/driver_agy_message_test.go` — pins the message text and the headless argv, and proves that a route change changes the argv.
- `go/internal/bridge/coverage_batch3_test.go` — the agy warn test expects the new message.
- `go/internal/bridge/realizer_adopt_family_effort_test.go` — the no-effort family fixture is `claude-cloud`, because agy now declares an effort param.
- `go/internal/bridge/parallel_serial_tests_test.go` — lists the seven new serial tests, which swap the catalog directory seam.

## Design Decisions
The driver reads only `cfg.Realization.LaunchFlags`. It never reads a model name. A manifest change is enough to change the argv. Rejected: a model switch in the driver, because it copies the route data.

## Verification
`TestAgyDriver_*` pins the argv, the messages and the route-change behavior. The ACS predicates `TestC2679_001` to `TestC2679_003` run these tests.

## Compatibility
A launch without route data keeps the argv `-p <prompt> --dangerously-skip-permissions`. The tmux manifests do not change.

## Limitations
The effort value passes to agy as the route states it. The driver does not check it against a list of agy levels.
