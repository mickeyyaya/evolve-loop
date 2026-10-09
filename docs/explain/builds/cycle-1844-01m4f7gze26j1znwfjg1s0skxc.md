# Build Explanation — Cycle 1844

## Build Binding
- Cycle: 1844
- Base SHA: 2ecfde06ce99ec35ae80c314cd80f874b1446236

## Summary
`evolve phase <name>` and `evolve compose --phases` now run spec phases (catalog `llm` phases with no compiled runner, such as `plan-review` and `spec-verify`, plus `.evolve/phases/*` overlays) in addition to the compiled built-ins. An unknown name exits 10 and lists both the built-in set and the spec set.

## Rationale
An operator could not rerun `plan-review` or `spec-verify` outside a cycle because both commands only consulted the compiled `phases/registry`. The cycle itself already mounts these phases on `specrunner`, so the smallest change is one resolver in `phasecmd` that tries the registry first and the merged catalog second, constructing the same spec runner with the default bridge and prompts that built-in factories use. Both commands call that one resolver, so they cannot drift.

## Changed Areas
- `go/internal/cli/phasecmd/spec_phase.go` — adds `ResolveRunner`, `CatalogPhaseNames`, `FormatUnknownPhaseError` and `IsKnownPhase`: one resolver and one name partition so both CLI verbs agree on what is runnable.
- `go/internal/cli/phasecmd/spec_phase_test.go` — table tests for the known-name check, the native/control exclusions, the catalog-unavailable message and the unrunnable-spec miss.
- `go/internal/cli/phasecmd/phase.go` — `evolve phase` checks the name against both sets before reading stdin, then dispatches through `ResolveRunner` (extracted `resolvePhaseRunner` to stay inside the function-size ratchet).
- `go/cmd/evolve/cmd_compose.go` — compose validates and dispatches through the same resolver, emits the dual-set error, and lower-cases phase names so `Ship` cannot bypass the `--ship-anyway` guard once resolution is case-insensitive.
- `go/cmd/evolve/cmd_compose_test.go` — regression test proving a mixed-case `Ship` is still refused (fails without the lower-casing).
- `docs/operations/runtime-reference.md` — documents spec phase execution through `evolve phase` and `evolve compose`, the exclusions and the exit-10 message.
- `go/acs/cycle1844/predicates_test.go` — the TDD phase's cycle predicates for this contract (authored by the TDD phase, not edited by the build).
- `.evolve/evals/cli-phase-spec-phases.md` — the eval for this task (authored before the build).

## Design Decisions
Spec-runnable means kind `llm`, role not `control`, and no compiled factory: this is the same filter `registerBuiltinSpecRunners` applies in the cycle, so `ship`, `memo`, `retrospective` and the native scanners never resolve to a spec runner. The unknown check runs before the request is parsed, so a typo exits 10 without demanding stdin; the root for that check is `EVOLVE_PROJECT_ROOT` or the cwd. A catalog that cannot load still leaves the built-ins runnable and names the load error in the unknown-phase message instead of hiding it.

## Verification
`go test -tags acs -count=1 ./acs/cycle1844` passes all ten predicates; `go test -count=1 ./internal/cli/phasecmd ./cmd/evolve` passes, including the size ratchet; the mixed-case ship test was shown to fail with the lower-casing reverted.

## Compatibility
Built-in phases resolve exactly as before and receive the unchanged request. The unknown-phase message keeps the `unknown phase` wording; when the catalog has no runnable spec it keeps the old `(known: …)` form. Compose phase names are now case-insensitive.

## Limitations
CLI resolution does not re-run the cycle's `ValidateUserSpecWithCatalog` routing check on overlay specs, and it does not wire the cycle's contract verifier or host effects into the spec runner.
