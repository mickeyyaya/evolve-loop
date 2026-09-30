# Comment history: `internal/swarm`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/swarm/apicover_named_test.go:3` — above `import (`

```text
// apicover_named_test.go — public-API coverage (ADR-0050 Phase 5).
//
// apicover flags an exported symbol UNCOVERED unless a _test.go in this package
// NAMES the identifier (and, for funcs/methods, also executes it >0%). The 14
// types below were exercised only INDIRECTLY by the existing suite (fakes
// implement the interfaces structurally; producers return the report structs)
// but their bare identifiers never appeared in test source, so apicover could
// not see them. Each test here NAMES the type via a meaningful use:
//   - interface seams: a compile-time `var _ Iface = concreteImpl` satisfaction
//     assertion, then a real method call through the interface variable;
//   - report/result structs: bind the value the REAL producer returns to a typed
//     variable and assert on its fields (no bare literals, no `_ = pkg.X`);
//   - the SessionStatus enum: asserted through the SessionRegistry it lives on.
//
// Rule 9: every assertion probes intent (a contract the type guarantees), not
// the mere existence of the symbol.
```

### `go/internal/swarm/kill.go:33` — above `func ExecTmuxKill(ctx context.Context, session string) error {`

```text
// ExecTmuxKill is the production TmuxKiller (shared by swarmrunner teardown and
// `evolve swarm reap`). SAFETY: it refuses an empty session name — tmux resolves
// an empty `-t` target to the CLIENT'S CURRENT session, so a blank name fired
// from inside any tmux pane kills the caller's own session (the 2026-06-11
// killer-B forensics: a test live-firing the empty case destroyed every soak
// session on the shared default socket). Otherwise best-effort: a missing
// session is the desired end state, so the tmux exit code is ignored.
```

### `go/internal/swarm/kill_exec_test.go:12` — above `func withTmuxRunStub(t *testing.T, stub func(ctx context.Context, args ...string) error) *[][]string {`

```text
// ExecTmuxKill is the production TmuxKiller. These tests pin the 2026-06-11
// killer-B contract: `tmux kill-session -t ''` resolves to the CLIENT'S CURRENT
// session, so an empty name fired from inside any tmux pane (an agent, a soak
// driver, a test) kills the caller's own session. The empty case must be
// refused BEFORE any tmux exec; everything else stays best-effort.
//
// All cases go through the tmuxRun seam — the unit suite must never touch a
// real tmux server (the unseamed predecessor tests live-fired kill-session
// against the shared default socket and destroyed soak sessions #1-#4).
```

### `go/internal/swarm/mergetrain_adversarial_test.go:3` — above `import (`

```text
// mergetrain_adversarial_test.go — cycle-281 test amplification.
// Targets ExecGitMerger.Merge (0%): the production git-shelling merger must
// return an error wrapping ErrMergeConflict when the worktree directory is
// invalid or the merge fails. This is inherently an integration-style test but
// does NOT require a real git repository — an invalid directory produces an
// exec error, which is mapped to ErrMergeConflict.
```

### `go/internal/swarm/partition.go:10` — above `func Validate(plan SwarmPlan) ValidationResult {`

```text
// Validate is the pure, mode-aware partition validator — the correctness crux of
// the writer swarm. It runs after the planner and before any worker launches.
// It never touches the filesystem or git, so it is exhaustively table-testable.
//
//   - WRITER mode (strict): every target file must be owned by at most one
//     worker. ANY overlap → Collapse to N=1 (a writer swarm runs only on a
//     provably-disjoint plan; per ADR-0032, "when not cleanly independent, do
//     NOT swarm writers"). The depends_on DAG must also be acyclic with no
//     dangling refs; the resulting topological order is the merge-train order.
//   - READER mode (lenient): overlap is allowed (read overlap wastes tokens,
//     never corrupts), so the disjointness check is skipped. Only N≥2 and DAG
//     validity (usually a no-op — readers rarely declare deps) are checked.
//
// A plan that IsFallback() (planner declared non-partitionable, or <2 workers)
// short-circuits to Collapse without inspecting file ownership.
```

### `go/internal/swarm/provision.go:55` — above `baseOverride string`

```text
// baseOverride is the operator override for the worktree base, resolved once
// from policy.json (worktree.base) and injected via NewGitWorkerProvisioner.
// Empty ⇒ the built-in <root>/.evolve/worktrees default. Replaces the former
// EVOLVE_WORKTREE_BASE env read (flag-reduction, ADR-0064).
```

### `go/internal/swarm/provision.go:152` — above `if fi, err := os.Stat(wt); err == nil && fi.IsDir() {`

```text
// Reuse an existing VALID worktree; tear down a stale stub git rejects.
// Validity needs BOTH probes: a `.git` entry at the worktree root (a plain
// stub dir inside the parent repo passes rev-parse by walking up to the
// parent's .git, silently "reusing" a non-worktree — cycle-283 finding) and
// a rev-parse to reject a corrupt/orphaned .git entry.
```

### `go/internal/swarm/provision_collision_integration_test.go:14` — above `func TestGitWorkerProvisioner_ConcurrentSiblingsNoCollision(t *testing.T) {`

```text
// TestGitWorkerProvisioner_ConcurrentSiblingsNoCollision is the swarm mirror of
// the core sibling-collision test: two worktrees of one repo each provision a
// cycle-1 integration + worker branch under a SHARED base. Pre-runscope both
// minted bare cycle-1-integration / cycle-1-w0 and collided on the global branch
// namespace; with the per-root lane each sibling gets distinct names.
```

### `go/internal/swarm/provision_retry_test.go:3` — above `import (`

```text
// provision_retry_test.go — RED contract for cycle-1268 task
// `worktree-provisioning-retry-consolidate`, adoption site #2:
// gitWorkerProvisioner.addWorktree (provision.go:147).
//
// This is the HIGHEST-contention site in the tree: a writer swarm provisions N
// worker worktrees concurrently against the SAME shared .git, which is exactly
// the lock window PR #401 documented. It is also structurally identical to
// pre-fix core.gitWorktree.Create — same reuse/stale-stub probe, then a single
// unretried Capture("worktree","add","-B",...) with no attempt loop.
//
// Both production entry points are driven (CreateWorker AND CreateIntegration):
// wiring the retry into one path only is the same defect, just narrower (#373).
//
// The knobs arrive as a struct field carrying gitexec.WorktreeAddRetry rather
// than a swarm-local copy of the attempt/backoff constants — a second private
// copy is precisely the copy-paste the "consolidate" in this task's name
// forbids. swarm still does not import core (provision.go:14-19); gitexec is
// the shared floor both already depend on.
```

### `go/internal/swarm/provision_retry_test.go:33` — above `func swarmAddFailRunner(failures, attempts *int) sysexec.RunFunc {`

```text
// swarmAddFailRunner mirrors the live incident at the swarm seam: the first
// *failures `worktree add` calls return rc=255 with only "Preparing worktree"
// on stderr; every other git call (the reuse rev-parse probe, later attempts)
// succeeds so the provisioner's own control flow is what is under test.
```

### `go/internal/swarm/provision_test.go:263` — above `func TestWorktreeBase_RelativeProjectRootRefused(t *testing.T) {`

```text
// TestWorktreeBase_RelativeProjectRootRefused pins the LAST gap of the
// swarm-tests-relative-worktree-base inbox defect (cycle-297). Cycle 296 moved
// the IsAbs guard into worktreeBase, but only on the override branch. The
// DEFAULT branch still returned
// filepath.Join(projectRoot, ".evolve", "worktrees") verbatim — which is
// RELATIVE when projectRoot is relative (e.g. "."). A relative worktree base
// breaks `git worktree add` (resolved against an unintended cwd) and the
// tree-diff guard. With no override, worktreeBase("", ".") must return
// ("", error) whose message identifies that the base/root must be absolute,
// BEFORE any caller touches git/MkdirAll. This is the negative (anti-no-op)
// axis: the RED baseline returned ".evolve/worktrees" with a nil error, so this
// test fails until the default branch also guards IsAbs.
```

### `go/internal/swarm/reap_orphans.go:16` — above `var orphanNamespaces = []string{"evolve-bridge-", "evolve-recipe-"}`

```text
// reap_orphans.go — crash-recovery orphan GC.
//
// The per-run registry reaper (reap_runsessions.go) is the CLEAN-PATH teardown:
// it kills only the sessions a run recorded in its own file, so it is
// structurally incapable of touching another run's sessions (the 2026-06-11
// killer-B protection). But that guarantee has a hole: a SIGKILL'd loop never
// runs its teardown, and the NEXT loop can't reap the corpse because the dead
// run's sessions aren't in the new run's registry. Orphans then accumulate
// across crashes until the shared tmux server starves the machine.
//
// This GC closes the hole by reaping on a DIFFERENT axis — process liveness.
// Every auto-generated session name bakes in its creator's PID
// (bridge.resolveSession: "...-pid<PID>-n<nonce>-<ts>"). A session whose PID is
// dead can only be a corpse, so it is safe to kill. Crucially this preserves
// the killer-B guarantee by construction: a LIVE concurrent run's PID is alive,
// so its sessions are skipped, never killed. The failure modes are all
// fail-safe — an unparseable name, a foreign name, or a recycled-and-now-live
// PID all SKIP (leak), never mis-kill. The worst case is a leak the next sweep
// catches; it is never a wrong kill.
//
// LIMITATION: only sessions carrying a -pid<N> token are GC-eligible. Sessions
// without one — named sessions (bridge.NamedSessionName, "evolve-bridge-named-
// <name>") and ad-hoc test-harness sessions — are counted SkippedUnparseable and
// left for their creator or an operator to reap. All auto-generated bridge/recipe
// phase sessions (the ones that accumulate from crashed runs) DO carry the token,
// so this covers the production accumulation case; closing the named-session gap
// would mean baking a creator PID into NamedSessionName (a separate change).
```

### `go/internal/swarm/reap_orphans_test.go:1` — above `package swarm`

```text
// reap_orphans_test.go — crash-recovery orphan GC contract.
//
// The registry reaper (reap_runsessions.go) only kills sessions a LIVE run
// recorded in its own per-run file; a SIGKILL'd loop never reaps, and the next
// loop can't reap the corpse (it's not in the new run's registry). This GC
// closes that gap by reaping sessions whose CREATOR PID is dead — and it must
// do so without reintroducing the 2026-06-11 killer-B class (a live concurrent
// run's sessions must survive). These tests pin both halves: reap the dead,
// never touch the living.
```

### `go/internal/swarm/reap_runsessions.go:10` — above `type ReapRunReport struct {`

```text
// reap_runsessions.go — CB.5 run-teardown reaper (concurrency campaign W4).
//
// A run's tmux sessions are reaped FROM ITS OWN REGISTRY FILE
// (<workspace>/tmux-sessions.jsonl, written by the bridge at session
// creation) — never from a server-wide listing. Per-run file ⇒ a run's
// teardown is structurally incapable of touching another run's sessions
// (the 2026-06-11 killer-B class: fuzzy teardown on the shared server
// destroyed every concurrent soak). Killing an already-dead session is a
// no-op by ExecTmuxKill's best-effort contract, so reaping after a clean
// cycle (where per-launch cleanup already killed everything) is harmless.
```

### `go/internal/swarm/reap_runsessions_test.go:1` — above `package swarm`

```text
// reap_runsessions_test.go — CB.5 contract (concurrency campaign W4), reap
// half: run teardown kills exactly the sessions in the RUN'S OWN registry
// file — never a glob over the shared tmux server, never another run's
// sessions. The 2026-06-11 killer-B forensics are the why: name-fuzzy
// teardown on a shared server is how every soak that day died.
```

### `go/internal/swarm/registry_concurrency_test.go:10` — above `func TestSessionRegistry_ConcurrentRegister_NoRaceNoLostUpdate(t *testing.T) {`

```text
// registry_concurrency_test.go — Phase 2 / S2.5 (modularization campaign,
// ADR-0050). SessionRegistry.mu guards r.m.Sessions, which the dispatcher
// mutates from N worker goroutines concurrently: dispatcher.go spawns one
// Register per worker when Concurrency>=2 (the default = len(plan.Workers)).
// registry_test.go had ZERO concurrency tests, and the one incidental
// concurrent path (Concurrency:2) trips -race only ~1/20 runs because the
// workers' other work staggers the two single Register goroutines — so a
// stripped mutex would pass CI ~95% of the time.
//
// This drives real contention via fixtures.StressN and asserts the no-lost-
// update invariant under -race. RED-check: delete r.mu.Lock()/Unlock() from
// Register — concurrent upsertLocked appends to r.m.Sessions race (go test
// -race trips reliably) and the final Snapshot count drops below n*k. That
// proves the test asserts the lock, not merely "did not panic".
```

### `go/internal/swarm/types.go:1` — above `package swarm`

```text
// Package swarm implements the multi-tmux-LLM-CLI subagent swarm harness: a
// reusable primitive that lets one orchestration phase dispatch N heterogeneous
// workers (each its own CLI/model) that collaborate on a partitioned task.
//
// This file defines the pure value types shared across the package. The
// partition validator (partition.go) and topological merge-order (topo.go) are
// pure functions over these types — no I/O, no core dependency — so they are
// exhaustively table-testable. Dispatch, registry, merge-train, and reaper
// (later increments) build on top.
//
// The central rule is the WRITER/READER asymmetry (see ADR-0032):
//   - WRITER swarm (e.g. build): workers WRITE code. Partitions MUST be
//     completely disjoint by file ownership; a non-disjoint plan is REJECTED and
//     the phase falls back to a single writer (N=1). Fan-in is a serialized git
//     merge-train.
//   - READER swarm (e.g. scout/audit/research): workers only READ. Partitions
//     are best-effort by investigative aspect; OVERLAP IS ALLOWED (read overlap
//     wastes tokens, never corrupts). Fan-in is summary synthesis (no git).
```

### `go/internal/swarm/worktreebase_config_test.go:8` — above `func TestWorktreeBase_OverrideAndDefault(t *testing.T) {`

```text
// TestWorktreeBase_OverrideAndDefault locks the flag-reduction change (ADR-0064):
// the swarm worktree base comes from the injected override (policy.json
// worktree.base, threaded via NewGitWorkerProvisioner) — NOT the EVOLVE_WORKTREE_BASE
// env var, which is removed. An absolute override wins; a relative override is
// refused; an empty override falls back to <root>/.evolve/worktrees.
```
