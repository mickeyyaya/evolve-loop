# Cycle 1284 Dossier

**Goal:** 9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
**Final verdict:** FAIL
**Run ID:** 01KZ5Y8GHG5EA2PPG7BWZZF1PA

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 2m23s |  |
| triage | plan | PASS | 1m1s |  |
| fault-localization | plan | PASS | 2m27s |  |
| tdd | plan | PASS | 6m34s |  |
| build | build | PASS | 5m45s |  |
| coverage-gate | evaluate | PASS | 2m23s |  |
| adversarial-review | evaluate | PASS | 5m22s |  |
| bug-reproduction | evaluate | PASS | 2m53s |  |
| audit | evaluate | WARN | 5m11s |  |
| ship | control | FAIL | 1s |  |
| ship | control | FAIL | 0s |  |
| ship | control | FAIL | 1s |  |
| retro | control | FAIL | 9m36s |  |

## Timing

**Total:** 43m35s across 13 phases (0 retried) · **Longest:** retro 9m36s

| Archetype | Wall-clock |
|-----------|------------|
| build | 5m45s |
| control | 9m37s |
| evaluate | 15m49s |
| plan | 12m24s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|unknown|96f17cfe3dfe` · **Class:** unknown

- phase ship: ship: native: [GIT_STAGE_FAILED/transient @atomic-ship] ship: git add failed (rc=1): <nil>: The following paths are ignored by one of your .gitignore files: .evolve/evals hint: Use -f if y


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 1284

