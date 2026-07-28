# ADR-0078 — `fleet.landing`: the single-writer prefix-queue landing composer

- **Status:** Accepted (cycle-1144) — backfill of an already-landed surface
- **Relates to:** ADR-0057 (merge-to-main gate), ADR-0072 (system-failure floor),
  ADR-0077 (documentation floor for architecture-labeled changes), the
  `fleet.scheduling` dial (`wave` / `pool`)
- **Surface:** `.evolve/policy.json` `fleet.landing` — closed vocabulary
  `per-lane` (default) | `prefix-queue`
- **Implementation:** `go/internal/fleet/prefixqueue.go` (composer),
  `go/internal/phases/ship/planlanding.go` + `landprefixes.go` (landing plan),
  `go/internal/policy/policy.go` (resolution + fail-safe)

## Context

`fleet.landing` was added with the prefix-speculation landing queue
(inbox item `prefix-speculation-landing-queue`, campaign
`merge-efficiency-2026-07`) and shipped with **real resolved-config semantics
and zero documentation** — no entry in `control-flags.md`, none in
`runtime-reference.md`, no ADR. An operator reading either reference could not
discover the key, learn its vocabulary, or know when the non-default value is
the right choice.

That gap is the reason this ADR exists at all: it is the first enforced instance
of the [ADR-0077](0077-docs-floor-for-architecture-changes.md) documentation
floor, and the live proof the gap the floor closes was real rather than
hypothetical. The cycle-1144 classifier
(`docsfloor.IsArchitectureClass`) run over a change set shaped exactly like the
original landing — `go/internal/policy/policy.go` touched, no docs — flags it.

The engineering problem the composer solves is separate and older. With
`per-lane` landing, every PASS lane fast-forward-merges and pushes main itself,
serializing on `.evolve/ship.lock`. At fleet width ≥3 that produced two failure
classes: contention on the git index during concurrent landings (the cycles
981/982 lost-work class — an audit-PASSED lane whose work never reached main),
and whole-wave re-verification whenever any one lane turned out red, because
nothing localized the culprit.

## Decision

Make the landing strategy **policy config, not a feature flag**
(`no_feature_flags_use_design_patterns`): `fleet.landing` selects a landing
Strategy, mirroring `fleet.scheduling`'s `wave`/`pool` idiom.

### `per-lane` (default)

Today's behaviour, unchanged. Each PASS lane ff-merges and pushes main under the
ship lock.

### `prefix-queue`

A single-writer composer (`fleet.PrefixQueue`) owns the SOLE main-push path;
lanes never push. Modeled on Zuul / GitHub merge-queue prefix speculation:

1. PASS lanes enqueue as `LaneCandidate{ID, Tier, Files}` in FIFO order.
2. The composer builds candidate trees as queue **prefixes** — `L1`, `L1+L2`,
   `L1+L2+L3` — and verifies each against the native gate set.
3. The first failing prefix names the culprit **positionally** (Zuul NNFI — No
   New Failures Introduced): the lane at the boundary is the one that broke it.
   Lanes behind it re-form without it. **No bisection subsystem is needed** —
   the ordering does the localization.
4. The speculation window is an AIMD control loop: start 3, +1 per green, halve
   on red, floor 1.
5. Lanes are risk-tiered like Rust rollups. `TierIffy` (core / cross-cutting)
   lanes and any **overlap-zone** lane — one sharing a touched file with a lane
   already in the composing group — get a solo prefix slot, so the positional
   culprit signal stays unambiguous.

Because there is exactly one writer to main, the concurrent-push git-index
contention class is structurally impossible under this mode rather than merely
rarer.

### Fail-safe resolution

An unknown value resolves to `per-lane` — **never** the composer — plus a
surfaced warning naming the rejected value
(`policy.go`: `fleet.landing: unknown value %q, falling back to "per-lane"`).
This is the same fail-safe direction as `fleet.scheduling`: an operator typo
must never silently escalate the main-push path into the unsoaked strategy. The
default direction is the whole point — falling back to the *new* mode would make
a typo indistinguishable from an opt-in.

## Consequences

- **Operators** get a documented dial with an explicit when-to-choose-which rule
  (`runtime-reference.md`): stay on `per-lane` at width 1–2 or with infrequent
  landings, where speculation cost buys nothing; choose `prefix-queue` at width
  ≥3 with frequent landings, where lock contention and whole-wave
  re-verification dominate.
- **Speculation is not free.** Verifying `k` prefixes costs up to `k` gate runs
  per landing round. The AIMD window and the solo slots bound that cost; a queue
  that keeps going red collapses to window 1, which is `per-lane` throughput
  with a single writer.
- **The default did not change.** Absent block ⇒ `per-lane` ⇒ byte-identical to
  pre-composer behaviour, so this backfill documents a surface without moving
  any operator onto it.
- **The docs floor now has a worked example.** A future architecture-class
  landing that ships with no `docs/architecture/` or `runtime-reference.md`
  delta is flagged mechanically instead of being noticed cycles later by
  whoever next reads the code.
