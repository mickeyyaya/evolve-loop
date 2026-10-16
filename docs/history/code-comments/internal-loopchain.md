# Comment history: `internal/loopchain`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/loopchain/defaults.go:81` — above `func FleetLaneActive(evolveDir string) (active bool, err error) {`

```text
// FleetLaneActive reports whether a SIBLING fleet lane holds a live run under
// evolveDir. Discovery is gc.Discover, the lease-aware run-dir scan the
// retention engine owns; the lease of each Live dir is then re-read here for
// the two questions retention never asks — whose is it, and is the owner
// alive? A run dir is a sibling when its lease is not this process's own
// (runlease.Lease.OwnerPID == os.Getpid(): the whole chain runs in one process
// and every lease it writes carries its pid, while a different lane is by
// construction a different process — cycle 1364) AND its owner is live
// (runlease.OwnerLive: a sealed lane's lease outlives its process and stays
// fresh for a TTL — 2026-09-15, twice, with no lane running). Retention keeps
// its TTL-only Live on purpose (it errs toward keeping dirs). A dir Live
// through gc's other liveness source (the current workspace, no fresh lease)
// has no pid to compare and is NOT excluded — the fail-safe posture. A
// discovery error is unverifiable safety state.
```

## comment-zero h1

### `go/internal/loopchain/decision.go:15` — above `func StartDecision(n, maxBatches, inboxPending int, brake bool) (reason string, stop bool) {`

```text
// StartDecision decides whether batch n (0-based) may start, and names the
// reason when it may not. Pure, so the precedence between the three
// pre-batch stop conditions is testable without running a batch: the
// operator brake outranks everything (an explicit instruction), then a
// drained inbox AFTER at least one batch (the success exit), then the runaway
// cap. The `n > 0` scope on the drained-inbox exit is the min-one-batch
// guarantee (cycle 1098): a drained inbox is a CONTINUE condition, not a
// START condition, so opting into chaining is never weaker than the
// pre-chain contract; it never widens the cap (a non-positive cap still runs
// nothing — no cap+1).
```

### `go/internal/loopchain/defaults.go:17` — above `func GitAhead(projectRoot, runningCommit string) (ahead bool, err error) {`

```text
// GitAhead reports whether HEAD carries commits beyond runningCommit — the
// ancestor-check idiom `evolve reset-sha` uses, inverted: ahead=true means
// runningCommit is a STRICT ancestor of HEAD. An empty runningCommit (an
// unstamped dev binary) is a quiet no-op. Any git/repo failure returns an
// error the caller treats as "skip this boundary's refresh", never a halt.
// The Makefile stamps a 12-char SHORT commit while `git rev-parse HEAD`
// returns the full SHA, and `merge-base --is-ancestor` treats equal commits
// as ancestors too — so the stamp is resolved to its full SHA FIRST (Q-C1,
// cycle 1320) or an up-to-date binary would always look ahead.
```

### `go/internal/loopchain/defaults.go:48` — above `func GitProvenance(runningCommit func() string) func(projectRoot string) (string, phaseintegrity.ProvenanceVerified) {`

```text
// GitProvenance is the boundary re-pin's provenance: the running binary's
// build commit (read from runningCommit at every call, never snapshotted)
// plus a closure asserting a commit is an ancestor of HEAD. An empty commit
// is unverifiable and returns false — a stripped or tampered binary can never
// self-authorize a re-pin, and there is no sentinel substitute (cycle 1320:
// "the rebuild just succeeded, so it must be HEAD" is not a verification).
```

### `go/internal/loopchain/driver_test.go:204` — above `sibling := exec.Command("sleep", "30")`

```text
// A sibling is a DIFFERENT LIVE process: os.Getpid()+1 only happened to
// be alive; since liveness is the owner's (2026-09-15) the fixture holds a
// real child for the duration.
```

### `go/internal/loopchain/driver_test.go:519` — above `func TestFleetLaneActive_DeadOwnerIsNotASibling(t *testing.T) {`

```text
// A sealed lane's lease outlives its process: the writer stops heartbeating
// at exit but the file stays "fresh" for a full TTL (cycle 1679 sealed at
// 17:03Z; the boundary refresh at 17:03 read its 17:02 heartbeat as an active
// sibling and refused to rebuild — LOOP_BOUNDARY_REFRESH_SKIPPED step=lane_active
// with no lane running, twice on 2026-09-15). Liveness is the OWNER, not the
// timestamp: a fresh lease whose owner pid is gone is not a sibling; a lease
// with no owner pid stays a sibling (nothing to probe — refuse conservatively).
```

### `go/internal/loopchain/importgraph_test.go:3` — above `import (`

```text
// importgraph_test.go — the package is a core-free leaf beside cmd/evolve
// (ADR-0103 unit 13 §2): stdlib plus the six named internal packages — never
// internal/core, internal/fleet or pkg/version (the running commit and the
// quota-pause reader are injected) — signalcenter/importgraph_test.go idiom.
```

### `go/internal/loopchain/limits_test.go:3` — above `import (`

```text
// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 13 §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (signalcenter/limits_test.go
// idiom; comments inside a function count, its doc comment does not).
```

### `go/internal/loopchain/loopchain.go:1` — above `package loopchain`

```text
// Package loopchain is unit 13 of the component breakdown (ADR-0103): the
// loop's chain engine. The Driver is the outer batch-chaining loop (cycle
// 1075; the standing operator directive that lanes keep running until the
// inbox is empty) — at every boundary it decides whether another batch may
// start (operator brake > boundary refresh > drained inbox > runaway cap) and
// after every batch how the exit code moves the chain (rc 0/3 continue, rc 5
// defers on the quota wall, anything else stops and propagates). The
// Refresher is the boundary binary refresh (cycle 1314): ahead-check →
// fleet-lane guard → loop breaker → rebuild → re-exec target → provenance-
// gated re-pin → audit log → arm the breaker → flush the Center → exec. Both
// take their process collaborators explicitly (the batch, the rebuild, the
// exec, the running commit, the quota-pause reader) so the leaf is core-free
// and the host's package-var test seams project into them at call time.
// Failure modes are loop.warning WARN and loop.halt INCIDENT under module
// loop; the INFO-shaped [chain] report lines stay on the warn writer. Design:
// docs/architecture/decomposition/13-loopwave.md.
```

### `go/internal/loopchain/loopchain.go:115` — above `func emit(c *signalcenter.Center, origin string, kind signalcenter.Kind, sev signalcenter.Severity, code signalcenter.Co…`

```text
// emit is the unit's one producer shape: a batch-level (cycle 0) event under
// module loop from origin, delivered to the Center (nil is a no-op).
```

### `go/internal/loopchain/refresh.go:33` — above `type Refresher struct {`

```text
// Refresher is the boundary binary refresh (cycle 1314): a chained or
// multi-wave loop that runs for many boundaries could keep executing a
// running binary that fell behind HEAD while fixes landed on main. It is
// called only BETWEEN batches (the loop body is single-threaded per
// boundary, so no lock is needed); every degrade returns refreshed=false and
// the CURRENT binary keeps running the loop — it never halts.
```

### `go/internal/loopchain/refresh_test.go:73` — above `s = readDirOp.ReplaceAllString(s, "open $1: not a directory")`

```text
// The os.ReadDir fault is a *fs.PathError whose Op is Go-version- and
// OS-dependent (`open` darwin/Go 1.27, `fdopendir` darwin/Go 1.23,
// `readdirent` linux — PR #599 CI); the goldens keep `open`. Same
// canonicalization as cmd/evolve's u13Template.
```
