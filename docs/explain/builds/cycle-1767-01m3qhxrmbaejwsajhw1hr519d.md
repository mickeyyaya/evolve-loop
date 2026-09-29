# Build Explanation — Cycle 1767

## Build Binding
- Cycle: 1767
- Base SHA: e404b914d41968fc2ba4124694c46f5dd7826b7b

## Summary
The three size-ratchet offenders in `go/internal/cli/phasecmd` — `runPhaseVerify` (74 lines), `phasesValidate` (69) and `phasesCreate` (123) — are now 36, 34 and 48 lines. Each shrink is a behavior-preserving Extract Method: the same checks run in the same order and print the same messages with the same exit codes.

## Rationale
The size ratchet allows these three functions only as listed offenders. Bringing each one under the 50-line `sizeratchet.MaxLines` limit makes its `offenders.json` entry unclaimed slack, which a later boundary tighten can remove. `offenders.json` itself is left unchanged, because an allowance is a ceiling and this lane never edits it.

## Changed Areas
- `go/internal/cli/phasecmd/phase_verify.go` — `runPhaseVerify` now calls three new helpers. `splitPhaseArg` separates the phase positional from the flags. `withCycleState` loads the cycle state into the roots for contracts with explanation sections or effects, and prints the same WARN lines. `reportVerifyResult` prints the JSON, OK or FAIL output and maps it to exit 0 or 1.
- `go/internal/cli/phasecmd/phases.go` — `phasesValidate` now calls three new helpers. `splitStrictProvenance` removes the `--strict-provenance` flag from the arguments. `warnMissingProvenance` prints the WARN line for each profile without `generated_from` and returns the profile names the "no user phases" branch needs. `reportUserSpecVerdicts` prints the OK and FAIL verdicts.
- `go/internal/cli/phasecmd/phases_create.go` — `phasesCreate` now calls five new helpers. `parseCreateFlags` (with the `createFlags` struct) parses the flags and runs the usage checks. `checkPersonaPath` returns the agents/-escape refusal, the overwrite refusal, or the missing-persona warning. `writePhaseFiles` marshals the spec, writes phase.json and the persona, and rolls back on a persona write failure. `rebuildInventory` rebuilds the phase inventory and adds any failure warning. `createdEnvelope` builds the success envelope.
- `.evolve/evals/sizeratchet-shrink-phasecmd.md` — the task's eval and graders, written upstream this cycle.
- `go/acs/cycle1767/predicates_test.go` — the cycle's ACS predicates, written upstream by the TDD phase.

## Design Decisions
Every helper is unexported and stays in the file of the function it came from, so the package API is unchanged. The existing comments moved with their code, and no comment was added. The checks that end in a refusal still run in the original order: collision, then the agents/ escape, then persona overwrite. So an input that failed on a given check before still fails on that check, with the same envelope.

## Verification
The existing `go/internal/cli/phasecmd` test suite passes without changes (`go test -count=1 ./internal/cli/phasecmd`). All five cycle-1767 ACS predicates are green: each function fits the ratchet limit, offenders.json is unchanged, the module-wide ratchet check passes, the package tests pass, and no comment lines were added.

## Compatibility
The CLI output, JSON envelope fields and exit codes of `evolve phase verify`, `evolve phases validate` and `evolve phases create` do not change. `offenders.json` is byte-identical to the base.

## Limitations
The three `offenders.json` entries stay in place until a boundary tighten removes them. Other oversized functions in the package are out of scope for this lane.
