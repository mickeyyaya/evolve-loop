# Build Explanation — Cycle 2464

## Build Binding
- Cycle: 2464
- Base SHA: 9199077a41a2a27797f7602371ac961752073ea3

## Summary
The wave plan now preempts one planned slot at a wave boundary. A higher-ranked item takes the lowest unstarted slot when its score lead reaches `inbox_priority.preempt_margin`. The ledger records the swap.

## Rationale
The rank and the margin already existed, but no code read `preempt_margin`. The wave reused the previous decision and kept a stale order. One swap per plan is the smallest change that gives the item behavior (inbox plan P4).

## Changed Areas
- `go/internal/triagecap/preempt.go` — adds the swap decision and the ledger record, next to the ranked backlog reader that already owns the rank inputs.
- `go/internal/loopwave/plan.go` — calls the swap after the prune and the widen, and writes the ledger line only on a real plan, never on a preview.
- `go/internal/triagecap/preempt_test.go` — tests the swap rules and the ledger line at the triagecap seam.
- `go/internal/loopwave/preempt_record_test.go` — tests that a failed ledger write warns and keeps the swap.
- `docs/architecture/packages/internal-triagecap.md` — records the swap rules and the new exports.
- `docs/architecture/packages/internal-loopwave.md` — records the call site and the preview rule.

## Design Decisions
The swap lives in `triagecap`, not in `loopwave`. `loopwave` is a leaf with a fixed import list, and `triagecap` already imports the rank packages and the ledger path. A running lane is a claimed file, so it is not in the queue and never a victim. An adopted continuation carries a `continuation` field and is skipped. Only one slot changes for each plan, and a candidate that shares a file with a kept slot is skipped, so the lanes stay file-disjoint.

## Verification
The cycle predicates `TestC2464_001` to `TestC2464_005` run the loopwave and cmd/evolve tests. The cmd test goes through the production plan function. The new triagecap tests cover the empty plan, the narrow lead, the selector, overlap and an unreadable inbox. loopwave coverage stays at 100.0%.

## Compatibility
No signature changes. A decision with `committed_floors` is not touched. The default margin stays 0.05 from the existing policy.

## Limitations
A later wave that reuses the same decision can write the same preempt line again. The plan is not persisted after the swap.
