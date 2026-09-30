# Comment history: `internal/faillearn`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/faillearn/faillearn.go:68` — above `Defects        []string 'yaml:"defects,omitempty"'`

```text
// Defects carries the failed phase's self-reported defect list (ADR-0039
// §7) — real failure content for KB recall, not just the summary string.
// omitempty keeps supervisor-synthesized lessons byte-identical when the
// defect list is just the summary (see RenderLessonYAML).
```

### `go/internal/faillearn/faillearn.go:139` — above `func StructuredDefects(ev FailureEvent) []string {`

```text
// StructuredDefects returns the defect list when it carries REAL content
// (ADR-0039 §7 self-reported defects) — a list that merely echoes the
// synthesized summary adds nothing over Description, so it is omitted
// (keeps pre-v2 lessons byte-identical via omitempty).
//
// Exported because the inbox remediation path (core.writeDeterministicLearning)
// must apply the SAME rule: two definitions of "is this a real defect?" drift,
// and the lesson and the queue would then disagree about what happened.
```

### `go/internal/faillearn/faillearn_test.go:19` — above `func fixtureEvent() FailureEvent {`

```text
// fixtureEvent is the canonical cycle-243 reproduction (retro bridge
// exit=81) used across render tests. Fixed Now keeps output byte-stable.
```

### `go/internal/faillearn/inbox.go:12` — above `type InboxItem struct {`

```text
// inbox.go — the remediation half of the failure floor
// (batch-integrity-review-2026-08-04.md F1(ii)).
//
// Cycle-1255's retrospective filed two remediation items that existed ONLY
// inside retrospective-report.md: the loop's own queue never saw them, so the
// defects were laundered away by later continuations. The floor already
// guaranteed the retrospective survives; it guaranteed nothing about the WORK
// the retrospective asks for. WithInbox closes that gap in the same call, so
// "the retro was written" can no longer be true while "the remediation was
// queued" is false.
```

### `go/internal/faillearn/inbox.go:23` — above `type InboxItem struct {`

```text
// InboxItem is one retro-derived remediation todo destined for `.evolve/inbox`.
//
// The JSON tags mirror inboxbatch.Item's wire shape (id/title/weight/kind/
// priority/files/injected_by) because inboxbatch is the CONSUMER and binds by
// tag. faillearn is a leaf package (stdlib + yaml.v3), so it cannot import
// inboxbatch to borrow the type — parity is by tag and is asserted on the raw
// JSON keys in inbox_transactional_test.go. Renaming a tag here silently
// produces items the loader drops (the cycle-1190 Class-field shape).
//
// InjectedBy is deliberately non-empty for every item this package writes:
// inboxbatch.ConsoleRouted treats an empty InjectedBy as operator-authored and
// honors a route:"lane" override from it. An autofiled item must never inherit
// that authority.
```

### `go/internal/faillearn/inbox.go:92` — above `if !skipped {`

```text
// cycle-1282 DEF-4: ids are deterministic (`retro-<cycle>-<slug>` over
// agent-authored defect text), so a concurrent fleet lane or stale state
// from an earlier run of the same cycle number can already hold the
// filename. writeIfAbsent used to return nil there and WriteArtifacts
// still reported success — the real remediation item was DROPPED with no
// error, no diagnostic, no telemetry, reproducing the very 1255 state
// this package closes. A skip is now only tolerated when the file on
// disk carries the SAME item: that is an idempotent retry. Different
// content under our id is an id collision, and dropping our item to
// honor theirs is not a decision this floor gets to make silently.
```

### `go/internal/faillearn/inbox.go:136` — above `func (c writeConfig) unqueuedItems() []InboxItem {`

```text
// unqueuedItems returns the configured items that are NOT in the queue, read
// back from disk rather than inferred from where the write stopped.
//
// cycle-1290 D2: writeInboxItems is not atomic across items — it writes one file
// per item and returns on the FIRST failure, so every item before the failing one
// is already queued. preserveDiagnosis had only c.inboxItems to work from and so
// listed ALL of them as "still UNQUEUED", overclaiming which remediation was lost
// and sending the operator (or the next continuation) looking for work that is
// already filed.
//
// Reading the inbox back — rather than returning the failure index from
// writeInboxItems and slicing — is deliberate: the artifact's claim is about what
// is in the queue, so the queue is what it should be checked against. That also
// keeps it right for the states an index cannot describe: an item written by a
// CONCURRENT fleet lane under the same deterministic id (queued, though this call
// did not write it), and an id collision where a DIFFERENT item occupies our name
// (unqueued, though the file exists).
```

### `go/internal/faillearn/inbox_drop_test.go:10` — above `func inboxFixture(t *testing.T) (inboxDir string, item InboxItem) {`

```text
// inbox_drop_test.go — cycle-1282 DEF-4 regression lock. Inbox filenames are
// fully deterministic (`retro-<cycle>-<slug>` over agent-authored defect text),
// so a concurrent fleet lane at standing width 3, or stale state from an earlier
// run of the same cycle number, can already hold the name. writeIfAbsent
// returned nil there and WriteArtifacts still reported success: the real
// remediation item was DROPPED with no error, no diagnostic, no telemetry —
// reproducing the exact 1255 state ("filed" in the report, absent from the
// queue) that this package exists to make unreachable.
```

### `go/internal/faillearn/inbox_failure_degraded_test.go:10` — above `const degradedRetroName = "retrospective-unqueued.md"`

```text
// inbox_failure_degraded_test.go — RED contract for cycle-1290 T2
// (`faillearn-inbox-failure-preserves-diagnosis`), the residual the cycle-1287
// landing note explicitly "named here rather than closed":
//
//	"a disk-level inbox failure still suppresses the retrospective".
//
// The 1255 invariant is load-bearing and is NOT reversed: an on-disk
// `retrospective-report.md` may never claim remediation that reached no queue, so
// WriteArtifacts must keep aborting before it writes that file and must keep
// returning the error. What it must stop doing is losing the DIAGNOSIS with the
// queue write: today an unwritable inbox dir yields zero artifacts on disk, so the
// failure analysis dies with the failure it describes.
//
// Design decision this contract freezes (surfaced rather than assumed — the scout
// report asks for "a retrospective marked UNQUEUED" while
// inbox_transactional_test.go asserts `retrospective-report.md` is ABSENT on this
// arm, and that file is required to stay unmodified and green): the degraded
// artifact is published under a DISTINCT name, `retrospective-unqueued.md`. A
// distinct name satisfies both halves at once — no reader or gate that keys on
// `retrospective-report.md` can mistake a degraded diagnosis for a complete,
// queued one, and the diagnosis survives. Reusing the canonical name would force
// an edit to the transactional test, which hypothesis H3 defines as the signal
// that the design is wrong.
```

### `go/internal/faillearn/inbox_partial_write_test.go:11` — above `func partialWriteItems() []InboxItem {`

```text
// inbox_partial_write_test.go — RED contract for cycle-1292 T1, closing
// cycle-1290 defect D2 (`.evolve/runs/cycle-1290/defect-dispositions.json`,
// evidence `go/internal/faillearn/writer.go:96`):
//
//	writeInboxItems is not atomic across items — it writes one file per item and
//	returns on the FIRST failure, so every item BEFORE the failing one is already
//	on disk. preserveDiagnosis then lists every configured item as "still
//	UNQUEUED", so on a partial write the degraded artifact OVERCLAIMS which
//	remediation reached no queue.
//
// The overclaim is the defect the continuation-defect-ledger lane exists to
// catch: an artifact asserting on disk something that is false on disk. The
// direction of the error is safe (re-filing an identical item is idempotent by
// writeIfAbsent's same-content rule), so what is at stake is operator and
// next-continuation confusion — an item listed as unqueued that IS queued sends
// the reader looking for work that is already filed.
//
// What this contract does NOT freeze: the mechanism. preserveDiagnosis today has
// only c.inboxItems to work from and therefore CANNOT distinguish queued from
// unqueued (scout Key Finding 1); making it able to is the builder's design
// choice — a returned count, a returned id set, a partial-write marker type. The
// assertions below are on the EMITTED ARTIFACT only, so any of those fixes pass
// and none is mandated.
//
// The 1255 invariant and the 1287 residual fix are both untouched here:
// retrospective-report.md must still be absent on every failure arm, and
// WriteArtifacts must still return the error. Those are asserted alongside the
// new property because a fix that regresses either while getting the item list
// right is the fix being wrong.
```

### `go/internal/faillearn/inbox_partial_write_test.go:108` — above `func collidingInboxDir(t *testing.T, failIdx int) string {`

```text
// collidingInboxDir prepares an inbox directory in which the item at index
// failIdx of partialWriteItems() cannot be written: a file already sits under
// that id carrying DIFFERENT content, which writeInboxItems refuses to drop
// (the cycle-1282 DEF-4 id-collision rule).
//
// This injection — rather than an unwritable directory — is what produces a
// genuine PARTIAL write: every item before failIdx is written normally, so the
// arm exercises the "some reached disk" state instead of the "none did" state.
```

### `go/internal/faillearn/inbox_pathsafety_test.go:10` — above `func traversalIDs() []string {`

```text
// inbox_pathsafety_test.go — RED contract for cycle-1282 D7
// (.evolve/runs/cycle-1279/audit-report.md, LOW): WithInbox concatenates
// `it.ID + ".json"` into a path with no sanitisation (inbox.go:81). The sole
// current caller is safe (remediationSlug emits [a-z0-9-] only), so this is not
// presently exploitable — it is a trap on a NEWLY EXPORTED API, and the next
// caller is the one that falls in.
//
// The rule: an id that is not a bare filename is rejected loudly, exactly as an
// empty id already is. Rejection (not sanitisation-and-write) is the right
// shape here — a silently rewritten id produces an item nobody can address by
// the id they filed it under, which is the erasure this package exists to stop.
```

### `go/internal/faillearn/inbox_transactional_test.go:11` — above `func remediationEvent() FailureEvent {`

```text
// inbox_transactional_test.go — RED contract for cycle-1279 Task 3
// (`retro-inbox-transactional-write`, batch-integrity-review-2026-08-04.md F1
// solution bullet ii).
//
// The defect this pins: cycle-1255's retrospective filed two remediation items
// that "exist only inside .evolve/runs/cycle-1255/retrospective-report.md and
// never reached the inbox" — so the loop's own remediation queue never saw
// them and the defects were laundered away by later continuations. There is no
// mechanism today that writes retro-derived remediation items into
// `.evolve/inbox` in the SAME atomic call as the retrospective/lesson.
//
// API pinned by this contract (functional options — the repo idiom; the three
// existing WriteArtifacts callers stay byte-identical):
//
//	type InboxItem struct{ ID, Title, Kind, Priority, InjectedBy string; Weight float64; Files []string }
//	func WithInbox(dir string, items []InboxItem) Option
//	func WriteArtifacts(ev FailureEvent, runDir, lessonsDir string, opts ...Option) error
//
// InboxItem's JSON tags MUST match inboxbatch.Item's wire shape (id, title,
// weight, kind, priority, files, injected_by) — faillearn is a leaf package
// (stdlib + yaml.v3 only) so it cannot import inboxbatch; parity is by tag,
// asserted below on the raw JSON keys rather than by a Go type reference.
```

### `go/internal/faillearn/inbox_transactional_test.go:106` — above `var wire map[string]any`

```text
// Assert on the RAW JSON keys: inboxbatch.Item is the consumer and it
// binds by wire tag, so a Go-side field rename that broke the tag would
// silently produce items the loader drops (the cycle-1190 Class-field
// shape of this same bug).
```

### `go/internal/faillearn/novelty.go:11` — above `const defaultNoveltyThreshold = 0.9`

```text
// novelty.go — near-duplicate suppression on the lesson-write seam
// (cycle-1494, `sleep-time-kb-consolidation`).
//
// writeIfAbsent already dedupes by exact PATH, but the lesson id is
// "cycle-N-<scope>-<slug>": the SAME observation recurring on a later cycle
// lands under a different filename, so an install that fails the same way for
// twenty cycles accumulates twenty near-identical lessons. Recall then ranks a
// corpus that is mostly one repeated failure, which is the growth the inbox
// item ("identical observation twice → one write") asks to bound.
//
// The gate is deliberately conservative in ONE direction: suppressing a lesson
// destroys failure evidence and cannot be undone, so anything short of an
// almost token-identical match is written. Two hard rules follow from that:
//
//   - a materially different failure is never suppressed (that is the whole
//     value of the corpus), and
//   - corpus rot is inert — an unparseable neighbour is skipped, never treated
//     as a reason to drop the incoming lesson, and never rewritten or deleted.
```

### `go/internal/faillearn/novelty_test.go:10` — above `func recurringEvent(cycle int) FailureEvent {`

```text
// novelty_test.go — the near-duplicate gate on the lesson-write seam
// (cycle-1494, `sleep-time-kb-consolidation`). Every case drives the real
// WriteArtifacts and asserts on what actually landed on disk.
```

### `go/internal/faillearn/writer.go:73` — above `func (c writeConfig) preserveDiagnosis(ev FailureEvent, runDir string, cause error) error {`

```text
// preserveDiagnosis publishes the failure analysis under unqueuedRetroName after
// the queue write failed, and returns cause unchanged so the caller still fails
// loudly. Preserving the diagnosis is an ADDITION to the abort, never a
// replacement for it.
//
// The residual the cycle-1287 landing named rather than closed: the abort
// ordering is correct — a retrospective may not claim remediation that reached no
// queue — but an unwritable inbox left ZERO artifacts on disk, so the analysis of
// the failure died together with the failure it described, and the next
// continuation had to re-derive it. The degraded artifact carries the diagnosis
// plus the ids of every item that did NOT reach the queue, which is the work that
// would otherwise be lost.
//
// A failure to publish the degraded artifact is joined onto cause rather than
// replacing it: the queue failure is the diagnosis the caller acts on.
```

### `go/internal/faillearn/writer.go:118` — above `func writeIfAbsent(path string, data []byte) (skipped bool, err error) {`

```text
// writeIfAbsent atomically writes data to path unless it already exists, and
// REPORTS whether it skipped. Preserving a richer existing artifact is the
// intended behavior for the retrospective and the lesson; for the inbox it is a
// dropped remediation item, so the caller — not this function — decides what a
// skip means (cycle-1282 DEF-4: a silent skip made "the retro was written"
// true while "the remediation was queued" was false, which is precisely the
// 1255 state WithInbox exists to make unreachable).
//
// Exclusive by construction (cycle-1285 F4). The previous shape was
// stat-then-write, so two fleet lanes could both observe "absent" and both
// write, and the DEF-4 content-equality check above only ever observed that
// race after the fact. Publishing with os.Link makes "does not exist" and "I
// created it" ONE atomic step: link fails EEXIST against a concurrent winner,
// which is reported as an ordinary skip and re-enters the DEF-4 check. The
// temp file is written whole before it is linked, so a crash mid-write can
// never leave a partial artifact under the real name — the property
// atomicwrite gave us and rename-based publishing would give away here,
// because rename silently CLOBBERS an existing file.
```

### `go/internal/faillearn/writer_mode_test.go:10` — above `const publishedMode fs.FileMode = 0o644`

```text
// writer_mode_test.go — RED contract for cycle-1290 T1
// (`faillearn-publish-mode-parity`, cycle-1287 audit defects[0] / F1 MEDIUM).
//
// The defect: writeIfAbsent publishes through os.CreateTemp (mode 0600) + os.Link
// with no Chmod, while internal/atomicwrite.Bytes documents and enforces 0644 for
// every other published runtime artifact. So the failure floor's OWN artifacts —
// retrospective-report.md, lessons/*.yaml, .evolve/inbox/*.json — land 0600: read
// only by the uid that minted them, while other fleet lanes and the operator are
// the intended readers. Nothing in the tree pins the mode today, which is why the
// 1285 stat-then-write → link-publish rewrite could drop it silently.
//
// publishedMode is the contract constant: the same literal atomicwrite.Bytes
// applies. It is spelled out here rather than imported so faillearn stays a leaf
// package (stdlib + yaml.v3) in test builds too; the parity is asserted against
// atomicwrite's documented value, cited at atomicwrite.go:61-63.
```
