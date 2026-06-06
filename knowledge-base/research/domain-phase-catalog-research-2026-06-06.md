# Domain Phase Catalog — Research Basis (Non-Software Domains)

**Date:** 2026-06-06
**Method:** 5 parallel web-research agents (one per domain), each returning cited findings on (a) most-adopted canonical workflows, (b) standard artifact sections, (c) top-3 checkpoint-phase candidates. A 111-agent adversarial deep-research run was attempted first but failed on a harness StructuredOutput bug; this simpler fan-out replaced it. Canonical-framework claims (PMBOK, Scrum Guide, Google SRE, COSO, Strategyzer…) are low-controversy and source-stable, so single-researcher-with-citations was judged sufficient rigor.
**Predecessor:** [micro-phase-catalog-research-2026-06-05.md](micro-phase-catalog-research-2026-06-05.md) (software-only by scope — this round fills the explicitly-deferred non-software gap).
**Feeds:** [docs/architecture/domain-phase-catalog.md](../../docs/architecture/domain-phase-catalog.md)

---

## 1. Project Management (`goal_type: project-management`)

### Canonical workflows + adoption
- **PMBOK process groups** (Initiating, Planning, Executing, Monitoring & Controlling, Closing) — PMI Pulse of the Profession: organizations with standardized PM practices meet original goals in 90% of projects; standardized risk management used by 64–68% of enterprises [PMI Process Groups Practice Guide; PMI Pulse 2024].
- **Scrum** — 5 events (Sprint, Planning, Daily, Review, Retrospective); ~87% of agile respondents use Scrum [Scrum Guide 2020; 17th/18th State of Agile].
- **PRINCE2** — 7 principles / 7 themes / 7 processes; "manage by stages", "manage by exception" [prince2.com].
- **Critical Path Method** — forward/backward pass, ES/EF/LS/LF, zero-float chain [Asana CPM; Institute of PM].

### Standard artifacts (sections/columns)
- **Risk Register** (PMI): Risk ID, Description, Category, Trigger, Probability, Impact, Score (P×I), Response Strategy (Avoid/Mitigate/Transfer/Accept; Exploit/Enhance/Share/Accept for opportunities), Action, named Risk Owner. Living document [PMI Risk Analysis; Wrike; 4PMTI].
- **RAID Log**: Risks / Assumptions / Issues / Dependencies quadrants [Asana; ProjectManager.com].
- **Scope Statement** (PMBOK): Scope Description, Deliverables, Acceptance Criteria, Exclusions, Constraints & Assumptions [PM Academy; PM Study Circle].
- **WBS / Scope Baseline**: deliverable-oriented hierarchical decomposition under the 100% Rule; baseline = scope statement + WBS + WBS dictionary [PMI Practice Standard for WBS].
- **Critical-path network**: activities with FS/SS/FF/SF links, float per activity [Asana; Lucid].

### Kept phases
`risk-register` (plan) · `scope-baseline` (plan) · `dependency-map` (evaluate). Killed: RAID-log phase (subsumed: R→risk-register, D→dependency-map; A/I are live operational state, not checkpoint output); WBS-full-decomposition (too heavyweight for one LLM checkpoint; scope-baseline carries the deliverables skeleton).

## 2. Business Strategy (`goal_type: business-strategy`)

### Canonical workflows + adoption
- **Bain Management Tools & Trends** (since 1993) is the adoption benchmark; SWOT near-universal as a screening tool [Bain].
- **Porter's Five Forces** (1979, HBS) — canonical industry-structure framework [HBS ISC].
- **OKRs** — Grove→Doerr→Google; re:Work playbook: ≤5 objectives/quarter, 3–5 measurable KRs each, ~70% attainment = success [Google re:Work; whatmatters.com].
- **Business Model Canvas** — Osterwalder 9 blocks [Strategyzer].
- **TAM-SAM-SOM** — standard nested market-sizing model, top-down or bottom-up [HubSpot; Amazon Ads].

### Standard artifacts (sections)
- **Five Forces analysis**: 5 forces + industry-attractiveness/strategic implication [HBS ISC; Mindtools].
- **OKR set**: 1 Objective + 3–5 measurable Key Results (outcomes, not activities) [re:Work].
- **Market sizing**: TAM → SAM → SOM + methodology note [HubSpot; Seer].
- **SWOT matrix**: 4 quadrants internal/external × positive/negative [TechTarget; Asana].
- **BMC**: 9 blocks [Strategyzer].

### Kept phases
`forces-analysis` (evaluate) · `market-sizing` (evaluate) · `okr-draft` (plan). Killed: `swot-analysis` (researcher-ranked below Five Forces for checkpoint value — quadrants degrade to generic listing without industry-structure grounding; revisit in a follow-up wave); `business-model-canvas` (9 blocks exceed the ≤4-section artifact discipline; candidate for a future composite).

## 3. Accounting & Finance (`goal_type: accounting-close`)

### Canonical workflows + adoption
- **Month-end close** with close-management checklists (FloQast/BlackLine practice model) — APQC (~2,300–3,000 orgs): median monthly close ~6.0–6.4 calendar days; Ventana 2022: full automation → ≤6-day close 69% vs 29% [APQC; Ventana via Numeric].
- **Account/balance-sheet reconciliation** — COSO-named control activity, core SOX 404/ICFR control; AICPA risk-ranking + materiality tolerances [COSO; PCAOB AS 2201; JofA].
- **Budget variance analysis** — core FP&A deliverable: Budget/Actual/Variance($/%)/F-U/driver columns [Wall Street Prep].
- **FP&A cadence** — flash BD4, package BD8, rolling 12–18-mo forecast BD14 [Farseer; WSP].

### Standard artifacts (sections/columns)
- **Close checklist**: Task / Owner / Due day / Status (done·in-review·blocking) / Reviewer sign-off / Evidence link [FloQast].
- **Reconciliation**: GL balance → source balance → reconciling items → adjustments/JEs → difference vs tolerance → preparer+reviewer certification [BlackLine; JofA].
- **Variance report**: line item / budget / actual / variance $ / variance % / favorable-unfavorable / driver explanation [WSP].
- **Controls matrix**: process / risk / control activity / owner / frequency / test evidence, mapped to COSO 5 components & 17 principles [AuditBoard; PCAOB].

### Kept phases
`account-reconcile` (evaluate) · `variance-analysis` (evaluate) · `close-checklist` (control). Killed: controls-matrix/SOX-narrative phase (audit-firm territory, weak fit for an LLM checkpoint artifact v1); rolling-forecast phase (cadence artifact, folded as Reforecast Impact section of variance-analysis).

## 4. Product Management (`goal_type: product-discovery`)

### Canonical workflows + adoption
- **PRD** — Cagan/SVPG canonical 4 parts (Purpose, Features, Release Criteria, Rough Timing); modern elite templates (Lenny, Aakash Gupta) converge on Problem, Goals, Non-Goals, Success Metrics, Requirements, Risks; "Non-Goals" 2nd-most-common element [SVPG; Lenny's Newsletter].
- **Opportunity Solution Tree** — Torres 2016, *Continuous Discovery Habits*: Outcome → Opportunities → Solutions → Assumption Tests [Product Talk; Product School].
- **Story Mapping** — Patton: backbone → tasks → release slices → walking skeleton [jpattonassociates].
- **North Star framework / metric tree** — Amplitude: 1 NSM + 3–5 input metrics [Amplitude Playbook].
- **JTBD** — job story (When/I want/So I can) + Forces of Progress [Christensen/Moesta; Strategyn].
- **A/B test design doc** — 7 sections: background, hypothesis, variants, metrics (1 primary + guardrails), sample size/MDE, implementation, analysis [GrowthBook; VWO].

### Kept phases
`opportunity-map` (plan; renamed from "opportunity-solution-tree" to fit `<object>-<action>` naming) · `prd-draft` (plan) · `metric-tree` (evaluate) — in pipeline order: opportunities are mapped before the PRD is drafted. Killed: `user-story-map` (overlaps opportunity-map's outcome→solution decomposition; OST researcher-ranked higher for discovery rigor); `ab-test-design` (strong artifact but routes on an experiment-goal subtype — defer to a follow-up wave); JTBD (interview-technique more than checkpoint artifact).

## 5. Operations / Reliability (`goal_type: ops-incident`)

### Canonical workflows + adoption
- **Blameless postmortem** — Google SRE Book ch. 15; mirrored by Atlassian/PagerDuty/incident.io; DORA 2024 tracks post-incident review practice + notes AI-drafted PIR docs as emerging norm [sre.google; DORA 2024].
- **Runbook/playbook response** — SRE Workbook definition; DORA measures playbook presence [sre.google; DORA].
- **Incident Command** — ICS/NIMS adaptation: IC, Deputy, Scribe, SMEs; three Cs [PagerDuty Response].
- **Capacity planning** — SRE Book ch. 18: organic+inorganic demand forecast vs capacity [sre.google].

### Standard artifacts (sections)
- **Postmortem** convergent core across Google/PagerDuty/Atlassian templates: Impact, Timeline, Root Cause, Action Items, Lessons Learned [PagerDuty template; Atlassian template].
- **Runbook**: trigger/objective, step-by-step diagnosis, mitigation/resolution, verification, escalation + owner metadata [SRE Workbook; Emmer template].
- **Capacity plan**: demand forecast (organic+inorganic), current capacity/scaling data, resource allocation/gap [SRE Book ch. 18].

### Kept phases
`incident-postmortem` (evaluate) · `runbook-draft` (control) · `capacity-plan` (plan). Killed: incident-command phase (live-coordination role structure, not a pipeline checkpoint artifact).

---

## 6. Cross-domain selection principles applied

1. **Artifact-first**: every kept phase maps to a document practitioners already standardize (risk register, reconciliation, postmortem…) — the LLM produces a well-known shape, the contract gate checks well-known sections.
2. **≤4 required sections** per artifact (contract-gate discipline; matches micro-phase-catalog "one classify rule" principle).
3. **Checkpoint over process**: phases are review/checkpoint moments, not whole workflows (e.g. close-checklist verifies close-readiness; it does not perform the close).
4. **Naming**: all `<object>-<action[-nominal]>` per the two-tier rule; collisions checked against the existing catalog (note: `dependency.*` signal namespace is taken by `dependency-audit`, so `dependency-map` emits `depmap.*`).
5. **Never ship authority**: all 15 are `optional:true`, route on `scout.goal_type` string equality, gate on `require_sections` + `fail_if_empty` — same class as `threat-model`/`rollback-plan`.

## 7. Sources (per domain, abbreviated — full URLs in section citations above)

PM: PMI (Process Groups, Risk Analysis, WBS Practice Standard, Pulse 2024), Scrum Guide 2020, Scrum.org State of Agile, prince2.com, Asana/Lucid CPM.
Strategy: Bain Management Tools & Trends, HBS ISC Five Forces, Google re:Work + whatmatters.com OKR playbook, Strategyzer BMC, HubSpot/Amazon Ads/Seer TAM-SAM-SOM.
Accounting: APQC close-cycle benchmark, Ventana 2022 (via Numeric), BlackLine reconciliation, FloQast close checklist, AICPA/Journal of Accountancy, COSO/AuditBoard, PCAOB AS 2201, Wall Street Prep FP&A.
Product: SVPG/Cagan PRD, Lenny's Newsletter templates, Aakash Gupta PRD guide, Product Talk/Product School OST, jpattonassociates story mapping, Amplitude North Star Playbook, Strategyn JTBD, GrowthBook/VWO A/B guides.
Ops: Google SRE Book ch. 15/18 + Workbook, PagerDuty Response (postmortem template, IC roles), Atlassian postmortem templates, DORA 2024 report, Emmer runbook template.
