# Comment history: `internal/inboxmover`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/inboxmover/claim_lane_scope_already_claimed_test.go:3` — above `import (`

```text
// claim_lane_scope_already_claimed_test.go — ClaimLaneScope exists so a
// committed id the lane never claimed still reaches processing/ before the
// FAIL drain; an id the lane DID claim (it sits in processing/cycle-N/) is
// already there, and re-claiming it produced a false INBOX_CLAIM_NOT_FOUND on
// every closeout (cycle 1675's stream, the 2026-09-14 poison-loop incident).
```

### `go/internal/inboxmover/claimstate.go:3` — above `import "github.com/mickeyyaya/evolve-loop/go/internal/inboxmover/lifecycle"`

```text
// claimstate.go — the read-only view of where an item stands in the claim
// lifecycle. Locate is the ONE walk over the inbox root and processing/cycle-*/
// (Promote's source resolution, the continuation scope readers and the
// ADR-0100 declared-effects gate all go through it), and it derives the layout
// from inboxbatch exactly as the writer, Claim, does — so no reader can drift
// from where a claim actually lands. The walk lives in the lifecycle leaf
// since ADR-0103 unit 06; this file keeps the spelling.
```

### `go/internal/inboxmover/claimstate_test.go:3` — above `import (`

```text
// claimstate_test.go — the read-only lifecycle reader the ADR-0100
// declared-effects gate consumes (deliverable/effects.go). This package owns
// the processing/cycle-N/ layout; the gate never re-derives it.
```

### `go/internal/inboxmover/closesmarker.go:3` — above `import (`

```text
// closesmarker.go — the builder-authored closure marker parser.
//
// Consuming an inbox item used to be a separate act from the ship that closed
// it, so forgetting was always possible: #453 landed
// schema-aligned-salvage-layer without consuming its item and wave cycle-1448
// re-picked already-shipped work as live scope. The cure is to let the landing
// itself carry the closure claim — a line-anchored `Closes-Inbox: <id>` in
// build-report.md, unioned into promoteInbox's committed set under the EXACT
// pre-existing cycle-598 landing gate.
//
// Why a marker and not diff inference: an item's `connects_to` is a HINT, not
// an acceptance predicate, so inferring closure from touched paths would
// consume items an unrelated diff merely brushed past. Over-consumption is
// silent data loss; under-consumption only costs a bookkeeping cycle. Closure
// is therefore an explicit, line-anchored, auditor-checkable assertion.
```

### `go/internal/inboxmover/closesmarker_test.go:3` — above `import (`

```text
// closesmarker_test.go — RED contract for `ClosesInboxIDs`, the builder-authored
// closure marker parser (cycle 1452, inbox item consumption-rides-landing-ship
// weight 0.92).
//
// Why a marker and not diff inference: `connects_to` in an inbox item is a HINT,
// not an acceptance predicate, so inferring closure from touched paths would
// consume items an unrelated diff happened to brush past. Silent over-consumption
// is strictly worse than the under-consumption we have today (data loss vs. a
// wasted bookkeeping cycle), so closure must be an explicit, line-anchored,
// auditor-checkable assertion by the Builder.
//
// The contract this file freezes (doNotModifyTests):
//
//  1. A line whose first non-blank content — after an optional markdown bullet
//     (`-` / `*` / `+`) — is `Closes-Inbox:` (marker matched case-insensitively)
//     contributes its comma-separated ids.
//  2. Ids are trimmed of whitespace and surrounding backticks, and must match
//     `[A-Za-z0-9._-]+`; anything else on the line is dropped, not guessed at.
//  3. Result is deduped, first-seen order preserved; nil when nothing matched.
//  4. NOT line-anchored ⇒ NOT a marker. Prose that merely mentions the marker
//     mid-sentence contributes nothing. This is the anti-false-positive half and
//     the reason a substring/regex-anywhere implementation cannot pass.
```

### `go/internal/inboxmover/consoleroute_claim_test.go:3` — above `import (`

```text
// consoleroute_claim_test.go — RED contract for ADR-0074 I1 enforcement at the
// physical handoff. Claim is the one operation that hands an inbox item to a
// lane (inbox/ → processing/cycle-N/); a console-routed item must be REFUSED
// here even if a triage LLM names it — prompts advise, the mover enforces.
```

### `go/internal/inboxmover/consoleroute_claim_test.go:55` — above `func TestClaim_LaneOverrideClaims(t *testing.T) {`

```text
// route:"lane" override claims normally over a declared directory scope that
// holds protected files (a declared protected FILE binds — F35, pinned in
// lifecycle's claim test).
```

### `go/internal/inboxmover/continuation_lanescope_test.go:3` — above `import (`

```text
// continuation_lanescope_test.go — ADR-0076 slice C, G2 (cycle-1104) resolve
// side. G1's ResolveContinuation reads ONLY this cycle's inbox processing
// claims, so a lane whose scope came from the wave planner (no claim file) can
// never resolve a binding — cycle-1078's preserved snapshot was orphaned for
// exactly that reason. ResolveContinuationForScope adds the second identity
// class: claims FIRST (G1 semantics untouched), then the scope-id-keyed
// registry for the lane's todo ids.
```

### `go/internal/inboxmover/continuation_lanescope_test.go:56` — above `func TestResolveContinuationForScope_FallsBackToLaneScopeRegistry(t *testing.T) {`

```text
// TestResolveContinuationForScope_FallsBackToLaneScopeRegistry — the headline
// AC: a cycle with NO processing claim at all still resolves the binding its
// lane-scope todo id carries. This is the cycle-1078 case, end to end.
```

### `go/internal/inboxmover/continuation_lanescope_test.go:77` — above `func TestResolveContinuationForScope_ClaimWinsOverRegistry(t *testing.T) {`

```text
// TestResolveContinuationForScope_ClaimWinsOverRegistry — G1 is unaffected:
// when a claim carries a stamp, that stamp is returned even though the lane
// scope also has a registry binding. Ordering, not replacement.
//
// The claim is seeded under the lane's OWN id. It previously used a different
// id ("task-a" against scope "scope-a"), which made the assertion pass only
// because the claim path ignored lane scope entirely — the defect that let
// cycle-1536 adopt cycle-1535's continuation, ship its eval file, and destroy
// that lane's landing. Claim ids and lane scope ids are ONE namespace (both are
// inbox item ids: seedClaim writes {"id": …}, lane-scope.json carries todo_ids),
// so a claim outside the lane's scope is a PEER's work, and the out-of-scope
// case is pinned by TestResolveContinuationForScope_DoesNotAdoptAPeerLanesClaim.
// The ordering rule this test exists for is unchanged.
```

### `go/internal/inboxmover/continuation_lanescope_test.go:176` — above `func TestResolveContinuation_ClaimOnlyPathUnchanged(t *testing.T) {`

```text
// TestResolveContinuation_ClaimOnlyPathUnchanged — REGRESSION on G1. The
// original claim-only entry point must keep ignoring the registry entirely, so
// callers that deliberately want claim semantics (and PR #363's behaviour) are
// byte-identical after this extension.
```

### `go/internal/inboxmover/continuation_release.go:3` — above `import (`

```text
// continuation_release.go — the ONE release transaction every retirement path
// shares (cycle-1515). `releaseContinuationOnRetire` already owned the
// preserve-then-delete order for the write side (Promote/quarantine); the
// read-side guard in ResolveContinuationForScope and the new operator surface
// (`evolve continuation release`) both need the SAME transaction, and a second
// copy of it is exactly the drift that produced audit cycle-1507's H2 (the
// read-side delete skipped preservation and sent the salvage pointer to stderr
// only). So the transaction lives here once and the three callers reach it.
```

### `go/internal/inboxmover/continuation_release.go:23` — above `func ReleaseContinuationBinding(opts Options, scopeID, reason, releasedBy string) (continuation.Continuation, bool, erro…`

```text
// ReleaseContinuationBinding releases scopeID's registry binding, preserving the
// released VALUE into the scope's item file FIRST (wherever that item currently
// lives — pending root, a processing claim, or a retirement subtree). Returns
// the released value, whether the registry entry was actually deleted, and any
// registry error.
//
// A scope with no binding is a clean miss (zero, false, nil) — releasing
// nothing is not a failure. The delete is DeleteRegistryEntryIfCycle, so a
// sibling lane that rebound the scope between the read and the delete keeps its
// fresh binding (released=false, no error).
//
// releasedBy names the AUTHORITY the release was made under and is recorded
// beside the reason and the timestamp. Every caller declares it rather than
// inheriting a blank: a binding is the lineage the defect-ledger gate reads as
// anti-tamper evidence, so an erasure that names no actor is itself the defect
// (the gap the cycle-1684 operator-authority gate closes). Runtime lifecycle
// paths name themselves; the operator surface names the authority path that
// unlocked it.
```

### `go/internal/inboxmover/continuation_release.go:84` — above `func retiredAtCycle(path string) int {`

```text
// retiredAtCycle returns the cycle a retired item copy was retired in, or 0 when
// the copy carries no cycle evidence at all.
//
// This is the recency half of the read-side guard's evidence test (audit
// cycle-1507 H1): a retired copy from cycle 900 says NOTHING about a binding
// minted at cycle 1484 — the item was re-filed and rebound after that
// retirement, and treating the old copy as proof of death releases live
// preserved work. Only a retirement that is not older than the binding is
// evidence of a ghost. Unknown (0) is not "stale": the ordinary quarantine copy
// carries no stamp, and refusing to act on the common case would disarm the
// belt entirely.
```

### `go/internal/inboxmover/continuation_release.go:150` — above `func ReconcileConsumedBindings(opts Options) (released []string) {`

```text
// ReconcileConsumedBindings projects inbox/consumed/ into the continuation
// registry, the way the fingerprint reconciler projects it into the ack
// ledger: a binding whose item lives ONLY in consumed/ is definitionally dead
// — leaving it live is the immortal-binding class (cycles 1487/1497; recurred
// as cycle-1558 through the operator consume, which moved the file but not
// the binding, and the next wave minted a zero-delivery lane off it).
//
// Two guards before any release, both inherited from the audit of cycle-1507
// (the read-side guard in ResolveContinuationForScope measured 7 of 91 real
// bindings that a guardless release would have destroyed):
//   - LIVE COPY: a re-filed item the batch loader can still reach owns the
//     binding — skip.
//   - RECENCY: a consumed copy OLDER than the binding is stale evidence (the
//     id was re-filed and rebound after that retirement) — skip, loudly.
//
// The release itself goes through the ONE shared transaction
// (ReleaseContinuationBinding): preserve-then-delete, loud on a failed
// preserve, cycle-guarded delete. Per-item failures WARN and never block.
// Called before every cycle dispatch on the blocker-breaker path — the sweep
// is O(consumed items) with a registry read per bound id; consumed/ is never
// pruned, so revisit the cost if the corpus grows an order of magnitude.
```

### `go/internal/inboxmover/continuation_release_test.go:51` — above `func TestReleaseContinuationBinding(t *testing.T) {`

```text
// TestReleaseContinuationBinding covers the ONE release transaction every
// retirement path now shares (the fix for audit cycle-1507's H2: the read-side
// delete used to skip preservation and send the salvage pointer to stderr only).
```

### `go/internal/inboxmover/continuation_release_test.go:91` — above `for _, want := range []string{'"released_by":"unit-test-authority"', '"released_at":'} {`

```text
// WHO, beside the WHEN and WHY the record already carried: an erasure
// of the lineage the defect-ledger gate reads as anti-tamper evidence
// must name the authority it was made under (cycle-1684).
```

### `go/internal/inboxmover/continuation_release_test.go:146` — above `func TestResolveContinuationForScopeRecency(t *testing.T) {`

```text
// TestResolveContinuationForScopeRecency is the regression test for audit
// cycle-1507's H1. The read-side guard deletes a ROOT-OWNED binding on
// agent-writable evidence, so the evidence has to be recent: a retired copy
// OLDER than the binding means the item was re-filed and rebound after that
// retirement, and releasing on it destroys live preserved work. Same-or-newer
// retirement (and the unstamped ordinary quarantine copy) is still evidence —
// a guard that refuses to act on the common case is no guard at all.
```

### `go/internal/inboxmover/continuation_resolve.go:3` — above `import (`

```text
// continuation_resolve.go — ADR-0076 slice C, resolve side: the orchestrator's
// composition-root lookup for "does this cycle's claimed scope carry preserved
// work to resume?". Reads the cycle's processing claims (the same dir the
// claim/release lifecycle owns) and returns the first stamped continuation in
// deterministic filename order. Validation is the orchestrator's job
// (validateContinuation re-screens against live git state); this is pure
// tolerant lookup — any unreadable item is skipped.
```

### `go/internal/inboxmover/continuation_resolve.go:30` — above `func resolveClaim(opts Options, cycle int, inScope map[string]bool) *continuation.Continuation {`

```text
// resolveClaim walks this cycle's claimed items in sorted order and returns the
// first STAMPED one that inScope admits. inScope nil admits every claim — the
// legacy, scope-blind reading.
//
// The scope filter exists because a cycle's processing dir can hold claims for
// several ids, and a lane may legitimately be working only one of them. Without
// the filter the walk returns whichever stamped claim it meets first, which
// silently hands a lane a PEER LANE's continuation: cycle-1536, scoped to
// "pipeline-defect-infra-systemic", adopted "pipeline-replay-contract-boundary"
// (cycle-1535's task), shipped that lane's eval file, and left cycle-1535 unable
// to rebase — a PASS cycle whose landing was destroyed. Unstamped claims are
// skipped, so the lane's own claim not carrying a stamp is exactly the case that
// let the peer's win.
```

### `go/internal/inboxmover/continuation_resolve.go:98` — above `func ResolveContinuationForScope(opts Options, cycle int, scopeIDs []string) *continuation.Continuation {`

```text
// ResolveContinuationForScope is the composition root's lookup once a cycle's
// scope identity can come from either class (ADR-0076 slice C, G2). Inbox
// claims are tried FIRST — G1's semantics are untouched, and a claim that
// carries a stamp always wins — then the scope-id-keyed registry over scopeIDs
// in the order the lane declares them (deterministic, so a re-run resolves the
// same binding). A claim merely EXISTING never suppresses the fallback; only a
// stamped one does. An entry with no snapshot ref is not resumable work, so it
// is not a binding. A corrupt registry degrades to nil (fresh start) with a
// loud line — the orchestrator must never crash mid-cycle over a salvage index.
```

### `go/internal/inboxmover/continuation_resolve.go:125` — above `retiredPath, retiredIn := scopeRetiredAt(opts, id)`

```text
// Live-scope guard (planner-and-adoption-live-scope-guard). This is
// the ONE seam both the wave planner's lane-scope minting and the
// post-triage adoption path go through, and it used to trust a
// registry hit without asking whether the scope id still names a
// live pending item — so a parked/consumed scope re-armed on every
// wave with no adoption event and no carryover entry (cycles 1487,
// 1497). The belt holds even if a release call is ever missed at a
// pool-exit path; releasing the ghost here stops it re-arming.
//
// Two conditions the audit of cycle-1507 (H1/H2) made mandatory
// before this read path is allowed to DELETE a root-owned binding:
//
//  1. RECENCY. A retired copy older than the binding is not evidence
//     of a ghost — the item was re-filed and rebound AFTER that
//     retirement, and its archived copy sits in consumed/ forever.
//     Releasing on that evidence destroys live preserved work
//     (measured: 7 of 91 real bindings on the runtime plane).
//     Unknown (0) is not stale — the ordinary quarantine copy
//     carries no cycle stamp, and refusing to act on the common case
//     would disarm the belt entirely.
//  2. PRESERVE FIRST. The release goes through the ONE shared
//     transaction (ReleaseContinuationBinding), so the salvage
//     pointer lands in the item file before the delete instead of
//     on stderr only.
```

### `go/internal/inboxmover/continuation_resolve_scope_test.go:3` — above `import (`

```text
// continuation_resolve_scope_test.go — a lane must not adopt another lane's
// continuation.
//
// Live incident (wave-20260822a-verify): cycle-1536's lane-scope.json declared
// todo_ids ["pipeline-defect-infra-systemic"], but its audit report reads
// "Task (continuation-bound): pipeline-replay-contract-boundary (ADR-0076
// continuation of cycle 1532)" — a peer lane's task. It then shipped adcbddb2
// carrying .evolve/evals/pipeline-replay-contract-boundary.md, a file belonging
// to cycle-1535's lane. cycle-1535 could no longer rebase onto main (its own
// eval file was already there under someone else's commit), hit a non-derived
// conflict, routed to the debugger, and lost a PASS cycle's landing entirely.
//
// Cause: ResolveContinuationForScope takes scopeIDs and never applies them to
// the inbox-claim path. ResolveContinuation walks processing/cycle-N/*.json in
// sorted order and returns the FIRST claim carrying a snapshot — skipping
// unstamped ones — so a stamped claim for a DIFFERENT scope wins over the
// lane's own scope entirely.
//
// The claim-first ordering is deliberate (G1) and stays. What changes is that a
// SCOPED lane only adopts claims belonging to its own scope. An unscoped cycle
// (sequential/solo, empty scopeIDs) keeps today's behavior byte-for-byte —
// there is no lane identity to violate there, and narrowing it would break G1.
```

### `go/internal/inboxmover/continuation_retire.go:3` — above `import (`

```text
// continuation_retire.go — the RELEASE half of the continuation-registry
// lifecycle (ADR-0076 slice C, G2). `continuation.DeleteRegistryEntry*` has
// existed since the 2026-08-10 immortal-entries stall, but nothing called it
// when an item LEFT the pending pool, so dispatch had two stores — inbox items
// and scope-keyed registry bindings — and every retirement path touched only
// one. Live burn: cycle-1487 parked `context-fill-telemetry-and-cap` out of
// .evolve/inbox and the next wave dispatched it anyway from cycle-1484's
// binding, burning a third lane on the same deterministic collision.
//
// Two rules, both here so the definitions cannot drift apart:
//
//  1. Retirement releases (releaseContinuationOnRetire) — the binding VALUE is
//     preserved into the retired item file's released_continuations[] first, so
//     the salvage pointer survives the release; the delete is
//     DeleteRegistryEntryIfCycle so a sibling lane that rebound the scope
//     between the read and the release keeps its fresh binding.
//  2. Liveness is the batch loader's own reach (scopeHasLiveItem) — an id is
//     LIVE iff it sits in the inbox ROOT (LoadDir's non-recursive scan) or in
//     processing/cycle-*/ (a lane currently holding it). consumed/, quarantine/,
//     processed/, rejected/ and retry/ are NOT live: LoadDir skips subdirs,
//     which is exactly why a parked item stops being picked.
```

### `go/internal/inboxmover/continuation_retire.go:81` — above `if perr := appendReleasedContinuation(itemPath, continuation.RedactHostPaths(c), reason, retireAuthority, opts.Now().UTC…`

```text
// The preserved pointer rides the ship commit into a TRACKED .evolve/inbox
// item on a public remote, so the absolute host paths are collapsed to "~"
// first (audit cycle-1507 M1). Only Worktree/FindingsPath change; the
// snapshot/base/branch refs salvage actually resumes from are untouched.
```

### `go/internal/inboxmover/continuation_retire.go:142` — above `func scopeRetiredAt(opts Options, scopeID string) (string, string) {`

```text
// scopeRetiredAt returns the path of the retired copy holding scopeID and the
// retirement subtree it sits in, or ("", "") when the id is not retired
// anywhere. The PATH is returned as well as the subtree name because both
// read-side consumers need it: the guard preserves the released pointer into
// that exact file, and the recency test reads its cycle stamp.
//
// The guard refuses on POSITIVE retirement evidence, not on mere absence, and
// the distinction is load-bearing in both directions:
//
//   - A lane scope that names no inbox item at all is NOT proof of retirement —
//     the wave planner also mints lane scopes from carryoverTodos, which never
//     have an inbox file (the cycle-1078 orphan class this registry exists to
//     serve). Treating absence as death would trade the re-dispatch defect for
//     the salvage-loss defect: every carryover lane's preserved work released
//     out from under it.
//   - An id sitting in consumed/, quarantine/, processed/, rejected/ or retry/
//     IS proof: those are the pool exits, and that is the exact cycle-1487/1497
//     shape (item parked, binding immortal, lane minted anyway).
//
// Residual gap, stated rather than papered over: an item whose file was deleted
// outright leaves no evidence for this belt to find. That case is closed on the
// WRITE side by the transactional retire (releaseContinuationOnRetire /
// consume), which is the primary fix; this read-side guard is the belt.
```

### `go/internal/inboxmover/continuation_retire_test.go:3` — above `import (`

```text
// continuation_retire_test.go — durable regression coverage for the two halves
// of park-consume-releases-continuation-binding. The cycle-1507 ACS predicates
// pin the same contract, but they are cycle-scoped and get archived; this is
// the coverage that travels with the package.
```

### `go/internal/inboxmover/continuation_retire_test.go:144` — above `func TestResolveContinuationForScope_GhostScopeRefusedAndReleased(t *testing.T) {`

```text
// TestResolveContinuationForScope_GhostScopeRefusedAndReleased covers the read
// half: a binding whose scope id has no live pending item is refused, logged
// and released — the cycle-1487/1497 re-dispatch-forever shape.
```

### `go/internal/inboxmover/continuation_retire_test.go:185` — above `func TestResolveContinuationForScope_NoItemAnywhereStillAdopts(t *testing.T) {`

```text
// TestResolveContinuationForScope_NoItemAnywhereStillAdopts is THE
// anti-overreach control that shaped the guard: the wave planner also mints
// lane scopes from carryoverTodos, which never have an inbox file at all
// (cycle-1078's orphan class — the reason the scope-keyed registry exists).
// Absence of an item is therefore NOT evidence of retirement; only a copy
// sitting in a pool-exit dir is. A guard that released here would trade the
// re-dispatch defect for a salvage-loss defect.
```

### `go/internal/inboxmover/continuation_stamp_test.go:3` — above `import (`

```text
// continuation_stamp_test.go — ADR-0076 slice C (S3): the FAIL-release stamps
// each released item with the cycle's continuation manifest, TRANSACTIONALLY
// with the release itself (pipeline-forensics lesson: item consumption must be
// transactional with landing — a stamp that happens in a separate pass can be
// lost to a crash between them). No manifest ⇒ byte-identical release.
// Quarantine is terminal parking: a quarantined item sheds any stamp so a
// later operator revival starts fresh.
```

### `go/internal/inboxmover/dispatchstate.go:3` — above `import (`

```text
// dispatchstate.go — dispatch-time task-state resolution for the fleet
// freshness gate (cycle 767, inbox id dispatch-freshness-gate). The gate must
// re-resolve a planned task id against the CURRENT inbox lifecycle immediately
// before lane launch; this package owns the lifecycle dirs, so the resolver
// lives here and the fleet/cmd layers stay lifecycle-layout-agnostic.
```

### `go/internal/inboxmover/dispatchstate.go:31` — above `var retirementStates = []string{StateConsumed, StateQuarantine, StateProcessed, StateRejected, StateRetry}`

```text
// retirementStates are the lifecycle states an item lands in when it LEAVES
// the pending pool, in resolution order (first hit wins when an id sits in
// two dirs). The ONE list for every reader that asks "has this id retired?":
// ResolveDispatchState here and scopeRetiredAt (continuation_retire.go).
// processed/ and rejected/ nest a cycle-<N> level (lifecycle.promoteDestPath);
// every state is scanned flat AND nested so no layout change can hide a
// retired item from one reader but not the other (cycle 1682: a shipped item
// resolved unknown, survived both prunes and the launch probe, and was
// re-pinned to a lane).
```

### `go/internal/inboxmover/dispatchstate.go:45` — above `Detail string`

```text
// e.g. "cycle-748" when State==StateProcessing
```

### `go/internal/inboxmover/dispatchstate.go:47` — above `Path string`

```text
// Path is the LIVE record's absolute path, populated only for
// StatePending. Auto-minted ids are deliberately stable per category (the
// dedup identity), so consumed/ accumulates same-id namesakes forever and
// a bare id is structurally unsafe to hand to an agent: cycle-1548 worked
// an already-cured two-week-old consumed record because its prompt carried
// only the name. The resolver had this path in hand all along; carrying it
// is what lets dispatch disclose the one correct file.
```

### `go/internal/inboxmover/dispatchstate.go:57` — above `func ResolveDispatchState(opts Options, taskID string) DispatchState {`

```text
// ResolveDispatchState classifies taskID against the inbox lifecycle dirs:
// inbox/ → pending (with its declared deps), processing/cycle-N/ → processing
// (Detail names the cycle), processed|rejected|retry|quarantine/ → that state,
// and no evidence anywhere → unknown.
//
// quarantine/ is load-bearing here, not decorative: without it a todo parked by
// the ADR-0072 S5 retry ceiling fell through to StateUnknown, which the
// dispatch freshness gate fails OPEN on — so the ceiling would park a poison
// todo and the very next wave would launch it again. Best-effort reads throughout — an unreadable
// dir or malformed file is treated as no evidence, never an error, so a bad
// inbox can only ever fail OPEN at the dispatch gate.
```

### `go/internal/inboxmover/dispatchstate_path_test.go:3` — above `import (`

```text
// dispatchstate_path_test.go — a pending task's dispatch state must carry the
// LIVE record's path.
//
// cycle-1548 (soak-20260823a halt): the dispatched scope id resolved to 17
// on-disk records — one LIVE in inbox/, sixteen namesakes in consumed/ — and
// the resolver, which had the live path in hand (FindFileByTaskID), DISCARDED
// it. The prompt then carried a bare id, the agent name-searched the tree, and
// every phase report cited a consumed record from a halt CURED two weeks
// earlier. The auto-minted P0 ids are deliberately stable per category (the
// dedup identity), so namesakes accumulate forever: name-based resolution is
// structurally unsafe here, and the resolved PATH is the only safe handle.
```

### `go/internal/inboxmover/dispatchstate_test.go:25` — above `func TestResolveDispatchState(t *testing.T) {`

```text
// TestResolveDispatchState pins the lifecycle classification the fleet
// dispatch freshness gate builds on (cycle 767, dispatch-freshness-gate):
// each lifecycle dir maps to its state, pending carries the declared deps,
// processing names the owning cycle, and no evidence anywhere is StateUnknown
// (the fail-open posture — a bad or absent inbox must never false-skip a lane).
```

### `go/internal/inboxmover/dispatchstate_test.go:56` — above `{"task-poison", StateQuarantine, "", ""},`

```text
// A todo parked by the ADR-0072 S5 retry ceiling must classify as
// quarantine, NOT fall through to StateUnknown: the dispatch freshness
// gate fails OPEN on unknown, so an unclassified quarantine would let the
// very next wave relaunch the poison todo the ceiling just parked.
```

### `go/internal/inboxmover/extra_coverage_test.go:3` — above `import (`

```text
// extra_coverage_test.go — the readTaskIDOrUnknown fallbacks and the
// writeLedger tests moved to the lifecycle leaf with the code (ADR-0103 unit
// 06: lifecycle/item_test.go TestReadTaskIDOrUnknown_Fallbacks,
// lifecycle/ledger_test.go TestLedgerLine_NilLedgerIsSilent_AppendFailureIsTheVerbatimLine).
```

### `go/internal/inboxmover/extra_coverage_test.go:177` — above `ActiveCycleFn: func() (string, error) { return "99", nil },`

```text
// cycle-3 is orphaned
```

### `go/internal/inboxmover/failurecount_read_test.go:3` — above `import (`

```text
// failurecount_read_test.go — ADR-0076 slice D: the exported read path from
// item id → durable failure_count (written by bumpFailureCount on FAIL
// release). Searches the inbox root AND processing/cycle-*/ (a claimed item
// mid-cycle still resolves). Absent item / absent field → (0, false/true)
// per contract below.
```

### `go/internal/inboxmover/inboxmover.go:1` — above `package inboxmover`

```text
// Package inboxmover ports legacy/scripts/utility/inbox-mover.sh.
//
// Atomic inbox lifecycle transitions (v9.6.0+). Three subcommands:
//
//	claim <task_id> <cycle>                     inbox/ → processing/cycle-N/
//	promote <task_id> <new_state> [<cycle>]     processing/ → processed|rejected|retry/
//	  [--commit-sha <sha>]
//	recover-orphans                             processing/cycle-X/ → inbox/ (dead cycles)
//
// All state transitions use a single atomic os.Rename (same-FS). Ledger
// writes are best-effort — failure to write the ledger never blocks a
// lifecycle operation.
//
// Exit codes (cmd layer maps from sentinel errors):
//
//	0 — success (or promote no-op for ship.sh compat)
//	1 — not-found / bad args (claim)
//	2 — mv failed (claim only)
//
// Since ADR-0103 unit 06 this file is the SEAM: the movers, the processed-
// record primitives and the ledger line live in the lifecycle leaf
// (internal/inboxmover/lifecycle); this file owns Options and its resolved
// defaults (the fallback file ledger, the git landing probe, the cycle-state
// reader), the ONE construction of a Mover per call (mover) and the Strangler
// facades every production root, the ship phase and the ACS predicates keep.
// Design: docs/architecture/decomposition/06-inboxmover.md.
```

### `go/internal/inboxmover/inboxmover.go:53` — above `ErrConsoleRouted = lifecycle.ErrConsoleRouted`

```text
// ErrConsoleRouted refuses the lane handoff of an operator-owned item
// (ADR-0074 I1): route:"console-*" or a protected fix surface.
```

### `go/internal/inboxmover/inboxmover.go:101` — above `IsProtectedPath func(path string) bool`

```text
// IsProtectedPath is the control-plane SCOPE predicate for the
// ADR-0074 claim floor (guards.IsProtectedScope at composition roots — F29).
// nil disables only the files-derived rule; an explicit route:"console-*"
// field always refuses the claim.
```

### `go/internal/inboxmover/inboxmover.go:107` — above `Signals *signalcenter.Center`

```text
// Signals is the root's Signal Center the mover's inbox.warning events go
// to (ADR-0103 unit 06). nil — every literal but the FAIL closeout's today
// — is unwired: the leaf prints the legacy [inbox-mover] line instead (the
// two-link producer), so nothing goes silent on a Center-less root.
```

### `go/internal/inboxmover/inboxmover.go:144` — above `func shaLandedOnMain(root, sha string) (bool, error) {`

```text
// shaLandedOnMain reports whether sha is an ancestor of main via
// `git merge-base --is-ancestor <sha> main`. Exit 0 = ancestor (landed),
// exit 1 = cleanly-not-an-ancestor (unlanded). Any other exit (128 = non-git
// dir / unknown rev / no local main) or seam error is fail-open (treated as
// landed) so a non-repo ProjectRoot never blocks a promotion — delivery
// evidence gates, it never manufactures a false negative from missing git —
// and, since unit 06's review fold, RETURNED as the error so the leaf's
// landing gate reports INBOX_LANDED_CHECK_FAILED instead of promoting in
// silence (the hard-coded main is 06-F10's).
```

### `go/internal/inboxmover/inboxmover.go:227` — above `func ShouldQuarantine(failureCount, ceiling int, systemLevelFailure bool) bool {`

```text
// ShouldQuarantine is the pure ADR-0072 S5 decision (see lifecycle.ShouldQuarantine).
```

### `go/internal/inboxmover/inboxmover.go:232` — above `func ReleaseFromQuarantine(opts Options, taskID string) (PromoteResult, error) {`

```text
// ReleaseFromQuarantine is the operator escape hatch for ADR-0072 S5: it moves
// an item out of .evolve/inbox/quarantine/ back to the inbox root and resets
// its failure_count to 0, so the next cycle's triage can re-pick it.
```

### `go/internal/inboxmover/inboxmover.go:254` — above `func ReleaseCycleProcessingWithReason(opts Options, cycle int, reason string) (RecoverResult, error) {`

```text
// ReleaseCycleProcessingWithReason is ReleaseCycleProcessing with an explicit
// ledger reason for each released item. An empty reason keeps the generic
// "cycle-release". Callers that drain because delivery failed (e.g. an
// unlanded ship commit, cycle-598 shape) pass a reason carrying "unlanded" so
// the ledger durably distinguishes a delivery-failure retry from an ordinary
// residual drain (inbox-promotion-requires-landed-ship).
```

### `go/internal/inboxmover/inboxmover.go:264` — above `type quarantinePolicy = lifecycle.Policy`

```text
// quarantinePolicy is the drain's historical spelling of the leaf's Policy —
// the ADR-0072 S5 decision inputs (ceiling, system-level, the committed set
// with its nil-means-whole-drain contract) are documented ONCE, on
// lifecycle.Policy; an alias keeps outcome.go's literal and this file's
// signature on that one struct instead of a hand-projected mirror.
```

### `go/internal/inboxmover/inboxmover.go:271` — above `func releaseCycleProcessing(opts Options, cycle int, reason string, quar *quarantinePolicy) (RecoverResult, error) {`

```text
// releaseCycleProcessing is the shared drain core: the plain release-to-root
// when quar is nil, the ADR-0072 S5 failure drain (bump, quarantine at the
// ceiling, fail-open) when it is not. It stays UNEXPORTED on purpose (audit
// D3): ApplyCycleOutcome is the one public door into the cycle-outcome
// lifecycle, so the PASS-promote and FAIL-bump halves cannot drift apart
// behind a second entry point (never_duplicate_centralize; the leaf's
// Mover.Release is reachable only through this file — TestLifecycle_OnlyHostImportsTheLeaf).
```

### `go/internal/inboxmover/inboxmover.go:312` — above `func SupersededInboxIDs(triageDecisionJSON []byte) []string {`

```text
// SupersededInboxIDs extracts the top-level "superseded" string array from a
// triage-decision.json body: deduped, order-preserving. Returns nil on an
// absent field or invalid JSON — never panics.
//
// This is the data-driven declaration that feeds ReconcileSuperseded at ship,
// replacing the prose-only "verify vs HEAD, move to consumed" carryover
// instruction that silently lapsed for cycles 544..548. It names inbox items
// whose underlying work already shipped under a DIFFERENT id (e.g. cycle 544
// shipped the fleet-starvation observer as "recover-ship-fleet-starvation-
// observer", stranding its originating request "loop-self-prioritize-unmet-
// fleet-concurrency" in the inbox root).
```

### `go/internal/inboxmover/inboxmover.go:377` — above `func RouteConsole(opts Options, taskID, reason string, cycle int) (RouteResult, error) {`

```text
// RouteConsole is the FAIL closeout's per-item breaker: the item is rewritten
// in place — wherever the lane's claim left it — with route:console-manual
// and the refusal as routed_reason, so the ADR-0074 claim floor refuses every
// later lane (docs/incidents/2026-09-14-triage-refusal-poison-loop.md).
```

### `go/internal/inboxmover/inboxmover_amplified_test.go:3` — above `import (`

```text
// inboxmover_amplified_test.go — cycle-308 adversarial amplification tests
// for ReleaseCycleProcessing (inbox-promote-on-ship-missing).
//
// Targets gaps in the TDD contract: missing dirs, empty dirs, nil Stderr
// (panic guard), mixed-outcome batches, and double-move not counted as recovered.
```

### `go/internal/inboxmover/inboxmover_quarantine_test.go:10` — above `func failDrain(opts Options, cycle, ceiling int, systemLevel bool) (OutcomeResult, error) {`

```text
// failDrain runs the ADR-0072 S5 failure drain over processing/cycle-<cycle>/
// through the ONE public lifecycle door, ApplyCycleOutcome. Audit D3 retired
// the ReleaseCycleProcessingWithQuarantine wrapper these tests used to call: it
// had no production caller, so it was a second entry point into a lifecycle
// ApplyCycleOutcome owns. Leaving CommittedIDs nil selects the same whole-dir
// bump the wrapper performed, so these assertions cover the identical path.
```

### `go/internal/inboxmover/inboxmover_quarantine_test.go:93` — above `if err := os.MkdirAll(filepath.Join(inbox, "processing", "cycle-6"), 0o755); err != nil {`

```text
// Re-claim for the next cycle: move root item into processing/cycle-6/.
```

### `go/internal/inboxmover/inboxmover_release_test.go:3` — above `import (`

```text
// inboxmover_release_test.go — RED tests for cycle-308 task
// `inbox-promote-on-ship-missing` (inbox item 2026-06-12T17-08-02Z).
//
// The gap: items claimed into processing/cycle-<N>/ are stranded forever when
// a cycle FAILs (RecoverOrphans is never auto-called) or when a claimed item is
// dropped from triage's top_n on a SUCCESSFUL ship (promoteInbox only promotes
// top_n). processing/ currently holds orphans from cycles 124/234/240/243/248/
// 265/294/295.
//
// New API this file pins (Builder implements in inboxmover.go):
//
//	ReleaseCycleProcessing(opts Options, cycle int) (RecoverResult, error)
//
// — a SCOPED, idempotent release of processing/cycle-<cycle>/*.json back to the
// inbox root. Scoped means it touches ONLY the named cycle's dir (never a
// concurrent batch's other cycles). A pre-existing inbox-root file with the same
// basename (double-move race) is a WARN, not an error, and must NOT clobber the
// existing file. Helpers makeRepo/dropProcessingFile/dropInboxFile/setCycleState
// live in inboxmover_test.go (same package).
```

### `go/internal/inboxmover/inboxmover_release_test.go:50` — above `if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-7", "a.json")); err == nil {`

```text
// cycle-7 dir drained.
```

### `go/internal/inboxmover/inboxmover_release_test.go:54` — above `if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-9", "b.json")); err != nil {`

```text
// cycle-9 untouched — release MUST be scoped to the named cycle only.
```

### `go/internal/inboxmover/inboxmover_release_test.go:82` — above `func TestInboxRelease_FailedCycleReleasesAllClaimed(t *testing.T) {`

```text
// TestInboxRelease_FailedCycleReleasesAllClaimed is the cycle-fail terminal
// scenario: every item claimed under processing/cycle-N/ returns to inbox root so
// the next batch re-triages them — no permanent strand (the cycle-124/234/240/...
// orphan class). Behavioral: asserts on the real filesystem side effects.
```

### `go/internal/inboxmover/inboxmover_release_test.go:104` — above `left, _ := os.ReadDir(filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-12"))`

```text
// processing/cycle-12 fully drained.
```

### `go/internal/inboxmover/inboxmover_test.go:183` — above `func TestPromote_ProcessedRefusedWhenNotLanded(t *testing.T) {`

```text
// === Promote: ancestry-gated delivery evidence (cycle-598 incident) =======
//
// Root cause (inbox item inbox-promotion-requires-landed-ship): Promote used
// to key promotion on the caller-supplied newState alone (a proxy for "cycle
// verdict PASS"), never checking whether the ship commit actually landed on
// main. A push-rejected ship whose recovery path still reported PASS could
// promote to processed/ with a commit that git log --all never contained,
// silently dropping the directive while reporting it done.
//
// The fix under test: Options gains an IsLandedFn seam (nil defaults to a
// real `git merge-base --is-ancestor <sha> main` check against ProjectRoot,
// fail-open on a seam/exec error so a non-git ProjectRoot — as used by every
// pre-existing Promote test above — never regresses). When newState ==
// "processed" and PromoteOpts.CommitSHA is set, Promote consults IsLandedFn:
// landed=false reroutes the item to retry/ (never processed/) and the ledger
// records why; landed=true promotes normally.
```

### `go/internal/inboxmover/inboxmover_test.go:336` — above `if _, err := os.Stat(filepath.Join(repo, ".evolve", "inbox", "processing", "cycle-5", "task-c.json")); err != nil {`

```text
// task-c should still be in processing/cycle-5/.
```

### `go/internal/inboxmover/inboxmover_test.go:428` — above `func TestReadActiveCycle_MissingFile(t *testing.T) {`

```text
// === promoteDestPath / intPtr / strPtr — moved to the lifecycle leaf with the
// code (ADR-0103 unit 06: lifecycle/promote_test.go TestPromoteDestPath_AndCycleOrZero,
// lifecycle/ledger_test.go TestIntPtr_StrPtr_FoldLifecycleMessage).
```

### `go/internal/inboxmover/ledger_chained_test.go:3` — above `import (`

```text
// ledger_chained_test.go — the fleet-concurrency chain-break generator
// (console-plane forensics 2026-08-11, item ledger-fleet-concurrency-chain):
// writeLedger raw-O_APPEND'ed unchained NDJSON (no prev_hash, no flock, no
// tip update) into the hash-chained ledger.jsonl on essentially every cycle's
// ship.postship — each such line breaks the walk at that point, and it also
// defeated the Rebaseline seal (which must bind the physical predecessor).
// Every inbox-lifecycle record now goes through the chained append path: same
// flock, same prev_hash/entry_seq, same atomic tip replace.
```

### `go/internal/inboxmover/lifecycle_golden_test.go:3` — above `import (`

```text
// lifecycle_golden_test.go — ADR-0103 unit 06 step 0: characterization goldens
// captured on the pre-extraction code (8e8f080f) and replayed byte-for-byte
// through the lifecycle leaf's facades. Roots and the pid are templated
// ({{ROOT}}, {{PID}}); the chained ledger's prev_hash (which covers absolute
// paths) is templated to {{HASH}}. No golden is regenerated by a flag: a drift
// is a declared deviation with its own commit, never a silent re-capture.
```

### `go/internal/inboxmover/lifecycle_golden_test.go:274` — above `writeItemAt(t, proc(13, "t9.json"), '{"id":"t9"}')`

```text
// :582 / :595 / :598 / :611 — the active cycle skipped, a root directory
// twin fails the move, a clean orphan recovers (and cycle-8's t3 clobbers
// its root twin — the preserved quirk).
```

### `go/internal/inboxmover/lifecycle_pins_test.go:3` — above `import (`

```text
// lifecycle_pins_test.go — ADR-0103 unit 06 step 0: the order invariants and
// preserved quirks of the lifecycle movers, pinned on the pre-extraction code
// (green on 8e8f080f, each proven red by its named mutant) and kept green
// through the leaf's facades. Every fixture that needs a filesystem fault uses
// a FILE where a directory is expected or a DIRECTORY where a file is expected
// (root-proof); the one chmod fixture asserts the fault actually happened.
```

### `go/internal/inboxmover/lifecycle_seam_test.go:3` — above `import (`

```text
// lifecycle_seam_test.go — ADR-0103 unit 06 step 3: the host seam. Options is a
// value copied per call, so the ONE projection onto the leaf is (Options).mover
// and the ONE construction site is pinned by a source scan; every seam is
// threaded; every production and ACS spelling is kept; the wired-root golden
// renders the fifteen replaced lines through the root's StderrSink.
```

### `go/internal/inboxmover/lifecycle_seam_test.go:221` — above `func TestApplyCycleOutcome_FailDrain_EmitsInboxCodesThroughOptionsSignals(t *testing.T) {`

```text
// Test 50 — the FAIL drain through Options.Signals: the quarantine failure
// renders in the Center (origin Mover.Release, the cycle), the fallback line
// does not print, the INFO lines still do, and the FAIL closeout's lane-scope
// pass leaves an already-claimed id alone — no INBOX_CLAIM_NOT_FOUND, no
// console duplicate (the false not-found of the 2026-09-14 poison-loop
// incident; 06-F7/06-F11 retired with it).
```

### `go/internal/inboxmover/outcome.go:3` — above `import (`

```text
// outcome.go — the SINGLE cycle-outcome lifecycle seam.
//
// Before this file the inbox lifecycle had two half-implementations and a hole
// in the middle:
//
//   - PASS side: promotion was agent-driven prose, so cycle-1147 shipped three
//     menu items in ONE commit and promoted none of them — processed/cycle-1147/
//     was empty and all three re-entered the very next triage
//     (menu-pass-promotes-committed-ids).
//   - FAIL side: the drain that bumps failure_count walks ONLY
//     processing/cycle-N/. Nothing ever put a wave lane's worked ids there, so
//     the ADR-0072 S5 retry ceiling was structurally unreachable for fleet work
//     — batch-14 burned four FAILs on the same items with failure_count never
//     leaving 0 (wave-lane-task-quarantine-dead).
//
// ApplyCycleOutcome is the one entry point both closeout paths now call, so the
// PASS-promote and FAIL-bump halves cannot drift apart again
// (never_duplicate_centralize).
```

### `go/internal/inboxmover/outcome.go:42` — above `SystemLevel  bool`

```text
// ADR-0072 S3 system failure: NEVER quarantines (AC4)
```

### `go/internal/inboxmover/outcome.go:135` — above `func ClaimLaneScope(opts Options, cycle int, ids []string) ([]string, error) {`

```text
// ClaimLaneScope moves each resolvable id from the inbox root into
// processing/cycle-<cycle>/ and returns the ids actually claimed. An id it
// cannot resolve — absent, already claimed by another wave, or console-routed
// (ADR-0074) — is logged and skipped: a partial claim must never abort a lane.
// The error return is reserved for a future whole-operation failure and is
// currently always nil, so callers can wire it without a behavior change.
//
// Placement note (deliberate, load-bearing): this is called from
// ApplyCycleOutcome's FAIL path rather than at wave dispatch. Triage builds its
// menu from inboxbatch.LoadDir on the inbox ROOT only (triage.go:113), so
// claiming a lane's scope BEFORE triage runs would hand triage an empty inbox
// and starve the very cycle the claim exists to track. Claiming at outcome time
// puts the worked ids in processing/cycle-N/ exactly when the drain needs them
// there, with no starvation window.
```

### `go/internal/inboxmover/outcome.go:154` — above `if loc, lerr := lifecycle.Locate(opts.InboxDir, id); lerr == nil && loc.Cycle == cycle {`

```text
// The lane's own claim already put it in processing/cycle-N/: it is where
// the drain needs it, and re-claiming it from the root would only raise a
// false INBOX_CLAIM_NOT_FOUND (cycle 1675, the 2026-09-14 poison-loop incident).
```

### `go/internal/inboxmover/outcome.go:228` — above `func ClosedDroppedIDs(body []byte) []string {`

```text
// ClosedDroppedIDs returns the ids triage dropped WITH a close-class reason —
// an affirmative statement the work is landed or the item is dead. The
// carryover twin is retired reason-blind (carryover.Lifecycle.RetireTriageDropped, a
// soft 20-slot advisory store); the durable tracked queue gets the stricter
// reason gate. Parses the "id" key only, matching the core sibling reader.
// SIBLING READER: triageDroppedIDs (internal/core/carryover/workspace.go, ADR-0103 unit 03)
// parses the SAME dropped[] field for the carryover twin — kept apart only by
// the inboxmover→adapters/ledger→core import cycle. A schema change to
// dropped[] must land in BOTH readers or consumption and carryover retirement
// drift apart on the same document.
```

### `go/internal/inboxmover/outcome_test.go:3` — above `import (`

```text
// outcome_test.go — unit coverage for the cycle-outcome seam's readers and
// result surface. The end-to-end filesystem lifecycle (PASS-promote,
// FAIL-bump, quarantine-at-ceiling, menu semantics) is pinned by the cycle-1156
// ACS predicates; these tests cover the parsing and reporting edges those
// predicates deliberately do not assert on.
```

### `go/internal/inboxmover/reconcile_consumed_bindings_test.go:39` — above `write("refiled.json", '{"id":"refiled","consumed":{"cycle":5}}')`

```text
// recency: consumed at cycle 5, rebound at cycle 9 → kept
```

### `go/internal/inboxmover/release_reason_test.go:1` — above `package inboxmover`

```text
// release_reason_test.go — pins ReleaseCycleProcessingWithReason (cycle-752,
// inbox-promotion-requires-landed-ship): an explicit reason lands verbatim in
// the ledger entry for each released item; an empty reason keeps the generic
// "cycle-release" default (byte-compatible with the pre-existing wrapper).
```

### `go/internal/inboxmover/retirementstates_test.go:5` — above `func TestRetirementStates_CoverEveryRetiredStateOnce(t *testing.T) {`

```text
// retirementStates is the ONE list behind every "has this id retired?" reader;
// a State* constant added without a row here is the cycle-1682 class again.
```

### `go/internal/inboxmover/root_failure.go:3` — above `import (`

```text
// root_failure.go — ADR-0080 P2: FAIL-side attempt accounting for
// ROOT-RESIDENT items. The ADR-0072 S5 bump+quarantine lives on the
// processing/-release path, which wave lanes never enter (items stay in the
// inbox root; only PASS promotes them) — so graded audit FAILs incremented
// nothing and the task-retry ceiling was structurally unreachable:
// workspace-hygiene burned 12 lanes, quarantine-dead 7, failure_count 0.
// RecordRootTaskFailure is the root-resident twin, called by the loop after
// a task-level cycle FAIL for each triage-COMMITTED id (menu semantics: an
// unworked menu id never bumps).
//
// CONCURRENCY (review HIGH): the root is SHARED — two lanes that committed
// the same id can FAIL concurrently, and an unserialized read-modify-write
// both loses attempts and can rename a stale copy back into the root after
// the other lane quarantined it (resurrection — the ceiling defeated in the
// exact contended case it exists for). All bumps therefore serialize on one
// inbox-level flock, and the item is re-resolved INSIDE the lock.
```

### `go/internal/inboxmover/root_failure_test.go:3` — above `import (`

```text
// root_failure_test.go — ADR-0080 P2: FAIL-side attempt accounting for
// ROOT-RESIDENT items. Wave lanes never claim into processing/, so the
// existing release-path bump+quarantine (ADR-0072 S5) is structurally
// unreachable for graded FAILs: workspace-hygiene burned 12 lanes and
// quarantine-dead 7 with failure_count still 0. RecordRootTaskFailure is the
// root-resident twin: bump the durable counter where the item actually
// lives; at the ceiling, move it to the terminal quarantine/ dir.
```
