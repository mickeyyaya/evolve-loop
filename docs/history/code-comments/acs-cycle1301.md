# Comment history: `acs/cycle1301`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1301/predicates_test.go:3` — above `package cycle1301`

```text
// Package cycle1301 materialises the cycle-1301 acceptance criteria for the one
// fleet-scoped task pinned to this lane:
//
//	T1 demotion-ledger-remedy-status-field
//	(todo-id demotion-ledger-records-salvage-attempted-vs-no-remedy-possible)
//
// What the cycle must deliver. The demotion ledger record — the JSON the
// triage capacity clamp auto-files at .evolve/inbox/auto-heuristic-demotion-
// triagecap-c<older>-c<newer>.json when the identical-rejection pattern fires
// (ADR-0046 Layer 2, go/internal/triagecap/demotion.go) — today carries only a
// prose `action` narrative. Nothing on the record says whether a salvage of the
// underlying gate defect was ATTEMPTED or whether the loop concluded NO REMEDY
// was possible; commit 29915424 had to explain two gate demotions in a queue
// chore commit body because the ledger itself cannot answer that. This cycle
// adds an explicit, caller-declared `remedy_status` field with a closed
// vocabulary {pending, salvage_attempted, no_remedy_possible} and an honest
// default (`pending` at file time — the demotion just fired, no remedy decision
// has been made yet), never a silent blank and never an unvalidated string.
//
// Predicate strategy — every predicate exercises real behaviour, never a source
// grep (the cycle-85 degenerate-predicate ban):
//
//   - 001 is the WIRING PROOF and the crux: it drives the REAL production
//     caller — triagecap.NewReviewer(config.StageEnforce).Review(...) over a
//     temp project whose state.json replays the 301/302 identical-rejection
//     pair — and asserts the ledger file the reviewer itself wrote carries
//     remedy_status=="pending". A predicate that called the record builder
//     directly would pass on dead code; this one stays RED until reviewer.go's
//     call site actually threads the field through.
//   - 002 is the regression guard: the SAME reviewer-written file must keep
//     every pre-existing field (id, action, priority, weight, relieved_cycle,
//     evidence_pointer, injected_by) at its current shape and value. An
//     additive field must not disturb what the ledger already promised.
//   - 003 pins the closed vocabulary and the normalisation contract through the
//     exported seam triagecap.NormalizeRemedyStatus: the three canonical values
//     survive verbatim; blank, unknown, wrong-case and whitespace-padded input
//     fall back to pending and are NEVER echoed verbatim (the negative case).
//   - 004 pins that the record BUILDER honours an explicitly declared terminal
//     outcome: a record built with salvage_attempted / no_remedy_possible
//     marshals those exact strings into the `remedy_status` JSON key, and a
//     junk status is normalised rather than written through.
//
// Fixture note: predicates 001/002 deliberately use the package's REAL seams
// (KnownPackages / readWindow / readFailedApproaches over a temp project root)
// rather than the in-package unexported test hooks — the external package can
// only reach the production constructor, which is exactly the reachability
// property being proven. The temp root holds no Go packages, so floor counting
// falls back to the min-1 prose rule: three floor-bearing ## top_n items count
// 3 against a cap of 2 (window K=1 ⇒ Cap=ceil(1.25)=2), which puts Review on
// its rejection path and therefore into the demotion consult.
```
