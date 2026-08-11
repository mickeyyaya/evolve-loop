# Eval: add-code-correctness-verification-doc

> Graders for the code correctness verification documentation task.

## Graders

| # | Type | Command / Check | Expected |
|---|------|----------------|----------|
| 1 | bash | `test -f docs/code-correctness-verification.md` | exit 0 |
| 2 | bash | `grep -q "property-based\|property based\|PropertyGPT" docs/code-correctness-verification.md` | exit 0 |
| 3 | bash | `grep -q "formal\|Formal\|verification" docs/code-correctness-verification.md` | exit 0 |
| 4 | bash | `grep -q "specification\|invariant\|precondition\|postcondition" docs/code-correctness-verification.md` | exit 0 |
| 5 | bash | `grep -q "unit test\|unit-test\|test coverage" docs/code-correctness-verification.md` | exit 0 |
| 6 | bash | `grep -q "AI-generated\|LLM-generated\|generated code" docs/code-correctness-verification.md` | exit 0 |
| 7 | bash | `wc -l < docs/code-correctness-verification.md \| awk '{exit ($1 > 400)}'` | exit 0 — under 400 lines |
| 8 | bash | `grep -c "^|" docs/code-correctness-verification.md \| awk '{exit ($1 < 5)}'` | exit 0 — at least 5 table rows |
| 9 | bash | `grep -q "anti-pattern\|Anti-pattern\|ANTI-PATTERN" docs/code-correctness-verification.md` | exit 0 |
| 10 | bash | `grep -q "Scout\|Builder\|Auditor" docs/code-correctness-verification.md` | exit 0 — maps to evolve-loop agents |
