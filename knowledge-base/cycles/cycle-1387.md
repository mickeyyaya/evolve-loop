# Cycle 1387 Dossier

**Goal:** 9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
**Final verdict:** FAIL
**Run ID:** 01KZB2TS0YMKJVR7V3CJXATDHS

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 1m47s |  |
| triage | plan | PASS | 1m3s |  |
| tdd | plan | PASS | 9m49s |  |
| build | build | PASS | 10m35s |  |
| audit | evaluate | WARN | 4m5s |  |
| ship | control | FAIL |  |  |
| audit | evaluate | WARN | 38s |  |
| ship | control | FAIL |  |  |
| audit | evaluate | WARN | 36s |  |
| ship | control | FAIL |  |  |
| retro | control | FAIL | 5m22s |  |

## Timing

**Total:** 33m56s across 11 phases (0 retried) · **Longest:** build 10m35s

| Archetype | Wall-clock |
|-----------|------------|
| build | 10m35s |
| control | 5m22s |
| evaluate | 5m20s |
| plan | 12m40s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|gate-block|cd49274beab2` · **Class:** gate-block

- phase ship: ship repo-contract gate: [REPO_CONTRACT_GATE/precondition @atomic-ship] repo-contract scanner pack RED in the lane worktree (exit status 1) — pushing would red main; fix the violation in


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 1387

