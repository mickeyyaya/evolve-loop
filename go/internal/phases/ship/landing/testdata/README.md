# Unit 07 goldens (ADR-0103 — the ship landing)

Captured on `8e8f080f` (2026-09-14) by a throwaway full-argv recorder BEFORE any
code moved, then deleted; the JSON goldens carry `captured_at` in their header
and `ship-binding.golden.json` is the writer's exact bytes. Both the host pins
(`internal/phases/ship/landing_pins_test.go`) and the leaf's own tests replay
these files. Never regenerate silently: a missing golden fails the pin.

- `push_direct|pushonly.golden.json` — two push sites × the repair matrix
- `ship-binding.golden.json` — the binding writer's bytes for a fixed body

## Amendment (2026-10-07, the two-phase landing)

- Retired: `landing_integrate.golden.json` and `push_worktree.golden.json`. They pinned the order that stranded cycle 1830: the ff-merge of `main` before the push. The worktree site now pushes the lane commit first (ADR-0039 §8.1). `internal/phases/ship/landing_order_test.go` pins the new order.
- Changed by hand in `push_direct` and `push_pushonly`: a rejection with no error no longer ends in `: <nil>`. The argv, the streams and the Debug keys are as captured.
