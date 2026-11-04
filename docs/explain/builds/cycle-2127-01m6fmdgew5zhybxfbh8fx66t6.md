# Build Explanation — Cycle 2127

## Build Binding
- Cycle: 2127
- Base SHA: ca2f3ed59f6455db6a07ceb4f37ba4b448dfa451

## Summary
The inbox rank now scores freshness in place of age. Freshness decays from the premise verification date, or from the filing date when no premise was verified. The `inbox lint` verb warns about a declared path that is gone on main.

## Rationale
The old age boost raised the rank of the oldest items. Those items are the most likely to rest on a stale premise. A factor that falls with time since verification reverses that bias. A warn-only lint finds dead paths early and never blocks the queue.

## Changed Areas
- `go/internal/inboxbatch/item.go` — adds `PremiseVerifiedAt` and `VerifiedAt`, which parse the verification stamp.
- `go/internal/inboxrank/inboxrank.go` — renames the factor to `freshness` and scores 0.5 to the power of days over the window.
- `go/internal/policy/inbox_priority.go` — names the factor `freshness` and reads a legacy `age` key as its weight.
- `go/cmd/evolve/cmd_inbox.go` — adds `lintDeadPaths`, which warns on stderr and keeps the exit code.
- `go/cmd/evolve/cmd_inbox_rank_print.go` — prints the new factor name in the explanation.
- `go/cmd/evolve/cmd_inbox_lint_deadpath_test.go` — tests the dead-path warning.
- `go/cmd/evolve/cmd_inbox_rank_test.go` — updates the rank table expectations.
- `go/cmd/evolve/cmd_inbox_backfill_e2_test.go` — updates the factor name.
- `go/cmd/evolve/testdata/inbox-rank-all.golden.json` — regenerated for the new scores.
- `go/internal/inboxbatch/filed_test.go` — tests `VerifiedAt`.
- `go/internal/inboxrank/inboxrank_test.go` — tests the freshness curve.
- `go/internal/inboxrank/testdata/order-2026-10-06.golden` — regenerated for the new order.
- `go/internal/policy/inbox_priority_test.go` — tests the legacy key.
- `docs/architecture/packages/internal-inboxbatch.md` — states the premise verification fields.
- `docs/architecture/packages/internal-inboxrank.md` — states the freshness factor.
- `docs/architecture/packages/internal-policy.md` — states the legacy key rule.

## Design Decisions
One policy key change keeps an operator `policy.json` that still names `age` loadable. The lint reads the tracked tree, not the disk, so an untracked file counts as gone.

## Verification
The ACS suite for cycle 2127 reports red=0. The inboxbatch, inboxrank and policy packages pass.

## Compatibility
A policy file that names `age` keeps loading. The rank order of dated items changes: a new item now scores high, and an old unverified item scores low. An undated item scored 0 under the old age factor and still scores 0.

## Limitations
The lint does not fail the run. The freshness window reuses `age_halflife_days`.
