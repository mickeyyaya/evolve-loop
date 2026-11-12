---
score_cap:
  - criterion: "A tree that holds only pipeline output (.commit-gate/ and .evolve/ at any depth) passes the leftovers gate"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^(TestRebuildableIgnored_PipelineOutputAtAnyDepth|TestPlanDevWorktrees_ATreeHoldingOnlyPipelineOutputIsArchived|TestPlanDevWorktrees_APipelineOnlyCommitGateTreeIsArchived)$' ./internal/gc"
  - criterion: "An .env file or a nested repository beside pipeline output still keeps the tree"
    max_if_missing: 3
    evidence: "cd go && go test -count=1 -run '^(TestPlanDevWorktrees_AnEnvFileBesidePipelineOutputKeepsTheTree|TestPlanDevWorktrees_AnIgnoredFileInsidePipelineOutputDirectoryStaysRebuildable|TestPlanDevWorktrees_ANestedRepositoryBesidePipelineOutputKeepsTheTree)$' ./internal/gc"
  - criterion: "The earlier rebuildable and nested repository guards stay green"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^(TestRebuildableIgnored|TestPlanDevWorktrees_ANestedGitRepositoryBlocksTheArchive|TestPlanDevWorktrees_AnIgnoredFileBlocksTheArchiveAndIsNamed)$' ./internal/gc"
---

# Eval: gc X3, pipeline output is rebuildable

> Pins plan decision D3 and component X3 of docs/plans/gc-closes-the-legacy-gap-2026-11.md. The rebuildable list gains `.evolve/` and `.commit-gate/` at any depth, and the nested repository guard stays. Source incident: 140 dev trees held only pipeline output and were kept (cycle 2327 scout).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| pipeline-only-passes | Only pipeline output in a tree gives the archive action | 4/10 | `go test -run TestPlanDevWorktrees_ATreeHoldingOnlyPipelineOutputIsArchived ./internal/gc` |
| real-work-stays | `.env` or a nested repository keeps the tree | 3/10 | `go test -run TestPlanDevWorktrees_AnEnvFileBesidePipelineOutputKeepsTheTree ./internal/gc` |
| guards-preserved | The earlier guards stay green | 5/10 | `go test -run TestRebuildableIgnored ./internal/gc` |
