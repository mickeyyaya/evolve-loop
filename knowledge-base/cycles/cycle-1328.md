# Cycle 1328 Dossier

**Goal:** 9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
**Final verdict:** FAIL
**Run ID:** 01KZ8GXY4RJCVVR6SKWZ2S09QQ

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 2m55s |  |
| triage | plan | PASS | 1m4s |  |
| tdd | plan | PASS | 5m52s |  |
| build | build | PASS | 3m32s |  |
| audit | evaluate | PASS | 3m26s |  |
| ship | control | FAIL |  |  |
| audit | evaluate | PASS | 38s |  |
| ship | control | FAIL |  |  |
| audit | evaluate | PASS | 37s |  |
| ship | control | FAIL |  |  |
| retro | control | FAIL | 5m42s |  |

## Timing

**Total:** 23m45s across 11 phases (0 retried) · **Longest:** tdd 5m52s

| Archetype | Wall-clock |
|-----------|------------|
| build | 3m32s |
| control | 5m42s |
| evaluate | 4m41s |
| plan | 9m50s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|unknown|76d0f4fca190` · **Class:** unknown

- phase ship: ship repo-contract gate: [REPO_CONTRACT_GATE/precondition @atomic-ship] repo-contract scanner pack RED in the lane worktree (exit status 1) — pushing would red main; fix the violation in


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 1328

