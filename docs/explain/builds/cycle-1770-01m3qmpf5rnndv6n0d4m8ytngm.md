# Build Explanation — Cycle 1770

## Build Binding
- Cycle: 1770
- Base SHA: fe0f8f203ba2a3fb1cfcd11b3b1ca75bd18032ec

## Summary
The four size-ratchet offenders in the test tooling — `parseArgs` (95 lines) and `artifactsFor` (79) in the fake CLI, `FakeExec.Run` (56) in the fixtures package, and `runCycle` (95) in the routing scenario engine — are now 43, 39, 47 and 50 lines. Each shrink is a behavior-preserving Extract Method: the same flags are consumed, the same artifacts are written, the same command responses are resolved, and the same scenario expectations are asserted.

## Rationale
The size ratchet allows these four functions only as listed offenders. Bringing each one under the 50-line `sizeratchet.MaxLines` limit turns its `offenders.json` entry into unclaimed slack that a later boundary tighten can remove. `offenders.json` itself is left unchanged, because an allowance is a ceiling and this lane never edits it.

## Changed Areas
- `go/cmd/evolve-fake-cli/main.go` — `parseArgs` now delegates the argument loop to `scanFlags`, which fills a new unexported `argFlags` struct, and the CLI-family inference to the `argFlags.style` method. `artifactsFor` now reads the proof-of-read line through `challengeTokenLine` and builds the audit report plus the candidate `acs-verdict.json` through `auditArtifacts`. The switch keeps every other phase inline.
- `go/test/fixtures/cmd_runner.go` — `FakeExec.Run` now records the call and resolves its scripted response (subcommand key, then name-only key, then `Default`) through `recordAndResolve`, which holds the mutex for exactly that critical section. Stdin draining and stdout/stderr writes stay outside the lock, as before.
- `go/internal/routingtest/engine.go` — `runCycle` now converts `ScenarioSpec.Enable` through `phaseEnablesOf` and hands its post-run checks (phase sequence, absent phases, decision inserts and clamps, routing ledger minimum, retro marker, proposer cadence) to `assertCycleExpectations`.
- `.evolve/evals/sizeratchet-shrink-test-tooling.md` — the task's eval and graders, written upstream this cycle.
- `go/acs/cycle1770/predicates_test.go` — the cycle's ACS predicates, written upstream by the TDD phase.

## Design Decisions
Every helper is unexported and stays in the file of the function it came from, so no package API changes. The existing comments moved with their code, and no comment was added. `artifactsFor` still prefixes the challenge token after the switch, so the audit report gets the token exactly as before, and `acs-verdict.json` never does. `recordAndResolve` switches from an explicit unlock to `defer`, which is equivalent because the section has no early return and no I/O.

## Verification
The existing test suites of the three packages pass without changes (`go test -count=1 ./cmd/evolve-fake-cli ./test/fixtures ./internal/routingtest`). All five cycle-1770 ACS predicates are green: the four functions fit the ratchet limit, offenders.json is unchanged, the module-wide ratchet check passes, the package tests pass, and no comment lines were added.

## Compatibility
The fake CLI's accepted flags, emitted artifacts and exit codes do not change; `FakeExec` keeps its recorded `Calls` order and response precedence; routing scenarios assert the same expectations. `offenders.json` is byte-identical to the base.

## Limitations
The four `offenders.json` entries stay in place until a boundary tighten removes them. `assertCycleExpectations` calls `t.Helper()`, so a failing expectation is now reported at the `runCycle` call line rather than inside the helper; the failure messages themselves are unchanged.
