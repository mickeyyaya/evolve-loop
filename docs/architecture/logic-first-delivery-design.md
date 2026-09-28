# Logic-first delivery — design document

- **Status:** living document, kept current with every landing. Last updated 2026-09-27 22:30.
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
| C3 | `identityCarryForward` after the rebind (B3): the auditor row of the run names T0, the audit artifact and the base its worktree stood on (`latestAuditEntry`; `worktree_base_sha`, else `git_head` on an older row), which must be base0 = the base before the rebind, T1 = `git write-tree`, base1 = `HEAD`; C1 holds; the composed-tree gates run under `treefence` and the index must still be T1 afterwards; the record goes to the root ledger ship reads; return Ship, else Audit | a rebase that changed no byte of the change ships on the verdict it already earned |
| C4 | ship accepts a carry (B4): inside the one binding rule (`auditBindingSatisfied`), when the tree ship holds is not the bound tree, the newest `identical-rebase` record for this audit (`LaneAuditRef` = the audit artifact's SHA) whose `AuditedTreeSHA` is the bound tree and whose `TreeStateSHA` is the tree held is re-proven: the ledger chain verifies (0.4 s on a 147K-line ledger), both bases' ancestry in the tree's own directory, C1's byte equality, and the record's patch-id is the proven bytes'; the plane-HEAD comparison stays for the resume detection | ship never trusts the writer, and a carry cannot launder a change |
| C5 | the debugger's resolution rejoins the rebase (B5): when the recovery the debugger served began as a fleet rebase — inside the ship-recovery budget, after the contention backoff, never on an in-place worktree — the resolved worktree's tracked state is carried as a commit on the fork point, rebased onto main, pended on the new base and routed as any rebased change (`resumeFleetRebaseAfterDebugger` after `decideAfterDebugger`); a Build or Audit re-run the debugger asked for still runs, on the rebased tree; a tests-first request yields to the route (the explanation contract's post-TDD refresh cannot follow a moved binding) and says so; a conflict that survives, a rebase the host cannot finish or a spent budget ends the cycle on stderr and as `ORCHESTRATOR_REBASE_REENTRY_ABORTED` in the Signal Center | a resolved conflict never re-audits on the base the ship diverged from; 1719 paid one audit round and one ship attempt for that (§5.9) |
| C5b | the resume heal for an interrupted re-entry: a rebase left in progress is aborted and a carrier commit pended, so `evolve loop --resume` continues instead of ending the cycle on a mutated tree | designed | `core/resume_execution.go` |

What it does not change: a rebase the proof declines still returns to Audit (or Build when the change is not identical), and the audit-bound tree of a cycle that never rebased is checked exactly as today.

One scoping, stated: ship re-proves the ancestry and the bytes itself and verifies the ledger chain the record sits in, but does not re-run the composed gates (build, test, acs, apicover) a second time; the writer refuses a record whose gates are not green, and the chain proves the record came through the writer, so a ship-side gate check would be dead code. The orchestrator also declines a carry when the audited worktree stood on another base than the one the change was authored on; the row's `git_head` is main's HEAD when the audit was bound, not that base, which cycles 1724 and 1727 paid a second audit to show. Re-running the gates in ship (C4b) is a follow-up if a forged, re-chained ledger is ever a credible threat.

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

What it does not change: the builder's rule stays "no material path — declare `NOT_APPLICABLE` and write no record" (the reference says so, with the test-only example); the host merely stops punishing a builder that over-delivers. The same cycle then met a second process failure at ship: the importer backstop ran 89 targets under full load and `bridge/channel.TestChannel_EndToEnd` — a package the cycle did not touch — failed on the 10 ms sleep between its two ticks (`answer text = ""`, the assistant envelope outside the span); the test now waits for the first tick's envelope in the feed. The backstop's own rung is F7 (§7.3): when the importer backstop's reds are all named tests and at most three, each red package is re-run by itself before any of them is the lane's RED — the whole package, not the one test, so a same-package order dependency or shared fixture still reproduces and only the pack's concurrent load is removed; the argv and each package's outcome are written to the scan log. Green by itself is flake evidence, said in two places — a `ship.warning` `SHIP_BACKSTOP_FLAKE` in the Signal Center, where recurrence across cycles is countable, and a stderr line naming the tests — and the ship proceeds; still red is the lane's RED naming what is still red; a package whose re-run named nothing keeps its first-run reds, since nothing was proven about them; a build failure is never re-run. The scanner pack (guard suites) and the added-test backstop (red-first reproducers) keep no such rung: their reds are the ship's own. What the re-run cannot hide: a test the change really broke fails by itself too. What it cannot prove: one green run without the pack's load is evidence of a timing window, not proof — the line says so, and a recurring flake is F7b's work. The wiring proof (`TestRepoContractGate_AnImporterRedThatIsGreenAloneShipsAsFlakeEvidence`) drives the production caller with a module whose test reds once and is green when its package runs by itself. Queuing the hygiene fix from that evidence (an inbox item) is F7b, designed.

The wave's other lane, 1719 (`docs-consistency-sweep`, a document deliverable), showed the X2 class from a new angle. Its round-1 audit failed on one HIGH — option cost figures not traced to the evidence file, a defect in the deliverable itself, which the repair round fixed completely by the round-2 auditor's own account. Round 2 then failed on two more numbers: the explanation document's Verification section said `17/17 GREEN` while the repair round's TDD re-run had grown the eval to 19 checks (the auditor reproduced 19/19), and one count in the recommendation cited an evidence entry that records no such count. The first is staleness the pipeline itself produced — the repair round re-runs TDD before the build, so the eval the builder described is not the eval the auditor ran — and it lives only in the explanation document of a change the auditor called correct and complete; under X2 that is a documentation correction, never the round's FAIL. The second is the deliverable's own traceability claim broken by one number, the auditor's call to make. What the pipeline should do with a round whose only residual defects are of the first kind is X2's rung; a round with both kinds is a repair round, as it was.

The same cycle's ship then met the wave's other landing (1718's closeout dossier) and its fleet rebase hit a genuine conflict: both lanes had edited `docs/operations/rescue-branch-disposition-2026-06-07.md`, 1718's builder outside its test-hygiene item's declared files — the partition separates declared files, not touched ones. The host aborts such a rebase and hands the debugger a clean tree on the base the ship diverged from; the debugger took main's version of the file, and nothing re-ran the rebase. The cycle re-authored, re-audited (PASS) on the stale base, shipped, diverged again, and only then unwound, rebased cleanly and re-authored on the new base — one audit round and one ship attempt for a resolution that was already right. C5 (= ADR-0105 B5, §5.8) closes that: after a debugger that served a fleet-rebase recovery, the resolved tree is carried as a commit on the fork point, rebased, pended and routed as any rebased change; a conflict that survives the debugger ends the cycle loudly instead of a reship that cannot fast-forward. The partition gap itself (a builder's edits outside its item's files) is recorded, not fixed.

### 5.10 Waves 11–20: what the loop delivered and what the architecture let through (the deliverables review)

The operator asked for a review of the last ten waves' deliverables before any further fix (2026-09-27). The review covered the ten loop logs from 2026-09-26 15:40 to 2026-09-27 12:47, cycles 1702 to 1720, and read all eleven shipped diffs. Four parallel read-only reviews checked each diff against its inbox item and eval, using the owner's rules: task delivered, root cause over workaround, single source of truth, tests that prove intent, no narrative comments, functions under 50 lines, declared scope, and docs updated.

| Wave | Cycles | Lanes realized / sized | Outcome | What it exposed | Answer |
|---|---|---|---|---|---|
| 11 | 1702, 1703 | 2 / 2 | both shipped | — | — |
| 12 | 1704, 1705 | 1 / 2 | 1704 shipped; 1705 halted the loop (`system_failure_halt`) | a Build retracting its own explanation draft was read as infra-systemic | #651 (merged) |
| 13 | 1706, 1707 | 1 / 2 | 1706 shipped; 1707 FAIL | the TDD agent refused its own task as an intruder; a passed build aborted on the explanation floor | P3 (merged); F0 (designed) |
| 14 | 1708, 1709 | 0 / 2 | both FAIL on capacity | every CLI family walled; capacity counted as failure | Q1, Q2, Q3 (merged) |
| 15 | 1712, 1713 | 1 / 2 | 1712 shipped after an audit FAIL; 1713 no work | a pin no inbox item backed; a verified build sealed FAIL on one explanation sentence | W1, H1b, X1 (merged); X2 (designed) |
| 16 | 1714, 1715 | 1 / 2 | 1714 FAIL; 1715 shipped | a top_n card on a protected surface; a byte-identical rebase re-audited | R1, R2, B3/B4 (merged) |
| 17 | 1716, 1717 | 2 / 2 | both shipped | 1717 paid the last byte-identical re-audit | B3/B4 |
| 18 | 1718, 1719 | 2 / 2 | both shipped | a cycle's own record read as history; a timing flake at the backstop; a debugger's resolution left on the stale base | X3, F7, B5 (merged) |
| 19 | 1720 | 1 / 2 | shipped | a sized lane lost: the backlog's only lane items were blocked | W3 (this section) |
| 20 | none | 0 / 2, six times | six empty waves until `max_cycles` | the plan offered lanes the gate refused, and a launch of nothing counted as a wave | W3 (this section) |

Cycles 1710 and 1711 were minted inside wave 14's window and stopped after scout; neither shipped.

**The deliverables.** All eleven ships delivered the task their item asked for, and test discipline was the strongest signal in the batch. The predicates drive real subprocesses, AST call graphs and `go test -overlay` mutants rather than log text. The defects are in what the pipeline let through, not in what the lanes chose to do:

- **Superseded attempts land in the acceptance ledger.** 1712, the third attempt at its item, shipped `go/acs/cycle1707` and `go/acs/cycle1708` beside its own `cycle1712`. They re-encode the same predicates for two cycles that never shipped. The same attempts' explanation documents were archived, so the two artifact kinds follow two retention rules (A2).
- **Class tickets close on instance fixes.** 1706's item was a flake class at its second sighting. The fix migrated the three sighted tests to the new `internal/gittest` fixture, while 116 test files still build raw git repositories in `t.TempDir()` and nothing stops a new one (G3).
- **House rules exist only as prose.** The lanes add a doc comment on every new function while the comment campaign removes them, because the phase personas still ask for them (G1). A bugfix cycle leaves `ship/consume.go`'s `consumeCommittedItems` at about 110 lines and `stage` at about 99, because the 50-line limit is a test only in the packages that pin their own (G2). 1712 edited `modelquery/latest.go` outside its item's declared files, and 1718 edited a doc that later conflicted with 1719, because nothing compares declared files with touched files (§5.9).
- **Test support replicates.** At least seven `go/acs/cycleN` packages re-author the same git-subprocess wrapper and environment scrub. 1715 copies two commit anchors from `cycle1529`, and 1717 parses the task-contract header twice. These are minor; the fix is a shared `acsassert` helper.

**The pipeline.** The waves' process defects were answered as they surfaced, as the table shows. The review adds five architecture-level findings. Each is a replicated or unenforced belief, the disease the canonical retrospective named:

1. **Dispatchability was decided in five places (W3, fixed).** "May a lane take this id now" was answered by the backlog reader (console routing only, deps ignored), by two carried-id prunes with different "consumed" sets, by the launch gate (lifecycle and deps), and by the refill (weight only). At wave 20 the plane's inbox held 169 top-level items: 162 console-owned (67 console kinds, 37 console routes, 58 protected surfaces declared or mentioned), 5 without an `id`, and 2 lane material. Both lane items waited on a console-owned dependency (`egps-regression-tia-selection`, `chronicle-s7a-historian-shadow`). The widen planned the two blocked items, the gate refused both, and the refill's backlog had nothing else. The dispatch then reported the empty launch as a wave: `wave N: 0/0 lanes ok` printed six times, and the batch ended on `max_cycles`. Wave 19 lost its second lane the same way.
2. **The acceptance ledger decays silently (A1).** 249 predicate files run tests by name through `go test -run`, which exits 0 when a name matches nothing. At review time, 26 exact test names referenced by predicates no longer existed, because the tests were renamed or deleted after their predicates were written, so those checks pass vacuously. Two more would have joined them in this landing, had the cycle-1182 predicates not been repointed with the renames.
3. **Item identity is replicated (W5).** The canonical reader, `inboxbatch.LoadFile`, gives an item without an `id` its filename stem. The lifecycle's reader (`lifecycle.readID`) and the planner's backlog read the JSON field alone. The five id-less items are visible to triage, invisible to the planner, and unclaimable by the lifecycle.
4. **Any quota shrink switches off the starvation detector (W4).** `WaveObservation.Starved` excuses a wave whenever the bench shrank it. Waves 19 and 20 realized 1 and 0 of 2 sized lanes and counted as not starved, because codex's bench had shrunk the width from 3 to 2.
5. **A deterministic failure is retried at every dispatch (Q5).** The codex deep pin (`gpt-5.6-sol`) fails `model_unsupported` on the account's entitlement at every deep or top dispatch. That is about eight bounces per wave, each paying a boot before the claude fallback. The operator's pin decision is pending, but the bench need not wait for it. The same class explains wave 20's six identical empty waves.

**Order of fixes.** W3 lands first: it cost a whole batch, and every later wave depends on an honest plan. A1 follows, because an acceptance ledger that passes vacuously undermines every gate built on it. Then come Q5, W6 (the work-supply census, so an empty plan names why), G1, A2, W4, W5, G2 and G3, each as one component with its own tests. The census also matters beyond the loop: with 162 of 164 pending items console-owned, the loop's lane supply is structurally tiny, and whether that is the intended division of work is the operator's call, not the loop's.

### 5.11 Wave 24: an incapable last rung sealed two capacity failures as FAIL (cycles 1733, 1734)

Wave 24 went 0/2 after five consecutive ships. Both cycles failed the same way, in a phase whose every capable rung failed for a reason that was not the lane's logic: claude-tmux stalled (1733's bug-reproduction: "agent hung: stalled for 3 consecutive busy intervals") or refused the pasted prompt as untrusted text (1734's triage), codex-tmux was walled ("usage limit … try again at Oct 14th, 2026", exit 85), and the universal-fallback tail's last rung, ollama-tmux, refused the phase at launch (exit 10, `source-writing phase rejected`). The runner surfaces the last rung's error, so a structural refusal of a driver that can never run a lane phase decided the verdict: FAIL, where the walk had met a capacity wall and Q1's deferral applies. Three facts made it a class:

- The tail appended every discovered family, capable or not: ADR-0104 made the tail unconditional and banned only agy by operator judgment.
- ollama's launch guard reads `cfg.Worktree != ""` as "a writer phase", a belief CB.1 retired (every phase runs cwd=worktree), so it refuses every lane phase, not only writers; its real limit is having no tool use.
- With codex walled for two weeks, claude-tmux is the only capable rung, so any claude hiccup walks to the incapable tail.

| Id | Component | Status |
|---|---|---|
| T1 | the tail holds only drivers that can run the phase: manifest `toolless`, `bridge.HasToolUse`, `universalFallbackTail` (ADR-0104 §4) | built (this landing) |
| T2 | an exhausted walk that met a quota wall is a capacity outcome whatever the rung order (a claude stall after codex's wall also defers): `DispatchTiered.Walled`, `bridgechain.WallKeeper` in the runner's walk and the bridge handle's alike; the pause names every rung | built (this landing) |
| T3 | a pasted prompt carries a typed authorizing turn, so claude does not decline it as untrusted | designed (`claude-tmux-typed-authorizing-turn`) |
| T4 | the advisor's plan is clamped to the phase registry's `insert_when`, so a refactor lane never runs bug-reproduction or fault-localization (1733 failed in one) | designed (F42, `document-lane-planned-bugfix-phases`, evidence added) |

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
| F7 | when the importer backstop's reds are all named tests and at most three, each red package is re-run by itself — the whole package, so same-package state stays in play and only the pack's concurrent load is removed; the argv and each package's outcome in the scan log: green ⇒ `SHIP_BACKSTOP_FLAKE` (a `ship.warning` in the Signal Center and a loud stderr line naming the tests: one re-run without the pack's load is evidence, not proof) and the ship proceeds; still red ⇒ the lane's RED naming what is still red; a package whose re-run named nothing keeps its first-run reds; a build failure is never re-run; the scanner pack and the added-test backstop keep no such rung (§5.9; 1718's `TestChannel_EndToEnd`) | shipped | `phases/ship/repocontract.go` (`clearedAlone`, `runRepoContractPackagesAlone`, `packFailure`) |
| F7b | the flake evidence queues the hygiene fix: an inbox item naming the test and the scan log, so the lesson becomes work instead of a WARN line | designed | `phases/ship`, `internal/inboxmover` |

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
| Q5 | a `model_unsupported` exit benches the family's model at that tier until the pin changes, like Q2's credential bench, instead of booting it at every deep or top dispatch | designed (§5.10) | `internal/clihealth`, `internal/bridgechain` |

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
| W3 | one dispatchability rule (`inboxmover.ResolveDispatchability`, with `PendingDispatchability` for a known-pending item): the carried-id prunes, the backlog the seed, the widen and the refill read, and the launch gate all ask it, from one root (`EvolveDir`; Q-W7 and Q-W8 resolved), so the plan never offers a lane the gate refuses; the triage menu, `evolve inbox batches`, the planner's backlog and the no-work check (lane-scoped and sequential) read one per-item judgment (`PlaceOnLaneMenu`, folded by `PartitionLaneMenu`, injected into core as `WithLaneMenu`), so no menu offers a waiting item and an empty commitment over withheld work is honest no-work, while a cycle's own claim still counts as the work it took, and the no-work check fails closed only on an unreadable inbox file, never on a notice about a readable one (`inboxbatch.ScanDir`); the refill ranks with the shared `triagecap.RankForDispatch`; `Ran` means a lane launched, so a launch the gate empties takes the min-width repair and empty-backlog path instead of counting a wave, and that path now counts toward work-supply starvation (§5.10: waves 19 and 20) | shipped | `internal/inboxmover` (`dispatchability.go`), `internal/triagecap` (`PruneUndispatchable`, `ReadInboxBacklog`, `RankForDispatch`), `internal/loopwave` (`dispatch`, `probe`, `refill`, `pruneUndispatchable`) |
| W4 | the starvation observation compares realized lanes with the wave's sized width, so a quota shrink explains only the lanes it removed; supersedes `go/acs/cycle544`'s rule that any shrink disables the detector | designed (§5.10) | `internal/fleet/starvation.go`, `cmd/evolve/cmd_loop_window.go`, `go/acs/cycle544` |
| W5 | one item identity: every inbox reader resolves every item as `inboxbatch.LoadFile` does (its id fallback and its sanitizer), or the host stamps the id at intake, so no item is visible to triage but invisible to the planner and unclaimable by the lifecycle; reachable since W3, whose empty-backlog path runs a sequential cycle whose no-work check counts an id-less lane item as work the lifecycle cannot claim, so until W5 lands the inbox must hold no id-less item. The guard covers the inbox as it stands at launch: the 2026-09-27 inbox curation stamps ids on the five, which console sessions filed by hand, and the boundary refuses to launch while `grep -L '"id"' .evolve/inbox/*.json` lists a file; an id-less item filed mid-wave ends its cycle as a loud claim failure, not a silent one. The same identity gap has a quiet twin on the lane path: a scoped id the loader's sanitizer rewrites (a control character, or over 160 characters) no longer matches the menu's id, so an empty commitment over it ends as planned no-work; no tracked id is affected today (all 121 are 55 characters or fewer), and W5 closes both | designed (§5.10) | `internal/inboxbatch`, `internal/inboxmover/lifecycle`, `internal/triagecap` |
| W7 | the claim floor refuses an item whose declared dependency has not landed, so a triage pick that names one anyway fails loudly instead of building ahead of its dependency (W3 stops offering it; nothing yet enforces it at the claim) | designed (§5.10, second architecture review) | `internal/inboxmover` (the claim floor) |
| W8 | the batch classifier's dependency half (`inboxbatch.depRule`'s deps loop, `topoOrder`'s dependency ordering) is removed with its tests: under W3 a dependent never shares a menu with its unlanded dependency, so the grouping is unreachable (links and campaigns stay) | designed (§5.10, second architecture review) | `internal/inboxbatch`, `go/acs/cycle1205` |
| W6 | a work-supply census: an empty or short plan's signal names the pending items by why they are not lane material (console kind, console route, protected surface, unmet dependency with its blocker, no id) | designed (§5.10) | `internal/triagecap`, `internal/loopwave` |
| X1 | `.evolve/inbox/` is non-material to the explanation document: the host claims, moves, stamps and retires those records | shipped | `internal/explanationdocs` |
| X2 | an explanation review that names only the document of a verified build is a recovery rung, never the audit's FAIL | designed (§5.6) | `phases/audit/explanation_review_gate.go`, the recovery agent (F4/F6) |
| X3 | the cycle's own explanation record is never history: only foreign cycle records are immutable, in every branch; a no-material diff declared `REQUIRED` is verified with an empty material set, one declared `NOT_APPLICABLE` keeps an undeclared own record as documentation; the Verify mirror branches on the recorded status | shipped (§5.9) | `internal/explanationdocs` (`checkBuildV1`, `verifyResolvedV1`) |

### 7.8 Routing at triage (R)

| Id | Component | Status | Where |
|---|---|---|---|
| R1 | a top_n card naming a protected surface is moved by the host into `escalate_block` with the console-route reason and the hook returns PASS with a warning per card; the planned no-work closeout routes the item; a route that cannot be recorded fails closed | shipped | `phases/triage/protected_route.go`, `phases/triage/triage.go` |
| R2 | the triage prompt lists the protected surfaces and the drop reason; the persona carries the rule | shipped | `phases/triage/triage.go` (`inboxBatchesSection`), `agents/evolve-triage.md` |

### 7.9 The acceptance ledger (A) and the rules on a lane's output (G)

| Id | Component | Status | Where |
|---|---|---|---|
| A1 | a predicate that runs tests by name fails when a name matches no test, so a rename or deletion cannot turn it vacuous; the 26 stale names found at review are repointed or retired | designed (§5.10) | `pkg/acsassert`, `go/acs` |
| A2 | a superseded attempt's artifacts follow one retention rule: its predicate package is archived with its explanation document, never landed beside the shipping cycle's | designed (§5.10) | `internal/phases/ship`, the continuation carry |
| G1 | the phase personas carry the no-comments rule, so lanes stop adding what the comment campaign removes | designed (§5.10) | `agents/`, the engineering-craft skill |
| G2 | the function-size limit is one repo-wide ratchet instead of a test per package | designed (§5.10) | a lint step |
| G3 | a class ticket closes on an enforced invariant: a ratchet refuses a new raw git fixture outside `internal/gittest` | designed (§5.10) | a repo-contract test |

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
// W3 — internal/inboxmover (shipped)
type Dispatchability struct { Dispatchable bool; Reason string } // Reason: "deps unmet: needs <dep>" | "consumed: <state>[ <cycle dir>]"
func ResolveDispatchability(opts Options, taskID string) Dispatchability // pending with deps met, or no lifecycle evidence
func PendingDispatchability(opts Options, deps []string) Dispatchability // the pending branch, for a reader that already knows
type MenuPlace int // MenuReady | MenuConsole | MenuWaiting
func PlaceOnLaneMenu(opts Options, it inboxbatch.Item, isProtected func(string) bool) (MenuPlace, string) // console route first, then deps; the reason is the rule's
type LaneMenu struct { Ready, Console, Waiting []inboxbatch.Item; ConsoleReasons, WaitingReasons []string }
func PartitionLaneMenu(opts Options, items []inboxbatch.Item, isProtected func(string) bool) LaneMenu // PlaceOnLaneMenu folded over a list; reasons as "<id>: <reason>"
// W3 — internal/inboxbatch (shipped)
type LoadWarning struct { Text string; Unreadable bool } // Unreadable: a file it could not read or parse; otherwise a notice about an included item
type DirScan struct { Items []Item; Warnings []LoadWarning }
func ScanDir(dir string) (DirScan, error) // the one loader; LoadDir projects its warnings to text in the same order
func (s DirScan) HasUnreadable() bool
// W3 — internal/triagecap (shipped)
func PruneUndispatchable(evolveDir string, committed []FleetCandidate) []FleetCandidate
func RankForDispatch(cands []FleetCandidate) []FleetCandidate // weight, then a declared surface, then input order; the refill's order too
func ReadInboxBacklog(evolveDir string, isProtected func(string) bool) []FleetCandidate // PlaceOnLaneMenu's ready items that carry an id
// W3 — internal/core (shipped)
type LaneMenuFn func(inboxRoot string, items []inboxbatch.Item) []inboxbatch.Item
func WithLaneMenu(fn LaneMenuFn) Option // the composition root injects PartitionLaneMenu's ready set; claims keep the routing-only rule
func (o *Orchestrator) LaneMenuWired() bool
// W3 — internal/loopwave (shipped): dispatch returns Outcome{} (Ran=false) when the launcher launched no lane
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
| the ship's importer backstop went RED on `bridge/channel.TestChannel_EndToEnd`, a timing-window test in a package the cycle did not touch, under the load of 89 targets (1718 ship → re-audit) | process | a re-audit and a second ship attempt | the test waits on the feed (shipped); F7 (shipped); F7b (designed) |
| a debugger resolved a genuine fleet-rebase conflict on the base the ship diverged from, the cycle re-authored and re-audited there, and its next ship diverged again (1719 ship, twice) | process | an audit round and a ship attempt | C5 = B5 (shipped) |
| the plan offered lanes the launch gate refused (items blocked on a console-owned dependency) and a launch of nothing counted as a wave (waves 19 and 20) | process | a lane in wave 19; all six iterations of wave 20's batch | W3 (shipped) |
| the codex deep pin fails `model_unsupported` on the account's entitlement at every deep or top dispatch (waves 15 to 20) | process | about eight boots per wave before the claude fallback | Q5 (designed); the pin is the operator's call |

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
| 2026-09-27 | F7 (§5.9, §7.3) reviewed twice. First pass: code-simplifier merged two dedupe-and-sort copies; go-reviewer FAIL (the per-package merge dropped an unproven package's names and could blame a proven-green one; untested; the still-red test could not tell first from second); code-reviewer WARNING (re-running one test alone hides a same-package order dependency; the line overclaimed proof; the WARN never reached the Signal Center; cleared packages were not logged). Reworked: the whole package re-runs by itself, an unproven package keeps its first-run reds, the merge is unit-tested through the `goTestJSONFn` seam, every package's outcome is logged, the line says evidence not proof, `backstopFlakeSignal` emits `ship.warning` `SHIP_BACKSTOP_FLAKE`. Second pass: go-reviewer PASS, none; code-reviewer WARNING — the event's origin must name a real symbol (now `Phase.runNative`) and a load-dependent regression can still clear on one solo run (disclosed in §5.9; F7b is the operational backstop, designed) |
| 2026-09-27 | C5 (= ADR-0105 B5; §5.8, §5.9) reviewed twice. First pass: code-simplifier no edit; go-reviewer BLOCK (no `inPlaceWorktree` guard before the first git mutation; repo-wide `git add -A` could carry untracked debris); code-reviewer WARNING (the debugger's RERUN target was discarded; the re-entry bypassed the recovery budget and backoff; the git sequence has no checkpoint for resume). Reworked: the guard checks the in-place worktree; the carrier stages tracked paths only and says what it leaves behind; a Build or Audit re-run the debugger asked for runs on the rebased tree while a tests-first request yields to the route and says so; the budget is checked, the backoff applied and a re-entry counts one attempt; every abort emits `ORCHESTRATOR_REBASE_REENTRY_ABORTED`; the resume heal is C5b, designed. Second pass: go-reviewer PASS (one MINOR — warn on leftover untracked files — applied); code-reviewer PASS (two MINORs — a full debugger round trip costs two budget units, and B5 extends the ship-ahead-of-the-heal precedent — both recorded in ADR-0105) |
| 2026-09-27 | W3 (§5.10, §7.7) reviewed: code-simplifier no edit; one combined code and Go review PASS, none (it traced that only the freshness gate can return no results for a non-empty plan, and that the one-lane repair inherits the fix); architecture-reviewer WARNING, FIX_THEN_MERGE, all fixed except one LOW: the triage menu and `evolve inbox batches` still offered items waiting on a dependency, which the sequential fallback could now pick (HIGH: `inboxmover.PartitionPending` holds them back in both, listed loudly); a zero-lane wave no longer reached the starvation tracker (MEDIUM: the coordinator observes both paths through one helper, `observeWorkSupply`); the planner read two lifecycle roots that `--evolve-dir` separates (MEDIUM: every planner reader reads `EvolveDir`, Q-W7 and Q-W8 resolved); stale names, the rule restated in four docs, and the seed path's different signal (LOW: fixed, the docs link to the rule); the `consumed: processing` wording for in-flight ids (LOW: kept, pinned by goldens operators grep; renamed when those goldens next move) | Second pass: the combined code and Go review PASS with three MINORs, applied (a local named `waiting` held reasons; `dependency_blocked` shipped strings beside `console_routed`'s records, now one `excludedItem` shape; the batches command extracted from `runInbox`); the architecture review confirmed the three fixes and raised a HIGH and three MEDIUMs: the classifier's dependency grouping is now unreachable (recorded as W8, a deletion with its pins, not this landing, because it touches a documented classifier feature), the menus hide a waiting item but the claim floor does not refuse one (W7), the repair width was a bare 1 in two places (now `loopwave.RepairWidth`), and `dispatchFleetIteration` exceeded 50 lines (split into `dispatchPool`, `waveRequest`, `completeWave` and `repairMinWidth`). An inbox review then found the sixth copy of the rule: the sequential no-work check counted protected-surface and waiting items as claimable, so W3's empty-backlog path would have sealed a misattributed FAIL; the check now reads the same lane menu (`core.WithLaneMenu`). Third pass (architecture and one combined code and Go review, on the last delta): the extraction and the default preserved; a MAJOR regression fixed red-first (the no-work check had run this cycle's own claims through the dependency rule, so a claimed waiting item would have ended as silent no-work and stranded in processing; claims now keep the routing-only rule); the lane-menu rule composed in two places (HIGH) became one per-item Specification, `PlaceOnLaneMenu`, with an agreement test pinning the planner's backlog to the menu; the lane path and `laneMenuOf` gained tests; the pool's routing backstop reads the configured `EvolveDir`; stale origin strings and doc wording fixed. Fourth pass (architecture and one combined code and Go review, on the third pass's delta; six of seven mutants caught): stale text fixed (internal-loopwave.md restated the deleted `PartitionPending` and listed the pool among `RootsOf`'s callers, a `nullWaveEngine` comment listed a caller it no longer has, and §8 lacked the lane-menu exports); the uncaught mutant closed (console routing judged before the dependency rule, now a test row); the agreement test's claim narrowed to items that carry an id, with the id-less difference pinned as W5's characterization test; the claim scans' helper renamed `admittedIDs`, because the word "dispatchable" on the one scan that must not ask the dependency rule invited the third pass's regression; W5 recorded as reachable through W3's sequential fallback, with the ids stamped before the next launch. Fifth pass (the three lenses on the fourth pass's delta): every fix confirmed and each new test killing only its own mutant, but the rationale moved into internal-core.md exposed a HIGH that W3 routes the P0 through: the no-work check failed closed on any loader warning, and those include notices about readable items (a sanitized field, a duplicate id) that the tracked inbox carries on 48 of its 121 items, so a sequential cycle whose triage honestly committed nothing sealed a FAIL; fixed red-first by typing the loader's warnings (`inboxbatch.ScanDir`, `LoadWarning.Unreadable`, `LoadDir` its projection) and failing closed only on an unreadable file, with the stale comments on the check deleted and the doc wording corrected (the pending-only menu judgment, the second known backlog difference, the W5 guard's scope and check).

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
| 2026-09-27 | F7 built and shipped: the importer backstop re-runs its named reds once alone before any is the lane's RED (green ⇒ `SHIP_BACKSTOP_FLAKE`, the ship proceeds; red ⇒ the lane's RED; ambiguous ⇒ the first verdict; a build failure never re-runs; at most three named reds); the F7 row, §5.9's last paragraph and the 1718 ship evidence row updated; F7b (the inbox item) designed |
| 2026-09-27 | §5.8 C5 (= ADR-0105 B5) and §5.9: 1719's fleet-rebase conflict was resolved by the debugger on the stale base and rediscovered at the next ship; the resolved tree now rejoins the rebase route; evidence row; the partition gap (edits outside an item's declared files) recorded |
| 2026-09-27 | §5.10: the review of waves 11–20 (all eleven ships delivered their task; five architecture findings, each a replicated or unenforced belief); W3 shipped (one dispatchability rule, the refill's shared ranking, `Ran` means a lane launched); W4, W5, W6, Q5 and §7.9 (A1, A2, G1, G2, G3) designed; two evidence rows; the architecture review's fixes (the triage menu, the one root, the zero-lane starvation) |
| 2026-09-28 | Wave 21 (the first with W3 live) shipped 1721, 1722, 1723, 1724 and 1726 on fresh inbox work, which W3's planner offered; 1725 met a genuine fleet-rebase conflict on `go/.apicover-enforce`, the read-only fence reverted the debugger's resolution, and the cycle ended WARN with its claim stranded. Fixed: a cycle the debugger ends is a diagnosed FAIL, so the failure walk releases its claim; the append-only registry merges as a union (`.gitattributes`). Open: the fence (`fleet-rebase-debugger-resolution-fenced`), the boundary re-exec that restarts the cycle budget and the wave index, and the carry that declines an audit already bound on the rebase target. |
| 2026-09-28 | §5.8 C3: the carry reads the base the audited worktree stood on. The auditor row gains `worktree_base_sha` (`recordAuditBinding` writes the cycle's worktree base), and `identityCarryForward` compares it, not the row's `git_head` (main's HEAD when the audit was bound), with base0; an older row falls back to `git_head`. Cycles 1724 and 1727 each paid a second audit of a byte-identical rebase because a sibling's landing had moved main before their audit. `carry-accepts-audit-bound-on-rebase-target` consumed. The review added: `auditledger.Entry.AuditedBase` owns the projection (the row schema's package), `carriedAudit` declines a row that names no base, and a reflection-driven round trip pins every `LedgerEntry` field through the hand-kept wire struct. Also found: RUNG 0 holds the same belief in two places, its snapshot (`cmd/evolve/cmd_composition_wiring.go`, `main...git_head`) and ship's composition check (`internal/phases/ship/composition.go`, `AuditedBase` against `git_head`), filed as `composition-snapshot-reads-the-worktree-base` |
| 2026-09-28 | §5.8 C5 (= ADR-0105 B5), the fence: the debugger of a fleet-rebase conflict writes its resolution. The rebase returns its non-derived conflicted paths, `CycleState.ShipRecoveryConflicts` records them (cleared by the ship latch), `PhaseRequest.WorktreeWritablePaths` hands them to that debugger alone on the live and resume dispatch, its prompt names them, and `treefence` keeps writes to them while reverting every other (ADR-0097 decision 6). Pinned end to end by a genuine conflict through a debugger held by the production fence, shipped on main. The review added: conflicted paths are read with `-z` so the fence compares raw paths, the prompt lists them bare and the persona alone says what they mean, and three kill tests; `worktree-fence-one-value` filed (the out-of-band retro runs unfenced; the fence pair can be half-set). `fleet-rebase-debugger-resolution-fenced` consumed; its third acceptance line (the union attribute in `.gitattributes`) had landed in #680 |
| 2026-09-28 | §5.11: wave 24 went 0/2 (1733, 1734) after the streak reached five (1727, 1729, 1730, 1732, 1731); both walked a stalled or refusing claude-tmux and a walled codex-tmux into ollama-tmux's structural refusal, which sealed FAIL. T1 and T2 built (the tail holds only drivers that can run the phase; an exhausted walk that met a wall surfaces the wall and defers); T3, T4 designed; the loop halted for the P0 |
