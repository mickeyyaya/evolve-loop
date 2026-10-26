# Continuation defect ledger

> Cycle-1279 · closes [batch-integrity-review-2026-08-04.md](../operations/batch-integrity-review-2026-08-04.md) **F1** bullets (i) and (ii).
> Written in the issue / gap / solution shape operating-policy §3.8 requires.

## Issue

A named CRITICAL defect raised by the audit of cycle-1255 survived the
`1255 → 1268-salvage → 1270 → 1272` chain. The chain ended with the defect recorded as
"verified closed", and nobody ever fixed it.

No step in that chain lied. Each continuation narrowed the wording of the defect,
renamed it, or declared it already-handled upstream. Each individual report
was defensible on its own. The defect was erased **collectively**, by the
accumulation of individually-honest restatements across four cycles.

The retrospective half had the same shape. The retro of cycle-1255 "filed" two
remediation items that existed only inside
`.evolve/runs/cycle-1255/retrospective-report.md`. The own queue of the loop never saw
them, so nothing downstream was able to work them. Later continuations were
free to treat the defects as gone.

## Gap

Two mechanisms were missing, both of the same kind: **the machine held no
addressable record that outlived the cycle that raised it.**

1. The structured `defects[]` of a rejecting audit lived in the report prose and in
   the `evolve-verdict` sentinel of one artifact. Nothing persisted
   them as *rows with ids*. A later cycle had nothing to reconcile against, only
   prose to re-summarize. A gate cannot diff prose.
2. `faillearn.WriteArtifacts` guaranteed that the *retrospective* survives an
   LLM-retro outage. It guaranteed nothing about the *work that the retrospective
   asks for*. "The retro was written" can be true while "the remediation was
   queued" is false, with no error anywhere.

## Solution

### 1. The ledger (`go/internal/phases/audit/defect_ledger.go`)

Both halves hang off `hooks.Classify`, the verdict seam of the audit phase. It is the same
entry that `quarantineProbesForRequest` uses. Nothing is reachable only from a helper.

Where the code lives (ADR-0103 unit 09, 2026-09-14):

- `go/internal/core/defectledger` holds the schema and the vocabulary (`OPEN`/`FIXED`/`DEFERRED`, the `PRESCRIPTION: ` prefix, the pre-flight markers, the schema example).
- It also holds the Center-free reader/writer and the gate itself (emit, arm, grade, pre-flight, the prompt block).
- `defect_ledger.go` is the seam of the audit package onto it. It holds the projections, the one wired construction, the citation resolver injected as a Strategy, and the facades.
- The failure modes of the seam are the `AUDIT_LEDGER_*` codes in [signal-codes.md](signal-codes.md).
- The design is in [decomposition/09-defectledger.md](decomposition/09-defectledger.md).

**Emit.** A FAIL whose sentinel carries a structured failure block persists
`<workspace>/defect-ledger.json`:

```json
{"origin_cycle": 1255,
 "entries": [{"id": "d1", "text": "<verbatim defect>", "status": "OPEN"}]}
```

`origin_cycle` names the cycle that *raised* the defects, so lineage survives
past the immediate ancestor. The ledger stores defect text **verbatim**. Narrowing the
text is the laundering.

A PASS, or a FAIL with no structured defects, mints
nothing. If every cycle emitted an empty ledger, every later cycle looks like a
continuation and the gate below is vacuous. Re-emission on a retry
merges by text and does not replace. Replacement is deletion by another
name.

**Prescriptions (cycle-1327, F3).** The failure block of a WARN can also carry
`prescription: []string`. It is a named remediation for a *foreseen risk*, not
something that is itself wrong. An example is cycle-1258: "run `git add -f X` or
`dropIgnoredPaths` will silently drop it". It is recommended that auditors populate this
field, distinct from `defects[]`, whenever a WARN verdict prescribes a
concrete fix that must not silently vanish.

`emitDefectLedger` mints an OPEN row for each prescription exactly as it does for defects. It tags the row
`"PRESCRIPTION: <text>"`. An operator who reads `defect-ledger.json` can then tell
"what was wrong" from "a foreseen risk's named fix" without a second ledger or
a schema-breaking `Kind` field. The reconcile/evidence gate below treats a
prescription-sourced entry identically to a defect-sourced one, with the same ids and
the same disposition rules. There is no parallel mechanism. A WARN with an empty (or
absent) `prescription[]` and no `defects[]` still mints nothing.

**Reconcile.** A cycle whose workspace holds a `continuation-manifest.json` does two things.
The manifest is the existing `internal/continuation` lineage marker, and nobody invented a parallel field.
The cycle loads the ledger of its ancestor from
`<projectRoot>/.evolve/runs/cycle-<N>/defect-ledger.json`.
It reads its own `<workspace>/defect-dispositions.json`:

```json
{"dispositions": [
  {"id": "d0f3a7c1e59b246d8a0c4e6f13579bde2", "status": "FIXED",
   "evidence": "go/internal/phases/audit/defect_ledger.go:267-356"},
  {"id": "d9c8b7a6958473625140f3e2d1c0b9a87", "status": "DEFERRED",
   "reason": "out of this lane's scope; queued as disposition-evidence-tolerant-unmarshal"}
]}
```

Every value above is **literal**, not a placeholder:

- The ids are the content-hash ids of the ancestor ledger (`"d" + 32 hex`). Copy them verbatim from `entries[].id`. Never renumber them `d1`/`d2`.
- The `evidence` cite is **bare** (`path` / `path:N` / `path:N-M`, no trailing ` (annotation)`).
- Prose belongs in `reason`.

Cycles 1397/1399/1400 each invented a different wrong shape. The schema was a list of field *names* with no document to copy.

This block is that document. The same example is carried in
`agents/evolve-auditor.md` and in `dispositionSchemaExample`
(`go/internal/phases/audit/defect_ledger.go`). The code echoes it inline whenever
the file is rejected as unparseable. There is one schema in three projections.
`defect_ledger_doc_example_test.go` holds them in sync.

`evidence` is allowed to be a single citation string **or an array of citation strings**
(`"evidence": ["a.go:12", "b.go:34"]`). The array is joined with `"; "` into the
written-back ledger, and **every** element must resolve. The tolerance is on the
SHAPE only. The gate still rejects as unparseable an `evidence` that is neither a string nor an array of strings
(an object, a number). It does not degrade it to
empty. An empty array is still an unevidenced closure.

The dispositions of the cycle must account for every inherited `OPEN` entry, and the gate checks them. An entry is unaccounted when it has no
disposition, has `FIXED` without evidence, has `DEFERRED` without a reason, or has an
unrecognized status. An unaccounted entry blocks the PASS and the gate names the offending **ids** in an
error diagnostic.

The gate writes the reconciled ledger back into the workspace of *this* cycle
before grading. The disposition is therefore visible in the own artifact of the audit, and a human does not need to infer it from a diff.
The operator can also read it on the run where the gate blocks.

`status ∈ {OPEN, FIXED, DEFERRED}`. Entries transition; they are never deleted.
**A ledger that shrinks is a ledger that launders.**

### Blockers cannot be deferred (2026-10-20)

Cycle 1573 deferred 19 of its 19 inherited defects, and both blockers of its parent were among them. The gate counted the deferrals as delivery, and wave 3 shipped 0 of 3 lanes (inbox `continuation-blocker-defects-not-deferrable`).

- **Severity at mint.** Each audit row takes a `severity` when Emit mints it. The source is the finding of the audit report whose id leads the defect text (`H1 go/x.go:3 ...` reads finding `H1` from `reportdoc.Findings`). If the report has no such finding, the letter of the id decides: `C` CRITICAL, `H` HIGH, `M` MEDIUM, `L` LOW. A defect with no leading id, and each prescription, has no severity.
- **Severity tags.** A leading severity tag decides first: `[HIGH] ...`, `HIGH: ...` or `**[CRITICAL]** ...`. A code-review row already carries its severity, so an inherited HIGH review row is a blocker too.
- **The rule.** A CRITICAL or HIGH inherited row is a blocker (`isBlocker` in `go/internal/core/defectledger/severity.go`). The gate refuses a DEFERRED disposition of a blocker as `DEFERRED but it is a <severity> blocker`, and the cycle stays blocked until a FIXED disposition with resolving evidence. A MEDIUM, LOW or unrated row can still be DEFERRED with a reason.
- **The prompt.** The inherited-defects block marks each blocker row as `(<severity> blocker: FIXED only)`.
- **The id shape.** The id is a capital letter and digits, then a space, a colon, a `*`, a `)`, a `]`, a `.` or the end. Thus `C2-foo.go: ...` is a path, not a finding id, and has no severity.
- **The overflow stand-in.** The one row that stands for the defects past the cap keeps the highest cut severity, but it names no defect to repair. Thus it stays deferrable.
- **Limit.** A ledger minted before this change has no severity on its audit rows, so its rows can still be deferred.

### 2. Transactional remediation (`go/internal/faillearn/inbox.go`)

`WriteArtifacts` gains functional options, and the three existing callers stay
byte-identical. `WithInbox(dir, items)` writes one `<id>.json` per
remediation item with `inboxbatch.Item` wire-tag parity (`id`, `title`,
`weight`, `kind`, `priority`, `files`, `injected_by`). faillearn is a leaf
package and cannot import `inboxbatch`. Parity is therefore by tag, and a test asserts it on
the raw JSON keys.

The inbox write goes **first**, and its failure aborts before the retrospective
is written. With the reverse order, the exact 1255 state ("filed" in the
report, absent from the queue) is reachable as the outcome of a partial failure.
The code deliberately does not try rollback of already-written items. `writeIfAbsent`
makes the retry idempotent, and a real remediation item that stays queued is the safe
direction to fail in.

The production caller is `Orchestrator.writeDeterministicLearning`
(`go/internal/core/failure_learning.go`). It files one item per
**self-reported** structured defect. It does not file the synthesized summary echo,
because that echo is a restatement of the failure and not an actionable item. Item weight comes
from `policy.RetroAutofileDefaultWeight()`, never a literal at the call site.

### 3. Unconditional prescription carryover (cycle-1375, F3)

The Reconcile step above is a strong gate but a **scoped** one. It fires only
when the workspace of the *current* cycle holds a `continuation-manifest.json`
that binds it to the ledger-holding ancestor. The ordinary case never forms
that binding, because triage/scout select the next lane by task content and never by ledger lineage.
The system therefore silently dropped an OPEN `"PRESCRIPTION: "` row from a WARN-shipped cycle at ship,
and no gate ever consulted it again. The own prescription of cycle-1258
("materialize `.evolve/evals/artifact-ready-crosspoll-debounce.md`,
`git add -f` past `.gitignore`") sat OPEN and unenforced across the
1233→1249→1252→1254→1258 salvage chain. It is the live instance of exactly this
gap.

`carryover.Lifecycle.MergePrescriptions` closes the gap.
It lives in `go/internal/core/carryover/workspace.go` (ADR-0103 unit 03).
The `core.MergeWorkspacePrescriptionCarryover` facade remains for tests.
The fix does not change the hardened arming logic of the Reconcile gate.

It is a cycle-terminal hook. `finalizeCycle` (`go/internal/core/cyclerun.go`) calls it inside `carryover.Lifecycle.Closeout` (memo merge → prescription merge → triage-dropped retirement).
It mirrors the tolerant-decode/dedup/TTL pattern of `Closeout` exactly. The hook does three things:

- It reads `<workspace>/defect-ledger.json` unconditionally (no continuation-manifest required).
- It keeps only `status == "OPEN"` entries whose `text` carries the `"PRESCRIPTION: "` prefix that `emitDefectLedger` already writes.
- It merges one `state.CarryoverTodos` entry per surviving row.

`carryoverTodos` is the **already-mandatory** channel that the Deliverable Contract of scout enforces when
non-empty. From the next cycle onward, every WARN prescription therefore reaches an
operator-visible decision point regardless of continuation binding. The hook leaves a
`FIXED`/`DEFERRED` prescription alone. It is already resolved and
must not re-nag.

This is deliberately a second, independent read path over the
same ledger rows. It does not widen the arming rules of `reconcileAgainstAncestor`.
That file carries a dense history of subtle anti-laundering bypass defects
(cycle-1285 F2, cycle-1282 DEF-1/2/3), and a carryover-based fix avoids
touching it.

## Degrade posture (deliberately asymmetric)

| Situation | Behavior | Why |
|---|---|---|
| No continuation manifest | silent no-op | the overwhelming majority of cycles; the green path must be unperturbed |
| Manifest unparseable | WARN, no block | cannot establish that this *is* a continuation |
| Ancestor left no ledger | WARN naming the ancestor cycle, no block | for example, an ancestor predating this mechanism. A *deleted* ledger is indistinguishable from that, so the gate records it and does not assume it is benign (D2 below) |
| Ancestor ledger unreadable | **FAIL** | a continuation cannot be graded against a ledger it cannot read |
| `defect-dispositions.json` **missing** | **FAIL** | this is the defect itself, not an environment gap. This differs from the missing-worktree case of `probe_quarantine.go`, which correctly degrades open. Degrading open here hands the gate its cheapest bypass |
| `defect-dispositions.json` unparseable | **FAIL** | same reason: write garbage, ship |
| Ledger write-back fails | **FAIL** | an invisible disposition is not a disposition |
| Ledger *emit* fails | WARN only | the ledger is a record *of* the rejection; failing to write it must not change the rejection |

## Hardening pass — cycle-1282 (D1–D7)

Cycle-1279 landed the mechanism above. Its own audit then rejected it with seven
defects. Every one of them is a way that the anti-laundering ledger can itself
launder. Cycle-1282 is the continuation that closes them.

### Issue

Before the fix, the ledger was able to lose rows quietly. It was able to mint ids that re-bound to
different text. It was able to honor a closure claim that nobody can follow. One
`rm` was able to disarm it. It skipped the WARN path entirely. The retro half filed the
synthesized summary echo as a priority-H bug and concatenated an unvalidated id
into a filesystem path.

### Gap

| # | Gap | Fix |
|---|---|---|
| **D1** (CRITICAL) | `reconcileContinuationDefects` rebuilt `merged` from `ancestor.Entries` alone and truncate-wrote it. An ordinary second `Classify` therefore erased the entries that `emitDefectLedger` had appended on the first. No adversary was required. Ids were positional (`"d"+len+1`). Any shrink or reorder re-bound an id to different text, and a disposition keyed on it closed something else. | Reconcile now MERGES onto the ledger already in the workspace, indexed by id. It adds ancestor entries and never substitutes them for what is there. Ids come from `defectID(text)` = `"d"+sha256(text)[:4]`, stable across cycles and chains. |
| **D2** (HIGH) | The ancestor ledger lives outside the workspace. A missing one returned `(nil, false)` with no diagnostic. One `rm` disarmed the gate in silence. | The missing-ancestor path emits a WARN that names the ancestor cycle and the path it looked in. It does not block, on purpose. Ancestors that predate the mechanism are legitimate and common (cycle-1279 itself left no ledger). If the gate blocks here, every real continuation breaks, and the diagnostic already removes the silence. |
| **D3** (HIGH) | Closure evidence was validated for non-emptiness after trim. `evidence:"x"` therefore transitioned an inherited CRITICAL to FIXED. | `evidenceResolves` requires the citation to name a file that exists under the workspace or the project root. It tolerates the `path:line` and `path:line:col` forms that auditors actually write. The gate reports a rejected closure by id **and** writes it back as `OPEN`. The artifact must not assert a closure that the gate refused. |
| **D4** (HIGH) | Three of the five disposition arms and the non-OPEN carry-forward had zero executing coverage. They are the headline rule of the acceptance criterion. | `TestClassify_DispositionArms` (6-case table) + `TestClassify_CarriesForwardAlreadyDispositionedAncestorEntry`. |
| **D5** (MEDIUM) | `failure_learning.go` claimed to file only self-reported defects but implemented that as `structured != nil`. A classed-but-defectless block left `ev.Defects` as the summary echo and filed it as a priority-H inbox bug. | The call site now applies `faillearn.StructuredDefects`. It is the one rule that the lesson writer already uses, exported and not restated. The lesson and the queue therefore cannot disagree about what a real defect is. |
| **D6** (MEDIUM) | Emit fired on `VerdictFAIL` only. A WARN-shipped cycle that carried structured defects left the next continuation nothing to inherit. | Emit fires on FAIL **or** WARN. It still mints nothing when the verdict carries no structured defects, so empty ledgers cannot make the reconcile gate vacuous. |
| **D7** (LOW) | `WithInbox` concatenated `it.ID + ".json"` into a path with no validation. This is safe for today's only caller, but a trap on a newly exported API. | The code rejects by name ids that are not bare filenames. It rejects and does not sanitise. A silently rewritten id yields an item that nobody can address by the id they filed it under. That is the erasure this package exists to stop. |

### Solution invariant

One sentence survives all seven: **a ledger that shrinks is a ledger that
launders.** Every fix above preserves rows and transitions status. The fixes are: merge over
rebuild (D1), record over silence (D2), OPEN over unverifiable FIXED (D3),
reject over rewrite (D7).

## F1 clause (iii) — closure by assertion

**Issue.** A closure claim was self-certifying. The 1255 → 1272 chain retired a
named CRITICAL with a bookkeeping line and nothing that a reader can check.

**Gap.** The ledger *proved* closure, but nothing obliged a human-facing claim to
reference it. The proof was a filing cabinet that nobody had to open. The earlier
draft of this section deferred the work for want of a chokepoint. It reasoned that
`internal/changeloggen` renders changelogs from commit subjects, while the 1272
line was agent-authored prose.

**Solution.** The chokepoint is the audit report itself, and it already exists:
every report passes `hooks.Classify`. `go/internal/phases/audit/closure_claim.go`
runs there, line-scoped. A claim that a prior cycle's defect is closed must
name `defect-dispositions.json` or `defect-ledger.json` on that same line.
The gate deliberately does not accept a whole-document citation. With whole-document citation, one incidental mention
vouches for twenty unevidenced claims.

**Matcher posture (cycle-1431, #448).** The original unbounded substring match
force-FAILed four narrative-PASS audits. "closed" appeared inside *disclosed* on lines
that literally ended "still open" (cycles 1339/1371/1428/1431, the last an
ADR-0072 batch halt). The matchers are now word-bounded and two-rung:

- The STRONG rung ("verified closed") is **never** guard-suppressed. An appended
  "…still open" clause must not become a one-token bypass of the citation demand.
- The WEAK rung (a bare "closed" plus a cycle reference) accepts
  negation/openness guards ("not/never/isn't … closed", "still/remains open").

The four false-RED halts bought this posture revision, and the
per-id dispositions gate backstops it. The full design record is in `closure_claim.go`. The pins are in
`closure_claim_boundary_test.go`, including the compound-line bypass try.

**Disposition.** Clause (iii) is FIXED, per the record in
`defect-dispositions.json` for this lineage. The gate is exercised by
`TestC1287_DocsPassClosureCitationGate` and its anti-neutering twin
`TestC1287_ClosureGateRejectsUncitedClaim`, and the gate grades this document itself.

## Cycle-1287 continuation — the five defects cycle-1285's audit reproduced

Per-defect dispositions (operating-policy §3.9). Each was RED in the committed
reproducer before this landing. Each is now a tree-resident lock in
`go/internal/phases/audit/repro_cycle1285_test.go` and
`go/internal/core/repro_cycle1285_test.go`.

| Defect | Sev | Issue | Gap | Solution |
|---|---|---|---|---|
| F1 | HIGH | Two ordinary defect lines that diverged only after rune 60 minted one inbox id. The DEF-4 collision refusal then aborted `WriteArtifacts` before the retrospective. The caller downgraded that to a stderr WARN. The result: no retrospective, no lesson, one item for two defects. | `remediationSlug` is lossy twice (60-rune cap *and* punctuation collapse) yet was used as an identity. | `remediationFingerprint` appends a digest of the FULL title unconditionally, so ids are injective. The ordering that made an inbox failure fatal to the retrospective is left intact. The collision was the cause, not the ordering. |
| F2 | HIGH | `rm continuation-manifest.json` closed every inherited OPEN defect with verdict PASS and zero diagnostics. The file is a 0644 file in the own workspace of the graded agent. One garbage byte did the same. | Arming depended solely on a workspace file, while every defense sat downstream of arming. | Arming is split out of grading. A corrupt manifest now BLOCKS (the rule that `readDispositions` already applied). A missing one falls back to the root-owned `continuation-registry.json`, keyed by the scope in `lane-scope.json` of this lane. Manifest/registry disagreement blocks. |
| F3 | MEDIUM | `Defect-Ledger.json` cited the own record of the gate and closed all inherited defects on a case-insensitive volume. | The self-vouching switch compared basenames exactly. `os.Lstat` two lines later did not. | `strings.EqualFold`. The rejection and the resolution now agree on what "the same file" means. |
| F5 | MEDIUM | A report that *quoted* an inherited defect, whose text contains the phrase, and declared it still OPEN was graded FAIL. | The gate matched the phrase, not the assertion. | `stripQuotedSpans`. Quoting is reporting. An unquoted assertion is claiming. The design rejected negation markers as a signal: with negation markers, appending "not closed" is a bypass cheaper than the citation demanded. |
| F4 | LOW | `writeIfAbsent` was stat-then-write. Before the fix, two fleet lanes were able to both observe "absent". The DEF-4 equality check only observed the race afterwards. | No exclusive create. | Publish by `os.Link` from a fully-written temp file in the same directory. Create-or-EEXIST is one atomic step, and no partial artifact can appear under the real name. |

**Known ceiling.** The F2 fallback needs the scope of the lane to look the binding up.
A workspace with BOTH the manifest and `lane-scope.json` destroyed is not
recoverable from the registry alone. Arming on any registry entry
blocks every ordinary cycle in a project where any lane ever preserved work. This
is recorded and not papered over.

## Cycle-1290 continuation — the two defects cycle-1287 handed forward

Third hop of the `continuation-defect-ledger` lineage (1285 → 1287 → 1290). Both
entries were RED in the committed reproducer before this landing. They are now
tree-resident locks in `go/internal/faillearn/writer_mode_test.go` and
`go/internal/faillearn/inbox_failure_degraded_test.go`.

| Defect | Sev | Issue | Gap | Solution |
|---|---|---|---|---|
| 1287-F1 (defects[0]) | MEDIUM | The failure floor published its own artifacts at 0600. The artifacts are `retrospective-report.md`, `lessons/*.yaml` and `.evolve/inbox/*.json`. Every other runtime artifact publishes 0644. | `os.CreateTemp` creates at 0600, and `os.Link` preserves it. The 1285 stat-then-write → link-publish rewrite dropped the mode silently, because nothing in the tree pinned it. | `tmp.Chmod(publishedFileMode)` on the TEMP before the link, so the published inode carries the documented 0644 of `internal/atomicwrite`. The design rejected a chmod of the destination. With a chmod of the destination, the code also rewrites the mode of a PRESERVED pre-existing artifact on the skip path. |
| 1287 residual (UNQUEUED diagnosis) | MEDIUM | A disk-level inbox failure aborted before any artifact was written. The failure ANALYSIS died with the queue write, and the next continuation had to re-derive it. | The abort ordering is correct and load-bearing. The missing piece: "do not claim the remediation was queued" had been implemented as "write nothing at all". | `preserveDiagnosis` publishes the analysis under the distinct name `retrospective-unqueued.md`. The file is marked UNQUEUED and names every item that reached no queue. The ordering is unreversed, and `WriteArtifacts` still returns the error. |

**Disposition.** Both are FIXED, per the records in `defect-dispositions.json` for
this lineage. The mode parity is fixed by `TestC1290_001` /
`TestWriteArtifacts_PublishedArtifactsHaveMode0644`. The unqueued-diagnosis
residual is fixed by `TestC1290_002` / `TestWriteArtifacts_InboxFailureWritesUnqueuedRetro`.
F2 of cycle-1287 (audit eval-existence path convention) stays DEFERRED in
`defect-dispositions.json`, queued as `audit-eval-existence-path-convention`.
It is a different surface, not this lane's.

**Why a distinct filename.** `retrospective-report.md` on disk ASSERTS that the
remediation reached the queue. That is the 1255 invariant, pinned by
`inbox_transactional_test.go`, which this landing left byte-identical (predicate
`TestC1290_004`). The design uses a distinct name. If it reused the canonical name for a degraded artifact, that lock then needs a weakening.

That is the signal that the design, not the contract, is
wrong. No reader or gate that keys on the canonical name can mistake `retrospective-unqueued.md`
for a complete retrospective.

**Known ceiling.** The code writes the degraded artifact only when a `runDir` exists.
Loop-scope fatals call `WriteArtifacts` with an empty `runDir` (no cycle workspace
exists yet). There the error is still returned, but no diagnosis is
published. The invention of a cwd-relative location is worse than no publication.
`TestWriteArtifacts_InboxFailureWithNoRunDirStillErrors` pins this, and this document records it
and does not paper over it.

## Cycle-1292 continuation — the two defects cycle-1290 handed forward

Fourth hop of the `continuation-defect-ledger` lineage (1285 → 1287 → 1290 →
1292). Both entries were RED in the committed reproducer before this landing. They
are now tree-resident locks in `go/internal/faillearn/inbox_partial_write_test.go`
and `go/acs/cycle1292/predicates_test.go`.

| Defect | Sev | Issue | Gap | Solution |
|---|---|---|---|---|
| 1290-D1 | MEDIUM | Both governed documents asserted that 1287-F2 was "DEFERRED, queued as `audit-eval-existence-path-convention`". This was false on disk: no such item existed under `.evolve/inbox`. It was an unbacked deferral claim inside the very lane that exists to catch unbacked claims. | The role guard of the audit phase denies writes outside its workspace. The audit that raised the deferral was therefore unable to file the queue entry that its own record cited. | The queue entry now exists: `.evolve/inbox/2026-08-04T22-40-00Z-audit-eval-existence-path-convention.json`. The BUILD phase filed it, because that phase holds write authority over the tracked `.evolve/inbox` path. `TestC1292_004` pins it. The test drives the real consumer `inboxbatch.LoadDir` and does not stat a filename. A file that the loader drops backs no deferral claim. |
| 1290-D2 | MEDIUM | `writeInboxItems` is not atomic across items. It writes one file per item and returns on the FIRST failure, so items before the failing one are already queued. `preserveDiagnosis` nonetheless listed EVERY configured item as "still UNQUEUED". It overclaimed which remediation was lost. | `preserveDiagnosis` held only `c.inboxItems`. It was therefore unable to tell queued from unqueued. | `writeConfig.unqueuedItems` READS THE INBOX BACK. It names only the items that are absent, or occupied by different content under our id. A read of the queue is right for the states that a failure index cannot describe. Example: a concurrent fleet lane filed an item under the same deterministic id. That item is queued. An id collision is not. |

**Disposition.** Both are FIXED, per the records in `defect-dispositions.json` for
this lineage. `1287-F2` itself stays DEFERRED. It is a different surface (the audit
eval gate, not this lane). A real queue entry now backs that deferral, and prose no longer does.

**Why read-back, not a failure index.** The claim of the degraded artifact is about
what is in the QUEUE, so the claim is checked against the queue. Any doubt
(no inbox dir, an unaddressable id, an unreadable file) reports UNQUEUED. The failure direction
stays "name work that may already be filed" over "lose work".
Re-filing an identical item is idempotent by the same-content rule of `writeIfAbsent`.
An omitted item is work that nobody goes looking for.

**Known ceiling.** The read-back is a point-in-time observation. Suppose a concurrent lane
files one of our ids in the window between the failed write and the publication of the degraded
artifact. Then the code reports that id as unqueued. The direction is the safe
one, and no cross-lane lock is worth taking on a failure path.

`WriteArtifacts`
still returns the error, and `retrospective-report.md` is still absent on every
failure arm. Accuracy is an ADDITION to failing loudly, never a replacement for
it. `TestWriteArtifacts_PartialWrite_TotalFailureNamesEveryItem` and
`..._FirstItemFailsNamesEveryItem` pin the no-under-claim half.

## Registry lifecycle: release at the adoption-decline site (ADR-0085, #428)

The root-owned registry gained its missing delete half after the 2026-08-10
absorbing-FAIL incident (cycles 1412/1418). The adopter DECLINES a binding when its snapshot is landed, base-diverged or worktree gone.
The code releases such a binding at the decline site: `releaseDeclinedBinding` → `DeleteRegistryEntryIfCycle`.
Check and delete happen under one flock hold, so the concurrent rebind of a sibling lane
survives.

Lane agents still have no write path to the registry. The
no-manifest anti-tamper block of the gate above is byte-identical. Deletion of the
WORKSPACE manifest under a live binding still blocks. Declination by the
ORCHESTRATOR now releases. Full rationale and rejected alternatives:
[ADR-0085](adr/0085-continuation-registry-release.md).

## Code-review rows (ADR-0124, 2026-10-07)

**Issue.** The audit was the only writer of the ledger, so a defect reached the record only after the gate rejected. The new `code-review` phase ([ADR-0124](adr/0124-code-review-phase.md), [design](review-loop-and-quality-index.md)) reviews the diff of the build before the audit. Its findings need the same addressable home.

**Gap.** With a second store, findings have two homes. With a text prefix (like `PRESCRIPTION: `), every reader that needs the severity, the source or the dimension parses prose.

**Solution.**
- **Four additive `omitempty` fields on each entry:** `source` (`code-review`; empty is the audit), `round` (the cycle's code-review dispatch count), `severity` and `dimension` (a quality-index dimension). Old ledgers parse and re-serialize byte-identical.
- **One merge rule for both writers.** `internal/codereview` appends the review's rows through `defectledger.Append`, the function the audit's Emit now uses.
  - Dedupe, the 64-row cap and the stand-in for a cut are each kept per source, keyed on `(source, text)`.
  - The id of a non-audit row hashes `source + "\xff" + text`, a preimage that no JSON-decoded defect text can spell.
  - A review row therefore never dedupes, cuts or swallows an audit row, and the bytes of an audit row are unchanged.
  - The stand-in takes the cut rows' status, reason, source and round, and their highest severity.
- **The row text is position-free:** `[SEVERITY] dimension location — title | scenario | evidence | fix`, without the report's `CR<n>` id, so a renumbered re-report is the same row.
- **Provenance survives inheritance.** `mergeInherited` and `gradeClaim` carry `source`, `round`, `severity` and `dimension` through whatever status the grade gives. An inherited code-review row therefore stays visible as one to the audit of the continuation.

**Shadow posture.** In the shadow stage (`workflow.findings_repair.code-review.stage`, compiled default) every code-review row is written **DEFERRED** with the stage as its reason. The Reconcile gate carries a non-OPEN row verbatim and never owes it. Its empty-ancestor check and lineage vouch count only owed rows (audit rows, and OPEN rows of any source). An ancestor that holds only shadow rows is therefore the empty ancestor, so the audit of a continuation is unaffected.

The prescription carryover reads only OPEN rows. The ledger still never shrinks.

**In enforce** (the design's §5–§6, components Q3–Q5), rows are OPEN.
- The build answers them in this cycle's `defect-dispositions.json`.
- `applyReviewDispositions` grades those answers for same-cycle rows with `gradeClaim`'s rules: FIXED needs resolving evidence, and DEFERRED needs a reason. The reconcile gate above grades only inherited rows.
- A strict deferral the reviewer rejects twice becomes `DISPUTED` (a status Q4 adds), and the audit rules on it. A row still DISPUTED at cycle close becomes OPEN, so a continuation inherits it as owed.
