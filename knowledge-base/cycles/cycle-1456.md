# Cycle 1456 Dossier

**Goal:** Work through the todo inbox by weight; pipeline-repair items first.
**Final verdict:** FAIL
**Run ID:** 01M001C33GGMXARC8FTXPD1QS5

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 2m25s |  |
| triage | plan | PASS | 45s |  |
| fault-localization | plan | PASS | 1m36s |  |
| tdd | plan | PASS | 3m51s |  |
| build | build | PASS | 3m55s |  |
| test-amplification | evaluate | PASS | 10m51s |  |
| security-scan | evaluate | PASS | 1m17s |  |
| bug-reproduction | evaluate | PASS | 2m37s |  |
| audit | evaluate | PASS | 3m45s |  |
| ship | control | FAIL | 6s |  |
| ship | control | FAIL | 6s |  |
| audit | evaluate | PASS | 27s |  |
| ship | control | FAIL | 5s |  |
| retro | control | FAIL | 7m2s |  |

## Timing

**Total:** 38m49s across 14 phases (0 retried) · **Longest:** test-amplification 10m51s

| Archetype | Wall-clock |
|-----------|------------|
| build | 3m55s |
| control | 7m20s |
| evaluate | 18m57s |
| plan | 8m37s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|unknown|99c38818197c` · **Class:** unknown

- phase ship: ship: native: [GIT_STAGE_FAILED/precondition @atomic-ship] ship: git add failed (rc=1): <nil>: The following paths are ignored by one of your .gitignore files: .evolve/inbox/processed hint


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 1456

