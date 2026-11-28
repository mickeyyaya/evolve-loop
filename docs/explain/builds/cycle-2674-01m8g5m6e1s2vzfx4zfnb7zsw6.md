# Build Explanation — Cycle 2674

## Build Binding
- Cycle: 2674
- Base SHA: e703c94329802a40f044bce55452bfa52f19df3a

## Summary
The `skillcheck` package now enforces the `24,000`-byte per-file cut of `agy` on loaded rule files, and `AGENTS.md` fits under that cut.

## Rationale
Moving long rule prose out of `AGENTS.md` and adding `rule_budgets` to `agy` keeps `AGENTS.md` uncut without touching `guidance/rules/`.

## Changed Areas
- `AGENTS.md` — moves detailed invariants, core rules, and historical runtime facts to `docs/operations/agents-full-rules.md` to fit under `24,000` bytes.
- `go/internal/skillcheck/templates/host_profiles.json` — adds `rule_budgets` of `24,000` bytes for `AGENTS.md`, `GEMINI.md`, and `.agents/rules/*.md` on the `agy` profile.
- `go/internal/skillcheck/hostprofile.go` — parses `rule_budgets` on `HostProfile` and implements `ruleFileProblems` to check loaded rule files on disk.
- `go/internal/skillcheck/skillcheck.go` — reports `ruleFileProblems` in `Run` and `Check` so over-limit rule files fail `evolve skills check`.

## Design Decisions
Using a separate `rule_budgets` field on `HostProfile` preserves the `16,384`-byte `budgets` table for rendered `CLAUDE.md` and `GEMINI.md` files.

## Verification
Unit tests in `go/internal/skillcheck`, `cliroute`, `guidance`, ACS predicates in `go/acs/cycle2674`, `evolve skills check`, and `evolve docs ste-lint --strict` pass.

## Compatibility
`CLAUDE.md` and `GEMINI.md` keep their `instructions-digest` blocks and `AGENTS.md#operating-rules-and-runtime-facts` links intact without schema or CLI breaks.

## Limitations
`evolve skills check` checks rule-file byte sizes on disk and does not estimate the aggregate `20,000`-token rule budget of `agy`.
