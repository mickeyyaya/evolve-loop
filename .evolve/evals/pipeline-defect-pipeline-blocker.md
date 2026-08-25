---
score_cap:
  - criterion: "The ADR-0072 halt auto-filer mints the auto-filed inbox item's id as pipeline-defect-<category>-cycle<N>, never <category> alone"
    max_if_missing: 7
    evidence: "cd go && go test -run TestWritePipelineEscalation_IdentityIncludesCycleNumber -count=1 ./cmd/evolve"
  - criterion: "Two ADR-0072 halts sharing a category (the common case — 17 on-disk records already share pipeline-defect-pipeline-blocker) never collide on disk; each cycle's record survives"
    max_if_missing: 7
    evidence: "cd go && go test -run TestWritePipelineEscalation_DistinctCyclesNeverCollideOnDisk -count=1 ./cmd/evolve"
  - criterion: "Pre-existing dossier+inbox-item auto-filing behaviour is not weakened while fixing the identity mint"
    max_if_missing: 5
    evidence: "cd go && go test -run TestWritePipelineEscalation_WritesDossierAndInboxItem -count=1 ./cmd/evolve"
---

# Eval: ADR-0072 halt auto-filer mints unique record identities

> Pins the fix for the recurring inst-L1543c empty-scope pipeline defect
> (cycle 1543, cycle 1548 audit defect H1, and this cycle — 1550 — a third
> live occurrence). `writePipelineEscalation`
> (go/cmd/evolve/cmd_loop_escalation.go) auto-files a P0 pipeline-repair
> inbox item using a deterministic filename derived from `sf.Category`
> alone (`pipeline-defect-<category>.json`). Because the category, not the
> cycle, is the sole key, every later halt sharing a category silently
> overwrites the earlier halt's on-disk record — state.json documents "17
> on-disk records share the id pipeline-defect-pipeline-blocker (1 live +
> 16 consumed)". A fleet lane's scope snapshot (lane-scope.json) captures
> only the bare id at triage time; by the time scout runs, that id can
> resolve to a different halt's content than the one the lane was actually
> scoped to, or to none scout's live inbox scan recognizes — producing a
> lane that dispatches through all eight phases and lands an empty diff
> (git diff main...HEAD empty), exactly what cycle 1548 was FAILed for
> (defect H1) and what this cycle (1550) reproduced live: triage's only
> top_n item was the bare id "pipeline-defect-pipeline-blocker" with the
> generic action "fix the pipeline defect/blocker identified for this
> lane", while scout's own live scan selected two entirely different,
> unrelated tasks. The carryover queue already names the fix
> (todo-halt-autofiler-mints-unique-ids, HIGH, first_seen_cycle 1548):
> "Make the ADR-0072 halt auto-filer mint pipeline-defect-<category>-cycle<N>
> unconditionally, so a category label can never be reused as a record
> identity."

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| identity-mint | id includes the cycle number | 7/10 | `go test -run TestWritePipelineEscalation_IdentityIncludesCycleNumber ./cmd/evolve` |
| no-collision | distinct cycles' records both survive on disk | 7/10 | `go test -run TestWritePipelineEscalation_DistinctCyclesNeverCollideOnDisk ./cmd/evolve` |
| anti-weakening | pre-existing dossier+inbox coverage still green | 5/10 | `go test -run TestWritePipelineEscalation_WritesDossierAndInboxItem ./cmd/evolve` |
