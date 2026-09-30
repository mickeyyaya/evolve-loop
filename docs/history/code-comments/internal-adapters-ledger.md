# Comment history: `internal/adapters/ledger`

The history this package's comments carried, by the rule `commentaudit check` uses, recorded by `commentaudit history` from the comments each section's change removed. Each entry is the comment as it was, where it sat and the code below it. See [the code-comments convention](../../conventions/code-comments.md).

## comment reduction backfill, rounds 1-12 and the zero-comment policy (3ce14dd0 to f27afd8b)

### `go/internal/adapters/ledger/anchor.go:1` — above `package ledger`

```text
// anchor.go — ADR-0048 ledger epoch-anchor (the ledger-1740 disposition).
//
// A predecessor's bytes were rewritten post-hoc (cycle-107 era), permanently
// breaking the SHA chain at that point; `evolve ledger verify` correctly stays
// RED on it. The non-destructive remedy (ADR-0048 §non-goals, operator-chosen
// over a destructive rebaseline) is an EPOCH-ANCHOR: declare a known-good
// genesis at a post-damage line and verify FORWARD from it. The damaged segment
// is PRESERVED in the file (auditable), it is simply no longer chain-validated.
//
// This is an operator TRUST decision — the ADR requires sign-off — so it is an
// explicit command (`evolve ledger anchor <seq>`), never automatic. The anchor
// binds to the target line's CURRENT SHA, so any later alteration of the trusted
// prefix self-invalidates the anchor (walkChain then fails "anchor not found")
// rather than silently extending trust to tampered bytes.
```

### `go/internal/adapters/ledger/anchor.go:30` — above `var ErrAmbiguousAnchorSeq = errors.New("ambiguous entry_seq: carried by more than one distinct line")`

```text
// ErrAmbiguousAnchorSeq is returned when more than one DISTINCT line carries the
// requested entry_seq, so no single line can be bound without the operator
// naming it.
//
// entry_seq is not unique in real history: pre-CA.1 concurrent Appends raced the
// tip and wrote fork siblings sharing a seq (walkChain accepts them; see its
// FORK SIBLING carve-out). Binding the FIRST match — what this command did until
// cycle-1433 — silently picks the EARLIER sibling, which moves the epoch anchor
// BACKWARD and re-exposes lines the operator believed were already sealed. An
// anchor is a trust decision, so an ambiguous one must be refused, not guessed;
// the caller disambiguates by line SHA, which names exact bytes.
```

### `go/internal/adapters/ledger/anchor.go:68` — above `func isOperatorSeal(e core.LedgerEntry) bool {`

```text
// isOperatorSeal reports whether e is an in-band operator epoch seal.
//
// The marker is accepted in EITHER field, because the ledger carries two
// shapes and only one of them was ever recognised: the PRODUCTION writer
// (core.SealCycle, go/internal/core/reset.go) emits `kind:"reset"` with the
// marker in `cycle_label` ("reset-seal-cycle-108"), while the doc-level shape
// puts it in `kind`. Matching on Kind alone made this resolver inert on every
// real ledger — unit-green, live-red — so `evolve ledger verify` kept crying
// wolf on the adjudicated line-1740 damage (cycle-1191). Role is still the
// authority check: a phase agent cannot silence Verify under its own role.
```

### `go/internal/adapters/ledger/anchor.go:104` — above `type VerifiedScope struct {`

```text
// VerifiedScope names WHICH history a successful verification actually
// validated. The two outcomes are very different claims — every byte from
// genesis, or a strict walk that resumed at an epoch anchor whose preserved
// prefix an operator adjudicated (ADR-0048) — and until cycle-1677 both ended
// at the same `OK: chain intact` string, which is the shape that let the
// ledger-1740 damage stay invisible for as long as it did. The anchor is named
// by its OWN identity, read out of the ledger, so no literal can stand in for
// it: two ledgers sealed at different lines report differently.
//
// The zero value means full-strict verification (no anchor in play).
```

### `go/internal/adapters/ledger/anchor_ambiguity_test.go:62` — above `func TestAnchor_RejectsAmbiguousSeq(t *testing.T) {`

```text
// TestAnchor_RejectsAmbiguousSeq: a seq carried by two distinct lines is refused
// with ErrAmbiguousAnchorSeq, naming both candidates, and writes NO anchor file.
// Before cycle-1433 this bound the FIRST sibling and exited 0, silently moving
// the epoch anchor backward past a line the operator believed was sealed.
```

### `go/internal/adapters/ledger/composition.go:1` — above `package ledger`

```text
// composition.go — kernel verification of composition-verdict entries
// (merge ladder RUNG 0, cycle-786; knowledge-base/research/
// merge-concurrency-2026: review verdicts follow the CHANGE via git
// patch-id, gates follow the TREE).
//
// A composition-verdict entry records that an audit verdict carried
// forward across a conflict-free trivial rebase. It is deterministic and
// kernel-recomputable: anyone can re-derive the patch-id of its two
// persisted diff artifacts and compare against the recorded patch_id —
// zero LLM tokens. Verify/VerifyDeep do exactly that for every such
// entry; a mismatch (drifted composed diff, forged patch_id, missing
// artifact) is tampering and breaks the chain like any hash break.
```

### `go/internal/adapters/ledger/composition_method_test.go:1` — above `package ledger`

```text
// composition_method_test.go — RED contract for the merge ladder RUNG 2
// `method` field (cycle-941, merge-rung2-scoped-review-core; knowledge-base/
// research/merge-concurrency-2026).
//
// RUNG 0 records every composition-verdict line with method:"trivial-rebase"
// unconditionally (composition.go). RUNG 2 composes after a scoped merge review
// resolves an overlapping change; its verdict must be distinguishable —
// method:"scoped-review" — so the ship-side reader and any audit can tell a
// trivial-rebase carry-forward from a reviewed composition. This adds a
// caller-supplied `Method` field to CompositionVerdictInput, defaulting to
// TrivialRebaseMethod when blank so existing RUNG 0 callers are byte-for-byte
// unchanged.
//
// RED at authoring: ScopedReviewMethod and CompositionVerdictInput.Method are
// undefined (compile failure). Builder contract: add the const + field and
// thread Method into the persisted line (defaulting empty → TrivialRebaseMethod).
// DO NOT modify this file. Reuses honestWriteInput/passingComposedGates from
// composition_write_test.go (same package).
```

### `go/internal/adapters/ledger/composition_test.go:1` — above `package ledger`

```text
// composition_test.go — unit pins for the composition-verdict kernel
// checker (cycle-786). The end-to-end `evolve ledger verify` exit-code
// contract lives in cmd/evolve/cmd_ledger_composition_test.go; these tests
// name the exported API (apicover) and exercise Verify directly.
```

### `go/internal/adapters/ledger/composition_write_test.go:1` — above `package ledger`

```text
// composition_write_test.go — RED contract for WriteCompositionVerdict, the
// RUNG 0 producer (cycle-787; knowledge-base/research/merge-concurrency-2026).
// Cycle-786 landed the reader (ship.tryTrivialRebaseCarryForward) and the
// kernel verifier (verifyCompositionLine); nothing in the tree writes a
// composition-verdict line yet. These tests encode the writer's contract:
//
//   - round-trip: a written line is accepted by the existing kernel verify
//     and carries every field both consumer structs read;
//   - fail-closed at write time: a patch_id that does not recompute from the
//     supplied diffs, or a gate_results map not green on the full
//     ciparity.RequiredComposedGates set, is an error and appends NOTHING;
//   - empty/whitespace-only diffs are rejected (mirrors PatchID(nil) erroring).
//
// Builder contract: implement WriteCompositionVerdict in composition.go to
// turn these GREEN. Do not modify this file.
```

### `go/internal/adapters/ledger/ledger.go:220` — above `func (l *FileLedger) appendChainedFromTail(fill func(seq int, prevHash string) any) error {`

```text
// appendChainedFromTail is the REPAIR-path variant of appendChained: it
// chains the new entry from the PHYSICAL last line of the file, not from
// ledger.tip. The tip tracks the last line written through the chained path,
// but walkChain validates physical predecessors — so when a foreign writer
// has raw-appended lines past the tip (the fleet-concurrency damage class),
// a tip-chained seal binds the wrong predecessor and is rejected by
// sealChainsFromPrev (console-plane live failure 2026-08-11). Full-file read
// per call: acceptable for operator repair, wrong for the hot append path —
// which is why appendChained stays tip-based.
```

### `go/internal/adapters/ledger/ledger.go:303` — above `func walkChain(lines [][]byte, anchorLineSHA string) (lastSeq int, lastSha string, sawV837 bool, err error) {`

```text
// walkChain is THE chain walk, shared by Verify (live file only) and
// VerifyDeep (decompressed segments + live tail, L3.3) so the two can
// never diverge on what "intact" means.
// The walk is strict, with two carve-outs the PRODUCTION ledger's history
// requires (both made plain `evolve ledger verify` red on the real file,
// unnoticed, until the L3.3 acceptance run surfaced them):
//
//   - RE-GENESIS seam: an Append against a missing/lost tip re-seeds the
//     chain (entry_seq==0 + zero prev_hash). One exists (line 15,
//     2026-05-07 — the day v8.37 chain hashing landed). Accepted: a seam
//     is visible, every later line still hashes over its bytes, and the
//     tip + L3.3 segment anchors bind the end state. A zero prev with a
//     NONZERO seq stays a break.
//   - FORK SIBLING: pre-CA.1 concurrent Appends raced the tip and wrote
//     sibling entries sharing one parent (e.g. lines 263/264 with equal
//     seqs; line 273 with a +1 seq — the racy seq is unreliable, the hash
//     linkage is the trustworthy part). Accepted exactly when the entry's
//     prev equals the PREVIOUS line's prev (shared parent); the chain
//     resumes from the last sibling. The CA.1 flock prevents new ones.
```

### `go/internal/adapters/ledger/ledger.go:325` — above `inEpoch := anchorLineSHA == ""`

```text
// ADR-0048 ledger epoch-anchor (ledger-1740): when an operator has recorded a
// trusted genesis line, lines BEFORE it are NOT chain-validated — the
// historical damage is real, preserved (never deleted), and accepted by
// explicit operator sign-off. inEpoch starts true when no anchor is set, so
// the no-anchor path is byte-identical to the pre-anchor behavior; strict
// validation always resumes for every line AFTER the anchor.
```

### `go/internal/adapters/ledger/ledger.go:351` — above `if e.Kind == CompositionVerdictKind {`

```text
// Composition-verdict entries are kernel-recomputable (cycle-786):
// both persisted diff artifacts must re-derive the recorded patch_id,
// or the entry is tampered and breaks the chain like a hash break.
```

### `go/internal/adapters/ledger/ledger_bench_test.go:11` — above `func benchEntry(seq int) core.LedgerEntry {`

```text
// Phase 4 task #19: ledger append throughput. The ledger is the
// hot-path on every phase boundary — orchestrator, role-gate, and
// phase-gate all record entries. Bash baseline is roughly 10-20 ms per
// append (jq + tee + flock + sha256sum). Go target ≤ 0.6× per parent
// plan §6 item 8.
//
// Run: go test -bench=. -benchmem -run=^$ ./internal/adapters/ledger/
```

### `go/internal/adapters/ledger/ledger_real_iter_test.go:10` — above `func TestIter_RealLedger_NoStringCycleError(t *testing.T) {`

```text
// TestIter_RealLedger_NoStringCycleError confirms that the live reader
// can iterate the project's real .evolve/ledger.jsonl end-to-end
// without the "json: cannot unmarshal string into Go struct field
// LedgerEntry.cycle of type int" failure that the dispatcher logged at
// line 1740 during cycle-107 (2026-05-26).
//
// The test is skipped when the file isn't reachable (CI sandboxes,
// fresh checkouts), so this acts as a project-local smoke without
// constraining external test environments.
```

### `go/internal/adapters/ledger/ledger_real_iter_test.go:63` — above `if withLabel == 0 {`

```text
// Sanity: we know the v10.16.0 manual entry exists in this project's
// ledger. If the project ledger is loaded, we expect at least one
// CycleLabel-carrying entry (the legacy bad line at seq=1740).
```

### `go/internal/adapters/ledger/ledger_seal_concurrency_test.go:11` — above `func TestSeal_ConcurrentWithCrossProcessAppend_ChainStaysVerifiable(t *testing.T) {`

```text
// ledger_seal_concurrency_test.go — Phase 2 / S2.1 (modularization campaign,
// ADR-0050). The append-vs-append cross-process race is already covered by
// TestAppend_TwoProcessStress. The UNTESTED, safety-critical windows on the
// hash chain are the other two mutators of ledger.jsonl:
//
//   - Seal vs a concurrent cross-process Append. Seal (seal.go) holds
//     ledger.lock to truncate the live file, RELEASES it, then re-acquires it
//     for the chained anchor Append. A foreign `evolve` process can grab
//     ledger.lock anywhere in that handoff. The chain must stay verifiable and
//     no append may be lost. RED-check: drop the inner flock.Lock(l.lockPath)
//     from Seal's sealLocked wrapper (keep only seal.lock) — the truncation then
//     races a foreign append and rewriteLive's tmp+rename drops it; childN
//     below goes < stressN. This proves the test asserts the lock handoff, not
//     merely "did not panic".
//   - Anchor vs a concurrent Seal. Anchor (anchor.go) takes NEITHER l.mu NOR a
//     flock; it reads the full chain while Seal rewrites it. It must never bind
//     ledger-anchor.json to a line the seal removed.
//
// Both assert an INVARIANT (VerifyDeep intact / anchor points at a real line),
// never a bare no-panic. seedLedger(t, n) (*FileLedger, string) is the shared
// helper from seal_test.go (returns the seeded ledger + its dir).
```

### `go/internal/adapters/ledger/rebaseline.go:1` — above `package ledger`

```text
// rebaseline.go — seal a densely damaged chain prefix in ONE operator call.
//
// `evolve ledger anchor` is the remedy for ONE accepted break: the operator names
// the post-damage line and the chain verifies forward from it. That does not
// scale to the shape the console-plane ledger actually has — ~180 dense breaks
// from pre-CA.1 fleet-concurrency interleaving — because each anchor call binds a
// single line and the LAST one wins, so repairing N breaks would mean N sequential
// invocations with only the final one in effect. That is why that ledger was left
// broken rather than repaired.
//
// Rebaseline uses the IN-BAND seal instead (ADR-0081; see resetSealKindPrefix in
// anchor.go): it APPENDS one operator-role `reset-seal-*` entry at the tip. The
// entry chains from the current tip like any other line, so effectiveAnchorSHA
// moves the epoch anchor to it and walkChain validates strictly forward from
// there — however many breaks lie behind it. The damaged prefix is preserved
// byte-for-byte (this only ever appends; nothing is rewritten or truncated), it
// is simply no longer chain-validated, which is exactly the preservation remedy
// ADR-0048 chose over a destructive rebuild.
//
// It is gated on an operator note for the same reason `anchor` is an explicit
// command: declaring a prefix trusted-but-unvalidated is a TRUST decision, and an
// unattributable one is worse than a red chain.
```

### `go/internal/adapters/ledger/rebaseline.go:58` — above `entry := core.LedgerEntry{`

```text
// Append chained from the PHYSICAL last line, not the tip: the damage
// class this command exists for (out-of-band interleaved appends) leaves
// foreign lines PAST the tip, and sealChainsFromPrev tests the physical
// predecessor — a tip-chained seal binds the wrong line and is rejected
// (console-plane live failure 2026-08-11). Same flock as the normal path,
// so a concurrent chained writer cannot interleave; the tip moves to the
// seal so subsequent normal appends chain green from it.
```

### `go/internal/adapters/ledger/rebaseline_foreign_tail_test.go:3` — above `import (`

```text
// rebaseline_foreign_tail_test.go — the console-plane live failure 2026-08-11:
// Rebaseline appended its seal through the TIP-chained path, but the physical
// last line of the file was a FOREIGN record (inboxmover's raw O_APPEND write:
// no prev_hash, no tip update), so the seal's prev_hash pointed at the last
// CHAINED line, sealChainsFromPrev rejected it against the physical
// predecessor, the anchor never moved, and the command failed with "seal
// appended but the chain still does not verify forward" — on exactly the
// damage class (out-of-band interleaved appends) the command was built for.
// A seal must bind the file AS IT PHYSICALLY EXISTS, not as the tip remembers
// it.
```

### `go/internal/adapters/ledger/rebaseline_test.go:87` — above `func TestRebaseline_PreservesDamagedPrefixBytes(t *testing.T) {`

```text
// TestRebaseline_PreservesDamagedPrefixBytes: the repair is append-only. A
// rebaseline that greened the chain by rewriting or truncating history would
// destroy the auditable record — the outcome ADR-0048 rejects in favour of the
// epoch anchor.
```

### `go/internal/adapters/ledger/seal_role_trust_test.go:3` — above `import "testing"`

```text
// seal_role_trust_test.go — cycle-1194 continuation (ADR-0081 audit defect):
// the epoch-anchor resolver must trust an in-band seal ONLY when its Role is
// exactly "operator" — the marker core.SealCycle writes for an explicit human
// `evolve cycle reset`. A line whose Role is "operator-autoseal" (what the
// unattended boot self-heal path, core.AutosealStaleMarker, now writes) must
// NOT move the anchor, even when it is itself perfectly hash-valid — hash
// validity alone proves nothing about WHO wrote the line, only that the bytes
// chain correctly, which requires no secret at all.
```

### `go/internal/adapters/ledger/seal_test.go:443` — above `func TestSeal_RealLedgerCopy(t *testing.T) {`

```text
// Acceptance (plan L3.3, adjusted to reality): the plan asked for "deep
// verify green on a copy of the real ledger before/after seal" — but the
// REAL ledger has genuine pre-hardening damage (line 1740, 2026-05-26:
// entry 1740 chains from a hash matching nothing — its predecessor's
// bytes were rewritten post-hoc; the same line the Iter regression test
// memorializes). Blessing that class would gut the verifier, so the
// honest chain-preservation property is VERDICT preservation: sealing
// never changes what verification says — green stays green (covered by
// the synthetic tests above), and a broken ledger stays broken at the
// SAME line with the SAME hashes. Skips when the real ledger is not
// reachable (CI sandboxes) — same convention as TestIter_RealLedger.
```

### `go/internal/adapters/ledger/signals.go:3` — above `import (`

```text
// signals.go — ADR-0101 S4a: the Signal Center observes the file ledger at its
// ONE append chokepoint. WithSignals is a construction-time option (functional
// options; explicit DI at the root) that installs the observer: every
// core.LedgerEntry written through Append — the orchestrator's records, the
// bridge's stop_review, the inbox mover's lifecycle lines (AppendLifecycle),
// the seal's segment anchor — is also a ledger.appended INFO signal naming
// its ledger line (fields.entry_seq); a failed Append is a WARN
// LEDGER_APPEND_FAILED and the error still returns. Deliberately NOT a wrapper
// type: Go embedding promotes methods without virtual dispatch, so a Decorator
// over *FileLedger would let every promoted append path (AppendLifecycle,
// Seal) write unobserved — the S4a architecture review's HIGH-1. The lines the
// observer does not see (self-constructed ledgers, the repair marker) are the
// inventory pinned in signals_test.go.
```

### `go/internal/adapters/ledger/signals_test.go:3` — above `import (`

```text
// signals_test.go — ADR-0101 S4a: the Signal Center observes the file ledger
// at its ONE append chokepoint. Every core.LedgerEntry written through Append
// — the orchestrator's records, the bridge's stop_review, the inbox mover's
// lifecycle lines (AppendLifecycle), the seal's segment anchor — is also a
// ledger.appended INFO signal naming its ledger line; a failed Append is a
// WARN LEDGER_APPEND_FAILED and the error still returns. The observer is
// installed by the WithSignals construction option, not by a wrapper type:
// Go embedding promotes methods without virtual dispatch, so a Decorator over
// *FileLedger let AppendLifecycle and Seal append unobserved (S4a
// architecture review HIGH-1). The go/ast guard at the end keeps every line
// writer either on the chokepoint or on the documented exempt inventory.
```

### `go/internal/adapters/ledger/verify_scope_test.go:3` — above `import (`

```text
// verify_scope_test.go — cycle-1677: a successful verification must state
// WHICH history it accepted. `OK: chain intact` was one string for two very
// different claims — every byte from genesis, or a strict walk that resumed at
// an operator-adjudicated epoch anchor — and that ambiguity is what let the
// ledger-1740 damage stay invisible. These tests pin the scope VerifyScope /
// VerifyDeepScope report, including the part a sidecar file cannot answer.
```

### `go/internal/adapters/ledger/verify_scope_test.go:138` — above `func TestVerifyDeepScope_ReportsTheSameScopeAsVerifyScope(t *testing.T) {`

```text
// TestVerifyDeepScope_ReportsTheSameScopeAsVerifyScope: an operator's two
// verification commands must not disagree about what was verified (#373, the
// wired-into-one-path-only defect).
```
