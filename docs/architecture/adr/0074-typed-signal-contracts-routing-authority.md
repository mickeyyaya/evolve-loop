# ADR-0074: Typed signal contracts — routing authority, verified consumers, transactional evidence

- **Status:** Accepted (slice 1 implemented; slices tracked in `.evolve/inbox`)
- **Date:** 2026-07-22
- **Deciders:** operator (console session) + strong review (commit-gate dual review)
- **Supersedes/extends:** builds on ADR-0072 (system-failure policy), ADR-0073
  (mint registry), and the cycles 215–231 retrospective's coherence diagnosis.

## Context

Batch-5 (cycles 1028–1037) closed with zero pipeline false-REDs — the
v22.4–v22.7 hardening held — yet three of its four FAILs were one disease:
lanes drew tasks whose fix surface is the pipeline's own **control plane**
(`guards.ProtectedSurfaceManifest` paths a cycle is forbidden to write).
Cycle-1034 built the disposition gate but could not enroll it in the manifest;
1035 drew the guard-phase-hook task (fix = `.claude/settings.json`); 1036 drew
the role.go allowance (fix = a protected guard file). Recurrence on the class:
4+ across two weeks (cycle-858 was the first recorded instance).

The routing knowledge existed every time — in retro preventive actions, in
operator annotations (`route: console-manual` prose), in review findings — but
**no component consumed it**. Investigation found the same producer-without-
consumer shape across the open pipeline defects:

- `internal/retrofile` (built cycle 657 to auto-file retro preventive actions):
  zero production callers; every FAIL's structured recommendations evaporate.
- `internal/recurrence` Escalator/Autofiler (cycle 661): defined, unwired.
- Carryforward/prune API (cycle 962), `LandPrefixes` (975 era): shipped inert.
- Reviewer flagged the state RMW race MEDIUM in cycle-992 — nothing escalated
  it; it destroyed data in cycle-1001.
- Cycle-948 recorded PASS; its commit never landed; the driving item was
  consumed anyway (consumption fires on *pick*, not on *land*).
- FAIL-cycle dossiers record `retro: skipped` while `retrospective-report.md`
  exists on disk.

## Decision

One architectural principle, three enforced invariants:

> **Signals that cross the control-plane/work-plane boundary must be typed,
> enforced contracts — never prose, convention, or an artifact nobody reads.**

### I1 — Routing authority is typed and enforced at every handoff

Every inbox item has a dispatch authority: **lane** (default) or **console**
(operator-owned). It is carried by a first-class `route` field
(`console-manual`, `console-salvage`, `lane` override) and *derived* for the
statically-blocked class: any declared fix-surface file matching
`guards.IsProtectedSurface` console-routes the item automatically.

Enforcement lives where selection authority actually crosses into dispatch —
the **plan-time gate**: `triage-decision.json`'s `top_n`/`committed_floors` is
the load-bearing selection→build handoff, and `fleet.TodosFromTriage` (single-
sourced by the wave scheduler, the pool scheduler, the min-width repair path,
and the wave-seed-inbox fallback) refuses console-routed ids via a required
`RoutedFn` parameter, loudly (WARN per refusal at the composition root). All
layers share ONE classifier (`inboxbatch.ConsoleRouted`/`PartitionConsole`/
`RoutedResolver`):

1. **Plan-time gate (binding)** — a console-routed id never becomes a lane
   todo, whichever scheduler consumes the decision.
2. **Handoff backstop (binding)** — `inboxmover.Claim` refuses console-routed
   items with typed `ErrConsoleRouted` (CLI exit 3); the triage agent doc +
   permission profile bind the claim step to `evolve inbox-mover claim`
   (wiring-pinned by test — the prior doc invoked a deleted script, an I2
   violation caught in this ADR's own strong review).
3. **Visibility (advisory only)** — triage prompt composition partitions the
   backlog and lists exclusions (`console_routed_excluded: …ids`). This is
   UX, not enforcement: the agent enumerates the raw inbox directory, so the
   prompt hint alone can never be the control.

An id the LLM mis-picks anyway is refused at plan time; an item claimed out of
band is refused at the mover.

### I2 — Every signal producer has a verified live-path consumer

A mechanism ships only with proof that its output is consumed on the composed
production path (wiring proof), the same way apicover already forbids
uncalled exports. Applies retroactively to the inert set: retrofile,
recurrence Escalator/Autofiler, carryforward/prune. The failure-disposition
router (queued 0.96; S1+S2 salvaged from cycle-1034) is the unifying consumer
for retro output: mandatory `disposition.json` + deterministic routing floors
+ boundary applier.

### I3 — State transitions are transactional with their evidence

Recorded state must be entailed by on-disk/git evidence at the moment of
recording: PASS ⇒ commit reachable from main (ship-landed floor, queued 0.96);
item consumed ⇒ work landed (transactional consumption); dossier phase records
⇒ phase artifacts exist (dossier-retro mislabel fix, queued).

## Consequences

- Control-plane work becomes an explicit **operator lane**: console-routed
  items surface in the triage prompt's exclusion note and are worked at batch
  boundaries through the sanctioned manual-ship flow (commit-gate → ship).
  A cycle that legitimately *creates* a new gate-shaped file still FAILs the
  protectedsurface tripwire (the security floor is untouched) — but the
  disposition router (I2) turns that into a structured operator handoff
  instead of a discarded cycle.
- False-positive derivations (an item that only *reads* a protected file) are
  overridden explicitly with `route: "lane"` — with an ADR-0073 clamp-parity
  floor: an **agent-autofiled** item (`injected_by` set) cannot lane-override a
  protected derivation, so agent-authored fields never widen agent authority.
  The `route` field itself is unauthenticated (sandbox-off, nothing on disk is
  — ADR-0073's conclusion); the accepted residual is that a forged override at
  most forces a *doomed* pipeline, because the ship-time protectedsurface
  tripwire still blocks the merge. The inverse gaming vectors (self-setting
  `route:console` or padding `files[]` with a protected path to dodge lane
  work) are bounded the same way: they bury an item in the loud, operator-
  reviewed exclusion list rather than making anything mergeable.
- The derivation matches only surfaces **already on the manifest** and scans
  every token of each `files[]` entry. A task that will *create* a new
  gate-shaped file (the cycle-1034 shape) is out of the derivation's reach by
  construction — that class is handled by the ship tripwire plus the
  disposition router's structured operator handoff (I2), not by routing.
- The pass-rate metric stops being polluted by structurally-doomed draws; the
  remaining FAIL budget is reserved for honest rejections.

## Slice map (implementation tracking)

| Slice | Invariant | Status |
|---|---|---|
| Routing classifier + **plan-time gate** (`TodosFromTriage` RoutedFn, all schedulers) + claim backstop + agent-doc/profile claim binding | I1 | **implemented (this ADR's PR)** |
| Triage prompt partition + exclusion note | I1 (advisory UX) | implemented (this ADR's PR) |
| Disposition router S1+S2 (assembler + schema gate; cycle-1034 salvage + manifest enrollment) | I2 | console boundary work |
| Disposition router S3+S4 (floors/router + boundary applier; wires retrofile/recurrence) | I1+I2 | queued 0.96 |
| Ship-landed floor: postship landing gate + `IsLandedFn` are live; transactional item consumption remains | I3 | partially live; remainder queued 0.96 |
| Pre-dispatch AC authoring guard (scout/TDD operator-action disposition) | I1 | queued 0.90 |
| Operator worklist surface (`evolve inbox` console section, stable terminal bucket) | I1 ergonomics | queued 0.70 |
| Dossier record coherence (retro-skipped mislabel) | I3 | queued 0.60 |

## Alternatives considered

- **Prompt-level instruction only** ("triage: skip console items") — rejected:
  prose is the failure mode this ADR exists to end; the 1036 wave read a
  prompt composed before the annotation existed.
- **Weight suppression** (drop console items below the lane band) — rejected:
  weight encodes priority, not authority; conflating them hides operator work.
- **Hard delete from inbox to a side directory** — rejected: loses the single
  backlog view and the triage-visible exclusion note; racy with claims
  mid-wave.
