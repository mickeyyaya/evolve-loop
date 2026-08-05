# Cycle 1348 Dossier

**Goal:** 9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
**Final verdict:** FAIL
**Run ID:** 01KZ9CXCGR55W4TR8QPWGKY10F

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 3m29s |  |
| triage | plan | PASS | 1m7s |  |
| tdd | plan | PASS | 4m12s |  |
| build | build | PASS | 2m48s |  |
| audit | evaluate | WARN | 3m42s |  |
| ship | control | FAIL | 3s |  |
| audit | evaluate | WARN | 41s |  |
| ship | control | FAIL | 2s |  |
| ship | control | FAIL | 2s |  |
| retro | control | FAIL | 9m55s |  |

## Timing

**Total:** 26m0s across 10 phases (0 retried) · **Longest:** retro 9m55s

| Archetype | Wall-clock |
|-----------|------------|
| build | 2m48s |
| control | 10m2s |
| evaluate | 4m23s |
| plan | 8m48s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|unknown|96f17cfe3dfe` · **Class:** unknown

- phase ship: ship: native: [GIT_STAGE_FAILED/transient @atomic-ship] ship: git add failed (rc=1): <nil>: The following paths are ignored by one of your .gitignore files: .evolve/evals hint: Use -f if y


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 1348

