# Option 1 — One design card; the guard sits in the ship step

Citations: [assumptions-and-evidence.md](../assumptions-and-evidence.md).

## What changes

A single card, `docs/architecture/integration-driver-enforce.md`, defines `EnforceActor` with `Resume`, `Flush` and `WaveEnd`. It decides each section 9 input with fact, decision and consequence (E1). The `delivered` inbox state holds the item off the menu (E3). The checkpoint carries a lane identity, and the ship step refuses a non-lane landing at enforce, because every resume path meets the ship step (A3).

## Causal chain to the goal

Acceptance 1 needs a landing actor (E2, E4). Acceptance 2 needs a state the menu and the sweep both read (E3). Acceptance 3 needs one check at the point where all paths meet. A check at one point cannot be skipped by a new resume path.

## Expected effect

- 7 of 7 open inputs have one decision each (E1).
- 3 of 3 acceptance criteria map to a journey step (A1, A2, A3).
- PR (c) starts at the red tests, not at design (E7).

## Cost and time to effect

One document cycle, two files edited and one added. No code (A5). No edit time was measured.

## Risks and early detection

1. **The ship-side check reaches a protected file.** Detection: the build of PR (c) names the protected path, and the blocker path of the builder applies (A5).
2. **The card misreads the checkpoint format.** The reading comes from the parent spec (E6). Detection: the first red test of the checkpoint identity.
