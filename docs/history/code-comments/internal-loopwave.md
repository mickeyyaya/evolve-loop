# Comment history: `internal/loopwave`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/loopwave/decision.go:8` — above `func WidenNarrowDecision(data []byte, evolveDir string, count int, protected func(string) bool) []byte {`

```text
// WidenNarrowDecision turns a present-but-narrow prior triage-decision.json
// into a fleet-width one: the committed top_n (pruned of consumed ids through
// triagecap.PruneConsumed) is backfilled from the inbox backlog up to `count`
// mutually file-disjoint lanes and each lane deepened with its cluster mates,
// then re-marshalled as {"top_n": …} only (remarshalTopN — Q-W3). Best-effort:
// count<2, an unparseable decision, or one carrying committed_floors (which
// PlanFromTriage dispatches ahead of top_n — a top_n-only re-marshal would
// silently DROP the floors) returns the original bytes; so does a decision
// already fleet-width or one nothing can be added to — UNLESS the prune
// dropped an id, which disarms BOTH shortcuts (those bytes still carry the
// consumed id; cycle-1116 — Q-W2). An EMPTY top_n is not a no-op: it widens
// fully from the backlog (cycle-554).
```

### `go/internal/loopwave/dispatch.go:62` — above `type DispatchRequest struct {`

```text
// DispatchRequest is ONE wave's collaborators: the fleet config (Count gates
// Dispatch and sizes the fan-out), the wave index, the control-plane
// preflight, the plan source, the launcher and the ADR-0074 routed resolver.
```

### `go/internal/loopwave/dispatch.go:96` — above `specs, _, err := fleet.PlanFromTriage(decisionJSON, cardPackages, count, req.Routed)`

```text
// ADR-0074 plan-time gate: refusals (console-routed ids) are logged by
// the routed resolver the moment they fire, so the slice is discarded.
```

### `go/internal/loopwave/dispatch.go:125` — above `func (e *Engine) ForceOneLane(ctx context.Context, req DispatchRequest) (Outcome, error) {`

```text
// ForceOneLane is the min-width repair's dispatcher (cycle 547): up to ONE
// disjoint candidate through the same isolated launcher path, capped at a
// single lane, WITHOUT the ShouldRunWave gate (the caller already knows the
// operator wanted a fleet and only reached here because the wave-sized count
// shrank). Silent: RepairMinWidth reports.
```

### `go/internal/loopwave/dispatch.go:134` — above `func (e *Engine) RepairMinWidth(ctx context.Context, fleetCfg, waveCfg policy.FleetConfig, req DispatchRequest) (handled…`

```text
// RepairMinWidth is the cycle-547 min-width repair the coordinator reaches on
// Dispatch's (ran=false, err=nil) case. Eligibility is the operator-asserted
// width alone: fleetCfg.Count>1 means the operator wanted a fleet, so both
// the quota-shrunk shape (waveCfg.Count<=1) and the empty-plan-at-full-
// capacity shape repair to one isolated lane rather than the leak-prone
// sequential fallthrough; true sequential stays reserved for
// fleetCfg.Count<=1. The four branches each report ONE loop.wave WARN whose
// reason keeps the sentence the inline switch printed: guard not met →
// LOOP_WAVE_EMPTY_PLAN cause=empty_triage_plan (nothing invoked); a lane
// dispatched → LOOP_MIN_WIDTH_REPAIR, handled=true (the caller continues);
// an empty backlog → LOOP_WAVE_EMPTY_PLAN cause=empty_backlog; a step error →
// LOOP_WAVE_DISPATCH_FAILED path=repair.
```

### `go/internal/loopwave/dispatch.go:185` — above `func (e *Engine) RoutedResolver() fleet.RoutedFn {`

```text
// RoutedResolver is the composition-root wiring of the ADR-0074 plan-time
// gate: a fresh inbox load per wave (mid-batch inbox changes must be seen),
// the protected-surface port, and a WARN line the moment a console-routed id
// is refused — kept as a line because the pool scheduler reaches it with no
// Center (F6).
```

### `go/internal/loopwave/importgraph_test.go:3` — above `import (`

```text
// importgraph_test.go — the package is a leaf beside cmd/evolve (ADR-0103 unit
// 13 §2): stdlib plus the twelve named internal packages — never internal/core
// and never internal/guards directly (the protected-surface predicate is a
// port; note fleet and triagecap already reach both transitively, so this is
// the leaf-ness DECLARATION, the compiler stays the cycle guard —
// signalcenter/importgraph_test.go idiom).
```

### `go/internal/loopwave/launcher.go:17` — above `func (e *Engine) Launcher(wave, concurrency int, launch fleet.LaunchFn) Launcher {`

```text
// Launcher builds the production launcher for one wave: a fleet.Supervisor
// over launch (the same exec launcher `evolve fleet` uses, so lanes inherit
// EVOLVE_FLEET=1 + EVOLVE_FLEET_SCOPE) wrapped in the dispatch freshness gate
// (cycle 767): immediately before launch every spec's scope ids are
// re-resolved against the CURRENT inbox lifecycle + deps, stale ids are
// skipped with a logged reason and freed slots are refilled from the pending
// backlog — a lane slot is never burned on known-dead work. Decorating here
// gates BOTH the wave path and the min-width repair at one seam; the wave
// index rides into the gate so its one WARN is wave-indexed.
```

### `go/internal/loopwave/launcher_test.go:16` — above `func lifecycleItem(t *testing.T, evolveDir, state, id string, deps ...string) {`

```text
// lifecycleItem plants an inbox todo where inboxmover resolves `state`:
// pending at the inbox root, processing under processing/cycle-9/, every
// other state under inbox/<state>/.
```

### `go/internal/loopwave/limits_test.go:3` — above `import (`

```text
// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 13 §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (signalcenter/limits_test.go
// idiom; comments inside a function count, its doc comment does not).
```

### `go/internal/loopwave/loopwave.go:1` — above `package loopwave`

```text
// Package loopwave is unit 13 of the component breakdown (ADR-0103): the
// loop's wave engine. One Engine owns the sequential-vs-wave gate, the ONE
// dispatch body the wave fan-out and the min-width repair share, the fleet
// config loaders, the freshness-gated launcher, the quota/budget sizing and
// the plan source (the prior cycle's triage decision, pruned then widened, or
// the inbox seed). The wave coordinator (cmd_loop_window.go), the pool
// scheduler, the budget probe and the loop's halt and escalation producers
// stay in cmd/evolve. The Engine holds the project roots, four explicit ports
// (the prior-cycle readers, the protected-surface predicate, the bench
// shrink), the warn writer the KEPT report lines still print to, and the
// Signal Center accessor; it reports its failure modes as loop.wave WARN
// under module loop (every event carries fields.wave).
//
// The leaf lives beside cmd/evolve, not under internal/core: fleet, guards,
// inboxmover and triagecap reach internal/core transitively, so a core
// sub-package would cycle. Design:
// docs/architecture/decomposition/13-loopwave.md.
```

### `go/internal/loopwave/loopwave.go:169` — above `func ReloadFleetConfig(evolveDir string, prev policy.FleetConfig, warn io.Writer) policy.FleetConfig {`

```text
// ReloadFleetConfig re-resolves the committed fleet block at a wave boundary
// (cycle 739) so an operator width directive committed mid-batch takes effect
// at the next wave. Same resolution as LoadFleetConfig with ONE deliberate
// divergence: an unreadable or malformed policy HOLDS prev (the operator's
// standing width commitment) and WARNs on warn — kept as a line, not a code,
// because the boundary reload runs on the pool path too (F3). The "fleet
// config reloaded" line prints only when a dispatch-relevant value changed
// (count, min_lanes, plan_source, scheduling — not concurrency). A package
// function like its batch-start sibling: the reload reads one root and never
// needed an engine.
```

### `go/internal/loopwave/plan.go:126` — above `func (e *Engine) pruneRouted(data []byte) []byte {`

```text
// pruneRouted drops every top_n id the plan-time gate would refuse — the
// ADR-0074 classifier NOW routes it to the console (an operator stamp or a
// protected surface landed after the prior cycle's triage committed it) —
// before the widen, for the consumed prune's reason: kept, a routed id holds a
// lane slot the widen will not refill, and the gate refuses it only after the
// lanes are cut (F34: wave 7 ran 1 of 2 lanes, 2026-09-26). Same fidelity and
// passthroughs as pruneConsumed; routedBase is the gate's own authority.
```

### `go/internal/loopwave/plan_test.go:155` — above `func TestPlanFn_ConsoleRoutedPriorIDsArePrunedBeforeWidening(t *testing.T) {`

```text
// TestPlanFn_ConsoleRoutedPriorIDsArePrunedBeforeWidening (F34, wave 7,
// 2026-09-26): the prior cycle's triage committed ids the classifier NOW
// routes to the console — an operator stamp landed after that triage, or the
// declared surface is protected. Kept, they filled the fleet width, the widen
// short-circuited, and the plan-time gate refused them only after the lanes
// were cut: the wave ran 1 of 2 lanes. Pruned BEFORE the widen (the same
// ordering the consumed prune keeps), their slots refill from the backlog.
```

### `go/internal/loopwave/plan_test.go:306` — above `writeJSON(t, filepath.Join(evolveDir, "inbox", "gamma.json"), map[string]any{"id": "gamma", "weight": 0.95, "files": []s…`

```text
// The predicate excludes protected backlog items from the widening. It
// judges the DECLARED surface — path-shaped files[] tokens (F29) — so the
// protected item declares a path; the heavier gamma would win the lane
// without the predicate, so the exclusion is observable.
```
