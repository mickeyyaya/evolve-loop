# Cycle 1295 Dossier

**Goal:** 9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
**Final verdict:** FAIL
**Run ID:** 01KZ6RQCFVEVCTBDC7EW24GPJ5

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 2m5s |  |
| triage | plan | PASS | 1m10s |  |
| fault-localization | plan | PASS | 7m15s |  |
| tdd | plan | PASS | 12m55s |  |
| build | build | PASS | 8m58s |  |
| adversarial-review | evaluate | PASS | 36s |  |
| bug-reproduction | evaluate | PASS | 3m1s |  |
| ci-parity-preflight | plan | FAIL | 29s |  |
| retro | control | FAIL | 5m55s |  |

## Timing

**Total:** 42m25s across 9 phases (0 retried) · **Longest:** tdd 12m55s

| Archetype | Wall-clock |
|-----------|------------|
| build | 8m58s |
| control | 5m55s |
| evaluate | 3m38s |
| plan | 23m54s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ci-parity-preflight|infra-error|b16c017532b1` · **Class:** infra-error

- phase ci-parity-preflight: ci-parity-preflight: bridge: bridge: launch exit=10: [bridge] no driver for cli=claude


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 1295

