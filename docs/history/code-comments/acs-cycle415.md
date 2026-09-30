# Comment history: `acs/cycle415`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle415/predicates_test.go:3` — above `package cycle415`

```text
// Package cycle415 materializes the cycle-415 acceptance criteria for three prompt-compaction tasks:
//   - tdd-prompt-reference-index-tail (Task 1)
//   - triage-prompt-reference-index-tail (Task 2)
//   - compact-marker-presence-gate (Task 3)
//
// Goal: add line-anchored ## Reference Index headings to evolve-tdd-engineer.md and
// evolve-triage.md so the shipped StripOnDemandSections compaction fires on every cycle
// (currently 0 bytes stripped from the two largest always-on phase docs).
//
// AC map (1:1 with scout-report.md top_n; R9.3 floor-binding):
//
//	tdd-prompt-reference-index-tail:
//	  AC1 stripped body ≥1500 B smaller than full tdd-engineer.md body               → C415_001 (RED)
//	  AC2 required behavior anchors survive above the ## Reference Index marker        → C415_002 (pre-existing GREEN)
//	  AC3 synthetic: buried rule absent from stripped body (negative/anti-gaming)      → C415_003 (pre-existing GREEN)
//
//	triage-prompt-reference-index-tail:
//	  AC1 stripped body ≥1200 B smaller than full triage.md body                      → C415_004 (RED)
//	  AC2 required output sections + carryover rule survive strip                      → C415_005 (pre-existing GREEN)
//	  AC3 versioned-historical sections absent from stripped body (negative — RED)     → C415_006 (RED)
//
//	compact-marker-presence-gate:
//	  AC1 all 6 always-on phase docs have a line-anchored ## Reference Index heading   → C415_007 (RED)
//	  AC2 inline prose mention does NOT satisfy the gate (negative)                    → C415_008 (pre-existing GREEN)
//
// Adversarial diversity (per SKILL §6):
//
//	Negative: synthetic buried rule → C415_003; inline prose mention → C415_008; versioned sections in stripped → C415_006.
//	Edge/OOD: heading at EOF (StripOnDemandSections covered by cycle-413); inline mention mid-body → C415_008.
//	Semantic: byte-delta (C415_001/C415_004) vs anchor-presence (C415_002/C415_005) vs gate (C415_007) are distinct behaviors.
//
// Deferred (zero predicates per R9.3): multi-marker config-driven strip (beyond-ask #2),
// report-size caps, orchestrator.md under-compaction (993 B).
```

### `go/acs/cycle415/predicates_test.go:69` — above `if saved < 64 {`

```text
// Recalibrated 2026-08-10 (was 1500): that floor required the predicate-
// quality rules below the strip marker, deleting them from every dispatched
// tdd prompt (docs/incidents/2026-08-10-persona-strip-lobotomy.md). Marker
// now at EOF; the phasecoherence keep-guard governs strippable content.
```

### `go/acs/cycle415/predicates_test.go:140` — above `if saved < 64 {`

```text
// Recalibrated 2026-08-10 (was 1200): that floor required the inbox-
// ingestion + idempotency-skip-list instructions below the strip marker
// (persona-strip lobotomy incident, queue-starvation half). Marker now at
// EOF with the step-3b verbose example as the genuine tail.
```

### `go/acs/cycle415/predicates_test.go:195` — above `for _, kept := range []string{`

```text
// INVERTED 2026-08-10 (persona-strip lobotomy incident): these sections
// were misclassified "versioned-historical" because their headings carry
// version tags — they are the inbox-ingestion and idempotency operating
// rules, and stripping them starved the queue. They must SURVIVE.
```

### `go/acs/cycle415/predicates_test.go:245` — above `func TestC415_008_InlineMentionNotCountedAsMarker(t *testing.T) {`

```text
// TestC415_008_InlineMentionNotCountedAsMarker asserts that an inline prose mention
// of ## Reference Index does NOT satisfy the compact-marker gate.
// Negative / anti-gaming: a naive strings.Contains check would accept inline mentions
// (the cycle-413 strip fix demonstrated this failure mode already).
// Pre-existing GREEN: bodyHasCompactMarker mirrors StripOnDemandSections's line-anchored logic.
```
