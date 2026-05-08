---
description: Write RED tests first — the contract Builder must satisfy. Mandatory hop before /build.
---

# /tdd

Run the TDD-engineer phase against the current cycle. Writes failing tests that define the contract for Builder. Tests MUST be RED (fail on current code) — a green-from-start test is not a contract.

## When to use

- After `/scout` (or `/plan-review` returns PROCEED)
- Before any `/build` invocation
- When fixing a bug (write the regression test first)

## Execution

```bash
bash scripts/dispatch/subagent-run.sh tdd-engineer <cycle> <workspace>
```

## Output

- New/modified test files (Builder will read these)
- `<workspace>/tdd-contract.md` — the contract document
- `<workspace>/team-context.md` — `## TDD Contract` section populated

## See also

- `skills/evolve-tdd/SKILL.md`
- `agents/evolve-tdd-engineer.md`
- `.evolve/profiles/tdd-engineer.json`
