---
score_cap:
  - criterion: "evolve inbox consume --resolution/--cycle stamps consumed{at,via,cycle,resolution} on the moved item"
    max_if_missing: 8
    evidence: "cd go && go test -count=1 -run '^TestRunInbox_Consume_WithFlagsStampsConsumedRecord$' ./cmd/evolve"
  - criterion: "evolve inbox consume with no flags still stamps the moved item, defaulting cycle to \"console\" and resolution to \"\""
    max_if_missing: 7
    evidence: "cd go && go test -count=1 -run '^TestRunInbox_Consume_NoFlagsDefaultsConsumedStamp$' ./cmd/evolve"
  - criterion: "a missing item still fails non-zero and stamps nothing, even when --resolution/--cycle are passed"
    max_if_missing: 5
    evidence: "cd go && go test -count=1 -run '^TestRunInbox_Consume_MissingItemWithFlagsStillFailsAndStampsNothing$' ./cmd/evolve"
  - criterion: "the two pre-existing zero-flag consume regression tests (move-then-ack invariant) still pass unmodified"
    max_if_missing: 6
    evidence: "cd go && go test -count=1 -run '^(TestRunInbox_Consume_MovesItemAndAcksFingerprint|TestRunInbox_Consume_ItemWithoutFingerprintStillMoves)$' ./cmd/evolve"
---

# Eval: evolve inbox consume writes the consumed{at,via,cycle,resolution} stamp

> Pins the public behavior of `evolve inbox consume <item> [--resolution X]
> [--cycle N]` introduced in cycle 1776. Before this fix, the CLI moved a
> consumed item into `.evolve/inbox/consumed/` and acked any named
> pipeline-defect fingerprint, but never wrote the `consumed{at, via, cycle,
> resolution}` stamp that `go/internal/inboxmover/continuation_release.go`
> already reads as an established on-disk convention — console operators had
> to hand-edit the moved file to add it. Source: inbox item
> `inbox-consume-records-resolution` (filed 2026-09-29), scouted in
> `.evolve/runs/cycle-1776/scout-report.md`.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| flagged-stamp | Flags produce a stamp with exact at/via/cycle/resolution values | 8/10 | `go test -run '^TestRunInbox_Consume_WithFlagsStampsConsumedRecord$' ./cmd/evolve` |
| default-stamp | No-flag call still stamps with documented defaults | 7/10 | `go test -run '^TestRunInbox_Consume_NoFlagsDefaultsConsumedStamp$' ./cmd/evolve` |
| negative-missing-item | Flags never bypass the missing-item failure path | 5/10 | `go test -run '^TestRunInbox_Consume_MissingItemWithFlagsStillFailsAndStampsNothing$' ./cmd/evolve` |
| regression-move-then-ack | Pre-existing zero-flag tests keep passing unmodified | 6/10 | `go test -run '^(TestRunInbox_Consume_MovesItemAndAcksFingerprint\|TestRunInbox_Consume_ItemWithoutFingerprintStillMoves)$' ./cmd/evolve` |
