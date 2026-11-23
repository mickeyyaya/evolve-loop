---
score_cap:
  - criterion: "evolve ship --paths ships a path whose deletion is already staged and exits 0"
    max_if_missing: 4
    evidence: "cd go && go test -tags integration -count=1 -run '^TestManualShipPaths_AlreadyStagedDeletionShipsAndExitsZero$' ./internal/phases/ship"
  - criterion: "a path in no index, no HEAD and no disk is refused with a message that names the path"
    max_if_missing: 6
    evidence: "cd go && go test -tags integration -count=1 -run '^TestManualShipPaths_PathKnownNowhereRefusesNamingThePath$' ./internal/phases/ship"
  - criterion: "a staged deletion and a modified path named together ship in one commit without an undeclared sibling"
    max_if_missing: 6
    evidence: "cd go && go test -tags integration -count=1 -run '^TestManualShipPaths_StagedDeletionAndModifiedPathShipTogether$' ./internal/phases/ship"
---

# Eval: evolve ship --paths accepts an already-staged deletion

> The manual ship class stages with `git add -A -- <paths>`. When a named path is a deletion that is already staged, the path is in HEAD only, and git exits 128 with `did not match any files`. The ship then fails with a message that does not name any path. This eval pins the fix. Source: inbox item `ship-paths-accepts-an-already-staged-deletion`, cycle 2624.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| staged-deletion | The deletion ships and the exit is 0 | 4/10 | `TestManualShipPaths_AlreadyStagedDeletionShipsAndExitsZero` |
| unknown-path | The refusal names the unknown path and leaves HEAD alone | 6/10 | `TestManualShipPaths_PathKnownNowhereRefusesNamingThePath` |
| mixed-set | A deletion and a modification ship together, no undeclared path | 6/10 | `TestManualShipPaths_StagedDeletionAndModifiedPathShipTogether` |
