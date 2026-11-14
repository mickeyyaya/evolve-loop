# Build Explanation — Cycle 2377

## Build Binding
- Cycle: 2377
- Base SHA: 5da3551a32ee8caacaa852c5a6517f98bbda7744

## Summary
`evolve wave latency --check` now judges each recorded wave against two bounds from `policy.json`. It leaves out a wave with a quota pause, a loop stop reason or an operator brake. Two breaching waves in a row emit the WARN signal `LATENCY_BUDGET_EXCEEDED` and file one inbox item. The operator brake stop now records a `loop.halt` signal with its stop reason, so the check can read it.

## Rationale
The plan in `docs/plans/latency-gates-2026-10.md` (section 4.3) asks for a soft wave gate. The measures already exist in `internal/lanetime`, so the judgement is a pure function of those measures and a list of marks from the project stream. The command stays thin: it reads files, calls the function, and files the result. A soft gate must not fail the command, so the command exits 0 on a breach.

## Changed Areas
- `go/internal/lanetime/check.go` — adds `Check`, `BreachRun`, `Bounds`, `WaveInput`, `Verdict`, and `Mark`. The package stays a leaf that imports only the standard library, and the bounds arrive as arguments.
- `go/internal/lanetime/read.go` — adds `Stream` and `ReadStream` to read samples and marks in one pass, and `ReadSamples` calls `ReadStream`. It also reads `stop_reason` and marks paused cycles.
- `go/internal/lanetime/cycle.go` — adds the `Paused` field to `Cycle`.
- `go/internal/lanetime/check_test.go` — covers each rule, the streak and the mark reader. The package stays at 100% statement coverage.
- `go/cmd/evolve/cmd_wave_latency_check.go` — new file with the `--check` run. It reads the waves, the policy bounds and the marks. It files one item with `inboxfile.Mint`. It emits the WARN signal only when the item is new.
- `go/cmd/evolve/cmd_wave_latency_check_test.go` — runs the check in process: the alarm and its rerun, an open wave, a plane with no data, and four unreadable inputs.
- `CHANGELOG.md` — adds the entry for the wave latency check.
- `go/cmd/evolve/cmd_wave_latency.go` — adds the `--check` flag, which excludes `--wave` and `--cycles`.
- `go/cmd/evolve/cmd_wave.go` — adds `--check` to the usage line.
- `go/cmd/evolve/cmd_loop_window.go` — the brake stop emits a `loop.halt` signal with `stop_reason` set to `loop_operator_brake`.
- `go/cmd/evolve/cmd_wave_latency_test.go` — updates the usage assertion and tests the flag exclusion.
- `go/cmd/evolve/cmd_root_guard_test.go` — lists the new signal root in the expected roots.
- `go/cmd/evolve/parallel_serial_tests_test.go` — lists the two brake tests of the TDD phase as serial.
- `go/internal/policy/wave_latency.go` — adds the `wave_latency` section with the two bounds and their defaults of 0.8 and 0.25.
- `go/internal/policy/wave_latency_named_test.go` — names the new policy exports with real assertions.
- `go/internal/policy/policy.go` — adds the `WaveLatency` field to `Policy`.
- `go/internal/signalcenter/event.go` — adds the closed module `latency` and the code constant.
- `go/internal/signalcenter/registry.go` — registers `LATENCY_BUDGET_EXCEEDED` with the module `latency`.
- `go/internal/signalcenter/event_test.go` — counts 40 modules.
- `go/internal/signalcenter/registry_latency_test.go` — proves the code is registered and normalizes without drift.
- `docs/architecture/signal-codes.md` — adds the `latency` section by hand for `LATENCY_BUDGET_EXCEEDED`. The generator `evolve signals codes generate` writes to the main checkout instead of the worktree.
- `docs/architecture/packages/internal-lanetime.md` — documents the wave check.
- `docs/architecture/packages/internal-policy.md` — documents the two bounds.

## Design Decisions
- The item id is `latency-wave-breach-from-wave-<first wave of the run>`. A run that grows keeps one item, a rerun files no second item, and a later run after a recovery gets a new id.
- A wave with no sample and no span is left out, so missing data neither counts nor resets a run.
- A signal belongs to a wave by pid or by time window, because a quota pause is written by a cycle process with another pid.
- A new signal module `latency` was needed because a code must start with the prefix of its module.

## Verification
The seven ACS predicates of cycle 2377 pass with `go test -tags acs ./acs/cycle2377`. The brake test `TestPrepareIteration_ABrakeStopRecordsALoopHaltSignalWithTheStopReason` passes. The tests of `internal/lanetime`, `internal/policy`, `internal/signalcenter` and `cmd/evolve` pass.

## Compatibility
Existing `evolve wave latency` flags and output do not change. The policy section is optional and has defaults. The brake stop adds one `loop.halt` event; the oversight listener already stops admission on that kind, which matches the brake meaning.

## Limitations
The check reads the whole wave history on each run. The boundary run of the check (T7) and the closing of the item on recovery are separate components of the plan.
