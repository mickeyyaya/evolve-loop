---
score_cap:
  - criterion: "All 5 domain goal-type rows present in the Goal-Type Recipes table"
    max_if_missing: 5
    evidence: "grep -qE '^\\| *project-management *\\|' agents/evolve-router.md && grep -qE '^\\| *business-strategy *\\|' agents/evolve-router.md && grep -qE '^\\| *accounting-close *\\|' agents/evolve-router.md && grep -qE '^\\| *product-discovery *\\|' agents/evolve-router.md && grep -qE '^\\| *ops-incident *\\|' agents/evolve-router.md"
  - criterion: "project-management recipe carries risk-register → scope-baseline → dependency-map"
    max_if_missing: 6
    evidence: "grep -E '^\\| *project-management *\\|' agents/evolve-router.md | grep risk-register | grep scope-baseline | grep -q dependency-map"
  - criterion: "business-strategy recipe carries forces-analysis → market-sizing → okr-draft"
    max_if_missing: 6
    evidence: "grep -E '^\\| *business-strategy *\\|' agents/evolve-router.md | grep forces-analysis | grep market-sizing | grep -q okr-draft"
  - criterion: "accounting-close / product-discovery / ops-incident recipes carry their spec §4.1 chains"
    max_if_missing: 6
    evidence: "grep -E '^\\| *accounting-close *\\|' agents/evolve-router.md | grep -q close-checklist && grep -E '^\\| *product-discovery *\\|' agents/evolve-router.md | grep -q metric-tree && grep -E '^\\| *ops-incident *\\|' agents/evolve-router.md | grep -q capacity-plan"
---

# Eval: Extend Goal-Type Recipes with 5 domain goal types

> Pins the goal-type half of the domain-phase-catalog integration wave
> (spec §4.1), shipped in cycle 12: five new rows — `project-management`,
> `business-strategy`, `accounting-close`, `product-discovery`,
> `ops-incident` — appended to the Goal-Type Recipes table in
> `agents/evolve-router.md`, each carrying its three-phase domain chain
> around the mandatory spine. Without these rows the advisor cannot
> classify-then-route a domain goal and falls back to ad-hoc composition,
> defeating the five shipped domain waves (PM cycle 6, Strategy cycle 8,
> Accounting/Ops cycle 5, Product cycle 10). Source incident: cycle 11
> implemented identical content with ACS 79/79 green but FAILED audit on a
> missing challenge-token header in build-report.md, so nothing was
> committed; cycle 12 is the sanctioned re-ship. Eval authored per the
> cycle-131 lesson (missing `.evolve/evals/<slug>.md` = automatic CRITICAL
> FAIL at audit).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| rows-present | All 5 goal-type rows in the recipes table | 5/10 | `grep -E '^\| *<goal> *\|'` on all 5 goals |
| pm-chain | project-management chain verbatim | 6/10 | row grep for risk-register/scope-baseline/dependency-map |
| strategy-chain | business-strategy chain verbatim | 6/10 | row grep for forces-analysis/market-sizing/okr-draft |
| remaining-chains | accounting/product/ops chains verbatim | 6/10 | row grep on chain-terminal phases |

All evidence commands are doc-presence checks: the recipe table is operator
guidance consumed by the router persona, not a subprocess-emitting system —
`# acs-predicate: config-check` waiver applies (cycles 2, 4, 5, 6, 9 lesson;
ACS predicates `acs/cycle-12/001–004` carry the waiver inline).

## Acceptance Criteria (cycle-12 ACS mapping)

| AC | Grader | ACS predicate |
|---|---|---|
| AC-1 all 5 rows present | [code] | `acs/cycle-12/001-goal-type-rows-present.sh` |
| AC-2 project-management chain | [code] | `acs/cycle-12/002-pm-recipe-contains-phases.sh` |
| AC-3 business-strategy chain | [code] | `acs/cycle-12/003-strategy-recipe-contains-phases.sh` |
| AC-4 accounting/product/ops chains | [code] | `acs/cycle-12/004-accounting-product-ops-recipes.sh` |
| AC-5 scope guard: no Go changes | [code] | `acs/cycle-12/005-scope-guard-t1-no-go-changes.sh` |
