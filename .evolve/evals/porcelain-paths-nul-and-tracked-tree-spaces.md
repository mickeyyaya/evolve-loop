---
score_cap:
  - criterion: "QuarantineDirtyTree stashes a non-ASCII dirty file and leaves the tree clean"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2474_00[123]_' ./acs/cycle2474"
  - criterion: "QuarantineDirtyTree still stashes dirty names with edge spaces"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2474_004_' ./acs/cycle2474"
  - criterion: "TrackedTree, TrackedFiles and TrackedSet return names exactly as git stores them"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC2474_00[567]_' ./acs/cycle2474"
---

# Eval: Path handling with NUL-separated git output

> Pins two defects found by coverage backfill A: `porcelainPaths` in
> `go/internal/core/boot_preflight.go` and `TrackedTree` in
> `go/internal/repostate/tracked.go` read git output without `-z`. Git quotes
> non-ASCII names, and `strings.TrimSpace` strips spaces that belong to names.
> Source: inbox item backfill-a-path-handling-defects-lane, cycle 2474.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| non-ASCII quarantine | dirty é.go is stashed, loop-managed non-ASCII paths stay dirty | 7/10 | `go test -tags acs -run 'TestC2474_00[123]_' ./acs/cycle2474` |
| edge-space quarantine | names with leading or trailing spaces are stashed | 6/10 | `go test -tags acs -run 'TestC2474_004_' ./acs/cycle2474` |
| tracked-tree names | the three tracked-file readers keep exact names | 7/10 | `go test -tags acs -run 'TestC2474_00[567]_' ./acs/cycle2474` |
