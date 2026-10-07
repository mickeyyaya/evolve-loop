# code-review-simplify — COMPACT projection (the Self-review hook)

> This is the whole preloaded load for a source-writing phase; the full skill (pipeline, scoring math, report schema) is `skills/code-review-simplify/SKILL.md`, read only when you need it. Do not write the full report: record only the block in step 6.

**Self-review = your own pass over your own files. REQUIRED on a code cycle, never a gate.**

0. Before your first edit, save `git status --porcelain` to your workspace (never the worktree).
1. Scope: only your own files, the ones that changed after you started. A path already in the saved status is earlier-phase-owned unless your dispatch names it as yours to write (a debugger's conflicted paths): read it, never edit it, record a finding there as `Declined: earlier-phase-owned`. Examples: `test-report.md`'s `testFiles`, `go/acs/` predicates, the bug-reproduction reproducer. The TDD phase never edits production code. Run `git add -N <new paths>` first: `git diff HEAD` hides new files.
2. Tier by the size of your change: under 50 lines and 1-3 files is lightweight; 50-200 lines or 3-10 files is standard; over 200 lines, 10+ files or a security-sensitive path (auth, login, password, token, secret, payment, billing, checkout, eval, grader, agents/, skills/) is full. Full runs its correctness, security and performance lenses in turn, in this agent: dispatch no subagents.
3. Score correctness, security, performance and maintainability from 0.0 to 1.0; composite = 0.35 correctness + 0.25 security + 0.15 performance + 0.25 maintainability. Each of these lowers a score: a function over 50 lines, cognitive complexity over 15, nesting over 4, a file over 800 lines, a near-duplicate block over 6 lines, a hardcoded secret, an injection path. Your own changed Go `*_test.go` files also get the `golang-test-review` checklist (`skills/golang-test-review/SKILL.md`).
4. Apply only behaviour-preserving simplifications (extract, flatten nesting, remove dead code, rename). Never trim input validation, error handling, security checks or tests; never weaken a test. Never add a comment: a better name, an extracted function, a type or a test says it. Fix every CRITICAL or HIGH finding in your files, or decline it with a reason.
5. Re-run your phase's own check (the build's tests, or the TDD phase's RED run). Revert a simplification that breaks it.
6. Record in your report:

```markdown
## Self-Review
- Skill: code-review-simplify, tier <lightweight|standard|full> (<N> files, <M> lines)
- Scores: composite <0.NN>, correctness <0.NN>, security <0.NN>, performance <0.NN>, maintainability <0.NN>
- Applied: <technique at file:line, ...> | none
- Declined: <finding: reason, ...; Declined: earlier-phase-owned for an earlier phase's file> | none
- Go tests: golang-test-review applied to <your test files> | no *_test.go change
- Re-verified: <command> -> <N/N PASS, or the RED run>
```

The scores never block your handoff. A report without this block, or without its `- Scores:` line, still ships; the runner records the advisory `RUNNER_SELF_REVIEW_MISSING`. A `code-review` phase that loads this skill in review mode reviews independently and edits nothing; this hook is the writer's alone.
