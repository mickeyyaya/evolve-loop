# Comment history: `internal/fleet`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/fleet/fleet.go:1` — above `package fleet`

```text
// Package fleet is the ADR-0049 S6 concurrent-cycle supervisor: it launches and
// reaps N evolve cycles that run at the SAME time. Each cycle runs in its own
// process with EVOLVE_FLEET=1, so the orchestrator skips the whole-cycle global
// project lock (root-cause R1, orchestrator.fleetMode) and the per-resource
// flocks the safety-net slices put in place — state.json (S2), the ledger chain
// (CA.1), the .evolve/ship.lock integrator (S5) — serialize the shared writes,
// while each cycle's per-run worktree/workspace + run-scoped ship reads (S3) and
// audit binding (S4) keep it isolated. The supervisor is the missing PRODUCER
// for the EVOLVE_FLEET flag (the bridge consumer guard already exists).
```

### `go/internal/fleet/freshness.go:1` — above `package fleet`

```text
// freshness.go — the dispatch freshness gate (cycle 767, inbox id
// dispatch-freshness-gate, campaign loop-reliability-2026-07).
//
// Width-3 batch 2026-07-13 postmortem: ~3 of 8 failed lane-slots were doomed
// at dispatch — a task shipped before its lane launched, a consumed task was
// re-picked when consumption raced dispatch, and a deps-unmet task was
// dispatched 3x. The gate re-resolves every spec's scope ids against CURRENT
// inbox/consumed state and deps immediately before lane launch: stale ids are
// skipped with a logged reason, a spec whose whole scope went stale has its
// slot refilled from the pending backlog, and an honest empty-scope build
// after the gate verdicts SKIPPED — never FAIL.
```

### `go/internal/fleet/freshness.go:25` — above `Reason string`

```text
// non-empty when !Fresh, e.g. "consumed: promoted processed cycle-748" or "deps unmet: needs <dep-id>"
```

### `go/internal/fleet/freshness_test.go:3` — above `import (`

```text
// freshness_test.go — TDD contract for the dispatch freshness gate (cycle 767,
// inbox id dispatch-freshness-gate, weight 0.95, campaign loop-reliability-2026-07).
//
// Width-3 batch 2026-07-13 postmortem: ~3 of 8 failed lane-slots were doomed
// at dispatch. (1) push-ci-watch-remote-parity was dispatched AFTER the task
// had shipped (cycle 748) and the honest "no in-scope work remains" build was
// FAILed by the review gate; (2) token-resolver-production-wiring was
// re-picked at 754 after landing at 745 (consumption raced dispatch);
// (3) token-telemetry-s6-rollups was dispatched 3x against an unmet dep.
//
// The gate: immediately before lane launch, re-resolve every spec's scope ids
// against CURRENT inbox/consumed state and deps. Stale ids are skipped with a
// logged reason, the freed slot is refilled from the pending backlog, and an
// honest empty-scope build after the gate verdicts SKIPPED — never FAIL.
//
// Contract the Builder implements (new file freshness.go, package fleet —
// DO NOT modify these tests; make them pass):
//
//	// TaskFreshness is one task id re-resolved at dispatch time.
//	type TaskFreshness struct {
//		Fresh  bool   // still pending in the inbox AND all deps satisfied
//		Reason string // non-empty when !Fresh, e.g. "consumed: promoted processed cycle-748" or "deps unmet: needs <dep-id>"
//	}
//
//	// FreshnessProbeFn re-resolves one task id against current state
//	// (production wiring reads .evolve/inbox lifecycle dirs + deps;
//	// tests inject a fake).
//	type FreshnessProbeFn func(taskID string) TaskFreshness
//
//	// RefillFn returns the next pending backlog item as a lane spec.
//	// exclude holds every id this wave already owns (kept AND skipped) so a
//	// refill can never duplicate a live lane or resurrect a skipped id.
//	// ok=false → no pending candidate; the slot stays empty (a shorter wave,
//	// never a doomed lane).
//	type RefillFn func(exclude map[string]bool) (CycleSpec, bool)
//
//	// FreshnessSkip records one id skipped at dispatch, with its reason.
//	type FreshnessSkip struct {
//		TaskID string
//		Reason string
//	}
//
//	// FreshenSpecs applies the gate: probes every scope id, filters stale
//	// ids out of their specs (a spec whose WHOLE scope went stale is dropped
//	// and its slot refilled; a spec with remaining live ids keeps its slot),
//	// and logs one WARN line per skipped id (id + reason) to warn.
//	// Returns the launchable specs and the skip records.
//	func FreshenSpecs(specs []CycleSpec, probe FreshnessProbeFn, refill RefillFn, warn io.Writer) (kept []CycleSpec, skipped []FreshnessSkip)
//
//	// ClassifyEmptyScopeBuild maps a lane build outcome to its final verdict.
//	// After the freshness gate ran for the lane, an honest
//	// "no in-scope work remains" report is SKIPPED — never FAIL (never punish
//	// an honest empty result), and never PASS either (no work is not work).
//	// Without the gate, or when the build claimed real in-scope work, the
//	// original verdict stands unchanged.
//	func ClassifyEmptyScopeBuild(freshnessGateRan, reportsNoInScopeWork bool, originalVerdict string) string
```

### `go/internal/fleet/packagegraph_test.go:1` — above `package fleet`

```text
// packagegraph_test.go — cycle-871 TDD contract for merge ladder RUNG 1
// (inbox merge-rung1-package-graph-disjoint; research
// knowledge-base/research/merge-concurrency-2026/README.md finding #2:
// "file-level disjointness is explicitly insufficient (merge skew: rename +
// call-site)"). fleet.Partition today buckets todos by literal file paths
// only; it has no notion of Go package import-graph reachability, so two
// todos that touch disjoint files but are connected through the import graph
// (e.g. one edits a package, the other edits a caller of that package) are
// wrongly treated as safe to co-schedule concurrently.
//
// This file pins the package-graph resolver's contract using REAL repo
// packages (no synthetic fixtures needed): go/internal/fleet transitively
// imports go/internal/ipcenv (see partition.go's import block), while
// go/internal/acsrunner has zero import relationship with fleet in either
// direction (verified via `go list -deps` at authoring time).
//
// RED status at authoring (cycle 871): every test below fails to COMPILE —
// TransitivePackageSet, IsGlobalZone and GlobalZoneFiles do not exist yet.
// That is the correct RED signal (Builder's job is to add packagegraph.go).
```

### `go/internal/fleet/partition.go:10` — above `func PlanCycles(todos []Todo, count int) (specs []CycleSpec, deferred []Todo) {`

```text
// PlanCycles partitions a backlog into at most `count` concurrent cycle specs,
// each scoped to a DISJOINT set of todos, plus the deferred todos that could not
// be co-scheduled this wave (run them in a later wave). It is the single adapter
// from the advisor's backlog to fleet's launch specs (ADR-0049 E): each spec
// carries its todo IDs in Scope and in Env[fleetScopeEnvKey] (comma-joined) so
// the launched cycle's triage selects only its subset. Empty buckets yield NO
// spec — when the backlog can't fill `count` cycles, fewer launch (not idle ones).
```

### `go/internal/fleet/partition.go:61` — above `func Partition(todos []Todo, n int) (buckets [][]Todo, deferred []Todo) {`

```text
// Partition assigns todos to n concurrent cycle buckets such that every repo
// file is owned by AT MOST ONE bucket (ADR-0049 E: the advisor's "plan, separate
// and assign the todos to independent cycles"). Each bucket runs as its OWN
// concurrent `evolve cycle run` in its own worktree, so the load-bearing
// invariant is CROSS-bucket disjointness: a file appearing in two buckets means
// two cycles edit it at once and collide on the shared tree at ship time. Two
// todos touching one file therefore CLUSTER into the same bucket (one cycle, one
// worktree, sequential — safe), never spread across buckets.
//
// Greedy file-ownership pass (deterministic — input order preserved, ties break
// to the lowest bucket index, no map-iteration nondeterminism):
//   - a todo whose files are all unclaimed → the least-loaded bucket (spreads
//     independent work across cycles);
//   - a todo overlapping exactly ONE bucket's files → joins that bucket;
//   - a todo whose files are split across ≥2 buckets would bridge two concurrent
//     cycles into a collision, so it is DEFERRED to a later wave rather than
//     co-scheduled (the caller runs deferred todos once the current wave lands).
```

### `go/internal/fleet/partition_test.go:220` — above `func assertGraphConnectedNotSplit(t *testing.T, buckets [][]Todo, deferred []Todo, idA, idB string) {`

```text
// assertGraphConnectedNotSplit is the cycle-871 load-bearing invariant (rung 1
// of the merge ladder, research finding #2): two todos whose transitive
// package sets intersect (or either touches a global-zone file) must be
// co-scheduled into the SAME bucket, or deferred — NEVER split across two
// distinct concurrent buckets, since that would let two cycles independently
// touch reachable code and merge-skew (rename + call-site) on landing.
```

### `go/internal/fleet/partition_test.go:345` — above `func TestPlanFromTriage_OverlappingDeclaredFilesCollapseToOneLane(t *testing.T) {`

```text
// TestPlanFromTriage_OverlappingDeclaredFilesCollapseToOneLane pins the cycle-523
// fix: two top_n cards declaring the SAME file must land in ONE lane. Before the
// fix PlanFromTriage set Todo{Files: []string{id}}, so distinct ids were trivially
// file-disjoint and spread to two colliding lanes (the fictional-disjoint defect
// inbox item wave-seed-partitions-on-id-not-real-files, weight 0.92).
```

### `go/internal/fleet/pool.go:11` — above `type PoolConfig struct {`

```text
// PoolConfig configures a rolling lane pool (cycle-550 supervisor-continuous-
// lane-keeping). Target is the width the pool tries to hold — up to Target lanes
// run at once, drawn incrementally from the backlog. Concurrency optionally caps
// the live-lane count below Target (<=0 ⇒ follow Target); it never raises the cap
// above Target.
```

### `go/internal/fleet/pool.go:29` — above `func RunPool(ctx context.Context, cfg PoolConfig, backlog []Todo, launch LaunchFn, onTransition func(PoolTransition)) []…`

```text
// RunPool maintains up to cfg.Target concurrently-running lanes drawn from
// backlog and BACKFILLS a replacement the instant any lane exits (PASS or FAIL),
// instead of the wave barrier's "wait for every sibling before re-planning". It
// removes the min-over-time width collapse Supervisor.Run suffers when lane
// durations are skewed (batch bh1rt946t: one early exit stranded the wave 1-wide).
//
// Selection: the initial fill dispatches disjoint todos in backlog order (the
// SAME file-ownership rule Partition applies statically, here applied
// INCREMENTALLY against the currently-RUNNING set). On any lane exit it selects
// the highest-Priority pending todo whose Files are disjoint from every
// STILL-RUNNING lane's Files (lowest backlog index breaks ties) and dispatches it
// as a replacement, without waiting for any sibling. When no disjoint candidate
// exists the pool simply runs fewer lanes — never zero while pending work
// remains, never the unisolated in-supervisor sequential path (per L4, every
// dispatch goes through the same isolated `launch` seam as Supervisor.Run).
//
// onTransition (nil-safe) fires on every live-lane-count change. Result.Index
// indexes into backlog. A zero-length backlog returns immediately with zero
// results and zero launch calls (the pool-mode analogue of the wave path's
// empty-plan guard). RunPool returns once every backlog item has been dispatched
// and has finished.
```

### `go/internal/fleet/pool_test.go:3` — above `import (`

```text
// pool_test.go — RED contract for cycle-550's supervisor-continuous-
// lane-keeping task (L5, "the ceiling-keeper" that completes the fleet-width
// architecture: L1 triage-supply-disjoint-topn, L2 wave-seed-partitions-on-
// real-files, L4 eliminate-sequential-fallback-min-width-lane).
//
// PROBLEM (inbox 2026-07-06T16-00-00Z-supervisor-continuous-lane-keeping.json,
// operator directive verbatim: "the supervisor must try its best to honor the
// setting"): Supervisor.Run (fleet.go:75) is a WAVE BARRIER — it launches
// every spec in the batch, waits for ALL of them via sync.WaitGroup, and only
// then returns. Batch bh1rt946t evidence: wave 0 dispatched 2 lanes, one
// failed early, and the supervisor stayed 1-wide for the REST of that wave
// instead of immediately backfilling a replacement — realized width is
// min-over-time, not the operator's configured fleet.count, whenever a lane's
// duration is skewed relative to its siblings (which is the common case, not
// the exception).
//
// FIX CONTRACT (new surface this cycle — undefined until Builder adds it, so
// this package's test build fails to compile today; that compile failure IS
// the RED evidence, mirroring the cycle-465/507/547 precedent):
//
//	type PoolConfig struct { Target, Concurrency int }
//	type PoolTransition struct { Live, Target int }
//	func RunPool(ctx context.Context, cfg PoolConfig, backlog []Todo,
//		launch LaunchFn, onTransition func(PoolTransition)) []Result
//
//	RunPool maintains up to cfg.Target concurrently-running lanes drawn from
//	backlog (bounded by cfg.Concurrency, <=0 meaning follow Target). It
//	dispatches an initial fill of up to Target disjoint todos (by the SAME
//	file-ownership rule fleet.Partition already applies statically -- here
//	applied INCREMENTALLY against the currently-RUNNING set rather than a
//	fixed per-wave partition), then on ANY lane's exit -- PASS or FAIL, it
//	does not matter which -- immediately selects the next pending todo whose
//	Files are disjoint from every STILL-RUNNING lane's Files (highest
//	Priority first) and dispatches it as a replacement, WITHOUT waiting for
//	any sibling lane to finish. When no disjoint candidate exists the pool
//	simply runs fewer lanes (never zero while pending work remains, and never
//	falls back to the unisolated in-supervisor sequential path -- per L4,
//	every dispatch in this function goes through the same isolated `launch`
//	seam as Supervisor.Run). onTransition (nil-safe) is invoked on every
//	live-lane-count change with the data backing the "lanes live: N/target"
//	telemetry line -- formatted by the CALLER (this package stays I/O-free,
//	same idiom as FleetConfig.Warnings). Result.Index indexes into backlog.
//	RunPool returns once every backlog item has been dispatched and finished
//	(a zero-length backlog returns immediately with zero results and zero
//	launch calls -- the pool-mode analogue of the wave path's D1 empty-plan
//	guard).
//
// ADVERSARIAL DIVERSITY (skills/adversarial-testing §6):
//   - Positive : TestRunPool_BackfillsReplacementWhileSiblingLaneStillRunning
//     (the core rolling-pool behavior -- a barrier-preserving impl fails this)
//   - Negative : TestRunPool_CollidingFilesNeverCoRunButAllEventuallyDispatch
//     (the strongest anti-no-op for the disjointness rule: a naive "just
//     backfill blindly" impl that ignores file overlap fails here by running
//     2 colliding-file lanes at once)
//   - Edge     : TestRunPool_EmptyBacklogIdlesCleanlyNoLaunchCalls
//   - Semantic : TestRunPool_EmitsShrinkAndRecoveryTransitions (telemetry
//     data distinct from the dispatch-timing behavior itself)
//   - Selection: TestRunPool_BackfillPrefersHighestPriorityDisjointCandidate
```

### `go/internal/fleet/prefixqueue.go:8` — above `type RiskTier int`

```text
// prefixqueue.go implements the cycle-975 inbox item prefix-speculation-landing-queue
// (campaign merge-efficiency-2026-07): a single-writer landing composer modeled on
// Zuul / GitHub merge-queue "prefix" speculation. Fleet lanes never push main
// themselves; instead a PrefixQueue holds a FIFO of PASS lane candidates and builds
// composed candidate trees as queue PREFIXES (L1, L1+L2, L1+L2+L3), which are verified
// against the native gate set. The first failing prefix names the culprit positionally
// (Zuul NNFI — No New Failures Introduced) and the lanes behind it re-form without it,
// so no bisection subsystem is needed. The window is an AIMD control loop (start 3,
// +1 per green, halve on red, floor 1). Lanes are risk-tiered like Rust rollups: an
// iffy (core / cross-cutting) lane and any overlap-zone lane (sharing a touched file
// with a lane already in the composing group) get a solo prefix slot.
//
// Landing strategy is policy config (fleet.landing: per-lane | prefix-queue), NOT an
// env flag (standing rule no_feature_flags_use_design_patterns) — see LandingMode.
```

### `go/internal/fleet/prefixqueue_apicover_test.go:8` — above `func pqContains(ids []string, id string) bool {`

```text
// prefixqueue_apicover_test.go — default-tag public-API coverage for the salvaged
// prefix composer (ADR-0069: the acs-tagged go/acs/cycle975+981 predicates do NOT
// run under `go test ./internal/...`, so repo-wide apicover flags every
// prefixqueue.go export as uncovered — the exact gap that reds main). Each test
// names and exercises a real contract (Rule 9), it is not a bare reference.
```

### `go/internal/fleet/preflight.go:3` — above `import (`

```text
// preflight.go — FLEET-AS-POLICY S3(a): the dirty-control-plane wave
// preflight. Fixes the fleet-trial-#1 failure class (cycle-467 scout H1): a
// dirty .evolve/policy.json in the MAIN checkout killed an audit-PASSED lane
// at ship time; this guard surfaces the same condition at wave START, before
// any lane is planned or launched.
//
// It lives in internal/fleet — NOT internal/guards — because the guards
// package is itself pipeline-protected surface (an autonomous cycle editing
// it would trip its own integrity gate), and because keeping the helper
// importable leaves the door open to the generalized launch-path preflight
// (`evolve fleet --plan`, single-cycle loop; scout B2).
```

### `go/internal/fleet/preflight_test.go:3` — above `import (`

```text
// preflight_test.go — fleet-s3-guards AC1/AC2 (cycle 467): RED-first contract
// for the dirty-control-plane wave preflight. PreflightControlPlane does not
// exist yet; this file fails to COMPILE until Builder adds
// go/internal/fleet/preflight.go — that compile failure IS the RED evidence.
//
// Contract: PreflightControlPlane(repoRoot string) error inspects the git
// working tree at repoRoot (production: the MAIN checkout, cfg.ProjectRoot)
// and refuses — a non-nil error — when any uncommitted change (modified
// tracked file OR untracked addition) touches the pipeline integrity control
// plane per guards.IsProtectedSurface. The error is ACTIONABLE: it names the
// offending path and the remediation (`evolve ship --class manual`). It lives
// in internal/fleet (NOT internal/guards — the guards package is itself
// protected surface, and keeping the helper importable leaves the door open
// to the generalized launch-path preflight, scout B2). Fixes the
// fleet-trial-#1 class (scout H1): a dirty .evolve/policy.json killed an
// audit-PASSED lane at ship; the preflight surfaces it at wave START instead.
```

### `go/internal/fleet/quota.go:3` — above `import (`

```text
// quota.go — FLEET-AS-POLICY S3(b): the quota-aware wave Count shrink. The
// bench SSOT is clihealth.Store (the bridge writes a bench on every quota
// classification — cycle-283 forensics), so no new probing happens here: the
// caller intersects Store.Active() with the families the wave needs and
// passes only the relevant entries.
```

### `go/internal/fleet/quota_test.go:3` — above `import (`

```text
// quota_test.go — fleet-s3-guards AC3 (cycle 467): RED-first contract for the
// quota-aware wave Count shrink. QuotaAwareCount does not exist yet; this
// file fails to COMPILE until Builder adds it — that compile failure IS the
// RED evidence.
//
// Contract: QuotaAwareCount(count int, benched map[string]string, warn
// io.Writer) int. benched maps a REQUIRED CLI family to its active bench
// reason/pattern — the caller intersects clihealth.Store.Active() (the quota
// bench SSOT, scout Key Finding 4 / H2: the bridge already writes benches on
// every quota classification, no new probing) with the families the wave
// needs, and passes only the relevant entries. Each benched required family
// shrinks the effective lane count (a benched family cannot absorb its share
// of concurrent lanes), clamped to a minimum of 1 so the loop always retains
// its sequential fallback. Every shrink WARNs on `warn`, naming the family
// AND the bench reason so the operator sees WHY capacity dropped.
```

### `go/internal/fleet/quota_test.go:74` — above `func TestQuotaAwareCount_MinLanesFloorHoldsCapacity(t *testing.T) {`

```text
// TestQuotaAwareCount_MinLanesFloorHoldsCapacity is the fleet.min_lanes fix
// (2026-07-03): the operator's asserted concurrent-lane budget survives a
// transient CLI-family bench. count=2, one benched family (codex), floor=2 →
// the wave STAYS at 2 (2 lanes on the healthy claude family) instead of the
// pre-fix collapse to 1. The WARN still names the benched family + reason so
// the operator sees the bench, but reports capacity HELD, not shrunk. Gaming
// this: return count-1 (ignores the floor) fails; drop the WARN fails.
```

### `go/internal/fleet/starvation.go:1` — above `package fleet`

```text
// starvation.go — the L3 leg of the fleet-concurrency-respect architecture:
// detect when the fleet keeps realizing fewer lanes than the operator asked for
// because the WORK SUPPLY (triage plan / inbox backlog) ran dry, not because a
// quota/capacity shrink benched a CLI family. After K consecutive such waves the
// loop self-files one weighted inbox todo naming the cause, so the next batch
// widens the plan instead of silently running under-utilized forever.
//
// This lands INSIDE internal/fleet (already go/.apicover-enforce:124) rather than
// a new leaf package — cycle 542 built the identical logic as a new
// internal/fleethealth package and the ship-gate correctly FAILed it
// (TestApicoverEnforce_CoversEveryInternalPackage: the package was never added to
// the enforce list). Extending an already-enforced package sidesteps that
// completeness gate entirely.
```

### `go/internal/fleet/starvation_test.go:10` — above `func TestStarvation_QuotaShrunkWaveNeverStarves(t *testing.T) {`

```text
// starvation_test.go — non-acs regression coverage for the recovered L3
// work-supply-starvation observer (starvation.go). The ACS predicates in
// go/acs/cycle544 are the cycle-scoped gate; these tests are the PERMANENT,
// package-local coverage of the observer's public contract and directly satisfy
// AC-4 (cmd_loop.go's `case ran:` side effect covered OUTSIDE package main) and
// AC-5 (quota-shrunk anti-no-op) via package-fleet unit tests — the coverage the
// recovered starvation.go needs and that the cycle-543 diff never provided.
```

### `go/internal/fleet/todosfromtriage_test.go:3` — above `import (`

```text
// todosfromtriage_test.go — test-amplification for cycle 553's newly EXPORTED
// TodosFromTriage (extracted from PlanFromTriage as a pure refactor so the
// rolling pool dispatch path rolls the SAME backlog the wave path
// partitions; see build-report.md "New Surface" + triageplan.go's doc
// comment surfaced via `go doc`). Black-box against the spec only: before
// this cycle the function was unexported and only ever exercised indirectly
// through PlanFromTriage's returned CycleSpecs; this file pins its contract
// DIRECTLY at the new exported boundary. Fixtures mirror the JSON shapes
// already established by triageplan_test.go / triageplan_amplify_test.go
// (pre-existing, unmodified this cycle) so the parse-schema assumptions are
// grounded in prior-precedent tests, not guessed.
```

### `go/internal/fleet/triageplan.go:8` — above `type triageDecision struct {`

```text
// triageDecision is the subset of a wave's triage-decision.json (companion
// artifact written by the triage phase; schema owned by
// internal/triagecap.ReadDeclaredFloors) that PlanFromTriage consumes.
// TopN mirrors the orchestrator's projected-decision shape (real
// triage-decision.json artifacts, e.g. .evolve/runs/cycle-464/, commonly carry
// only top_n[].id and no committed_floors at all — see PlanFromTriage doc).
```

### `go/internal/fleet/triageplan.go:61` — above `type RoutedFn func(id string) (routed bool, reason string)`

```text
// RoutedFn is the ADR-0074 plan-time routing authority: id → (console-routed,
// reason). The triage-decision top_n is the load-bearing selection→dispatch
// handoff, so enforcement lives HERE (both schedulers + the wave-seed fallback
// single-source this parse) rather than in prompts, which only advise.
// Composition roots pass inboxbatch.RoutedResolver; nil = no routing context
// (unit tests) — everything dispatchable. Unknown ids are dispatchable by the
// resolver's contract (scout-originated work has no inbox item).
```

### `go/internal/fleet/triageplan.go:113` — above `if routed != nil {`

```text
// ADR-0074 plan-time gate: a console-routed (operator-owned) id must
// never become a lane todo — refused loudly, never silently dropped.
```

### `go/internal/fleet/triageplan_amplify_test.go:3` — above `import (`

```text
// triageplan_amplify_test.go — test-amplification (salvaged from cycle 465)
// for the PlanFromTriage contract. Black-box against the spec only (tdd
// handoff + build-report contract lines): floors → one Todo per DISTINCT id
// → PlanCycles disjoint lanes; cards are a fallback when floors are
// absent/empty, never a union; malformed input rejects with zero specs;
// degenerate counts and large-scale inputs never panic or over-schedule.
```

### `go/internal/fleet/triageplan_consoleroute_test.go:3` — above `import (`

```text
// triageplan_consoleroute_test.go — RED contract for ADR-0074 I1 at the REAL
// dispatch chokepoint (architect review finding 2): triage-decision.json's
// top_n is the load-bearing selection→build handoff, consumed here by BOTH
// schedulers (wave PlanFromTriage, pool TodosFromTriage) and by the
// wave-seed-inbox fallback. A console-routed id must be refused at plan time
// — loudly (refusals list) — never silently planned into a lane.
```

### `go/internal/fleet/triageplan_test.go:3` — above `import (`

```text
// triageplan_test.go — RED-first contract for PlanFromTriage (FLEET-AS-POLICY
// S2, salvaged from cycle 465's preserved worktree per cycle-466's operator
// T1: fix D1 empty-plan livelock + nil cardPackages). See scout-report.md
// Task 1 and .evolve/evals/s2-wave-salvage-fix-d1.md for the acceptance
// criteria this file materializes. PlanFromTriage does not exist yet in this
// worktree; every test below fails to COMPILE until Builder adds it — that
// compile failure IS the RED evidence (mirrors cycle-465's precedent, and
// cycle-464's C464_001-004 before it).
```

### `go/internal/fleet/triageplan_test.go:129` — above `func TestPlanFromTriage_ProductionFixtureTopNOnlyFallback(t *testing.T) {`

```text
// TestPlanFromTriage_ProductionFixtureTopNOnlyFallback (AC2): a
// triage-decision.json shaped like the REAL cycle-464 artifact — top_n[].id
// cards, NO committed_floors field — with the caller-supplied cardPackages
// left nil (production's productionWavePlanFn never threads a package list)
// must still yield >=1 non-empty lane. This is the scout report's severity
// amplifier: real triage decisions commonly carry no committed_floors at
// all, so the floorless+cardless livelock is the COMMON path, not an edge
// case — D1 fires on the first wave of any real batch without this
// fallback. Gaming fake this kills: a fixture doctored WITH committed_floors
// (dodges the real-world shape that triggered D1).
```

### `go/internal/fleet/wave_disjoint_test.go:3` — above `import (`

```text
// wave_disjoint_test.go — fleet-s3-guards AC4 (cycle 467): the WAVE-LEVEL
// disjointness regression pin. TestPartition_CrossBucketFileDisjoint pins the
// invariant at the bucket level only; nothing pinned it on the []CycleSpec
// output of PlanWaves / PlanFromTriage — the shape the wave launcher actually
// consumes (scout Key Finding 5). These are regression pins over EXISTING
// behavior (expected pre-existing GREEN once the package compiles): if a
// future change lets two specs of one wave share a file, concurrent lanes
// would collide on the shared tree at ship time.
```
