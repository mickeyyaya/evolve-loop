---
score_cap:
  - criterion: "test script exists"
    max_if_missing: 6
    evidence: "find scripts/tests/X-test.sh -type f"
  - criterion: "docs reference X"
    max_if_missing: 7
    evidence: "grep -q 'X' docs/README.md"
  - criterion: "regression suite green"
    max_if_missing: 5
    evidence: "bash scripts/tests/regression.sh"
---

# Eval: Example — Score Caps Reference

> Reference demonstrating three canonical `score_cap` patterns (c41, Ghosh Pattern #2).
> Replace placeholder paths and criteria with task-specific values in real evals.

## Score Cap Patterns

| Pattern | Criterion | max_if_missing | Evidence command |
|---|---|---|---|
| (a) test-file-must-exist | test script exists | 6/10 | `find scripts/tests/X-test.sh -type f` |
| (b) docs-must-mention-X | docs reference X | 7/10 | `grep -q 'X' docs/README.md` |
| (c) regression-suite-must-pass | regression suite green | 5/10 | `bash scripts/tests/regression.sh` |

## Interpretation

- `max_if_missing: 6` means: if the `evidence` command exits nonzero (requirement absent), the
  criterion score is capped at 6/10, regardless of Auditor prose-quality judgment.
- Multiple caps: the **lowest** `max_if_missing` wins.
- Caps override Auditor reasoning — deterministic structural gate, not advisory.
- `max_if_missing` integer scale: 1–10. Evidence exit 0 = present (no cap). Exit nonzero = absent (cap fires).

## Code Graders (bash commands that must exit 0)

- `grep -c "check_score_caps" scripts/verification/eval-quality-check.sh | awk '{exit ($1 < 1)}'`
- `grep -c "score_cap_enforcement" agents/evolve-auditor.md | awk '{exit ($1 < 1)}'`
