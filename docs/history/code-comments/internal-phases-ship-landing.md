# Comment history: `internal/phases/ship/landing`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## ship-landing two-phase landing (cycle 1830), 2026-10-07

### `go/internal/phases/ship/landing/landing.go:1` — above `package landing`

```text
// Package landing is unit 07 of the component breakdown (ADR-0103): the ship
// phase's fleet landing. One stateless Landing owns the ff-merge of the cycle
// branch into main (with the tracked-binary reset before it), the push with
// its ONE inline push-race repair and the post-push head read, the two git
// probes the host shares with it (IsAncestor, Capture), and the ship-binding
// witness the delivery record and the lost-landing floor read. The staging
// onion, the run-scope resolution, the post-push tree verification and the
// fleet REBASE engine stay with the host and core. The Landing holds a Git
// port, the operator streams through an accessor, the phase name and run
// identity the host stamps, and the Signal Center accessor; it never reads
// the environment, never takes the integrator lock, never touches the host's
// Options or RunResult, never writes stderr, spells no phase name, and
// reports its four degraded outcomes as ship.warning under module ship.
// Design: docs/architecture/decomposition/07-shipgitops.md.
```
