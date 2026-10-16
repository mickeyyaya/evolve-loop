# Comment history: `internal/cycleoutcome`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/cycleoutcome/cycleoutcome.go:88` — above `committed := CommittedIDsFor(in.Workspace)`

```text
// Continuation/retry cycles carry NO triage-decision.json (the
// continuation path binds the task directly), so the committed set
// resolved nil and the durable failure_count was never bumped — after 15
// FAILs every live inbox item sat at 0 and the TaskRetryCeiling
// quarantine + deep-escalation governors were unreachable (2026-08-10
// investigation). The lane-scope pin is the worked set on those cycles.
// File-ABSENT, not merely empty (diff-review MEDIUM): a fresh cycle whose
// triage legitimately committed zero ids must not have its lane-scope menu
// blamed for the failure — the pinned items were explicitly declined.
```

## comment-zero D1

### `go/internal/cycleoutcome/cycleoutcome.go:1` — above `package cycleoutcome`

```text
// Package cycleoutcome is the importable FAIL-closeout seam: the symmetric
// sibling of the PASS half that already lives inside the cycle process
// (internal/phases/ship/postship.go).
//
// Why a package and not more inline `main` code: the FAIL closeout used to be
// an inline block in cmd_loop.go's SEQUENTIAL branch only. `package main` is
// not importable, so (a) no predicate could ever pin the behavior, and (b) the
// wave path — whose lanes run `evolve cycle run` as a subprocess and whose
// fleet.Result{Index,ExitCode,Err} carries neither cycle number nor workspace —
// had no way to reuse it. Fleet-dispatched work therefore never bumped
// failure_count and the ADR-0072 S5 retry ceiling was structurally unreachable
// (wave-lane-task-quarantine-dead; batch-14 burned four FAILs on the same ids
// with failure_count stuck at 0).
//
// Both closeout paths now call ApplyFailure, so the two halves cannot drift
// apart again (never_duplicate_centralize).
```

### `go/internal/cycleoutcome/cycleoutcome.go:34` — above `type FailureInputs struct {`

```text
// FailureInputs is one failed cycle's closeout context. Workspace is the
// cycle's .evolve/runs/cycle-N dir — the sole source of the committed set
// (triage-decision.json), so the seam bumps exactly the ids triage worked and
// never a lane's whole claimed menu (PR #366 menu semantics).
```

### `go/internal/cycleoutcome/cycleoutcome.go:43` — above `SystemLevel bool`

```text
// ADR-0072 S3: releases, never bumps, never quarantines
```

### `go/internal/cycleoutcome/cycleoutcome.go:53` — above `Ledger inboxmover.LedgerAppender`

```text
// Ledger is the chained-append seam the lifecycle lines go through — the
// root's Signal-Center-observed ledger (ADR-0101 S4a). nil lets the inbox
// mover fall back to its own, unobserved file ledger over the same file.
```

### `go/internal/cycleoutcome/cycleoutcome.go:57` — above `Signals *signalcenter.Center`

```text
// Signals is the root's Signal Center the inbox mover's inbox.warning
// events go to (ADR-0103 unit 06). nil = unwired: the mover prints its
// legacy [inbox-mover] line instead.
```

### `go/internal/cycleoutcome/cycleoutcome.go:99` — above `routed := false`

```text
// The per-item breaker (docs/incidents/2026-09-14-triage-refusal-poison-loop.md):
// a protected-surface refusal is deterministic AND operator-owned, so the
// refused card is routed console-manual in place — wherever the lane's
// claim left it — BEFORE the drain releases it, and the claim floor refuses
// every later lane. The route is that item's disposition and the cycle's
// other committed ids were never worked (triage stopped the cycle), so the
// drain then runs with Routed set (no bump, no park — SystemLevel stays the
// fact it is). A route that cannot happen has said so on the stream and
// falls back to the task-level bump — the S5 ceiling is the second breaker.
```

### `go/internal/cycleoutcome/cycleoutcome.go:111` — above `fmt.Fprintf(stderr, "[cycleoutcome] WARN: route-console skipped: refused subject %q is not in the committed set %v — tas…`

```text
// An agent-authored id never widens what an agent may do (ADR-0073):
// a Subject outside this cycle's committed set is a mis-copy, not a
// disposition — say so and let the task-level bump run.
```

### `go/internal/cycleoutcome/cycleoutcome.go:161` — above `func IsTaskLevelFailure(c cycleclassify.Classification) bool {`

```text
// IsTaskLevelFailure reports whether the classification blames the TASK (and so
// may accrue failure_count toward the S5 ceiling). Transient infra, quota storms
// and kernel breaches are the pipeline's fault, not the todo's (ADR-0072 AC4).
// Exported because the loop's quarantine surface asks the same question — one
// definition, so "what counts as the task's fault" cannot fork.
```

### `go/internal/cycleoutcome/cycleoutcome.go:189` — above `func LaneScopeIDs(workspace string) []string {`

```text
// LaneScopeIDs reads the lane-scope pin's todo ids from the workspace — the
// triage-less (continuation/lane) committed set, shared by BOTH outcome
// halves: the FAIL closeout here (failure_count accounting, PR #439) and the
// PASS promotion in phases/ship/postship.go (consumption-rides-landing-ship)
// — one reader, so the two sides can never disagree about what a triage-less
// cycle worked.
// Delegates to the same projection used by the TDD and Build contracts.
```

### `go/internal/cycleoutcome/cycleoutcome_ledger_test.go:3` — above `import (`

```text
// cycleoutcome_ledger_test.go — ADR-0101 S4a: the failure walk appends its
// inbox-lifecycle lines through the ledger the caller injects (the root's
// Signal-Center-observed one), never through a self-constructed FileLedger
// over the same file (S4a architecture review HIGH-1).
```

### `go/internal/cycleoutcome/cycleoutcome_signals_test.go:3` — above `import (`

```text
// cycleoutcome_signals_test.go — ADR-0103 unit 06 step 4 (test 53): the FAIL
// closeout threads the root's Signal Center into the inbox mover beside the
// ledger it already threads, so the walk's INBOX_* events reach signals.ndjson.
```

### `go/internal/cycleoutcome/cycleoutcome_test.go:94` — above `func TestApplyFailureSystemLevelDoesNotQuarantine(t *testing.T) {`

```text
// TestApplyFailureSystemLevelDoesNotQuarantine pins ADR-0072 AC4: a pipeline
// failure releases the committed id but must never walk it toward the ceiling.
```

### `go/internal/cycleoutcome/lane_scope_projection_test.go:3` — above `import (`

```text
// lane_scope_projection_test.go — the durable regression pin for AC3 of inbox
// item `multi-slug-lane-scope-reconciliation` (cycle-1620): ONE projection
// source yields the lane's slug set for every consumer.
//
// Why this is the defect's other half. cycle-1480's audit FAIL H1 reads "TDD
// minted a cycle-wide predicate suite covering both slugs while the Builder
// contract bound only the first slug; nothing reconciles the two scopes." Two
// scopes exist to disagree only because two parsers of lane-scope.json exist:
// go/internal/core/lanescope.go and this package. A coherence gate on top of
// two derivations treats the symptom; collapsing the derivations removes the
// class (never_duplicate_centralize_via_design_patterns).
//
// Import directions, compiler-probed 2026-09-09 (cycle-644 obligation — never
// freeze a pin whose import shape is unproven):
//
//	core -> cycleoutcome  BLOCKED: cycleoutcome -> inboxmover -> adapters/ledger
//	                      -> core is an import cycle ("import cycle not allowed").
//	cycleoutcome -> core  BUILDABLE (`go build ./internal/cycleoutcome` clean),
//	                      despite the stale "cannot import core" comment on
//	                      cycleoutcome.go's LaneScopeIDs.
//
// So the collapse has two legal shapes — a leaf package both import, or this
// package delegating to core — and these tests pin the BEHAVIOR and the
// declaration COUNT, never which shape was chosen.
```

### `go/internal/cycleoutcome/lane_scope_projection_test.go:49` — above `func TestLaneScopeProjection_OrderedAndFailOpen(t *testing.T) {`

```text
// TestLaneScopeProjection_OrderedAndFailOpen pins the projection contract every
// consumer inherits: the pinned members come back IN ORDER, and every degraded
// pin fails OPEN to the empty set. A projection that returned a partial set on a
// malformed pin would hand two consumers two different truths — the exact
// divergence this AC removes — and one that aborted would recreate the
// cycle-760..762 false-abort destruction class.
```

### `go/internal/cycleoutcome/lane_scope_projection_test.go:72` — above `func TestLaneScopeProjection_SingleWireShapeDeclaration(t *testing.T) {`

```text
// TestLaneScopeProjection_SingleWireShapeDeclaration is the AC's literal
// grep-proof: "no second parser of lane-scope slugs". Two declarations of the
// wire shape are two parsers, and two parsers are what let the TDD scope and the
// Builder contract scope diverge in cycle-1480.
```

### `go/internal/cycleoutcome/lanescope_apicover_named_test.go:3` — above `import (`

```text
// lanescope_apicover_named_test.go — apicover named binding for the exported
// LaneScopeIDs (issue #433 class: a newly exported surface needs a NAMED
// covering test in its owning package; the ship-side consumer and the
// ApplyFailure path exercise it indirectly). Behavior is pinned by
// lanescope_fallback_test.go end-to-end; this binds the exported name.
```

### `go/internal/cycleoutcome/lanescope_fallback_test.go:3` — above `import (`

```text
// lanescope_fallback_test.go — RED contract for the stable failure identity
// (2026-08-10 investigation, agent C): continuation/retry cycles have NO
// triage-decision.json (the continuation path binds the task directly), so
// ApplyFailure derived a nil committed set and the durable failure_count was
// never bumped — all 81 live inbox items sat at 0 after 15 FAILs, leaving
// TaskRetryCeiling quarantine and deep-escalation permanently unreachable.
// Fallback: when triage committed nothing, the lane-scope pin
// (<workspace>/lane-scope.json TodoIDs) is the worked set.
```

### `go/internal/cycleoutcome/nowork.go:3` — above `import (`

```text
// nowork.go — the planned-no-work closeout (F30), the sibling of ApplyFailure.
//
// A fleet lane whose triage answered for its scoped item without committing it
// (committedset.Dispositions: a reasoned drop, a skip, an escalation) ends as
// planned no-work. The item is still pending in the inbox root — triage builds
// its menu there — so the next wave would draw it into another lane and
// another no-work end. This closeout hands it to the console IN PLACE with the
// lane's reason, and the ADR-0074 claim floor refuses every later lane. The
// console confirms and retires it: an unshipped lane never retires work (the
// anti-laundering principle behind the closure-claim gate).
```

### `go/internal/cycleoutcome/nowork.go:59` — above `func ApplyNoWork(in NoWorkInputs) (NoWorkResult, error) {`

```text
// ApplyNoWork hands every lane-scoped item the lane's triage answered for to
// the console and returns the ids it routed; then — the sibling of
// ApplyFailure's routed drain — it releases the cycle's claims to the inbox
// root with no failure bump, because triage claims before it selects and a
// no-work lane worked nothing (F30 architecture review C1): a routed item
// arrives console-visible instead of stranded in the gitignored claim dir.
// A cycle without a lane pin has no scope to hand over and releases nothing
// (a sequential no-work end keeps its historical lifecycle). An answered id
// OUTSIDE the scope is never touched (an agent-authored id never widens what
// a closeout may do — ApplyFailure's rule); one already routed to the console
// keeps its original evidence (review m3); one no longer in the inbox (a ship
// consumed it, or it was never an inbox item) is simply gone, not a route
// failure (review m4). Route and release faults are on the stream and are
// returned joined, for the caller to WARN — a lifecycle hiccup never changes
// a cycle's exit code.
```

### `go/internal/cycleoutcome/nowork_route_test.go:3` — above `import (`

```text
// nowork_route_test.go — F30's loop breaker. A fleet lane whose triage answered
// for its scoped item without committing it (a reasoned drop, a skip, an
// escalation) now ends as planned no-work instead of a claim failure — but the
// item is still pending in the inbox root, so the next wave would draw it into
// another lane and another no-work end. The no-work closeout hands every
// scoped item the lane answered for to the console, in place, with the lane's
// reason: the console confirms and retires it (an unshipped lane never retires
// work — the anti-laundering principle behind the closure-claim gate).
```

### `go/internal/cycleoutcome/nowork_route_test.go:106` — above `func TestApplyNoWork_AppendsTheRouteThroughTheInjectedLedger(t *testing.T) {`

```text
// TestApplyNoWork_AppendsTheRouteThroughTheInjectedLedger: like the failure
// walk (ADR-0101 S4a), the hand-off's lifecycle line goes through the root's
// observed ledger, never a self-constructed one over the same file.
```

### `go/internal/cycleoutcome/nowork_route_test.go:128` — above `func TestApplyNoWork_ReleasesWhatTheLaneClaimed(t *testing.T) {`

```text
// TestApplyNoWork_ReleasesWhatTheLaneClaimed (F30 architecture review C1):
// triage claims before it selects, so an answered scoped item may sit in
// processing/cycle-N/. The hand-off routes it there, in place, and then
// releases the cycle's claims to the root with no failure bump — the item
// arrives console-visible instead of stranded in a gitignored claim dir.
```

### `go/internal/cycleoutcome/refusal_route_test.go:3` — above `import (`

```text
// refusal_route_test.go — the FAIL closeout's per-item breaker. A phase-refusal
// (cycleclassify.ClassPhaseRefusal, read from the C1 record) is TASK-level, so
// the drain's failure_count bump and the ADR-0072 S5 ceiling finally reach the
// item; and the one refusal that is deterministic AND operator-owned — a top_n
// card naming a protected surface — routes the committed item to
// console-manual on the FIRST hit, in place, before the drain releases it
// (docs/incidents/2026-09-14-triage-refusal-poison-loop.md: one item drew nine
// lanes, cycles 1650–1675, because none of this was reachable).
```

### `go/internal/cycleoutcome/refusal_route_test.go:77` — above `func TestApplyFailure_ProtectedSurfaceRefusal_RoutesTheItemConsole(t *testing.T) {`

```text
// The lane's triage claimed the item into processing/cycle-7/ and then refused
// it for a protected surface: the closeout routes it console-manual IN PLACE,
// the drain releases it to the root already routed, no failure_count is
// bumped, the ceiling does not quarantine it (the route is the disposition),
// and the stream carries INBOX_ITEM_ROUTED_CONSOLE naming the refusal.
```
