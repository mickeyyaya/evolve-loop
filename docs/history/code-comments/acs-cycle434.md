# Comment history: `acs/cycle434`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle434/predicates_test.go:3` — above `package cycle434`

```text
// Package cycle434 materialises the cycle-434 acceptance criteria for the
// completion of slice S4 of the SignalCenter consolidation campaign (goal:
// aceb01835f2c8df46c16628d7fe0630b945bf15669c965afedd19f38c826e4fd).
//
// S2 (#291), S3 (cycle-431), and S4's checkpoint migration (cycle-432) are
// all landed on main. S4's own charter — "remove [driver/reviewer] direct
// call sites so consumers no longer parse CLI chrome" — is INCOMPLETE
// against itself: two direct panestream.PaneBusy consumers survive
// (autorespond.go:282 auto-responder busy-gate; driver_tmux_repl.go:587
// idle_reached busy/idle bracket). This cycle's sole task closes that gap.
//
// Task (top_n):
//
//	s4-complete-residual-busy-callsites (M, P0):
//	  Add a STATELESS panestream.SignalCenter.BusyOf(rendered, profile) bool
//	  projection (no Observe, no per-session state — delegates to the SAME
//	  PaneBusy definition the registered Busy(sessionKey) handler already
//	  uses) and route both residual call sites through it, so no `bridge`
//	  consumer parses CLI chrome directly anymore.
//
// AC map (1:1, R9.3 floor-binding; predicates for the ## top_n task only):
//
//	AC1 auto-responder busy-gate preserved (positive)             → C434_001 pre-existing GREEN (value already correct; migration is behavior-preserving) + C434_002 (idle counterpart pin)
//	AC2 idle_reached fires once on busy→idle via facade (positive)→ C434_003 pre-existing GREEN (TestChannelE2E_RealFixtures_ClaudeSpan, channel_e2e_test.go — unaffected by BusyOf delegating to the identical PaneBusy definition)
//	AC3 no direct panestream.PaneBusy( at either residual site
//	    (negative, discriminating)                                → C434_004 RED today + C434_005 RED today
//	AC4 empty pane / unknown profile → not-busy, no panic; BusyOf
//	    is stateless (no session-state mutation), nil-receiver-safe
//	    (edge/OOD)                                                 → C434_006 RED today (compile fail) + C434_007 RED today (compile fail)
//	AC5 -race green + apicover -enforce 0 uncovered (regression)  → C434_008 RED today (compile fail cascade) + C434_009 RED today (compile fail cascade)
//
// RED strategy: C434_004/005 are independently RED on their own merits (the
// two call sites genuinely still call panestream.PaneBusy( inline today —
// verified by direct `go test -run` against the CURRENT tree before this
// cycle's test files existed). C434_001/002/003 currently PASS standalone
// (the migration is designed to be behavior-preserving, H1) but the whole
// panestream test binary — and therefore every predicate that shells into
// a package sharing a build with signalcenter_busyof_test.go — fails to
// COMPILE once BusyOf is referenced and does not yet exist; C434_006-009
// are RED for that same root cause (a hard, non-gameable RED: no
// implementation can accidentally satisfy a compile error).
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C434_004/005 (the two-site "keep the direct call AND also
//	            route through the center" cheapest fake — a source-region
//	            scan defeats a redundant call, not just a value check)
//	Edge/OOD:   C434_006 (empty pane / unknown-profile zero-value, and a
//	            nil *SignalCenter receiver)
//	Semantic:   C434_007 (BusyOf must NOT create session state — a distinct
//	            property from "returns the right bool", proven by asserting
//	            Aggregate()/Busy()/Changed() stay at their unobserved
//	            defaults after BusyOf-only calls)
//
// 1:1 enforcement:
//
//	predicate=9 (C434_001-009) → total AC = 5 (each AC gets >=1 predicate) ✓
```

### `go/acs/cycle434/predicates_test.go:198` — above `func TestC434_009_ApicoverEnforceClean(t *testing.T) {`

```text
// TestC434_009_ApicoverEnforceClean (AC5, regression — RED today, compile
// fail cascades into the coverage run): apicover -enforce must report 0
// uncovered / 0 false-green symbols on both touched packages — this is the
// recurring CI-break class from cycles 413/426/430.
```
