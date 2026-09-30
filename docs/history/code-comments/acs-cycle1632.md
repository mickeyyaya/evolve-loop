# Comment history: `acs/cycle1632`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1632/predicates_test.go:3` — above `package cycle1632`

```text
// Package cycle1632 materializes the acceptance criteria for cycle 1632's
// single fleet-scoped task `tokenopt-handoff-digests` — the retry of the
// cycle-1593 attempt. That attempt was audit-FAILed for (H1) a shadow
// comparator that compared the raw legacy value against the newly-capped
// typed getter and so leaked the uncapped text into the ledger, and (H2/M1)
// a raw ctxSnap["carryover_summary"] writer that ADDED 218,480 bytes to every
// below-enforce triage prompt on a path that previously carried zero.
//
// The contract this cycle (single-sourced in internal/phaseio):
//
//	MaxFieldBytes    exported positive cap (bytes)
//	TruncationMarker exported non-empty visible marker
//	CapField(s)      identity at/under the cap; rune-aligned prefix + marker
//	                 above it, always <= MaxFieldBytes, idempotent
//	NewCycleInputs   applies CapField to Carryover and PreviousVerdict
//	shadow comparator compares CapField(legacy) against the typed getter
//	NO raw carryover_summary writer on any dispatch path
//
// AC map (1:1 with test-report.md ## AC-Materialization):
//
//	AC1 oversized field capped with a visible marker (exported helper)  → TestC1632_001_OversizedFieldIsCappedWithVisibleMarker
//	AC2 the cap reaches the real dispatch → triage prompt at enforce     → TestC1632_002_EnforceDispatchBoundsCarryoverInRealTriagePrompt
//	AC3 empty / exact-boundary / multibyte inputs stay safe             → TestC1632_003_BoundaryAndMultibyteInputsStaySafe
//	AC4 cap-equivalent legacy value → no false shadow mismatch (live)   → TestC1632_004_ShadowDispatchEmitsNoFalseMismatchForCappedCarryover
//	AC5 a genuinely mutated typed value is still reported as drift      → TestC1632_005_GenuineDriftStillDetected_BindsCoreTest
//	AC6 StageOff/StageShadow mint no raw carryover bytes (baseline 0)   → TestC1632_006_NoRawCarryoverWriterBelowEnforce
//
// Audit-repair round (audit round 1 FAILed the build on H1/M1, L1 prescribed):
//
//	AC7 (H1) build-report.md claims no full closure of the partially-met
//	    inbox item and names where the unmet remainder is queued          → TestC1632_007_BuildReportDoesNotClaimFullClosureOfPartialInboxItem
//	AC8 (H1) the unmet remainder (per-edge explicit artifact-flow config;
//	    the ComposePrompt phases still without a digest; instead-of, not
//	    additive) is a tracked, claimable, rankable .evolve/inbox item    → TestC1632_008_UnmetRemainderIsQueuedAsTrackedInboxItem
//	AC9 (M1) an oversized scalar in ANY one UpstreamDigest section can
//	    never evict another present section (degraded rows survive)       → TestC1632_009_UpstreamDigestOversizedScalarCannotEvictOtherSections
//	AC10 (L1) tdd renders the digest at internal/phaseio's package-default
//	    cap — no forked literal at the call site                          → TestC1632_010_TDDPromptDigestCapIsThePackageDefault
//
// Audit round 2 (the rebuild's `Status: PASS` rested on round 1's suite line):
//
//	AC11 (H1) every `[acs suite]` receipt in build-report.md is this cycle's,
//	     green, and counts >= the declared TestC1632_ predicates             → TestC1632_011_BuildReportSuiteReceiptIsFreshForThisRound
//
// Adversarial axes (skills/adversarial-testing §6):
//   - NEGATIVE — 001 rejects a silent prefix cut and an unchanged pass-through;
//     005 is the anti-blanket-suppression case (a comparator that simply
//     skips long fields passes 004 and FAILS 005); 006 fails the moment any
//     writer mints the key.
//   - EDGE / OOD — 003 drives "", the exact boundary, cap-1, and 2/3/4-byte
//     runes straddling the boundary.
//   - SEMANTIC — 001/003 pin the REPRESENTATION, 002 pins REACHABILITY through
//     the production dispatch seam into the real triage prompt, 004 pins the
//     COMPARATOR through the live shadow artifact + ledger, 006 pins the
//     PROMPT BASELINE. Five distinct behaviors, not one restated.
//
// No grep-only predicates (cycle-85 ban): 001/003 call the exported helper
// and DTO constructor; 002/004/006 drive the real core.Orchestrator.RunCycle
// over the production PhaseIO seam (fixtures fakes, a real git worktree so the
// Build explanation contract seals) and compose the REAL triage prompt from
// the dispatched request; 005 binds the frozen core unit test by its
// `--- PASS:` marker (one package, -run-narrowed — the cycle-976/1587
// precedent) using the shared "binding test X did NOT pass" vocabulary so a
// phantom (renamed / never-written) binding is classified, not just red.
//
// Reachability probe (cycle-644 rule): every package-qualified pin here was
// compiled from this package during authoring — core already imports phaseio
// (internal/core/phaseio_shadow.go), and acs/cycle1632 → core/triage/phaseio/
// fixtures is a leaf import; no cycle is possible.
```

### `go/acs/cycle1632/predicates_test.go:201` — above `for _, in := range []string{overCap('x', 218480-phaseio.MaxFieldBytes), overCap('y', 1)} {`

```text
// The cycle-1593 measured incident size, and the smallest over-cap input.
```

### `go/acs/cycle1632/predicates_test.go:301` — above `type shadowDoc struct {`

```text
// ---------------------------------------------------------------------------
// AC4 — comparator through the LIVE shadow path: an over-cap legacy value
// dispatched at StageShadow yields no cycle_inputs.carryover mismatch in the
// shadow artifact or the ledger, and no ledger message carries the raw text.
// Guards the cycle-1593 round-2 half-fix (cap in the DTO, raw compare in the
// comparator).
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1632/predicates_test.go:376` — above `func TestC1632_006_NoRawCarryoverWriterBelowEnforce(t *testing.T) {`

```text
// ---------------------------------------------------------------------------
// AC6 — prompt baseline: with a populated carryover backlog in state and no
// carryover_summary in the request, StageOff and StageShadow dispatch NO
// carryover bytes — no Context key minted, zero PhaseInput, and the real
// triage prompt has no carryover line. Enforce is included: no writer may
// exist on ANY path (cycle-1593 M1 measured +4,096 there too).
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1632/predicates_test.go:423` — above `const laneItemID = "tokenopt-handoff-digests"`

```text
// ===========================================================================
// Audit-repair round — the audit's own findings, encoded as RED before the
// rebuild (never a weakening of the six predicates above).
//
//   H1  build-report.md declared `Closes-Inbox: tokenopt-handoff-digests` for
//       an item whose second acceptance criterion (per-edge explicit
//       artifact-flow config) is unimplemented and whose first is met for one
//       phase, additively; committedInboxIDs (internal/phases/ship/postship.go)
//       unions triage top_n ∪ lane-scope ∪ the marker, so a PASS landing
//       retires the item with the unmet remainder recorded NOWHERE — the
//       third audit on this lane to find it (cycle-1604 debb0673…/d7f2448d…).
//   M1  Handoffs.UpstreamDigest renders the typed views first and truncates
//       blindly at the rune cap with no per-section budget, so one oversized
//       agent-authored scalar (ScoutView.CycleSizeEstimate is copied
//       unvalidated from JSON) evicts every `degraded:` row (cycle-1604
//       df875c36…/d69bd9f3…). TestC1604_003 could not observe it because its
//       fixture set only Degraded.
//   L1  tdd.go passes a bare 1024 literal that forks the unexported
//       defaultUpstreamDigestRunes (cycle-1604 dbd6c43a…/dd88cbdc…).
//
// Dual-root idiom (go/acs/README.md): EVOLVE_PROJECT_ROOT → the STATE root
// (.evolve/runs/cycle-1632/build-report.md lives on main), EVOLVE_WORKTREE_ROOT
// → the SOURCE root (the tracked .evolve/inbox copy and go/ sources the ship
// lands). Each falls back to the repo root the suite runs from.
// ===========================================================================
```

### `go/acs/cycle1632/predicates_test.go:587` — above `func TestC1632_008_UnmetRemainderIsQueuedAsTrackedInboxItem(t *testing.T) {`

```text
// ---------------------------------------------------------------------------
// AC8 (H1) — the remainder is DURABLE: exactly one fresh inbox-root item links
// to the lane item, names all four parts of the unmet work in its acceptance
// (the words the ADR-0098 Task Contract will project next time), is resolvable
// by the real claim-path resolver, is rankable, and is git-tracked (cycle-93:
// on-disk-but-untracked is dropped at ship). A scout-report `## Deferred`
// paragraph is prose, not a queue.
// ---------------------------------------------------------------------------
```

### `go/acs/cycle1632/predicates_test.go:774` — above `var suiteReceiptRe = regexp.MustCompile('\[acs suite\] cycle=(\d+) verdict=(\w+) green=(\d+) red=(\d+) skip=(\d+) total=…`

```text
// ===========================================================================
// Audit round 2 (audit-repair, second rejection): the rebuild did the H1 file
// writes but the remainder's acceptance never said "per-edge", so 007/008
// stayed RED on the handed-off bytes — and build-report.md nevertheless
// claimed `./acs/cycle1632 PASS` while its `## Regression Slice` repeated
// round 1's `[acs suite] … (cycle=9 …)` line verbatim (the round-2 lane had 21
// this-cycle rows). The suite was never re-run after 007–010 landed; a
// `Status: PASS` rested on a stale receipt (claim-discrepancy).
//
//	AC11 (H1, round 2) every `[acs suite]` receipt in build-report.md is a
//	     fresh, green run over THIS tree's predicates            → TestC1632_011_BuildReportSuiteReceiptIsFreshForThisRound
//
// The wording half of round-2 H1 needs no new predicate: 007/008 already
// encode it and are RED on the current bytes (never weakened).
// ===========================================================================
```

### `go/acs/cycle1632/predicates_test.go:818` — above `func TestC1632_011_BuildReportSuiteReceiptIsFreshForThisRound(t *testing.T) {`

```text
// ---------------------------------------------------------------------------
// AC11 (H1, round 2) — the landing report's suite receipt is a run over THESE
// predicates, and it is green. Every `[acs suite]` line the report carries must
// name this cycle, report verdict=PASS with red=0, and count at least as many
// this-cycle rows as there are TestC1632_ predicates declared on the tree: a
// receipt with fewer rows than declared tests cannot have been produced by
// running them (round 2 pasted cycle=9 against 10 declared). A report with no
// receipt at all, a red receipt, or a stale one left beside a fresh one is
// each RED — a `Status: PASS` may rest only on a suite run over this tree.
// ---------------------------------------------------------------------------
```
