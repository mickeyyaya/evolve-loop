---
score_cap:
  - criterion: "evolve salvage list --json prints one row per salvage leaf, sorted by cycle, with leaf, cycle, branch, HEAD, changed-file count, patch bytes, untracked-file count, salvaged_at and a landed tri-state against origin/main, and writes nothing"
    max_if_missing: 4
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1803_009' ./acs/cycle1803"
  - criterion: "the text form prints one line per leaf carrying the full head, the age and landed=yes|no|unknown"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1803_010' ./acs/cycle1803"
  - criterion: "an unreadable leaf (malformed or missing HEAD, corrupt untracked.tgz) exits 2 naming it on stderr while every readable leaf is still listed"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1803_011' ./acs/cycle1803"
  - criterion: "an empty, missing or leafless salvage directory prints nothing and exits 0; usage errors exit 10"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1803_012' ./acs/cycle1803"
  - criterion: "gc's salvage writer, gc.ListSalvage and the CLI agree on one leaf salvaged by a real gc run, through the one exported gc.OperatorSalvageDir helper"
    max_if_missing: 5
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1803_013' ./acs/cycle1803"
  - criterion: "the salvage and failures usage lines name salvage list and prune --dry-run"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run 'TestC1803_014' ./acs/cycle1803"
---

# Eval: evolve salvage list

> Pins `evolve salvage list [--json]` (inbox item `cli-salvage-list`, cycle
> 1803). `evolve gc` salvages a dirty or unmerged cycle worktree into
> `.evolve/operator-salvage/<leaf>/{HEAD,uncommitted.patch,untracked.tgz}`
> before removing it. Before this cycle the operator inventoried that directory
> by hand: listing it, reading each HEAD and sizing each patch. The list is
> read-only, names every leaf it cannot read and exits 2 for it, and reads the
> layout through one path helper exported from internal/gc, so gc and the list
> cannot drift apart.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| json-inventory | per-leaf cycle/HEAD/patch stats/landed, sorted, read-only | 4/10 | `go test -tags acs -run 'TestC1803_009' ./acs/cycle1803` |
| text-inventory | one line per leaf with head, age, landed | 6/10 | `go test -tags acs -run 'TestC1803_010' ./acs/cycle1803` |
| loud-unreadable | exit 2 naming each bad leaf, readable leaves still listed | 5/10 | `go test -tags acs -run 'TestC1803_011' ./acs/cycle1803` |
| quiet-empty | no leaves means no output and exit 0 | 6/10 | `go test -tags acs -run 'TestC1803_012' ./acs/cycle1803` |
| one-path-owner | gc writer, ListSalvage and CLI agree through gc.OperatorSalvageDir | 5/10 | `go test -tags acs -run 'TestC1803_013' ./acs/cycle1803` |
| discoverability | usage lines name the new verbs | 8/10 | `go test -tags acs -run 'TestC1803_014' ./acs/cycle1803` |
