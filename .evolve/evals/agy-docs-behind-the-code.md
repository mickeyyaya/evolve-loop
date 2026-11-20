---
score_cap:
  - criterion: "The tier table of docs/reference/model-routing.md states the deep and top chains of .evolve/policy.json cli_routing"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestModelRoutingDocMatchesPolicy$' ./internal/cliroute"
  - criterion: "ADR-0150 amends ADR-0136, and ADR-0136 links ADR-0150 as its amendment"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestADR0150AmendsADR0136$' ./internal/cliroute"
  - criterion: "The drift check rejects a stale chain order, an empty tier table and a key that no struct tag defines"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestModelRoutingDocDriftIsDetected$' ./internal/cliroute"
  - criterion: "The routing bullets of AGENTS.md state the clis, default, deep, top and builder chains, the deep share and the Gemini models of the policy and the agy manifest"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestAgentsMdRoutingMatchesPolicy$' ./internal/cliroute"
  - criterion: "The AGENTS.md drift check rejects a stale chain, a merged statement, a stale agy-first claim and an unknown Gemini model"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestAgentsMdRoutingDriftIsDetected$' ./internal/cliroute"
---

# Eval: agy-docs-behind-the-code

> Pins the facts of the agy routing documents to the checked-in routing table. Source incident: the console audit of agy on 2026-11-06 found that model-routing.md kept the chain order of ADR-0136 after ADR-0150 and the policy changed the deep chain to claude, agy, agy-claude with a 50% share. Cycle 2547 added the drift test. Cycle 2561 found that AGENTS.md still said that `clis` and `default` are agy-first, that deep seats run agy first and that balanced runs Gemini 3.8 Flash, and added the same check for AGENTS.md.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| doc-matches-policy | deep and top rows, bullets, share and keys equal the policy | 6/10 | `go test -run '^TestModelRoutingDocMatchesPolicy$' ./internal/cliroute` |
| adr-link | ADR-0150 Amends ADR-0136; ADR-0136 Amended by ADR-0150 | 7/10 | `go test -run '^TestADR0150AmendsADR0136$' ./internal/cliroute` |
| detector-negative | the check fails on a stale chain, an empty table, an unbuilt key | 7/10 | `go test -run '^TestModelRoutingDocDriftIsDetected$' ./internal/cliroute` |
| agents-md-matches-policy | the routing bullets of AGENTS.md equal the policy and the manifest | 7/10 | `go test -run '^TestAgentsMdRoutingMatchesPolicy$' ./internal/cliroute` |
| agents-md-detector-negative | the check fails on a stale chain, a merged statement, an agy-first claim, an unknown model | 6/10 | `go test -run '^TestAgentsMdRoutingDriftIsDetected$' ./internal/cliroute` |
