---
score_cap:
  - criterion: "At enforce the dossier closeout reaches the bookkeeping batch and not main, and an inbox stamp is held by the batch"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2627_001_|TestC2627_002_' ./acs/cycle2627"
  - criterion: "The bookkeeping batch skips L2 and flushes alone at each code flush and at wave end, keeping its entries when a flush fails"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2627_003_|TestC2627_004' ./acs/cycle2627"
  - criterion: "At shadow the closeout and the inbox stamp commit exactly as before and feed no batch"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2627_001b_|TestC2627_005_|TestC2627_006_' ./acs/cycle2627"
---

# Eval: wave integration I6, the bookkeeping batch

> Pins the mechanism of ADR-0147 I6 (decision D12): at enforce the loop's own commits go to a bookkeeping batch that skips L2 and flushes at each code flush and at wave end; at shadow they stay as today. The production wiring (a `WithBookkeeping` call, a durable sink, the flusher) belongs to I11 and is not pinned here. Source: cycle 2627, commit 3f6841969.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| enforce-routing | Dossier closeout and inbox stamp go to the batch, not main | 6/10 | `TestC2627_001_`, `TestC2627_002_` |
| l2-and-flush | Batch skips L2, flushes alone, keeps entries on a failed flush | 6/10 | `TestC2627_003_`, `TestC2627_004`, `TestC2627_004b_` |
| shadow-identity | Shadow commits equal today's commits | 7/10 | `TestC2627_001b_`, `TestC2627_005_`, `TestC2627_006_` |
