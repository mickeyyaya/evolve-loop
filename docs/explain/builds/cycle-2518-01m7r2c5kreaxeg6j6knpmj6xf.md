# Build Explanation — Cycle 2518

## Build Binding
- Cycle: 2518
- Base SHA: 35b489cfb815c9a66f1fc3b9a80e25c2e9388f26

## Summary
The `asserts-own-setup` rule of the test audit now follows aliases and holders. A field that the code under test writes is no longer own setup. A local alias of a literal is now own setup. The two tests of cycle 2506 assert the field directly.

## Rationale
The rule had two blind spots. It flagged `st.F` when a call wrote it through a holder, such as `engine{st: st}` followed by `e.run()`. It also missed `v := 1; x := v` because only composite literals counted as setup. A cycle 2506 lane hid the false positive behind locals, which created the false negative. One analysis fixes both sides.

## Changed Areas
- `go/internal/testaudit/rules.go` — links each literal variable to the variables its definition mentions, so a call that receives a holder also clears the literal inside it. Pure expressions, aliases and `var` declarations now define literals. A write inside a closure, or inside a loop or an `if` that a production value controls, clears the written variable. A `for` loop counter is never setup.
- `go/internal/core/failure_learning_engine_test.go` — the test asserts `st.FailedAt` directly and no longer copies the timestamps into locals.
- `go/internal/core/leak_ownership_recovery_test.go` — the test asserts `gate.calls` directly and no longer copies it into `reruns`.
- `go/internal/recovery/apicover_named_test.go` — the old test compared a struct copy with its source, which the new rule correctly reports. It now checks the fields of `PhaseOutcome` by reflection.
- `go/internal/ciwatch/classify_test.go` — the `seen` counter counted only the loops of the test, which the new rule reports. The counter and its check are removed.
- `go/acs/cycle2518/predicates_test.go` — the three predicates of the cycle: holder escape, alias follow, and direct assertion in the two core tests.
- `.evolve/evals/test-audit-asserts-own-setup-alias-blind-spot.md` — the eval graders that map to the three predicates.

## Design Decisions
Links between variables are symmetric, and a drop propagates to a fixpoint. This can only remove findings, never add one. A holder built with a call stays out of the literal set but keeps its links, so passing it on still clears what it holds. The alternative, to treat each derived variable that mentions a literal as an escape, drops too many true findings and was rejected.

## Verification
The predicates `TestC2518_001`, `TestC2518_002` and `TestC2518_003` pass. The shipped predicates of cycles 2489, 2501, 2504, 2506 and 2514 pass after the rule change. The packages `testaudit`, `core`, `recovery` and `ciwatch` pass with `-count=1`. The ACS suite reports `green=266 red=0`.

## Compatibility
No exported symbol changes. The rule reports new findings only for real own-setup assertions, and the repository has none left.

## Limitations
The control rules cover assignment and increment. A `switch` with a production tag is not tracked.
