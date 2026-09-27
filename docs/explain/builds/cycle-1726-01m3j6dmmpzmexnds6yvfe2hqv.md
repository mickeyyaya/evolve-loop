# Build Explanation — Cycle 1726

## Build Binding
- Cycle: 1726
- Base SHA: 9fa77c2e46ad594d34a4927b0cab6875a180437c

## Summary
The 50-line function limit is now one repo-wide ratchet. The new package `go/internal/sizeratchet` walks the whole Go module with go/ast and compares every non-test function against a checked-in offender list. The list holds the 299 functions over 50 lines at the start of this cycle. A new function past 50 lines fails CI. A listed offender fails CI if it grows. If it shrinks, its allowance must be lowered in the same diff, and if it is healed or deleted, its entry must be removed. So the list can only get shorter.

## Rationale
Until now the limit was enforced only by packages that call `test/structure.CheckLimits` on their own directory. That let a bugfix cycle leave `ship/consume.go`'s `consumeCommittedItems` far over the limit with no test objecting. The ratchet is an ordinary `go test` in its own package, so CI's existing `make test-integration` step runs it over `go list ./...` with no workflow edit. An alternative was to add `CheckLimits` calls to every package, but it was rejected: it would need about 300 fixes or per-package exemptions, and every new package would still have to opt in.

## Changed Areas
- `go/internal/sizeratchet/sizeratchet.go` — new package. `Walk` measures function spans by module-relative key, `LoadOffenders` reads and validates the flat JSON allowance list, and `Check` applies the ratchet rules (new offender, growth, slack, stale entry) and names every violation, with its fix, in one error.
- `go/internal/sizeratchet/offenders.json` — the seeded census of the 299 current offenders at their exact sizes, generated from `Walk` over the module root.
- `go/internal/sizeratchet/sizeratchet_test.go` — the repo-wide gate. It finds the module root by walking up to the nearest `go.mod`, then fails on any `Check` violation. It has no build tag, so it runs under both plain `go test` and CI's `-tags integration`.
- `go/internal/sizeratchet/apicover_named_test.go` — unit tests that name and execute every exported symbol (`MaxLines`, `FuncSpan`, `Walk`, `LoadOffenders`, `Check`), as the apicover graduation requires.
- `go/.apicover-enforce` — enrolls `./internal/sizeratchet` in the per-package apicover hard-fail set.
- `go/acs/cycle1726/predicates_test.go` — the TDD phase's acceptance predicates for this cycle. They are unchanged by Build and are committed with the package they verify.
- `.evolve/evals/function-size-ratchet.md` — the TDD phase's eval for this task. It is unchanged by Build and committed with the cycle.
- `docs/explain/builds/cycle-1726-01m3j6dmmpzmexnds6yvfe2hqv.md` — this explanation document.

## Design Decisions
- The limit is strictly greater than 50 lines. Exactly 50 passes, matching the acceptance wording ("grows past", "exceeds"). Size runs from the `func` keyword to the closing brace, so doc comments are excluded, the same as `test/structure`.
- Keys are `<slash dir>.<Name>` or `<slash dir>.<Receiver>.<Name>`, with `*` and type parameters stripped from the receiver. `internal/bridge` has four different `Launch` methods over 50 lines, and each needs its own allowance.
- Same-key declarations in build-tagged files share one allowance, which is set by their largest span.
- Allowances must be exact rather than ceilings. Under a ceiling, a shrunk offender could silently grow back.
- Test files, `vendor`, `testdata`, and `.`- or `_`-prefixed directories are skipped. Build constraints are ignored, so platform files are measured too.

## Verification
- `go test -count=1 -tags acs ./acs/cycle1726/` passed 8/8. The predicates cover the boundaries, the growth, slack, and stale rules, walk semantics, list validation, list-equals-census, a CI-recipe run over a module copy with a planted 51-line function and a grown `consumeCommittedItems`, and apicover graduation.
- `go test -count=1 -tags integration ./internal/sizeratchet/` passes on the current tree.

## Compatibility
There is no runtime or CLI change. Per-package `test/structure.CheckLimits` tests keep their stricter `>= 50` rule. A lane that shrinks or removes a listed offender must now also edit `offenders.json`. The failure message names the exact edit.

## Limitations
The ratchet covers function length only. It does not cover nesting or file length, and it does not measure test files. The list is maintained by hand from the error text; there is no regenerate flag. Offenders changed by concurrent lanes will need a list update when they merge.
