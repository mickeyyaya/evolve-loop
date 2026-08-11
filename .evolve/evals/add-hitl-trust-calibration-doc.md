# Eval: add-hitl-trust-calibration-doc

> Graders for the HITL trust calibration documentation task.

## Graders

| # | Type | Command / Check | Expected |
|---|------|----------------|----------|
| 1 | bash | `test -f docs/hitl-trust-calibration.md` | exit 0 |
| 2 | bash | `grep -q "confidence.threshold\|confidence threshold\|Confidence Threshold" docs/hitl-trust-calibration.md` | exit 0 |
| 3 | bash | `grep -q "trust.calibration\|trust calibration\|Trust Calibration" docs/hitl-trust-calibration.md` | exit 0 |
| 4 | bash | `grep -q "handoff\|Handoff\|escalation\|Escalation" docs/hitl-trust-calibration.md` | exit 0 |
| 5 | bash | `grep -q "fallback\|Fallback\|multi-tier" docs/hitl-trust-calibration.md` | exit 0 |
| 6 | bash | `grep -q "HITL\|human-in-the-loop\|Human-in-the-Loop" docs/hitl-trust-calibration.md` | exit 0 |
| 7 | bash | `wc -l < docs/hitl-trust-calibration.md \| awk '{exit ($1 > 400)}'` | exit 0 — under 400 lines |
| 8 | bash | `grep -c "^|" docs/hitl-trust-calibration.md \| awk '{exit ($1 < 5)}'` | exit 0 — at least 5 table rows |
| 9 | bash | `grep -q "anti-pattern\|Anti-pattern\|ANTI-PATTERN" docs/hitl-trust-calibration.md` | exit 0 |
| 10 | bash | `grep -q "Scout\|Builder\|Auditor\|phase-gate" docs/hitl-trust-calibration.md` | exit 0 — maps to evolve-loop agents |
