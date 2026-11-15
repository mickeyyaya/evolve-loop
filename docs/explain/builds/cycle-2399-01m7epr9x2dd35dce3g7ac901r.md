# Build Explanation — Cycle 2399

## Build Binding
- Cycle: 2399
- Base SHA: 49bbd538137ee7b8f2ecce08450cb296de484c77

## Summary
An inbox item can now declare typed probes. The intake refuses an item whose probes all pass. The boundary sweep consumes such an item and proposes a quarantine for an item whose declared paths are all gone.

## Rationale
Merged work kept its rank in the top 12 of the inbox. A probe is a fact that Go can check, so the curator needs no LLM to find done work. The quarantine is a judgment proposal, because a gone path does not prove that the work is done.

## Changed Areas
- `go/internal/inboxbatch/probes.go` — adds the `Probe` type, the three kind spellings and `ProbeKinds`.
- `go/internal/inboxbatch/item.go` — adds the `Probes` field to `Item`.
- `go/internal/inboxcurator/probes.go` — adds `Tree` and `Tree.ProbesPass`, the one place that checks a probe.
- `go/internal/inboxcurator/intake.go` — `CheckIntake` validates probes. `Admit` refuses an item whose probes pass, with `ErrProbesPass`.
- `go/internal/inboxcurator/landing.go` — `Sweep` takes a `Tree`, proposes a deterministic consume for passing probes and a judgment quarantine for gone paths. Adds `ProposeQuarantine`.
- `go/internal/inboxcurator/curate.go` — `Inputs` gains `PathExists`. The proposal signal text names the quarantine.
- `go/cmd/evolve/cmd_inbox_add.go` — builds the `Tree` for the intake. It indexes tests only when the item has probes.
- `go/cmd/evolve/cmd_inbox_curator.go` — gives `Curate` a path check rooted at the project.
- `docs/architecture/inbox-curator.md` — adds decisions D9 and D10.
- `docs/architecture/packages/internal-inboxcurator.md` — states the probe intake and the `Sweep(open, tree, minShare)` sweep in STE, removing the stale signature and over-long paragraph.
- `docs/architecture/packages/internal-inboxbatch.md` — states the probe type and its three kinds.
- `docs/architecture/signal-codes.md` — documents `quarantine` on `INBOX_CURATOR_PROPOSAL`.

## Design Decisions
`Tree` bundles the two tree questions, so `Rules` and `Sweep` take one value. An unknown kind never passes. An item with no probes never asks the tree. The probe refusal comes before the overlap judgement, and no override lifts it.

## Verification
The unit tests in `inboxbatch`, `inboxcurator`, and `cmd/evolve`, and the cycle 2399 predicates cover each new branch. The `inboxcurator` package has 100% statement coverage, and all three architecture docs pass `ste-lint --strict` with 0 findings.

## Compatibility
An item without probes behaves as before. The `probes` key is omitted from the JSON when empty.

## Limitations
Only three probe kinds exist. The quarantine is never applied by the curator.
