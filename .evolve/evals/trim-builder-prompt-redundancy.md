# Eval: Trim redundant/duplicated prose in evolve-builder.md (token reduction)

Goal: a behavior-preserving token reduction of the Builder agent prompt. Remove
redundant, verbose, or duplicated instructional text (e.g. the multiply-restated
"at turn 18/20+, write the report and stop" rule, and repeated
"NEVER claim Status: PASS from self-assessment alone" anecdote restatements)
WITHOUT dropping any required section header or any behavior-bearing rule.

Baseline (committed 5abc9166): 288 lines, 2835 words, 21994 bytes.

## Code Graders (bash commands that must exit 0)
- `[ $(wc -l < agents/evolve-builder.md) -lt 280 ]`
- `[ $(wc -w < agents/evolve-builder.md) -lt 2780 ]`
- `[ $(wc -c < agents/evolve-builder.md) -lt 21994 ]`
- `grep -q '^name: evolve-builder$' agents/evolve-builder.md`
- `grep -q '^## STOP CRITERION$' agents/evolve-builder.md`

## Regression Evals (required sections + behavior-bearing rules preserved)
- `grep -q '^## Inputs$' agents/evolve-builder.md`
- `grep -q '^## Strategy Handling$' agents/evolve-builder.md`
- `grep -q '^## Core Principles$' agents/evolve-builder.md`
- `grep -q '^## Worktree Isolation$' agents/evolve-builder.md`
- `grep -q '^## Turn budget$' agents/evolve-builder.md`
- `grep -q '^## Shared Constraints$' agents/evolve-builder.md`
- `grep -q '^## Workflow$' agents/evolve-builder.md`
- `grep -q '^## Reference Index' agents/evolve-builder.md`
- `grep -q '^## AC-TABLE Region' agents/evolve-builder.md`
- `grep -q '^## Pre-handoff Regression Slice' agents/evolve-builder.md`
- `grep -q '^## Pre-handoff Git Tracking Attestation' agents/evolve-builder.md`
- `grep -q '^## EGPS Predicate Authoring$' agents/evolve-builder.md`
- `grep -q '^## Output$' agents/evolve-builder.md`
- `grep -q '^## POSTHOC enforcement' agents/evolve-builder.md`
- `grep -q '^## Reflection Authoring' agents/evolve-builder.md`
- `grep -q 'Completion Gates' agents/evolve-builder.md`
- `grep -q 'report-written' agents/evolve-builder.md`
- `grep -q 'Builder MUST NOT write or modify ACS predicates' agents/evolve-builder.md`
- `grep -q 'AC-TABLE-BEGIN' agents/evolve-builder.md`
- `grep -q 'POSTHOC' agents/evolve-builder.md`
- `grep -q 'reflection-authoring-step.md' agents/evolve-builder.md`

## Acceptance Checks (verification commands)
- `[ -s agents/evolve-builder.md ]`
- `git diff --stat -- agents/evolve-builder.md | grep -q 'evolve-builder.md'`

## Negative / Anti-gaming
- Deleting a whole required section to win the line-count check FAILs: each Regression Eval asserts a distinct required `## ` header still exists.
- Reflowing/joining lines to drop line count while adding words FAILs: `wc -w` and `wc -c` both require strict decreases, so pure reformat without removal does not pass.

## Thresholds
- All Code Graders + Regression Evals: pass@1 = 1.0
