# Build Explanation — Cycle 2501

## Build Binding
- Cycle: 2501
- Base SHA: c87e7879638550417df391d4d24afb567aad31a3

## Summary
The test-audit analyzer no longer treats a container that a test fills from production output as the test's own setup. Tests that assert such a container now count as behavioral tests.

## Rationale
The analyzer rule `asserts-own-setup` flagged a map or struct that starts as a literal. It did so even when a loop fills it from the output of the code under test. The first build of this cycle changed the syntax of about 40 tests to avoid the rule. That hides the false positive. The detector is the single place that decides, so the fix lives there and every future test benefits.

## Changed Areas
- `go/internal/testaudit/rules.go` — `dropEscaped` now drops a literal variable from the own-setup set. This happens when an assignment or an increment writes to it from a value that is not pure over literals. The new helper `dropProductionFilled` makes that decision for the right side and for the index key of the left side.

## Design Decisions
The rule reuses `pureOver`, the existing purity check. A literal on the right side (`m["a"] = 1`, `cfg.Name = "x"`, `m["a"]++`) keeps the variable as own setup. A test that only checks its own literals is still flagged. A call, a range variable or a selector on a non-literal root (`byName[it.name] = it.n`, `agg.total = sum(produce())`) drops the variable. The alternative was a syntax-level exemption for `make(map...)`. It was rejected because it leaves `map[k]v{}` flagged.

## Verification
ACS predicate 006 passes: production-filled containers are not flagged. Predicate 007 passes: literal-filled containers are still flagged. Predicate 003 passes: the analyzer still separates tautology from behavior. The package tests of testaudit and apicover pass.

## Compatibility
No exported symbol changes. The analyzer can only flag fewer tests; the 141-test snapshot stays at zero findings outside the protected carve-out.

## Limitations
The check `pureOver` is true only for a literal, an operator over pure parts, or a name rooted in a literal variable.
A call over only literals counts as production data. So do a composite literal (`m["a"] = []int{1}`), a named constant and a local scalar.
Each one drops the variable, so a rare tautology can pass.

The 16 findings in protected control-plane test files still need a console-owned item.
