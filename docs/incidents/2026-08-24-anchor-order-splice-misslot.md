# 2026-08-24 — Anchor-order-sensitive splice mis-slots planned phases (cycle-1550)

## Symptom

soak-20260824a wave 1, cycle-1550: `phase-plan.json` AND `phase-replan.json`
both scheduled `bug-reproduction` at index 3 (after fault-localization, ahead
of tdd=4 and build=5), but the executor ran it EIGHTH — after build sealed.
The phase wrote its red-first reproduction test
(`TestArtifactDetector_PreExistingArtifactRequiresPostDispatchWrite`) 17
minutes after `build-report.md`, no builder remained to green it, and audit
correctly FAILed the lane on the RED test. The auto-escalation's canned
`repro_hint` ("the verdict-surface path forged a negative verdict") was WRONG
for this instance — the audit verdict was legitimate; the defect is upstream.

## Root cause

Execution order is `cfg.Order` (`effectiveOrder`), never the plan's order —
the plan gates membership only. `cfg.Order` is assembled by
`ApplyUserRouting`/`spliceAfter` over the discovered specs, which arrive
ALPHABETICALLY sorted (`DiscoverUserSpecs`). A spec whose `after:` anchor is
an alphabetically-LATER catalog phase found its anchor absent from the order
at splice time and silently took the before-audit fallback slot:
`bug-reproduction` (after: `fault-localization`) sorts before its anchor →
spliced first → anchor miss → placed just before audit = post-build. FOUR
tracked anchors were alphabetically inverted: `bug-reproduction →
fault-localization` (the catastrophic one — post-build red-first),
`cleanup-sweep → mutation-gate`, `benchmark-gate → perf-profile`, and
`mutation-gate → test-amplification` (so `cleanup-sweep → mutation-gate →
test-amplification` is a live 2-deep chain that specifically needs the
multi-pass fixpoint). The mis-slot was invisible for the life of the catalog
because the fallback was silent.

## Fix

Placement inside `ApplyUserRouting` (`go/internal/phasespec/routing.go`) is a
FIXPOINT over the batch: each pass places specs whose anchor is empty or
already present; deferred specs retry while any pass makes progress; a
permanently-unresolvable anchor takes the before-audit fallback LOUDLY. The
escape distinguishes three shapes: a truly-absent anchor force-places its spec
(warned); a spec anchored to a stuck batch-mate is HELD so it still lands
after its anchor once that one places; an anchor deadlock (cycle) force-places
exactly one dependency target (warned) and lets the rest — cycle members and
tails alike — resolve honorably on later passes. An activation overlay whose
name is already in the order stays silent regardless of its anchor (nothing
moves). Trigger/enable
registration keeps input order; empty-anchor and anchor-present behavior is
byte-identical. The mint path (single-spec calls) keeps its semantics for
present anchors and now warns on absent ones.

## Regression pins

`internal/phasespec/routing_anchor_fixpoint_test.go` — the cycle-1550 shape
(alphabetical batch, later-spec anchor honored), a 3-deep fully-reversed
anchor chain, unresolvable-anchor warn+fallback, anchor-cycle termination.
`cmd/evolve/routing_order_realtree_test.go` — layer-N+1 wiring over the REAL
production composition (config.Load + discoverUserSpecsClamped +
ApplyUserRouting on the tracked phase tree): every anchored phase follows its
anchor, plus a named pin that `bug-reproduction` sits after
`fault-localization` and before `build` (red on the unfixed code with both
victims' names). Mutation-tested: 4 mutants, all killed.

## Related open design

The red-first-deliverable class itself (a lane shipping a deliberately-failing
test) is item `red-first-deliverable-reds-main` (three occurrences; this
incident is the third and supplied the mechanism). This fix removes the
manufacturing defect (planned-pre-build phases can no longer silently execute
post-build); the ship-gate-vs-t.Skip convention question in that item stays
open.
