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

## comment-zero h2

### `go/internal/phases/ship/landing/golden_test.go:3` — above `import (`

```text
// golden_test.go — test 22: the goldens captured on the HOST before the move
// (testdata/*.golden.json, 8e8f080f) replayed through the leaf. A golden row
// is a whole host scenario; the leaf owns the window from the reset (or the
// push) up to its post-push head read, and the `[ship]   OK: ff-merged` and
// `[ship] REPAIR:` log lines — the host's prelude, its `rev-parse HEAD^{tree}`
// and its own OK lines are outside the leaf and are cut before comparing.
```

### `go/internal/phases/ship/landing/importgraph_test.go:3` — above `import (`

```text
// importgraph_test.go — the package is a leaf under the ship phase (ADR-0103
// unit 07 §2): stdlib plus the three named internal packages, never
// internal/core or internal/phases/ship (the compiler is the cycle guard;
// this is the leaf-ness declaration — the failurelearning/importgraph_test.go
// idiom).
```

### `go/internal/phases/ship/landing/landing_test.go:96` — above `func newLanding(f *fakeGit) (*Landing, *[]signalcenter.Event) {`

```text
// newLanding builds a Landing over the fake git with a recording Center,
// phase "ship" (the host's spelling, handed in — the leaf spells no phase),
// cycle 7 and run r7.
```

### `go/internal/phases/ship/landing/limits_test.go:3` — above `import (`

```text
// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 07 §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (the failurelearning/
// limits_test.go copy; comments inside a function count, its doc comment
// does not).
```

### `go/internal/phases/ship/landing/push.go:190` — above `func (l *Landing) repairPush(ctx context.Context, req PushRequest, rejected *shiperr.ShipError, out *PushResult) error {`

```text
// repairPush is the inline push-race repair (ADR-0039 §8 mode #4), run at
// the push site so the healed path still flows through the post-push
// verification and the binding. Returns nil when the push landed (retried,
// or already present on origin); otherwise the error to surface — the
// original transient rejection (declined, with Debug stamped), or a
// precondition reclassification (repair_outcome=needs-reaudit) when origin
// diverged and the only legitimate route is a re-audit on the new base.
// Never rebases, never force-pushes. The host's guards come in on the
// request; out reports what ran.
```
