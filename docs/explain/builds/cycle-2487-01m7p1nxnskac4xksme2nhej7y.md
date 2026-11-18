# Build Explanation — Cycle 2487

## Build Binding
- Cycle: 2487
- Base SHA: c33a0ee552aebe2ec3dfe72f4307ae363b00a4ee

## Summary
`evolve phases validate` and `evolve phases create` now refuse a user phase whose insert_when or skip_when field no phase and no typed signal produces.

## Rationale
The function `phasespec.UnproducedTriggerFields` existed but had no production caller. A phase that read a signal nothing emits passed both commands and stayed unreachable at runtime. One shared helper calls it from both commands, so the two cannot drift.

## Changed Areas
- `go/internal/cli/phasecmd/phases_triggers.go` — new helper that builds the routing config, applies the user phases, keeps only the checked phases and calls the producer check with `router.HarnessProduces`.
- `go/internal/cli/phasecmd/phases.go` — validate prints a FAIL line for each refusal and exits 2.
- `go/internal/cli/phasecmd/phases_create.go` — create returns a refusal envelope that names the phase and the field, before it writes any file.

## Design Decisions
Only the validated phases are checked. Built-in phases that the registry routes are out of scope, so an unrelated built-in trigger cannot fail a user command.

## Verification
The seven ACS predicates of cycle 2487 pass, and the phasecmd and phasespec packages pass.

## Compatibility
The scaffold of `evolve phases add` uses `build.files_touched`, a typed signal, so it still validates.

## Limitations
The check covers insert_when and skip_when only, as the existing function does.
