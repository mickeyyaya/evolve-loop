# Cycle 1465 Dossier

**Goal:** Work through the todo inbox by weight; pipeline-repair items first.
**Final verdict:** FAIL
**Run ID:** 01M00GR30DNFHRQRN99YX5F6QT

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 8m56s |  |
| triage | plan | PASS | 56s |  |
| fault-localization | plan | PASS | 3m35s |  |
| tdd | plan | PASS | 13m22s |  |
| build | build | PASS | 34m32s |  |
| error-handling-scan | evaluate | PASS | 5m15s |  |
| coverage-gate | evaluate | PASS | 8m50s |  |
| adversarial-review | evaluate | PASS | 4m6s |  |
| bug-reproduction | evaluate | PASS | 5m23s |  |
| audit | evaluate | WARN | 5m52s |  |
| ship | control | FAIL | 6s |  |
| ship | control | FAIL | 6s |  |
| audit | evaluate | WARN | 28s |  |
| ship | control | FAIL | 6s |  |
| retro | control | FAIL | 7m39s |  |

## Timing

**Total:** 1h39m11s across 15 phases (0 retried) · **Longest:** build 34m32s

| Archetype | Wall-clock |
|-----------|------------|
| build | 34m32s |
| control | 7m57s |
| evaluate | 29m53s |
| plan | 26m49s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|unknown|99c38818197c` · **Class:** unknown

- phase ship: ship: native: [GIT_STAGE_FAILED/precondition @atomic-ship] ship: git add failed (rc=1): <nil>: The following paths are ignored by one of your .gitignore files: .evolve/inbox/processed hint


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 1465

