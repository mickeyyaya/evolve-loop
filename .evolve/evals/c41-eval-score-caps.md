---
score_cap:
  - criterion: "test suite passes (score_cap enforcement verified)"
    max_if_missing: 7
    evidence: "bash scripts/tests/eval-score-caps-test.sh"
---

# Eval: c41-eval-score-caps

> Acceptance criteria for c41: deterministic score caps in eval YAML frontmatter (Ghosh Pattern #2).

## Code Graders (bash commands that must exit 0)

- `bash scripts/tests/eval-score-caps-test.sh | grep -q "5/5 PASS"`

## Acceptance Criteria

| # | Criterion | Verification |
|---|---|---|
| 1 | score_cap parser in eval-quality-check.sh | `grep -c "check_score_caps" scripts/verification/eval-quality-check.sh` ≥ 1 |
| 2 | score_caps_ceiling in JSON output | `grep -c "SCORE_CAPS_CEILING" scripts/verification/eval-quality-check.sh` ≥ 1 |
| 3 | Cap fires on synthetic eval + caps_fired:1 in output | `bash scripts/tests/eval-score-caps-test.sh` exits 0 |
| 4 | D.score_cap_enforcement in evolve-auditor.md | `grep -c "score_cap_enforcement" agents/evolve-auditor.md` ≥ 1 |
| 5 | Reference eval exists | `test -s .evolve/evals/_example-score-caps.md` |
| 6 | No existing evals broken | eval-quality-check.sh exits 0 on directory scan |
| 7 | This eval file exists | `test -f .evolve/evals/c41-eval-score-caps.md` |

## Thresholds
- All checks: pass@1 = 1.0
