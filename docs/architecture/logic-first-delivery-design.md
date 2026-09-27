# Logic-first delivery — design document

- **Status:** living document, kept current with every landing. Last updated 2026-09-27 00:30.
- **Decision record:** [ADR-0106](adr/0106-logic-first-delivery.md). **Policy:** [operating-policy §0](../operations/operating-policy.md).
- **Landings:** the design in #655 (merged `5600b77a`; it replaced #653 after a CHANGELOG conflict); the first code train in #656 (merged; eight commits, replacing #654 the same way); ADR-0105 B1 in #652 (merged `e5af27fe`).
- **Owner of the request:** the operator. **Priority:** P0; everything else parks.
- **Research:** [Self-recovering agent loops and Claude Code fleets in 2026](../research/self-recovering-agent-loops-2026.md) — the state of practice this design is compared against, with recommendations R1–R14 mapped onto these components.

## 1. The request

Three statements, verbatim, in the order given on 2026-09-26.

1. "I want each phase to focus on LOGIC, if the failed reason is related to 'format' or process related errors, the pipeline should try to recover it, not blocking. Because the most important deliverables should be the logic to solve the particular issues, building features, completing the quests, building the architecture that sustains through different changes, having the flexibility to adjust through multiple changes requests while maintaining the same quality. Make this as the highest priority and policy for refactory the pipeline."
2. "I want you to put this refactory quest as the P0 and drop everything to make it happen first. Focus on the logic, other format related errors can be fixed through recovery agent to help structure the deliverables around the core logics."
3. Goal as set: "prioritize the logic in code not the format and process, the pipeline should assist and check and recover it to its original code intention, the delivered code and doc should speaks for itself, if agent made error by following the format / structure, we should assign it back or recover if it delivers enough evidence for building / fulfill the request. ultrathink to design and architecture"

Standing constraints that shape the design: every fix and feature is decomposed into small components, each tested and landed on its own; config over Go literals; no feature flags; one source of truth, never a duplicate; pipeline integrity outranks throughput; a passed audit is trusted and never blocked by a process issue.

## 2. Goals and non-goals

**Goals**

- A cycle whose logic is right ships even when a deliverable's form or a process step is wrong.
- Every block names a defect in the change's behaviour, or is an integrity block; nothing else is final.
- Recovery is cheap, bounded and honest: it regenerates projections of existing evidence and never substitutes for missing logic.
- The pipeline decides between assigning work back and recovering it on evidence the kernel computed, never on an agent's account of itself.
- The delivered code and its explanation document speak for themselves; reports are projections of them.

**Non-goals**

- Softening any judgment. An audit FAIL on substance, a red predicate, a security finding: all stay final.
- Recovering an audit FAIL on narrative fidelity (a report that misdescribes the diff). That is a judgment; a later ADR may revisit it with evidence.
- Changing the gate's standard of well-formedness. Recovery changes who writes a file, never what passes.
- Unbounded retries. Every rung is budgeted from config.

## 3. Vocabulary

| Term | Meaning |
|---|---|
| **Logic block** | A defect in the change's behaviour: a failing or missing test, a regression, a security finding, an audit finding on substance. Final. |
| **Form or process block** | The deliverable's shape, a derivable file, a binding to a tree that moved, a race, a transport hiccup, bookkeeping. Recovered; final only when recovery is exhausted. |
| **Integrity block** | Evidence that the kernel's account of the cycle cannot be trusted: the treefence, the predicate-authority fence, ADR-0072 incoherence, a sandbox or recovery-guard violation. Never recovered: the cycle aborts and a P0 is filed. |
| **Evidence** | What the kernel computed or owns: the diff against the worktree base, the host's own test runs, a parse of a document's shape, the inbox item's acceptance, the base SHA. Never an agent's self-assessment. |
| **Projection** | A report or secondary file that restates evidence in a contracted shape (`build-report.md`, `triage-decision.json`). A defect in a projection is form. |
| **Recover** | Regenerate a projection from evidence, toward the original intention (the task contract's acceptance, the intent document). |
| **Assign back** | Re-dispatch the phase's owner on its preserved worktree, keeping the diff and every green run, with a correction that names the missing evidence, never the missing format. |
| **Rung** | One step of the correction ladder: salvage, live-fix (dormant), recover, re-dispatch. |
| **Decision-bearing field** | A field only the phase's owner may author: a verdict line, `## AC-Materialization`, the deliverable-kind and cycle-size headers, `top_n`/`deferred`/`dropped`. |

## 4. Principles (operating-policy §0)

1. Three kinds of block; only logic and integrity are final.
2. Recovery is layered, cheapest first: a host derivation before any judge → deterministic rungs → the recovery agent → a re-dispatch.
3. Recovery never launders: the gate that rejected a deliverable judges its repair; the verdict is re-earned, never carried; judgment and control phases are never repaired; the kernel proves nothing outside the grant changed and that every added line has a source; every recovery is recorded where the auditor and the dossier see it.
4. The evidence decides between assigning back and recovering. A missing deliverable is always assigned back.
5. Bounded and loud: recovery rounds come from config; exhaustion is recorded as a pipeline defect and still counts toward the ADR-0072 halts.

## 5. Architecture

### 5.1 The path of a form failure

```
phase dispatch (bridge, sandboxed)
   │
   ▼
host step ─────── effects (inbox claim) ──── derivations (H2: triage-decision.json from the report)
   │
   ▼
runner judge ──── classifies the verdict from the deliverable
   │
   ▼
contract gate ─── whole violation set ──┬── any logic or integrity member ──► back as a whole (as today)
                                        │
                                        └── all form, phase qualifies (registry `recovery` block)
                                                │
                                                ▼
                                    evidence.Sufficient (E1)  ── kernel inputs only
                                                │
                            ┌───────────────────┴───────────────────┐
                            ▼                                       ▼
                     Missing non-empty                         Sufficient
                     ASSIGN BACK (E2):                         LADDER (F1/F6):
                     re-dispatch the owner on its              salvage ─► live-fix (dormant) ─► recover (agent,
                     preserved worktree; correction            fenced by F2, provenance-checked) ─► re-dispatch
                     names `Missing`, never a code                     │
                                                                       ▼
                                                            breaker-neutral re-check ─► one review ─► V0 verdict refresh
                                                                       │
                                                                       ▼
                                                                    audit (sees the recovery: L1)
```

A missing or empty primary never enters the ladder: it is assigned back, because the pipeline cannot tell a finished phase that forgot its report from one that stopped early, and for a contracted phase the report is where the owner declares the outcome.

### 5.2 The process ladder at ship (ADR-0105)

A passed audit meets a moved `main` at ship for reasons that touch none of its files: a peer's landing, a sibling's closeout dossier. ADR-0105's rungs (B1 unwind-rebase-pend, B2 explanation rebind, B3/B4) carry the verdict across a tree change that provably did not touch the change; P2 keeps a fleet lane's dossier out of `main` until the wave boundary. These are deterministic and have no LLM.

### 5.3 What speaks for itself

The change is the code, its tests and its explanation document (ADR-0102). Everything a phase writes about the change is a projection. Recovery may regenerate a projection; it never touches the change itself, which lives in the fenced worktree.

### 5.4 The agent's identity and its pane (P3)

An agent that inspects its environment must recognise its own traces. Cycle 1707's tdd agent listed the tmux sessions, found its own, read its own prompt file, and refused the phase as a prompt injection racing "the real agent"; the operator's one-line clarification an hour later resumed it. The block that says that line before the agent needs it is [`internal/bridge/phaseidentity`](packages/internal-bridge-phaseidentity.md).

- **Finished by the driver, appended.** The engine composes the prompt before a session exists; only the tmux driver knows the session name, so `prepareTmuxREPL` appends `phaseidentity.Block` to the bytes it pastes. It goes after the composed prompt so the engine's bytes stay a byte-identical prefix (the cached skill and policy blocks hold across dispatches) and the deliverable path stays the last thing the agent reads. `resolved-prompt.txt` is the exact pasted bytes, so the block is auditable per phase.
- **In the prompt, not the system prompt.** `--append-system-prompt` exists for claude only; the block is for every tmux CLI and lands in the same bytes for each.
- **Null object, and only true claims.** No agent name or no pane → no block; a phase whose answer the bridge reads from the pane (`completion: stdout`) gets no sole-writer claim, because a statement meant to stop an agent from doubting its own facts must never contain a false one.
- **Suggestions off through the environment.** claude's prompt suggestion is a background model request per turn and dim text under the input box that reads like agent output. `CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false` takes precedence over the setting and, unlike a `--settings` flag, is honoured under the profiles' `--setting-sources project`. The channel is the manifest's `default_env`, realized once (`Realization.Env`), exported in the pane by the tmux boot and passed to the process by the headless drivers; keys are validated as shell identifiers at parse, the loop's, the bridge's and the credential variables are refused, and every fact rendered into the block is stripped of control bytes and backticks (security review, 2026-09-27).
- **What the block does not fix.** The repository's CLAUDE.md is written for console operators and a phase agent reads it; the block's last line answers that in-prompt, and the content itself belongs to the target repository. The twelve profiles that repeat the same claude flag list are a later centralization into the manifest's `default_args`.

### 5.5 Capacity is not a verdict (the wave-14 deep-dive)

Wave 14 (cycles 1708 and 1709) failed with every CLI family walled: claude-tmux answered `auth_recheck` fourteen times (a credential wall; the operator's `/login` cleared it), codex-tmux `rate_limit` eight times and `model_unsupported` six (the deep pin the account rejects), ollama-tmux refused the source-writing phases. No logic ran. The orchestrator already treats a first-dispatch exhaustion as a **deferral**: `pauseForQuota` emits `quota.paused`, writes the quota checkpoint, records the abort with the `all-families-exhausted` prefix that `cyclehealth` classifies DEFERRED, skips failure learning, and the batch returns rc 5 so the chain waits with the checkpoint intact (1709 took this path). Three seams do not reach that one function:

| Id | Component | What is wrong today | Design |
|---|---|---|---|
| Q1 | exhaustion during a correction re-dispatch is a deferral | `cyclerun_correction.go` wraps `runner.Run`'s `ErrAllFamiliesExhausted` as "correction N dispatch failed", records a FAIL outcome, writes a failure lesson and a digest, and lets the retrospective dispatch (which hits the same wall); 1708 was sealed FAIL this way | the correction loop, the remediation gate re-run and the resume review gate check `errors.Is(err, ErrAllFamiliesExhausted)` and return through `pauseForQuota` — one seam, one classification; a test per caller pins the DEFERRED prefix, the `quota.paused` signal, the `all_families_exhausted` ledger kind and the absence of a lesson |
| Q2 | a credential wall benches the family and names the operator | `clihealth.Benchable` benches `rate_limit`/`exhausted` only, so `auth_recheck` re-dispatched to claude fourteen times in one wave and the halt read as quota | `auth_recheck` benches the family with the same strike-scaled cooldown a quota wall gets (30 min doubling to 4 h) but stays active for routing until a probe clears it — the cooldown only schedules the canary's next probe, and the operator's login makes that probe succeed; a login pane's stale reset hint never sets the bench; the fix names no login command, because the families' logins differ (agy trusts a directory, ollama signs in). The fix is durable on the bench entry (`operator_action` in `cli-health.json`), and the chain walker's bench line and the canary's re-bench line print it. Trade-offs taken: the canary spends one probe per cooldown (30 min → 4 h) against a CLI that needs a login until the operator logs in; a bench is advice to the chain, not a veto, so a judgment phase the balanced-tier floor pins to claude still dispatches into the wall and then defers through Q1. Not built: a Signal Center ERROR code for the wall and the loop's defer message distinguishing "wait for the operator" |
| Q3 | a deferred cycle never counts toward the halts | the consecutive-failure breaker reads every `failure-digest.json`; a deferral must never write one (1709 did not; 1708 did, through Q1's defect) | a test pins that no digest exists after a deferral through every seam Q1 covers, and the zero-ship halt rule counts deferred lanes as neither shipped nor failed |
| Q4 | a fallback chain never appends a CLI that cannot serve the phase | the universal fallback appended ollama-tmux to the retrospective chain; it refused the source-writing phase with exit 10 and the refusal became the retro's FAIL | the chain builder filters candidates by the phase's requirements before appending them; a refusal is impossible by construction |

Q1–Q3 land before the next soak wave; Q4 is a routing hygiene follow-up. None of them changes a verdict: a capacity wall stays a deferral, a credential wall becomes an operator halt, and the FAIL streak counts logic.

### 5.6 A pin without an inbox item (the wave-15 deep-dive)

Wave 15 (cycles 1712 and 1713) was the first wave on the P3/Q1/Q2 train. Lane 1713 was pinned to `gittest-fixture-centralize`, sealed `SKIPPED_UNKNOWN` with `triage-empty-commitment` and shipped nothing, and the same pin had already cost 1709 and 1710. The chain, from the run dirs:

| Cycle | Lane pin | Decision | End |
|---|---|---|---|
| 1706 | `tempdir-cleanup-vs-git-flake` | `top_n: [gittest-fixture-centralize]`: an id triage authored, with no inbox item behind it | shipped 51edfb7f; the promote no-op'd ("not found: already moved?") |
| 1709 | `gittest-fixture-centralize` | `top_n: [fromgit-test-gittest-migration, treestate-test-gittest-migration]`: two more authored ids | deferred on capacity (exit 85) |
| 1710 | `gittest-fixture-centralize` | none | deferred |
| 1713 | `gittest-fixture-centralize` | first report: `## superseded` with nothing under it, declined ("states neither cards nor none"), so `missing_secondary` and a correction; second report: `dropped: already-shipped: 51edfb7f` | planned no-work |

Two defects, one of form and one of lifecycle, neither of logic:

- **H1b: an empty optional bucket is no cards.** The strict reader (H1) accepted a present bucket only when it stated cards or `none`; a heading followed directly by the next heading is what an agent writes when it has nothing to say, and declining it turned a complete answer into a gate rejection and a re-dispatch. `deferred`, `dropped` and `superseded` left empty now derive to `[]`; `top_n` left empty is still a missing commitment and still declines, because an absent or empty `top_n` must never read as "commit to nothing" by accident (the invariant H1 already pinned for the absent case).
- **W1: an id no inbox item backs leaves no retirement evidence.** Every retirement door tolerates such an id by doing nothing: `Promote` no-ops ("a missing item never blocks the ship"), `RecordRootTaskFailure` no-ops, `ClaimLaneScope` skips with a WARN, and the no-work closeout treats it as "simply gone". Every reader that decides whether to run the id again reads the same inbox lifecycle and, by design, keeps an id with no evidence: the wave planner's prune fails open (over-pruning starves the wave) and the launcher's freshness probe calls it fresh (not every planned id is inbox-backed). So the moment triage authors an id and the cycle ships it, nothing in the system can ever say the id is done, and the prior-decision carry re-pins it into a lane every wave until a lane's triage happens to drop it, and even that drop reaches no reader. Three lanes went to it in one soak.

The design keeps one source of truth. The two doors where such an id is finished write the evidence the readers already consult, as a record in the retirement dir `Promote` would have moved the item to:

| Door | When | Record | Reason |
|---|---|---|---|
| the PASS seam (`ApplyCycleOutcome`, through ship's post-commit closeout) | a committed id `Promote` could not move and no inbox item backs | `processed/cycle-N/<sha8>-<id>.json` | `ship-promote-processed: no inbox item backs the id` |
| the planned no-work closeout (`ApplyNoWork`) | every scoped id no inbox item backs, answered or not | `rejected/cycle-N/<id>.json` | the lane's own answer (`lane triage (cycle N) dropped: already-shipped: 51edfb7f`) or `the lane's triage answered for nothing` |

`inboxmover.RetireUnbacked` is the one door and writes only for an id whose `ResolveDispatchState` is unknown, so an inbox-backed item keeps its own lifecycle (a no-work lane routes it to the console in place and never retires it: the anti-laundering rule stands) and an already retired id is never written twice. The record carries `id`, `unbacked: true`, `retired_reason`, `retired_cycle` and the ship sha, is found by `FindFileByTaskID` like any item, and the plan-time prune and the dispatch-time probe drop the id on their next read. A FAIL writes nothing: the id must stay retryable.

What W1 does not fix: a failing authored id still has no durable failure count, so the S5 retry ceiling cannot reach it (W2, designed). The lane pin that started the chain (1706 committing an id other than its pin) is triage's choice and stays legal; the retirement makes it finite.

**X1: a host-owned inbox record is not the builder's to explain.** Lane 1712 (the third cycle on `lineage-datestamp-normalization`, after 1707 and 1708) sealed FAIL with the code verified: ACS 10/10, EGPS red 0, a mutation probe that catches a partial fix. The one defect was a sentence in the explanation document. The chain: the build floor's material-path set included `.evolve/inbox/2026-08-05T15-30-00Z-lineage-datestamp-normalization.json` (twelve corrections across the lane's three cycles demanded that path), the builder wrote "content unchanged" for the item it moved to `consumed/`, the ship's in-commit consumption then stamped that file with a `consumed` block (`via: ship`), and the auditor's explanation review found the sentence contradicted by the diff (`NEEDS_CORRECTION`), which ADR-0102 turns into the audit's FAIL. The builder was made to describe an effect the host produces after the build and was judged on it. `nonMaterialPrefixes` exempted `.evolve/runs/`, `.evolve/worktrees/` and `.evolve/evals/` and not `.evolve/inbox/`, whose records the host claims, moves, stamps and retires; it now does (`TestMaterialPaths_InboxLifecycleRecordsAreTheHosts`). The auditor reads the same host-derived set, so the review has nothing to demand for the path either.

**X2 (designed): an inaccurate explanation of a verified build is a form failure.** ADR-0102 made the auditor's honest `NEEDS_CORRECTION` force the audit's FAIL so that the document could not be waved through. Under this policy the judgment stays and the verdict changes: when the code is verified and the review names only the document, the correction is a recovery rung on the document (the recovery agent of F4/F6, or the host for a mechanical claim such as a rename's similarity), re-reviewed, and the cycle ships; only a review that names the code blocks. X1 removes the manufactured case; X2 removes the class.

### 5.7 A card on a protected surface is a route, not a verdict (wave 16, cycle 1714)

Wave 16's first cycle on the W1/H1b/X1 main was pinned to `file-size-decomposition-backlog`, an inbox item with no declared files. Its triage named `go/internal/core/orchestrator.go` in the card, the classify hook refused the report with `TRIAGE_PROTECTED_SURFACE`, the cycle sealed FAIL after two phases, and the closeout's per-item breaker (F30) routed the item console-manual. The item did go where it belongs; the verdict did not. Three facts made it a class, not an accident:

- **The planner could not judge it.** The wave planner's protected predicate and the prompt partition judge an item's declared files; an item with none is disjoint with everything and always pickable. The surface is known only once triage names files.
- **The agent was never told.** The triage persona did not mention protected surfaces, and the prompt listed only the console-routed items excluded from its menu. The agent's only way to learn the rule was the refusal.
- **The refusal was a verdict.** A deterministic, operator-owned routing fact (the same card refuses every time, and the console decides) ended the cycle as the task's FAIL: no logic was judged, the streak broke, and the item's route came from the failure path.

The design keeps everything F30 built and moves the decision one step earlier, into the host's hands:

- **R1: the classify hook records the route instead of refusing.** Every top_n card whose `files=` names a lane-forbidden path (`protectedTopNCards`, all of them, in report order) leaves `top_n` and joins `escalate_block` with the reason `protected-surface: <path> — control-plane changes go through the console route (operator-gated), not lane top_n` (`routeProtectedCards`, a rewrite of `triage-decision.json` that keeps every other key and is idempotent). The hook returns PASS with one warning per card (`TRIAGE_PROTECTED_SURFACE`, severity warning, subject the card). From there the existing machinery does the rest, unchanged: an emptied `top_n` is the planned no-work end (`HasEmptyTriageCommitment`, F30), the escalation is the lane's answer (`committedset.Dispositions`), and the no-work closeout routes the item console-manual with that reason (`ApplyNoWork`); a `top_n` that still holds other cards continues the cycle on them, so the lane is not wasted. A route that cannot be recorded fails closed with the same code as an error, and so does a card the host cannot name (no id) or one the decision never committed (the report and its derived decision disagree): a protected card must never reach the spine, and the rewrite proves every routed card left `top_n` or was already escalated. The host effects run before the judge, so the decision file exists when the hook rewrites it.
- **R2: the agent is told.** The triage prompt's inbox section names the protected surfaces (`guards.ProtectedSurfaceManifest`, the one list) and the drop reason `protected-surface: <path>`; the persona's drop rule carries the same sentence. An agent that follows it never produces a card the host has to move; one that does not is corrected without a verdict.

`RefusalDisposition(TRIAGE_PROTECTED_SURFACE)` stays as it was (task-level, route console): it now governs only the fail-closed case, where the code still travels on a FAIL.

### 5.8 The carry: a byte-identical rebase ships without a second audit (P1 = ADR-0105 B3 and B4)

Cycles 1712 and 1715 each passed their audit, met a moved plane `main` at ship (a sibling's `dossier: cycle-N closeout` commit, the dominant contention with two or more lanes), unwound (B1), rebased byte-identically, rebound their explanation (B2), and then re-audited an unchanged tree for seven to ten minutes, with a second chance for the verdict to flip. The re-audit is the rule today, not an accident:

- `routeRebasedExplanation` returns Audit after the rebind and never consults the carry-forward rungs; they run only for a cycle without the explanation contract, which no cycle is any more.
- The existing RUNG 0 (`compositionCarryForward`) diffs commits (`git diff main...HEAD`), and after B1 the change is pended in the index, so the composed diff is empty and the patch-id can never match.
- Ship has no reader of carry records: `internalAuditBoundTreeSHA` must equal the staged tree (or differ only by sanctioned inbox consumption), so even a carried verdict would be refused as tree drift.

The design is ADR-0105's, in four components, each its own commit, unwired before wiring:

| Id | Component | What it proves or does |
|---|---|---|
| C1 | `treedelta.Delta(base, tree)` and `treedelta.Identical(base0, T0, base1, T1)`, a leaf both core and ship use (core's `treeDelta`/`identicalChange` delegate) | the byte-exact change (`diff --binary --full-index --no-ext-diff --no-textconv --no-renames`) of the audited tree on its base equals the pended tree's change on the new base, and is not empty |
| C2 | the carry record: `composition-verdict{method:"identical-rebase"}` gains `AuditedTreeSHA` beside `TreeStateSHA` (the composed tree) | ship can match the record to the audit it carries (`LaneAuditRef` = the audit artifact's SHA), the audited tree it binds and the tree it will commit |
| C3 | `identityCarryForward` after the rebind (B3): the auditor row of the run names T0 and the audit artifact (`latestAuditEntry`), base0 = the base before the rebind, T1 = `git write-tree`, base1 = `HEAD`; C1 holds; the composed-tree gates run under `treefence` and the index must still be T1 afterwards; the record goes to the root ledger ship reads; return Ship, else Audit | a rebase that changed no byte of the change ships on the verdict it already earned |
| C4 | ship accepts a carry (B4): inside the one binding rule (`auditBindingSatisfied`), when the tree ship holds is not the bound tree, the newest `identical-rebase` record for this audit (`LaneAuditRef` = the audit artifact's SHA) whose `AuditedTreeSHA` is the bound tree and whose `TreeStateSHA` is the tree held is re-proven: the ledger chain verifies (0.4 s on a 147K-line ledger), both bases' ancestry in the tree's own directory, C1's byte equality, and the record's patch-id is the proven bytes'; the plane-HEAD comparison stays for the resume detection | ship never trusts the writer, and a carry cannot launder a change |

What it does not change: a rebase the proof declines still returns to Audit (or Build when the change is not identical), and the audit-bound tree of a cycle that never rebased is checked exactly as today.

One scoping, stated: ship re-proves the ancestry and the bytes itself and verifies the ledger chain the record sits in, but does not re-run the composed gates (build, test, acs, apicover) a second time; the writer refuses a record whose gates are not green, and the chain proves the record came through the writer, so a ship-side gate check would be dead code. The orchestrator also declines a carry when the auditor row was bound on another base than the one the change was authored on. Re-running the gates in ship (C4b) is a follow-up if a forged, re-chained ledger is ever a credible threat.

### 5.9 The cycle's own record is not history (wave 18, cycle 1718)

Cycle 1718 (`profiles-test-hygiene`) built a test-only change — five `internal/profiles` test files, a doc, an eval and its ACS predicates — and explained it: the builder wrote the cycle's own explanation document at the canonical path and declared it. The contract classes unambiguous test files as non-material, so the host's material set was empty, and the build floor's no-material branch refused the handoff with `not-applicable Build changed immutable cycle explanation record(s): docs/explain/builds/cycle-1718-….md`, naming the document the build had just written as if it were published history. The correction rung re-dispatched the builder (a codex deep launch that bounced on `model_unsupported`, then claude deep) to delete its own document and declare `NOT_APPLICABLE`; the second handoff passed. The cycle paid a full builder dispatch to remove a document that contradicted nothing.

The defect is an asymmetry, not a policy: the material branch already excluded the cycle's own path from the immutability rule (`foreignHistoryFailures(paths, current)`), the no-material branch did not (`changedCycleRecords(paths)`), and the message called the builder's deliverable immutable. Under §4 the facts are: the diff is non-material (the host's judgment, from Git); the record is the cycle's own (its path binds the cycle and run id, and it did not exist at base); the builder declared it. No material change is hidden and no published record is rewritten, so nothing may block.

X3 makes immutability one rule in every branch and lets the declaration choose the path:

| The diff | The cycle's own record | The declaration | Before | Now |
|---|---|---|---|---|
| material | present | REQUIRED | verified | verified (unchanged) |
| non-material | absent | NOT_APPLICABLE | N/A | N/A (unchanged) |
| non-material | present | REQUIRED | refused as history | verified like any explanation, with an empty material set (the builder explained a non-material change; `Changed Areas` may cite only paths in the diff) |
| non-material | present | NOT_APPLICABLE | refused as history | N/A; the undeclared record rides as documentation (the cycle's own, newly added, binding no verification — its content is not validated and no review is owed to it; once landed it is a published record, immutable to every later cycle) |
| any | a foreign record changed | any | refused | refused: `published cycle records are immutable` — one message, every branch |

The Verify mirror (audit, ship, `VerifyLanded`) branches on the recorded status rather than on the material set, so a required view with an empty material set verifies at ship exactly as at build. Its own foreign-record check sat behind the diff-SHA check — any path added after the build check changes the base-bound diff's SHA, which Verify compares first — and is removed as dead; `TestNonMaterialDiff_ARecordAddedAfterTheCheckDoesNotVerify` pins that the SHA check carries it.

What it does not change: the builder's rule stays "no material path — declare `NOT_APPLICABLE` and write no record" (the reference says so, with the test-only example); the host merely stops punishing a builder that over-delivers. The same cycle then met a second process failure at ship: the importer backstop ran 89 targets under full load and `bridge/channel.TestChannel_EndToEnd` — a package the cycle did not touch — failed on the 10 ms sleep between its two ticks (`answer text = ""`, the assistant envelope outside the span); the test now waits for the first tick's envelope in the feed. The backstop's own rung for a named red in a package the ship did not change (one isolated re-run of that test before it is the lane's RED; a green re-run is flake evidence, recorded, and the ship proceeds; a red one stays RED) is F7 in §7.3, designed and not built.

## 6. Decision tables

### 6.1 Routing by violation code (`deliverable`, beside the codes)

| Code | Route |
|---|---|
| `missing_effect` | host step (as today) |
| a derivable owed file: `missing_secondary`, `empty_secondary`, `malformed_secondary` of `triage-decision.json` | host derivation; a decline routes to re-dispatch, never to the agent (a decision document is the phase's judgment) |
| `stray_in_worktree` | salvage |
| `missing_section`; `empty_secondary` / `malformed_secondary` of a non-decision file | evidence sufficient: recovery agent; insufficient: assign back naming `Missing` |
| `missing_artifact`, `empty_artifact` | assign back: the owner is re-dispatched with `Present` as the correction; the diff and every green run are kept |
| a floor rejection without a code (explanation floor, build floor) | once floors carry stable codes (F0), routed with the whole set; until then, re-dispatch as today |
| `bad_verdict` | re-dispatch (the gate's deterministic verdict salvage already ran) |
| `missing_challenge_token` | re-dispatch (proof of read is never supplied by a helper) |
| `failure_context_missing`, `failure_class_unknown` | re-dispatch (the class is a judgment that drives retries) |
| `unbound_effect` | pipeline defect (a registry declaration nothing binds) |

### 6.2 Evidence per kind (`evidence.Sufficient`, E1)

| Kind (registry `recovery.evidence`) | `Present` when | `Missing` names |
|---|---|---|
| **change, code kind** (build) | the phase's own change is non-empty (the tree at this dispatch differs from the tree now); the build floor approved this tree hash and actually ran; this cycle's predicates under `acs/cycleN` exist with a complete inventory, none red and none skipped, on the current tracked tree; their bytes equal the TDD-end snapshot; the explanation document is present whenever material paths exist; the protected-surface floor is clean | each absent item, by name (the red or skipped predicate, the weakened file, the missing explanation) |
| **change, document kind** (ADR-0099) | `solutioncheck` reports nothing for every committed id; the diff stays inside the solution root and the explanation document | the failing check; a path outside the root |
| **tests** (tdd) | the diff against the base touches only test files and `acs/cycleN`; the host's own run of those predicates on the base tree fails each on an assertion (E0's capture) | a predicate that passes on the base, or fails to compile |
| **document** (scout, triage, build-planner, any phase so declared) | the decision-bearing sections parse; no tests run | the section that does not parse |
| judgment and control phases | never evaluated | — |

Facts that are reported but never conditions: paths outside the triage footprint (the footprint is agent-declared; every honest build touches tests and the explanation document beyond it); whether the predicates cover the acceptance (the auditor's judgment).

### 6.3 Which phases qualify

`phase-registry.json` declares per phase `recovery: {evidence: change|tests|document, decision_sections: [...]}`. A phase without the block is never evaluated. `remediationDenied` (audit, retrospective, debugger, adversarial-review, premise-challenge, plan-review) stays as the floor beneath that.

## 7. Components

Status: **shipped** (commit on a branch, PR open or merged) · **built** (green in a worktree, not shipped) · **designed** (in this document and the ADR only). Every component is one commit with its own tests, mutation-checked.

### 7.1 Policy and process (P)

| Id | Component | Status | Where |
|---|---|---|---|
| D0 | operating-policy §0 + ADR-0106 + this document | merged, #655 | `docs/` |
| P1 | ADR-0105 B1 unwind-rebase-pend; then the resume heal, B3, B4 | B2 merged (#649); B1 merged (#652); B3 and B4 shipped (§5.8 C1–C4); the resume heal designed | `core/ship_recovery*.go`, `core/identity_carry*.go`, `phases/ship/carry.go`, `internal/treedelta` |
| P2 | a fleet lane's closeout dossier waits for the wave boundary | built, parked (`fix/dossier-commits-at-wave-boundary`; architect N1–N5 applied) | `dossier/publish_pending.go`, `cmd_loop_dossiers.go` |
| P3 | the pasted prompt ends by stating who the agent is (phase, cycle, session, prompt files, sole writer); phase panes export the manifest's `default_env`, and claude-tmux turns prompt suggestions off | shipped, PR pending (`feat/phase-identity`, four commits) | `bridge/phaseidentity`, `driver_tmux_prepare.go`, `driver_tmux_boot.go`, `manifests/claude-tmux.json` |
| P4 | an exit-85 escalation names its pattern in `cause_code` and the cause line, anchored to the line start | merged, #656 | `bridge/launchoutcome/cause.go` |

### 7.2 Host derivations (H)

| Id | Component | Status | Where |
|---|---|---|---|
| H1 | one triage-report reader: `Derive` (strict) and `Project` (lenient); `triagecap` delegates; one stamp `projected_by_orchestrator`; protected surface | merged, #656 | `internal/triagedecision` |
| H1b | an empty optional bucket (`deferred`, `dropped`, `superseded`) derives to no cards; an empty `top_n` still declines | shipped | `internal/triagedecision` |
| H2 | registry `outputs.derived_from`; the host derives an absent or empty declared secondary after the effects, before the judges, waiting out a write in flight | merged, #656 | `deliverable/host_effects.go`, `phasespec`, `phasecontract`, `phase-registry.json` |
| H3 | the `## Explanation Documentation` declaration derived by the host | designed | `deliverable/host_effects.go`, `explanationdocs` |

### 7.3 Evidence (E) and floors (F0, F6a)

| Id | Component | Status | Where |
|---|---|---|---|
| E0 | host RED capture at the end of TDD | designed | `phases/tdd`, a host runner of `acs/cycleN` on the base tree |
| E1 | `evidence.Sufficient` over an injected evidence struct; registry `recovery` block | designed | `internal/evidence` (new leaf) |
| E2 | the assign-back correction names `Missing`; only the directive changes | designed | `core/retry_backoff.go` (`composeCorrection`), `core/resume.go` |
| F0 | stable codes for floor rejections; routing on the whole violation set | designed | `explanationdocs`, `core/build_floor_reviewer.go`, `deliverable` |
| F6a | E1 and E2 wired into the re-dispatch, before any agent rung | designed | `core/cyclerun_correction.go` |
| F7 | the importer backstop re-runs a named red test of a package the ship did not change once, alone, before it is the lane's RED: green ⇒ flake evidence (a WARN naming the test, and an inbox item for its hygiene fix so the lesson queues work), the ship proceeds; red ⇒ RED as today (§5.9; 1718's `TestChannel_EndToEnd`) | designed | `phases/ship/repocontract.go` (`runClassifiedPackRetrying`, after `realRed`) |

### 7.4 Verdict and ladder (V, F)

| Id | Component | Status | Where |
|---|---|---|---|
| V0 | verdict refresh after an approved rung, adopted only under the guard rule | designed | `core/cyclerun_correction.go`, the phase classifiers |
| F1 | `interaction.RungRecover` after live-fix, gated on `Repairable` | merged, #656 (unwired) | `internal/interaction/correction.go` |
| F2 | `recoveryguard`: whole-workspace fence + treefence; `Scope{Worktree, Workspace, Allowed, Unfenced, UnfencedStems}`; protected surface | merged, #656 (unwired) | `internal/recoveryguard` |
| F2b | provenance check: every block the agent's report names is byte-equal to its source in the pre-rung snapshot | designed | `internal/recoveryguard` |
| F3 | recovery-agent profile (codex family by default with claude as fallback, per the balanced-tier floor; sandbox on, read-only repo, run-dir grant, no network declared) and persona | merged, #656 (unwired) | `.evolve/profiles/deliverable-recovery.json`, `agents/evolve-deliverable-recovery.md` |
| F3b | a `{cycle}` write-grant template so a profile grants only its own cycle's run dir | designed | `bridge/sandbox_paths.go` |
| F4 | `bridgeDeliverableRecoverer`: dispatch, prompt, strict report parse | designed | `core/` beside `failure_advisor.go` |
| F5 | `workflow.recovery_rounds`: policy.json → policy → config → orchestrator option | designed | `internal/policy`, `internal/config`, `core/orchestrator.go` |
| F6 | wire the ladder: recover, guard, breaker-neutral check, one review, V0; an integrity violation aborts with a P0; signal codes | designed | `core/cyclerun_correction.go` |
| F6b | one ladder Strategy for the live path and resume | designed | `core/resume.go` |
| F7 | salvage executes at the default config (no cwd candidate in fleet mode; tracked candidates skipped); depends on V0 | designed | `core/correction_ladder.go` |
| F8 | composition root wires the recoverer, with a wiring-proof test | designed | `cmd/evolve/cmd_cycle.go` |

### 7.5 Capacity (Q)

| Id | Component | Status | Where |
|---|---|---|---|
| Q1 | exhaustion during a correction re-dispatch, a remediation re-run and the resume review gate defers through `pauseForQuota` (`isQuotaWall`: the runner's exit 85 means its whole family chain was walled — one sample suffices, unlike the first dispatch's two-sample rule, which predates the tiered chain); a non-wall failure stays a failure; the pause records the phase's total dispatches | shipped on the P3 train | `core/cyclerun_correction.go`, `core/cyclerun_remediate.go`, `core/resume.go` |
| Q2 | `auth_recheck` (`clihealth.CredentialPattern`) benches the family; unlike a quota bench it stays active for routing after its cooldown (time says nothing about a login) while the canary probes it, and a succeeding probe after the operator's login clears it; the bench entry carries `operator_action` (`clihealth.OperatorAction`), which the chain walker's bench line and the canary's re-bench line print | shipped on the P3 train | `internal/clihealth`, `internal/bridgechain`, `cmd/evolve/cli_health_canary.go` |
| Q3 | a deferral writes no failure digest and counts toward no halt: the wave breaker reads only digests and a deferral writes none (Q1's `TestRunCycle_WalledCorrectionRedispatch_DefersLikeAFirstDispatch`); the sequential loop now pauses on a deferral before it records a failed approach or runs the failure closeout, so a capacity wall never demotes the item or counts as a recoverable failure (`TestRunLoop_AFreshCycleWalledOnCapacityIsNobodysFailedApproach`) | shipped | `core/blocker_breaker.go`, `cmd/evolve/cmd_loop_sequential_dispatch.go` |
| Q4 | the fallback chain filters candidates by the phase's requirements | designed (§5.5) | `internal/bridgechain` |

### 7.6 Ledger of outcomes (L)

| Id | Component | Status | Where |
|---|---|---|---|
| L1 | recovered paths and the evidence verdict in `CycleResult.Remediations`, the dossier, the audit prompt, and an in-file marker | designed | `core/`, `dossier/`, the audit prompt |
| L2 | recovery-rung exhaustion recorded as a system-level pipeline defect; breakers unchanged; an exhausted assign-back stays the task's logic FAIL | designed | `core/blocker_breaker.go`, the retro paths |

### 7.7 Lane lifecycle (W) and the explanation document (X)

| Id | Component | Status | Where |
|---|---|---|---|
| W1 | a shipped or declined id no inbox item backs is retired by record: `lifecycle.(*Mover).RetireUnbacked` writes it, `inboxmover.RetireUnbacked` is the one door (unknown dispatch state only), the PASS seam retires what `Promote` could not move (`OutcomeResult.RetiredUnbacked`, processed) and the no-work closeout retires every such scoped id (`NoWorkResult.Retired`, rejected, the lane's answer as the reason); the planner's prune and the freshness probe read the record | shipped | `internal/inboxmover`, `internal/inboxmover/lifecycle`, `internal/cycleoutcome`, `internal/phases/ship` |
| W2 | a failing authored id accrues a durable failure count toward the S5 ceiling | designed (§5.6) | `internal/inboxmover` |
| X1 | `.evolve/inbox/` is non-material to the explanation document: the host claims, moves, stamps and retires those records | shipped | `internal/explanationdocs` |
| X2 | an explanation review that names only the document of a verified build is a recovery rung, never the audit's FAIL | designed (§5.6) | `phases/audit/explanation_review_gate.go`, the recovery agent (F4/F6) |
| X3 | the cycle's own explanation record is never history: only foreign cycle records are immutable, in every branch; a no-material diff declared `REQUIRED` is verified with an empty material set, one declared `NOT_APPLICABLE` keeps an undeclared own record as documentation; the Verify mirror branches on the recorded status | shipped (§5.9) | `internal/explanationdocs` (`checkBuildV1`, `verifyResolvedV1`) |

### 7.8 Routing at triage (R)

| Id | Component | Status | Where |
|---|---|---|---|
| R1 | a top_n card naming a protected surface is moved by the host into `escalate_block` with the console-route reason and the hook returns PASS with a warning per card; the planned no-work closeout routes the item; a route that cannot be recorded fails closed | shipped | `phases/triage/protected_route.go`, `phases/triage/triage.go` |
| R2 | the triage prompt lists the protected surfaces and the drop reason; the persona carries the rule | shipped | `phases/triage/triage.go` (`inboxBatchesSection`), `agents/evolve-triage.md` |

## 8. Interfaces

Shipped signatures are exact; designed ones are the contract the component must meet.

```go
// H1 — internal/triagedecision (shipped)
func Derive(report []byte, cycle int, lanePin []string) ([]byte, error) // strict; declines with the reason
func Project(report string, cycle int) ([]byte, error)                   // lenient; never declines
func SectionBody(report, heading string) (string, bool)
func ParseSection(body string) Section // Items, Rejected, Prose, None
func ActionOf, ReasonOf(rest string) string; FilesOf(rest string) []string
func SplitDeclaredFiles(rest string) (tokens []string, stripped string); DeclaredFilePath(tok string) (string, bool)
// H1b (shipped): section(report, name, strict, required) reads an empty optional bucket as no cards; Section.empty()

// H2 — registry and contract (shipped)
phasespec.IO.DerivedFrom map[string]string          // outputs.derived_from: owed basename → primary basename
phasecontract.Contract.DerivedFrom map[string]string
func (h *HostEffects) Perform(ctx, in core.ReviewInput) error // effects, then derive(); WARN on a decline

// F1 — internal/interaction (shipped)
const RungRecover = "recover" // salvage → live_fix → recover → redispatch
CorrectionInput.Repairable bool

// F2 — internal/recoveryguard (shipped)
type Scope struct{ Worktree, Workspace string; Allowed, Unfenced, UnfencedStems []string }
func Begin(ctx, scope Scope) (*Guard, error) // fails closed
func (g *Guard) End(ctx) Outcome            // Restored (violations), Unfenced (reported), Err
func (o Outcome) Clean() bool

// P3 — internal/bridge/phaseidentity (shipped)
const Heading = "## Who you are (stated by the evolve bridge)"
type Facts struct{ Agent string; Cycle int; Session, PromptFile, PastedFile, Artifact string }
func Block(f Facts) string // "" without Agent and Session
// P3 — internal/bridge (shipped): manifest default_env → Realization.Env → exportLines (pane) / driverEnv (headless)

// P4 — internal/bridge/launchoutcome (shipped)
// exit 85: cause_code = the escalation pattern (rate_limit, model_unsupported, …) or unknown_prompt

// W1 — internal/inboxmover/lifecycle (shipped)
func (m *Mover) RetireUnbacked(taskID, newState string, p PromoteOpts, reason string) (string, error) // processed|rejected; <state>/cycle-N/[<sha8>-]<id>.json; plain ids only
// W1 — internal/inboxmover (shipped)
func RetireUnbacked(opts Options, cycle int, state, reason, commitSHA string, ids []string) ([]string, error) // only ids whose dispatch state is unknown
OutcomeResult.RetiredUnbacked []string // the PASS seam: committed ids Promote could not move and no inbox item backs
// X1 — internal/explanationdocs (shipped): nonMaterialPrefixes gains ".evolve/inbox/"
// P1 — internal/treedelta (shipped)
func Args(base, tree string) []string // diff --binary --full-index --no-ext-diff --no-textconv --no-renames
func Delta(ctx, git Git, dir, base, tree string) ([]byte, error)
func Identical(ctx, git Git, dir, base0, tree0, base1, tree1 string) (audited, composed []byte, ok bool, err error)
// P1 — internal/adapters/ledger (shipped)
const IdenticalRebaseMethod = "identical-rebase"; CompositionVerdictInput.AuditedTreeSHA
func LatestCompositionVerdict(ledgerPath, method, laneAuditRef string) (CompositionVerdict, bool, error) // newest wins
// P1 — internal/core (shipped)
func (o *Orchestrator) identityCarryForward(ctx, cycle int, cs CycleState, base0, projectRoot string) bool // after the rebind: Ship on a proven carry
// P1 — internal/phases/ship (shipped)
func carrySatisfied(ctx, opts *Options, dir, actual string) (bool, string) // inside auditBindingSatisfied; Options.internalAuditArtifactSHA
// R1 — internal/phases/triage (shipped)
type protectedCard struct{ ID, Path string }
func protectedTopNCards(body string, forbidden func(string) bool) []protectedCard // every card, report order; nil forbidden = manifest membership
func routeProtectedCards(decisionPath string, cards []protectedCard) error         // top_n -= cards; escalate_block += {task_id, "protected-surface: <path> — …"}; other keys kept; idempotent
func routedCardDiagnostics(cards []protectedCard) []core.Diagnostic                // one warning per card, code TRIAGE_PROTECTED_SURFACE, subject the card
// W1 — internal/cycleoutcome (shipped)
type NoWorkResult struct{ Routed, Retired []string }
func ApplyNoWork(in NoWorkInputs) (NoWorkResult, error) // every scoped id no inbox item backs is retired as rejected

// E1 — internal/evidence (designed)
type Input struct{ Phase, Workspace, Worktree, BaseSHA string; Kind string; Acceptance []string; DecisionSections []string; /* host results injected */ }
type Verdict struct{ Sufficient bool; Present, Missing []string }
func Sufficient(in Input) Verdict // pure over kernel inputs

// E2 — core (designed)
// composeCorrection(missing []string) string — names Missing, never a violation code; rr.Reason, Blocks, signals unchanged

// F5 — policy (designed)
workflow.recovery_rounds int // compiled default 1; policy.json override; own key
```

## 9. Security model

| Threat | Guard |
|---|---|
| The recovery agent edits code, tests or the explanation document | F2 fences the worktree with `treefence`; a change is restored and is an integrity violation (abort + P0) |
| It forges proof of read, the lane pin, a verdict artifact, test logs | F2 fences the whole workspace; only `Allowed` paths may differ; everything else is restored and reported |
| It plants a file under a telemetry name | `Unfenced` matches exact paths or below a directory; generated-name artifacts are tolerated by stem, direct children only, and reported in `Outcome.Unfenced` |
| It swaps a fenced or allowed path for a link or a directory | undone at `End`; a link at an allowed path at `Begin` refuses the guard |
| It invents content | F2b provenance: every added block maps to a byte range of a pre-rung snapshot or a host log; a block without a source fails the rung |
| It carries a verdict it did not earn | V0 re-classifies; a verdict field must be byte-identical; a missing primary is never authored |
| Extra re-reviews demote the gate's breaker | rung checks are breaker-neutral; one review, only after they pass |
| A judgment is repaired | registry allowlist + `remediationDenied` |
| It reaches a sibling lane's run dir | the shared `.evolve/runs` grant is every phase's today; F3b narrows it to `{cycle}` |
| It exfiltrates what it reads | the profile declares no network; the wrapper forces network on today for every dispatch (filed: `sandbox-wrapper-forces-network-on`); the tmux drivers enforce no tool list (filed: `tmux-drivers-ignore-profile-tool-lists`); the filesystem grant is the boundary that holds |
| A fleet lane without a pin lets `Committed()` fall to the agent's `top_n` | filed: `fleet-lane-launches-without-a-lane-pin` |

## 10. Evidence and cost

Cycles ~1550–1707 (the inventory gathered for ADR-0106):

| Class | Nature | Cycles lost | Component |
|---|---|---|---|
| a peer's landing forces Build and Audit to re-run on a byte-identical change | process | 14 | P1 |
| `missing_effect` (inbox claim) | form | 6 | fixed (ADR-0100 F36) |
| `missing_secondary` `triage-decision.json` | form | 4, recurring after F36 | H1/H2 |
| bridge `submit_wedged` / "unknown prompt" exits | process | 6 + 8 (the escalation reports name every one of the 8: 4 `rate_limit`, 4 `model_unsupported`) | P4 (counting); the codex deep pin is the operator's call |
| loop halt from a form or process root cause (1700, 1705) | process | 2, each stops the loop | ADR-0072 stays; classification only |
| an agent refused its own task as an intruder (1707 TDD) | process | ~1 hour | P3, the persona's identity statement |
| a correction that touched only the explanation document left the primary unrewritten, so the finished phase idled through a review interval (1707 build) | process | ~20 min | F0; completion on the corrected file |
| a passed build aborted when the explanation floor exhausted its correction budget (1707, after a rebase and a passed re-audit) | form | the cycle | F0, E1/E2 |
| a verified build (ACS 10/10, EGPS red 0, mutation probe) sealed FAIL because the explanation document's sentence about a host-consumed inbox record contradicted the consumption stamp (1712 audit; the floor demanded the path twelve times across 1707, 1708 and 1712) | form | the cycle | X1 (shipped), X2 |
| a top_n card naming a protected surface sealed the cycle FAIL, and the item was routed only by the failure path (1714; the item declared no files, the agent was never told the surfaces) | process | the cycle | R1 (shipped), R2 (shipped) |
| a test-only build that explained itself was refused because the floor counted the cycle's own record as immutable history; the correction rung re-dispatched the builder to delete it (1718 build) | form | a builder dispatch (plus a codex `model_unsupported` bounce) | X3 (shipped) |
| the ship's importer backstop went RED on `bridge/channel.TestChannel_EndToEnd`, a timing-window test in a package the cycle did not touch, under the load of 89 targets (1718 ship → re-audit) | process | a re-audit and a second ship attempt | the test waits on the feed (shipped); F7 (designed) |

No `missing_section` or `bad_verdict` rejection is recorded in the range, so the recovery agent (F4/F6) lands last, after H1/H2 and the evidence decision are measured again.

Cost: an all-form rejection of a qualifying phase costs the host's run of this cycle's predicates (`acs/cycleN` only, bound to the tracked tree; the build floor's result is reused by tree hash; an audit seal is reused only when its evidence verifies for the current tree; document phases run nothing), bounded by `workflow.recovery_rounds`. The recovery agent is one bounded LLM call that replaces a whole-phase re-dispatch.

## 11. Rollout

Landing order and the checkpoint each must pass before the next starts.

| Step | Lands | Checkpoint |
|---|---|---|
| 1 | D0 (#655), P1 (#652), the code train (#656) | merged at a wave boundary in one burst; the plane synced; the full floor green on each |
| 2 | soak one wave | a triage lane that omits `triage-decision.json` proceeds without `GATE_CONTRACT_REJECTED [missing_secondary]`; a codex `rate_limit` escalation shows `cause_code=rate_limit`; the first fleet-rebase recovery logs "unwound its ship commit" |
| 3 | P2 (dossier at the boundary), P3 (identity prompt, no suggestions — shipped 2026-09-27) | a FAIL sibling's closeout no longer moves `main` under a passed lane; no agent refusal on identity |
| 4 | E0 → E1 → E2 → F0 → F6a | an insufficient all-form failure is re-dispatched with `Missing`; identity, block count and signals byte-identical with and without E2 |
| 5 | V0, F7 | salvage's approval routes PASS; a rung on an empty primary keeps FAIL |
| 6 | F2b, F3b, F4, F5, F6, F6b, F8 | a malformed build report is repaired and approved without re-dispatch; a guard violation aborts with a P0; a failed rung leaves the breaker unchanged; a resumed cycle reaches the rung |
| 7 | L1, L2 | the dossier and the audit prompt list the rung, paths and evidence; exhaustion files a P0 and the breakers still count |

Merges happen only at wave boundaries. Each step is its own PR; each component is its own commit with its tests.

## 12. Open questions and risks

- **The codex deep/top pin** (`gpt-5.6-sol`) is rejected by the account since 2026-09-14; every codex deep dispatch fails over to Claude. Re-pinning is a model-cost decision for the operator.
- **`allow_network` is not enforced** by the wrapper for any profile today. Until the filed item lands, the filesystem grant is the only OS boundary for every helper, including the recovery agent.
- **E1's predicate snapshot** (the TDD-end bytes of `acs/cycleN`) needs a kernel capture that does not exist yet; it lands with E0.
- **Floor rejections without codes** (F0) hide Build's commonest form failure from the routing table until they carry codes.
- **Provenance granularity** (F2b): byte ranges are strict; a repair that reorders a table may need line-level matching. Decide with the first real repair.
- **Document-kind cycles**: E1's `solutioncheck` path is designed, not measured.
- **Other CLIs' suggestion features**: codex, agy and ollama panes show no next-prompt suggestion today; if one appears, its off-switch is a `default_env` entry in that CLI's manifest, not code.
- **One rule for a walled dispatch**: the first dispatch's ladder still wants two all-85 attempts (`allFamiliesQuotaExhausted`, from before the tiered chain walked every family in one `Run`), while Q1 reads one walled `Run` as the same fact; unify on `isQuotaWall` once the ladder's tests model the chain.
- **One rule for a walled dispatch**: the first dispatch's ladder still wants two all-85 attempts (`allFamiliesQuotaExhausted`, from before the tiered chain walked every family in one `Run`), while Q1 reads one walled `Run` as the same fact; unify on `isQuotaWall` once the ladder's tests model the chain.
- **Completion after a correction**: the bridge completes a corrected phase only when the primary artifact is rewritten; a correction whose violation names only a secondary or the explanation document should complete on that file's rewrite (or on `evolve phase verify` passing); it lands with F0.

## 13. Review log

| Date | Review | Verdict | What changed |
|---|---|---|---|
| 2026-09-26 | architect, round 1 (design) | APPROVE-WITH-CHANGES | sandbox claim corrected; V0 verdict refresh; whole-workspace fence; breaker-neutral checks; H1/H2 into the host step; F6b shared ladder; L2 relabel only; integrity class; auditor visibility; landing order by cycles lost |
| 2026-09-26 | architect, round 2 (the evidence decision) | APPROVE-WITH-CHANGES | a missing report is always assigned back; per-kind evidence with this cycle's own unweakened red-on-base predicates; footprint a fact, not a condition; document-kind evidence; floor codes (F0); H1 as the strict mode of the existing projector; E2 changes only the directive; registry allowlist |
| 2026-09-26 | docs review | WARNING → APPROVE | policy renumbered as §0; §4 names control phases; re-check after both revisions approved |
| 2026-09-26 | go-reviewer (train) | APPROVE-WITH-MINOR | four minors applied |
| 2026-09-26 | code-reviewer (train) | WARNING | the derivation's write-in-flight grace (MAJOR) applied; the duplicate projector absorbed |
| 2026-09-26 | security-reviewer (train) | BLOCK → APPROVE-WITH-MINOR | unfenced boundary; allowed-path type check; no network declared; anchored markers; three gaps filed |
| 2026-09-27 | code-simplifier, go-reviewer, code-reviewer (Q2) | no edits; WARNING → fixed; WARNING → fixed | a login pane's stale reset hint never sets the bench; the fix names no login command (the families' logins differ); a credential bench holds for routing until a probe clears it; the fix is durable on the bench entry; the design rows state the shipped shape and its trade-offs |
| 2026-09-27 | code-simplifier, go-reviewer, code-reviewer (W1, H1b) | one gofmt alignment; PASS with three MINORs → applied; PASS with two MINORs → one applied | a present item whose move fails is never stamped unbacked (regression test); the lock-free write is an idempotent create, said in one line; the no-work closeout logs what it routed and retired; the plain-id guard is the one place a path is built from a caller's id |
| 2026-09-27 | code-simplifier, go-reviewer, code-reviewer (R1, R2) | the rewrite split into two helpers; BLOCK → fixed; BLOCK → fixed | an id-less card and a card the decision never committed both fail closed (the removal set can never hold an empty key; a report-versus-decision mismatch is a fault, not a silent commit); the ACS predicate that pins the admission tests by name follows the renames; the package's atomic JSON writer replaces an inline copy |
| 2026-09-27 | code-simplifier, go-reviewer, code-reviewer (P1: B3/B4) | two redundant rebinds; PASS with a MAJOR → fixed; WARNING with the same MAJOR → fixed | ship trusted the record's gate results from an unchained ledger read: ship now verifies the chain (0.4 s on 147K lines) and binds the record's patch-id to the bytes it proved, the ship-side gate check goes (the writer refuses red gates and the chain proves the writer), the git adapter forwards the real exit code, and the orchestrator declines a carry whose auditor row was bound on another base; the reviewers confirmed the full-index byte identity subsumes B2's lineage check at ship |
| 2026-09-27 | go-reviewer, code-reviewer (Q3) | PASS with two MINORs → applied; PASS with two MINORs → one applied | the skipped failure closeout is intentional (a wall is resumable, its claims stay claimed) and every wall producer wraps it as a cycle-level failure, both said in one comment; the summary assertion parses the loop's JSON instead of matching its spacing |
| 2026-09-27 | code-simplifier, go-reviewer, code-reviewer (Q1) | one closure; APPROVE-WITH-MINOR → applied; WARNING → justified and fixed | the one-sample reading of a walled `Run` stated in code and §12; the deferral records the phase's total dispatches; the digest assertion made non-vacuous |
| 2026-09-27 | code-simplifier, go-reviewer, code-reviewer, security-reviewer (P3) | no edits; APPROVE; WARNING → fixed; APPROVE-WITH-MINOR → hardened | the sole-writer line was false for stdout-completion phases (fixed); a manifest `default_env` could set credential or loop variables the guards never see (refused at parse); facts rendered into the block are sanitized; no manifest pattern may match the block (pinned) |
| 2026-09-26 | consistency audit (every doc vs the design vs the shipped code) | INCONSISTENCIES-FOUND → fixed | three stale package pages, one stale sentence in phase-architecture.md, one imprecise ADR sentence; two new package pages |
| 2026-09-26 | code-reviewer (design document) | APPROVE-WITH-MINOR | the exit-85 census corrected; the audit-seal clause restored; F2b cross-referenced |
| 2026-09-27 | X3 (§5.9) reviewed: code-simplifier no edit; go-reviewer PASS, one MINOR (an undeclared own record is unvalidated content that becomes an immutable published record — now stated in the contract doc and the §5.9 table); code-reviewer PASS, none (traced the four status × material combinations through build and Verify; confirmed the removed mirror check dead behind `diffSHA256`, which hashes every changed path) |

## 14. Document history

| Date | Change |
|---|---|
| 2026-09-26 | Created after the design landed in ADR-0106 and the first six components shipped (#654). |
| 2026-09-26 | Review (APPROVE-WITH-MINOR): the exit-85 census corrected (the eight unknown-prompt exits were four `rate_limit` and four `model_unsupported`); the audit-seal reuse clause restored; F2b cross-referenced from the ADR. |
| 2026-09-26 | F3 routed to the codex family with claude as fallback: the balanced-tier floor (`TestClaudeFamilyFloor`) reserves claude for judgment phases with a justification, and the recovery agent is an analytic helper. |
| 2026-09-26 | #652, #655 and #656 merged at the wave-13 boundary (1706 shipped, 1707 failed on form); statuses updated; two wave-13 observations added to the evidence and the open questions. |
| 2026-09-27 | Q2 shipped on the P3 train: a credential wall benches like a quota wall; the bench entry carries the operator's fix and the bench lines print it (code review: the fix must outlive the log tail). Deferred: a Signal Center ERROR for the wall, the defer message's operator wording, and protection for floor-pinned judgment phases (a bench is advice, not a veto). |
| 2026-09-27 | Q1 shipped on the P3 train: five tests pin the three seams and the negative case; eleven mutants killed; `isQuotaWall` reads the runner's exit 85 alone, because the sentinel half of the check had no caller (a mutant proved it). |
| 2026-09-27 | §5.5 and §7.5: the wave-14 deep-dive (three consecutive FAILs: 1707 form, 1708 and 1709 capacity) designs Q1–Q4 — capacity is a deferral through the one seam that already exists, a credential wall is an operator halt, and neither counts toward the FAIL streak. |
| 2026-09-27 | P3 shipped: §5.4 records the design (driver-appended statement; environment channel over a settings flag; what it does not fix); §8 signatures; §12 the other-CLIs question. Wave 14 (1708, 1709) failed on capacity — every CLI family walled (`auth_recheck`, `rate_limit`, `model_unsupported`) — with `cause_code` naming each pattern, the live proof of P4. |
| 2026-09-27 | §5.6 and §7.7: the wave-15 deep-dive (1713 planned no-work on a pin shipped by 1706 and carried through 1709 and 1710) designs and ships W1, the retirement record for an id no inbox item backs, and H1b, an empty optional bucket read as no cards, and X1, the inbox record made non-material to the explanation document (1712's verified build sealed FAIL on one sentence about a host-stamped file); §8 signatures; W2 and X2 designed |
| 2026-09-27 | §5.7 and §7.8: wave 16's first cycle (1714) sealed FAIL on a top_n card naming a protected surface; R1 moves such a card into `escalate_block` by the host's hand and lets the planned no-work closeout route it (never a verdict); R2 tells the agent the surfaces and the drop reason; §8 signatures; the evidence row |
| 2026-09-27 | §5.8: the carry (P1 = ADR-0105 B3/B4) designed as four components after 1712 and 1715 each re-audited a byte-identical rebase; the P1 row points at it |
| 2026-09-27 | §5.8 C1–C4 shipped: a byte-identical rebase ships on its audited verdict (`treedelta` leaf; the carry record names the audited tree; `identityCarryForward` after the rebind; ship re-proves the carry inside its one binding rule); the P1 row and §8 updated; the gate re-run in ship scoped out as C4b |
| 2026-09-27 | Q3 verified and closed: both breakers read only failure digests or FAIL verdicts, which a deferral never produces; the one gap was the sequential dispatcher recording a failed approach and running the failure closeout before its quota pause, now moved after the check |
| 2026-09-27 | §5.9: cycle 1718's test-only build explained itself and the floor refused its own record as immutable history; X3 makes foreign-only immutability the one rule in every branch, verifies a declared explanation of a non-material diff with an empty material set, and branches the Verify mirror on the recorded status; the X3 row, the evidence rows for 1718's build and ship, the channel test's feed wait, F7 designed |
