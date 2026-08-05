# Cycle 1329 Dossier

**Goal:** 9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
**Final verdict:** FAIL
**Run ID:** 01KZ8GXY5C7V8XBP1ZWAF0VRVM

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 2m12s |  |
| triage | plan | PASS | 1m3s |  |
| fault-localization | plan | PASS | 1m37s |  |
| tdd | plan | PASS | 6m19s |  |
| build | build | PASS | 5m40s |  |
| bug-reproduction | evaluate | PASS | 5m14s |  |
| audit | evaluate | PASS | 7m9s |  |
| ship | control | FAIL |  |  |
| audit | evaluate | PASS | 37s |  |
| ship | control | FAIL |  |  |
| audit | evaluate | PASS | 37s |  |
| ship | control | FAIL |  |  |
| retro | control | FAIL | 6m3s |  |

## Timing

**Total:** 36m29s across 13 phases (0 retried) · **Longest:** audit 7m9s

| Archetype | Wall-clock |
|-----------|------------|
| build | 5m40s |
| control | 6m3s |
| evaluate | 13m37s |
| plan | 11m9s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|unknown|76d0f4fca190` · **Class:** unknown

- phase ship: ship repo-contract gate: [REPO_CONTRACT_GATE/precondition @atomic-ship] repo-contract scanner pack RED in the lane worktree (exit status 1) — pushing would red main; fix the violation in


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 1329

