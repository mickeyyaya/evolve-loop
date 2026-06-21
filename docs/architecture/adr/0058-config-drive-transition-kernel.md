# ADR-0058: Config-drive the orchestrator transition kernel (verdict branch + phase-identity overrides)

Status: Accepted
Date: 2026-06-21
Extends: ADR-0035 (unified phase descriptor), ADR-0038 (phase plugin system); realizes the governing phase-agnostic-flow law.

## Context

The governing operator law requires the orchestrator FLOW engine to be PHASE-AGNOSTIC: every phase-specific difference must live in JSON config, never in Go flow control (`switch phase`, `if phase==X`, phase-name string literals driving transitions). After PA-1 (ship/retro self-registration) and PA-3a (`optionalInfraSkip` → `isConfiguredMandatory`), the residual *flow* phase-identity is concentrated in the transition kernel:

1. `statemachine.go` `Next(current,verdict)` — a `switch current` of linear successors plus the only real verdict branch (`case PhaseAudit: PASS/WARN→ship, FAIL→retro`).
2. `cyclerun_record.go` `if cr.current == PhaseRetro` — history-driven successor override that bypasses `Next()`.
3. `cyclerun_record.go` `if cr.current == PhaseDebugger` — signal-driven successor override that bypasses `Next()`.
4. `resume.go` — a duplicate of (2) on the resume path.
5. `statemachine.go` `mandatoryAnchors` — the canonical anchor ORDER as a package literal.

CRUX: `Next()` has the signature `(Phase, string)` — it holds NO catalog/config. Both live callers (`cyclerun_select.go`, `resume.go`) ARE Orchestrator/cycleRun methods that hold `o.catalog`+`o.cfg`, but thread nothing through.

These are trust-kernel code. Hard invariants that MUST survive byte-identical: (1) every current transition (default spine, trivial-skip edges, advisory routing); (2) the non-gameable floor `ship ⇒ build ∧ audit ∧ tdd` (SpineSatisfiedUpTo keyed on real on-disk RoutingSignals, Audit requiring PASS/WARN); (3) early-exit is ONLY a no-ship convergence from pre-build points; (4) the SM is the runtime authority the ledger SHA-chain/EGPS/audit-binding depend on.

`phasespec.PhaseSpec` already RESERVES `OnPass`/`OnFail` (json `on_pass`/`on_fail`, currently unused).

Source verification surfaced two facts that shape the decision: (a) `debugger`/`start`/`end` are NOT registry `phases[]` entries — they are built-in control phases registered as runners in `cmd_cycle.go`; (b) the catalog entry is named `retrospective` while `core.PhaseRetro` stringifies to `"retro"`. The runtime SSOT registry is `docs/architecture/phase-registry.json` (loaded at `cmd_cycle.go`).

## Decision

**1. Keep the legality graph (`NewStateMachine().allowed`) HARDCODED** as a config-independent trust anchor. `on_pass`/`on_fail` only SELECT among already-legal edges; they can never invent one. Invariant #4 requires at least one trust anchor that JSON edits cannot move.

**2. Make `Next()` resolution config-driven** by giving the StateMachine its catalog via a setter — `WithCatalog(specFor, orderNext)` — and ACTIVATING the reserved `OnPass`/`OnFail` fields plus one new field `BranchingStrategy`. The signature of `Next` is unchanged; authority stays in the kernel. When the catalog is unset (bare unit-test SMs), `Next` degrades to the exact literal switch.

**3. Resolve the audit verdict branch via `spec.OnPass`/`spec.OnFail`** (audit: `on_pass:ship`, `on_fail:retrospective`); WARN aliases on_pass; unknown verdict returns the identical `ErrTransitionInvalid: audit verdict %q`.

**4. Collapse the two phase-identity overrides into one generic gate** keyed on `spec.BranchingStrategy` (`"history"`=retro, `"signal"`=debugger). The `decideAfterRetro*`/`decideAfterDebugger` POLICY bodies are unchanged — the law targets flow DISPATCH, not failure-adapter/debugger-protocol policy.

**5. Provide control-phase branch metadata via a `builtinControlSpecs` SSOT seam** overlaid by the catalog accessor, because `debugger` has no registry home. Registry entries win over the seam (registry SSOT precedence). This is the one place Go data describes a phase, justified and documented as the control-phase metadata seam (not a flow `switch`).

**6. Canonicalize every spec lookup** (`PhaseRetro→"retrospective"`, control overlay) through `sm.specFor`, reusing the existing `phaseFromRouter` mapping, so name skew cannot silently fall through to a wrong edge.

**7. Stage with "dark plumbing":** the order-successor path is plumbed but unconsulted in the kernel-refactor slices; the literal switch stays the LIVE path until a transition-table oracle proves `orderNext==literal` for every default phase under the shipped registry. Then it is flipped per the oracle.

**8. Leave the integrity floor untouched:** `SpineSatisfiedUpTo`, `anchorArtifactPresent` (incl. the Audit PASS/WARN gate), `CanTerminateEarly`. The Audit PASS/WARN floor is a documented trust-kernel exception that must NOT be config-driven. `mandatoryAnchors` ORDER is derived from `cfg.Order∩cfg.Mandatory` (S6), but the artifact map stays hardcoded.

## Byte-identity oracle (the trust proof)

`TestTransitionKernelOracle_*` — a frozen golden captured from HEAD, re-run unchanged after every slice, covering BOTH transition entry points (live `selectNext`→`Next` and `resume.go`→`Next`):
- **Next table**: the full `(phase × verdict)` cross product → exact `(Phase, error-string)`, asserted against a frozen golden.
- **Legality matrix**: `CanTransition(from,to)` over `Phase×Phase` equals a frozen golden (proves the legality graph is byte-untouched).
- **Branch-override goldens**: retro (history) + debugger (signal) scenario replays on live AND resume paths.

## Slice plan (each independently shippable + byte-identical)

- **PA-BIG-0** (this ADR): the byte-identity oracle + this ADR. Pure additive, zero production change.
- **S0/S1**: `builtinControlSpecs` seam + `specFor`; `WithCatalog`; activate `on_pass`/`on_fail` + `BranchingStrategy` field; audit verdict via spec; `orderNext` DARK. Registry: audit `on_pass:ship`/`on_fail:retrospective`.
- **S2**: retro override → generic `BranchingStrategy:history` gate (+ resume.go duplicate).
- **S3**: debugger override → generic `BranchingStrategy:signal` gate.
- **S4**: load-time validator + hard registry guard (activating fields present).
- **S5**: flip `linearNext` to consult `orderNext` after the oracle proves `orderNext==literal`.
- **S6**: `mandatoryAnchors` order from `cfg.Order∩cfg.Mandatory`.
- **S7** (GRAFT D, splittable): config-drive `decideAfterDebugger`/`decideAfterRetro` recovery-target maps.

## Open questions — resolved with conservative, byte-identity-preserving defaults

- **Control-phase promotion** (debugger/start/end into registry `phases[]`): DEFERRED — keep the `builtinControlSpecs` seam (promoting expands the advisor SELECT surface; out of scope for this campaign). Revisit in a follow-up ADR.
- **User-overlay capability** (may `phase.json` overlays set `on_pass`/`on_fail`/`branching_strategy`?): RESTRICTED to built-in/registry phases for now (smaller trust surface; the legality graph + spine floor still gate any future expansion).
- **WARN handling**: WARN aliases `on_pass` (EGPS soft-pass) — settled invariant; no `on_warn` field (YAGNI).

## Consequences

Positive:
- Adding/changing a verdict-branching phase = editing `phase-registry.json` (`on_pass`/`on_fail`/`branching_strategy`) — zero Go flow changes for registry phases.
- Removes every `switch current`/`if current==X` from the transition kernel and the two record/resume overrides.
- Two independent backstops (legality graph + artifact-backed spine gate) remain on every selected edge; the floor is unchanged → invariants #2/#4 preserved by construction.
- Byte-identity provable by a transition-table oracle run against BOTH the live and resume paths, per slice.

Negative / accepted:
- Adds a `builtinControlSpecs` Go seam for control phases absent from the registry — a small, documented, SSOT'd concession (not a flow switch).
- `validate.go` + a hard registry test become a new CI dependency (a dropped activating field fails CI).
- The `decideAfterRetro`/`decideAfterDebugger` POLICY bodies retain phase-name literals in their recovery-target maps until GRAFT D (S7) closes the residual; an honest, documented gap scoped OUT of "flow dispatch".

## Alternatives considered

- **A. Config-drive the legality graph too (build `allowed` from catalog).** Rejected: removes the config-independent trust anchor invariant #4 relies on; a malformed registry could then widen the legality oracle itself. Strictly higher trust-kernel risk for no flow-agnosticism gain (the graph is an oracle, not flow).
- **B. Move Next()'s decision up into the Orchestrator (caller resolves).** Rejected: violates invariant #4 (SM is the runtime authority); scatters the transition logic the ledger depends on.
- **C. Full rewrite to a generic data-driven FSM.** Rejected: cannot offer the dark-staging byte-identity proof; large blast radius on a trust kernel.
- **D. Put debugger in the main registry to give it a branch field.** Considered; deferred — debugger/start/end as registry phases is a larger catalog-completeness change. The `builtinControlSpecs` seam is the minimal change.
