# Continuation defect ledger

> Cycle-1279 · closes [batch-integrity-review-2026-08-04.md](../operations/batch-integrity-review-2026-08-04.md) **F1** bullets (i) and (ii).
> Written in the issue / gap / solution shape operating-policy §3.8 requires.

## Issue

A named CRITICAL defect raised by cycle-1255's audit survived the
`1255 → 1268-salvage → 1270 → 1272` chain and ended up recorded as
"verified closed" without anyone ever fixing it.

No step in that chain lied. Each continuation narrowed the defect's wording,
renamed it, or declared it already-handled upstream, and each individual report
was defensible on its own. The defect was erased **collectively** — by the
accumulation of individually-honest restatements across four cycles.

The retrospective half had the same shape: cycle-1255's retro "filed" two
remediation items that existed only inside
`.evolve/runs/cycle-1255/retrospective-report.md`. The loop's own queue never saw
them, so nothing downstream could ever work them, and later continuations were
free to treat the defects as gone.

## Gap

Two mechanisms were missing, both of the same kind — **the machine held no
addressable record that outlived the cycle that raised it.**

1. A rejecting audit's structured `defects[]` lived in the report prose and in
   the `evolve-verdict` sentinel of a single cycle's artifact. Nothing persisted
   them as *rows with ids*, so a later cycle had nothing to reconcile against —
   only prose to re-summarize. Prose cannot be diffed by a gate.
2. `faillearn.WriteArtifacts` guaranteed that the *retrospective* survives an
   LLM-retro outage. It guaranteed nothing about the *work the retrospective
   asks for*: "the retro was written" could be true while "the remediation was
   queued" was false, with no error anywhere.

## Solution

### 1. The ledger (`go/internal/phases/audit/defect_ledger.go`)

Both halves hang off `hooks.Classify` — the audit phase's verdict seam, the same
entry `quarantineProbesForRequest` uses. Nothing is reachable only from a helper.

**Emit.** A FAIL whose sentinel carries a structured failure block persists
`<workspace>/defect-ledger.json`:

```json
{"origin_cycle": 1255,
 "entries": [{"id": "d1", "text": "<verbatim defect>", "status": "OPEN"}]}
```

`origin_cycle` names the cycle that *raised* the defects, so lineage survives
past the immediate ancestor. Defect text is stored **verbatim** — narrowing the
text is the laundering. A PASS, or a FAIL with no structured defects, mints
nothing: an empty ledger on every cycle would make every later cycle look like a
continuation and would make the gate below vacuous. Re-emission on a retry
merges by text rather than replacing, because replacement is deletion by another
name.

**Prescriptions (cycle-1327, F3).** A WARN's failure block may also carry
`prescription: []string` — a named remediation for a *foreseen risk*, not
something itself wrong (e.g. cycle-1258: "run `git add -f X` or
`dropIgnoredPaths` will silently drop it"). Auditors should populate this
field, distinct from `defects[]`, whenever a WARN verdict prescribes a
concrete fix that must not be allowed to silently vanish. `emitDefectLedger`
mints an OPEN row for each prescription exactly as it does for defects, tagged
`"PRESCRIPTION: <text>"` so an operator reading `defect-ledger.json` can tell
"what was wrong" from "a foreseen risk's named fix" without a second ledger or
a schema-breaking `Kind` field. The reconcile/evidence gate below treats a
prescription-sourced entry identically to a defect-sourced one — same ids,
same disposition rules, no parallel mechanism. A WARN with an empty (or
absent) `prescription[]` and no `defects[]` still mints nothing.

**Reconcile.** A cycle whose workspace holds a `continuation-manifest.json`
(the existing `internal/continuation` lineage marker — no parallel field was
invented) loads its ancestor's ledger from
`<projectRoot>/.evolve/runs/cycle-<N>/defect-ledger.json` and reads its own
`<workspace>/defect-dispositions.json`:

```json
{"dispositions": [{"id": "d1", "status": "FIXED", "evidence": "go/internal/core/fleet.go:120"},
                  {"id": "d2", "status": "DEFERRED", "reason": "out of lane scope; queued as …"}]}
```

Every inherited `OPEN` entry must be accounted for. Unaccounted — no
disposition, `FIXED` without evidence, `DEFERRED` without a reason, or an
unrecognized status — blocks the PASS and names the offending **ids** in an
error diagnostic. The reconciled ledger is written back into *this* cycle's
workspace before grading, so the disposition is visible in the audit's own
artifact rather than inferable from a diff a human must run, and so the operator
can read it even on the run where the gate blocks.

`status ∈ {OPEN, FIXED, DEFERRED}`. Entries transition; they are never deleted.
**A ledger that shrinks is a ledger that launders.**

### 2. Transactional remediation (`go/internal/faillearn/inbox.go`)

`WriteArtifacts` gains functional options — the three existing callers stay
byte-identical — and `WithInbox(dir, items)` writes one `<id>.json` per
remediation item with `inboxbatch.Item` wire-tag parity (`id`, `title`,
`weight`, `kind`, `priority`, `files`, `injected_by`). faillearn is a leaf
package and cannot import `inboxbatch`, so parity is by tag and is asserted on
the raw JSON keys.

The inbox write goes **first** and its failure aborts before the retrospective
is written. The reverse order would leave the exact 1255 state — "filed" in the
report, absent from the queue — reachable as the outcome of a partial failure.
Rollback of already-written items is deliberately not attempted: `writeIfAbsent`
makes the retry idempotent, and a real remediation item left queued is the safe
direction to fail in.

The production caller is `Orchestrator.writeDeterministicLearning`
(`go/internal/core/failure_learning.go`), which files one item per
**self-reported** structured defect. The synthesized summary echo is not filed —
that is a restatement of the failure, not an actionable item. Item weight comes
from `policy.RetroAutofileDefaultWeight()`, never a literal at the call site.

### 3. Unconditional prescription carryover (cycle-1375, F3)

The Reconcile step above is a strong gate but a **scoped** one: it fires only
when the *current* cycle's workspace holds a `continuation-manifest.json`
binding it to the ledger-holding ancestor. The ordinary case — triage/scout
select the next lane by task content, never by ledger lineage — never forms
that binding, so an OPEN `"PRESCRIPTION: "` row from a WARN-shipped cycle was
silently dropped at ship with no gate ever consulting it again. cycle-1258's
own prescription ("materialize `.evolve/evals/artifact-ready-crosspoll-debounce.md`,
`git add -f` past `.gitignore`") sat OPEN and unenforced across the
1233→1249→1252→1254→1258 salvage chain as the live instance of exactly this
gap.

`MergeWorkspacePrescriptionCarryover` (`go/internal/core/prescription_carryover.go`)
closes it without touching the Reconcile gate's hardened arming logic. It is a
cycle-terminal hook — called from `finalizeCycle`
(`go/internal/core/cyclerun.go`) immediately beside `MergeWorkspaceCarryover`,
whose tolerant-decode/dedup/TTL pattern it mirrors exactly — that reads
`<workspace>/defect-ledger.json` unconditionally (no continuation-manifest
required), keeps only `status == "OPEN"` entries whose `text` carries the
`"PRESCRIPTION: "` prefix `emitDefectLedger` already writes, and merges one
`state.CarryoverTodos` entry per surviving row. `carryoverTodos` is the
**already-mandatory** channel scout's Deliverable Contract enforces when
non-empty, so from the next cycle onward every WARN prescription reaches an
operator-visible decision point regardless of continuation binding. A
`FIXED`/`DEFERRED` prescription is left alone — it is already resolved and
must not re-nag. This is deliberately a second, independent read path over the
same ledger rows, not a widening of `reconcileAgainstAncestor`'s arming rules:
that file carries a dense history of subtle anti-laundering bypass defects
(cycle-1285 F2, cycle-1282 DEF-1/2/3) that a carryover-based fix avoids
touching.

## Degrade posture (deliberately asymmetric)

| Situation | Behavior | Why |
|---|---|---|
| No continuation manifest | silent no-op | the overwhelming majority of cycles; the green path must be unperturbed |
| Manifest unparseable | WARN, no block | cannot establish that this *is* a continuation |
| Ancestor left no ledger | WARN naming the ancestor cycle, no block | e.g. an ancestor predating this mechanism — but a *deleted* ledger is indistinguishable from that, so it is recorded rather than assumed benign (D2 below) |
| Ancestor ledger unreadable | **FAIL** | a continuation cannot be graded against a ledger it cannot read |
| `defect-dispositions.json` **missing** | **FAIL** | this is the defect itself, not an environment gap — unlike `probe_quarantine.go`'s missing-worktree case, which correctly degrades open. Degrading open here would hand the gate its cheapest bypass |
| `defect-dispositions.json` unparseable | **FAIL** | same reason: write garbage, ship |
| Ledger write-back fails | **FAIL** | an invisible disposition is not a disposition |
| Ledger *emit* fails | WARN only | the ledger is a record *of* the rejection; failing to write it must not change the rejection |

## Hardening pass — cycle-1282 (D1–D7)

Cycle-1279 landed the mechanism above; its own audit then rejected it with seven
defects, every one of them a way the anti-laundering ledger could itself
launder. Cycle-1282 is the continuation that closes them.

### Issue

The ledger was a record that could quietly lose rows, mint ids that re-bind to
different text, honor a closure claim nobody can follow, be disarmed by one
`rm`, and skip the WARN path entirely — while the retro half filed the
synthesized summary echo as a priority-H bug and concatenated an unvalidated id
into a filesystem path.

### Gap

| # | Gap | Fix |
|---|---|---|
| **D1** (CRITICAL) | `reconcileContinuationDefects` rebuilt `merged` from `ancestor.Entries` alone and truncate-wrote it, so an ordinary second `Classify` erased the entries `emitDefectLedger` had appended on the first — no adversary required. Ids were positional (`"d"+len+1`), so any shrink or reorder re-bound an id to different text and a disposition keyed on it closed something else. | reconcile now MERGES onto the ledger already in the workspace, indexed by id; ancestor entries are added, never substituted for what is there. Ids come from `defectID(text)` = `"d"+sha256(text)[:4]`, stable across cycles and chains. |
| **D2** (HIGH) | The ancestor ledger lives outside the workspace, and a missing one returned `(nil, false)` with no diagnostic — one `rm` disarmed the gate in silence. | The missing-ancestor path emits a WARN naming the ancestor cycle and the path it looked in. Non-blocking on purpose: ancestors that predate the mechanism are legitimate and common (cycle-1279 itself left no ledger), so blocking would break every real continuation while the diagnostic already removes the silence. |
| **D3** (HIGH) | Closure evidence was validated for non-emptiness after trim, so `evidence:"x"` transitioned an inherited CRITICAL to FIXED. | `evidenceResolves` requires the citation to name a file that exists under the workspace or the project root, tolerating the `path:line` and `path:line:col` forms auditors actually write. A rejected closure is reported by id **and** written back as `OPEN` — the artifact must not assert a closure the gate refused. |
| **D4** (HIGH) | Three of the five disposition arms plus the non-OPEN carry-forward had zero executing coverage — and they are the acceptance criterion's headline rule. | `TestClassify_DispositionArms` (6-case table) + `TestClassify_CarriesForwardAlreadyDispositionedAncestorEntry`. |
| **D5** (MEDIUM) | `failure_learning.go` claimed to file only self-reported defects but implemented that as `structured != nil`. A classed-but-defectless block left `ev.Defects` as the summary echo and filed it as a priority-H inbox bug. | The call site now applies `faillearn.StructuredDefects` — the one rule the lesson writer already uses, exported rather than restated, so the lesson and the queue cannot disagree about what a real defect is. |
| **D6** (MEDIUM) | Emit fired on `VerdictFAIL` only, so a WARN-shipped cycle carrying structured defects left the next continuation nothing to inherit. | Emit fires on FAIL **or** WARN. It still mints nothing when the verdict carries no structured defects, so the reconcile gate cannot be made vacuous by empty ledgers. |
| **D7** (LOW) | `WithInbox` concatenated `it.ID + ".json"` into a path with no validation — safe for today's only caller, a trap on a newly exported API. | Ids that are not bare filenames are rejected by name. Rejection, not sanitisation: a silently rewritten id yields an item nobody can address by the id they filed it under, which is the erasure this package exists to stop. |

### Solution invariant

One sentence survives all seven: **a ledger that shrinks is a ledger that
launders.** Every fix above preserves rows and transitions status — merge over
rebuild (D1), record over silence (D2), OPEN over unverifiable FIXED (D3),
reject over rewrite (D7).

## F1 clause (iii) — closure by assertion

**Issue.** A closure claim was self-certifying. The 1255 → 1272 chain retired a
named CRITICAL with a bookkeeping line and nothing a reader could check.

**Gap.** The ledger *proved* closure but nothing obliged a human-facing claim to
reference it, so the proof was a filing cabinet nobody had to open. The earlier
draft of this section deferred the work for want of a chokepoint, reasoning that
`internal/changeloggen` renders changelogs from commit subjects while the 1272
line was agent-authored prose.

**Solution.** The chokepoint is the audit report itself, and it already exists:
every report passes `hooks.Classify`. `go/internal/phases/audit/closure_claim.go`
runs there, line-scoped — a claim that a prior cycle's defect is closed must
name `defect-dispositions.json` or `defect-ledger.json` on that same line.
Whole-document citation is deliberately not accepted: one incidental mention
would vouch for twenty unevidenced claims.

**Disposition.** Clause (iii) is FIXED, per the record in
`defect-dispositions.json` for this lineage; the gate is exercised by
`TestC1287_DocsPassClosureCitationGate` and its anti-neutering twin
`TestC1287_ClosureGateRejectsUncitedClaim`, and this document is itself graded
by it.

## Cycle-1287 continuation — the five defects cycle-1285's audit reproduced

Per-defect dispositions (operating-policy §3.9). Each was RED in the committed
reproducer before this landing and is now a tree-resident lock in
`go/internal/phases/audit/repro_cycle1285_test.go` and
`go/internal/core/repro_cycle1285_test.go`.

| Defect | Sev | Issue | Gap | Solution |
|---|---|---|---|---|
| F1 | HIGH | Two ordinary defect lines diverging only after rune 60 minted one inbox id; the DEF-4 collision refusal then aborted `WriteArtifacts` before the retrospective, and the caller downgraded that to a stderr WARN — no retrospective, no lesson, one item for two defects | `remediationSlug` is lossy twice (60-rune cap *and* punctuation collapse) yet was used as an identity | `remediationFingerprint` appends a digest of the FULL title unconditionally, so ids are injective; the ordering that made an inbox failure fatal to the retrospective is left intact because the collision was the cause, not the ordering |
| F2 | HIGH | `rm continuation-manifest.json` — a 0644 file in the graded agent's own workspace — closed every inherited OPEN defect with verdict PASS and zero diagnostics; one garbage byte did the same | Arming depended solely on a workspace file, while every defense sat downstream of arming | Arming split out of grading; a corrupt manifest now BLOCKS (the rule `readDispositions` already applied), and a missing one falls back to the root-owned `continuation-registry.json` keyed by this lane's `lane-scope.json` scope. Manifest/registry disagreement blocks |
| F3 | MEDIUM | `Defect-Ledger.json` cited the gate's own record and closed all inherited defects on a case-insensitive volume | The self-vouching switch compared basenames exactly; `os.Lstat` two lines later did not | `strings.EqualFold` — the rejection and the resolution now agree on what "the same file" means |
| F5 | MEDIUM | A report *quoting* an inherited defect whose text contains the phrase, and declaring it still OPEN, was graded FAIL | The gate matched the phrase, not the assertion | `stripQuotedSpans` — quoting is reporting, asserting unquoted is claiming. Negation markers were rejected as a signal: appending "not closed" would be a bypass cheaper than the citation demanded |
| F4 | LOW | `writeIfAbsent` was stat-then-write, so two fleet lanes could both observe "absent"; the DEF-4 equality check only observed the race afterwards | No exclusive create | Publish by `os.Link` from a fully-written temp file in the same directory: create-or-EEXIST is one atomic step, and no partial artifact can appear under the real name |

**Known ceiling.** The F2 fallback needs the lane's scope to look the binding up.
A workspace with BOTH the manifest and `lane-scope.json` destroyed is not
recoverable from the registry alone, because arming on any registry entry would
block every ordinary cycle in a project where any lane ever preserved work. This
is recorded rather than papered over.

## Cycle-1290 continuation — the two defects cycle-1287 handed forward

Third hop of the `continuation-defect-ledger` lineage (1285 → 1287 → 1290). Both
entries were RED in the committed reproducer before this landing and are now
tree-resident locks in `go/internal/faillearn/writer_mode_test.go` and
`go/internal/faillearn/inbox_failure_degraded_test.go`.

| Defect | Sev | Issue | Gap | Solution |
|---|---|---|---|---|
| 1287-F1 (defects[0]) | MEDIUM | The failure floor published its own artifacts — `retrospective-report.md`, `lessons/*.yaml`, `.evolve/inbox/*.json` — at 0600, while every other runtime artifact publishes 0644 | `os.CreateTemp` creates at 0600 and `os.Link` preserves it; the 1285 stat-then-write → link-publish rewrite dropped the mode silently because nothing in the tree pinned it | `tmp.Chmod(publishedFileMode)` on the TEMP before the link, so the published inode carries `internal/atomicwrite`'s documented 0644. Chmod-ing the destination was rejected: it would also rewrite the mode of a PRESERVED pre-existing artifact on the skip path |
| 1287 residual (UNQUEUED diagnosis) | MEDIUM | A disk-level inbox failure aborted before any artifact was written, so the failure ANALYSIS died with the queue write and the next continuation had to re-derive it | The abort ordering is correct and load-bearing; what was missing is that "do not claim the remediation was queued" had been implemented as "write nothing at all" | `preserveDiagnosis` publishes the analysis under the distinct name `retrospective-unqueued.md`, marked UNQUEUED and naming every item that reached no queue. The ordering is unreversed and `WriteArtifacts` still returns the error |

**Disposition.** Both are FIXED, per the records in `defect-dispositions.json` for
this lineage: the mode parity by `TestC1290_001` /
`TestWriteArtifacts_PublishedArtifactsHaveMode0644`, the unqueued-diagnosis
residual by `TestC1290_002` / `TestWriteArtifacts_InboxFailureWritesUnqueuedRetro`.
Cycle-1287's F2 (audit eval-existence path convention) stays DEFERRED in
`defect-dispositions.json`, queued as `audit-eval-existence-path-convention` —
a different surface, not this lane's.

**Why a distinct filename.** `retrospective-report.md` on disk ASSERTS that the
remediation reached the queue; that is the 1255 invariant, pinned by
`inbox_transactional_test.go`, which this landing left byte-identical (predicate
`TestC1290_004`). Reusing the canonical name for a degraded artifact would have
required weakening that lock — the signal that the design, not the contract, is
wrong. `retrospective-unqueued.md` cannot be mistaken for a complete retrospective
by any reader or gate that keys on the canonical name.

**Known ceiling.** The degraded artifact is written only when a `runDir` exists.
Loop-scope fatals call `WriteArtifacts` with an empty `runDir` (no cycle workspace
has been created yet); there the error is still returned but no diagnosis is
published, because inventing a cwd-relative location is worse than not publishing.
Pinned by `TestWriteArtifacts_InboxFailureWithNoRunDirStillErrors`, and recorded
here rather than papered over.

## Cycle-1292 continuation — the two defects cycle-1290 handed forward

Fourth hop of the `continuation-defect-ledger` lineage (1285 → 1287 → 1290 →
1292). Both entries were RED in the committed reproducer before this landing and
are now tree-resident locks in `go/internal/faillearn/inbox_partial_write_test.go`
and `go/acs/cycle1292/predicates_test.go`.

| Defect | Sev | Issue | Gap | Solution |
|---|---|---|---|---|
| 1290-D1 | MEDIUM | Both governed documents asserted 1287-F2 was "DEFERRED, queued as `audit-eval-existence-path-convention`" — false on disk: no such item existed under `.evolve/inbox`. An unbacked deferral claim, inside the very lane that exists to catch unbacked claims | The audit phase's role guard denies writes outside its workspace, so the audit that raised the deferral could not file the queue entry its own record cited | The queue entry now exists: `.evolve/inbox/2026-08-04T22-40-00Z-audit-eval-existence-path-convention.json`, filed by the BUILD phase (which does hold write authority over the tracked `.evolve/inbox` path). Pinned by `TestC1292_004`, which drives the real consumer `inboxbatch.LoadDir` rather than stat-ing a filename — a file the loader drops backs no deferral claim |
| 1290-D2 | MEDIUM | `writeInboxItems` is not atomic across items: it writes one file per item and returns on the FIRST failure, so items before the failing one are already queued. `preserveDiagnosis` nonetheless listed EVERY configured item as "still UNQUEUED", overclaiming which remediation was lost | `preserveDiagnosis` held only `c.inboxItems` and so could not tell queued from unqueued | `writeConfig.unqueuedItems` READS THE INBOX BACK and names only the items that are absent, or occupied by different content under our id. Reading the queue — rather than slicing at the failure index — is what makes it right for the states an index cannot describe: an item filed by a concurrent fleet lane under the same deterministic id is queued; an id collision is not |

**Disposition.** Both are FIXED, per the records in `defect-dispositions.json` for
this lineage. `1287-F2` itself stays DEFERRED — a different surface (the audit
eval gate, not this lane) — and that deferral is now backed by a real queue entry
rather than by prose.

**Why read-back, not a failure index.** The degraded artifact's claim is about
what is in the QUEUE, so the queue is what the claim is checked against. Any doubt
— no inbox dir, an unaddressable id, an unreadable file — reports UNQUEUED, so the
failure direction stays "name work that may already be filed" over "lose work";
re-filing an identical item is idempotent by `writeIfAbsent`'s same-content rule,
while an omitted item is work nobody goes looking for.

**Known ceiling.** The read-back is a point-in-time observation. A concurrent lane
that files one of our ids in the window between the failed write and the degraded
artifact's publication would be reported as unqueued; the direction is the safe
one, and no cross-lane lock is worth taking on a failure path. `WriteArtifacts`
still returns the error and `retrospective-report.md` is still absent on every
failure arm — accuracy is an ADDITION to failing loudly, never a replacement for
it (`TestWriteArtifacts_PartialWrite_TotalFailureNamesEveryItem` and
`..._FirstItemFailsNamesEveryItem` pin the no-under-claim half).
