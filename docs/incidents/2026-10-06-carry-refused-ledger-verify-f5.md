# No byte-identical carry ever shipped: the carry's evidence died with its worktree (cycles 1766–1810, 2026-09-29 to 2026-10-06)

## What happened

ADR-0105's carry lets a lane ship on its audited verdict when its audited change rebases byte for byte onto a peer's landing. Six lanes carried between 2026-09-29 and 2026-10-06. Each logged `rebase is byte-identical: the audited verdict carries, shipping without a second audit (ADR-0105 B3)`. Then ship refused each one with `SHIP_AUDIT_BINDING_TREE_MISMATCH`, which read `… error=<nil>`, and the cycle paid a full re-audit. None of the six ever shipped on its carry.

| Cycle | Wave log, carry line → ship refusal | Carry (UTC) | Carry record (0-based ledger line, entry_seq) | Binary had the 2026-09-30 binding fix? | Outcome |
|---|---|---|---|---|---|
| 1766 | `wave44.log:149899` → `:292768` | 2026-09-29 21:17 | 154453, seq 154005 | no | re-audit, then shipped |
| 1768 | `wave45.log:12757` → `:19400` | 2026-09-29 22:19 | 154690, seq 154242 | no | re-audit, then shipped |
| 1772 | `wave47.log:287134` → `:429886` | 2026-09-30 02:15 | 155267, seq 154819 | no | re-audit, then shipped |
| 1773 | `wave48.log:149209` → `:291924` | 2026-09-30 04:41 | 155607, seq 155159 | no | re-audit, then shipped |
| 1782 | `wave54.log:294589` → `:444461` | 2026-09-30 14:10 | 156527, seq 156079 | yes | re-audit, then shipped |
| 1810 | `wave67.log:162809` → `:173128` | 2026-10-06 13:04 | 159790, seq 159342 | yes | re-audit |

On 2026-10-06, `evolve ledger verify` on the plane reported:

```
[ledger] BROKEN: core: ledger hash chain broken: line 154453 composition-verdict audited_diff_path unreadable: open …/runtime/.evolve/worktrees/cycle-cd3ae73e-1766/.evolve/composition-artifacts/composition-1766-audited.diff: no such file or directory
```

All twelve diffs the six records name were gone. The worktrees that held them had been cleaned up with their cycles.

## Root cause

Two refusals were at work, one on top of the other:

1. **Until 2026-09-30, ship never consulted the carry at all** (the 2026-09-30 CHANGELOG entry). `verifyAuditBinding`'s predicate-execution tree check compared the held tree with the audited one directly. Cycles 1766 to 1773 ran binaries without that fix, so they were refused there first.
2. **ADR-0105 F5: the carry's evidence lived in the cycle worktree.**
   - `identityCarryForward` (`go/internal/core/identity_carry_forward.go:62`) passed `ArtifactDir: <worktree>/.evolve/composition-artifacts` to the ledger writer, but wrote the record into the project ledger. The ADR said "artifacts live in the host workspace"; the code did not do that.
   - Worktrees are deleted when their cycle ends (`core/worktree.go`, `Cleanup`).
   - Ship's re-proof of a carry, `carrySatisfied` (`go/internal/phases/ship/carry.go`), requires a whole-ledger `ledger.Verify`. Verify re-derives every composition line's patch-id from the files the line names, and it treats a missing file as a broken chain (`adapters/ledger/ledger.go`, `walkChain`; `composition.go`, `verifyCompositionLine`).
   - So once 1766's worktree was cleaned up, the plane ledger stopped verifying, and every later carry was declined at that step, including 1782 and 1810, which ran binaries with the 2026-09-30 fix.
3. **The decline was silent.** `carrySatisfied` returned `(false, "")` on every path, so the refusal named no reason. `error=<nil>` is the predicate tree check's own error value, which was nil.

The 2026-09-26 design review had said "F5 must land together with B3". B3 shipped on 2026-09-27 with "the F5 artifact relocation stays separate", and no inbox item tracked F5.

### Why the tests missed it

- The 2026-09-30 fix's test helper (`writeCarryOf`, `phases/ship/carry_test.go`) kept the carry's diffs in `repo/.evolve`. No cleanup removes that directory, so the test never saw a carry whose evidence was gone.
- Every carry test built a fresh ledger holding exactly one carry. No test ran two sequential carries with the first carry's worktree removed in between. The second carry is the shape that failed live.
- No test used the real ledger writer through the real identity carry and then removed the worktree.

### Two more hazards the review found

- **A verify race.** Verify read the 49 MB ledger, walked it (seconds), and only then read `ledger.tip` without the ledger lock. A peer's append in between read as a tip mismatch, which would also decline a carry.
- **RUNG 0/2 wrote through a linked ledger.**
  - `compositionCarryForward` and `scopedMergeCarryForward` (`core/composition_carryforward.go`) passed `<worktree>/.evolve/ledger.jsonl`. That path is a symlink to the plane ledger (`linkGuardDeps`), but the writer derives its tip, lock and artifacts from the path's directory, which is the worktree's.
  - So a rung-0/2 line would have chained from a worktree-local tip, landed in the plane ledger, and forked the plane chain, and its diffs would have died with the worktree.
  - The live ledger holds no such line, because every live cycle now carries the explanation contract and takes the identity carry instead.

## The fix

Phase 1 of the [ledger restructure plan](../plans/ledger-restructure-2026-10.md) ([ADR-0123](../architecture/adr/0123-ledger-durable-evidence-segments-incremental-verify.md)), built test-first, one component at a time:

- **C1, a durable evidence store.** `internal/ledgerartifacts` is a write-once, content-addressed store at `.evolve/ledger-artifacts/sha256/<2>/<62>`. `Put` is idempotent. `Get` re-checks the sha and refuses damaged bytes. gc hard-protects the directory beside `ledger-segments`.
- **C2, composition verdicts write into the store.**
  - `WriteCompositionVerdict` stores both diffs beside the ledger it writes, and records `audited_diff_sha256` and `composed_diff_sha256`. The `*_diff_path` fields name the store objects, so an older binary's verify still finds them.
  - Callers no longer choose a directory: the `ArtifactDir` input is gone.
  - Verify resolves each diff in this order: from the store by sha, then through a chained evidence record (C3), then from the recorded path. A committed diff it cannot produce is still a chain break (fail closed).
- **C3, `evolve ledger evidence restore [--dry-run]`.**
  - For each composition line whose diffs are not durable, it takes the diff from the recorded path if it can, or else regenerates it from the commits and trees the line names, with the carry's own `treedelta` flags.
  - It accepts only bytes that re-derive the recorded patch-id, stores them, and appends one chained `composition-evidence` record naming the line and the stored diffs.
  - It never rewrites a line.
- **C4, rungs 0 and 2 write through the project ledger**, not the worktree's linked one.
- **C5, a loud and race-free carry check.**
  - `carrySatisfied` returns its reason, and the TREE_MISMATCH message ends with `(carry not re-proven: <reason>)`. When a record existed, ship also raises the Signal Center warning `SHIP_CARRY_NOT_REPROVEN`.
  - Verify reads the chain and the tip as one snapshot under the ledger lock.

Regression tests, each red on the old code:

- `core`:
  - `TestCarryRecord_VerifiesAfterItsCycleWorktreeIsRemoved`: the real writer through the real identity carry.
  - `TestCompositionCarry_ChainsFromTheProjectLedgersTipThroughALinkedWorktree`: both rungs.
- `phases/ship`:
  - `TestVerifyAuditBinding_ShipsASecondCarryAfterTheFirstCarrysWorktreeIsGone`: the 1782/1810 shape. Its red run reproduced the live `TREE_MISMATCH … error=<nil>`.
  - `TestVerifyExecutionTree_NamesWhyACarryWasNotReProven`.
  - `TestCarrySatisfied_SignalsACarryRecordItCouldNotReProve`.
- `adapters/ledger`:
  - `TestVerify_AStoredDiffThatIsGoneIsStillAChainBreak`.
  - `TestVerify_ResolvesALegacyCompositionLineThroughALaterChainedEvidenceRecord`.
  - `TestRestoreCompositionEvidence_RebuildsALegacyCarryFromTheGitObjectsItsLineNames`.
  - `TestVerify_APeerAppendBetweenTheChainAndTipReadsIsNotABreak`.
- `ledgerartifacts` and `gc`: the store's own tests, and `TestApply_RefusesTheLedgerEvidenceStore`.

The full list is in the CHANGELOG entry and the package pages.

## Repairing the live ledger

The plane ledger stays BROKEN at line 154453 until it is repaired. Two repairs were considered.

| Option | What it does | Cost | CLI support |
|---|---|---|---|
| (a) Anchor or rebaseline past the last pre-fix carry (ADR-0048 sidecar anchor, ADR-0081 in-band seal; the precedents are the 2026-08-10 anchor at seq 113890 and the 2026-08-11 rebaseline at seq 114871) | declares everything before the anchor operator-adjudicated | strict verification restarts at the anchor, so about 23k lines after the current anchor (seq 136212) stop being checked, including the six carries | `evolve ledger anchor <seq> --note …` or `evolve ledger rebaseline --note …` (both exist) |
| (b) Restore the diffs from git (chosen, D3) | regenerates the twelve diffs from the commits and trees their lines name, proves each against the recorded patch-id, stores it, and chains one evidence record per line | none to history: strict verification keeps covering everything after seq 136212 | `evolve ledger evidence restore` (C3; it did not exist before this change) |

**Evidence for (b).** A read-only check on 2026-10-06 found all 24 commits and trees the six records name still in the store. All 12 rebuilt diffs re-derive their recorded patch-id. The trees are unreachable objects, so `git gc` may prune them from about 2026-10-13. The restore must run before then.

**Rehearsal.** On 2026-10-06 the restore ran against a copy of the plane ledger (`ledger.jsonl`, `ledger.tip` and `ledger-anchor.json` copied into a scratch directory, with `--repo` reading the plane's git objects):

- the dry run reported six `would be rebuilt` lines and wrote nothing;
- the real run rebuilt six and appended six `composition-evidence` records;
- `evolve ledger verify` and `evolve ledger verify --deep` both reported OK, strict from seq 136212;
- a second run reported all six `durable`.

**The commands, for the operator, at a boundary.** Run them after this change lands and the plane is rebuilt (`make -C go build`, `evolve reset-sha`), with no loop running, from the plane root (`runtime/`):

```
evolve ledger verify                          # expect BROKEN at line 154453
evolve ledger evidence restore --dry-run      # expect 6 lines: 1766 1768 1772 1773 1782 1810, "would be rebuilt"; exit 0
evolve ledger evidence restore                # expect 6 "rebuilt" and "[ledger] OK: … the chain verifies"; exit 0
evolve ledger verify                          # expect OK, strict from epoch anchor entry_seq=136212
evolve ledger verify --deep                   # expect OK
```

To preview without touching the plane, run the same commands with `--evolve-dir <copy> --repo <plane root>` on a copy of `ledger.jsonl`, `ledger.tip` and `ledger-anchor.json`. Take the tip first, and retry until the copied tip names the copied last line.

If any line reports `unrestorable`, the restore exits 1 and leaves that line as it was. Fall back to option (a) for it, after reading the reason.

## Known limits

- **A defective evidence record cannot be replaced.** Restore never appends a second `composition-evidence` record for a line that already has one. So a record written with a wrong or empty digest would leave its line unrestorable, and the only remedy would be an operator anchor or rebaseline past it (ADR-0048, ADR-0081).
  - **Why it cannot happen today.** Restore appends a record only after both diffs of the line proved their patch-id and were stored, and a line with one unprovable diff gets no record at all (`TestRestoreCompositionEvidence_AppendsNoRecordWhenOneDiffCannotBeProven`). The review found one path that could have written a half-proven record: the action ranking that let an unrestorable diff rank below a rebuilt one. That test pins it. Only a future bug could write a defective record.
  - **A cheap way out, not built.** Verify already lets the later record for a line win (`indexCompositionEvidence` keeps the last record it reads per line). So restore could append a superseding record when the existing record's objects fail to prove, without rewriting history. It would need its own test (a defective record, then a proven one, and Verify resolves through the second). It is left as an option.
- **Durability covers the object and its directory, not newly made parents.** `Put` fsyncs the object before its rename and the fan-out directory after it (ADR-0123 D2). A fan-out or store directory created by the same `Put` is not fsynced in its parent. A crash test is out of reach in a unit test, because an fsync's effect only shows across a kernel crash; the in-process tests cover every write and its error paths. Consolidating the three durable writers into `atomicwrite` is filed as `durable-atomic-write-single-implementation`.
- **A missing `ledger.lock` means an unlocked read.** If `.evolve` is restored or copied without its lock file, a writer that appears mid-read can show a transient false chain break. That fails closed (a carry falls back to a re-audit). It cannot happen on the plane, whose `ledger.lock` exists and is gc-protected. Any other failure to open the lock is an error, never an unlocked read (`TestVerify_FailsLoudlyWhenTheChainLockCannotBeOpened`).
- **The anchor sidecar is read outside the snapshot.** `loadAnchorSHA` runs after `readSnapshot`, and `AnchorLine` writes without the chain lock, so an operator anchor racing a verify can show a false "anchor not found". Pre-existing.
- **A `.evolve` shared across pid namespaces or hosts.** A live writer in another namespace reads as gone (ESRCH here), so its temp file can be reaped; its Chmod or Rename then fails loudly and no line is written (fails closed). A writer this user cannot signal (EPERM) is never reaped (`TestPut_KeepsTheTempOfALiveWriterItCannotSignal`).
- **Directories newly created by `MkdirAll` are not fsynced in their parents** (`sha256/` and `sha256/xx/`).
- **Mutants no test can kill, by design.** No object fsync and a swallowed directory-sync error are unobservable: no crash seam shows an fsync, and a healthy filesystem never fails one. A reader mutex taken exclusively only serializes in-process readers, which no behavior distinguishes. A swallowed reaper `ReadDir` error is equivalent, because the directory sync on the same directory fails and surfaces it.
- **The restore only covers lines Verify checks.** Lines before the epoch anchor are operator-adjudicated and not chain-validated. Restore skips them too, so it can never refuse a line Verify ignores (`TestRestoreCompositionEvidence_SkipsLinesBeforeTheEpochAnchorAsVerifyDoes`).

## What it taught

- **A finding the review says must land with a component cannot be deferred without an inbox item.** F5 was named, deferred in one sentence, and forgotten for nine days.
- **A test helper that writes the evidence itself proves the helper, not the system.** The proof has to use the production writer, and it has to delete what production deletes.
- **"Silent decline" is a defect class of its own.** Six re-audits left no stated reason anywhere. Every rung now says why it declined (ADR-0106 asks for a loud record at every rung).

## Related

- [ADR-0105](../architecture/adr/0105-identity-preserving-fleet-rebase.md): F5, B3 and the rollout steps, corrected.
- [ADR-0123](../architecture/adr/0123-ledger-durable-evidence-segments-incremental-verify.md) and the [ledger research](../research/ledger-structure-research-2026-10.md).
- [logic-first-delivery-design §5.8](../architecture/logic-first-delivery-design.md), C8.
- Package pages: [ledgerartifacts](../architecture/packages/internal-ledgerartifacts.md), [adapters/ledger](../architecture/packages/internal-adapters-ledger.md), [gc](../architecture/packages/internal-gc.md), [phases/ship](../architecture/packages/internal-phases-ship.md), [core](../architecture/packages/internal-core.md).
- Inbox `carry-recovery-to-ship-end-to-end-proof`, which gains the acceptance line "two sequential carries with cleanup between both ship" and stays open.
