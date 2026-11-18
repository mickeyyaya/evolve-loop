# Build Explanation — Cycle 2519

## Build Binding
- Cycle: 2519
- Base SHA: 35b489cfb815c9a66f1fc3b9a80e25c2e9388f26

## Summary
The `evolve wave supply` verb now prints one advice line: `console: build`, `console: feed` or `console: finish`. The JSON form carries the same text in `data.advice`.

## Rationale
Plan K3 of the console build-or-route plan says the console reads the supply at each boundary. The supply verb is the verb that the console already runs, so the advice lives there. The rule is a pure function, so a test can pin each boundary without a process.

## Changed Areas
- `go/internal/orchestrator/advice.go` — adds `ConsoleAdvice` and the batch limit of 10. More than 10 open PRs give finish. Fewer admissible items than the window give feed. Any other case gives build.
- `go/internal/orchestrator/advice_test.go` — pins each boundary: 10 PRs still build, 11 finish, 7 of 8 items feed, finish outranks feed.
- `go/cmd/evolve/cmd_wave_supply.go` — counts open PRs through `gh` and prints the advice after the text report. For JSON, it wraps the supply in a view with the `advice` field.
- `go/cmd/evolve/cmd_wave.go` — adds the `openPRs` port to `waveEnv` and wires `ghOpenPRCount` in the default environment.
- `go/cmd/evolve/cmd_wave_test.go` — the harness environment returns zero open PRs, so the tests that use it do not call `gh`. One test, `TestGhOpenPRCount_ReachesTheRealGhPort`, runs the real `gh` port.
- `go/cmd/evolve/cmd_wave_supply_test.go` — covers the advice line in text mode and the warning when `gh` fails.

## Design Decisions
Finish outranks feed, because D6 stops every new build while D4 only stops feature builds. The JSON view embeds `orchestrator.Supply`, so the persisted `supply.json` and the existing decoders keep their shape. A `gh` failure is a warning and counts as zero open PRs, so the verb still reports the supply.

## Verification
`go test -count=1 ./internal/orchestrator` passes. The wave tests of `cmd/evolve` pass. `evolve acs suite --cycle 2519` reports green=268 red=0, which includes the five predicates of this cycle.

## Compatibility
The text output gains one last line. The JSON data gains one field. No existing field changes.

## Limitations
The open PR count covers every open PR of the repository, not only console PRs. The limit of 10 is a constant, as plan question Q2 leaves it open for measurement.
