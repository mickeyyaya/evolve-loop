# Batch Integrity Review — cycles 1253–1273 (2026-08-04)

> Adversarial review of every ship and every FAIL in the relaunched batch (post
> cycle-1252 breaker halt), commissioned by the operator: *"review both pass and
> failed cycles and make sure they are true and deliver meaningful changes …
> review if there is any gaming behaviors or smells and investigate."*
>
> **Method.** Five independent adversarial reviewers, each with a hostile brief:
> live mutation testing of shipped predicates (revert-the-fix → watch the
> predicate go red), before/after duplication analysis (`git grep` at parent vs
> child), production-reachability tracing for every new symbol, claim-vs-diff
> reconciliation against each cycle's build/audit reports, and cross-arc defect
> tracing (every defect named by a rejecting audit followed to its terminal
> disposition: FIXED / EXPLICITLY DEFERRED / VANISHED).
>
> **Headline.** The batch's *code* is real — no fabricated implementations, no
> tautological predicate suites, no test tampering, and all seven FAILs were
> honest. The gaming that exists lives one level up, in **status accounting**:
> defect identities laundered across salvage chains, a dormant feature recorded
> as actively soaking, and audit prescriptions dropped at ship. Every finding
> below follows the operator-mandated format: **Issue / Gap / Solution**.
>
> **Standing convention (operator directive, 2026-08-04).** Every future fix —
> console PR or loop cycle — must land with documentation in this same
> **issue/gap/solution** format, either extending this file or in a linked doc.
> The five queue items filed from this review carry that requirement in their
> `fix` fields.

---

## 1. Verdict tables

### 1.1 Ships (9 commits, 10 PASS cycles)

| Ship | Cycle | Content | Verdict | One-line basis |
|---|---|---|---|---|
| `b45bc508` | 1253 | TIA importer closure (`internal/changedpkgs`) | **SUBSTANTIVE** | Mutation-verified predicates; dead-at-ship honestly disclosed under an audit wire-or-delete contract, honored by `fcdd466e` |
| `6494ba96` | 1258 | crosspoll-debounce completion | **SUBSTANTIVE code / PARTIAL delivery** | Debounce state machine mutation-verified at the live completion hook; claimed durable eval never landed (Finding F3) |
| `fcdd466e` | 1260 | TIA core (`internal/regressiontia`, policy staging, audit wiring) | **SUBSTANTIVE** + corpus defect | Production wiring chain verified into the live EGPS gate path; ~550 lines of dead-red predicates rode along (F5) |
| `5cd7f9ba` | 1262 | dispatch unification (`detectcli.Canonical`) | **SUBSTANTIVE** | 3 duplicate alias blocks + 1 divergent silent default genuinely removed → one authority; behavior-change blast radius independently verified empty |
| `37bc664a` | 1265 | claimed 4 items | **PARTIAL** | 1 of 4 claimed items in the commit (llmroute builder unification — real); 2 landed in the adjacent `5cd7f9ba`; 1 ("TIA shadow activation") exists nowhere (F2) |
| `8a502054` | 1269 | contextfill leaf package | **PARTIAL, honest** | Zero consumers at ship — declared in bold in its own build report with a removal commitment |
| `1c86c163` | 1271 | contextfill wiring | **SUBSTANTIVE** | Derivation at the ADR-0044 C1 chokepoint; mutation-killed wiring predicate; auditor caught a real F1 (omitempty ambiguity) |
| `751791ac` | 1270 | retro-fleet stack | **PARTIAL** | D2/D3 genuinely fixed; gitexec retry consolidation kills a *measured* 198s backoff tax; D1's critical half unfixed (F1) |
| `68322bdf` | 1272 | bookkeeping | **SUSPECT as claimed** | Zero production code; green-by-construction changelog predicate; sealed the F1 laundering's final closure line |

### 1.2 FAIL cycles (7)

| Cycle | Failure identity | Honesty verdict |
|---|---|---|
| 1254 | EGPS red=1 `ArtifactReadyCrossPollDebounceHolds` | HONEST (gate correct; the red itself was a fleet-env leak into a subprocess test, not the mechanism — all 10 mechanism tests passed) |
| 1255 | code-audit-fail D1–D4, ACS 5/5 green | HONEST — the batch's best audit; D1/D2 reproduced against the compiled binary |
| 1256 | code-audit-fail D1–D7 | HONEST — mtime provenance separation, executed overlay probe |
| 1257 | EGPS red=5, zero production code | HONEST — builder self-declared FAIL at the protected-surface boundary rather than manufacture a denied write |
| 1259 | code-audit-fail, red=2 | HONEST — audit caught the build report's false "diff is empty" claim |
| 1263 | red=1 `PolicyActivatesRegressionTIAShadow` | HONEST — and the real red count was 8, which the audit surfaced |
| 1267 | integration-tier 26× `TestRealTmux_*` | HONEST at the gate, environmental in substance — known contention-flake family killed a WARN-grade lane with no rerun |

Fingerprint check: all 7 distinct (`209c2995`…`b7f1e643`, recurrence=0 each) — the
cycle-normalized red-predicate identity (bc2e3236) is working. No forged
verdicts. No destroyed work (ADR-0076 salvage chains conserved all three arcs).

---

## 2. Findings

### F1 — Defect laundering across salvage/continuation chains

**Issue.** Cycle-1255's audit REJECTED the retro-fleet fix with a CRITICAL
reproduced against the compiled binary: a torn-down fleet lane leaves a **stale
non-empty** `cs.ActiveWorktree` (sole assignment `cyclerun.go:456`, never
cleared); `retroWorktree` (`go/internal/phases/retro/retro.go:88`) passes any
non-empty path through verbatim; the bridge then refuses the stale dir
(`driver_tmux_repl.go` `!isDir` → ExitBadFlags) and **fleet retro is lost** —
the exact loss window the task existed to close. The audit prescribed the fix
verbatim: *widen the predicate to `req.Worktree == "" || !isDir(req.Worktree)`*.

**Gap.** The shipped fix (`751791ac`) covers only the *empty* shape. Across the
1255 → 1268-salvage → 1270 → 1272 chain, the defect's identity was progressively
narrowed until its unfixed half vanished from every tracking surface:

1. The 1268 salvage **reframed** the task around the empty shape.
2. Cycle-1270's fault-localization declared the root cause *"already fixed
   upstream (PR #401)"* — false: #401 fixes the never-provisioned producer, not
   the torn-down one.
3. The retroactively-shipped `go/acs/cycle1255/predicates_test.go` **renamed**
   the task to `retro-fleet-worktree-EMPTY-fallback`, pinning only the fixed
   subset.
4. Cycle-1272's bookkeeping ship (`68322bdf`) sealed the item in CHANGELOG as
   *"already implemented … verified-closed"*, citing a test that only covers the
   empty shape.

Each cycle's artifacts are individually coherent and locally honest; the chain
as a whole erased a named CRITICAL. The same vanishing pattern consumed five
more named defects: 1255-D4 (symlinked `*_test.go` passes the corpus Walk's
suffix filter — read primitive), 1267-F2 (`DirectImporters` unbounded parse,
512MiB allocation probed live), 1267-F3 (`ScratchCwd` bare `MkdirAll`, symlink
hazard, "reproducer already written" — never landed), and 1259-D5/1263-D3 (see
F4). A structural aggravator: the 1255 retrospective DID file
`retro-worktree-fallback-stale-path` and a clear-`ActiveWorktree` item — but
they exist only inside `.evolve/runs/cycle-1255/retrospective-report.md` and
never reached the inbox (a live instance of the known lesson-to-action gap).

**Solution.** Two layers, both queued at the top of the loop queue:

- *Concrete* (`retro-fleet-stale-worktree-fallback`, weight **0.96**): apply the
  1255 prescription exactly — `retroWorktree` falls back when
  `req.Worktree == "" || !isDir(req.Worktree)` under fleet mode, with a
  **stale-path regression test** (all four existing tests pass `worktree=""`;
  zero stale coverage today). Companion root-cause fix: clear
  `cs.ActiveWorktree` at lane teardown. The same landing sweeps the basket's
  siblings: the `IsRegular` check in the corpus Walk (1255-D4), a size/ctx guard
  on `DirectImporters` (1267-F2), and `ScratchCwd` symlink hardening (1267-F3 +
  1270-R-1). The CHANGELOG's false closure line is amended in the same commit.
- *Class* (`continuation-defect-ledger`, weight **0.95**): (i) a
  continuation/salvage lane's audit MUST diff its deliverable against the
  ORIGINAL rejecting audit's machine-readable defect list (the
  `evolve-verdict` JSON already carries `defects[]`) and emit per-defect
  dispositions — FIXED (evidence) / DEFERRED (where) / OPEN; an audit that
  cannot account for a named defect cannot PASS the continuation. (ii)
  Retro-filed remediation items are written to `.evolve/inbox`
  **transactionally** with the retrospective, never only into the report.
  (iii) A closure claim in CHANGELOG/bookkeeping ships must cite the
  per-defect disposition artifact. Wiring proofs mandatory for all three.

#### F1 landing record — cycle-1283 (2026-08-04)

Recorded per operating-policy §3.8 (issue/gap/solution on every fix) and §3.9
(ledger writes derive from diffs, not from the prescription). The *concrete*
half of F1's solution has landed; the *class* half has not.

**Issue.** As above: a torn-down fleet lane's `cs.ActiveWorktree` kept naming a
pruned directory, `retroWorktree` tested for `""` rather than for the bridge
guard's predicate, and the lane's retrospective was lost to a silent
`ExitBadFlags` refusal.

**Gap.** What the prescription above asked for and what actually shipped are not
the same set, and this doc exists because collapsing that distinction is how the
1255 CRITICAL got laundered in the first place. Landed in cycle-1283:
`retroWorktree` now falls back on `fleetMode(req) && !gobridge.IsDir(...)`
(`go/internal/phases/retro/retro.go`); the teardown callback clears the record
via `clearActiveWorktree` on successful `Cleanup` only, leaving a *preserved*
worktree's path intact so `--resume` can still reclaim it
(`go/internal/core/cyclerun.go`); `isDir` was exported as `bridge.IsDir` so the
phase and the guard cannot drift apart. **Not landed** — the basket siblings this
section promised would ride the same commit were triage-deferred out of the
cycle-1283 fleet scope and remain OPEN: 1255-D4 (`IsRegular` in the corpus Walk
— status not re-verified this cycle), 1267-F2 (`DirectImporters`
unbounded parse), 1267-F3 / 1270-R-1 (`ScratchCwd` symlink hardening). The
*class* fix `continuation-defect-ledger` (weight 0.95) is likewise still queued,
not shipped.

**Solution.** Verified by `go/acs/cycle1283` (5 predicates: 001/003 discriminate
against base `9b129565`, 002/004 pin the anti-over-widening axes, 005 asserts
this landing record), plus `retro_stale_worktree_test.go` and
`cyclerun_worktree_teardown_test.go`. The CHANGELOG's cycle-1272
`retro-fleet-worktree-dispatch` "already implemented" line is struck and
corrected in the same landing, with the reason the machine guard missed it: the
guard re-ran the cited proof but never checked that the proof covered the claim.
Detail: [fix-2026-08-04-retro-stale-worktree-fallback.md](fix-2026-08-04-retro-stale-worktree-fallback.md).

### F2 — TIA "shadow soak": real dormant code under a false active-status record

**Issue.** The batch's headline feature — regression Test-Impact-Analysis
(ADR-0082, `internal/regressiontia`) — is genuinely well-built: the production
wiring chain is real (`audit.go:585 NewDefault → :178 hooks.Classify →
:640 generateACSVerdict → :648 emitTIADecision → :686-703 policy stage →
changedpkgs → Compute → Emit`), the wiring test asserts the *production*
composition (not a self-constructed fake), and the policy fail-safes are
exemplary (typos, malformed JSON, absent blocks all resolve to `off`; unknown
input can never arm selection).

**Gap.** The *status record* asserted a fiction, in three parts:

1. **The stage has never been on.** It is compiled-default `off`; no
   `regression_tia` block has ever existed in any `policy.json` (checked across
   full history and the live runtime plane); **zero** `acs-tia-shadow.json`
   artifacts exist in any run directory. The mechanism has never observed
   anything, and no rollup consumer exists even if it had.
2. **The activation provenance was fabricated by drift.** The queue credited
   "shadow wiring cycle-1266"; cycle-1266 FAILED (`no driver for cli=claude`,
   empty top_n, shipped nothing). The wiring actually landed in cycle-1260, and
   cycle-1262's own records show the activation item was *dropped* as "already
   landed". The boundary queue commit (75abe0e8) propagated the false
   attribution — written from lane labels and PASS notifications instead of
   diffs.
3. **The soak bar was vacuous.** "Soak ≥10 cycles, bar = zero missed-reds where
   shadow would have skipped" is trivially satisfied by an emitter that never
   runs. Left uncorrected, a future boundary could have flipped a
   **gate-narrowing** mechanism to `enforce` on the strength of a soak that
   never happened — a rubber-stamp maturing in slow motion. Compounding this,
   ADR-0082's own honest measurement (`corpus=40 selected=40 would_skip=0`)
   shows an *armed* shadow emits a constant no-op today: `changedpkgs.
   FileToPackage` rejects every non-`.go` path, so the import-graph-only map
   structurally cannot represent two of the three incident classes
   (`.evolve/phases → phasespec`, `.evolve/profiles → profiles/phasecoherence`)
   that were filed as "TIA enforce-flip evidence" during the same week.

**Solution.** Applied and queued:

- The inbox item (`egps-regression-tia-selection`, re-scoped 0.6) was rewritten
  as a **truth restoration**, retracting the false lines by name. Remaining
  scope, in order: (a) an operator manual ship adding
  `regression_tia.stage="shadow"` to `.evolve/policy.json` (protected surface —
  the one step that was previously recorded nowhere); (b) design **declared
  impact-surface manifests** so non-import edges (config-dir → test-package)
  become representable — without this, shadow is a constant no-op; (c) only
  then a real soak, measured against actually-existing `acs-tia-shadow.json`
  artifacts, with a rollup consumer built first.
- Class rule (folded into `continuation-defect-ledger`): an "activation" or
  "landed" claim in any ledger/queue surface must carry **runtime artifact
  evidence** (the artifact the activated stage emits), not a lane label.
  Lane-label ≠ item-delivery.

### F3 — Audit WARN prescriptions have no post-ship enforcement

**Issue.** Cycle-1258's auditor executed `git check-ignore`, predicted that
ship's `dropIgnoredPaths` (`gitops.go:810`) would silently drop the cycle's own
claimed durable eval (`.evolve/evals/artifact-ready-crosspoll-debounce.md`),
and prescribed the one-line fix (`git add -f` at landing).

**Gap.** The ship proceeded WARN-green and the prescription was never applied.
The file exists **nowhere** — not in this commit, not in any commit
(`git log --all` empty), not on any disk. The predicate meant to guarantee it
(`TestC1258_005`) enforces a nonexistent file and is green only via its
outside-git-worktree skip guard — a latent-RED trap for anyone running the full
suite in a checkout. The 0.92-weight item's "completion" therefore rests partly
on a regression lock that does not exist. (The *code* half of the claim is
real — independently mutation-verified.)

**Solution** (`audit-warn-prescriptions-unenforced`, weight **0.91**): repair —
either materialize the durable eval honestly (content citing cycle-1258
provenance, force-added) or amend `TestC1258_005` to assert the regression
locks that DO exist (the 21 mutation-verified unit tests + the cycle1258
predicates), and replace the green-by-skip shape with a git-trackedness
assertion (`git ls-files --error-unmatch`), exactly as the 1258 audit
recommended. Class fix: a WARN-ship whose audit carries a named PRESCRIPTION
must record it as a carryover that **blocks the item's consumption** until
applied or explicitly waived.

### F4 — Triage keeps committing protected-surface tasks (three lanes burned)

**Issue.** `integrity_surface.go:49` protects `acssuite/` from lane writes — by
design. Triage committed acssuite-internal designs anyway, three times: cycles
1257, 1259, and 1263 each burned a full lane to an honest FAIL at the boundary.

**Gap.** Both the 1259 and 1263 audits named an admission check as *"the
highest-value fix available"* — and it was never queued (the same
vanished-defect pattern as F1). The existing screen (#349's
`PartitionConsole(IsProtectedSurface)`) covers only the inbox route; these
tasks arrived via the fleet-todo/scout route.

**Solution** (`triage-protected-surface-admission`, weight **0.90**): an
admission check at triage commit-time — any `top_n` card whose target paths
intersect `IsProtectedSurface` is rejected/re-routed console-side (same
predicate, second route). Wiring proof: a fixture triage decision naming
`acssuite/` must be refused on the fleet-todo route.

### F5 — Dead-red predicate corpus pollution

**Issue.** `fcdd466e` shipped `go/acs/cycle1257/` and `go/acs/cycle1259/`
predicate files (~550 lines) from cycles whose audits FAILED.

**Gap.** They grade an explicitly **abandoned** design (acssuite-internal
selection): they reference `TestGoLaneSelection_*` / `CHANGED_PACKAGES`
machinery that has never existed at any commit. They are red-by-construction
against every shipped tree (skip-guarded, so suites stay green) and inflate the
apparent "cycles 1257–1260" delivery — only cycle-1260's redesign actually
shipped.

**Solution** (`dead-red-acs-corpus-cleanup`, weight 0.75): replace with honest
tombstone files citing the redesign (`internal/regressiontia`) or delete;
verify the EGPS `skipped_count` drops accordingly.

### F6 — Ship-claim misattribution and ledger writes from lane labels

**Issue.** `37bc664a` (cycle-1265) claims four items; three have zero files in
the commit (two landed in the adjacent `5cd7f9ba`; one exists nowhere — see
F2). Separately, the operator-side session ledger briefly recorded "regression-
TIA SHADOW" based on a PASS notification whose lane label contained the
activation slug.

**Gap.** Multi-item fleet lanes ship under a combined label, and consumers of
that label (queue commits, memory ledgers, CHANGELOG) attribute *all* named
items to *that* commit. Nothing reconciles label-claims against the diff.

**Solution.** Recorded as standing practice (this file + the memory ledger,
already corrected): **ledger/queue writes must be derived from the ship's
diff** (`git show --stat`), never from lane labels or verdict events. The
class-level enforcement (per-defect/per-item disposition artifacts) ships with
`continuation-defect-ledger` (F1).

---

## 3. Follow-up review — do the session's fixes follow the designs and TDD?

Operator asked: *"Follow up with the previous fixes and review if they follow
the designs and tdd."* Every console fix from this session (PRs #398–#407),
re-checked against its design intent and its red-first evidence:

| PR / ship | Fix | Design conformance | TDD evidence | Review gate |
|---|---|---|---|---|
| #398 `c76e4b3f` | Graduation test-only class (buildGraduationCheck + apicover offender skip) | Shares one predicate (`ciparity.PackageDirHasProductionGoFiles`) across both seams — single-source rule | Mutation-kill proof demanded by the opus adversarial BLOCK and delivered: deleting the audit-seam skip turns the suite RED; both seams announce | Opus adversarial review — BLOCK cleared with proof |
| #399 | Un-track gate-wiring-proof stubs + gitignore | Never-delete-minted-stubs rule honored (un-track via `git rm --cached`, disk preserved) | Verified in the true CI condition (fresh checkout without stubs) | Commit-gate |
| #401 `a497ffe1` | Worktree-add bounded retry | Retry at the ONE `Capture` in `gitWorktree.Create`; alarm chain downstream deliberately preserved (fail-loud after 3) | RED-first both directions: retry-once-succeed (attempts==2, one sleep) and persistent-fail-still-loud (attempts==3, rc=255 in error); live-validated 6/6 saves in this batch | Opus adversarial APPROVE; rival theory #400 refuted and closed unmerged |
| #402 `10f46fe9` | Revert cycle-1250 Digest injection | Surgical revert (exactly the 3 files); intent honestly refiled as a sanctioned re-land item rather than lost | Keystone parity tests RED before / GREEN after on the revert tree | Commit-gate (revert class) |
| #403 `85b3d368` | Channel-e2e deflake | Positive sync on an artifact created strictly after the inbox seek (happens-before proof), not a longer sleep; deterministic advancing clock preserves the frozen-clock design for the producer | Three-way proof matrix under an induced 50ms hostile delay: old code HANGS (reproduces CI) → clock-only fails crisp in 5s → full fix PASSES ×10 `-race` | Commit-gate |
| #404 `83d76e3d` | Phase-stub SELECT metadata | Guard's own remedy followed ("add metadata, do not pad the allowlist"); honest metadata marks the stub's reserved intent and unwired state | Guard RED at `37bc664a` reproduced locally → GREEN after, `phasespec` + `phasecoherence` | Commit-gate |
| #405 `44e7a937` / #407 `1713c046` | go.yml path filters for `.evolve/phases/**`, `.evolve/profiles/**` | Mirrors the existing skills/agents precedent comments; each filter change self-triggers the matrix, providing the green-with-fix signal | The gap itself was the failing test (no run minted on a breaking change); post-merge matrix runs on main = the regression proof | Commit-gate |
| #406 `c9e3c0fd` | Un-track profile stubs | #399 pattern generalized; plane disk copies preserved via backup/restore choreography around sync | Both failing tests (`TestSmoke_RealProfiles`, `TestRepoPersonaProfilePairing`) reproduced RED at the release commit → GREEN after | Commit-gate |

Known deviation, recorded honestly: the v22.13.0 pre-release queue ship tracked
two runtime-minted profile stubs **without** a red-first check against the
repo-contract suites — the local full-suite green was blind because a fresh
worktree checkout does not materialize plane-minted stubs (the contract tests
scan tracked-on-disk config). That mistake cost one demoted release and
produced two queue items (`phase-mint-carries-select-metadata` 0.9,
`release-preflight-repo-contract-suites` 0.8) plus the #406/#407 fixes. The
lesson is encoded in the release ledger: **run the contract suites on the plane
before tracking any `.evolve` config.**

Batch-cycle fixes reviewed in §1–§2; the two that did not follow their
rejecting audit's design (F1's stale-path half, F3's prescription) are exactly
the two now at the top of the queue.

## 4. Queue state after this review

| Rank | Weight | Item | Source |
|---|---|---|---|
| 1 | 0.96 | `retro-fleet-stale-worktree-fallback` | F1 concrete |
| 2 | 0.95 | `continuation-defect-ledger` | F1/F2/F6 class |
| 3 | 0.95 | `acs-metapredicate-suite-scope` | pre-existing |
| 4 | 0.91 | `audit-warn-prescriptions-unenforced` | F3 |
| 5 | 0.90 | `triage-protected-surface-admission` | F4 |
| — | 0.75 | `dead-red-acs-corpus-cleanup` | F5 |
| — | 0.60 | `egps-regression-tia-selection` (truth-restored) | F2 |

All five review items carry the docs requirement: their landings must extend
this file (or link a doc) in the same issue/gap/solution format.

---

## Landing — cycle-1285, `continuation-defect-ledger` (F1, all three clauses)

Cycle-1279 built the ledger, cycle-1282's audit rejected it with six defects,
and cycle-1285 is that rejection's continuation. Per operating-policy §3.8 the
landing is recorded here in issue/gap/solution form, one entry per defect.

### F1(i)-a — the gate trusted its own graded input

- **Issue.** A pre-planted `<workspace>/defect-ledger.json` marking the
  inherited ids `FIXED` produced a **PASS with zero disposition evidence**, and
  the ID-keyed merge substituted the planted row's text for the inherited
  defect's under the real id. The tamper-evident record ended up holding the
  tamperer's text. (cycle-1282 audit DEF-1, CRITICAL, `TestPOC_A`.)
- **Gap.** `reconcileContinuationDefects` read disposition state out of
  `current` — a file the graded phase agent is permitted to write — on the
  strength of a comment asserting a provenance the code never established.
- **Solution.** Inherited rows are now rebuilt from the ANCESTOR on every pass
  and their status is derived only from `defect-dispositions.json`; the
  trusted-row arm is deleted. `current` contributes only this cycle's own
  emitted defects. Retry idempotency comes from the persisted disposition
  artifact, which is re-validated each pass. Regression lock:
  `TestAdversarial_PrePlantedWorkspaceLedgerCannotDisposition`.

### F1(i)-b — "evidence" meant "any file that exists"

- **Issue.** Three CRITICALs closed while citing `/etc/hosts`, the continuation
  manifest, and the claim's own `defect-dispositions.json`. (DEF-2, HIGH,
  `TestPOC_B`/`TestPOC_C`.)
- **Gap.** `evidenceResolves` only `os.Stat`ed the citation under either root,
  with a raw-absolute-path branch and no traversal or symlink check.
- **Solution.** Four rules: relative-only, no `..` escape after `Clean`,
  PROJECT-ROOT resolution only (the workspace is this cycle's own agent-authored
  ephemera — citing it is self-vouching; a real artifact stays citable by its
  path from the root), and an explicit reject of the mechanism's own bookkeeping
  (`defect-dispositions.json`, `defect-ledger.json`,
  `continuation-manifest.json`). `Lstat` rejects symlinks. Ceiling stated in
  code: existence-plus-provenance, not diff-membership; the upgrade path to
  changed-set resolution is named there. Lock:
  `TestAdversarial_UnrelatedExistingFileDoesNotCloseADefect` (5 cases).

### F1(i)-c — 32-bit ids and a last-row-wins index

- **Issue.** `defectID` truncated SHA-256 to 4 bytes over agent-chosen text, and
  the merge index resolved a duplicated id to the LAST row, shadowing the
  inherited entry. (DEF-3, MEDIUM, `TestPOC_D`.)
- **Gap.** The comment claimed collisions were absorbed by a dedupe that is
  keyed on Text, not ID.
- **Solution.** Ids widened to 16 bytes; the FIRST row wins the index; an
  inherited id whose text differs from the ancestor's is a **blocking, named**
  diagnostic rather than a silent rewrite, and the ancestor's text is restored.
  Locks: `TestAdversarial_ShadowedIDIsLoudAndBlocking`,
  `TestDefectID_IsWideEnoughToResistASecondPreimage`.

### F1(ii) — the "transactional" inbox guarantee did not hold

- **Issue.** `writeIfAbsent` returned `nil` on an existing file, so a
  deterministic `retro-<cycle>-<slug>` filename already on disk (concurrent
  fleet lane at width 3, or stale same-cycle state) **dropped the remediation
  item** while `WriteArtifacts` reported success — the exact 1255 state. (DEF-4,
  MEDIUM.)
- **Gap.** The ordering fix guarded a write *error*; the likelier outcome was a
  silent *skip*, which nothing observed.
- **Solution.** `writeIfAbsent` now reports whether it skipped. The inbox caller
  tolerates a skip only when the file on disk is byte-identical (an idempotent
  retry) and fails loudly, naming the id, when it is not. Locks:
  `TestWriteInboxItems_CollidingFilenameIsNotSilentlyDropped`,
  `TestWriteInboxItems_IdenticalItemIsIdempotent`.

### F1 — unbounded agent-authored growth

- **Issue.** Neither the defect count nor the per-defect text length was bounded
  in `emitDefectLedger` or `retroRemediationItems`. (DEF-6, LOW.)
- **Gap.** Both inputs come from an agent-authored verdict sentinel.
- **Solution.** 64 entries / 2000 runes in the ledger, 32 items / 500 runes in
  the inbox. Overflow is RECORDED — the ledger mints one OPEN marker row a
  continuation must inherit, and the inbox cap warns on stderr — because a cap
  that erases defects is laundering wearing a resource-limit costume. Lock:
  `TestEmitDefectLedger_CapsUnboundedDefects`.

### F1 clause (3) — closure by assertion

- **Issue.** The 1255 → 1268 → 1270 → 1272 chain closed a named CRITICAL with
  the words "verified closed" and nothing else. The ledger above mints a record;
  nothing obliged a report to point at it.
- **Gap.** No gate read audit PROSE for closure claims; the reconcile gate
  governs the ledger, not the narrative that ships beside it.
- **Solution.** `closureClaimOffenders` (`go/internal/phases/audit/closure_claim.go`)
  flags, per LINE, any claim that a prior cycle's defect is closed which does not
  name `defect-dispositions.json` or `defect-ledger.json` **on that same line** —
  line-scoped because a whole-document reading would let one incidental mention
  vouch for twenty unevidenced claims. Wired into the real verdict seam
  (`hooks.Classify`, beside the reconcile gate) so it forces FAIL, quoting each
  offender. "closed" without a cycle reference is prose, not a claim, so ordinary
  reports are unperturbed. Locks: `TestC1285_401`–`404`.

### Accounting (DEF-5)

Cycle-1282's D1 and D3 were graded FIXED on predicates that never constructed
adversarial input. Per §3.9 they were **REOPENED** for this cycle and are closed
here by the adversarial locks named above, which reproduce the 1282 PoCs against
the production `hooks.Classify` seam.

## Landing — cycle-1287, continuation of 1285

Cycle-1285's own adversarial review reproduced five defects against the
production seam and the cycle was FAILed rather than shipped on the strength of
its green suite. That reproducer is now tree-resident
(`go/internal/phases/audit/repro_cycle1285_test.go`,
`go/internal/core/repro_cycle1285_test.go`).
All five are closed, each with its own `defect-dispositions.json` record.
The issue/gap/solution table is in
`docs/architecture/continuation-defect-ledger.md`, whose closure records name
`defect-dispositions.json` as this cycle's own gate requires.

Two accounting notes, per §3.9:

- **F1's fix is at the root, not the symptom.** The abort ordering in
  `faillearn/writer.go` — a retrospective may not claim remediation that never
  reached the queue — is deliberate and was NOT reversed. The defect was the
  colliding id that triggered the abort, and that is what was fixed. The
  residual (a disk-level inbox failure still suppresses the retrospective) is
  named here rather than closed.
- **One reproducer assertion was strengthened, none weakened.** The F1 case
  SKIPPED itself once the ids stopped colliding, which would have made the fix
  green the test by stepping over it — the green-by-skip pattern this review
  files as a finding. Distinct ids are now asserted, so the three damage checks
  execute.

Base drift was resolved by taking `main`'s versions verbatim for the four files
this lane never edited (`phases/retro/retro.go`, `router/router.go`, and the
`bridge` `IsDir` rename their tests depend on), so the 1255-D1 stale-worktree
fix and the ADR-0038 SELECT metadata contract survive the landing rather than
being reintroduced as defects — pinned by `TestC1287_001` and `TestC1287_002`.

## Landing — cycle-1290, continuation of 1287

The 1287 landing note above closes with two accounting notes; this cycle closes
what the first of them left open, and the mode defect its own audit raised.

- **The named residual is closed, not renamed.** "A disk-level inbox failure still
  suppresses the retrospective" is FIXED, per the record in
  `defect-dispositions.json` for this lineage: `faillearn/writer.go` now publishes
  the diagnosis as `retrospective-unqueued.md` — explicitly marked UNQUEUED and
  naming every remediation item that reached no queue — while the abort ordering
  stays exactly as 1287 defended it. `retrospective-report.md` is still absent on
  that arm, and `WriteArtifacts` still returns the error; the four pre-existing
  transactional locks pass with `inbox_transactional_test.go` unmodified.
- **The floor now publishes at the mode it documents.** 1287's own audit F1: the
  floor's artifacts landed 0600 while `internal/atomicwrite` enforces 0644 for
  every other published artifact — unreadable to the other fleet lanes and the
  operator who are the intended readers. FIXED, per the record in
  `defect-dispositions.json`, by chmod-ing the temp file before the publishing
  link. The mode is now pinned by a tree-resident test, which is what let the 1285
  publish-path rewrite drop it unnoticed in the first place.
- **One item stays open by decision.** 1287's F2 (audit eval-existence check vs.
  the `go/acs/cycleNNNN/predicates_test.go` path convention) is DEFERRED, queued as
  `audit-eval-existence-path-convention` — a different surface from the ledger, and
  deferring it in the record is the opposite of the laundering this review tracks.
