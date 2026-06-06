# Domain Phase Catalog — Cross-Domain Checkpoint Phases (PM · Strategy · Accounting · Product · Ops)

**Date:** 2026-06-06
**Status:** Approved design; implementation queued as evolve-loop cycle goals (5 domain waves + 1 integration cycle) on branch `feat/domain-phase-catalog`
**Research basis:** [knowledge-base/research/domain-phase-catalog-research-2026-06-06.md](../../knowledge-base/research/domain-phase-catalog-research-2026-06-06.md) (5-domain cited web research)
**Builds on:** [micro-phase-catalog.md](micro-phase-catalog.md) (the software catalog — same design principles, naming rule, and additive-first decision), ADR-0028 (user-defined phases), ADR-0035 (spec-derived contracts)

---

## 1. Problem

The phase catalog covers software-development goal types only (bugfix/feature/refactor/security/performance/release). Goals from other domains — plan this project, analyze this market, verify this close, define this product, learn from this incident — all degrade to "docs / trivial: spine only". Yet these domains have *more* standardized checkpoint artifacts than software does (risk registers, reconciliations, postmortems are decades-old practitioner standards with canonical sections). The research confirms each domain's top workflows converge on document shapes an LLM phase can produce and a contract gate can verify.

**Decision: additive-first, identical to the micro-phase catalog.** All 15 phases are `optional: true` user phases (4 files each, zero Go). The spine and integrity floor are untouched. Domain phases route on new `scout.goal_type` values (free-form strings — no Go enum) and NEVER carry ship authority.

## 2. Design principles (inherited + domain-specific)

1. **One responsibility, one artifact, ≤4 required sections** (micro-catalog principle 1, tightened for contract-gate clarity).
2. **Artifact-first**: every phase emits a document practitioners already standardize; required sections come from the cited canonical templates, not invention.
3. **Checkpoint, not process**: the phase reviews/produces the control document; it does not execute the business process.
4. **Routing by goal classification**: `insert_when: [{field: "scout.goal_type", op: "==", value: "<domain>"}]`, `after: "triage"` — proven string-equality mechanism, advisor classifies.
5. **Signals are doc-shaped and advisory** (counts, flags); classify gates on `require_sections` + `fail_if_empty` only in v1 (shadow-first; tighten `fail_if_signal` from observed data later, same as mutation-gate's staged rollout).
6. **Naming**: `<object>-<action[-nominal]>` per the two-tier rule. Signal namespaces must not collide with the existing set (`dependency.*` is taken → `dependency-map` uses `depmap.*`).

## 3. The catalog — 15 phases, 5 domains

Per phase, **4 files** (matching every shipped user phase — do NOT rely on `evolve phases add`, whose scaffold writes only 3 files and puts `profile.json` inside the phase dir instead of `.evolve/profiles/`): `.evolve/phases/<name>/phase.json`, `.evolve/phases/<name>/agent.md`, `agents/evolve-<name>.md` (byte-identical persona mirror of the phase-dir `agent.md`), and `.evolve/profiles/<name>.json`. Artifact = `<name>-report.md` (uniform; simplifies the contract table). All `kind: llm`, `optional: true`, `writes_source: false`, `model: auto`, inputs `scout-report.md`, `prompt_context: ["goal"]`, `after: "triage"` unless noted. Profile template: fault-localization's (read-only repo, write `.evolve/runs/cycle-*`, no network, no git) — **explicit v1 decision**: domain phases judge over cycle inputs only; external data fetch (market data, GL feeds) is out of scope until a vetted network-profile pattern exists.

### Wave PM — `project-management`

| Phase | Archetype | Core value (the single risk it removes) | Required sections | Signals |
|---|---|---|---|---|
| `risk-register` | plan | Unowned, unscored threats surfacing late | Risks · Scoring · Response Strategies · Owners | `risk.count`, `risk.high_count` |
| `scope-baseline` | plan | Scope creep against no reference line / ambiguous done-criteria | Deliverables · Acceptance Criteria · Exclusions · Constraints and Assumptions | `scope.deliverables_count` |
| `dependency-map` | evaluate | Hidden cross-task blockers and an unknown critical path | Dependencies · Critical Path · Blockers | `depmap.count`, `depmap.blocker_count` |

Evidence: PMI risk-register columns; PMBOK scope statement + 100% Rule; CPM zero-float chain. (Research §1.)

### Wave Strategy — `business-strategy`

| Phase | Archetype | Core value | Required sections | Signals |
|---|---|---|---|---|
| `forces-analysis` | evaluate | Entering a structurally unprofitable market blind | Competitive Rivalry · Buyer and Supplier Power · Entry and Substitute Threats · Attractiveness Verdict | `forces.high_pressure_count` |
| `market-sizing` | evaluate | Pursuing an opportunity too small (or an inflated TAM) | TAM · SAM · SOM · Methodology and Assumptions | `market.layers_quantified` |
| `okr-draft` | plan | Activity-based, unmeasurable goals | Objective · Key Results · Confidence and Scoring | `okr.key_results_count` |

Evidence: HBS ISC Five Forces; HubSpot/Amazon Ads TAM-SAM-SOM; Google re:Work OKR playbook (3–5 KRs, ~70% target). (Research §2.)

### Wave Accounting — `accounting-close`

| Phase | Archetype | Core value | Required sections | Signals |
|---|---|---|---|---|
| `account-reconcile` | evaluate | An unsubstantiated GL balance (undetected error/fraud) | GL vs Source Balance · Reconciling Items · Adjustments · Sign-off | `reconcile.unexplained_count` |
| `variance-analysis` | evaluate | Unexplained budget-to-actual drift | Budget vs Actual · Classification · Drivers · Reforecast Impact | `variance.material_count` |
| `close-checklist` | control | An incomplete or unauthorized close | Tasks · Blocking Items · Sign-off | `close.blocking_count` |

Evidence: BlackLine reconciliation structure; Wall Street Prep variance columns; FloQast close checklist; COSO control activities. (Research §3.)

### Wave Product — `product-discovery`

| Phase | Archetype | Core value | Required sections | Signals |
|---|---|---|---|---|
| `opportunity-map` | plan | Solutioning without a validated, outcome-linked customer problem | Desired Outcome · Opportunities · Candidate Solutions · Assumption Tests | `opportunity.count` |
| `prd-draft` | plan | Building with no documented problem or success contract | Problem · Goals and Success Metrics · Requirements · Out of Scope | `prd.requirements_count` |
| `metric-tree` | evaluate | Shipping with no measurable definition of success | North Star Metric · Input Metrics · Guardrail Metrics | `metric.inputs_count` |

(Order is canonical and matches the §4.1 recipe: opportunities are mapped before the PRD is drafted — Torres' OST grounds the PRD problem statement.)

Evidence: SVPG/Lenny PRD convergence (Non-Goals 2nd-most-common element); Torres opportunity solution tree; Amplitude North Star (1 NSM + 3–5 inputs). (Research §4.)

### Wave Ops — `ops-incident`

| Phase | Archetype | Core value | Required sections | Signals |
|---|---|---|---|---|
| `incident-postmortem` | evaluate | Root cause and corrective actions unrecorded → incident recurs | Impact · Timeline · Root Cause · Action Items | `postmortem.action_items_count` |
| `runbook-draft` | control | On-call responders improvising with no validated recovery path | Trigger · Diagnosis · Resolution Steps · Escalation | `runbook.steps_count` |
| `capacity-plan` | plan | Capacity shortfall from unforecasted demand growth | Demand Forecast · Current Capacity · Capacity Gap | `capacity.gap_count` |

Evidence: Google SRE ch. 15 + PagerDuty/Atlassian postmortem convergence; SRE Workbook playbooks; SRE ch. 18 capacity. (Research §5.)

### Classify defaults (all 15)

```json
"classify": { "require_sections": [...per table...], "fail_if_empty": true, "verdict_on_pass": "PASS" }
```

`verdict_on_pass` is set on all 15 for uniformity, but it is **silently inert on the 6 plan and 2 control phases**: contract derivation (`phasecontract.verdictsFromSpec`, ADR-0035) emits a verdict vocabulary only when archetype = `evaluate` — so exactly 7 of 15 emit verdicts (consistent with the shipped `architecture-design` precedent of a non-evaluate phase carrying the field).

Section-count discipline (≤4) forced two documented drops from the researched templates: the postmortem's fifth convergent section *Lessons Learned* folds into **Action Items** (and feeds the existing `memo`/`lesson-extract` path), and the runbook's *Verification* step folds into **Resolution Steps**.

Naming note: `close-checklist` and `metric-tree` carry artifact-noun second components rather than action verbs — same accepted shape as the shipped `threat-model` and `rollback-plan` (artifact-name fidelity to the practitioner standard beats verb purity).

## 4. Advisor integration

### 4.1 Goal-Type Recipes table extension (`agents/evolve-router.md`, integration cycle)

| Goal type | Recipe (optional insertions around the mandatory spine) |
|---|---|
| project-management | risk-register → scope-baseline → dependency-map → [build = the planning deliverable] |
| business-strategy | forces-analysis → market-sizing → okr-draft → [build] |
| accounting-close | account-reconcile → variance-analysis → close-checklist → [build] |
| product-discovery | opportunity-map → prd-draft → metric-tree → [build] |
| ops-incident | incident-postmortem → runbook-draft → capacity-plan → [build] |

Also: add all 15 one-line core values to the "Phase Catalog — Core Values" table.

### 4.2 New signal namespaces

`risk.*`, `scope.*`, `depmap.*`, `forces.*`, `market.*`, `okr.*`, `reconcile.*`, `variance.*`, `close.*`, `prd.*`, `opportunity.*`, `metric.*`, `postmortem.*`, `runbook.*`, `capacity.*`. (Checked against micro-catalog §4.3 — no collisions; `dependency.*` deliberately avoided.)

### 4.3 Config

No `max_optional_insertions` change needed: every domain recipe inserts ≤3 (current limit 6).

## 5. Implementation plan (the campaign)

One wave per autonomous cycle on `feat/domain-phase-catalog` (order: PM → Strategy → Accounting → Product → Ops → integration). Per cycle, TDD-first:

1. **RED**: add the wave's 3 table cases to `go/internal/phasespec/usercatalog_research_test.go` (`TestResearchPhasesAreConfigOnly`) — artifact `<name>-report.md`, sections per §3 tables (canonical `## <section>` form), `hasVerdict` true iff archetype = evaluate. Test fails: phases not in catalog.
2. **GREEN**: author the 4 files per phase; `evolve phases validate <name>` exit 0; test passes.
3. Flip the wave's status in this document.

Integration cycle: router recipe table + core-values table + `docs/architecture/user-defined-phases.md` touch-up if guidance changed.

### Wave status

| Wave | Status |
|---|---|
| PM | ✅ done (cycle 6) |
| Strategy | ✅ done (cycle 8) |
| Accounting | ✅ done (cycle 5) |
| Product | ✅ done (cycle 10) |
| Ops | ✅ done (cycle 5) |
| Integration | ⬜ queued (cycle 6) |

## 6. Out of scope (documented kills — see research §§1–5)

RAID-log, WBS-full-decomposition, swot-analysis, business-model-canvas, controls-matrix, rolling-forecast, user-story-map, ab-test-design, JTBD-interview, incident-command. Marketing/legal/HR domains: deferred to a follow-up research round.

## 7. Verification

- Per phase: `evolve phases validate <name>` OK; appears in `evolve phases list` with SOURCE = `user`.
- Per wave: `go test ./internal/phasespec/... -count=1` green (table covers the wave).
- Catalog-level (post-merge, optional demo): a `project-management`-goal cycle's routing plan selects the PM recipe phases; a docs-goal cycle stays spine-only.
