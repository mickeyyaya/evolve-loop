# Comment history: `acs/cycle1568`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle1568/predicates_test.go:3` — above `package cycle1568`

```text
// Package cycle1568 encodes the cycle-1568 acceptance criteria for the
// `retrospective-delivery-relaunch` lane: the two production-path defects on
// the Retro dispatch route recorded in
// .evolve/inbox/2026-08-18T02-30-00Z-retro-prompt-delivery-stall.json —
//
//	retro-delivery-failure-relaunch  a typed, verified delivery failure
//	                                 (driver-classified submit_wedged, zero
//	                                 tokens, prompt parked at the pane) must
//	                                 trigger exactly one fresh Retro dispatch,
//	                                 while a generic artifact timeout must not.
//	retro-model-auto-normalization   Retro must never dispatch the literal
//	                                 "auto" model sentinel to the bridge.
//
// Every predicate here is BEHAVIORAL: it runs the retro phase's real
// Phase.Run → core.Bridge route through `go test` and asserts on the named
// PASS marker, so adding a magic string to a source file cannot green it.
```

### `go/acs/cycle1568/predicates_test.go:73` — above `func TestC1568_003_relaunch_test_drives_production_retro_route(t *testing.T) {`

```text
// AC3 (retro-delivery-failure-relaunch): the regression rides the ACTUAL
// Core→Bridge/Retro route, not a diagnostic serializer. Proven two ways: the
// reproducer is git-TRACKED (an untracked test is dropped at ship, cycle-93),
// and it drives retro.Phase.Run through the core.Bridge port.
```

### `go/acs/cycle1568/predicates_test.go:120` — above `func TestC1568_007_retro_package_suite_green(t *testing.T) {`

```text
// AC7 (both tasks, regression): the whole retro phase package stays green —
// the existing SKIPPED/PASS/FAIL verdict mapping, the profile-CLI dispatch
// pins (cycle-107 class), and the new relaunch/model contract must hold
// together, not one at the cost of another.
```
