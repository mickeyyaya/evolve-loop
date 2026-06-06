---
score_cap:
  - criterion: "All 15 domain phase rows present in the Phase Catalog — Core Values table"
    max_if_missing: 5
    evidence: "test \"$(grep -cE '^\\| *`(risk-register|scope-baseline|dependency-map|forces-analysis|market-sizing|okr-draft|account-reconcile|variance-analysis|close-checklist|opportunity-map|prd-draft|metric-tree|incident-postmortem|runbook-draft|capacity-plan)` *\\|' agents/evolve-router.md)\" -ge 15"
  - criterion: "Core Values table totals ≥29 phase rows (14 original + 15 domain)"
    max_if_missing: 6
    evidence: "test \"$(grep -cE '^\\| *`[a-z][a-z-]+` *\\|' agents/evolve-router.md)\" -ge 29"
  - criterion: "Integration wave row flipped to ✅ done (cycle 12) in domain-phase-catalog.md §5"
    max_if_missing: 6
    evidence: "grep -E '^\\| *Integration *\\|' docs/architecture/domain-phase-catalog.md | grep '✅' | grep -q 'cycle 12'"
  - criterion: "Integration row carries no stale ⬜ queued / cycle-6 reference"
    max_if_missing: 7
    evidence: "! grep -E '^\\| *Integration *\\|' docs/architecture/domain-phase-catalog.md | grep -qE '⬜|cycle 6'"
---

# Eval: Extend Phase Catalog Core Values with 15 domain phases + Integration status flip

> Pins the core-values half of the domain-phase-catalog integration wave
> (spec §3 + §4.1 note), shipped in cycle 12: the 15 domain phases
> (PM: risk-register/scope-baseline/dependency-map; Strategy:
> forces-analysis/market-sizing/okr-draft; Accounting:
> account-reconcile/variance-analysis/close-checklist; Product:
> opportunity-map/prd-draft/metric-tree; Ops:
> incident-postmortem/runbook-draft/capacity-plan) appended to the Phase
> Catalog — Core Values table in `agents/evolve-router.md`, and the
> Integration row in `docs/architecture/domain-phase-catalog.md` §5 flipped
> from `⬜ queued (cycle 6)` to `✅ done (cycle 12)`. The core-value
> one-liners are the advisor's selection criterion ("justify against the
> phase's CORE VALUE — the one risk it removes"); a domain phase absent from
> this table is unselectable in practice. The negative criterion (no stale
> queued reference) is the anti-no-op signal: a build that appends rows but
> forgets the flip stays capped. Source incident: cycle 11 implemented
> identical content with ACS 79/79 green but FAILED audit on a missing
> challenge-token header in build-report.md, so nothing was committed;
> cycle 12 is the sanctioned re-ship. Eval authored per the cycle-131 lesson
> (missing `.evolve/evals/<slug>.md` = automatic CRITICAL FAIL at audit).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| domain-rows | 15 domain phase rows present | 5/10 | backticked-row count over the 15 phase names |
| table-total | ≥29 phase rows total | 6/10 | backticked-row count ≥29 |
| status-flip | Integration ✅ done (cycle 12) | 6/10 | Integration row grep for ✅ + cycle 12 |
| stale-negative | no ⬜ / cycle-6 residue on Integration row | 7/10 | negated grep on the Integration row |

All evidence commands are doc-presence checks: the core-values table and
wave-status table are operator guidance, not a subprocess-emitting system —
`# acs-predicate: config-check` waiver applies (cycles 2, 4, 5, 6, 9 lesson;
ACS predicates `acs/cycle-12/006–008, 010` carry the waiver inline).

## Acceptance Criteria (cycle-12 ACS mapping)

| AC | Grader | ACS predicate |
|---|---|---|
| AC-1 all 15 domain rows present | [code] | `acs/cycle-12/006-core-values-15-domain-rows.sh` |
| AC-2 Integration flipped ✅ done | [code] | `acs/cycle-12/007-integration-row-flipped-done.sh` |
| AC-3 table totals ≥29 rows | [code] | `acs/cycle-12/008-core-values-count-ge-29.sh` |
| AC-4 scope guard: no Go changes | [code] | `acs/cycle-12/009-scope-guard-t2-no-go-changes.sh` |
| AC-5 negative: no stale queued reference | [code] | `acs/cycle-12/010-negative-integration-not-stale-queued.sh` |
