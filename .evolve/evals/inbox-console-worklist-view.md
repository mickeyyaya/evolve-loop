---
score_cap:
  - criterion: "`evolve inbox batches` renders console-routed items separately from the selectable lane listing, each carrying inboxbatch.PartitionConsole's own reason (explicit route field AND protected-files derivation)"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1678_001_InboxBatchesSeparatesConsoleRoutedItemsWithReasons ./acs/cycle1678"
  - criterion: "The separation is conditional on a real partition result — a backlog with nothing operator-owned renders every id inside the lane listing and invents no routing reason"
    max_if_missing: 8
    evidence: "cd go && go test -tags acs -count=1 -run TestC1678_002_LaneOnlyInboxKeepsEveryItemInTheBatchListing ./acs/cycle1678"
  - criterion: "Both `evolve inbox batches` output paths carry the same partition — a console section wired into the text renderer only leaves every machine consumer reading operator-owned work as dispatchable"
    max_if_missing: 7
    evidence: "cd go && go test -tags acs -count=1 -run TestC1678_011_JSONPathCarriesTheSamePartitionAsTheTextPath ./acs/cycle1678"
  - criterion: "A console-routed item is a TERMINAL bucket: `evolve inbox-mover claim` refuses it with exit 3 and states why, while a dispatchable item still claims with exit 0"
    max_if_missing: 6
    evidence: "cd go && go test -tags acs -count=1 -run TestC1678_004_ConsoleRoutedItemsAreUnclaimableByALane ./acs/cycle1678"
---

# Eval: the operator's own inbox command shows console-routed work as console-routed

> Pins the observable contract of `evolve inbox batches` once ADR-0074's routing
> authority is in play. The routing itself is already typed plumbing and has
> been since cycles 1034–1036: `inboxbatch.PartitionConsole` splits the backlog
> into lane-dispatchable and console-routed (operator-owned) with one
> human-readable reason per routed item; triage consumes it
> (`internal/phases/triage/triage.go` `inboxBatchesSection`), and
> `inboxmover.Claim` refuses a lane draw of an operator-owned item with exit 3.
>
> The command the OPERATOR actually types did not. `cmd_inbox.go` ran
> `inboxbatch.Classify` over every loaded item, so `evolve inbox batches`
> presented operator-owned work as a selectable lane batch with no reason and no
> separation — the single-backlog design (console-routed items stay physically
> in `.evolve/inbox`, which is correct: one view, no claim races) read as "all of
> this is yours to pick". ADR-0074 strong review recorded it as finding 8;
> cycle-1678 is where it was closed.
>
> The caps are written against BEHAVIOUR, not prose. Nothing here dictates a
> section heading: what is pinned is that the routed ids leave the `- batch …`
> selectable listing, that each arrives with the partitioner's OWN reason
> (`route:console-…` / `protected fix surface: <path>`), and that a backlog with
> nothing routed produces neither a routed id nor an invented reason. The
> conditional criterion is capped hardest because it is the anti-no-op half: an
> implementation that always prints a console heading, or that sweeps every item
> out of the lane listing, satisfies the first criterion and is still wrong.
> The claim-floor criterion doubles as the ProtectedSurfaceManifest pin —
> `go/internal/guards/role.go` leaving the manifest surfaces here as a loud
> failure rather than a silent pass.

## Score Cap Rationale

| Pattern | Criterion | max_if_missing | Evidence |
|---|---|---|---|
| console-section-with-reasons | Routed items leave the selectable listing and carry the partitioner's reason | 7/10 | `go test -tags acs -run TestC1678_001… ./acs/cycle1678` |
| conditional-not-boilerplate | A lane-only backlog renders no routed id and no invented reason | 8/10 | `go test -tags acs -run TestC1678_002… ./acs/cycle1678` |
| both-output-paths | `--json` carries the same partition as the text renderer (#373) | 7/10 | `go test -tags acs -run TestC1678_011… ./acs/cycle1678` |
| terminal-bucket-enforced | `inbox-mover claim` refuses routed ids (exit 3) and still claims lane work (exit 0) | 6/10 | `go test -tags acs -run TestC1678_004… ./acs/cycle1678` |
