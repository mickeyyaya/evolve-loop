# Comment history: `acs/cycle982`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle982/predicates_test.go:3` — above `package cycle982`

```text
// Package cycle982 materializes the cycle-982 acceptance criteria for the sole
// fleet lane this cycle is pinned to: salvage-prefix-queue-composer-core
// (goal 1f6d5bf8…, campaign merge-efficiency-2026-07). Per R9.3 no predicate
// here binds to any other lane's items — fleet_scope pins this lane to exactly
// this id and its three scout-derived, triage-committed (## top_n) tasks.
//
// Cycle-981 landed the prefix composer (go/internal/fleet/prefixqueue.go) and a
// PlanLanding *plan-emitting* seam, but left the core value INERT and unsafe:
//
//	T1 wire-resolveculprit-ship-landing        — ResolveCulprit has NO production
//	   (priority H, dependsOn T2)                caller; only ComposePrefixes is
//	                                             driven, so the positional-NNFI
//	                                             culprit-resolution engine that IS
//	                                             the design's value never runs on
//	                                             the composed main-push path
//	                                             (goal no-inert-API floor).
//	T2 prefixqueue-single-writer-race-safety   — PrefixQueue mutates lanes/window
//	   (priority H)                              with NO synchronization; the
//	                                             moment a concurrent driver is
//	                                             wired this is the silent
//	                                             lost-work class (948/949).
//	T3 prefixqueue-nnfi-postejection-reverify   — positional NNFI can land a
//	   (priority M)                              poisoned COMPOSITE: solo-green
//	                                             lanes whose union is red both
//	                                             land because the composed set is
//	                                             never re-verified as a whole.
//
// SUT SURFACE the Builder must add WITHOUT modifying this file (the RED contract).
// The one symbol that does not yet exist — ship.LandPrefixes — makes this package
// FAIL TO COMPILE now, which is the correct greenfield RED per go/acs/README.md
// ("a predicate package that fails to compile is a HARD suite error, never a
// silent PASS"). T2/T3 predicates ALSO encode behavioral RED that holds once the
// missing symbol lands (verified against the current tree: a cross-group poisoned
// composite lands today, and unsynchronized concurrent Enqueue loses appends
// 30/30 rounds).
//
//	T1 (new, package go/internal/phases/ship — the composed-path DRIVER that makes
//	    ResolveCulprit non-inert; it must ROUTE through PrefixQueue.ResolveCulprit,
//	    not reimplement NNFI inline):
//	  func LandPrefixes(cfg policy.FleetConfig, lanes []fleet.LaneCandidate,
//	                    verify func(laneIDs []string) bool) (landed, ejected []string)
//	  //   prefix-queue => enqueue the lanes, resolve culprits via
//	  //                   fleet.PrefixQueue.ResolveCulprit(verify); return
//	  //                   landed / ejected. Empty lane set => nil, nil.
//	  // AND at least one non-_test .go file under internal/phases/ship must
//	  // reference ResolveCulprit (the wiring proof for the no-inert-API floor).
//
//	T2 (go/internal/fleet/prefixqueue.go): guard lanes/window with a sync.Mutex
//	    (or sync.RWMutex) across Enqueue/OnGreen/OnRed/Window so concurrent
//	    Enqueues never lose an append and the AIMD window never tears below floor.
//
//	T3 (go/internal/fleet/prefixqueue.go): after resolving, re-verify the surviving
//	    landed set as a whole; if it fails, trim rather than land a poisoned
//	    composite — invariant: verify(landed) is ALWAYS true. The single-group
//	    independent-failure path (poisoned middle lane) stays unchanged.
//
// PREDICATE STYLE (cycle-85 anti-gaming): every load-bearing predicate CALLS the
// SUT and asserts on its return value / observed side effect. The two structural
// checks (ship caller present; mutex token present) are SUPPORTING only — each
// sits beside a behavioral assertion in the same AC and matches the scout's stated
// verifiableBy grep, so neither is a sole load-bearing source-grep.
//
// Adversarial diversity (skills/adversarial-testing §6):
//
//	POSITIVE → C982_001 (innocent lanes land through the live ship driver),
//	           C982_003 (concurrent Enqueue preserves every lane),
//	           C982_005 (independent-failure NNFI still ejects the true culprit).
//	NEGATIVE → C982_001 (poisoned middle lane must NOT land; NNFI budget is LINEAR
//	           — anti-bisection), C982_002 (ResolveCulprit must have a real ship
//	           caller — an inline reimplementation leaving it inert fails),
//	           C982_005 (a poisoned CROSS-GROUP composite must NOT land — the exact
//	           F2 gap; current code lands [A,B] here).
//	EDGE     → C982_001 (empty lane set => nil/nil), C982_003 (AIMD window floors
//	           at 1 under concurrent reds), C982_005 (single-lane / empty group).
//	SEMANTIC → live-ship-wiring / single-writer-safety / poisoned-composite-trim
//	           are three DISTINCT behaviors, not one restated.
//
// AC map (1:1 with the disposition table in test-report.md):
//
//	AC-T1b live ship driver ejects culprit, lands innocents (linear NNFI) → C982_001 (predicate)
//	AC-T1c empty lane set => nil landed/ejected                          → C982_001 (predicate)
//	AC-T1a a ship non-test file references ResolveCulprit (wiring proof)  → C982_002 (predicate)
//	AC-T2a concurrent Enqueue never loses an append                      → C982_003 (predicate)
//	AC-T2c AIMD window never drops below floor 1 under concurrent reds    → C982_003 (predicate)
//	AC-T2b prefixqueue.go guards shared state with sync.Mutex/RWMutex     → C982_004 (predicate)
//	AC-T3a a poisoned composite never lands (verify(landed) always true)  → C982_005 (predicate)
//	AC-T3b independent-failure single-group NNFI path unchanged           → C982_005 (predicate)
//	AC-T3c design-note comment documents the NNFI positional limitation   → manual+checklist (Auditor)
```

### `go/acs/cycle982/predicates_test.go:121` — above `cfg := policy.Policy{Fleet: &policy.FleetPolicy{Landing: "prefix-queue"}}.FleetConfig()`

```text
// prefix-queue policy so the driver routes through the composer (resolved via
// the real policy resolver, matching the cycle-981 wiring predicate).
```
