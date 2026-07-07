# TDD Craft — tests that actually verify

> Load when: writing any test. Extends the Iron Laws with mechanics; evidence base: Meta's mutation-guided test generation (73% acceptance at scale), TDFlow/TDAD agentic-TDD studies, the 1.2M-commit mock-usage analysis (arXiv 2602.00409).

## Red-first mechanics (RIGID)

1. Write the test for the NEW behavior before any production line.
2. **Run it and watch it fail for the right reason** — an assertion failure describing the missing behavior. A compile error is not a valid red (fix the scaffolding until the assertion is what fails). A test that passes immediately tests nothing new — revise it until it fails.
3. Commit-or-checkpoint the failing test where the workflow allows; the red state is evidence you understood the problem before solving it.
4. Implement minimally; run again; the SAME test goes green with no edits to it. **The implementer never edits the test to reach green** — if the test was wrong, that's a separate, explicit change with its own justification.

## Bug fixes: the twin protocol (RIGID)

- **Regression test**: a test that fails on the pre-fix code, reproducing the bug exactly (same failure mode, not just same area). If you can't make it fail pre-fix, you haven't captured the bug.
- **Preservation tests**: identify the blast radius (what the fix must NOT change) and pin it with tests that pass BOTH pre- and post-fix. Agents measurably over-add guard clauses (~2× humans) — preservation tests are what catch a "fix" that quietly narrows behavior.
- Both directions of a changed predicate get pinned: the case that now passes AND the neighboring case that must still refuse (one-sided tests invite the inverse regression). Terminology note: `fable/references/verification.md` calls this both-directions pair "regression twins" — same rule, different label; this file reserves "twin protocol" for the regression+preservation pairing above.

## Test quality over coverage (RIGID mindset, FLEXIBLE tooling)

- The question is never "is the line covered" but **"would this test FAIL if the logic inverted?"** — 85% line coverage can hide a 57% mutation kill rate. When mutation tooling exists, use it on changed packages; when it doesn't, apply the mental mutation: flip the condition, off-by-one the boundary, drop the call — would a test notice?
- One behavior per test; the name states the contract (`TestSealCycle_DeadOwnerFreshLease_SealsWithoutForce`), so a failure reads as a spec violation without opening the file.
- Assert on **contracts and values** (decode JSON and compare; check returned structs), never on implementation echoes (internal call order, private state, log phrasing beyond load-bearing markers).
- Table-driven for input-space sweeps; property-based (rapid/quickcheck-style) for invariants ("never touches live dirs", "roundtrip is identity") where enumerating cases undersells the space.
- Legacy-compat: every schema/format extension gets a test that old artifacts still parse.

## Anti-mock discipline (RIGID)

- Mock **only process boundaries**: network, clock/time, subprocess execution, filesystem where a real tempdir won't do. Never mock your own package's logic; never mock a collaborator just to avoid constructing it.
- A test asserting only "the mock was called with X" verifies wiring, not behavior — acceptable ONLY for pure dispatch seams, and then say so in the test name.
- Prefer fakes with real behavior (in-memory store, recording ledger, fixture exec keyed by command) over expectation-style mocks; fakes survive refactors, mock expectations break on every internal change.
- Injected time (`Now func() time.Time` / clock param) beats sleeping; injected exec (`RunFunc`) beats PATH games. If the code under test can't take these, that's a design finding — fix the seam, don't mock around it.

## Hermeticity (RIGID)

Principle in `fable/references/verification.md` §Hermetic tests; the Go-concrete mechanics: repos created by tests get their own `git config user.name/user.email` (subprocess git never inherits your test-process env identity); env vars are set or explicitly UNSET per test (`t.Setenv`, `env -u` semantics — set-but-empty behaves differently from unset); a test that passes on your machine and fails on a runner is a hermeticity bug in the test, not "flaky CI".

## What NOT to write

- Assertion-free tests, tests of getters/setters, tests that re-state the implementation line-by-line.
- Sleeping/polling tests where an injected clock or channel exists.
- A second test suite for behavior already pinned — extend the existing table (search first; Iron Law 3 applies to tests too).
