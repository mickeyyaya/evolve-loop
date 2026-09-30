# Comment history: `acs/cycle431`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## go/acs bulk strip (2026-10-01)

### `go/acs/cycle431/predicates_test.go:3` — above `package cycle431`

```text
// Package cycle431 materialises the cycle-431 acceptance criteria for slice
// S3 of the SignalCenter consolidation campaign (goal:
// aceb01835f2c8df46c16628d7fe0630b945bf15669c965afedd19f38c826e4fd).
//
// S2 (prior cycle, #291) shipped panestream.SignalCenter fully built and
// behaviorally tested but with ZERO consumers. S3 (this cycle) wires it as
// the AUTHORITATIVE liveness source in the tmux-REPL driver's stop-review
// checkpoint and retires the reviewer's pre-S3 Progressed/Busy boolean
// fallback (verdict becomes a pure function of StopEvent.State).
//
// Tasks:
//
//	signalcenter-s3-authoritative-driver (Task A — M, P0):
//	  Wire Observe+Aggregate at the checkpoint; retire the boolean fallback;
//	  keep Progressed/Busy populated for fatalpane.go C2 + logging; preserve
//	  the cycle-291 render-wedge override.
//
//	signalcenter-s3-wedge-invariant-corpus (Task B — S, P1, dependsOn Task A):
//	  A behavioral corpus pinning every wedge-incident invariant (311/312,
//	  254/255, 262, 286/288) against the migrated, center-authoritative
//	  verdict path.
//
// AC map (1:1, R9.3 floor-binding; predicates for ## top_n tasks only):
//
//	Task A (signalcenter-s3-authoritative-driver):
//	  AC1 driver calls Observe+Aggregate (positive)              → C431_001 RED (compile fail)
//	  AC2 verdict = f(State), boolean fallback retired (negative)→ C431_002 RED (behavioral contradiction TODAY)
//	  AC3 full bridge+panestream -race green (regression)        → C431_003 RED (compile fail)
//	  AC4 apicover -enforce clean on both packages (regression)  → C431_004 RED (compile fail cascades into coverage run)
//	  AC5 negative: center not bypassed                          → C431_005 RED (compile fail)
//	  AC6 edge: cycle-291 render-wedge preserved                 → C431_006 RED (compile fail)
//
//	Task B (signalcenter-s3-wedge-invariant-corpus):
//	  AC1 311/312 producing-not-capped (positive)                → C431_007 RED (compile fail — same package as Task A)
//	  AC2 254/255 bounded busy (positive)                        → C431_008 RED (compile fail)
//	  AC3 262 dead-pane not progress (negative)                  → C431_009 RED (compile fail)
//	  AC4 286/288 evidence survives (edge)                       → C431_010 RED (compile fail)
//	  AC5 corpus green under -race (regression)                  → C431_011 RED (compile fail)
//	  AC6 anti-gaming: names Liveness* + calls Review             → C431_012 RED (compile fail)
//
// RED strategy: C431_001, 003–012 are all RED for the SAME root cause —
// driver_tmux_repl_signalcenter_test.go references the not-yet-existing
// Deps.LivenessCenter field, so the whole internal/bridge test binary fails
// to COMPILE (a hard, non-gameable RED: no implementation can accidentally
// satisfy a compile error, and every test in the package — including Task
// B's, which depends on Task A — inherits the failure). C431_002 is
// independently RED on its own merits: it calls the REAL, ALREADY-COMPILING
// deterministicReviewer.Review and asserts the OPPOSITE of what it returns
// TODAY (Extend, via the still-present boolean fallback), so it fails for a
// behavioral reason even standing alone, with no dependency on the compile
// failure elsewhere.
//
// Adversarial diversity (SKILL §6):
//
//	Negative:   C431_002 (unset State + Progressed/Busy true must PAUSE, not
//	            extend — contradicts the current fallback), C431_005 (probe
//	            must be CALLED, not just state-coincidence), C431_009 (Hung
//	            must never extend unconditionally like Converging)
//	Edge/OOD:   C431_006 (blank-but-live render wedge), C431_010 (evidence
//	            across a dead-session empty capture), C431_007 (attempt
//	            count far past maxExtends)
//	Semantic:   C431_001 (state SOURCE) vs C431_005 (probe INVOCATION) are
//	            distinct proofs — a state-only check would pass on
//	            accidental coincidence even if the center were bypassed;
//	            C431_007 vs C431_008: unconditional-extend vs
//	            bounded-extend-then-pause are different reviewer behaviors,
//	            not one behavior restated
//
// 1:1 enforcement:
//
//	Task A: predicate=6 (C431_001–006) → total=6 ✓
//	Task B: predicate=6 (C431_007–012) → total=6 ✓
```

### `go/acs/cycle431/predicates_test.go:146` — above `func TestC431_004_ApicoverEnforceClean(t *testing.T) {`

```text
// TestC431_004_ApicoverEnforceClean (AC4, regression, RED — compile fail
// cascades into the coverage run): apicover -enforce must report 0
// uncovered / 0 false-green symbols on both touched packages — this is the
// recurring CI-break class from cycles 413/426/430.
```

### `go/acs/cycle431/predicates_test.go:209` — above `func TestC431_006_RenderWedgeOverridePreserved(t *testing.T) {`

```text
// TestC431_006_RenderWedgeOverridePreserved (AC6, edge, RED — compile
// fail): the cycle-291 blank-but-live render-wedge override must still
// promote Idle→BusyButStagnant after the migration to the
// center-authoritative State source.
```

### `go/acs/cycle431/predicates_test.go:222` — above `func TestC431_007_ConvergingProducingNeverCapped(t *testing.T) {`

```text
// TestC431_007_ConvergingProducingNeverCapped (AC1, positive, RED — compile
// fail): cycle-311/312 — a producing agent is never capped.
```

### `go/acs/cycle431/predicates_test.go:231` — above `func TestC431_008_BusyStagnantBoundedThenPause(t *testing.T) {`

```text
// TestC431_008_BusyStagnantBoundedThenPause (AC2, positive, RED — compile
// fail): cycle-254/255 — bounded busy extends then pauses at maxExtends.
```

### `go/acs/cycle431/predicates_test.go:240` — above `func TestC431_009_DeadPaneHungNotConverging(t *testing.T) {`

```text
// TestC431_009_DeadPaneHungNotConverging (AC3, negative, RED — compile
// fail): cycle-262 — a dead/echoing pane must classify Hung, never
// Converging.
```

### `go/acs/cycle431/predicates_test.go:250` — above `func TestC431_010_EvidenceSurvivesEmptyCapture(t *testing.T) {`

```text
// TestC431_010_EvidenceSurvivesEmptyCapture (AC4, edge, RED — compile
// fail): cycle-286/288 — non-empty pane evidence survives a dead-session
// empty capture.
```
