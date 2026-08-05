# Cycle 1326 Dossier

**Goal:** 9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
**Final verdict:** FAIL
**Run ID:** 01KZ8F4E87XAFDJP31X2Y8G38T

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | PASS | 2m36s |  |
| triage | plan | PASS | 58s |  |
| tdd | plan | PASS | 2m54s |  |
| build | build | PASS | 3m2s |  |
| audit | evaluate | PASS | 2m34s |  |
| ship | control | FAIL |  |  |
| audit | evaluate | PASS | 38s |  |
| ship | control | FAIL |  |  |
| audit | evaluate | PASS | 38s |  |
| ship | control | FAIL |  |  |
| retro | control | FAIL | 6m16s |  |

## Timing

**Total:** 19m34s across 11 phases (0 retried) · **Longest:** retro 6m16s

| Archetype | Wall-clock |
|-----------|------------|
| build | 3m2s |
| control | 6m16s |
| evaluate | 3m49s |
| plan | 6m28s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `ship|unknown|76d0f4fca190` · **Class:** unknown

- phase ship: ship repo-contract gate: [REPO_CONTRACT_GATE/precondition @atomic-ship] repo-contract scanner pack RED in the lane worktree (exit status 1) — pushing would red main; fix the violation in


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 1326

