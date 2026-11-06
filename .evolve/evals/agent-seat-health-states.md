---
score_cap:
  - criterion: "A pane at its prompt with no deliverable reads blocked in wave status"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestWaveStatus_SeatHealth_APaneAtItsPromptWithNoDeliverableReadsBlocked$' ./cmd/evolve"
  - criterion: "A dead session with an open run reads zombie, and an unreachable socket never does"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestWaveStatus_SeatHealth_ADeadSessionWithAnOpenRunReadsZombie|TestWaveStatus_SeatHealth_AnUnreachableSocketNeverReadsZombie)$' ./cmd/evolve"
  - criterion: "The seat view reads tmux only through the list seam and changes no file"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestWaveStatus_SeatHealth_TheViewReadsTmuxThroughTheListSeamAndChangesNothing$' ./cmd/evolve"
  - criterion: "The pure classifier names one state per fact set"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestSeatHealth_' ./internal/bridge/panestream"
  - criterion: "A dead session that was the last on its tmux socket reads zombie, against a real tmux server too"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestWaveStatus_SeatHealth_ADeadSessionTheLastOneOnItsSocketReadsZombie|TestWaveStatus_SeatHealth_AListFailureThatIsNotAGoneServerNeverReadsZombie|TestWaveStatus_SeatHealth_ARealTmuxSocketWhoseLastSessionDiedReadsZombie|TestWaveStatus_SeatHealth_ARealTmuxSessionAmongOthersReadsBlockedAtItsPrompt)$' ./cmd/evolve"
  - criterion: "One function owns the list-sessions call for the bridge view and the seat view"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^TestBridgeTmuxListing_OneFunctionOwnsTheListSessionsCall$' ./cmd/evolve"
  - criterion: "The spec says the view starts no server, seat or agent"
    max_if_missing: 5
    evidence: "! grep -q 'starts no process' docs/architecture/regression-proof-and-supervision.md && grep -q 'starts no server' docs/architecture/regression-proof-and-supervision.md"
  - criterion: "A deliverable written by an earlier round never hides the state of this round, and one written by this round reads idle"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestWaveStatus_SeatHealth_ADeliverableOfAnEarlierRoundNeverHidesTheStateOfThisRound|TestWaveStatus_SeatHealth_ADeliverableWrittenByThisRoundReadsIdle)$' ./cmd/evolve"
  - criterion: "A tmux list failure that is not a gone server reaches the operator as a warning, and a gone server or a healthy listing raises none"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestWaveStatus_SeatHealth_AListFailureThatIsNotAGoneServerWarnsTheOperatorAndKeepsTheSeat|TestWaveStatus_SeatHealth_AGoneServerIsAZombieNotAWarning|TestWaveStatus_SeatHealth_AHealthyListingRaisesNoWarning)$' ./cmd/evolve"
  - criterion: "A tmux list failure with no stderr leaves no empty reason after the separator"
    max_if_missing: 4
    evidence: "cd go && go test -count=1 -run '^TestBridgeTmuxListing_AFailureWithNoStderrNamesNoEmptyReason$' ./cmd/evolve"
  - criterion: "A real tmux session is listed under a C locale, where tmux rewrites a tab to an underscore"
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^(TestWaveStatus_SeatHealth_ARealTmuxSessionReadsBlockedUnderACLocale|TestBridgeSessions_ARealTmuxSessionListsAsUnwatchedUnderACLocale)$' ./cmd/evolve"
---

# Eval: Each seat shows one named health state

> Pins the seat health view of `evolve wave status`: working, blocked, stalled, zombie and idle, derived from the pane-watch snapshot, the tmux session list and the deliverable file. Source incident: a pane that waits for an answer read busy-stagnant (research F6.4; docs/architecture/regression-proof-and-supervision.md section 9). Cycle 2172.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| blocked-journey | prompt without deliverable reads blocked | 6/10 | `go test -run APaneAtItsPrompt ./cmd/evolve` |
| zombie-journey | dead session with open run reads zombie | 6/10 | `go test -run ADeadSession ./cmd/evolve` |
| read-only | only the list seam, no file change | 7/10 | `go test -run TheViewReads ./cmd/evolve` |
| classifier | pure state table | 6/10 | `go test -run TestSeatHealth_ ./internal/bridge/panestream` |
| last-session-dies | gone tmux server reads zombie (audit round 1, H1) | 7/10 | `go test -run RealTmux ./cmd/evolve` |
| one-lister | one list-sessions owner (audit round 1, M2) | 6/10 | `go test -run OneFunctionOwns ./cmd/evolve` |
| spec-wording | spec names what the view starts (audit round 1, M1) | 5/10 | `grep starts no server` |
| stale-deliverable | an earlier round's report never hides this round (audit round 2, M1) | 6/10 | `go test -run ADeliverableOfAnEarlierRound ./cmd/evolve` |
| list-failure-warning | a non-gone list failure warns (audit round 2, M2) | 6/10 | `go test -run AListFailureThatIsNotAGoneServer ./cmd/evolve` |
| empty-reason | no trailing separator on empty stderr (audit round 2, L1) | 4/10 | `go test -run AFailureWithNoStderr ./cmd/evolve` |
| c-locale | tmux tab rewrite under a C locale (integration tier, cycle 2172) | 7/10 | `go test -run UnderACLocale ./cmd/evolve` |
