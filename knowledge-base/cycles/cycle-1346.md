# Cycle 1346 Dossier

**Goal:** 9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
**Final verdict:** FAIL
**Run ID:** 01KZ9AS08PQFRE5T7GRB36SQT8

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 5m3s |  |
| triage | plan | PASS | 59s |  |
| tdd | plan | PASS | 2m57s |  |
| build | build | PASS | 4m6s |  |
| audit | evaluate | PASS | 5m18s |  |
| ship | control | FAIL | 3s |  |
| ship | control | FAIL | 2s |  |
| ship | control | FAIL | 2s |  |
| retro | control | FAIL | 6m29s |  |

## Timing

**Total:** 25m0s across 9 phases (0 retried) · **Longest:** retro 6m29s

| Archetype | Wall-clock |
|-----------|------------|
| build | 4m6s |
| control | 6m36s |
| evaluate | 5m18s |
| plan | 9m0s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|unknown|96f17cfe3dfe` · **Class:** unknown

- phase ship: ship: native: [GIT_STAGE_FAILED/transient @atomic-ship] ship: git add failed (rc=1): <nil>: The following paths are ignored by one of your .gitignore files: .evolve/evals hint: Use -f if y


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 1346

