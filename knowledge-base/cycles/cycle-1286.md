# Cycle 1286 Dossier

**Goal:** 9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
**Final verdict:** FAIL
**Run ID:** 01KZ62R4Q9EGBT7EAJBMXZP8ER

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 2m35s |  |
| triage | plan | PASS | 50s |  |
| fault-localization | plan | PASS | 5m56s |  |
| tdd | plan | PASS | 11m39s |  |
| build | build | PASS | 4m52s |  |
| coverage-gate | evaluate | PASS | 2m3s |  |
| adversarial-review | evaluate | PASS | 3m38s |  |
| bug-reproduction | evaluate | PASS | 4m14s |  |
| audit | evaluate | WARN | 3m50s |  |
| ship | control | FAIL | 1s |  |
| ship | control | FAIL | 1s |  |
| ship | control | FAIL | 1s |  |
| retro | control | FAIL | 6m24s |  |

## Timing

**Total:** 46m4s across 13 phases (0 retried) · **Longest:** tdd 11m39s

| Archetype | Wall-clock |
|-----------|------------|
| build | 4m52s |
| control | 6m26s |
| evaluate | 13m46s |
| plan | 21m0s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|unknown|96f17cfe3dfe` · **Class:** unknown

- phase ship: ship: native: [GIT_STAGE_FAILED/transient @atomic-ship] ship: git add failed (rc=1): <nil>: The following paths are ignored by one of your .gitignore files: .evolve/evals hint: Use -f if y


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 1286

