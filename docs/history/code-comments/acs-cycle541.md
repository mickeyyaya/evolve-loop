# Comment history: `acs/cycle541`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle541/predicates_test.go:3` — above `package cycle541`

```text
// Package cycle541 materialises the cycle-541 acceptance criteria for the single
// triage-committed (`## top_n`) task: triage-supply-disjoint-topn-for-fleet-width.
//
// TASK BINDING (R9.3 — predicates bind ONLY to triage `## top_n` work):
//
//	triage-report.md commits exactly ONE task to this lane:
//	  triage-supply-disjoint-topn-for-fleet-width (H) — C541_001..006
//	Every `## deferred` item (fix-treediff-guard-knowledge-base-carveout,
//	backfill-commit-orphaned-cycle-dossiers, and the rest of the out-of-fleet-
//	scope backlog) gets ZERO predicates here.
//
// FEATURE CONTEXT
//
//	Cycle-503 triage committed exactly ONE top_n task and starved the fleet wave
//	planner of the >=2 file-disjoint tasks it needs to fan out `fleet.count`
//	concurrent lanes. triagecap.SelectFleetWidthTopN (the SSOT greedy disjoint
//	packer) and its inbox-seed caller triagecap.SelectWaveSeedTopN already exist
//	and are GREEN, BUT they are wired ONLY into the wave planner's FALLBACK path
//	(cmd_loop_wave.go seedWavePlanFromInbox) — the branch that fires only when
//	the prior cycle's triage-decision.json is entirely ABSENT.
//
//	The GAP this cycle closes: a prior triage decision that IS present but
//	NARROW (fewer than `fleet.count` disjoint top_n items — e.g. THIS very
//	cycle's triage-report.md committed exactly 1) is read AS-IS by
//	productionWavePlanFn and never widened, so the fleet still collapses to a
//	single lane. The fix supplies a pure, single-sourced widening seam,
//	triagecap.WidenTopNToFleetWidth(committed, backlog, count), that backfills a
//	narrow committed set from the inbox backlog up to `count` MUTUALLY
//	FILE-DISJOINT lanes — never fabricating an overlapping pair — and wires it
//	into productionWavePlanFn so a narrow prior decision is widened before
//	fleet.PlanFromTriage partitions it.
//
// PREDICATE QUALITY (cycle-85): every load-bearing predicate EXERCISES the SUT —
// it CALLS the real triagecap / fleet functions and asserts on the returned
// selection / plan, never a "source file contains text X" grep.
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : C541_001 SelectFleetWidthTopN packs 2 disjoint candidates to 2.
//   - Negative : C541_002 all-overlapping candidates → widest disjoint set is 1,
//     never a fabricated overlapping pair (anti-no-op for the packer).
//   - E2E      : C541_003 a fleet-width decision → 2 disjoint CycleSpecs.
//   - Positive : C541_004 (RED driver) a NARROW committed set (1) + a disjoint
//     backlog candidate is WIDENED to 2 disjoint lanes — the un-starve.
//   - Negative : C541_005 (RED driver) widening a committed item with an
//     OVERLAPPING backlog item must NOT fabricate a 2nd lane — stays 1. This is
//     the strongest anti-no-op: a "pad to count regardless" impl FAILS here.
//   - Edge     : C541_006 (RED driver) count<2 preserves the committed set
//     byte-identically (legacy single-focus — no widening).
```
