# Cycle 31 — Bypass CLI Migration Notes

Cycle 31 removes six environment-backed emergency bypasses from the operator flag
registry and replaces them with explicit CLI booleans threaded through production
types. The worktree skip flag was also removed because it had no live reader.

## Implementation findings

- `envBypass()` was not exclusive to the six target flags: quota and document
  deletion guards also used it for unrelated controls. The helper was renamed to
  `envEnabled()` for those retained controls while Phase, Role, and Ship now
  receive bypass booleans through constructors.
- `evolve guard <name> --evolve-dir ...` placed common flags after the guard name,
  but the parent `flag.FlagSet` stopped parsing at the name. The command now parses
  a guard-local flag set after dispatch, which makes both `--evolve-dir` and the new
  `--bypass` flag effective in the existing hook invocation shape.
- Ship has two distinct emergency decisions, so `ship.Options` carries
  `BypassCommitGate` and `BypassPrefixGate` separately. This preserves auditability
  and avoids recreating a generic ambient bypass.

## Verification

- Cycle 31 ACS predicates: 8/8 pass.
- Flagreader regression: pass across Go, skills, agents, and root instructions.
- Full Go suite: pass.
- Native ACS suite: `green=87 red=0 skip=50 total=137`.
