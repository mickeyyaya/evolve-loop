# Build Explanation — Cycle 2261

## Build Binding
- Cycle: 2261
- Base SHA: 11f31ce965fb39b390facab41dd943492c6f50b2

## Summary
Headless codex now takes its reasoning effort from the codex-tmux family table. The flag de-duplication now treats a flag and its whole value run as one unit.

## Rationale
The headless driver wrote a fixed `high` effort while the family table held the real policy. That made two homes for one policy. The de-duplication paired a flag with one token, so the tail of a multi-value tool list was dropped when a later flag shared a value. One home and whole-run groups remove both defects.

## Changed Areas
- `go/internal/bridge/realizer.go` — `dedupeLaunchFlags` groups a flag with its whole run of non-flag tokens and compares whole groups.
- `go/internal/bridge/realizer.go` — `RealizeFor` also adopts the effort parameter of the family table that a manifest points at.
- `go/internal/bridge/driver_codex.go` — the fixed `high` literal is removed. The effort key guard matches only at the start of a token.
- `go/internal/bridge/driver_credentials_test.go` — the old test asserted the literal `high`. It now asserts one resolved effort override.
- `docs/architecture/cli-capability-matrix.md` — a paragraph names both codex effort keys and their one home.

## Design Decisions
The headless manifest adopts the family effort parameter through the existing family pointer, inside the realizer. No second table and no flag are added. A launch with no resolved effort emits no key, and codex uses its own default.

## Verification
The seven ACS predicates of cycle 2261 pass. The new bridge tests pass. Five bridge tests fail on the base commit and on this change alike.

## Compatibility
The argv of a launch that carries an effort is unchanged for codex-tmux. Headless codex now carries the resolved tier and the plan-mode key.

## Limitations
A headless launch with no resolved effort no longer forces `high`.
