# Logic-first delivery — design document

- **Status:** living document, kept current with every landing. Last updated 2026-10-20.
- **Decision record:** [ADR-0106](adr/0106-logic-first-delivery.md). **Policy:** [operating-policy §0](../operations/operating-policy.md).
- **Landings:**
  - The design in #655 (merged `5600b77a`). It replaced #653 after a CHANGELOG conflict.
  - The first code train in #656 (merged; eight commits). It replaced #654 in the same way.
  - ADR-0105 B1 in #652 (merged `e5af27fe`).
- **Owner of the request:** the operator. **Priority:** P0; everything else parks.
- **Research:** [Self-recovering agent loops and Claude Code fleets in 2026](../research/self-recovering-agent-loops-2026.md). This is the state of practice that this design is compared against. Recommendations R1–R14 map onto these components.

## 1. The request

Three statements, verbatim, in the order given on 2026-09-26.

1. "I want each phase to focus on LOGIC, if the failed reason is related to 'format' or process related errors, the pipeline should try to recover it, not blocking. Because the most important deliverables should be the logic to solve the particular issues, building features, completing the quests, building the architecture that sustains through different changes, having the flexibility to adjust through multiple changes requests while maintaining the same quality. Make this as the highest priority and policy for refactory the pipeline."
2. "I want you to put this refactory quest as the P0 and drop everything to make it happen first. Focus on the logic, other format related errors can be fixed through recovery agent to help structure the deliverables around the core logics."
3. Goal as set: "prioritize the logic in code not the format and process, the pipeline should assist and check and recover it to its original code intention, the delivered code and doc should speaks for itself, if agent made error by following the format / structure, we should assign it back or recover if it delivers enough evidence for building / fulfill the request. ultrathink to design and architecture"

Standing constraints that shape the design:

- Every fix and feature is decomposed into small components. Each component is tested and landed on its own.
- Config over Go literals.
- No feature flags.
- One source of truth, never a duplicate.
- Pipeline integrity outranks throughput.
- A passed audit is trusted and never blocked by a process issue.

## 2. Goals and non-goals

**Goals**

- A cycle whose logic is right ships even when a deliverable's form or a process step is wrong.
- Every block names a defect in the change's behaviour, or is an integrity block; nothing else is final.
- Recovery is cheap, bounded and honest: it regenerates projections of existing evidence and never substitutes for missing logic.
- The pipeline decides between assigning work back and recovering it on evidence the kernel computed, never on an agent's account of itself.
- The delivered code and its explanation document speak for themselves; reports are projections of them.

**Non-goals**

- Softening any judgment. An audit FAIL on substance, a red predicate, a security finding: all stay final.
- Recovering an audit FAIL on narrative fidelity (a report that misdescribes the diff). That is a judgment; a later ADR can revisit it with evidence.
- Changing the gate's standard of well-formedness. Recovery changes who writes a file, never what passes.
- Unbounded retries. Every rung is budgeted from config.

## 3. Vocabulary

| Term | Meaning |
|---|---|
| **Logic block** | A defect in the change's behaviour: a failing or missing test, a regression, a security finding, an audit finding on substance. Final. |
| **Form or process block** | The deliverable's shape, a derivable file, a binding to a tree that moved, a race, a transport hiccup, bookkeeping. Recovered; final only when recovery is exhausted. |
| **Integrity block** | Evidence that the kernel's account of the cycle cannot be trusted: the treefence, the predicate-authority fence, ADR-0072 incoherence, a sandbox or recovery-guard violation. Never recovered: the cycle aborts and a P0 is filed. |
| **Evidence** | What the kernel computed or owns. Examples: the diff against the worktree base and the host's own test runs. Also a parse of the shape of a document, the acceptance of the inbox item, and the base SHA. Never an agent's self-assessment. |
| **Projection** | A report or secondary file that restates evidence in a contracted shape (`build-report.md`, `triage-decision.json`). A defect in a projection is form. |
| **Recover** | Regenerate a projection from evidence, toward the original intention (the task contract's acceptance, the intent document). |
| **Assign back** | Re-dispatch the owner of the phase on its preserved worktree. Keep the diff and every green run. Send a correction that names the missing evidence, never the missing format. |
| **Rung** | One step of the correction ladder: salvage, live-fix (dormant), recover, re-dispatch. |
| **Decision-bearing field** | A field only the phase's owner is allowed to author: a verdict line, `## AC-Materialization`, the deliverable-kind and cycle-size headers, `top_n`/`deferred`/`dropped`. |

## 4. Principles (operating-policy §0)

1. Three kinds of block; only logic and integrity are final.
2. Recovery is layered, cheapest first: a host derivation before any judge → deterministic rungs → the recovery agent → a re-dispatch.
3. Recovery never launders. Five rules apply:
   - The gate that rejected a deliverable judges its repair.
   - The verdict is re-earned, never carried.
   - Judgment and control phases are never repaired.
   - The kernel proves that nothing outside the grant changed and that every added line has a source.
   - Every recovery is recorded where the auditor and the dossier see it.
4. The evidence decides between assigning back and recovering. A missing deliverable is always assigned back.
5. Bounded and loud: recovery rounds come from config. Exhaustion is recorded as a pipeline defect and still counts toward the ADR-0072 halts.

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

A missing or empty primary never enters the ladder: it is assigned back. The pipeline cannot tell a finished phase that forgot its report from one that stopped early. For a contracted phase, the report is where the owner declares the outcome.

### 5.2 The process ladder at ship (ADR-0105)

A passed audit meets a moved `main` at ship for reasons that touch none of its files. Examples are a landing of a peer and a closeout dossier of a sibling. The rungs of ADR-0105 carry the verdict across a tree change that provably did not touch the change. The rungs are B1 unwind-rebase-pend, B2 explanation rebind, and B3/B4. P2 keeps the dossier of a fleet lane out of `main` until the wave boundary. These rungs are deterministic and have no LLM.

### 5.3 What speaks for itself

The change is the code, its tests and its explanation document (ADR-0102). Everything a phase writes about the change is a projection. Recovery is allowed to regenerate a projection; it never touches the change itself, which lives in the fenced worktree.

### 5.4 The agent's identity and its pane (P3)

An agent that inspects its environment must recognise its own traces. The tdd agent of cycle 1707 listed the tmux sessions, found its own, and read its own prompt file. It refused the phase as a prompt injection racing "the real agent". A one-line clarification of the operator an hour later resumed it. The block that says that line before the agent needs it is [`internal/bridge/phaseidentity`](packages/internal-bridge-phaseidentity.md).

- **Finished by the driver, appended.** The engine composes the prompt before a session exists. Only the tmux driver knows the session name, so `prepareTmuxREPL` appends `phaseidentity.Block` to the bytes that it pastes. The block goes after the composed prompt for two reasons.
  - The bytes of the engine stay a byte-identical prefix (the cached skill and policy blocks hold across dispatches).
  - The deliverable path stays the last thing that the agent reads.
  - Separately, `resolved-prompt.txt` is the exact pasted bytes, so the block is auditable per phase.
- **In the prompt, not the system prompt** (revised 2026-09-28, §5.11 T3). `--append-system-prompt` exists for claude only. The block is for every tmux CLI and lands in the same bytes for each. Cycle 1734 showed the cost. Claude declined the whole paste, the block included, as "untrusted pasted text, not something you asked me to do". It declined the typed nudge of the bridge in the same way.
  - The fix splits what varies from what does not.
  - The facts (phase, cycle, session, prompt files, sole writer) still end every paste for every CLI. So the placement rule of this section and its byte-identical prefix hold.
  - A standing authority block (`phaseidentity.Authority()`) states three points. The operator launched this unattended pipeline. The pasted prompt is the instruction of the operator. The interactive conventions of the console are not the conventions of this pane.
  - That block is the system prompt of claude-tmux. There its bytes are the same on every spawn, and the system-prompt cache of the provider holds (`--exclude-dynamic-system-prompt-sections` of ADR-0071).
  - A CLI without the channel pastes the block before the facts.
- **Null object, and only true claims.** No agent name or no pane gives no block. A phase whose answer the bridge reads from the pane (`completion: stdout`) gets no sole-writer claim. The reason: a statement meant to stop an agent from doubting its own facts must never contain a false one.
- **Suggestions off through the environment.** The prompt suggestion of claude is a background model request per turn. It also shows dim text under the input box that reads like agent output. `CLAUDE_CODE_ENABLE_PROMPT_SUGGESTION=false` takes precedence over the setting. Unlike a `--settings` flag, it is honoured under the `--setting-sources project` of the profiles.
  - The channel is the `default_env` of the manifest, realized once (`Realization.Env`). The tmux boot exports it in the pane. The headless drivers pass it to the process.
  - The parser validates keys as shell identifiers. It refuses the variables of the loop, of the bridge and the credential variables.
  - Every fact that the block renders is stripped of control bytes and backticks (security review, 2026-09-27).
- **What the block does not fix.** The CLAUDE.md of the repository is written for console operators, and a phase agent reads it.
  - The last line of the block answered that in-prompt.
  - Since T3 (§5.11) the standing authority block answers it from the system prompt of claude. It names the conventions (confirm direction, stop and ask) that are not the conventions of the pane.
  - The content itself belongs to the target repository.
  - The twelve profiles that repeat the same claude flag list are a later centralization into the `default_args` of the manifest.

### 5.5 Capacity is not a verdict (the wave-14 deep-dive)

Wave 14 (cycles 1708 and 1709) failed with every CLI family walled:

- claude-tmux answered `auth_recheck` fourteen times (a credential wall; the `/login` of the operator cleared it).
- codex-tmux answered `rate_limit` eight times and `model_unsupported` six times (the deep pin that the account rejects).
- ollama-tmux refused the source-writing phases.

No logic ran. The orchestrator already treats a first-dispatch exhaustion as a **deferral**:

- `pauseForQuota` emits `quota.paused` and writes the quota checkpoint.
- It records the abort with the `all-families-exhausted` prefix that `cyclehealth` classifies DEFERRED.
- It skips failure learning.
- The batch returns rc 5, so the chain waits with the checkpoint intact (1709 took this path).

Three seams do not reach that one function:

| Id | Component | What is wrong today | Design |
|---|---|---|---|
| Q1 | exhaustion during a correction re-dispatch is a deferral | `cyclerun_correction.go` wraps the `ErrAllFamiliesExhausted` of `runner.Run` as "correction N dispatch failed". It records a FAIL outcome, writes a failure lesson and a digest, and lets the retrospective dispatch (which hits the same wall). 1708 was sealed FAIL this way | The correction loop, the remediation gate re-run and the resume review gate check `errors.Is(err, ErrAllFamiliesExhausted)` and return through `pauseForQuota`: one seam, one classification. A test per caller pins the DEFERRED prefix, the `quota.paused` signal, the `all_families_exhausted` ledger kind and the absence of a lesson |
| Q2 | a credential wall benches the family and names the operator | `clihealth.Benchable` benches `rate_limit`/`exhausted` only. So `auth_recheck` was re-dispatched to claude fourteen times in one wave, and the halt read as quota | `auth_recheck` benches the family with the same strike-scaled cooldown that a quota wall gets (30 min doubling to 4 h). The family stays active for routing until a probe clears it. See the Q2 notes below the table |
| Q3 | a deferred cycle never counts toward the halts | the consecutive-failure breaker reads every `failure-digest.json`. A deferral must never write one (1709 did not; 1708 did, through the defect of Q1) | A test pins that no digest exists after a deferral through every seam that Q1 covers. The zero-ship halt rule counts deferred lanes as neither shipped nor failed |
| Q4 | a fallback chain never appends a CLI that cannot serve the phase | the universal fallback appended ollama-tmux to the retrospective chain; it refused the source-writing phase with exit 10 and the refusal became the retro's FAIL | the chain builder filters candidates by the phase's requirements before appending them; a refusal is impossible by construction |

Q2 notes:

- The cooldown only schedules the next probe of the canary, and the login of the operator makes that probe succeed.
- A login pane never sets the bench from its stale reset hint.
- The fix names no login command, because the logins of the families differ (agy trusts a directory, ollama signs in).
- The fix is durable on the bench entry (`operator_action` in `cli-health.json`). The bench line of the chain walker and the re-bench line of the canary print it.
- Trade-off: the canary spends one probe per cooldown (30 min → 4 h) against a CLI that needs a login until the operator logs in.
- Trade-off: a bench is advice to the chain, not a veto. So a judgment phase that the balanced-tier floor pins to claude still dispatches into the wall and then defers through Q1.
- Not built: a Signal Center ERROR code for the wall, and a defer message of the loop that distinguishes "wait for the operator".

Q1–Q3 land before the next soak wave. Q4 is a routing hygiene follow-up. None of them changes a verdict: a capacity wall stays a deferral, a credential wall becomes an operator halt, and the FAIL streak counts logic.

### 5.6 A pin without an inbox item (the wave-15 deep-dive)

Wave 15 (cycles 1712 and 1713) was the first wave on the P3/Q1/Q2 train. Lane 1713 was pinned to `gittest-fixture-centralize`, sealed `SKIPPED_UNKNOWN` with `triage-empty-commitment` and shipped nothing, and the same pin had already cost 1709 and 1710. The chain, from the run dirs:

| Cycle | Lane pin | Decision | End |
|---|---|---|---|
| 1706 | `tempdir-cleanup-vs-git-flake` | `top_n: [gittest-fixture-centralize]`: an id triage authored, with no inbox item behind it | shipped 51edfb7f; the promote no-op'd ("not found: already moved?") |
| 1709 | `gittest-fixture-centralize` | `top_n: [fromgit-test-gittest-migration, treestate-test-gittest-migration]`: two more authored ids | deferred on capacity (exit 85) |
| 1710 | `gittest-fixture-centralize` | none | deferred |
| 1713 | `gittest-fixture-centralize` | first report: `## superseded` with nothing under it, declined ("states neither cards nor none"), so `missing_secondary` and a correction; second report: `dropped: already-shipped: 51edfb7f` | planned no-work |

Two defects, one of form and one of lifecycle, neither of logic:

- **H1b: an empty optional bucket is no cards.** The strict reader (H1) accepted a present bucket only when it stated cards or `none`. An agent that has nothing to say writes a heading followed directly by the next heading. The strict reader declined it. This turned a complete answer into a gate rejection and a re-dispatch.
  - `deferred`, `dropped` and `superseded` left empty now derive to `[]`.
  - `top_n` left empty is still a missing commitment and still declines. An absent or empty `top_n` must never read as "commit to nothing" by accident (the invariant that H1 already pinned for the absent case).
- **W1: an id no inbox item backs leaves no retirement evidence.** Every retirement door tolerates such an id by doing nothing:
  - `Promote` no-ops ("a missing item never blocks the ship").
  - `RecordRootTaskFailure` no-ops.
  - `ClaimLaneScope` skips with a WARN.
  - The no-work closeout treats it as "simply gone".

  Every reader that decides if it must run the id again reads the same inbox lifecycle. By design, these readers keep an id with no evidence:

  - The prune of the wave planner fails open (over-pruning starves the wave).
  - The freshness probe of the launcher calls it fresh (not every planned id is inbox-backed).

  So the moment triage authors an id and the cycle ships it, nothing in the system can ever say that the id is done. The prior-decision carry re-pins it into a lane every wave until the triage of a lane happens to drop it. Even that drop reaches no reader. Three lanes went to it in one soak.

The design keeps one source of truth. The two doors where such an id is finished write the evidence that the readers already consult. The evidence is a record in the retirement dir where `Promote` moves a backed item:

| Door | When | Record | Reason |
|---|---|---|---|
| the PASS seam (`ApplyCycleOutcome`, through the post-commit closeout of ship) | a committed id that `Promote` was unable to move and no inbox item backs | `processed/cycle-N/<sha8>-<id>.json` | `ship-promote-processed: no inbox item backs the id` |
| the planned no-work closeout (`ApplyNoWork`) | every scoped id that no inbox item backs, answered or not | `rejected/cycle-N/<id>.json` | the own answer of the lane (`lane triage (cycle N) dropped: already-shipped: 51edfb7f`) or `the lane's triage answered for nothing` |

`inboxmover.RetireUnbacked` is the one door. It writes only for an id whose `ResolveDispatchState` is unknown. So an inbox-backed item keeps its own lifecycle. A no-work lane routes it to the console in place and never retires it: the anti-laundering rule stands. An already retired id is never written twice.

The record carries `id`, `unbacked: true`, `retired_reason`, `retired_cycle` and the ship sha. `FindFileByTaskID` finds it like any item. The plan-time prune and the dispatch-time probe drop the id on their next read. A FAIL writes nothing: the id must stay retryable.

What W1 does not fix: a failing authored id still has no durable failure count, so the S5 retry ceiling cannot reach it (W2, designed). The lane pin that started the chain (1706 committing an id other than its pin) is the choice of triage and stays legal. The retirement makes it finite.

**X1: a host-owned inbox record is not the builder's to explain.** Lane 1712 (the third cycle on `lineage-datestamp-normalization`, after 1707 and 1708) sealed FAIL. The code was verified: ACS 10/10, EGPS red 0, a mutation probe that catches a partial fix. The one defect was a sentence in the explanation document. The chain was:

1. The material-path set of the build floor included `.evolve/inbox/2026-08-05T15-30-00Z-lineage-datestamp-normalization.json`. Twelve corrections across the three cycles of the lane demanded that path.
2. The builder wrote "content unchanged" for the item that it moved to `consumed/`.
3. The in-commit consumption of ship then stamped that file with a `consumed` block (`via: ship`).
4. The explanation review of the auditor found the sentence contradicted by the diff (`NEEDS_CORRECTION`). ADR-0102 turns this into the FAIL of the audit.

The builder had to describe an effect that the host produces after the build, and the audit judged it on that effect. `nonMaterialPrefixes` exempted `.evolve/runs/`, `.evolve/worktrees/` and `.evolve/evals/`. It did not exempt `.evolve/inbox/`, whose records the host claims, moves, stamps and retires. It now exempts it (`TestMaterialPaths_InboxLifecycleRecordsAreTheHosts`). The auditor reads the same host-derived set, so the review has nothing to demand for the path either.

**X2 (designed): an inaccurate explanation of a verified build is a form failure.** ADR-0102 made the gate record the `NEEDS_CORRECTION` of the auditor as an advisory. The auditor persona makes it the FAIL of the audit, so that nobody can wave the document through. Since 2026-09-28 the persona states this as deliberate policy until the rung of X2 exists, and names only the document in the FAIL reason. Since 2026-09-29 such a FAIL repairs at Build alone, not from TDD: X2a, §5.14.

Under this policy the judgment stays and the verdict changes. When the code is verified and the review names only the document, the correction is a recovery rung on the document. The recovery agent of F4/F6 does it, or the host does it for a mechanical claim such as the similarity of a rename. Then the review runs again, and the cycle ships.

Only a review that names the code blocks. X1 removes the manufactured case. X2 removes the class.

### 5.7 A card on a protected surface is a route, not a verdict (wave 16, cycle 1714)

Wave 16's first cycle on the W1/H1b/X1 main was pinned to `file-size-decomposition-backlog`, an inbox item with no declared files. Its triage named `go/internal/core/orchestrator.go` in the card. The classify hook refused the report with `TRIAGE_PROTECTED_SURFACE`. The cycle sealed FAIL after two phases, and the per-item breaker of the closeout (F30) routed the item console-manual. The item did go where it belongs, but the verdict did not. Three facts made it a class, not an accident:

- **Before the fix, the planner had no way to judge it.** The protected predicate of the wave planner and the prompt partition judge the declared files of an item. An item with none is disjoint with everything and always pickable. The surface is known only once triage names files.
- **The agent was never told.** The triage persona did not mention protected surfaces. The prompt listed only the console-routed items excluded from its menu. The only way for the agent to learn the rule was the refusal.
- **The refusal was a verdict.** A deterministic, operator-owned routing fact (the same card refuses every time, and the console decides) ended the cycle as the FAIL of the task. No logic was judged, the streak broke, and the route of the item came from the failure path.

The design keeps everything that F30 built and moves the decision one step earlier, into the hands of the host:

- **R1: the classify hook records the route instead of refusing.**
  - Every top_n card whose `files=` names a lane-forbidden path leaves `top_n` and joins `escalate_block` (`protectedTopNCards`, all of them, in report order). The reason is `protected-surface: <path> — control-plane changes go through the console route (operator-gated), not lane top_n`.
  - `routeProtectedCards` does this as a rewrite of `triage-decision.json`. The rewrite keeps every other key and is idempotent.
  - The hook returns PASS with one warning per card (`TRIAGE_PROTECTED_SURFACE`, severity warning, subject the card).
  - From there the existing machinery does the rest, unchanged:
    - An emptied `top_n` is the planned no-work end (`HasEmptyTriageCommitment`, F30).
    - The escalation is the answer of the lane (`committedset.Dispositions`).
    - The no-work closeout routes the item console-manual with that reason (`ApplyNoWork`).
    - A `top_n` that still holds other cards continues the cycle on them, so the lane is not wasted.
  - A route that cannot be recorded fails closed with the same code as an error. The same happens in two more cases.
    - A card that the host cannot name (no id).
    - A card that the decision never committed (the report and its derived decision disagree).
  - A protected card must never reach the spine. The rewrite proves that every routed card left `top_n` or was already escalated.
  - The host effects run before the judge, so the decision file exists when the hook rewrites it.
- **R1b: an emptied single-item lane answers for its pin (cycle 1757, 2026-09-29).** The id of a top_n card is the name that triage gives to a piece of work. In a lane pinned to one inbox item, it is often not the pinned id. Triage splits the item into sub-task cards, or it copies the slug of the scout. In 23 of 140 lane decisions from 1604 to 1757, the card ids were outside the pin.
  - R1 escalated the card under its own id. So when the route emptied `top_n`, the pin stayed unanswered. `unansweredClaimableWork` found claimable work, and the lane sealed FAIL (`triage-empty-commitment-claimable-work`) with no route and no `failure_count` bump. The item stayed first in line for the next wave.
  - Cycle 1757 did exactly that. The scout named the work of `goal-text-has-no-selection-authority` as `goal-text-selection-authority`. Its files reached `go/internal/loopwave/loopwave.go`. The route escalated only the alias.
  - Now the route escalates the pinned item as well (`withBoundItem`) when three conditions hold:
    - The route leaves `top_n` empty (`routeProtectedCards`).
    - The binding of the cycle is exactly one id (`committedset.Committed`: the pin of the lane, which outranks the labels of triage, minus what triage deferred).
    - The decision has not already answered that id (`committedset.DispositionsFrom`).
  - The reason of the new escalation is the reason of the first routed card, with the id of that card as suffix (`the lane's item, answered for by routed card "<id>"`). So the `routed_reason` of the item names what caused the route.
  - The existing machinery then ends the lane as planned no-work, and `ApplyNoWork` routes the item console-manual.
  - Unchanged:
    - A lane that keeps any committed card (the item is still in work).
    - A pin that triage deferred (a deferral is owed work, F30, so the lane keeps its claim-failed end).
    - A sequential cycle (its binding is its own `top_n`, every id of which the route just removed, so nothing is added).
    - A lane pinned to several items, where a card cannot be attributed to one of them. That last shape still seals claim-failed. R1c below makes its FAIL charge the owed pins.
- **R1c: the host records how triage ended the cycle at the C1 chokepoint (2026-09-29).** A claim-failed lane then charges its pins, and a planned no-work end is NO_WORK. Two host terminations escaped the C1 invariant of ADR-0044. That invariant says that every terminal path records a ship PASS, a salvage or an abort reason. The claim-failed FAIL and the planned no-work end both left the own PASS of triage as the last C1 entry. The results were:
  - Cycle 1757 (claim-failed) classified `integrity-breach` (system-level). `FailureInputsFor` released its item and bumped nothing. The committed set of the FAIL closeout was the empty `top_n` anyway. A rule of 2026-08-10 read an empty `top_n` as "declined" before F30 made a decline an answer.
  - Cycle 1758 (planned no-work) paged `FAILED_UNEXPLAINED` and self-filed a HIGH inbox defect.

  Now:
  - The closeout of `completeCycle` (`recordPlannedNoWorkOutcome`) records each ending through the new `Recorder.RecordEnding` (`recordTriageEnding`). The record is a triage entry with the verdict of the host and its termination reason as the abort reason.
    - The claim-failed entry is FAIL. It carries the own error diagnostics of triage first (a coded triage refusal is never shadowed). Then, in a lane, it carries `TRIAGE_SCOPE_UNANSWERED` that names the unanswered pins (`committedset.Unanswered`).
    - The planned no-work entry is SKIPPED with `triage-empty-commitment`.
    - An ending is not a phase run. `RecordEnding` appends the C1 entry and nothing else. It leaves `PhasesRun` and the usage sidecar alone and emits no `phase.outcome`. A planned no-work end otherwise prints a `phase.aborted` WARN, and the seal of the cycle already reports how it ended.
  - The existing readers do the rest.
    - `cycleclassify.classifyFromRecord` reads the code of the lane as the task-level refusal (`RefusalDisposition(TRIAGE_SCOPE_UNANSWERED)`).
    - A sequential cycle has no pin and no code, so its classification and batch halt are unchanged.
    - `cyclehealth.ClassifyOutcome` explains the claim-failed FAIL by its abort reason. It has one new arm, `NO_WORK`, for the no-work reason (beside the prefix arm of DEFERRED).
  - The FAIL closeout (`cycleoutcome.ApplyFailure`) charges `committedset.Unanswered` when triage committed nothing. The charge is the lane pin minus the answers of the decision, in pin order.
    - A deferred pin is owed and charged.
    - An answered pin is not charged.
    - An absent decision owes the whole pin.
    - `core.unansweredClaimableWork` reads the same projection. Its empty-scope gate reads the pin through the same `committedset.LanePin`.
  - Found on the way: a zero-try record (`recordChokepointEscape`, a missing runner) rewrote the usage sidecar of the phase with zero cost. This erased the usage of the dispatch that `cyclecost` sums. `Recorder.Record` now leaves an existing sidecar to a zero-try record.
  - Rejected: reading the seal of the cycle from `signals.ndjson`. The architecture review blocked it, for three reasons:
    - The rule of the Signal Center is that listeners observe and never decide (its §3, review 10).
    - The stream can drop or delay events by design.
    - The risk was that the field names of the seal are copied into the deciding code with nothing that pins producer to reader.

- **R2: the agent is told.** The inbox section of the triage prompt names the protected surfaces (`guards.ProtectedSurfaceManifest`, the one list) and the drop reason `protected-surface: <path>`. The drop rule of the persona carries the same sentence. An agent that follows it never produces a card that the host has to move. An agent that does not follow it is corrected without a verdict.

`RefusalDisposition(TRIAGE_PROTECTED_SURFACE)` stays as it was (task-level, route console): it now governs only the fail-closed case, where the code still travels on a FAIL.

### 5.8 The carry: a byte-identical rebase ships without a second audit (P1 = ADR-0105 B3 and B4)

Cycles 1712 and 1715 each passed their audit. Each cycle then followed this path:

- It met a moved plane `main` at ship (a `dossier: cycle-N closeout` commit of a sibling, the dominant contention with two or more lanes).
- It unwound (B1) and rebased byte-identically.
- It rebound its explanation (B2).
- It re-audited an unchanged tree for seven to ten minutes, with a second chance for the verdict to flip.

The re-audit is the rule today, not an accident:

- `routeRebasedExplanation` returns Audit after the rebind and never consults the carry-forward rungs. They run only for a cycle without the explanation contract. No cycle is in that case any more.
- The existing RUNG 0 (`compositionCarryForward`) diffs commits (`git diff main...HEAD`). After B1 the change is pended in the index. So the composed diff is empty, and the patch-id can never match.
  - **Outcome (2026-10-21):** RUNG 0 is retired. Its snapshot also diffed `main...git_head`, and `git_head` is the tip of main when the audit was bound. The live ledger held 45 carry records, all `identical-rebase`. Every new cycle has the explanation contract, so C3 serves the callers of RUNG 0. A legacy cycle with no contract tries RUNG 2, then goes to Audit with a WARN that names the reason. Ship accepts no `trivial-rebase` record.
- Ship has no reader of carry records. `internalAuditBoundTreeSHA` must equal the staged tree (or differ only by sanctioned inbox consumption). So ship refuses even a carried verdict as tree drift.

The design is ADR-0105's, in four components, each its own commit, unwired before wiring:

| Id | Component | What it proves or does |
|---|---|---|
| C1 | `treedelta.Delta(base, tree)` and `treedelta.Identical(base0, T0, base1, T1)`, a leaf both core and ship use (core's `treeDelta`/`identicalChange` delegate) | the byte-exact change (`diff --binary --full-index --no-ext-diff --no-textconv --no-renames`) of the audited tree on its base equals the pended tree's change on the new base, and is not empty |
| C2 | the carry record: `composition-verdict{method:"identical-rebase"}` gains `AuditedTreeSHA` beside `TreeStateSHA` (the composed tree) | ship can match the record to the audit it carries (`LaneAuditRef` = the audit artifact's SHA). It also matches the audited tree that the record binds and the tree that ship will commit |
| C3 | `identityCarryForward` after the rebind (B3). The steps are in the C3 details below the table | a rebase that changed no byte of the change ships on the verdict it already earned |
| C4 | ship accepts a carry (B4), inside the one binding rule (`auditBindingSatisfied`). The steps are in the C4 details below the table | ship never trusts the writer, and a carry cannot launder a change |
| C5 | the debugger resolution rejoins the rebase (B5). The steps are in the C5 details below the table | a resolved conflict never re-audits on the base that the ship diverged from. Cycle 1719 paid one audit round and one ship try for that (§5.9) |
| C5b | the resume heal for an interrupted re-entry. A rebase left in progress is aborted and a carrier commit pended. So `evolve loop --resume` continues and does not end the cycle on a mutated tree | designed | `core/resume_execution.go` |
| C6 | the composed-tree gates run in the CI environment (2026-09-29). See the C6 details below the table | shipped | `ciparity/cienv.go`, `phases/audit/ciparitygate/tierenv.go`, `cmd/evolve/cmd_composition_wiring.go` |
| C7 | the carry reaches the predicate binding of ship (2026-09-30). See the C7 details below the table | shipped | `phases/ship/audit.go` |

| C8 | the evidence of the carry outlives its worktree (2026-10-06, ADR-0123 Phase 1, F5). See the C8 details below the table | built (branch `fix/carry-artifacts-durable-f5`); the plane repair runs at a boundary | `internal/ledgerartifacts`, `adapters/ledger/composition.go`, `adapters/ledger/evidence_restore.go`, `core/composition_carryforward.go`, `phases/ship/carry.go` |
| C9 | the carry composes with the inbox consumption of the re-ship (2026-10-07). See the C9 details below the table | built (branch `fix/carry-composes-with-consumption`) | `phases/ship/carry.go` |

**C3 details (B3).** After the rebind, `identityCarryForward` runs these steps:

- The auditor row of the run names three items: T0, the audit artifact, and the base on which its worktree stood. The source is `latestAuditEntry` (`worktree_base_sha`, else `git_head` on an older row).
- That base must be base0 = the base before the rebind. T1 = `git write-tree`. base1 = `HEAD`.
- C1 holds.
- The composed-tree gates run under `treefence`. The index must still be T1 afterwards.
- The record goes to the root ledger that ship reads.
- Return Ship, else Audit.

**C4 details (B4).** Ship accepts a carry inside the one binding rule (`auditBindingSatisfied`). The case: the tree that ship holds is not the bound tree. Ship then re-proves the newest `identical-rebase` record for this audit that meets two conditions:

- `LaneAuditRef` = the SHA of the audit artifact.
- `AuditedTreeSHA` is the bound tree, and `TreeStateSHA` is the tree held.

The re-proof checks four points:

- The ledger chain verifies (0.4 s on a 147K-line ledger).
- The ancestry of both bases, in the own directory of the tree.
- The byte equality of C1.
- The patch-id of the record is the patch-id of the proven bytes.

The plane-HEAD comparison stays for the resume detection.

**C5 details (B5).** The case: the recovery that the debugger served began as a fleet rebase. It is inside the ship-recovery budget and after the contention backoff. It is never on an in-place worktree. Then:

- The tracked state of the resolved worktree is carried as a commit on the fork point.
- That commit is rebased onto main, pended on the new base and routed as any rebased change (`resumeFleetRebaseAfterDebugger` after `decideAfterDebugger`).
- A Build or Audit re-run that the debugger asked for still runs, on the rebased tree.
- A tests-first request yields to the route and says so. The post-TDD refresh of the explanation contract cannot follow a moved binding.
- A conflict that survives, a rebase that the host cannot finish or a spent budget ends the cycle. The end shows on stderr and as `ORCHESTRATOR_REBASE_REENTRY_ABORTED` in the Signal Center.

**C6 details (2026-09-29).** The composed-tree gates run in the CI environment.

- `runComposedGates` ran `make -C go <target>` with the full environment of the lane and discarded the output. The CI-parity gate of the audit scrubs exactly that state.
- `test` went red 14 times in waves 27–41 (18 carries). Each decline cost a deep re-audit that passed. The log said only which gates.
- Reproduced on a clean main: `make test` under lane state exits 2, deterministically per test, whenever `EVOLVE_WORKTREE_ROOT` reaches the run. `TestFlags*` resolve their root through `sourceRoot()`, which prefers it, so `generate` was able to write into the lane worktree too. `TestPredicateEnv_AllBranches` inherits it from `os.Environ()`. Why 18 carries passed is unverified.
- Now one allowlist serves both: `ciparity.CIEnvAllowlist` with the pure `CIEnv(environ)`, moved out of `ciparitygate`.
- A failing composed gate prints its name, target, worktree and a bounded output tail (`composedGatesTo(log)`, `tailWriter`).
- Follow-ups: a coded decline event, hermetic flags/predicate tests.

**C7 details (2026-09-30).** The carry reaches the predicate binding of ship. No carry had ever shipped.

- `verifyAuditBinding` checks that the tree on which the predicates of the audit ran is the tree that ship holds (#535, older than C4). It compared the two directly.
- C4 put the carry only inside `auditBindingSatisfied`, which that check never asked. The proof test that ADR-0105 named was never written.
- Cycles 1766, 1768 and 1772 each carried (`rebase is byte-identical: the audited verdict carries`). Each then failed `SHIP_AUDIT_BINDING_TREE_MISMATCH` with exactly the `audited_tree_sha` and `tree_state_sha` of the record. Each paid a deep re-audit.
- `verifyExecutionTree` now applies the one binding rule to a drift. `TestVerifyAuditBinding_ShipsACarriedRebaseWithoutASecondAudit` drives the real binding on a carried rebase (red on the old check).
- Still open: a test through the real recovery and the later stages of ship, and three binding sites that compare directly (inbox `carry-recovery-to-ship-end-to-end-proof`, `carried-ship-resume-binding-sites-skip-the-rule`).

**C8 details (2026-10-06, ADR-0123 Phase 1, F5).** No carry had shipped even after C7.

- `identityCarryForward` wrote its diffs into the cycle worktree and its record into the project ledger. The re-proof of C4 runs a whole-ledger Verify, which treats a missing diff as a broken chain.
- Once the worktree of 1766 was cleaned up, the plane ledger broke at line 154453. Every later carry was declined (1768, 1772, 1773, 1782, 1810), with `error=<nil>`.
- Now the ledger writer stores the diffs in `.evolve/ledger-artifacts/` (content-addressed, gc-protected) and records their sha256.
- `evolve ledger evidence restore` rebuilds the six lost ones from git and chains a record that names them.
- Rungs 0 and 2 write through the project ledger, not the linked ledger of the worktree.
- A declined carry names its reason in the TREE_MISMATCH message and as `SHIP_CARRY_NOT_REPROVEN`.
- Verify reads the chain and the tip under the ledger lock.

**C9 details (2026-10-07).** The carry composes with the inbox consumption of the re-ship.

- C4 accepted only a held tree equal to the `TreeStateSHA` of the record. The consumption rule, tried next, measures from the audited tree.
- The re-ship of a carried lane consumes its inbox items again (ADR-0105 B1). So it holds the tree of the record plus the consumption, which neither rule accepted.
- Cycle 1825 (wave 75) was sealed FAIL on a false `INTEGRITY_TREE_DRIFT` at the pre-commit check. Its "unsanctioned" paths were exactly the 12 of the peer.
- `carryExplains` now requires that the drift from the tree of the record to the held tree is only the sanctioned consumption. It re-proves the carry on the own tree of the record, inside `auditBindingSatisfied`. So the pre-commit, post-push, direct-path and verify-class checks all compose.

**Correction (2026-10-06).** C3, C4 and C7 were marked shipped, but none of them made a carry work live. Each was proven with records whose diffs sat in `repo/.evolve`, where no cleanup removes them. The live carry kept them in the cycle worktree (ADR-0105 F5, which the 2026-09-26 design review had said must land with B3). C8 is the missing part. The [incident](../incidents/2026-10-06-carry-refused-ledger-verify-f5.md) has all six occurrences.

What it does not change: a rebase that the proof declines still returns to Audit (or Build when the change is not identical). The audit-bound tree of a cycle that never rebased is checked exactly as today.

One scoping, stated: ship re-proves the ancestry and the bytes itself and verifies the ledger chain that holds the record. It does not re-run the composed gates (build, test, acs, apicover) a second time. The writer refuses a record whose gates are not green, and the chain proves that the record came through the writer. So a ship-side gate check, if added, is dead code.

The orchestrator also declines a carry when the audited worktree stood on another base than the base on which the change was authored. The `git_head` of the row is the HEAD of main when the audit was bound, not that base. Cycles 1724 and 1727 paid a second audit to show this. Re-running the gates in ship (C4b) is a follow-up, if a forged, re-chained ledger is ever a credible threat.

### 5.9 The cycle's own record is not history (wave 18, cycle 1718)

Cycle 1718 (`profiles-test-hygiene`) built a test-only change: five `internal/profiles` test files, a doc, an eval and its ACS predicates. The builder explained it. It wrote the explanation document of the cycle at the canonical path and declared it. The contract classes unambiguous test files as non-material, so the material set of the host was empty. The no-material branch of the build floor refused the handoff with `not-applicable Build changed immutable cycle explanation record(s): docs/explain/builds/cycle-1718-….md`. That message named the document that the build had just written as if it were published history.

The correction rung re-dispatched the builder (a codex deep launch that bounced on `model_unsupported`, then claude deep). The builder deleted its own document and declared `NOT_APPLICABLE`. The second handoff passed. The cycle paid a full builder dispatch to remove a document that contradicted nothing.

The defect is an asymmetry, not a policy:

- The material branch already excluded the own path of the cycle from the immutability rule (`foreignHistoryFailures(paths, current)`).
- The no-material branch did not (`changedCycleRecords(paths)`).
- The message called the deliverable of the builder immutable.

Under §4 the facts are:

- The diff is non-material (the judgment of the host, from Git).
- The record is the own record of the cycle (its path binds the cycle and run id, and it did not exist at base).
- The builder declared it.

No material change is hidden and no published record is rewritten, so nothing is allowed to block.

X3 makes immutability one rule in every branch and lets the declaration choose the path:

| The diff | The cycle's own record | The declaration | Before | Now |
|---|---|---|---|---|
| material | present | REQUIRED | verified | verified (unchanged) |
| non-material | absent | NOT_APPLICABLE | N/A | N/A (unchanged) |
| non-material | present | REQUIRED | refused as history | verified like any explanation, with an empty material set. The builder explained a non-material change. `Changed Areas` is allowed to cite only Build content: exact diff paths or patterns that cover them |
| non-material | present | NOT_APPLICABLE | refused as history | N/A. The undeclared record rides as documentation. It is the own record of the cycle, newly added, and binds no verification. Its content is not validated and no review is owed to it. Once landed, it is a published record, immutable to every later cycle |
| any | a foreign record changed | any | refused | refused: `published cycle records are immutable`. One message, every branch |

The Verify mirror (audit, ship, `VerifyLanded`) branches on the recorded status, not on the material set. So a required view with an empty material set verifies at ship exactly as at build. Its own foreign-record check sat behind the diff-SHA check. Any path added after the build check changes the SHA of the base-bound diff, and Verify compares that SHA first. So the foreign-record check is removed as dead. `TestNonMaterialDiff_ARecordAddedAfterTheCheckDoesNotVerify` pins that the SHA check carries it.

What it does not change: the rule of the builder stays "no material path: declare `NOT_APPLICABLE` and write no record" (the reference says so, with the test-only example). The host only stops punishing a builder that over-delivers.

The same cycle then met a second process failure at ship. The importer backstop ran 89 targets under full load. `bridge/channel.TestChannel_EndToEnd` failed, in a package that the cycle did not touch. It failed on the 10 ms sleep between its two ticks (`answer text = ""`, the assistant envelope outside the span). The test now waits for the first tick envelope in the feed.

The own rung of the backstop is F7 (§7.3). It applies when the reds of the importer backstop are all named tests and at most three. Then each red package is re-run by itself before any of them is the RED of the lane. The re-run covers the whole package, not the one test. So a same-package order dependency or shared fixture still reproduces, and only the concurrent load of the pack is removed. The argv and the outcome of each package are written to the scan log.

- Green by itself is flake evidence. Two places say it. One is a `ship.warning` `SHIP_BACKSTOP_FLAKE` in the Signal Center, where recurrence across cycles is countable. The other is a stderr line that names the tests. The ship proceeds.
- Still red is the RED of the lane, and it names what is still red.
- A package whose re-run named nothing keeps its first-run reds, because nothing was proven about them.
- A build failure is never re-run.

The scanner pack (guard suites) and the added-test backstop (red-first reproducers) keep no such rung. Their reds are the own reds of the ship.

- What the re-run cannot hide: a test that the change really broke fails by itself too.
- What it cannot prove: one green run without the load of the pack is evidence of a timing window, not proof. The line says so, and a recurring flake is the work of F7b.

The wiring proof (`TestRepoContractGate_AnImporterRedThatIsGreenAloneShipsAsFlakeEvidence`) drives the production caller with a module whose test reds once and is green when its package runs by itself. Queuing the hygiene fix from that evidence (an inbox item) is F7b, designed.

The other lane of the wave, 1719 (`docs-consistency-sweep`, a document deliverable), showed the X2 class from a new angle. Its round-1 audit failed on one HIGH: option cost figures not traced to the evidence file. This was a defect in the deliverable itself, which the repair round fixed completely by the own account of the round-2 auditor. Round 2 then failed on two more numbers:

- The Verification section of the explanation document said `17/17 GREEN`. The TDD re-run of the repair round had grown the eval to 19 checks (the auditor reproduced 19/19).
- One count in the recommendation cited an evidence entry that records no such count.

The first is staleness that the pipeline itself produced. The repair round re-runs TDD before the build, so the eval that the builder described is not the eval that the auditor ran. It lives only in the explanation document of a change that the auditor called correct and complete. Under X2 that is a documentation correction, never the FAIL of the round. The second is the broken traceability claim of the deliverable, by one number. It is the call of the auditor to make.

The recommended action for a round whose only residual defects are of the first kind is the rung of X2. A round with both kinds is a repair round, as it was.

The ship of the same cycle then met the other landing of the wave (the closeout dossier of 1718). Its fleet rebase hit a genuine conflict. Both lanes had edited `docs/operations/rescue-branch-disposition-2026-06-07.md`. The builder of 1718 edited it outside the declared files of its test-hygiene item. The partition separates declared files, not touched ones.

The host aborts such a rebase and hands the debugger a clean tree on the base that the ship diverged from. The debugger took the version of main for the file, and nothing re-ran the rebase. The cycle then took this path:

- It re-authored and re-audited (PASS) on the stale base.
- It shipped and diverged again.
- Only then it unwound, rebased cleanly and re-authored on the new base.

That was one audit round and one ship try for a resolution that was already right.

C5 (= ADR-0105 B5, §5.8) closes that. After a debugger that served a fleet-rebase recovery, the resolved tree is carried as a commit on the fork point. The commit is rebased, pended and routed as any rebased change. A conflict that survives the debugger ends the cycle loudly, in place of a reship that cannot fast-forward. The partition gap itself (a builder that edits outside the files of its item) is recorded, not fixed.

Since 2026-10-18 the build handoff floor reports each such edit as an `ORCHESTRATOR_SCOPE_DELTA` signal and does not block on it ([ADR-0087](adr/0087-scope-delta-adjudication.md), inbox `scopedelta-wire-or-delete`).

### 5.10 Waves 11–20: what the loop delivered and what the architecture let through (the deliverables review)

The operator asked for a review of the deliverables of the last ten waves before any further fix (2026-09-27). The review covered the ten loop logs from 2026-09-26 15:40 to 2026-09-27 12:47, cycles 1702 to 1720. It read all eleven shipped diffs. Four parallel read-only reviews checked each diff against its inbox item and eval. They used these rules of the owner:

- task delivered
- root cause over workaround
- single source of truth
- tests that prove intent
- no narrative comments
- functions under 50 lines
- declared scope
- docs updated

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

- **Superseded tries land in the acceptance ledger.** 1712, the third try at its item, shipped `go/acs/cycle1707` and `go/acs/cycle1708` beside its own `cycle1712`. They re-encode the same predicates for two cycles that never shipped. The explanation documents of the same tries were archived. So the two artifact kinds follow two retention rules (A2).
- **Class tickets close on instance fixes.** The item of 1706 was a flake class at its second sighting. The fix migrated the three sighted tests to the new `internal/gittest` fixture. Yet 116 test files still build raw git repositories in `t.TempDir()`, and nothing stops a new one (G3).
- **House rules exist only as prose.**
  - The lanes add a doc comment on every new function while the comment campaign removes them, because the phase personas still ask for them (G1).
  - A bugfix cycle leaves `consumeCommittedItems` of `ship/consume.go` at about 110 lines and `stage` at about 99. The 50-line limit is a test only in the packages that pin their own (G2).
  - 1712 edited `modelquery/latest.go` outside the declared files of its item. 1718 edited a doc that later conflicted with 1719. Nothing compares declared files with touched files (§5.9).
- **Test support replicates.** At least seven `go/acs/cycleN` packages re-author the same git-subprocess wrapper and environment scrub. 1715 copies two commit anchors from `cycle1529`, and 1717 parses the task-contract header twice. These are minor; the fix is a shared `acsassert` helper.

**The pipeline.** The waves' process defects were answered as they surfaced, as the table shows. The review adds five architecture-level findings. Each is a replicated or unenforced belief, the disease that the canonical retrospective named:

1. **Dispatchability was decided in five places (W3, fixed).** Five readers answered "may a lane take this id now":
   - the backlog reader (console routing only, deps ignored)
   - two carried-id prunes with different "consumed" sets
   - the launch gate (lifecycle and deps)
   - the refill (weight only)

   At wave 20 the plane inbox held 169 top-level items:

   - 162 were console-owned (67 console kinds, 37 console routes, 58 protected surfaces declared or mentioned).
   - 5 had no `id`.
   - 2 were lane material.

   Both lane items waited on a console-owned dependency (`egps-regression-tia-selection`, `chronicle-s7a-historian-shadow`). The widen planned the two blocked items, the gate refused both, and the backlog of the refill had nothing else.

   The dispatch then reported the empty launch as a wave: `wave N: 0/0 lanes ok` printed six times, and the batch ended on `max_cycles`. Wave 19 lost its second lane the same way.
2. **The acceptance ledger decays silently (A1).** 249 predicate files run tests by name through `go test -run`, which exits 0 when a name matches nothing. At review time, 26 exact test names referenced by predicates no longer existed. The tests were renamed or deleted after their predicates were written, so those checks pass vacuously. The risk was that two more names join the 26 in this landing. The repointing of the cycle-1182 predicates with the renames removed that risk.
3. **Item identity is replicated (W5).** The canonical reader, `inboxbatch.LoadFile`, gives an item without an `id` its filename stem. The reader of the lifecycle (`lifecycle.readID`) and the backlog read of the planner use the JSON field alone. The five id-less items are visible to triage, invisible to the planner, and unclaimable by the lifecycle.
4. **Any quota shrink switches off the starvation detector (W4).** `WaveObservation.Starved` excuses a wave whenever the bench shrank it. Waves 19 and 20 realized 1 and 0 of 2 sized lanes. They counted as not starved, because the bench of codex had shrunk the width from 3 to 2.
5. **A deterministic failure is retried at every dispatch (Q5).** The codex deep pin (`gpt-5.6-sol`) fails `model_unsupported` on the entitlement of the account at every deep or top dispatch. That is about eight bounces per wave, and each pays a boot before the claude fallback. The pin decision of the operator is pending, but the bench need not wait for it. The same class explains the six identical empty waves of wave 20.

**Order of fixes.** W3 lands first: it cost a whole batch, and every later wave depends on an honest plan. A1 follows, because an acceptance ledger that passes vacuously undermines every gate built on it. Then come Q5, W6 (the work-supply census, so an empty plan names why), G1, A2, W4, W5, G2 and G3. Each is one component with its own tests.

The census also matters beyond the loop. 162 of 164 pending items are console-owned, so the lane supply of the loop is structurally tiny. The operator decides if that is the intended division of work, not the loop.

### 5.11 Wave 24: an incapable last rung sealed two capacity failures as FAIL (cycles 1733, 1734)

Wave 24 went 0/2 after five consecutive ships. Both cycles failed the same way, in a phase whose every capable rung failed for a reason that was not the logic of the lane:

- claude-tmux stalled (the bug-reproduction of 1733: "agent hung: stalled for 3 consecutive busy intervals"). Or it refused the pasted prompt as untrusted text (the triage of 1734).
- codex-tmux was walled ("usage limit … try again at Oct 14th, 2026", exit 85).
- The last rung of the universal-fallback tail, ollama-tmux, refused the phase at launch (exit 10, `source-writing phase rejected`).

The runner surfaces the error of the last rung. So the structural refusal of a driver that can never run a lane phase decided the verdict. The verdict was FAIL, where the walk had met a capacity wall and the deferral of Q1 applies. Three facts made it a class:

- The tail appended every discovered family, capable or not. ADR-0104 made the tail unconditional and banned only agy by operator judgment.
- The launch guard of ollama reads `cfg.Worktree != ""` as "a writer phase", a belief that CB.1 retired (every phase runs cwd=worktree). So it refuses every lane phase, not only writers. Its real limit is that it has no tool use.
- With codex walled for two weeks, claude-tmux is the only capable rung. So any claude hiccup walks to the incapable tail.

| Id | Component | Status |
|---|---|---|
| T1 | the tail holds only drivers that can run the phase: manifest `toolless`, `bridge.HasToolUse`, `universalFallbackTail` (ADR-0104 §4) | built (this landing) |
| T2 | an exhausted walk that met a quota wall is a capacity outcome, whatever the rung order. A claude stall after the wall of codex also defers. The parts are `DispatchTiered.Walled` and `bridgechain.WallKeeper`, in the walk of the runner and of the bridge handle alike. The pause names every rung. Refined 2026-10-09: a pause needs an 85. Each other started rung is proven exhausted or neutral ([cycle 1853](../incidents/cycle-1853-secondary-defects.md)) | built (this landing) |
| T3 | the authority of the pane is stated where the CLI treats it as authoritative. See the T3 details below the table | built (this landing) |
| T4 | the plan of the advisor is clamped to the `insert_when` of the phase registry. So a refactor lane never runs bug-reproduction or fault-localization (1733 failed in one). See the T4 details below the table | built (the plan clamp); the rest of F42 built 2026-10-01 (ADR-0099 amendment) |

**T3 details.**

- `phaseidentity.Authority()` is a standing block, identical on every dispatch. It states three points:
  - The operator launched this unattended pipeline.
  - The pasted prompt is the instruction of the operator for the phase.
  - The interactive conventions of the console (confirm direction, stop to ask) are not the conventions of this pane.
- It is the system prompt of claude-tmux (`--append-system-prompt-file <workspace>/pane-authority.md`, manifest param `system_prompt_file`).
- Every CLI without that channel pastes it.
- The block of per-dispatch facts stays at the end of every paste.
- A typed line was the filed fix. The pane of 1734 refused the typed nudge of the bridge in the same way. The typed line was not applied, and the evidence of 1734 shows that it does not help.

**T4 details.**

- The plan clamp: a planned content phase whose `insert_when` has no `rubric_hint` runs only when its trigger fires at its turn (`insert-when-gates-plan`). A hint keeps the judgment of the advisor. Operator `enabled: on` still runs the phase. Floor phases are never gated (ADR-0052 amendment 2026-09-28).
- The rest of F42, built 2026-10-01 (ADR-0099 amendment):
  - The `phase.json` of the bugfix-category phases declare that they do not admit a document (`skip-when-gates-plan`).
  - A registry gate that removes a planned phase raises `ORCHESTRATOR_PLAN_PHASE_GATED`.
  - The plan prompt of the advisor carries the item of the lane and the rubric of the `skip_when` lines.
  - The scout report fallback counts tasks for the re-plan.
  - TDD on a document is decided once (released, the call of routing; the Task Contract states it; a planned phase records `plan:<phase>`).
  - Filed: `insert-when-fields-need-a-producer`.

### 5.12 F43: the audit binds the tree Ship will commit

Eight cycles (1626, 1647, 1674, 1684, 1685, 1694, 1705, 1735) failed their audit on "predicate execution tree includes undeclared inputs absent from the ship tree". The inputs were the own deliverables of the cycles: the explanation document in five, the TDD predicate and its eval in two. The audit modelled the ship tree as the index plus `add -u`, tracked files only. Ship commits something else: every path that the build and TDD reports declare, untracked or not, plus the changes that a declared directory covers (`shipmanifest.Stageable`).

The build report of 1735 named its document, and the test report of 1694 named its predicate and eval. Suppose that Ship ran on these cycles. In that case Ship commits exactly what the audit refused. The two disagree the other way too: an unnamed tracked edit passes the audit and is left out of the commit.

A first design staged the declared outputs of each phase through a new registry field. The design review rejected it as a second declared-outputs list beside the list of Ship (a duplicated belief). It moved the fix to its source: the audit asks the own selection of Ship which tree it will commit.

| Id | Component | Status |
|---|---|---|
| 1 | the refusal names its paths as `cyclestate` detail, stripped by every identity reader (F43 part 1) | built (#691) |
| 2 | `internal/shipmanifest`: Ship's pure path selection, `Stageable` the one composition, `RegularFileIn` the one file rule | built (#692) |
| 3 | `treefence.TakeStaged(ctx, worktree, pathspec)`: the tree that ship will commit, in a throwaway index (a real-index seed required; `Restore` refuses any but a full snapshot). See the component 3 details below the table | built (#693), corrected in component 4 |
| 4 | one ship tree, `shipmanifest.TakeShipTree` (`Select` → `TakeStaged` under `StageRetrying`). See the component 4 details below the table | built (this PR) |
| 5 | the report list derives from the `writes_source` phases of the registry, in place of a Go list | designed |

**Component 3 details.** Component 4 corrected the contract of component 3:

- After the declared paths it adds every tracked edit (`add -u`), because the audit binding stages them for Ship.
- `TakeTracked` (which `TakeStaged(nil)` now equals) was deleted.
- The declared add runs first. A declared file removed with plain `rm` is still in the seed, so `add -A --` stages its deletion. With `add -u` first, `add -u` drops the file from the index, and the declared add fails rc=128.

**Component 4 details.**

- Three consumers read `TakeShipTree`:
  - the audit (`predicateTreeFor` requires `Take` to equal it)
  - the binding (`worktreeContentSHA` records it as `WorktreeTreeSHA`)
  - Ship (its staged tree is pinned equal to it)
- The replays of 1735 and 1694 pass.
- An undeclared untracked input is refused by name. An undeclared tracked edit ships and is audited.
- With a workspace the selection is declared-only (no fallback to the changed set).
- The empty pathspec of Ship stages nothing.

**Component 4, as built.** The first version moved only the audit onto the selection of Ship. Its architecture review blocked it. The audit binding (`core.worktreeContentSHA`) recorded a third ship tree, `git add -u` then `write-tree`. Ship checks both the predicate receipt and its staged tree against it.

Suppose the first version shipped. In that case the eight cycles pass the audit and then fail at Ship as an integrity breach.


The review also corrected the premise that Ship left undeclared tracked edits out of the commit. The real-index `add -u` of the binding stages every tracked edit, so Ship commits them, and the audit must not refuse them.

The design review of the redesign settled four more points:

- `Select` falls back to the whole changed set in two cases: no manifest, and a manifest that covers nothing. Left as they are, those fallbacks carry untracked residue into the binding, which the cycle-1594 pins forbid. So with a workspace the selection is declared-only. Only a workspace-less manual ship stages every changed path.
- `TakeTracked` became dead and was deleted.
- `shipmanifest` imports `cyclestate` and not `core`, so `core` can call it.
- The empty-set guard lives in `StageRetrying`, so no stager can run `git add -A --` with no pathspec.

The delta review proved that the first order of the two adds (`add -u` first) broke every cycle that deletes a declared file with plain `rm`. The declared add now runs first (`TestTakeStaged_ADeclaredFileDeletedButNotStagedIsADeletion`, `TestTakeShipTree_ADeclaredFileRemovedWithPlainRmShipsItsDeletion`).

Ship runs `add -u` itself only on the worktree path (`worktreeShip.stageTrackedEdits`). This is the first staging step, where the `add -u` of the binding sits in production. So it is idempotent after the binding, and production is unchanged. The direct path serves the operator plane, where `add -u` is not wanted. If the direct path ran it, it sweeps every tracked plane edit into a manual commit. So the direct path stays without it.

The code review asked for that after the first redesign relied on the step of the binding. The hand-spliced parity test became an end-to-end one (`TestShipFromWorktree_CommitsTheOneShipTree`).

The predicate of cycle 1067 for the old fallback moved to the new contract (the #695 precedent). A read-only replay of the new comparison on the live lanes 1737 and 1738 found the full tree equal to the ship tree on both.

### 5.13 The size ratchet is a ceiling (wave 25, cycle 1735)

The diff of cycle 1735 shrank three functions listed in `go/internal/sizeratchet/offenders.json` by one line each. It did not lower their allowances. The ratchet required an allowance to be lowered exactly whenever its function shrank. The audits of the lane did not run the module-wide check. After a fleet rebase, the peer predicate of cycle 1736 (`TestC1736_003`, the module-wide `Check`) did run it.

The re-audit went red, the repair budget was already spent, and the cycle failed. The failure was process, not logic: the own message of the ratchet named the exact edit.

A first design had the host lower shrunk allowances at the build handoff. The design review rejected it, for three reasons:

- The build floor is a check, not a writer.
- `offenders.json` is material to the explanation contract, so a host write forces a Build re-author.
- Every lane that writes the file breaks the byte-identical rebase proof and conflicts with a peer on adjacent lines. This is the defect already filed as `sizeratchet-allowances-not-a-shared-registry`.

The fix moves the rule instead: an allowance is a ceiling. Growth past it still fails. A new function past 50 lines still fails. A shrunk, healed or deleted offender entry is slack that passes.

| Id | Component | Status |
|---|---|---|
| R1 | `sizeratchet.Check` treats an allowance as a ceiling; the cycle-1726 predicates and the evals follow; the pending shrink items leave `offenders.json` alone | built (this landing) |
| R2 | pure `Tighten` and a byte-stable `Encode` (`json.MarshalIndent(m, "", "  ") + "\n"` reproduces today's file; `TestEncode_ReproducesTheCheckedInListByteForByte`) | built (2026-10-18, `sizeratchet-allowances-not-a-shared-registry`) |
| R3 | `evolve ratchet tighten [--check]` (the design named it `evolve sizeratchet tighten`; the verb joined the `ratchet` family of `evolve ratchet check`), run at the wave boundary: the one writer of `offenders.json` | built, not wired (2026-10-18, `sizeratchet-allowances-not-a-shared-registry`). The verb exists and refuses to write inside a phase or under a live lease. The call from `evolve boundary run` is a follow-up. The boundary has no landing path for a main-tree write |
| R4 | a build-floor no-growth check against the base (the `commentaudit.ReadAtBase` pattern), so a function cannot regrow into its slack before a tighten. Built as `core.sizeFloorFailures` over `sizeratchet.CheckAgainstBase`, which runs `Check` against `Tighten(base spans, base list)` | built (2026-10-18, `sizeratchet-allowances-not-a-shared-registry`) |
| R5 | one exported offenders-path constant (written three ways today) and one 50-line cap (`test/structure/limits.go` duplicates it with `>=`). Built: `sizeratchet.OffendersRelPath` in every message, and `CheckLimits` reads `sizeratchet.MaxLines` with `>` | built (2026-10-18, `sizeratchet-allowances-not-a-shared-registry`) |

### 5.14 A document-only audit FAIL re-authors at Build, not TDD (wave 30, cycle 1745; X2a)

The round-1 auditor of cycle 1745 found the code correct. The results were 14 of 14 predicates, 50 of 50 and 30 of 30 mutants killed, and the ACS suite green. It failed the round only for one reason. The explanation document made two false claims (a line count, and why a probe sorts packages). That is the stated policy of the persona until the rung of X2 exists.

Both defects began with `docs/explain/builds/cycle-1745-01m3mak46kk0havqvsbxmhaq4q.md:<line>`, and the prescription said that no code change was needed. The disposition read only the declared class, `code-audit-fail`. The policy granted a retry and, with no adjudicator wired, took the first action of the envelope, `retry@tdd`. The round re-ran TDD, then the full build, then the audit. The TDD claude-tmux pane wedged on the way (exit 81, `submit_wedged`) before it fell back.

The brief also reached the builder without the defects of the audit. An agent-graded FAIL writes no floor reasons, so `readContinuationFindings` logged "findings artifact … audit-fail-reason.json unreadable".

X2a keeps the judgment and makes its repair the cheapest rung that can satisfy it. It is not X2: the auditor still fails the round, and a re-audit of the corrected document re-earns the verdict (principle 3). Nor is it the non-goal in §2. Nothing recovers the FAIL. The owner is assigned the document back, as Build already is after a rebase that the host cannot prove identical (ADR-0105, `routeRebasedExplanation`).

- **The class comes from the defects.** `explanationCorrectionDocument` (`core/explanation_correction.go`) locates each defect of the failure block by its first path-like token. A path-like token has a slash, or a stem of two or more characters with an extension that starts with a letter. So `e.g.` and `2.5` are prose and `probe.c` is a file. The function trims quotes, brackets and a `:line` or `#L` suffix first. So `H1:`, `H1 HIGH …:` and `Rationale (<doc>:30)` all resolve to the path.
  - The FAIL is `explanation-needs-correction` only when two conditions hold. Every location is the own document of the cycle (`explanationdocs.DocumentPath(cycle, runID)`, or that path under a worktree). And `CycleState.AuditFailReasons` is empty. A runner gate that diagnosed the FAIL is a defect outside the document.
  - Everything else keeps today's routing:
    - a code path
    - a bare report file before the document
    - a defect with no path
    - the document of another cycle or run
- **The route narrows a granted envelope.** `explanationCorrectionEnvelope` runs after `computeRetryEnvelope`, which is unchanged.
  - It narrows a grant (one whose legal set holds `retry@build`) to `retry@explanation` or `decline`. It passes a decline or a halt through untouched.
  - `retry@explanation` re-enters Build (`reentryPhase`).
  - The round spends the `code-audit-fail` budget like any repair, so at the cap the cycle declines to retro as before.
  - An adjudicator `retry@tdd` is clamped.
  - `ORCHESTRATOR_AUDIT_REPAIR_GRANTED` reads "repair round via build" with the class in its reason.
- **The Build dispatch is scoped.** `seedAuditRepairContext` adds `CtxKeyExplanationReauthor` (the document path) to the Build dispatch of that round only. It does so on the live and resume paths alike, from the same persisted inputs that the route read.
  - The build prompt opens with an "Explanation Re-author" section ahead of the Audit Repair findings.
  - The section says three things: edit only the named document; rewrite `build-report.md` with its `## Explanation Documentation`; leave code and tests unchanged.
  - The report rewrite is required because the bridge completes a Build only on the rewrite of its primary artifact (§12, "Completion after a correction").
- **The defects are read from the failure block.** The first landing recorded them. `recordExplanationCorrection` copied them into `audit-fail-reason.json`, each prefixed with the class, so that the repair brief and a continuation read them there. Since X2a.5 the brief reads the failure block itself for every agent-graded FAIL. The copy was a second record of the defect list, and it is retired: `retry@explanation` writes nothing.
  - The one reader is `auditRejectionReasons` (`core/repair_brief.go`).
  - When a runner gate diagnosed the FAIL, it returns the reasons of the runner, with today's rendering and precedence. It reads them from `CycleState.AuditFailReasons`. `runnerDiagnosedAudit` is the one shared predicate of the X2a classifier, this slot and the findings header. This is the carrier that the coherence floor also asks. It never reads `audit-fail-reason.json`, the forensic twin of the carrier.

    That file is written best-effort. `resetFloorFailReason` deliberately keeps the record of another phase across an audit dispatch. The risk was that a leftover record hides the failure block.
  - Otherwise it returns `phasecontract.ReadFailureBlock(workspace, audit).Defects`, less any defect that restates a finding that the brief lists. The comparison uses the title, the id and title, or the id, severity and title, on letters and digits. The brief lists the first eight actionable findings (`briefedFindings`).
  - The brief no longer reads the file, so nothing there WARNs.
  - The findings header says "the gate reasons above are their symptoms" only when the reasons of the runner lead. Above the defects of the failure block, and in the standing-findings brief, it reads "fix THESE".
  - The brief of the re-author carries each defect as the auditor wrote it, and the Explanation Re-author section scopes the round to the document.
  - The prompt of the retry adjudicator reads through the same function (the adjudicator is not wired in production).
- **A phase that dies inside a repair round is digested from its own failure, unless a runner gate diagnosed the FAIL.** `ensureFailureDigest` writes its fallback (the dying phase and its error) only when `audit-fail-reason.json` is absent. An agent-graded FAIL writes no record. So a TDD or Build that dies in its repair round is fingerprinted as that phase. This now includes the X2a re-author, whose Build the retired record had digested as the document defects of the audit.
  - A runner-downgraded FAIL keeps its gate record through the round (the next audit dispatch retires it). So a death inside that round is digested from the gate reasons.
  - That precedence is kept on purpose. The record is the untruncated, deterministic diagnosis of the runner. The dossier, the gate-reasons panel of the dashboard and audit calibration read it by name. The risk of an overwrite with the error of the dying phase is that it erases that evidence.
  - The risk of a digest assembled from anything but the file is that it disagrees with the reason file that the dossier shows beside it.
  - The round exists to repair those reasons, so the recurrence ledger counts the death against them.
  - The error of the dying phase is not lost. The phase-error path hands it to the retro as `failure_error`, and an abnormal exit seals it as the termination reason of the cycle.

Limits. The classifier reads the text of the auditor.

- A defect that names the document without its path ("explanation doc :97 …") keeps the full round. So the auditor reference now asks for the path-first form and says what the host does with it.
- A defect that begins with the document, but whose real remedy is in code, gets a document-only round. The re-audit judges the result. So the cost is one Build dispatch and one retry try from the `code-audit-fail` budget. At a cap of two, this is possibly the round that can fix the code. The cost is never a wrong ship.
- The defence is a rule of the auditor reference, pinned in its persona test. Each code defect is its own entry, and it begins with its own path.

| Id | Component | Status |
|---|---|---|
| X2a.1 | `explanationCorrectionDocument`: every defect located in the cycle's own document, no runner-gate diagnostic | built (this landing) |
| X2a.2 | `recordExplanationCorrection`: the defects in `audit-fail-reason.json`, retired by the next audit dispatch | built; removed 2026-09-29 by X2a.5 (it was a second copy of the failure block's defects) |
| X2a.3 | the build prompt's Explanation Re-author section (`CtxKeyExplanationReauthor`), with the report rewrite that completes the dispatch | built (this landing) |
| X2a.4 | `retry@explanation`: the narrowed envelope, the Build re-entry, the record and the seed wired on both dispatch roots | built (this landing) |
| X2a.5 | `auditRejectionReasons`: the repair brief and the adjudicator read the reasons of the runner (`CycleState.AuditFailReasons`). If the runner gave none, they read the defects of the failure block. This holds for every agent-graded FAIL. There is no "unreadable" WARN. The X2a.2 record is removed, so a re-author that dies is digested from its own failure | built (2026-09-29) |

### 5.15 A document correction of a verified build is a rung (X2)

The request is the inbox item `explanation-correction-is-a-rung`: an inaccurate explanation document of verified code cost a full audit-repair round in cycles 1712, 1719 and 1731. X2a (§5.14) made that round cheaper, but the audit still failed and the round still spent the `code-audit-fail` budget. X2 removes the FAIL.

**F15.1** The gate already records `NEEDS_CORRECTION` as an advisory (`phases/audit/explanation_review_gate.go:35`). Only the auditor rule turned it into the FAIL ("make it FAIL, because the audit-repair round is the path that corrects the document").
*Implication:* the host needs a path that corrects the document after a PASS. Then the rule can keep the PASS.

**F15.2** Build can already correct only the document. X2a's re-author dispatch edits the named document and rewrites `build-report.md`. The code and the tests stay unchanged (`phases/build/build.go`, `CtxKeyExplanationReauthor`).
*Implication:* the rung reuses that dispatch. It does not need a new agent.

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D15.1 | A PASS or WARN audit can review the document as `NEEDS_CORRECTION`. Then the host schedules Build with the review as its brief (`decideExplanationCorrection`). The audit runs again on the corrected document, and the cycle ships. | F15.1, F15.2 | A host rewrite of each claim: the review is prose, and only counts and paths are mechanical. That rung is a later step. A review-only dispatch: no phase does a document-only review today. A new phase is more work than the item asks for. |
| D15.2 | The budget is the failure-policy category `explanation-correction` (`max_retries`, default 1). It does not spend `code-audit-fail`. | principle 5 (bounded, from config) | A new policy key: a category already has a bound, a level and an override path (`FailurePolicyConfig`). |
| D15.3 | The budget can be spent while the review is still `NEEDS_CORRECTION`. Then the verified code ships, with a WARN `ORCHESTRATOR_EXPLANATION_CORRECTION_EXHAUSTED`. | goal 1 (a cycle whose logic is right ships) | A FAIL at exhaustion: that brings back the FAIL that X2 removes, for a defect of form. |
| D15.4 | A FAIL verdict never takes the rung. It keeps the X2a and repair routes. | the acceptance ("a review that names the code still FAILs") | A search of the review's paths for a code defect: the Evidence must cite every material path. Thus the paths do not show which side is wrong. The verdict does. |

How it works:

- **The trigger.** `pendingExplanationCorrection` reads the audit report of the workspace with `reportdoc.ReasonedReview`, the reader the gate uses. It returns the cycle's `explanationdocs.DocumentPath` and the review's Evidence.
- **The two dispatch roots.** `recordAndBranch` and the resume loop call `decideExplanationCorrection` after the audit FAIL decision.
- **The state.** `CycleState.ExplanationCorrections` holds the Evidence of each round, and its length is the budget used. `ExplanationCorrectionActive` seeds the next Build, and the Build that completes clears it.
- **The brief.** `seedAuditRepairContext` gives that Build `CtxKeyExplanationReauthor` and `CtxKeyExplanationCorrection`. The second key lists the document lines the Evidence cites (`explanationCorrectionBrief`) and quotes the Evidence. The build prompt opens an "Explanation Correction" section: edit only those lines, rewrite the report, leave the code and the tests unchanged. The section does not say that the audit rejected the build.
- **The record.** Each round emits INFO `ORCHESTRATOR_EXPLANATION_CORRECTION_RUNG` with `fields.round` and `fields.budget`. The cycle dossier carries `explanation_corrections` (`schemas/cycle-dossier.schema.json`).
- **The rule.** When only the document is wrong, the rendered audit rule keeps the PASS with `NEEDS_CORRECTION`. The auditor FAILs only for a defect in the code or the tests.

Limits. The re-review is a full audit dispatch at the audit's tier, because no smaller review exists. The rung saves the TDD dispatch, the FAIL and the repair budget, not the audit cost. The host rewrite of a mechanical claim (a count, a path) is not built.

## 6. Decision tables

### 6.1 Routing by violation code (`deliverable`, beside the codes)

| Code | Route |
|---|---|
| `missing_effect` | host step (as today) |
| a derivable owed file: `missing_secondary`, `empty_secondary`, `malformed_secondary` of `triage-decision.json` | host derivation; a decline routes to re-dispatch, never to the agent (a decision document is the phase's judgment) |
| `stray_in_worktree` | salvage |
| `missing_section`; `empty_secondary` / `malformed_secondary` of a non-decision file | evidence enough: recovery agent; evidence not enough: assign back naming `Missing` |
| `missing_artifact`, `empty_artifact` | assign back: the owner is re-dispatched with `Present` as the correction; the diff and every green run are kept |
| a floor rejection without a code (explanation floor, build floor) | once floors carry stable codes (F0), routed with the whole set; until then, re-dispatch as today |
| `bad_verdict` | re-dispatch (the gate's deterministic verdict salvage already ran) |
| `missing_challenge_token` | re-dispatch (proof of read is never supplied by a helper) |
| `failure_context_missing`, `failure_class_unknown` | re-dispatch (the class is a judgment that drives retries) |
| `unbound_effect` | pipeline defect (a registry declaration nothing binds) |

### 6.2 Evidence per kind (`evidence.Sufficient`, E1)

| Kind (registry `recovery.evidence`) | `Present` when | `Missing` names |
|---|---|---|
| **change, code kind** (build) | the own change of the phase is non-empty (the tree at this dispatch differs from the tree now). The build floor approved this tree hash and actually ran. The predicates of this cycle under `acs/cycleN` exist with a complete inventory, none red and none skipped, on the current tracked tree. Their bytes equal the TDD-end snapshot. The explanation document is present whenever material paths exist. The protected-surface floor is clean | each absent item, by name (the red or skipped predicate, the weakened file, the missing explanation) |
| **change, document kind** (ADR-0099) | `solutioncheck` reports nothing for every committed id; the diff stays inside the solution root and the explanation document | the failing check; a path outside the root |
| **tests** (tdd) | the diff against the base touches only test files and `acs/cycleN`. The own run of the host of those predicates on the base tree fails each on an assertion (the capture of E0) | a predicate that passes on the base, or fails to compile |
| **document** (scout, triage, build-planner, any phase so declared) | the decision-bearing sections parse; no tests run | the section that does not parse |
| judgment and control phases | never evaluated | — |

Facts that are reported but never conditions:

- Paths outside the triage footprint. The footprint is agent-declared, and every honest build touches tests and the explanation document beyond it.
- If the predicates cover the acceptance (the judgment of the auditor).

### 6.3 Which phases qualify

`phase-registry.json` declares per phase `recovery: {evidence: change|tests|document, decision_sections: [...]}`. A phase without the block is never evaluated. `remediationDenied` (audit, retrospective, debugger, adversarial-review, premise-challenge, plan-review) stays as the floor beneath that.

## 7. Components

Status: **shipped** (commit on a branch, PR open or merged) · **built** (green in a worktree, not shipped) · **designed** (in this document and the ADR only). Every component is one commit with its own tests, mutation-checked.

### 7.1 Policy and process (P)

| Id | Component | Status | Where |
|---|---|---|---|
| D0 | operating-policy §0 + ADR-0106 + this document | merged, #655 | `docs/` |
| P1 | ADR-0105 B1 unwind-rebase-pend; then the resume heal, B3, B4 | B2 merged (#649); B1 merged (#652); B3 and B4 shipped (§5.8 C1–C4); the resume heal designed | `core/ship_recovery*.go`, `core/identity_carry*.go`, `phases/ship/carry.go`, `internal/treedelta` |
| P2 | the closeout dossier of a fleet lane waits for the wave boundary. The publish refuses three cases: while another run is live, a differing corpus record, and a non-regular pending file | built, pending the review of the operator. It was salvaged from `fix/dossier-commits-at-wave-boundary` (architect N1–N5 applied) onto current main as local commits on `feat/dossier-pending-at-boundary`. The FIX_THEN_MERGE findings of the review followed as six more commits, and those of the re-review as six more | `dossier/publish_pending.go`, `core/cycle_closeout.go` (`dossierDestination`), `cmd_loop_dossiers.go` |
| P3 | the pasted prompt ends with a statement of who the agent is (phase, cycle, session, prompt files, sole writer). Phase panes export the `default_env` of the manifest, and claude-tmux turns prompt suggestions off | shipped, PR pending (`feat/phase-identity`, four commits) | `bridge/phaseidentity`, `driver_tmux_prepare.go`, `driver_tmux_boot.go`, `manifests/claude-tmux.json` |
| P2 | a fleet lane's closeout dossier waits for the wave boundary | built, parked (`fix/dossier-commits-at-wave-boundary`; architect N1–N5 applied) | `dossier/publish_pending.go`, `cmd_loop_dossiers.go` |
| P3 | the pasted prompt ends with a statement of who the agent is (phase, cycle, session, prompt files, sole writer). Phase panes export the `default_env` of the manifest, and claude-tmux turns prompt suggestions off | merged, #661 (`abc9ced0`) | `bridge/phaseidentity`, `driver_tmux_prepare.go`, `driver_tmux_boot.go`, `manifests/claude-tmux.json` |
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
| F7 | when the reds of the importer backstop are all named tests and at most three, each red package is re-run by itself. See the F7 details below the table | shipped | `phases/ship/repocontract.go` (`clearedAlone`, `runRepoContractPackagesAlone`, `packFailure`) |
| F7b | the flake evidence queues the hygiene fix. The fix is an inbox item that names the test and the scan log. So the lesson becomes work, not a WARN line | designed | `phases/ship`, `internal/inboxmover` |

**F7 details (§5.9; `TestChannel_EndToEnd` of 1718).**

- The re-run covers the whole package. So same-package state stays in play, and only the concurrent load of the pack is removed.
- The scan log holds the argv and the outcome of each package.
- Green ⇒ `SHIP_BACKSTOP_FLAKE` (a `ship.warning` in the Signal Center and a loud stderr line that names the tests). One re-run without the load of the pack is evidence, not proof. The ship proceeds.
- Still red ⇒ the RED of the lane, naming what is still red.
- A package whose re-run named nothing keeps its first-run reds.
- A build failure is never re-run.
- The scanner pack and the added-test backstop keep no such rung.

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
| Q1 | exhaustion during a correction re-dispatch, a remediation re-run and the resume review gate defers through `pauseForQuota`. See the Q1 details below the table | shipped on the P3 train | `core/cyclerun_correction.go`, `core/cyclerun_remediate.go`, `core/resume.go` |
| Q2 | `auth_recheck` (`clihealth.CredentialPattern`) benches the family. See the Q2 details below the table | shipped on the P3 train | `internal/clihealth`, `internal/bridgechain`, `cmd/evolve/cli_health_canary.go` |
| Q3 | a deferral writes no failure digest and counts toward no halt. The wave breaker reads only digests, and a deferral writes none (`TestRunCycle_WalledCorrectionRedispatch_DefersLikeAFirstDispatch`, from Q1). The sequential loop now pauses on a deferral before it records a failed approach or runs the failure closeout. So a capacity wall never demotes the item or counts as a recoverable failure (`TestRunLoop_AFreshCycleWalledOnCapacityIsNobodysFailedApproach`) | shipped | `core/blocker_breaker.go`, `cmd/evolve/cmd_loop_sequential_dispatch.go` |
| Q4 | the fallback chain filters candidates by the phase's requirements | designed (§5.5) | `internal/bridgechain` |
| Q5 | a `model_unsupported` exit benches the family's model at that tier until the pin changes. Like Q2's credential bench, it saves a boot at every deep or top dispatch | shipped (2026-10-21): `BenchOnEscalation` benches the scope `<family>@<model group of the tier>` for `MaxCooldown`, with an operator action. The bench stops binding when the tier resolves to another model. An `unknown_prompt` exit 85 is no quota wall (`llmroute.TieringExit`) | `internal/clihealth`, `internal/bridgechain`, `internal/llmroute`, `internal/phases/runner` |

**Q1 details.**

- `isQuotaWall`: the exit 85 of the runner means that its whole family chain was walled. One sample suffices, unlike the two-sample rule of the first dispatch, which predates the tiered chain.
- A non-wall failure stays a failure.
- The pause records the total dispatches of the phase.

**Q2 details.**

- Unlike a quota bench, the bench of `auth_recheck` stays active for routing after its cooldown, because time says nothing about a login. The canary probes it.
- A succeeding probe after the login of the operator clears it.
- The bench entry carries `operator_action` (`clihealth.OperatorAction`). The bench line of the chain walker and the re-bench line of the canary print it.

### 7.6 Ledger of outcomes (L)

| Id | Component | Status | Where |
|---|---|---|---|
| L1 | recovered paths and the evidence verdict in `CycleResult.Remediations`, the dossier, the audit prompt, and an in-file marker | designed | `core/`, `dossier/`, the audit prompt |
| L2 | recovery-rung exhaustion recorded as a system-level pipeline defect; breakers unchanged; an exhausted assign-back stays the task's logic FAIL | designed | `core/blocker_breaker.go`, the retro paths |

### 7.7 Lane lifecycle (W) and the explanation document (X)

| Id | Component | Status | Where |
|---|---|---|---|
| W1 | a shipped or declined id that no inbox item backs is retired by record. See the W1 details below the table | shipped | `internal/inboxmover`, `internal/inboxmover/lifecycle`, `internal/cycleoutcome`, `internal/phases/ship` |
| W2 | a failing authored id accrues a durable failure count toward the S5 ceiling | designed (§5.6) | `internal/inboxmover` |
| W3 | one dispatchability rule (`inboxmover.ResolveDispatchability`, with `PendingDispatchability` for a known-pending item). See the W3 details below the table | shipped | `internal/inboxmover` (`dispatchability.go`), `internal/triagecap` (`PruneUndispatchable`, `ReadInboxBacklog`, `RankForDispatch`), `internal/loopwave` (`dispatch`, `probe`, `refill`, `pruneUndispatchable`) |
| W4 | the starvation observation compares realized lanes with the sized width of the wave. So a quota shrink explains only the lanes that it removed. It supersedes the rule of `go/acs/cycle544` that any shrink disables the detector | designed (§5.10) | `internal/fleet/starvation.go`, `cmd/evolve/cmd_loop_window.go`, `go/acs/cycle544` |
| W5 | one item identity. See the W5 details below the table | designed (§5.10) | `internal/inboxbatch`, `internal/inboxmover/lifecycle`, `internal/triagecap` |
| W7 | the claim floor refuses an item whose declared dependency has not landed. So a triage pick that names one anyway fails loudly, and does not build ahead of its dependency. W3 stops offering it, but nothing yet enforces it at the claim | designed (§5.10, second architecture review) | `internal/inboxmover` (the claim floor) |
| W8 | the dependency half of the batch classifier (the deps loop of `inboxbatch.depRule`, the dependency ordering of `topoOrder`) is removed with its tests. Under W3 a dependent never shares a menu with its unlanded dependency, so the grouping is unreachable (links and campaigns stay) | shipped (cycle 1724, `7889cc5b`; §5.10, second architecture review) | `internal/inboxbatch`, `go/acs/cycle1205` |
| W6 | a work-supply census. The signal of an empty or short plan names the pending items by why they are not lane material. The reasons are: console kind, console route, protected surface, unmet dependency with its blocker, no id | designed (§5.10) | `internal/triagecap`, `internal/loopwave` |
| X1 | `.evolve/inbox/` is non-material to the explanation document: the host claims, moves, stamps and retires those records | shipped | `internal/explanationdocs` |
| X2 | A review that names only the document of a verified build is a rung, not the audit's FAIL. Build corrects the named document lines, and the audit re-reviews them before ship. | built (§5.15) | `core/explanation_correction_rung.go`, `core/cyclerun_record.go`, `core/resume_execution.go`, `phases/build/build.go`, `phases/audit/audit_rules.go`, `policy`, `dossier` |
| X2a | an audit FAIL whose every defect locates in the own explanation document of the cycle (and no runner gate diagnosed it) is `explanation-needs-correction`. See the X2a details below the table | shipped (§5.14) | `core/explanation_correction.go`, `core/audit_fail_decision.go`, `core/repair_eligibility.go`, `core/repair_brief.go`, `phases/build/build.go` |
| X3 | the own explanation record of the cycle is never history: only foreign cycle records are immutable, in every branch. A no-material diff declared `REQUIRED` is verified with an empty material set. One declared `NOT_APPLICABLE` keeps an undeclared own record as documentation. The Verify mirror branches on the recorded status | shipped (§5.9) | `internal/explanationdocs` (`checkBuildV1`, `verifyResolvedV1`) |

**W1 details.**

- `lifecycle.(*Mover).RetireUnbacked` writes the record. `inboxmover.RetireUnbacked` is the one door (unknown dispatch state only).
- The PASS seam retires what `Promote` was unable to move (`OutcomeResult.RetiredUnbacked`, processed).
- The no-work closeout retires every such scoped id (`NoWorkResult.Retired`, rejected, the answer of the lane as the reason).
- The prune of the planner and the freshness probe read the record.

**W3 details.**

- Three consumers ask the rule, from one root (`EvolveDir`; Q-W7 and Q-W8 resolved):
  - the carried-id prunes
  - the backlog that the seed, the widen and the refill read
  - the launch gate

  So the plan never offers a lane that the gate refuses.
- Four readers read one per-item judgment: the triage menu, `evolve inbox batches`, the backlog of the planner, and the no-work check (lane-scoped and sequential). The judgment is `PlaceOnLaneMenu`, folded by `PartitionLaneMenu`, injected into core as `WithLaneMenu`.
  - So no menu offers a waiting item.
  - An empty commitment over withheld work is honest no-work.
  - The own claim of a cycle still counts as the work that it took.
  - The no-work check fails closed only on an unreadable inbox file, never on a notice about a readable one (`inboxbatch.ScanDir`).
- The refill ranks with the shared `triagecap.RankForDispatch`. [ADR-0121](adr/0121-inbox-priority-is-a-computed-rank.md) (P2) superseded this on 2026-10-06: the backlog arrives in `inboxrank.Order`, and `RankForDispatch` is deleted.
- `Ran` means that a lane launched. So a launch that the gate empties takes the min-width repair and empty-backlog path, and does not count as a wave. That path now counts toward work-supply starvation (§5.10: waves 19 and 20).

**W5 details.** Every inbox reader resolves every item as `inboxbatch.LoadFile` does (its id fallback and its sanitizer), or the host stamps the id at intake. So no item is visible to triage but invisible to the planner and unclaimable by the lifecycle.

- W5 is reachable since W3. The empty-backlog path of W3 runs a sequential cycle. The no-work check of that cycle counts an id-less lane item as work that the lifecycle cannot claim. So until W5 lands, the inbox must hold no id-less item.
- The guard covers the inbox as it stands at launch. The 2026-09-27 inbox curation stamps ids on the five items that console sessions filed by hand. The boundary refuses to launch while `grep -L '"id"' .evolve/inbox/*.json` lists a file. An id-less item filed mid-wave ends its cycle as a loud claim failure, not a silent one.
- The same identity gap has a quiet twin on the lane path. The sanitizer of the loader rewrites a scoped id (a control character, or over 160 characters). Then the id no longer matches the id of the menu, so an empty commitment over it ends as planned no-work. No tracked id is affected today (all 121 are 55 characters or fewer). W5 closes both.

**X2a details.**

- The granted envelope narrows to `retry@explanation`.
- Build re-authors the document with TDD skipped.
- The brief carries the defects from the failure block (X2a.5, which also serves every other agent-graded FAIL). The copy of the first landing in `audit-fail-reason.json` is removed.
- The audit re-runs.
- The FAIL itself stays until X2.

### 7.8 Routing at triage (R)

| Id | Component | Status | Where |
|---|---|---|---|
| R1 | a top_n card that names a protected surface is moved by the host into `escalate_block` with the console-route reason. The hook returns PASS with a warning per card. The planned no-work closeout routes the item. A route that cannot be recorded fails closed | shipped | `phases/triage/protected_route.go`, `phases/triage/triage.go` |
| R1b | the route empties top_n, and the cycle is bound to one unanswered, undeferred item (`committedset.Committed`). Then that item is escalated with the reason of the first card, which names the card. So the lane is planned no-work, and the item is routed. R1c closes the multi-item residual | shipped | `phases/triage/protected_route.go` (`withBoundItem`), `phases/triage/triage.go` (`boundItems`) |
| R1c | the claim-failed and planned no-work endings are recorded at C1 (`RecordEnding`). The claim failure of a lane is the task-level refusal `TRIAGE_SCOPE_UNANSWERED`, and its closeout charges `committedset.Unanswered`. The no-work end is `NO_WORK`. A zero-try record keeps the usage sidecar of the phase | shipped | `core/cycle_closeout.go`, `core/outcome/recorder.go`, `cyclestate/result.go`, `committedset/committedset.go`, `cycleoutcome/cycleoutcome.go`, `cyclehealth/outcome.go`, `core/triage_termination.go` |
| R2 | the triage prompt lists the protected surfaces and the drop reason; the persona carries the rule | shipped | `phases/triage/triage.go` (`inboxBatchesSection`), `agents/evolve-triage.md` |

### 7.9 The acceptance ledger (A) and the rules on a lane's output (G)

| Id | Component | Status | Where |
|---|---|---|---|
| A1 | A predicate that runs tests by name fails when a name matches no test. A rename or a delete cannot turn it vacuous. The stale names found at review are repointed or retired. | shipped (2026-10-18): `acsassert.GoTests` refuses an exact pattern name that `Names` omits. `go/acs/regression/acsnames` fails on a predicate name that no test declares. 20 stale names are repointed or retired ([the ledger](acs-predicate-quality-gate.md#predicate-test-names-resolve-acsnames-2026-10-18)). | `pkg/acsassert` (`go_tests.go`), `go/acs/regression/acsnames`, `go/acs` |
| A2 | a superseded attempt's artifacts follow one retention rule: its predicate package is archived with its explanation document, never landed beside the shipping cycle's | shipped (2026-09-30): continuation adoption archives the ancestor's own `go/acs/cycle<M>` beside its explanation records. Ship backstop (2026-10-21): a cycle landing that adds a file under the package of another cycle stops with `FOREIGN_PREDICATE_PACKAGE` | `explanationdocs/predicate_archive.go` (`ArchiveSupersededPredicatePackages`), `acssuite/ancestors.go` (`AncestorCyclePackages`), `core/continuation_ancestor_predicates.go` (`prepareAdoptedTree`), `phases/ship/foreign_predicate_guard.go` (`refuseForeignPredicatePackages`) |
| G1 | the phase personas carry the no-comments rule, so lanes stop adding what the comment campaign removes | designed (§5.10) | `agents/`, the engineering-craft skill |
| G2 | the function-size limit is one repo-wide ratchet instead of a test per package | shipped (cycle 1726, `5cbc3e56`); since 2026-09-28 an allowance is a ceiling (§5.13) | a lint step |
| G3 | a class ticket closes on an enforced invariant: a ratchet refuses a new raw git fixture outside `internal/gittest` | shipped (cycle 1729, `2f7c8683`; §5.10) | a repo-contract test |

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
// RankForDispatch (weight, then a declared surface, then input order) was deleted by ADR-0121 P2 (2026-10-06)
func ReadInboxBacklog(evolveDir string, isProtected func(string) bool) []FleetCandidate // PlaceOnLaneMenu's ready items that carry an id, in inboxrank.Order since ADR-0121 P2
// W3 — internal/core (shipped)
type LaneMenuFn func(inboxRoot string, items []inboxbatch.Item) []inboxbatch.Item
func WithLaneMenu(fn LaneMenuFn) Option // the composition root injects PartitionLaneMenu's ready set; claims keep the routing-only rule
func (o *Orchestrator) LaneMenuWired() bool
// W3 — internal/loopwave (shipped): dispatch returns Outcome{} (Ran=false) when the launcher launched no lane
// R1 — internal/phases/triage (shipped)
type protectedCard struct{ ID, Path, Via string } // Via: the routed card an added bound item was answered for by
func protectedTopNCards(body string, forbidden func(string) bool) []protectedCard // every card, report order; nil forbidden = manifest membership
func routeProtectedCards(decisionPath string, cards []protectedCard, bound []string) error // top_n -= cards; escalate_block += {task_id, "protected-surface: <path> — …"}; + the bound item when top_n emptied (R1b); other keys kept; idempotent
func withBoundItem(cards []protectedCard, bound []string, decision []byte) []protectedCard   // R1b: + {bound[0], cards[0].Path, cards[0].ID} iff one bound id, unanswered, not a card
func routedCardDiagnostics(cards []protectedCard) []core.Diagnostic                // one warning per card, code TRIAGE_PROTECTED_SURFACE, subject the card
// R1c — internal/committedset (shipped)
func Unanswered(workspace string) []string // the lane pin minus the ids the decision answered (DispositionsFrom); a deferral is owed; no pin owes nothing
// R1c — internal/core, internal/core/outcome, internal/cyclehealth, internal/cyclestate (shipped)
func (cr *cycleRun) recordTriageEnding(verdict, reason string, diags []Diagnostic) // host ending → C1 via RecordEnding
func (r *Recorder) RecordEnding(timings *[]phasetiming.Entry, out recovery.PhaseOutcome) // the C1 entry only: no PhasesRun, no sidecar, no emission
const DiagCodeTriageScopeUnanswered = "TRIAGE_SCOPE_UNANSWERED" // RefusalDisposition: task-level, no console route
const CycleTerminationTriageClaimFailed = "triage-empty-commitment-claimable-work" // core aliases it
const OutcomeNoWork Outcome = "NO_WORK"
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
| It forges proof of read, the lane pin, a verdict artifact, test logs | F2 fences the whole workspace; only `Allowed` paths are allowed to differ; everything else is restored and reported |
| It plants a file under a telemetry name | `Unfenced` matches exact paths or below a directory; generated-name artifacts are tolerated by stem, direct children only, and reported in `Outcome.Unfenced` |
| It swaps a fenced or allowed path for a link or a directory | undone at `End`; a link at an allowed path at `Begin` refuses the guard |
| It invents content | F2b provenance: every added block maps to a byte range of a pre-rung snapshot or a host log. A block without a source fails the rung |
| It carries a verdict it did not earn | V0 re-classifies; a verdict field must be byte-identical; a missing primary is never authored |
| Extra re-reviews demote the gate's breaker | rung checks are breaker-neutral; one review, only after they pass |
| A judgment is repaired | registry allowlist + `remediationDenied` |
| It reaches a sibling lane's run dir | the shared `.evolve/runs` grant is every phase's today; F3b narrows it to `{cycle}` |
| It exfiltrates what it reads | the profile declares no network. The wrapper forces network on today for every dispatch (filed: `sandbox-wrapper-forces-network-on`). The tmux drivers enforce no tool list (filed: `tmux-drivers-ignore-profile-tool-lists`). The filesystem grant is the boundary that holds |
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
| a correction that touched only the explanation document left the primary unrewritten. So the finished phase idled through a review interval (1707 build; 1691 needed an operator touch) | process | ~20 min | resolved 2026-10-01: completion on worktree evidence ([ADR-0113](adr/0113-correction-completes-on-worktree-evidence.md)) |
| a passed build aborted when the explanation floor exhausted its correction budget (1707, after a rebase and a passed re-audit) | form | the cycle | F0, E1/E2 |
| a verified build (ACS 10/10, EGPS red 0, mutation probe) sealed FAIL. The explanation document had a sentence about a host-consumed inbox record, and it contradicted the consumption stamp (1712 audit). The floor demanded the path twelve times across 1707, 1708 and 1712 | form | the cycle | X1 (shipped), X2 |
| a top_n card that named a protected surface sealed the cycle FAIL. The item was routed only by the failure path (1714). The item declared no files, and nobody told the agent the surfaces | process | the cycle | R1 (shipped), R2 (shipped) |
| a test-only build that explained itself was refused, because the floor counted the own record of the cycle as immutable history. The correction rung re-dispatched the builder to delete it (1718 build) | form | a builder dispatch (plus a codex `model_unsupported` bounce) | X3 (shipped) |
| the audit FAIL of a verified build named only its explanation document. The repair round re-ran TDD and the whole build before the re-audit, and the TDD pane wedged on the way. The brief lacked the defects ("findings artifact … unreadable") (1745 round 1) | form | a TDD and a build dispatch, a wedged pane | X2a (shipped), X2a.5 (built) |
| the importer backstop of the ship went RED on `bridge/channel.TestChannel_EndToEnd` (1718 ship → re-audit). It is a timing-window test in a package that the cycle did not touch. It failed under the load of 89 targets | process | a re-audit and a second ship try | the test waits on the feed (shipped); F7 (shipped); F7b (designed) |
| a debugger resolved a genuine fleet-rebase conflict on the base that the ship diverged from. The cycle re-authored and re-audited there, and its next ship diverged again (1719 ship, twice) | process | an audit round and a ship try | C5 = B5 (shipped) |
| the plan offered lanes that the launch gate refused (items blocked on a console-owned dependency). A launch of nothing counted as a wave (waves 19 and 20) | process | a lane in wave 19; all six iterations of the batch of wave 20 | W3 (shipped) |
| the codex deep pin fails `model_unsupported` on the entitlement of the account at every deep or top dispatch (waves 15 to 20) | process | about eight boots per wave before the claude fallback | Q5 (designed); the pin is the call of the operator |
| the folder-trust dialog of claude was answered with a blind `Down,Enter`. The Down was dropped, and the Enter confirmed "No, exit". The readiness gate halted the wave (wave 59, 1 of 38 fires in waves 30–59b) | process | a wave, before any cycle | resolved 2026-10-05: one rule per menu state, Enter only on the observed Yes. Boot waits tick and judge one capture per pass, re-poll after a send and end on the loop guard as a boot timeout ([ADR-0118](adr/0118-confirm-a-menu-only-on-the-observed-choice.md)) |
| the startup update menu of codex took every codex launch of wave 59b. See the codex-menu details below the table | process | 19 codex launches (16 exit 81, 3 exit 80), each paid before its claude fallback | resolved 2026-10-05. See the codex-menu details below the table |
| the untagged self-check of the build floor ran a changed package under its own `-timeout 120s`. See the build-floor details below the table | process | the cycle, four times | resolved 2026-10-05. See the build-floor details below the table |

**Codex-menu details (wave 59b).**

- The boot judged readiness on the composer that codex draws before the menu mounts.
- The Enter of the prompt confirmed `Update now` (a brew upgrade that fails in the sandbox and exits codex to the shell).
- The bespoke `2`+Enter boot path matched the answered menu in the scrollback until the boot timed out.
- Resolved 2026-10-05:
  - Every realized codex launch turns the update check off (`-c check_for_update_on_startup=false` in the `default_args` of the manifest).
  - Manifest rules answer the menu on the observed cursor (Down off Skip, Enter only on the observed Skip).
  - The bespoke boot path is deleted.
  - Every smoke realizes the manifest.
  - A menu that no rule parses is held and fails closed ([ADR-0118 addendum](adr/0118-confirm-a-menu-only-on-the-observed-choice.md#addendum-2026-10-05-codexs-update-menu)).
  - The composer-first readiness race is filed (inbox `codex-readiness-judged-on-the-composer-drawn-before-a-startup-dialog`).

**Build-floor details (1787, 1791, 1792, 1798).**

- A green `./cmd/evolve` takes 105–127 s on the loop host. So the floor killed it on every correction round with a timeout dump from an unrelated test. The builder had no way to fix it.
- 1798 was locally ok at 105/109/115 s, ACS 178/178.
- Resolved 2026-10-05:
  - Every floor `go test` runs under `addedtests.PackageTimeout`.
  - A source scan refuses any other `-timeout` value in pipeline code.
  - The runner and the budget of the floor are protected surface ([ADR-0083 amendment](adr/0083-worktree-add-transience-classification.md)).
  - The slow-package alarm moves to inbox `cmd-evolve-unit-wall-time-and-package-budget-check`.

No `missing_section` or `bad_verdict` rejection is recorded in the range. So the recovery agent (F4/F6) lands last, after H1/H2 and the evidence decision are measured again.

Cost: an all-form rejection of a qualifying phase costs one host run of the predicates of this cycle. The run covers `acs/cycleN` only, bound to the tracked tree.

- The result of the build floor is reused by tree hash.
- An audit seal is reused only when its evidence verifies for the current tree.
- Document phases run nothing.

`workflow.recovery_rounds` bounds the cost. The recovery agent is one bounded LLM call that replaces a whole-phase re-dispatch.

## 11. Rollout

Landing order and the checkpoint each must pass before the next starts.

| Step | Lands | Checkpoint |
|---|---|---|
| 1 | D0 (#655), P1 (#652), the code train (#656) | merged at a wave boundary in one burst. The plane synced. The full floor is green on each |
| 2 | soak one wave | a triage lane that omits `triage-decision.json` proceeds without `GATE_CONTRACT_REJECTED [missing_secondary]`. A codex `rate_limit` escalation shows `cause_code=rate_limit`. The first fleet-rebase recovery logs "unwound its ship commit" |
| 3 | P2 (dossier at the boundary), P3 (identity prompt, no suggestions — shipped 2026-09-27) | the closeout of a FAIL sibling no longer moves `main` under a passed lane. No agent refuses on identity |
| 4 | E0 → E1 → E2 → F0 → F6a | an all-form failure with not enough evidence is re-dispatched with `Missing`. Identity, block count and signals are byte-identical with and without E2 |
| 5 | V0, F7 | the approval of salvage routes PASS. A rung on an empty primary keeps FAIL |
| 6 | F2b, F3b, F4, F5, F6, F6b, F8 | a malformed build report is repaired and approved without re-dispatch. A guard violation aborts with a P0. A failed rung leaves the breaker unchanged. A resumed cycle reaches the rung |
| 7 | L1, L2 | the dossier and the audit prompt list the rung, paths and evidence. Exhaustion files a P0, and the breakers still count |

Merges happen only at wave boundaries. Each step is its own PR; each component is its own commit with its tests.

## 12. Open questions and risks

- **The codex deep/top pin** (`gpt-5.6-sol`) is rejected by the account since 2026-09-14; every codex deep dispatch fails over to Claude. Re-pinning is a model-cost decision for the operator.
- **`allow_network` is not enforced** by the wrapper for any profile today. Until the filed item lands, the filesystem grant is the only OS boundary for every helper, including the recovery agent.
- **E1's predicate snapshot** (the TDD-end bytes of `acs/cycleN`) needs a kernel capture that does not exist yet; it lands with E0.
- **Floor rejections without codes** (F0) hide Build's commonest form failure from the routing table until they carry codes.
- **Provenance granularity** (F2b): byte ranges are strict; a repair that reorders a table can need line-level matching. Decide with the first real repair.
- **Document-kind cycles**: E1's `solutioncheck` path is designed, not measured.
- **Other CLIs' suggestion features**: codex, agy and ollama panes show no next-prompt suggestion today. If one appears, its off-switch is a `default_env` entry in the manifest of that CLI, not code.
- **One rule for a walled dispatch**: the ladder of the first dispatch still wants two all-85 tries. The check is `allFamiliesQuotaExhausted`, from before the tiered chain walked every family in one `Run`. Q1 reads one walled `Run` as the same fact. Unify on `isQuotaWall` once the tests of the ladder model the chain.
- **Completion after a correction** (resolved 2026-10-01, [ADR-0113](adr/0113-correction-completes-on-worktree-evidence.md)): the bridge used to complete a corrected phase only when the primary artifact was rewritten.
  - Every correction directive now asks for an appended `## Correction N` section.
  - A correction of a source-writing phase whose agent changed the worktree but not the deliverable completes at the first idle checkpoint on that evidence. The verify and review gate of core judge the unchanged deliverable.
  - It did not wait for F0. The evidence is the worktree change of the agent, not a floor code.
  - The re-author section of X2a still asks Build to rewrite `build-report.md` (§5.14). That now completes through the ordinary window.

## 13. Review log

Each entry gives the date, the review, the verdict and what changed.

- **2026-09-26: architect, round 1 (design).** Verdict: APPROVE-WITH-CHANGES. What changed:
  - The sandbox claim is corrected.
  - V0 verdict refresh.
  - Whole-workspace fence.
  - Breaker-neutral checks.
  - H1/H2 into the host step.
  - F6b shared ladder.
  - L2 relabel only.
  - Integrity class.
  - Auditor visibility.
  - Landing order by cycles lost.
- **2026-09-26: architect, round 2 (the evidence decision).** Verdict: APPROVE-WITH-CHANGES. What changed:
  - A missing report is always assigned back.
  - Per-kind evidence with the own unweakened red-on-base predicates of this cycle.
  - The footprint is a fact, not a condition.
  - Document-kind evidence.
  - Floor codes (F0).
  - H1 as the strict mode of the existing projector.
  - E2 changes only the directive.
  - Registry allowlist.
- **2026-09-26: docs review.** Verdict: WARNING → APPROVE. What changed: the policy is renumbered as §0, and §4 names control phases. The re-check ran after both revisions were approved.
- **2026-09-26: go-reviewer (train).** Verdict: APPROVE-WITH-MINOR. What changed: four minors applied.
- **2026-09-26: code-reviewer (train).** Verdict: WARNING. What changed: the write-in-flight grace of the derivation (MAJOR) is applied. The duplicate projector is absorbed.
- **2026-09-26: security-reviewer (train).** Verdict: BLOCK → APPROVE-WITH-MINOR. What changed: unfenced boundary; allowed-path type check; no network declared; anchored markers. Three gaps are filed.
- **2026-09-27: code-simplifier, go-reviewer, code-reviewer (Q2).** Verdicts: no edits; WARNING → fixed; WARNING → fixed. What changed:
  - The stale reset hint of a login pane never sets the bench.
  - The fix names no login command (the logins of the families differ).
  - A credential bench holds for routing until a probe clears it.
  - The fix is durable on the bench entry.
  - The design rows state the shipped shape and its trade-offs.
- **2026-09-27: code-simplifier, go-reviewer, code-reviewer (W1, H1b).** Verdicts: one gofmt alignment; PASS with three MINORs → applied; PASS with two MINORs → one applied. What changed:
  - A present item whose move fails is never stamped unbacked (regression test).
  - The lock-free write is an idempotent create, said in one line.
  - The no-work closeout logs what it routed and retired.
  - The plain-id guard is the one place where a path is built from the id of a caller.
- **2026-09-27: code-simplifier, go-reviewer, code-reviewer (R1, R2).** Verdicts: the rewrite split into two helpers; BLOCK → fixed; BLOCK → fixed. What changed:
  - An id-less card and a card that the decision never committed both fail closed. The removal set can never hold an empty key. A report-versus-decision mismatch is a fault, not a silent commit.
  - The ACS predicate that pins the admission tests by name follows the renames.
  - The atomic JSON writer of the package replaces an inline copy.
- **2026-09-27: code-simplifier, go-reviewer, code-reviewer (P1: B3/B4).** Verdicts: two redundant rebinds; PASS with a MAJOR → fixed; WARNING with the same MAJOR → fixed. What changed:
  - Ship trusted the gate results of the record from an unchained ledger read. Ship now verifies the chain (0.4 s on 147K lines). It binds the patch-id of the record to the bytes that it proved.
  - The ship-side gate check goes. The writer refuses red gates, and the chain proves the writer.
  - The git adapter forwards the real exit code.
  - The orchestrator declines a carry whose auditor row was bound on another base.
  - The reviewers confirmed that the full-index byte identity subsumes the lineage check of B2 at ship.
- **2026-09-27: go-reviewer, code-reviewer (Q3).** Verdicts: PASS with two MINORs → applied; PASS with two MINORs → one applied. What changed:
  - The skipped failure closeout is intentional (a wall is resumable, its claims stay claimed). Every wall producer wraps it as a cycle-level failure. Both are said in one comment.
  - The summary assertion parses the JSON of the loop and does not match its spacing.
- **2026-09-27: code-simplifier, go-reviewer, code-reviewer (Q1).** Verdicts: one closure; APPROVE-WITH-MINOR → applied; WARNING → justified and fixed. What changed:
  - The one-sample reading of a walled `Run` is stated in code and §12.
  - The deferral records the total dispatches of the phase.
  - The digest assertion is made non-vacuous.
- **2026-09-27: code-simplifier, go-reviewer, code-reviewer, security-reviewer (P3).** Verdicts: no edits; APPROVE; WARNING → fixed; APPROVE-WITH-MINOR → hardened. What changed:
  - The sole-writer line was false for stdout-completion phases (fixed).
  - A manifest `default_env` was able to set credential or loop variables that the guards never see (now refused at parse).
  - Facts that the block renders are sanitized.
  - No manifest pattern is allowed to match the block (pinned).
- **2026-09-26: consistency audit (every doc against the design and the shipped code).** Verdict: INCONSISTENCIES-FOUND → fixed. What changed: three stale package pages, one stale sentence in phase-architecture.md, and one imprecise ADR sentence are fixed. Two new package pages are added.
- **2026-09-26: code-reviewer (design document).** Verdict: APPROVE-WITH-MINOR. What changed: the exit-85 census is corrected, the audit-seal clause is restored, and F2b is cross-referenced.
- **2026-09-27: X3 (§5.9).** Reviewers: code-simplifier (no edit), go-reviewer (PASS, one MINOR), code-reviewer (PASS, none).
  - The MINOR of go-reviewer: an undeclared own record is unvalidated content that becomes an immutable published record. This is now stated in the contract doc and the §5.9 table.
  - The code-reviewer traced the four status × material combinations through build and Verify. It confirmed that the removed mirror check is dead behind `diffSHA256`, which hashes every changed path.
- **2026-09-27: F7 (§5.9, §7.3), reviewed twice.**
  - First pass:
    - code-simplifier merged two dedupe-and-sort copies.
    - go-reviewer: FAIL. The per-package merge dropped the names of an unproven package and allowed a wrong blame on a proven-green one. It was untested. The still-red test was unable to tell first from second.
    - code-reviewer: WARNING. Re-running one test alone hides a same-package order dependency. The line overclaimed proof. The WARN never reached the Signal Center. Cleared packages were not logged.
  - Reworked:
    - The whole package re-runs by itself.
    - An unproven package keeps its first-run reds.
    - The merge is unit-tested through the `goTestJSONFn` seam.
    - The outcome of every package is logged.
    - The line says evidence, not proof.
    - `backstopFlakeSignal` emits `ship.warning` `SHIP_BACKSTOP_FLAKE`.
  - Second pass:
    - go-reviewer: PASS, none.
    - code-reviewer: WARNING. The origin of the event must name a real symbol (now `Phase.runNative`). A load-dependent regression can still clear on one solo run (disclosed in §5.9; F7b is the operational backstop, designed).
- **2026-09-27: C5 (= ADR-0105 B5; §5.8, §5.9), reviewed twice.**
  - First pass:
    - code-simplifier: no edit.
    - go-reviewer: BLOCK. There was no `inPlaceWorktree` guard before the first git mutation. A repo-wide `git add -A` was able to carry untracked debris.
    - code-reviewer: WARNING. The RERUN target of the debugger was discarded. The re-entry bypassed the recovery budget and backoff. The git sequence has no checkpoint for resume.
  - Reworked:
    - The guard checks the in-place worktree.
    - The carrier stages tracked paths only and says what it leaves behind.
    - A Build or Audit re-run that the debugger asked for runs on the rebased tree. A tests-first request yields to the route and says so.
    - The budget is checked, the backoff is applied, and a re-entry counts one try.
    - Every abort emits `ORCHESTRATOR_REBASE_REENTRY_ABORTED`.
    - The resume heal is C5b, designed.
  - Second pass:
    - go-reviewer: PASS. One MINOR (warn on leftover untracked files) applied.
    - code-reviewer: PASS. Two MINORs: a full debugger round trip costs two budget units, and B5 extends the ship-ahead-of-the-heal precedent. Both are recorded in ADR-0105.
- **2026-09-27: W3 (§5.10, §7.7), reviewed in five passes.**
  - First pass:
    - code-simplifier: no edit.
    - One combined code and Go review: PASS, none. It traced two facts: only the freshness gate can return no results for a non-empty plan, and the one-lane repair inherits the fix.
    - architecture-reviewer: WARNING, FIX_THEN_MERGE. All findings are fixed except one LOW:
      - HIGH: the triage menu and `evolve inbox batches` still offered items that waited on a dependency, that the sequential fallback can now pick. `inboxmover.PartitionPending` holds them back in both, listed loudly.
      - MEDIUM: a zero-lane wave no longer reached the starvation tracker. The coordinator observes both paths through one helper, `observeWorkSupply`.
      - MEDIUM: the planner read two lifecycle roots that `--evolve-dir` separates. Every planner reader reads `EvolveDir` (Q-W7 and Q-W8 resolved).
      - LOW: stale names, the rule restated in four docs, and the different signal of the seed path. Fixed, and the docs link to the rule.
      - LOW: the `consumed: processing` wording for in-flight ids. Kept, pinned by goldens that operators grep. It is renamed when those goldens next move.
  - Second pass:
    - The combined code and Go review: PASS with three MINORs, applied. A local named `waiting` held reasons. `dependency_blocked` shipped strings beside the records of `console_routed`, now one `excludedItem` shape. The batches command is extracted from `runInbox`.
    - The architecture review confirmed the three fixes and raised a HIGH and three MEDIUMs:
      - The dependency grouping of the classifier is now unreachable. It is recorded as W8, a deletion with its pins. It is not in this landing, because it touches a documented classifier feature.
      - The menus hide a waiting item, but the claim floor does not refuse one (W7).
      - The repair width was a bare 1 in two places. It is now `loopwave.RepairWidth`.
      - `dispatchFleetIteration` exceeded 50 lines. It is split into `dispatchPool`, `waveRequest`, `completeWave` and `repairMinWidth`.
    - An inbox review then found the sixth copy of the rule. The sequential no-work check counted protected-surface and waiting items as claimable. With that check, the empty-backlog path of W3 was at risk of a sealed, misattributed FAIL. The check now reads the same lane menu (`core.WithLaneMenu`).
  - Third pass (architecture and one combined code and Go review, on the last delta):
    - The extraction and the default are preserved.
    - A MAJOR regression is fixed red-first. The no-work check had run the claims of this cycle through the dependency rule. With that code, a claimed waiting item was at risk of ending as silent no-work and stranding in processing. Claims now keep the routing-only rule.
    - The lane-menu rule composed in two places (HIGH). It became one per-item Specification, `PlaceOnLaneMenu`, with an agreement test that pins the backlog of the planner to the menu.
    - The lane path and `laneMenuOf` gained tests.
    - The routing backstop of the pool reads the configured `EvolveDir`.
    - Stale origin strings and doc wording are fixed.
  - Fourth pass (architecture and one combined code and Go review, on the delta of the third pass; six of seven mutants caught):
    - Stale text is fixed:
      - internal-loopwave.md restated the deleted `PartitionPending` and listed the pool among the callers of `RootsOf`.
      - A `nullWaveEngine` comment listed a caller it no longer has.
      - §8 lacked the lane-menu exports.
    - The uncaught mutant is closed (console routing judged before the dependency rule, now a test row).
    - The claim of the agreement test is narrowed to items that carry an id. The id-less difference is pinned as the characterization test of W5.
    - The helper of the claim scans is renamed `admittedIDs`. The word "dispatchable" on the one scan that must not ask the dependency rule invited the regression of the third pass.
    - W5 is recorded as reachable through the sequential fallback of W3, with the ids stamped before the next launch.
  - Fifth pass (the three lenses on the delta of the fourth pass):
    - Every fix is confirmed, and each new test kills only its own mutant.
    - The rationale that moved into internal-core.md exposed a HIGH that W3 routes the P0 through. The no-work check failed closed on any loader warning. Those warnings include notices about readable items (a sanitized field, a duplicate id), which the tracked inbox carries on 48 of its 121 items. So a sequential cycle whose triage honestly committed nothing sealed a FAIL.
    - Fixed red-first: the loader warnings are typed (`inboxbatch.ScanDir`, `LoadWarning.Unreadable`, `LoadDir` as its projection). The check fails closed only on an unreadable file.
    - The stale comments on the check are deleted.
    - The doc wording is corrected: the pending-only menu judgment, the second known backlog difference, and the scope and check of the W5 guard.
- **2026-09-28: F43 component 4 (§5.12), reviewed five times.**
  - code-simplifier: no edit.
  - Architecture review of the first version (audit onto the selection of Ship): BLOCK.
    - The binding (`worktreeContentSHA`) was a third ship tree, and Ship checks the receipt and the staged tree against it.
    - The refusal of undeclared tracked edits rested on a wrong premise (the `add -u` of the binding ships them).
    - Result: redesigned as one function, `shipmanifest.TakeShipTree`.
  - Design review of the redesign: FIX_THEN_MERGE. Declared-only selection under a workspace; `TakeTracked` deleted; `shipmanifest` off `core`; the empty guard in `StageRetrying`. Applied.
  - Delta architecture review: FIX_THEN_MERGE. The `add -u`-first order failed a declared plain-`rm` deletion rc=128. Reversed, pinned red-first.
  - Final architecture delta: FIX_THEN_MERGE. A swallowed `add -u` error survived. Pinned by a fault test.
  - Combined code/Go review: PASS with two MINORs → applied.
  - Delta: BLOCK. Ship relied on the `add -u` of the binding. The worktree path runs `add -u` itself, with an end-to-end test.
  - Re-verdict: BLOCK (declared deletion under the order of Ship). Refuted by `TestShipFromWorktree_ADeclaredFileRemovedWithPlainRmShipsItsDeletion` and a gone-path mutant → PASS.
- **2026-09-28: go-reviewer ×3 (P2 port: dossier leaf, core destination, cmd wiring; each diff-scoped on its staged commit).** Verdicts: PASS, PASS, PASS. What changed:
  - Leaf: pending read errors wrapped. A MEDIUM on the best-effort reset of `unstagePair` is left as a follow-up. It predates P2 and serves every dossier commit.
  - Core: one comment reworded.
  - Cmd: a WARN that asserted what it had not checked is reworded.
- **2026-09-28: the P2 review (architecture reviewer, code reviewer).** Verdict: FIX_THEN_MERGE → fixed. What changed:
  - H1: the publisher refuses while another run is live (`loopchain.FleetLaneActive`), not only at its call sites.
  - MAJOR: a pending pair never overwrites a corpus record. The restore-prior-bytes path and its surviving mutant H2 are deleted.
  - MINOR: a non-regular pending file is refused.
  - M2: the fleet binary is resolved in one place.
  - LOW: the publish of `evolve fleet` is pinned by behaviour through `fleetLaunchFactory`.
- **2026-09-28: the P2 re-review (architecture reviewer, code reviewer).** Verdict: every earlier finding closed; four new findings fixed. What changed:
  - MAJOR: the live-run check re-runs under the lock (TOCTOU).
  - HIGH: the discovery-error hold is pinned.
  - MEDIUM: the WARN names the blocking run and the reason through a new `loopchain.LiveSiblingRun` (`FleetLaneActive` delegates). It points at `evolve cycle reset`.
  - LOW: an identical untracked corpus record is committed.
- **2026-09-29: X2a (§5.14).** Reviewers: code-simplifier, one combined code and Go review, architecture review. Verdicts: no edit; PASS with one MEDIUM → fixed; Approve (0 CRITICAL, 0 HIGH, 3 MEDIUM, 2 LOW) → four fixed, one declined.
  - MEDIUM (code/Go): prose before the path (`e.g.`, `2.5`) was read as the location of the defect. `pathLike` now wants a two-character stem and a letter-led extension (red first, `probe.c` guard).
  - MEDIUM: `retry@explanation` joins the `retryAction` vocabulary block.
  - MEDIUM: §5.14 claimed that the adjudicator reads the record (it runs first). Corrected.
  - MEDIUM: a re-author Build that dies before the re-audit is digested from the record. Stated here. The class fix (the brief reads the failure block for every agent-graded FAIL) is filed as an inbox item.
  - LOW: the misroute cost is a retry try, not only a dispatch. Stated.
  - LOW declined: a `granted()` predicate for the envelope (one caller).
  - The mutation sweep of the architecture review: 11 mutants, 11 killed.
- **2026-09-29: X2a.5 (§5.14).** Reviewers: code-simplifier, one combined code and Go review, architecture review and re-review. Verdicts: no edit; PASS with one MEDIUM → resolved. FIX_THEN_MERGE (0 CRITICAL, 1 HIGH, 2 MEDIUM, 3 LOW) → Approve, MERGE (0 CRITICAL, 0 HIGH, 2 optional LOW → both taken).
  - MEDIUM (code/Go): a record that existed but was unreadable gave a WARN and emptied the gate slot. Resolved: the brief no longer reads the file.
  - HIGH: four mutants of the dedupe survived (the cap, the title form, the empty-section guard, the case fold). Two tests kill them, each mutant run before (exit 0) and after (exit 1).
  - MEDIUM: the findings header called the defects of the failure block "symptoms". The clause now appears only when the reasons of the runner lead.
  - MEDIUM: "a runner gate diagnosed this FAIL" had two homes (the in-memory carrier for the classifier, the file for the brief). The brief keys on `CycleState.AuditFailReasons`. Red first on a leftover record of another phase, a directory at the path and an unwritten record. The gate brief is pinned byte for byte, checked against an archive of origin/main.
  - LOW: the briefed selection is stated once (`briefedFindings`). The doc inconsistencies (the inbox move, a status, a sentence about the re-author section) are fixed.
  - LOW declined: moving the dedupe predicate into `reportdoc`. The compared forms are the own rendering of the brief. `FindingKey` is a looser lead-clause identity.
  - Re-review LOWs taken: the predicate is named once (`runnerDiagnosedAudit`, three sites), and a stale comment clause is deleted.

## 14. Document history

Each entry gives the date and the change.

- **2026-09-26.** Created after the design landed in ADR-0106 and the first six components shipped (#654).
- **2026-09-26.** Review (APPROVE-WITH-MINOR):
  - The exit-85 census is corrected. The eight unknown-prompt exits were four `rate_limit` and four `model_unsupported`.
  - The audit-seal reuse clause is restored.
  - F2b is cross-referenced from the ADR.
- **2026-09-26.** F3 is routed to the codex family with claude as fallback. The balanced-tier floor (`TestClaudeFamilyFloor`) reserves claude for judgment phases with a justification. The recovery agent is an analytic helper.
- **2026-09-26.** #652, #655 and #656 merged at the wave-13 boundary (1706 shipped, 1707 failed on form). Statuses are updated. Two wave-13 observations are added to the evidence and the open questions.
- **2026-09-27.** Q2 shipped on the P3 train. A credential wall benches like a quota wall. The bench entry carries the fix of the operator, and the bench lines print it (code review: the fix must outlive the log tail). Deferred:
  - A Signal Center ERROR for the wall.
  - The operator wording of the defer message.
  - Protection for floor-pinned judgment phases (a bench is advice, not a veto).
- **2026-09-27.** Q1 shipped on the P3 train. Five tests pin the three seams and the negative case. Eleven mutants were killed. `isQuotaWall` reads the exit 85 of the runner alone, because the sentinel half of the check had no caller (a mutant proved it).
- **2026-09-27.** §5.5 and §7.5: the wave-14 deep-dive (three consecutive FAILs: 1707 form, 1708 and 1709 capacity) designs Q1–Q4.
  - Capacity is a deferral through the one seam that already exists.
  - A credential wall is an operator halt.
  - Neither counts toward the FAIL streak.
- **2026-09-27.** P3 shipped.
  - §5.4 records the design: a driver-appended statement; an environment channel over a settings flag; what it does not fix.
  - §8 holds the signatures.
  - §12 holds the other-CLIs question.
  - Wave 14 (1708, 1709) failed on capacity. Every CLI family was walled (`auth_recheck`, `rate_limit`, `model_unsupported`), with `cause_code` naming each pattern. This is the live proof of P4.
- **2026-09-27.** §5.6 and §7.7: the wave-15 deep-dive. In it, 1713 planned no-work on a pin that 1706 shipped and that carried through 1709 and 1710. The entry designs and ships three items:
  - W1, the retirement record for an id that no inbox item backs.
  - H1b, an empty optional bucket read as no cards.
  - X1, the inbox record made non-material to the explanation document (the verified build of 1712 sealed FAIL on one sentence about a host-stamped file).

  §8 holds the signatures. W2 and X2 are designed.
- **2026-09-27.** §5.7 and §7.8: the first cycle of wave 16 (1714) sealed FAIL on a top_n card that named a protected surface.
  - R1 moves such a card into `escalate_block` by the hand of the host and lets the planned no-work closeout route it (never a verdict).
  - R2 tells the agent the surfaces and the drop reason.
  - §8 holds the signatures. The evidence row is added.
- **2026-09-27.** §5.8: the carry (P1 = ADR-0105 B3/B4) is designed as four components after 1712 and 1715 each re-audited a byte-identical rebase. The P1 row points at it.
- **2026-09-27.** §5.8 C1–C4 shipped: a byte-identical rebase ships on its audited verdict.
  - `treedelta` leaf.
  - The carry record names the audited tree.
  - `identityCarryForward` after the rebind.
  - Ship re-proves the carry inside its one binding rule.

  The P1 row and §8 are updated. The gate re-run in ship is scoped out as C4b.
- **2026-09-27.** Q3 is verified and closed. Both breakers read only failure digests or FAIL verdicts, which a deferral never produces. The one gap was the sequential dispatcher. It recorded a failed approach and ran the failure closeout before its quota pause. Those two steps now run after the quota check.
- **2026-09-27.** §5.9: the test-only build of cycle 1718 explained itself, and the floor refused its own record as immutable history.
  - X3 makes foreign-only immutability the one rule in every branch.
  - X3 verifies a declared explanation of a non-material diff with an empty material set.
  - X3 branches the Verify mirror on the recorded status.
  - Also added: the X3 row, and the evidence rows for the build and the ship of 1718.
  - Also added: the feed wait of the channel test, and F7 designed.
- **2026-09-27.** F7 built and shipped. The importer backstop re-runs its named reds once alone before any is the RED of the lane.
  - Green ⇒ `SHIP_BACKSTOP_FLAKE`, and the ship proceeds.
  - Red ⇒ the RED of the lane.
  - Ambiguous ⇒ the first verdict.
  - A build failure never re-runs.
  - At most three named reds.

  The F7 row, the last paragraph of §5.9 and the 1718 ship evidence row are updated. F7b (the inbox item) is designed.
- **2026-09-27.** §5.8 C5 (= ADR-0105 B5) and §5.9: the debugger resolved the fleet-rebase conflict of 1719 on the stale base, and the next ship rediscovered it. The resolved tree now rejoins the rebase route. The evidence row is added. The partition gap (edits outside the declared files of an item) is recorded.
- **2026-09-27.** §5.10: the review of waves 11–20. All eleven ships delivered their task. There are five architecture findings, each a replicated or unenforced belief.
  - W3 shipped: one dispatchability rule, the shared ranking of the refill, and `Ran` means a lane launched.
  - W4, W5, W6, Q5 and §7.9 (A1, A2, G1, G2, G3) are designed.
  - Two evidence rows are added.
  - The fixes of the architecture review: the triage menu, the one root, the zero-lane starvation.
- **2026-09-28.** Wave 21 (the first with W3 live) shipped 1721, 1722, 1723, 1724 and 1726 on fresh inbox work, which the planner of W3 offered. 1725 met a genuine fleet-rebase conflict on `go/.apicover-enforce`. The read-only fence reverted the resolution of the debugger, and the cycle ended WARN with its claim stranded.
  - Fixed: a cycle that the debugger ends is a diagnosed FAIL, so the failure walk releases its claim.
  - Fixed: the append-only registry merges as a union (`.gitattributes`).
  - Open: the fence (`fleet-rebase-debugger-resolution-fenced`).
  - Open: the boundary re-exec that restarts the cycle budget and the wave index.
  - Open: the carry that declines an audit already bound on the rebase target.
- **2026-09-28.** §5.8 C3: the carry reads the base on which the audited worktree stood.
  - The auditor row gains `worktree_base_sha` (`recordAuditBinding` writes the worktree base of the cycle).
  - `identityCarryForward` compares it with base0, not the `git_head` of the row (the HEAD of main when the audit was bound). An older row falls back to `git_head`.
  - Cycles 1724 and 1727 each paid a second audit of a byte-identical rebase, because a landing of a sibling had moved main before their audit.
  - `carry-accepts-audit-bound-on-rebase-target` is consumed.
  - The review added three points:
    - `auditledger.Entry.AuditedBase` owns the projection (the package of the row schema).
    - `carriedAudit` declines a row that names no base.
    - A reflection-driven round trip pins every `LedgerEntry` field through the hand-kept wire struct.
  - Also found: RUNG 0 holds the same belief in two places. One is its snapshot (`cmd/evolve/cmd_composition_wiring.go`, `main...git_head`). The other is the composition check of ship (`internal/phases/ship/composition.go`, `AuditedBase` against `git_head`). Filed as `composition-snapshot-reads-the-worktree-base`.
- **2026-09-28.** §5.8 C5 (= ADR-0105 B5), the fence: the debugger of a fleet-rebase conflict writes its resolution.
  - The rebase returns its non-derived conflicted paths.
  - `CycleState.ShipRecoveryConflicts` records them (cleared by the ship latch).
  - `PhaseRequest.WorktreeWritablePaths` hands them to that debugger alone on the live and resume dispatch.
  - Its prompt names them.
  - `treefence` keeps writes to them and reverts every other write (ADR-0097 decision 6).
  - A genuine conflict through a debugger held by the production fence pins it end to end. It shipped on main.
  - The review added three points. Conflicted paths are read with `-z`, so the fence compares raw paths. The prompt lists them bare, and the persona alone says what they mean. Three kill tests were added.
  - `worktree-fence-one-value` is filed (the out-of-band retro runs unfenced; the fence pair can be half-set).
  - `fleet-rebase-debugger-resolution-fenced` is consumed. Its third acceptance line (the union attribute in `.gitattributes`) had landed in #680.
- **2026-09-28.** §5.11: wave 24 went 0/2 (1733, 1734) after the streak reached five (1727, 1729, 1730, 1732, 1731). Both cycles walked a stalled or refusing claude-tmux and a walled codex-tmux into the structural refusal of ollama-tmux, which sealed FAIL.
  - T1 and T2 are built. The tail holds only drivers that can run the phase. An exhausted walk that met a wall surfaces the wall and defers.
  - T3 and T4 are designed.
  - The loop halted for the P0.
- **2026-09-28.** §5.11 T4 built. The plan path evaluates a trigger that is the whole admission rule of a phase (`insert_when` with no `rubric_hint`) at the turn of the phase. It skips a planned phase whose trigger does not fire (`insert-when-gates-plan`). Two floor-activation scenarios that pinned the old contract (a plan inserts tester; its trigger skips it) are rewritten. ADR-0052 is amended.
- **2026-09-28.** §5.11 T3 built and §5.4 revised.
  - A standing authority block (`phaseidentity.Authority()`) is the system prompt of claude-tmux (`--append-system-prompt-file`, manifest param `system_prompt_file`, `LaunchIntent`/`Realization.SystemPromptFile`). CLIs without the channel paste it.
  - The block of per-dispatch facts still ends every paste.
  - The first cut put the whole identity block in the system prompt. The architecture review found that a per-dispatch system prompt defeats the cache-stable system prompt of ADR-0071 and breaks the placement rule of this section. So the block was split.
  - The filed fix (a typed authorizing line) was dropped on the evidence of 1734: its pane refused the typed nudge too.
- **2026-09-28.** §5.12 F43: the ship tree of the audit is the own selection of Ship. Component 2 is built (`internal/shipmanifest`, `Stageable`, `RegularFileIn`). Components 3–5 are designed. A first design (a registry staging effect) was rejected by review as a second declared-outputs list.
- **2026-09-28.** §5.13: the size ratchet is a ceiling (cycle 1735 failed on three one-line shrinks whose allowances it had not lowered). R1 is built. The boundary tighten, the no-growth check and the single constants are designed. A host writer at the build handoff was rejected by review as a shared-registry writer.
- **2026-09-28.** The auditor persona states the rules that the code applies. Each rule is pinned with the behavior it describes:
  - WARN ships under the fluent default (only `workflow.strict_audit` refuses it).
  - An added comment is a LOW, advisory finding, and the auditor never asks for one.
  - `NEEDS_CORRECTION`, which the gate records as an advisory (ADR-0102), stays the FAIL of the auditor by stated policy until the document-only rung of X2 exists. The FAIL names only the document.

  §5.6 is corrected: the persona, not ADR-0102, makes it a FAIL. This comes from the inbox architecture review of 2026-09-28, which also filed N1–N6.
- **2026-09-28.** §5.12 component 4 built. The audit, the audit binding and Ship read one ship tree (`shipmanifest.TakeShipTree`: the index, every tracked edit, the declared paths).
  - The `TakeStaged` contract of component 3 is corrected (declared add, then `add -u`), and `TakeTracked` is deleted.
  - The selection is declared-only under a workspace.
  - The worktree path of Ship runs `add -u` itself.
  - An empty selection stages nothing.
  - The first version refused undeclared tracked edits. This was withdrawn on the premise correction of the review.
  - A pre-existing defect (the binary churn discard is a no-op after the binding) was filed separately.
- **2026-09-28.** P2 ported onto current main, test-first, in four commits (dossier leaf, core destination, cmd wiring, docs).
  - Drift resolved: `WithDossierCommit` became `WithDossierDestination`.
  - `prepareIteration`, `runFleet` and `runCampaignRun` sat at their size-ratchet ceilings. So the wave-binary block moved to `resolveWaveBinary`, and the fleet and campaign sites run their lanes through `runLanesThenPublish`.
  - The raw-git fixtures became `gittest`.
  - The text-order wiring test became behaviour tests whose ordering mutants fail.

  The P2 row is updated.
- **2026-09-28.** P2 review fixes: the publish refuses while another run is live, a differing or half corpus record, and a non-regular pending file. The fleet binary has one home. The P2 row is updated.
- **2026-09-28.** P2 re-review fixes: the live-run check also runs under the lock and names the blocking run. An identical untracked record is committed. The P2 row and the review log are updated.
- **2026-09-28.** Status pass (cycle 1746):
  - W8, G2 and G3 read shipped with the cycle whose PASS dossier consumed their inbox record (1724, 1726, 1729; each commit is on main).
  - P3 reads merged, #661 (`abc9ced0`).
  - The stale second **Status:** bullet of the header and the repeated walled-dispatch bullet of §12 are removed.
- **2026-09-29.** §5.14 X2a built. The audit FAIL of cycle 1745 named only the explanation document, and its repair round re-ran TDD and the build.
  - The class comes from the defects (`explanation-needs-correction`).
  - The granted envelope narrows to `retry@explanation` (a Build re-entry scoped to the document).
  - The defects are recorded in `audit-fail-reason.json`.
  - A mixed FAIL keeps today's round.

  Added: the X2a row, the evidence row, the §5.6 cross-reference, the §12 completion note, the fixes of the review and its review-log row. X2 (no FAIL at all) stays designed.
- **2026-09-29.** §5.14 X2a.5 built (inbox `repair-brief-reads-the-failure-block-for-every-agent-graded-fail`).
  - The repair brief read the reasons of the audit only from `audit-fail-reason.json`. So every agent-graded FAIL but the doc-only class gave a WARN "findings artifact … unreadable" on each tdd/build dispatch of its round. The brief listed the findings table without the defects of the failure block.
  - `auditRejectionReasons` now reads the reasons of the runner (`CycleState.AuditFailReasons`), or else the failure block, for the brief and the adjudicator.
  - The copy of the defects in X2a.2 is removed.
  - The "defects are recorded" bullet is rewritten as "read from the failure block".
  - A bullet states which failure a phase that dies inside a repair round is digested from, and why a gate record keeps precedence.
  - The X2a.2, X2a.5, §7.7 X2a and §10 1745 rows are updated.
  - After review: the gate slot keys on the in-memory carrier, not the file. The findings header names gate reasons only when they lead. The mutants of the dedupe are pinned. The review-log row is added.
- **2026-09-29.** §5.7 R1b and §7.8: lane 1757 of wave 39 sealed FAIL after 1756 shipped. Its only top_n card carried the slug of the scout, not the pinned id, and named `go/internal/loopwave/loopwave.go`. So R1 escalated the alias and left the pin unanswered (no route, no bump, first in line next wave).
  - The route now answers for the one bound item of the cycle (`committedset.Committed`: the pin minus deferrals) when it empties top_n. It names the card in the reason.
  - The multi-item residual is filed as `claim-failed-closeout-charges-unanswered-pins`.
  - §8 holds the signatures.
- **2026-09-29.** §5.7 R1c and §7.8: the claim-failed and planned no-work endings escaped the C1 chokepoint. Cycle 1757 classified `integrity-breach` and charged nothing. Cycle 1758 paged `FAILED_UNEXPLAINED`.
  - The closeout now records each ending through `RecordEnding` (the claim failure of a lane = task-level `TRIAGE_SCOPE_UNANSWERED`; no-work = `NO_WORK`).
  - The FAIL closeout charges `committedset.Unanswered`.
  - A zero-try record keeps the usage sidecar.
  - A seal-reading design was rejected in review (listeners never decide).
  - Inbox `claim-failed-closeout-charges-unanswered-pins` is consumed.
- **2026-09-29.** §5.8 C6: the composed-tree gates of the carry ran in the environment of the lane. They declined 14 of 32 carries in waves 27–41 on a false `test` red (each paid a deep re-audit that passed).
  - Reproduced: `make test` under lane state exits 2, and it is green in the CI environment.
  - The gates now run in `ciparity.CIEnv()`, the one allowlist that the CI-parity gate of the audit already used.
  - A failing gate names itself and its output.
- **2026-09-30.** §7.9 A2 shipped. Lane 1764 of wave 43 adopted the continuation of 1761 and failed at the build floor on the own predicate package of 1761. The allow-list fence of that package diffed against the base of 1761. It counted every file that main had gained since (the release, the gc landing, the dossiers).
  - Two consumers run every added tagged package: the added-test self-check of the build floor and the repo-contract pack of the ship backstop. So the fix is at the source.
  - `prepareAdoptedTree` archives the `go/acs/cycle<M>` of the ancestor (absent at the base) into the same dated archive as its explanation records. This is one retention rule.
  - The ACS suite already scoped a cycle to its own package plus the curated regression. The floor now sees the same scope.
  - Tests are red first (an end-to-end adoption probe at Build time, the selector table, the archive on a real repo). Mutation sweep: 5/5 killed.
- **2026-09-30.** Build explanation, a format-recovery rung (logic over format). A Changed Areas citation that is a directory, glob or brace list that covers Build-diff paths is accepted. Before, it cost a correction round (cycles 1765 and 1768 each lost one to `go/acs/cycle1765` / `go/acs/cycle1768/*_test.go`).
  - A pattern never credits a material path, so the per-path explanation stays owed.
  - Malformed brace lists (empty alternative, nested, unclosed, over 64 expansions) cover nothing.
  - A literal path is tried first.
  - The code is `changedAreaFailures` in `explanationdocs/changed_areas.go`.
  - Reviews: Go BLOCK (empty-alternative collapse to a bare prefix; unbounded expansion). Architecture FIX_THEN_MERGE (two unpinned mutants; literal-brace regression; `/...` and `/**`; persona and doc copies). All fixed. The nested-brace mis-parse of the Go re-review was fixed red-first. Mutation sweep: 9/9.
- **2026-09-30.** §5.8 C7: the carry never shipped. Every byte-identical carry (1766, 1768, 1772) passed `identityCarryForward`. Then each failed the predicate-execution tree check of ship, which compared the held tree with the audited one. That check never asked `auditBindingSatisfied`, where C4 had put the carry. Each paid a deep re-audit.
  - The check now applies the one binding rule (`verifyExecutionTree`).
  - A wiring test drives `verifyAuditBinding` on a carried rebase.
  - ADR-0105 named a proof test that was never written. It now names the real one.
- **2026-09-30.** Build explanation, a second format-recovery rung (logic over format). Changed Areas was read line by line. Three shapes left each path an explanation of a word or a comma:
  - a hard-wrapped bullet (``- `a.go` — migrates`` then the rest on the next line)
  - a colon followed by sub-bullets
  - a grouped bullet that names several paths before one explanation

  The floor sent the build back for a what/why that it had written. Eight cycles paid a Builder correction round to it: 1707, 1730, 1735, 1737, 1760, 1762, 1765 and 1772. The re-dispatch of 1772 ran at 131% context fill.

  - `changedAreaItems` now reads Markdown list items. Continuation lines and sub-bullets join the item. The next `- ` bullet at the same or a lesser indent, or unindented text after a blank line, ends it.
  - `splitChangedAreaItem` gives every path of a group the shared explanation. A span joins the group only when it covers a Build-diff path.
  - A nested bullet opens its own entry when its span names Build content or holds a `/`. So a nested invented path is still reported, as before. A backticked identifier stays prose and is never cited.
  - The `minExplanationBytes` floor, the diff check and the per-material-path check still run on each path.
  - Limit: `reportdoc.visibleLinesIdx` hides lines indented four spaces or by a tab as code before this reader runs. It is shared with verdict parsing, so its fix is its own component (inbox `reportdoc-indented-continuation-is-not-code`).
  - Reviews: Go BLOCK (a hyphen-glued prose span was grouped). Architecture FIX_THEN_MERGE (three unpinned mutants, the reportdoc limit, contract over-claims, copied values). Fixed by grouping on Build content.
  - Mutation sweep: 25/26 killed. The survivor is an equivalent guard.
- **2026-09-30.** §5.5 (capacity is not a verdict): wave 49 ended 0/1 on a capacity wall.
  - The prompt submit of claude-tmux wedged twice (transient; `evolve doctor live claude-tmux` was LIVE OK minutes later).
  - The codex fallback reported its account usage limit until 2026-10-14.
  - The item of the lane returned to the queue with no verdict.
  - The CLI-health bench read none of the dated banner of codex. So it benched 4 hours at a time and stored the pasted prompt as evidence.
  - `ParseResetHint` now reads a dated hint (capped at a day, so the canary still re-checks daily), and the bench keeps the banner.
- **2026-10-01.** §5.11 T4 rest (F42, the document lane of cycle 1692):
  - The deliverable-kind side of the plan clamp is phase data. It is a `skip_when` on `deliverable_kind` in every bugfix-category `phase.json`, pinned by a category invariant test over the real catalog.
  - Each registry-gated planned phase raises `ORCHESTRATOR_PLAN_PHASE_GATED`.
  - The plan input carries the pinned items of the lane, and the rubric projects `skip_when`.
  - The scout report fallback counts `### ` task headings. Every report-only cycle had digested `item_count=0`, so the shadow re-plan proposed to end 1692, 1770, 1774, 1779 and 1780.
  - Tdd for a document is released, not forbidden. Its reason reads `plan:tdd`, and the Task Contract prints the registry rule and its result. So no acceptance grades the routing of tdd.
- **2026-10-01.** §12 "Completion after a correction" resolved and the §10 row closed (console lane, inbox `correction-completion-needs-deliverable-rewrite`, [ADR-0113](adr/0113-correction-completes-on-worktree-evidence.md)).
  - The `worktree-evidence` completion contract carries the unrewritten deliverable of a correction forward when the idle agent changed the worktree (host paths excluded).
  - Every correction directive asks for an appended `## Correction N` section.
  - The pre-dispatch baseline guard holds for first dispatches, read-only phases and agents that changed nothing.
  - The predicate is on the protected-surface manifest.
  - Twenty-six mutants were killed.
- **2026-10-05.** §10 row added and closed (console lane, inbox `trust-prompt-confirm-only-when-the-cursor-is-on-yes`, [ADR-0118](adr/0118-confirm-a-menu-only-on-the-observed-choice.md)). Wave 59 halted at preflight. The blind `Down,Enter` of the claude trust dialog lost its Down and confirmed "No, exit".
  - The manifest now answers the dialog with two rules.
    - A navigation rule (`Down`, repeatable, bounded by the loop guard).
    - A confirm rule (`Enter`, only with the cursor seen on Yes).
  - Both boot waits tick and judge one capture per pass, and re-poll after any send. They end on the loop guard within seconds as `ExitREPLBootTimeout` (still a bench strike), not after the 60 s deadline.
  - The architecture review had three findings. The same change fixed all three.
    - The exit escaped the bench.
    - The recipe judged a frame that no rule saw.
    - The confirm regex matched a torn redraw.
- **2026-10-05.** §10 row added and closed (console lane, inbox `codex-update-menu-never-confirms-update`, [ADR-0118 addendum](adr/0118-confirm-a-menu-only-on-the-observed-choice.md#addendum-2026-10-05-codexs-update-menu)): no codex launch of wave 59b reached a working session.
  - codex 0.153.4 showed its update menu despite the `999.999.999` dismissal of the preflight.
  - The boot declared readiness on the composer that codex draws before the menu mounts. So the Enter of the prompt confirmed `Update now` (16 exit 81).
  - Where a boot pass saw the menu, the bespoke `2`+Enter kept matching it in the scrollback above the composer (3 exit 80).
  - Prevent, then defend, as data:
    - The `default_args` of the manifest carry `-c check_for_update_on_startup=false` (verified live: no menu with the same `version.json`).
    - Two rules in the shape of ADR-0118 answer the menu if it shows anyway.
    - The bespoke boot path is deleted, so no boot path sends keys outside the manifest rules.
  - Architecture review revision, the same day:
    - Every boot and live smoke (the readiness gate, `evolve doctor boot`, the CLI-health canary, `evolve doctor live`) realizes the manifest when its caller supplies none. So the canary no longer meets the menu (consumes `boot-smoke-launches-without-the-manifest-realization`).
    - A new boot-only `hold` policy fails closed on a menu that no rule parses. The cursor of the menu is the readiness marker of codex. The policy is dropped once readiness is declared, and it is last in the manifest.
    - The self-match guards slide the pane bottom over every line of each tracked file. This caught two claude fixtures that the numbered trust rule matched (now line-anchored).
  - Filed:
    - the composer-first readiness race (now also at the pre-submit capture)
    - bridge tests that write the real `~/.codex/config.toml`
    - the dead version sentinel
    - the whole-file gap of the plan-mode guard
- **2026-10-05.** §10 row added and closed (console lane, branch `fix/floor-test-budget-single-source`, the [ADR-0083 amendment](adr/0083-worktree-add-transience-classification.md)): cycles 1787, 1791, 1792 and 1798 failed after correct builds. The build floor ran a green `./cmd/evolve` (105–127 s) under a 120 s `-timeout` literal.
  - Every floor `go test` now runs under `addedtests.PackageTimeout`.
  - `TestGoTestTimeouts_AreTheSharedPackageBudget` refuses any other `-timeout` value in `go/internal` and `go/cmd`.
  - The runner file of the floor and the budget joined the protected surface.
  - Inbox `cmd-evolve-unit-wall-time-and-package-budget-check` carries the slow-package alarm.
  - Architecture review revision: `verifylock.MaxWait` bounds the wait of the coverage pass for the host verification lock. The ACS suite shares it (hold time for correctness, wait time for fleet fairness). The guard reads only go-test argvs. Inbox `verifylock-unreachable-from-sandboxed-pane` carries the sandbox gap of the lock.
- **2026-10-06.** §5.8 C8 and a correction: C3, C4 and C7 were not working live. No byte-identical carry had shipped (6 of 6 refused, 1766–1810). The diffs of the carry lived in the cycle worktree. The whole-ledger Verify of ship broke once the first one was cleaned up (ADR-0105 F5).
  - C8 is ADR-0123 Phase 1:
    - the content-addressed evidence store
    - store-backed composition verdicts
    - `evolve ledger evidence restore`
    - rungs 0 and 2 through the project ledger
    - a loud, race-free carry check
  - [Incident](../incidents/2026-10-06-carry-refused-ledger-verify-f5.md).
- **2026-10-07.** §5.8 C9: the carry did not compose with the inbox consumption of the re-ship. Cycle 1825 carried correctly (record `fcfb917`, exactly the 12 paths of the peer from the audited `92e7ba18`). It re-consumed its inbox (`9a96ef`) and was refused at the pre-commit check. C4 accepted only the exact tree of the record, and the consumption rule measured from the audited tree. The integrity class ended the cycle.
  - The carry now composes with the consumption in its one home.
  - The contradiction between B1 and B4 of ADR-0105 is resolved by a correction.
  - [Incident](../incidents/2026-10-07-cycle-1825-carry-refused-the-reships-consumption.md).
- **2026-10-18.** §7.9 A1 shipped (console lane, inbox `acs-run-names-vacuous`). `acsassert.GoTests` now refuses an exact pattern name that `Names` omits, because that name passed unseen.
  - 343 predicate files hold a `-run` flag, and only 14 call `GoTests`. So `go/acs/regression/acsnames` fails on a name that no test declares.
  - The first scan found 20 stale names in 16 packages. 9 predicates are repointed to the renamed tests, 1 is narrowed, and 14 are retired with a reason.
  - `TestC1678_011` parses the standard output alone.
- **2026-10-18.** §5.13 R2–R5 built (`sizeratchet-allowances-not-a-shared-registry`):
  - `Tighten` and `Encode`.
  - `evolve ratchet tighten [--check]` as the one writer.
  - The check of the floor against the base (`sizeFloorFailures`).
  - One path constant and one cap.
  - `treedelta.Identical` proves that the rebase of two shrink lanes in different packages is identical (`TestTighten_TwoLanesThatShrinkListedFunctionsInDifferentPackagesRebaseIdentically`).
  - R3 is built, not wired. The boundary commits nothing, and `sync-main` refuses a dirty tree, so `evolve boundary run` does not call the tighten. The console runs it by hand until that write can land.
  - The raw-git list keeps its exact-match rule until one tighten serves both lists.
  - Review round 1: the refusal does not name the writer, which refuses inside a phase or under a live lease (`refuseBoundaryWrite`). `moduleRelPrefix` uses `filepath.Rel`.
- **2026-10-20.** W5 shipped (console lane, inbox `inbox-item-identity-one-rule`). `inboxbatch.ItemID` is the one identity rule: the recorded id, else the filename stem, then the sanitizer of the loader.
  - `lifecycle.readID` and `readIDAndDeps` call it, and the backlog of the planner keeps an id-less item.
  - `TestInboxReaders_GiveEachItemOneID` pins the lane menu, the backlog and the lifecycle to one id, also for an id that the sanitizer rewrites.
  - `Item.IDFromFileName` is deleted.
  - Five more readers from an independent review call `ItemID` now, and an AST guard in `internal/guards` stops a new copy.
- **2026-10-20.** §5.15 X2 built (console lane, inbox `explanation-correction-is-a-rung`). The `NEEDS_CORRECTION` of a passing audit runs one document-only Build correction and a re-review before ship. The `explanation-correction` failure category sets the budget, and the dossier records each round.
- **2026-10-21.** §7.9 A2 backstop (console lane, inbox `superseded-attempt-acs-retention`). The archive at adoption covers one path only. Thus the landing check of a cycle ship reads the files that a lane adds under `go/acs/`.
  - A file under `go/acs/cycle<M>/` of another cycle stops the ship with `FOREIGN_PREDICATE_PACKAGE`.
  - `go/acs/cycle1707` and `go/acs/cycle1708` move to `docs/private/research/archived-2026-09-26/superseded-predicate-packages/`, beside their explanation records.
  - Their nine predicates stay in `go/acs/cycle1712`.
- **2026-10-21.** §7.5 Q5 shipped (console lane, inbox `codex-model-unsupported-bench`).
  - A `model_unsupported` escalation benches only the model of its tier, scoped `<family>@<model group>`, for `MaxCooldown`.
  - The bench line names the operator action.
  - A re-pin clears the bench, because `scopeCoversSeat` drops it for another model.
  - `llmroute.TieringExit` maps an `unknown_prompt` exit 85 to `ExitUnknownPromptNotQuota`. That exit still falls back to the next CLI in both walks. It never steps down the tier or surfaces a wall.
- **2026-10-21.** §5.8 outcome: RUNG 0 retired (console lane, inbox `composition-snapshot-reads-the-worktree-base`). `compositionCarryForward` and `tryTrivialRebaseCarryForward` of ship are removed. A legacy rebase tries RUNG 2, then goes to Audit with a stated reason.
