# ADR-0060: Fully data-driven transition kernel (validator-anchored)

Status: Accepted
Date: 2026-06-22
Supersedes: ADR-0058 decisions §1 (legality graph hardcoded) and §8 (artifact map hardcoded). Extends ADR-0058's slice campaign (S1–S6 shipped).
Campaign: PA-DDK (plan: `buzzing-shimmying-quasar`).

## Context

ADR-0058 made the orchestrator transition kernel *partly* config-driven (audit verdict branch, retro/debugger gates, mandatory-anchor set/order) while deliberately keeping the **legality graph** (`allowed`) and the **artifact-gate map** (`anchorArtifactPresent`) hardcoded as config-independent trust anchors — so config could only *narrow* legality, never widen it. ADR-0058 explicitly **rejected** "Alternative C: full generic data-driven FSM" because a bare generic FSM lets a malformed/malicious config widen the legality oracle and bypass the non-gameable ship floor.

The operator requirement has since hardened: **every phase name, function, and setting must be extracted to dynamic configuration** — zero hardcoded phase identity may drive behavior in production *or* test code. The kernel must become a generic engine; an operator changes the entire flow by editing the registry, not Go. This is Alternative C.

The blocker to Alternative C was always safety, not feasibility. Source review confirms the floor's *true* root of trust is **runtime, artifact-backed**: `SpineSatisfiedUpTo` reading real on-disk digests, the audit verdict routing in `Next`, and the ship-time tree-SHA binding + EGPS `red_count==0`. The hardcoded legality graph is only a *structural backstop*, not the load-bearing guarantee.

## Decision

**1. Config-drive the entire flow** — legality graph (`legal_successors`), artifact gates (`gate`), recovery targets (`recovery`), early-exit (`early_exit`), verdict branches (`on_pass`/`on_fail`, already live), branching strategy (already live), aliases — all declared per-phase in the registry. The Go kernel reads them; it holds no phase-name `switch` in flow logic.

**2. Relocate the trust anchor from the graph to a validator.** A hardcoded, **phase-agnostic** `ValidateSafetyInvariants(graph, cfg, catalog)` runs at load and HARD-FAILS any config whose flow could ship without the floor. Its invariants are quantified over config *roles* (`M = mandatory anchors`, `F = ship floor ∪ evaluator`, `E = evaluate-archetype phases`, `effectiveOrder` positions) — **never phase-name literals** — so renaming `scout`→`recon` or `audit`→`evaluate` does not weaken them. The validator, not the graph literal, is now the thing config cannot move. This is Alternative C **plus** the validator that Alternative C lacked — sound where bare-C was not.

**3. The digest stays trusted Go.** `router.RoutingSignals` (how on-disk handoff artifacts are read into objective signals) is the anti-spec-gaming verification root and remains code. Only the gate *thresholds* (`requires_present`, `verdict_in`) become config; the gate is *evaluated* against the trusted digest.

**4. Floor invariants (the validator's checks).** I1 every start→ship path traverses every `m∈M` in order (each dominates `ship`); I2/I3 some evaluate phase with a verdict-gate `⊆{PASS,WARN}` dominates `ship` and every direct ship-predecessor; I4 build→evaluate→ship reachable order only; I5 `→end` only before the first floor phase; I6 no edge skips an anchor; I7 single ship sink, canonical verdicts, floor-gate `⊆{PASS,WARN}`; I8 every `on_pass`/`on_fail`/recovery target ∈ that phase's `legal_successors`; I9 reachability + **`F ⊆ M`** (without which the runtime artifact gate goes inert).

**5. Acceptance = semantic-equivalence + a never-diff floor property.** Not strict byte-identity (ADR-0058's per-slice oracle relaxes to semantic-equivalence, already accepted in S6). A re-derived oracle (derivation + snapshot-golden + floor-property over a checked-in reference registry) guards each slice; the floor property `ship ⇒ scout ∧ build ∧ shippable-audit` may never diff.

## Why this is safe (bypass analysis summary)

With the validator at load AND the unchanged runtime layers (verdict routing, artifact gate, `CanTerminateEarly`, `ClampPlanToFloorWith`, ship tree-SHA binding): structural attacks (add `scout→ship`, drop audit, accept FAIL, early-exit post-build, shrink the floor) are **rejected at load** by I1–I9; runtime lies (claim PASS with no artifact) are **rejected by the artifact-backed digest + tree-SHA binding**, which never depended on the hardcoded graph. The floor degrades gracefully — no single point's failure ships unaudited code. This is strictly *stronger* than the hardcoded graph, which only guaranteed *specific literals*; the validator guarantees the *property* for any renaming.

## Consequences

Positive: the kernel is a generic engine; changing the flow is a registry edit; the floor guarantee is an explicit, tested, phase-agnostic property rather than an implicit literal.

Negative / accepted: the validator becomes a new CI-critical trust component (mitigated by adversarial broken-registry tests + a guard test on the shipped registry); a new test-fixture indirection (a reference registry + accessor loader) replaces inline phase constants in kernel tests.

## Alternatives considered

- **Keep the legality graph hardcoded as the one safety oracle** (config-drive only settings). Rejected by the operator requirement (the graph is phase identity in code). Safe but does not meet "everything in config."
- **Also config-drive the digest extraction.** Rejected: expands the trust surface of the anti-gaming root; the digest reads objective artifacts and must stay trusted Go.

## Rollout (PA-DDK slices)

DDK-0 land ADR-0058 S6; **DDK-1 (this ADR) + the validator over today's checkable invariants (additive)**; DDK-2 reference fixture + re-derived oracle; DDK-3 spineOrder→config; DDK-4 artifact-gate→config (+gate invariants); DDK-5 legality graph→config (+dominance invariants, validator-gated); DDK-6 recovery maps→config; DDK-7 early-exit + router floor→config; DDK-8 alias/name-skew→config (terminal, grep-clean). Each slice is an independent PR from its own worktree, oracle-green, floor-property-green.
