# Comment history: `acs/cycle1248`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1248/predicates_test.go:3` — above `package cycle1248`

```text
// Package cycle1248 materialises the cycle-1248 acceptance criteria for the two
// fleet-scoped tasks pinned to this lane under inbox item
// tdd-structural-test-reachability-probe:
//
//   - reachability-ambiguous-package-resolution → cover resolvePackage's
//     multi-candidate branch (frozenpins.go:302-323)
//   - reachability-gate-docs                    → document the live frozen-pin
//     gate in the canonical gates table
//
// Why these two and not the headline item. The probe itself already shipped:
// CheckCallSite/BuildImportGraph (cycle-1226), FrozenTestFiles/ExtractFrozenPins/
// CheckFrozenPins (cycle-1238), wired into `evolve phase verify tdd` at
// go/internal/cli/phasecmd/phase_verify.go:142-146 with a permanent regression
// guard. What survives is one untested branch and one undocumented gate.
//
// The untested branch, precisely. resolvePackage's ALIAS path is covered
// (frozenpins_test.go:151-275). Its fallback — base-name matching across the
// import graph, scored by longest common prefix with the pinning package and
// tie-broken lexically — has ZERO coverage today: `grep -n "resolvePackage"
// *_test.go` matches only doc comments. That fallback is the multi-package-name
// collision variant of the very cycle-644 failure class the feature exists to
// catch, and Go map iteration order is randomised, so an untested tie-break is a
// latent nondeterminism.
//
// Predicate strategy — every predicate EXECUTES the system under test (the
// cycle-85 degenerate-predicate ban). 001/005/007 run real `go test`
// invocations, each scoped to ONE named package (never a `./...` sweep — the
// flaky-predicate-shape rule). 002/003/004 assert on the NAMES that appear in
// 001's live `-v` run output, so they are claims about tests that actually
// executed and passed, not about source text. 006 is the sole file-content
// assertion and is a documentation-presence criterion by nature; it is paired
// with 007, which proves the documented gate is live rather than vapour.
//
// Naming contract Builder inherits (restated in test-report.md's handoff): the
// new tests live in package reachabilityprobe (resolvePackage is unexported),
// are named with the prefix `TestResolvePackage`, and their subtest names carry
// the case markers 002/003/004 match on.
```
