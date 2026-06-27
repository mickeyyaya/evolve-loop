# Cycle 392 Dossier

**Goal:** Optimize token usage across the pipeline and each phase. Make ONE small, fully-wired token reduction and ship it: pick a single agent prompt file under agents/*.md and remove redundant, verbose, or duplicated instructional text while preserving all behavior and required sections. HARD constraints: (1) do NOT add any policy/config/option/flag surface — inert config with no production callers FAILs audit (cycle-382 lesson); (2) do NOT edit control-plane surfaces — go/internal/acssuite, guards, .evolve/profiles, flagregistry, kernel hooks, .github (cycle-383 lesson); (3) keep the diff to 1-3 files, agent markdown only; (4) the reduction must be fully realized in the committed file this cycle, measured as fewer tokens/lines, not a scaffold.
**Final verdict:** FAIL
**Run ID:** 01KVYNS6NWQV5JH2YN03SXR82K

## Phases

| Phase | Verdict | Key Findings |
|-------|---------|--------------|
| cycle-recorded | FAIL | cycle completed; ledger walk deferred to future slice |

## Defects

- **audit-fail** (HIGH): cycle did not pass audit; see audit-report.md + acs-verdict.json — fix: address the audit findings recorded for this cycle


## Carryover

- **address-audit-findings** (high): resolve the audit findings that failed cycle 392

