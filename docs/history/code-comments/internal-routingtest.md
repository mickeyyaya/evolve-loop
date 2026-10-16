# Comment history: `internal/routingtest`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero d2

### `go/internal/routingtest/bricks.go:32` — above `func Done(phases ...string) Brick {`

```text
// Done sets the already-completed phases (PureKernel RouteInput.Completed).
// Defensive copy: storing the variadic backing array directly aliased caller
// memory (cycle-263 audit, C263_002 — same class as Mandatory below).
```

### `go/internal/routingtest/bricks.go:51` — above `func Mandatory(p ...string) Brick {`

```text
// Mandatory sets the ordered mandatory spine. Defensive copy: the cycle-263
// audit (C263_002) caught the variadic backing array stored directly —
// callers reusing/mutating their slice silently corrupted the spec
// (shared-backing-storage aliasing; immutability rule).
```

### `go/internal/routingtest/bricks.go:201` — above `func AgentPlan(entries ...router.PhasePlanEntry) Brick {`

```text
// AgentPlan scripts the advisor's UNCLAMPED upfront whole-cycle plan (ADR-0024
// §2). The engine clamps it to the integrity floor before threading — exactly
// as the orchestrator does — so a plan that reaches ship without build/audit is
// COMPLETED by the floor, never left weakened.
```

### `go/internal/routingtest/bricks.go:243` — above `func ExpectProposeAt(phases ...string) Brick {`

```text
// ExpectProposeAt asserts the EXACT set of just-completed phases at which the
// Proposer (LLM per-transition advice) was invoked — the hybrid-cadence proof
// (ADR-0024 §2): with an upfront plan, Propose fires only at branch transitions.
```

### `go/internal/routingtest/bricks_test.go:10` — above `func TestBricks_VariadicSlicesAreDefensivelyCopied(t *testing.T) {`

```text
// TestBricks_VariadicSlicesAreDefensivelyCopied pins the cycle-263 audit
// finding (C263_002): Mandatory() and Done() stored the caller's variadic
// backing array directly, so a caller mutating (or reusing) its slice after
// building the spec silently corrupted the scenario — shared-backing-storage
// aliasing. Immutability rule: return/store new objects, never alias caller
// memory.
```

### `go/internal/routingtest/consistency_test.go:62` — above `func TestSignalSpec_WrappedDualRenderingAgree(t *testing.T) {`

```text
// TestSignalSpec_WrappedDualRenderingAgree extends the keystone self-test to the
// ADR-0050 Phase-3 envelope: the SAME fixtures, rendered as payload-wrapped
// handoffs, must Digest to the SAME RoutingSignals as the pure rendering. This
// is the broad golden-equivalence proof (every routing fixture, not just the
// hand-picked ones in router/digest_payload_test.go) that the wrapped envelope
// is behavior-equivalent to the flat one.
```

### `go/internal/routingtest/engine.go:209` — above `if s.Expect.RetroPrefix != "" && !strings.Contains(res.RetroDecision, s.Expect.RetroPrefix) {`

```text
// Contains, not HasPrefix: the disposition gate (cycle-1046 verdict-path
// wiring) prefixes a loud reason when a scenario fixture delivers no
// disposition.json — the branch decision marker must still be present.
```

### `go/internal/routingtest/invariants.go:59` — above `"ship-implies-audit-in-plan": func(t *testing.T, in router.RouteInput, _ *router.Proposal, _ router.RouterDecision) {`

```text
// ADR-0024 §1 integrity floor: a CLAMPED plan that runs ship MUST also run
// build and audit. Asserts on in.Plan AS THREADED (already floor-clamped by
// the engine/orchestrator) — the non-configurable guarantee the kernel keeps
// regardless of what the advisor proposed or how small cfg.Mandatory is.
```

### `go/internal/routingtest/render.go:138` — above `func (s SignalSpec) WrappedHandoffFiles() map[string]string {`

```text
// WrappedHandoffFiles renders the fixture as the canonical ADR-0050 Phase-3
// payload-wrapped envelope (schema_version 2): each flat handoff JSON from
// HandoffFiles becomes the `payload` of an outer wrapper. router.Digest must
// yield identical RoutingSignals from these as from the flat HandoffFiles — the
// wrapped/flat equivalence invariant that lets the unified envelope replace the
// flat one without changing any routing decision.
```

### `go/internal/routingtest/spec.go:54` — above `Strict bool`

```text
// Strict mirrors the resolved policy.json workflow.strict_audit posture
// (replaces the EVOLVE_STRICT_AUDIT env scenario knob, flag-reduction
// ADR-0064). engine.go threads it straight into router.RouteInput.Strict.
```

### `go/internal/routingtest/spec.go:94` — above `GoalType              string`

```text
// ADR-0099 deliverable-kind signals (scout declares, triage is authoritative).
```

### `go/internal/routingtest/spec.go:100` — above `type AgentSpec struct {`

```text
// AgentSpec is the simulated LLM router brain: a scripted proposal per
// just-completed phase, or a forced error (to exercise degrade-to-static).
// Plan scripts the upfront whole-cycle plan (ADR-0024 §2) the integrity floor
// clamps; PlanError forces the planner to fail (degrade-to-static-spine path).
```

### `go/internal/routingtest/spec.go:139` — above `PhasesAbsent     []core.Phase`

```text
// must NOT appear in PhasesRun. Note: since ADR-0044 C1, PhasesRun includes aborted-but-DISPATCHED phases — use this only for phases routed around entirely (never dispatched)
```
