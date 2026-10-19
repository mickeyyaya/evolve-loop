---
score_cap:
  - criterion: "Two branches that archive comment history into adjacent pages merge with no conflict in the index"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run TestC1956_001_AdjacentPagesFromTwoBranchesMergeWithoutConflict ./acs/cycle1956"
  - criterion: "Two branches that append a section to the same page merge with no conflict"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1956_004_SamePageSectionsFromTwoBranchesMergeWithoutConflict ./acs/cycle1956"
  - criterion: "The merged index lists every page and carries its entry count"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1956_002|TestC1956_003' ./acs/cycle1956"
  - criterion: "Union merge stays on the history archive and go/internal/commentaudit stays untouched"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1956_005|TestC1956_006' ./acs/cycle1956"
---

# Eval: The generated comment-history index conflicts on every parallel merge (lane part)

> Pins the lane half of inbox item `history-index-merge-conflict-lane`. Each predicate runs the real `commentaudit history` in a `gittest` fixture on two branches and merges them with git, so the repo `.gitattributes` decides the result. Source incident: console boundary train w100, 2026-10-16, where seven branches conflicted on `docs/history/code-comments/README.md`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| adjacent-index-rows | Adjacent page rows merge without conflict | 5/10 | `go test -tags acs -run TestC1956_001 ./acs/cycle1956` |
| same-page-append | Same-page sections merge without conflict | 6/10 | `go test -tags acs -run TestC1956_004 ./acs/cycle1956` |
| index-coverage | Merged index lists every page with its count | 6/10 | `go test -tags acs -run 'TestC1956_002|TestC1956_003' ./acs/cycle1956` |
| scope-fence | Union stays on the archive, protected package untouched | 7/10 | `go test -tags acs -run 'TestC1956_005|TestC1956_006' ./acs/cycle1956` |
