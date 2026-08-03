# Cycle 1250 Dossier

**Goal:** 9d22f7f9398fa4e2fc8c2a77c8315a906816e20057ec26c8f437295cf59ca517
**Final verdict:** FAIL
**Run ID:** 01KZ3GHEZRWRZSJGE2YXQFGW28

## Phases

| Phase | Archetype | Verdict | Duration | Key Findings |
|-------|-----------|---------|----------|--------------|
| scout | plan | FAIL | 31s |  |
| retro | control | FAIL | 29s |  |

## Timing

**Total:** 59s across 2 phases (0 retried) · **Longest:** scout 31s

| Archetype | Wall-clock |
|-----------|------------|
| control | 29s |
| plan | 31s |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Failure

**Fingerprint:** `scout|infra-error|b590b53167d1` · **Class:** infra-error

- phase scout: scout: bridge: bridge: launch exit=10: [bridge] [claude-tmux] fleet mode: explicit worktree required (refusing process-cwd fallback)


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 1250

