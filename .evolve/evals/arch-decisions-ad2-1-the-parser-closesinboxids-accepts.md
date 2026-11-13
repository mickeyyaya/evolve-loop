---
score_cap:
  - criterion: "The inboxmover facade exposes RefsInboxIDs and the named red tests pass"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestClosesInboxIDs_NoneIsAnExplicitEmptySet|TestRefsInboxIDs_ReadsItsOwnMarkerOnly|TestRefsInboxIDs_NoneIsNotAnID)$' ./internal/inboxmover"
  - criterion: "Closes-Inbox none gives an explicit empty set through the facade"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestClosesInboxIDs_NoneIsAnExplicitEmptySet$' ./internal/inboxmover"
  - criterion: "The package note names RefsInboxIDs"
    max_if_missing: 4
    evidence: "grep -q RefsInboxIDs docs/architecture/packages/internal-inboxmover.md"
---

# Eval: inboxmover facade for the Refs-Inbox trailer

> Pins the `inboxmover` facade of the AD2-1 trailer grammar: `ClosesInboxIDs` treats `Closes-Inbox: none` as an explicit empty set, and `RefsInboxIDs` reads the `Refs-Inbox:` marker only. The leaf `closesinbox` already holds the parser; the facade lets the cmd and ship layers call it without importing the leaf. Source: cycle 2358, plan AD2-1.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| facade-refs | RefsInboxIDs exists and reads its own marker | 6/10 | `go test -run TestRefsInboxIDs ./internal/inboxmover` |
| none-empty-set | none is never an id | 7/10 | `go test -run TestClosesInboxIDs_NoneIsAnExplicitEmptySet ./internal/inboxmover` |
| note | package note states the change | 4/10 | `grep -q RefsInboxIDs docs/architecture/packages/internal-inboxmover.md` |
