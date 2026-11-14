# Build Explanation — Cycle 2386

## Build Binding
- Cycle: 2386
- Base SHA: 671e768a8abf1c83633ac6ed6b79457a4f3483ce

## Summary
Post-ship inbox promotion now filters candidate IDs against host-held cycle locks and the lane-scope pin, and logs a warning for each refused ID.

## Rationale
Unfiltered agent-written lists (`top_n`, `skip_shipped`, `superseded`, and `Closes-Inbox` markers) previously reached `Mover.Promote`, which resolves items across all `processing/cycle-*/` directories. Filtering at the promotion boundary blocks cross-lane item theft without modifying protected `go/internal/inboxmover/inboxmover.go`.

## Changed Areas
- `go/internal/inboxmover/outcome.go` — adds `FilterPromotionScope` to enforce cycle ownership and `lane-scope.json` pin membership, and guards `ApplyCycleOutcome` against promoting items held in another cycle's `processing/` directory.
- `go/internal/phases/ship/postship.go` — filters both `committedInboxIDs` and `SupersededInboxIDs` through `inboxmover.FilterPromotionScope` in `promoteInbox` and emits a `[ship] WARN:` log line for each refused ID.

## Design Decisions
Placing `FilterPromotionScope` in `go/internal/inboxmover/outcome.go` preserves the ADR-0079 boundary where only `inboxmover` imports `inboxmover/lifecycle`. Non-pinned cycles still promote items in the inbox root or their own `processing/cycle-<cid>/` directory, while always refusing items held by another cycle.

## Verification
Targeted unit and journey tests in `go/internal/phases/ship` (`TestPromoteInbox_*`), `go/internal/inboxmover/...`, `go/internal/cycleoutcome/...`, and `go/acs/cycle2386/...` all pass.

## Compatibility
Existing non-lane cycles without `lane-scope.json` continue to promote root and cycle-held items, and `ApplyCycleOutcome` retains its signature.

## Limitations
`ReconcileSuperseded` in protected `go/internal/inboxmover/inboxmover.go` remains unchanged; its callers in `postship.go` are guarded instead.
