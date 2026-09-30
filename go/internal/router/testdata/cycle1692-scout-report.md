<!-- challenge-token: ab1818044828f595 -->
goal_type: strategy-options
deliverable_kind: document

## Discovery Summary

Single-item, fenced scope (fleet_scope = `netflix-margin-device-experience`). No broad codebase scan performed — this is a document-deliverable solution cycle per ADR-0099, not a code-fix cycle. The inbox record (`.evolve/inbox/2026-09-10T00-10-00Z-netflix-margin-device-experience.json`) is a live PR-F soak item (#1), weight 0.80, route=lane, deliverable_kind=document. No prior `solutions/netflix-margin-device-experience/` directory exists yet in this worktree — first pass.

System health: no code changes are in scope this cycle, so no `go test` run is required for this task; last commit on branch: `4b77281a dossier: cycle-1691 closeout` (clean tree at cycle start).

## Key Findings

- The task is a strategy-options document: build 2+ mechanistically distinct candidate strategies to raise Netflix operating margin ~1pp via device experience (TV/streaming-device apps, partner integrations, playback quality, device-side cost-to-serve).
- Deliverable paths are explicit in the inbox record: `solutions/netflix-margin-device-experience/options/`, `recommendation.md`, `assumptions-and-evidence.md`.
- Acceptance criteria require: (1) ≥2 options each with a different causal mechanism and a quantified effect resting on a named assumption; (2) a recommendation with an Options Compared matrix, runner-up, and flip-evidence; (3) every quantitative claim cited to the evidence file, unsourced numbers labeled as assumptions; (4) `evolve solution check netflix-margin-device-experience` exits 0 with path:line audit citations; (5) routing plan justifies `tdd:false` as a document deliverable.
- Connects to `india-operator-bundle-plan` (a sibling PR-F soak item) and `examples/eval-solution.md` (canonical document-eval shape) — used as the eval template reference, not expanded in this cycle.

## Research

kb-search: grepped `knowledge-base/research/` and `.evolve/instincts/lessons/` for "document deliverable", "solution check", "strategy-options" — no on-point prior-cycle research artifacts specific to this domain (streaming/device economics) found; this is a fresh solution-domain item, so research is deferred to Build, which will need public-filing-style figures (device partner take-rate, cost-to-serve per stream, churn-elasticity to quality) sourced during drafting rather than pre-staged by Scout, per the "unsourced numbers are assumptions" rule. WebSearch/WebFetch not used (0/3, 0/5) — no KB gap severe enough to justify escalation at scout time; Build should source citations directly while drafting `assumptions-and-evidence.md`.

## Research → Implementation Map

| Finding | Task |
|---|---|
| 3 distinct margin mechanisms available (cost-to-serve, partner ARPU, churn/quality) | netflix-margin-device-experience (single task, multi-option document) |
| Canonical document-eval shape at `examples/eval-solution.md` | eval file for this slug models grader shape from that example |

## Hypotheses

1. Device-side cost-to-serve reduction (codec/bitrate efficiency, CDN/partner cost renegotiation) is the most directly quantifiable lever because it maps to a per-stream cost line item.
2. Partner-integration economics (device-maker revenue share / bundling placement) can move margin without touching playback cost, but the causal chain runs through negotiated take-rate, which is harder to source publicly.
3. Playback-quality-driven churn reduction is the slowest-acting lever (retention effects lag) but has the largest total addressable margin impact if quality materially affects churn elasticity.

## Beyond-the-Ask Hypotheses

1. A combined option (bundle cost-to-serve savings into partner renegotiation leverage) may beat any single lever — flag as a 4th "hybrid" option only if the recommendation's flip-evidence section has room; not required by acceptance criteria, so kept as a Build-discretion note, not a mandated 3rd distinct-mechanism option beyond the required 2.

## Selected Tasks

### Task: netflix-margin-device-experience

- **Deliverable kind:** document
- **Priority:** high (weight 0.80, PR-F soak #1)
- **Complexity:** M
- **Effort:** ~2 build turns (document drafting + evidence sourcing), ~1 audit turn
- **Target files (deliverable):**
  - `solutions/netflix-margin-device-experience/options/` (≥2 option files, one per mechanism: device-side cost-to-serve, partner-integration ARPU, playback-quality/churn — pick 2 minimum, distinct mechanisms, not the same lever at different sizes)
  - `solutions/netflix-margin-device-experience/recommendation.md` (Options Compared matrix, named runner-up, flip-evidence)
  - `solutions/netflix-margin-device-experience/assumptions-and-evidence.md` (every cited number, sourced or explicitly labeled assumption)
- **dependsOn:** []
- **verifiableBy:** `evolve solution check netflix-margin-device-experience` exits 0, AND each option file's quantified effect has a matching cited row in `assumptions-and-evidence.md`.
- **researchBacking:** inbox record acceptance criteria (direct); `examples/eval-solution.md` for document-eval shape (structural template only, not content evidence).
- **Recommended Skills:**
  - Primary: `solution-scout`/`solution-build` document-deliverable skill (per ADR-0099 preload)
  - Supplementary: `market-sizing` (for quantifying margin-pp impact), `adversarial-review` (post-build steelman of non-recommended options)
- **recommendedSkills JSON:** `[{"name":"solution-build","priority":"primary","rationale":"ADR-0099 document deliverable"},{"name":"market-sizing","priority":"supplementary","rationale":"quantify pp margin impact per option"},{"name":"adversarial-review","priority":"supplementary","rationale":"steelman non-recommended options before ship"}]`

## Acceptance Criteria Summary

1. ≥2 options, each a distinct mechanism, each with causal chain → margin and a quantified effect resting on a named assumption.
2. Recommendation follows from an Options Compared matrix; names runner-up; states flip-evidence.
3. Every quantitative claim cites an evidence-file entry supporting magnitude+direction; unsourced numbers labeled assumptions.
4. `evolve solution check netflix-margin-device-experience` exits 0; audit cites path:line per criterion and per option.
5. Routing plan justifies `tdd:false` (document deliverable) under the `solution(netflix-margin-device-experience)` ship prefix.

## Carryover Decisions

No `carryoverTodos` present in this cycle's context (fleet_scope restricts scout to this single assigned id; no inbox/carryover entries beyond the one fleet-scope record were surfaced). N/A — nothing to walk.

## Deferred

- Hybrid 4th option (cost-to-serve + partner renegotiation combined) — deferred to Build discretion, not required by acceptance criteria; do not let it displace the two mandated distinct-mechanism options.
- `india-operator-bundle-plan` (PR-F soak #2) — explicitly out of scope for this fenced cycle.

## Research Cache

| carryoverTodo | Status |
|---|---|
| (none — fleet_scope has no carryoverTodo entries this cycle) | NO_ENTRY |

## Build Plan Summary

1. Draft ≥2 option files under `solutions/netflix-margin-device-experience/options/`, each keyed to a distinct margin mechanism (cost-to-serve, partner ARPU, or churn/quality), each stating its causal chain and a quantified effect tied to a named assumption.
2. Source real figures where public (filings, published research) into `assumptions-and-evidence.md`; label every unsourced number as an assumption inline.
3. Build the Options Compared matrix in `recommendation.md`; select a recommendation, name the runner-up, and state the specific evidence that would flip the choice.
4. Run `evolve solution check netflix-margin-device-experience`; fix violations until exit 0.
5. Confirm routing plan sets `tdd:false` with an explicit document-deliverable justification before handoff to audit.

## Decision Trace

```json
{
  "goal_hash": "e51a1a498492896c8ab329adb0ce76cf6be70a1f481e2f4d6f1bf3e468f5b0d8",
  "cycle": 1692,
  "fleet_scope": "netflix-margin-device-experience",
  "deliverable_kind": "document",
  "goal_type": "strategy-options",
  "selected_tasks": ["netflix-margin-device-experience"],
  "task_proposals": [
    {
      "slug": "netflix-margin-device-experience",
      "complexity": "M",
      "effort_turns": 3,
      "dependsOn": [],
      "verifiableBy": "evolve solution check netflix-margin-device-experience exits 0",
      "recommendedSkills": [
        {"name": "solution-build", "priority": "primary", "rationale": "ADR-0099 document deliverable"},
        {"name": "market-sizing", "priority": "supplementary", "rationale": "quantify pp margin impact per option"},
        {"name": "adversarial-review", "priority": "supplementary", "rationale": "steelman non-recommended options before ship"}
      ]
    }
  ],
  "carryover_decisions": [],
  "auto_picked": []
}
```

## Reflection

- Friction: `ambiguous-input` — the inbox record proposes 3 candidate mechanisms but acceptance only requires 2; scout resolved by mandating the 2 strongest (cost-to-serve, partner ARPU) as required and deferring churn/quality + hybrid as Build discretion, to keep the task sized M rather than L.
