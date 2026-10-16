# internal/phaseio

## Purpose

`internal/phaseio` is the dependency-free contract leaf of the unified phase I/O envelope ([ADR-0050](../adr/0050-modularization-and-unified-phase-io.md), Phase 3). It defines the typed input that a phase receives (`PhaseInput`, with its `Handoffs`, `CycleInputs` and error channels) and the typed result it emits (`PhaseOutput`). A phase does not reach for sibling artifacts or share a mutable map.

## Design

- **Dependency rule.** The package imports only `internal/phasespec`. It must never import `internal/core` or `internal/router`. Both depend on it. The assembler that maps routing signals to `Handoffs` lives in a higher package.
- **Dormant until proven.** No live loop path reads these types yet. They sit behind `EVOLVE_PHASE_IO=off`, and they become authoritative only after the shadow-equivalence harness proves them equal to the legacy path.
- **Sealed inputs.** Every constructor deep-copies its reference fields: the env map, the error and correction pointers, the handoff views, and the nested values of generic signals. Accessors return values or copies, so a phase cannot change what a sibling observes. The zero value is valid, and every accessor tolerates it.
- **Active marker.** `PhaseInput.Active` is true only for an envelope the dispatch seam assembled. That seam always stamps `Phase`, and the zero value never sets it. Phases read the typed envelope when it is active, and they fall back to the legacy context map when it is not.
- **Absent versus zero.** Each handoff accessor returns `ok=false` when the phase is absent. So a phase can tell "no audit yet" from "audit reported zero". `Degraded` lists the anchor handoffs that failed to read for a reason other than absence. A non-empty list means that an absent signal may be a read miss, so routing must fail open.
- **Bounded free text.** `CapField` cuts the carryover and previous-verdict text to `MaxFieldBytes` on a rune boundary. It appends `TruncationMarker`, so a reader can see that text was dropped. A silent cut is not an acceptable cap. The capped value is idempotent, so the shadow comparator can cap a legacy value and compare it with the typed one.
- **Digest budget.** `UpstreamDigest` renders the sealed handoffs for prompt context. It JSON-quotes untrusted strings, reserves space for the separators, and shares the rest of the budget across sections. One long section cannot evict the sections after it.
- **Verdict vocabulary.** `Verdict` is the typed outcome, matching the EGPS gate strings. `IsValid` is case- and whitespace-sensitive, so a typo is refused. A `FailureBlock` is data, not prose, and the output carries no raw telemetry: tokens, cost and duration stay on the wire response.

## Invariants

- The package has no dependency on core or router. No test pins this rule yet, so a guard test is a follow-up.
- A mutation of a constructor argument after construction does not reach the sealed envelope. Pinned by `TestPhaseInput_SealedContext_NoMutation`, `TestPhaseInput_Sealed_NoMutationViaInit`, `TestHandoffs_Sealed_NoMutationViaInitOrAccessor` and `TestHandoffs_Generic_NestedValue_Sealed`.
- An absent handoff is distinguishable from a zero one. Pinned by `TestHandoffs_AuditAccessor_AbsentReturnsFalse`.
- The zero envelope is safe to read. Pinned by `TestPhaseInput_ZeroValue_Safe` and `TestHandoffs_ZeroValue_Empty`.
- `Active` is a positive marker, not a whole-struct zero comparison. Pinned by `active_test.go`.
- A bounded field keeps a rune-aligned prefix and a visible marker, and its bound holds. Pinned by the `CapField` tests in `cycleinputs_test.go`.
