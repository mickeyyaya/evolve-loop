# Build Explanation — Cycle 2050

## Build Binding
- Cycle: 2050
- Base SHA: 66b0bcc7e2f6f09823961034127a333d494162b8

## Summary
A non-spine phase that declares `outputs.signals` now gets a `handoff-<phase>.json` file after it completes. The routing digest reads that file, so `amplify.tests_added` reaches the router and mutation-gate can become admissible.

## Rationale
The router already read `handoff-<phase>.json` and the catalog already counted `outputs.signals` as a producer, but no runtime code wrote the file. The smallest fix is one writer inside the existing host-effects step, which runs after each phase.

## Changed Areas
- `go/internal/deliverable/handoff_signals.go` — parses the `evolve-signals` sentinel in the phase report and writes the declared keys to the handoff file.
- `go/internal/deliverable/host_effects.go` — keeps the spec lookup and calls the writer at the end of `Perform`.
- `agents/evolve-test-amplification.md` — tells the agent to emit the sentinel line.
- `go/internal/deliverable/handoff_signals_repro_test.go` — reproduces the missing writer through `Perform` and the digest.
- `go/acs/cycle2050/predicates_test.go` — eight predicates for the acceptance criteria.
- `.evolve/evals/non-spine-phase-handoff-signals-have-no-writer.md` — eval graders for the task.

## Design Decisions
The writer is generic: it uses the phase spec, not a phase name. It drops any key the spec does not declare. A missing or malformed sentinel writes no file. Rejected: a per-phase writer, because each new phase then needs new code.

## Verification
The eight ACS predicates and the deliverable package tests pass.

## Compatibility
Spine phases declare no `outputs.signals`, so their handoff files are unchanged. A phase with no declared signals gets no file.

## Limitations
A report must carry the sentinel on its own line. The writer does not infer signals from report prose.
