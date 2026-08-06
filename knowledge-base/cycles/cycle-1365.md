# Cycle 1365 Dossier

**Goal:** 9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
**Final verdict:** FAIL
**Run ID:** 01KZA5EXFMRB6AGFP82CKEDTJP

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 2m27s |  |
| triage | plan | PASS | 53s |  |
| tdd | plan | PASS | 6m58s |  |
| build | build | PASS | 12m1s |  |
| cicd-pipeline-audit | evaluate | PASS | 1m11s |  |
| bug-reproduction | evaluate | PASS | 4m30s |  |
| audit | evaluate | WARN | 4m43s |  |
| ship | control | FAIL | 3s |  |
| ship | control | FAIL | 2s |  |
| ship | control | FAIL | 2s |  |
| retro | control | FAIL | 5m57s |  |

## Timing

**Total:** 38m47s across 11 phases (0 retried) · **Longest:** build 12m1s

| Archetype | Wall-clock |
|-----------|------------|
| build | 12m1s |
| control | 6m5s |
| evaluate | 10m24s |
| plan | 10m17s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|unknown|96f17cfe3dfe` · **Class:** unknown

- phase ship: ship: native: [GIT_STAGE_FAILED/transient @atomic-ship] ship: git add failed (rc=1): <nil>: The following paths are ignored by one of your .gitignore files: .evolve/evals hint: Use -f if y


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 1365

