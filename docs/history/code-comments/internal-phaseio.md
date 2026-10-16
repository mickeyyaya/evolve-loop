# Comment history: `internal/phaseio`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment-zero h2

### `go/internal/phaseio/active_test.go:5` — above `func TestPhaseInput_Active(t *testing.T) {`

```text
// TestPhaseInput_Active: Active is the populated-marker the phase readers gate on
// (ADR-0050 §3.10 Slice 2) — true only for an assembled envelope (Phase stamped by
// the dispatch seam), false for the zero value.
```

### `go/internal/phaseio/cycleinputs_test.go:47` — above `func overCap(r rune, extra int) string {`

```text
// ---------------------------------------------------------------------------
// Cycle 1632 RED contract (task tokenopt-handoff-digests, retry of the
// cycle-1593 attempt): the two unbounded free-text fields that cross from the
// legacy Context map into the typed handoff (Carryover / PreviousVerdict) get
// ONE single-sourced bounded representation at DTO construction:
//
//	phaseio.MaxFieldBytes    — exported positive cap (bytes)
//	phaseio.TruncationMarker — exported non-empty visible marker
//	phaseio.CapField(s)      — s unchanged when len(s) <= MaxFieldBytes, else a
//	                           rune-aligned prefix of s + TruncationMarker with
//	                           len(result) <= MaxFieldBytes
//
// NewCycleInputs applies CapField to Carryover and PreviousVerdict. These tests
// are frozen (doNotModifyTests) — Builder implements, never edits.
// ---------------------------------------------------------------------------
```

### `go/internal/phaseio/cycleinputs_test.go:100` — above `bigVerdict := overCap('v', 218480-MaxFieldBytes)`

```text
// the cycle-1593 measured size
```

### `go/internal/phaseio/cycleinputs_test.go:105` — above `if c.Carryover() != CapField(bigCarry) {`

```text
// Single source: the DTO stores exactly what the exported helper produces,
// so a comparator that runs the legacy value through CapField agrees by
// construction (the cycle-1593 round-2 H1 false-mismatch class).
```

### `go/internal/phaseio/handoffs.go:10` — above `DeliverableKind   string`

```text
// "code|document" as scout declared it (ADR-0099); "" = undeclared
```

### `go/internal/phaseio/handoffs.go:20` — above `DeliverableKind string`

```text
// authoritative "code|document" after triage bounds top_n (ADR-0099); "" = undeclared
```

### `go/internal/phaseio/handoffs.go:38` — above `AuthoredBaseSHA string 'json:"authored_base_sha,omitempty"'`

```text
// AuthoredBaseSHA is the base the Builder's document was written against, set only by an
// identity-preserving rebind (ADR-0105); empty means BaseSHA is that base.
```

### `go/internal/phaseio/output.go:56` — above `CommitSHA       string`

```text
// the commit anchoring this phase's deliverable (ADR-0027), empty if none
```

### `go/internal/phaseio/phaseio.go:1` — above `package phaseio`

```text
// Package phaseio is the dependency-free contract leaf for the unified phase
// I/O envelope (ADR-0050 Phase 3). It defines the typed input/output a phase
// filter communicates through — a true pipes-and-filters contract (P4): each
// phase is given sufficient, sealed context (PhaseInput) and emits a typed
// result (PhaseOutput) without reaching for sibling artifacts or sharing a
// mutable map.
//
// Dependency rule (P3): this package imports only internal/phasespec (itself a
// leaf). It must NEVER import internal/core or internal/router — those depend on
// phaseio, never the reverse. The assembler that maps router.RoutingSignals →
// phaseio.Handoffs therefore lives in a higher package (Phase 3.3/3.4), not here.
//
// As of Phase 3.1 nothing reads these types in the live loop; they ship dormant
// behind EVOLVE_PHASE_IO=off (Phase 3.2) and become load-bearing only after the
// shadow-equivalence harness proves them equivalent to the legacy path.
```
