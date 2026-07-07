---
name: engineering-craft
description: Use when writing or changing any production code or tests — new features, bug fixes, refactors, or test authoring, in any language, by any model on any CLI.
---

# Engineering Craft — how code gets written here

> The craft companion to `fable-mode` (which governs how you *operate*; this governs the code you *produce*). Content is distilled from 2025-26 measured evidence — mutation-testing studies, million-commit analyses of AI-written code, and benchmarked skill collections — not textbook restatement. Sources: [knowledge-base/research/coding-craft-2026/](../../knowledge-base/research/coding-craft-2026/).

## Iron Laws (RIGID — no exceptions, no negotiation)

1. **No production code without a failing test first.** Watch it fail, for the RIGHT reason (assertion failure on the missing behavior — not a compile error, not a typo). If code came first anyway: delete it, write the test, start over. Yes, really.
2. **Completion = machine evidence.** Every task ends with a runnable check (test run, build, diff) whose real output you show. "Should work" is not a state of code.
3. **Search before you write.** Before adding any function/type/helper: search for the existing one. Duplication is the measured #1 AI-code pathology (+81% duplicated blocks since 2023). Extending beats re-creating; consolidating beats both.
4. **Never weaken a test to make it pass.** Tests encode intent. If a test is wrong, fix it in a commit that says so; if it's right, fix the code. Deleting/skipping/loosening assertions to go green is falsifying evidence.
5. **The diff you ship is the smallest correct one.** No drive-by refactors, no unrequested "improvements", no formatting churn outside touched lines. File separate work separately.

## Rationalization table (RIGID — if you think it, stop)

| The thought | The reality |
|---|---|
| "This is too simple to need a test" | Simple code breaks too; the test costs 2 minutes and is the only proof you ran it. |
| "I'll add tests after it works" | After-tests pass by construction and verify nothing. Red-first or start over. |
| "A mock is easier here" | Agents over-mock measurably (mocks in 36% of agent commits touching tests vs 26% of non-agent commits). Mock only process boundaries (network, clock, subprocess) — never your own logic. |
| "I'll extract an interface for testability" | Interfaces-for-mocking is inverted design. Consumer-side interfaces, only when ≥2 real implementations exist. (Different threshold from rule-of-three, which governs code duplication.) |
| "While I'm here, I'll clean this up" | Scope creep buries the actual change in review noise. Note it, file it, move on. |
| "Coverage is high, tests are good" | 85% coverage can hide a 57% mutation kill rate. Ask: would this test FAIL if the logic inverted? |
| "This helper probably doesn't exist yet" | Search first (Iron Law 3). The codebase is bigger than your context. |
| "The linter/reviewer is wrong, I'll ignore it" | Decline visibly with a reason, or comply. Silent disagreement re-litigates forever. |

## The write-code loop (RIGID)

1. **Scope contract**: one sentence of goal + explicit non-goals; identify the blast radius (what must NOT change behavior).
2. **RED**: failing test(s) — the new behavior's test + for bug fixes a *regression test* (fails pre-fix) AND *preservation tests* (pass pre-fix, must stay green).
3. **GREEN**: minimal implementation. Match surrounding conventions (naming, error style, comment density) even where you'd choose differently.
4. **REFACTOR**: only with green tests, only within scope; consolidate any duplication you introduced.
5. **EVIDENCE**: run the check, show output as `N/N PASS, no regression`; verify the diff against the scope contract (nothing outside it).

## Red flags in code you just wrote (RIGID)

- A test with no assertion, or asserting only that a mock was called.
- A new `if err != nil { return nil }` (swallowed error) or empty catch.
- A guard clause for a condition nothing can produce (agents add ~2× unnecessary guards — every guard needs a caller that triggers it or a test that proves the boundary).
- A comment narrating what the next line does (comments answer *why*; the code answers *what*).
- Two blocks that differ by a variable name (extract, or if <3 occurrences, deliberately leave and note — rule-of-three).
- An exported symbol nothing outside the package calls.

## References (progressive disclosure — load what the task needs)

| Reference | Load when |
|---|---|
| [references/tdd-craft.md](references/tdd-craft.md) | writing any test — red-first mechanics, test-quality-over-coverage, anti-mock discipline, regression twins, property-based and table-driven patterns |
| [references/clean-code.md](references/clean-code.md) | writing/reviewing production code — the empirically-surviving rules, duplication discipline, reviewability budgets, comment policy, error handling |
| [references/design-patterns.md](references/design-patterns.md) | structuring components — DI/Strategy/Specification/ports-adapters selection, misuse table, Go interface discipline, when NOT to abstract |

(Flat-namespace installs — e.g. the codex target — project only SKILL.md; the reference files are absent.)

## Works with (don't duplicate these)

- `minimalism` — the laziest-correct-solution ladder; invoke alongside this skill for green-field code.
- `refactor` / `code-review-simplify` — orchestrate refactoring and review sessions; this skill sets the standards they check against.
- `golang-test-review` — Go-specific test review depth.
- `fable-mode` (references/verification.md) — the operating-level completion/verification protocol this skill's Iron Law 2 instantiates for code.

## Precedence

User/project instructions (CLAUDE.md, AGENTS.md) outrank this skill where they conflict; this skill outranks default model behavior. RIGID sections are non-negotiable; everything else adapts to the codebase's existing conventions — convention-matching itself being one of the RIGID expectations.
