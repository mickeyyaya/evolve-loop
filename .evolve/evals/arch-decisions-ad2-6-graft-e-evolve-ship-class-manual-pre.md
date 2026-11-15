---
score_cap:
  - criterion: "evolve ship --class manual writes the Closes-Inbox trailer of the open item that the branch names"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestShipManual_WritesTheTrailerOfTheBranchItem$' ./internal/phases/ship"
  - criterion: "a message that already declares Closes-Inbox or Refs-Inbox keeps it, and a branch with no open item gets no trailer"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestShipManual_(KeepsATrailerTheMessageAlreadyDeclares|WritesNoTrailerWhenNoOpenItemNamesTheBranch)$' ./internal/phases/ship"
  - criterion: "the spec and the package note state the change"
    max_if_missing: 5
    evidence: "grep -Eq 'class manual.*Closes-Inbox|Closes-Inbox.*class manual' docs/architecture/inbox-curator.md && grep -Eq 'class manual.*Closes-Inbox|Closes-Inbox.*class manual' docs/architecture/packages/internal-phases-ship.md"
---

# Eval: evolve ship --class manual pre-fills the trailer from the item of the branch

> Pins component AD2-6 (Graft E) of `docs/plans/architecture-decisions-2026-11.md`. A console landing carries no `Closes-Inbox:` line today (0 of 1,684 commit bodies on `origin/main`, `docs/research/architecture-decisions-2026-11.md` F2.1), so the item stays open. The ship step now writes the trailer from the branch, and the curator consumes the item at the next boundary. Source incident: cycle 2391 (inbox item `arch-decisions-ad2-6-graft-e-evolve-ship-class-manual-pre`).

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| branch-item-trailer | The commit body closes the open item that the last part of the branch name names | 4/10 | `go test -run '^TestShipManual_WritesTheTrailerOfTheBranchItem$' ./internal/phases/ship` |
| declared-and-absent | A declared trailer stays; no open item means no trailer; an id prefix and a consumed item never match | 6/10 | `go test -run '^TestShipManual_(KeepsATrailerTheMessageAlreadyDeclares\|WritesNoTrailerWhenNoOpenItemNamesTheBranch)$' ./internal/phases/ship` |
| docs-state-the-change | The spec and the package note name the trailer | 5/10 | `grep -Eq 'class manual.*Closes-Inbox|Closes-Inbox.*class manual' docs/architecture/inbox-curator.md && grep -Eq 'class manual.*Closes-Inbox|Closes-Inbox.*class manual' docs/architecture/packages/internal-phases-ship.md` |
