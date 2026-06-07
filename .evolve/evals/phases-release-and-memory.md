---
score_cap:
  - criterion: "All 4 Wave-3 phase descriptors validate via the engine"
    max_if_missing: 6
    evidence: "for p in changelog-sync post-ship-monitor api-contract-design context-condense; do EVOLVE_PHASE_ROOTS=$(git rev-parse --show-toplevel)/.evolve/phases $(git rev-parse --show-toplevel)/go/bin/evolve phases validate $p || exit 1; done"
  - criterion: "changelog-sync is a control-archetype phase (deterministic tooling, not generative)"
    max_if_missing: 7
    evidence: "python3 -c \"import json,sys; sys.exit(0 if json.load(open('.evolve/phases/changelog-sync/phase.json')).get('archetype')=='control' else 1)\""
  - criterion: "Wave-3 profiles parse as JSON with the shipped profile schema's required fields"
    max_if_missing: 6
    evidence: "python3 -c \"import json; req=['name','cli','model_tier_default','role','sandbox','max_turns','max_budget_usd']; [1/0 for p in ['changelog-sync','post-ship-monitor','api-contract-design','context-condense'] for k in req if k not in json.load(open('.evolve/profiles/%s.json'%p))]\""
  - criterion: "Wave-3 phase artifacts are git-tracked (not gitignore-shadowed)"
    max_if_missing: 5
    evidence: "git ls-files --error-unmatch .evolve/phases/changelog-sync/phase.json .evolve/phases/post-ship-monitor/phase.json .evolve/phases/api-contract-design/phase.json .evolve/phases/context-condense/phase.json .evolve/profiles/changelog-sync.json .evolve/profiles/post-ship-monitor.json .evolve/profiles/api-contract-design.json .evolve/profiles/context-condense.json"
---

# Eval: Wave-3 release/feature/memory phases (changelog-sync, post-ship-monitor, api-contract-design, context-condense)

> Pins the Wave-3 micro-phase-catalog §3 contracts authored in cycle 247:
> four config-only phase descriptors with their archetypes (3× control,
> 1× plan), signal namespaces (`changelog.*`, `post_ship.*`, `contract.*`,
> `condense.*`), profile schema parity with the shipped profiles, and git
> tracking. Source context: cycle-247 task 2 (carryover
> `phases-release-and-memory`); archetype/signal spec from
> docs/architecture/micro-phase-catalog.md §3/§4.3. The archetype cap matters
> most: a changelog-sync minted as a generative phase would invert its design
> intent (deterministic drift detection wrapped in minimal LLM).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| validate-positives | 4 wave-3 phases validate exit 0 | 6/10 | per-phase `evolve phases validate` loop |
| archetype-contract | changelog-sync archetype == control | 7/10 | python3 JSON assert |
| profile-schema | 4 profiles JSON-valid w/ required fields | 6/10 | python3 schema sweep |
| tracking-guard | descriptors + profiles git-tracked | 5/10 | `git ls-files --error-unmatch` (dual-check rule) |
