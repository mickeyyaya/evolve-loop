# Comment history: `internal/inboxmover/lifecycle`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/inboxmover/lifecycle/claim_test.go:34` — above `func TestMover_Claim_Refused_EmitsClaimRefused(t *testing.T) {`

```text
// Test 23 — the ADR-0074 claim floor through the leaf: route:console-* refuses
// with a nil predicate; a protected fix surface refuses only when a predicate
// is injected; route:lane on an operator-authored item claims a declared
// directory scope but never a declared protected FILE (F35).
```

### `go/internal/inboxmover/lifecycle/hostonly_test.go:3` — above `import (`

```text
// hostonly_test.go — ADR-0079 D3 preserved by a pin (§6 test 44): Mover.Release
// is exported here (the compiler cannot hide it from the host), so the ONE
// public door into the cycle-outcome lifecycle — inboxmover.ApplyCycleOutcome —
// stays the only door by keeping internal/inboxmover the leaf's only importer.
```

### `go/internal/inboxmover/lifecycle/importgraph_test.go:3` — above `import (`

```text
// importgraph_test.go — the package is a leaf under inboxmover (ADR-0103 unit
// 06 §2): stdlib plus the four named internal packages — the ledger adapter
// (the LifecycleRecord type), continuation (the manifest reader), inboxbatch
// (the claim layout and the routing classifier) and the Signal Center — never
// gitexec, core, verifylock, guards or the host (the failurelearning/
// importgraph_test.go idiom).
```

### `go/internal/inboxmover/lifecycle/item.go:152` — above `func BumpFailureCount(path, reason string) (int, error) {`

```text
// BumpFailureCount increments the durable "failure_count" on an inbox item
// (the single source of truth for ADR-0072 S5 task-level failure memory) and
// stamps the latest failure reason, preserving every other field. Returns the
// new count. Atomic (write-tmp + rename) so a crash never leaves a
// half-written item. Any parse/IO error is returned so the caller can fail
// open. It never sheds the continuation stamp (the drain's bumpWith does).
```

### `go/internal/inboxmover/lifecycle/item.go:162` — above `func bumpWith(path, reason string, shedAt func(count int) bool) (int, error) {`

```text
// bumpWith is the ONE atomic rewrite of the failure counter: the count and
// the reason land in one write, and when shedAt reports the new count reached
// the ceiling the item's continuation stamp is shed in the SAME bytes —
// quarantine is terminal parking, so an operator revival starts fresh
// (ADR-0076 slice C). One rename instead of the two the old bump-then-shed
// performed; identical final bytes.
```

### `go/internal/inboxmover/lifecycle/lifecycle.go:1` — above `package lifecycle`

```text
// Package lifecycle is unit 06 of the component breakdown (ADR-0103): the
// inbox lifecycle mover. One Mover owns the five transitions over ONE inbox
// dir — Claim (inbox/ → processing/cycle-N/), Promote (→ processed | rejected
// | retry | quarantine), ReleaseFromQuarantine, the cycle drain Release (with
// the ADR-0072 S5 failure_count bump and quarantine park) and RecoverOrphans —
// plus the processed-record primitives (the one id→file resolver, the one
// atomic item rewrite, the failure counter's one reader and one writer) and
// the chained inbox-lifecycle ledger line. Every collaborator is injected at
// construction with a Null-Object default: the ledger appender, stderr, the
// clock, the active-cycle reader, the landing probe, the protected-path
// predicate, the continuation retire hook, the run-workspace spelling and the
// Signal Center accessor. The host (internal/inboxmover) resolves the
// production defaults, builds the Mover once per call and keeps every caller's
// spelling behind its facades. The leaf never imports gitexec, core, the
// guards or the host. Failure modes report as inbox.warning under module
// inbox through ONE producer with two links: the Center when a root wired one,
// else the byte-identical legacy `[inbox-mover] WARN:|ERROR:` line onto the
// injected stderr — nothing goes silent on a Center-less root.
// Design: docs/architecture/decomposition/06-inboxmover.md.
```

### `go/internal/inboxmover/lifecycle/lifecycle.go:33` — above `const (`

```text
// The unit's codes — fourteen WARN conditions (twelve at the unit, two from the 2026-09-14 poison-loop breaker) that replaced fifteen hand-written
// stderr lines and gave three silent arms a voice — registered with their docs.
```

### `go/internal/inboxmover/lifecycle/lifecycle.go:82` — above `ErrConsoleRouted = errors.New("inboxmover: item is console-routed (operator-owned) — refusing lane claim")`

```text
// ErrConsoleRouted refuses the lane handoff of an operator-owned item
// (ADR-0074 I1): route:"console-*" or a protected fix surface. Prompts
// advise; Claim enforces — a triage LLM naming the item cannot move it.
```

### `go/internal/inboxmover/lifecycle/lifecycle.go:88` — above `var validStates = map[string]bool{`

```text
// validStates is the set of allowed promote targets. "quarantine" is the
// ADR-0072 S5 terminal state: a task that has failed task_retry_ceiling times
// routes here (a sibling dir the triage scanner never walks) instead of being
// released back to the inbox root every cycle, so a poison todo stops being
// re-picked forever.
```

### `go/internal/inboxmover/lifecycle/lifecycle.go:185` — above `func WithProtectedPath(fn func(path string) bool) Option {`

```text
// WithProtectedPath installs the control-plane scope predicate of the
// ADR-0074 claim floor; nil keeps what is installed (the default nil disables
// only the files-derived rule — an explicit route:"console-*" field always
// refuses).
```

### `go/internal/inboxmover/lifecycle/limits_test.go:3` — above `import (`

```text
// limits_test.go — the clean-code limits the design promises (ADR-0103 unit
// 06 §4), enforced by a test rather than by review: every function < 50
// lines, nesting depth ≤ 4, every file < 800 lines (the failurelearning/
// signalcenter limits_test.go idiom; comments inside a function count, its doc
// comment does not). Promote (113 lines), the drain (114) and RecoverOrphans
// (52) were over the bar before the split.
```

### `go/internal/inboxmover/lifecycle/mover_test.go:3` — above `import (`

```text
// mover_test.go — the Mover's construction, its Null-Object defaults and the
// ONE producer's two links (ADR-0103 unit 06 §6 tests 19-21).
```

### `go/internal/inboxmover/lifecycle/promote.go:69` — above `if reroutedUnlanded {`

```text
// Transactional retire (park-consume-releases-continuation-binding): every
// Promote destination is OUT of the batch loader's reach, so the item's
// registry binding must go with it in this same operation (cycle-1487).
// The reason override lands BEFORE the hook so the preserved pointer and
// the ledger entry describe the same transaction with the same word
// (audit cycle-1507 L1).
```

### `go/internal/inboxmover/lifecycle/quarantine.go:3` — above `import (`

```text
// quarantine.go — the ADR-0072 S5 decision, the drain's park policy and the
// operator's release out of quarantine/ (inboxmover.go:432-487, :636-651).
```

### `go/internal/inboxmover/lifecycle/quarantine.go:13` — above `func ShouldQuarantine(failureCount, ceiling int, systemLevelFailure bool) bool {`

```text
// ShouldQuarantine is the pure ADR-0072 S5 decision: quarantine a task once its
// task-level failure count reaches the configured ceiling. A zero (or negative)
// ceiling disables quarantine entirely, and a system-level failure NEVER
// quarantines — the S3 floor halt takes precedence (AC4). The caller passes the
// ceiling (FailureThresholds.TaskRetryCeiling, default 2) and the system-level
// flag; the leaf deliberately does not import internal/policy so the package
// layering stays intact.
```

### `go/internal/inboxmover/lifecycle/quarantine.go:24` — above `type Policy struct {`

```text
// Policy carries the ADR-0072 S5 decision inputs for a failure drain: the
// task-level retry ceiling and whether this cycle's failure was system-level
// (an S3 floor halt), which suppresses the bump and the quarantine (AC4).
// Committed restricts the failure_count bump (and therefore quarantine) to the
// ids triage actually COMMITTED to the cycle; nil means "every item in the
// drain" — the legacy whole-dir behavior an outcome with no committed ids
// still selects (wave lanes claim a whole menu but work only the committed
// subset). A nil *Policy is the plain release-to-root drain.
```

### `go/internal/inboxmover/lifecycle/quarantine.go:43` — above `func (m *Mover) ReleaseFromQuarantine(taskID string) (PromoteResult, error) {`

```text
// ReleaseFromQuarantine is the operator escape hatch for ADR-0072 S5: it moves
// an item out of quarantine/ back to the inbox root and resets its
// failure_count to 0, so the next cycle's triage can re-pick it. Returns
// ErrNotFound when no quarantined item carries taskID. Idempotent-safe: a
// basename already present at the inbox root is left untouched (never
// clobbered) and reported as ErrMvFailed. The counter reset precedes the
// rename (a preserved quirk: a failed rename leaves a zeroed quarantined item)
// and is best-effort — a rewrite failure never blocks the release, but since
// the unit it is reported.
```

### `go/internal/inboxmover/lifecycle/release.go:3` — above `import (`

```text
// release.go — the cycle drain (inboxmover.go:653-785 on the base): every
// *.json under processing/cycle-<cycle>/ back to the inbox root, with the
// ADR-0072 S5 bump-and-park when a Policy is given and the ADR-0076 slice-C
// continuation stamp when the cycle's workspace carries a manifest. Split
// into the dir open, the stamp read, the park and the single release.
```

### `go/internal/inboxmover/lifecycle/release.go:82` — above `func (m *Mover) readStamp(cycle int) *continuation.Continuation {`

```text
// readStamp reads the cycle's continuation manifest (ADR-0076 slice C): when
// the FAILed cycle preserved salvageable work, every released item carries
// the stamp IN the release pass (transactional). Missing manifest ⇒ no-op; a
// corrupt one is reported and the items release unstamped.
```

### `go/internal/inboxmover/lifecycle/release.go:104` — above `func (m *Mover) parkAtCeiling(d drained, reason string, cycle int, q *Policy) (bool, string) {`

```text
// parkAtCeiling is the ADR-0072 S5 half of the drain: bump the committed
// item's durable failure_count (shedding its continuation stamp in the same
// atomic rewrite once the ceiling is reached — quarantine is terminal parking)
// and park it in quarantine/ at the ceiling. systemLevel gates the BUMP, not
// just the decision (AC4 in full). Every fault falls open to the plain release
// and says so: a bump that cannot rewrite, a park that cannot deliver — the
// un-parked poison item returns to the root and WILL be re-picked.
```

### `go/internal/inboxmover/lifecycle/release_test.go:51` — above `func TestMover_Release_QuarantineAtCeiling_ReplaysG2(t *testing.T) {`

```text
// Test 32 — the ADR-0072 S5 park at the ceiling replays the FAIL-drain
// golden's shape: count 2, the reason, the continuation shed, the promote
// ledger line; a Policy whose Committed excludes the id releases it un-bumped;
// SystemLevel bumps nothing.
```

### `go/internal/inboxmover/lifecycle/route.go:3` — above `import (`

```text
// route.go — RouteConsole: the FAIL closeout's per-item breaker for a
// deterministic refusal. A triage gate that refuses a top_n card because it
// names a protected surface has said, in so many words, that the item is
// operator-owned; retrying it lane after lane only burns the scout and triage
// tokens again (nine cycles for one item, 1650–1675 — the incident doc). The
// item is rewritten IN PLACE, wherever the lane's claim left it, so the drain
// that follows releases it already carrying route:console-manual and the
// ADR-0074 claim floor refuses every later lane.
```

### `go/internal/inboxmover/lifecycle/route_test.go:3` — above `import (`

```text
// route_test.go — RouteConsole's contract through the leaf: the FAIL closeout's
// per-item breaker for a deterministic triage refusal (a top_n card naming a
// protected surface). The item is rewritten IN PLACE — at the inbox root or
// inside processing/cycle-N/ where the lane's claim left it — with
// route:console-manual and the refusal as routed_reason, so the drain
// releases it already operator-owned and the ADR-0074 claim floor refuses
// every later lane. Incident: docs/incidents/2026-09-14-triage-refusal-poison-loop.md.
```

### `go/internal/inboxmover/lifecycle/route_test.go:147` — above `func TestMover_RouteConsole_ThenClaimIsRefused(t *testing.T) {`

```text
// Test 29 — the breaker closes: once routed, the very next lane claim is
// refused by the ADR-0074 floor (INBOX_CLAIM_REFUSED, reason
// route:console-manual) and the item stays at the root.
```
