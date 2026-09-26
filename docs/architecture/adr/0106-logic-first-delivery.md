# ADR-0106: Logic-first delivery — phases own the logic, the pipeline owns the form

- **Status:** Proposed, 2026-09-26. The operator's direction, P0: "I want each phase to focus on LOGIC, if the failed reason is related to 'format' or process related errors, the pipeline should try to recover it, not blocking." And: "Focus on the logic, other format related errors can be fixed through recovery agent to help structure the deliverables around the core logics." And the goal as set: "prioritize the logic in code not the format and process, the pipeline should assist and check and recover it to its original code intention, the delivered code and doc should speaks for itself, if agent made error by following the format / structure, we should assign it back or recover if it delivers enough evidence for building / fulfill the request."
- **Amends:**
  - [ADR-0044](0044-unified-phase-recovery-protocol.md), whose correction ladder gains a rung and is shared with resume;
  - [ADR-0100](0100-declared-deliverables-gate.md), whose host step gains a derivation, whose gate stays the judge of form, and whose gate message stays the correction directive (§3, §9) except that an assign-back names the missing evidence;
  - [operating-policy](../../operations/operating-policy.md) §0, which states the rule this ADR implements.
- **Builds on:** [ADR-0105](0105-identity-preserving-fleet-rebase.md) (the process ladder at ship), [ADR-0072](0072-system-failure-policy-and-halt.md) (the floor this ADR never weakens).
- **Review:** architecture review 2026-09-26, APPROVE-WITH-CHANGES; every finding is folded in below and listed in the record.

## Context

A cycle exists to deliver logic: a fix, a feature, an architecture that survives the next change request. Its deliverables also have a form: reports with required sections, JSON secondaries, artifacts at contracted paths, a binding to the tree it was audited on. Today a defect in the form costs as much as a defect in the logic, and often more.

Evidence, cycles ~1550–1707 (inventory in the record below):

| Class | Nature | Cycles lost | Recovery today |
|---|---|---|---|
| A peer's landing forces Build and Audit to re-run on a byte-identical change (`GIT_FLEET_REBASE_NEEDED`, `AUDIT_BINDING_HEAD_MOVED`) | process | 14 | ADR-0105 B2 merged, B1 in review, B3/B4 designed |
| `missing_effect` (inbox claim) | form | 6 | fixed (ADR-0100 F36, host performs the claim) |
| `missing_secondary` `triage-decision.json` | form | 4, still recurring (1697, 1707) | re-dispatch the whole phase |
| Bridge `submit_wedged` / "unknown prompt" exits | process | 6 + 8 | fresh session for some causes; a codex `rate_limit` is mislabeled unknown-prompt (1706, 1707) |
| Loop halt from a form or process root cause (env leak 1700, stray file 1705) | process | 2, each stops the loop | halt + P0 item |
| A phase agent refused its own task, taking itself for an intruder (1707 TDD, ~1 hour) | process | live | none; the operator answered it |

No `missing_section` or `bad_verdict` rejection is recorded in the range: the form recoveries that exist (the host claim, the gate's own `bad_verdict` salvage) already cover the shapes that occurred. Three structural facts remain.

1. **A form defect the gate cannot salvage re-runs the whole phase.** The correction ladder (`core/cyclerun_correction.go`) decides salvage → live-fix → re-dispatch. Salvage executes only when `recovery.phase_recovery` is `enforce`, and it is `shadow` by default, so production logs "would-salvage" and moves on. Live-fix is dormant (`NamedREPL` is hard-false). What remains is a full re-dispatch with a `## Correction` note: a fresh agent redoes logic that was already done in order to fix a file's shape, and may change the logic while doing it. Resume runs a second, separate ladder (`core/resume.go`) with no rungs at all.
2. **Nothing restructures a deliverable around logic that already exists.** Every LLM helper that could is scoped elsewhere: the failure advisor classifies dead panes, the retry adjudicator narrows audit-FAIL retries (and is not composed in production: `WithRetryAdjudicator` has no caller in `cmd/`).
3. **Process failures have no shared vocabulary.** A `rate_limit` escalation exits as unknown-prompt; a confused agent's refusal is handled as a stall; a sibling's closeout commit forces a re-audit. Each is handled, or not, at its own site.

## Decision

Every block is one of three kinds; the first and the third are final.

- **Logic.** A defect in the change's behaviour: a failing or missing test, a regression, a security finding, an audit finding on substance. It blocks.
- **Form or process.** The deliverable's shape, a derivable file, a binding to a tree that moved, a race, a transport hiccup, bookkeeping. It goes to recovery, and blocks only when recovery is exhausted.
- **Integrity.** Evidence that the kernel's account of the cycle can no longer be trusted: the treefence, the predicate-authority fence, ADR-0072 incoherence, a sandbox violation, a recovery-guard violation. It is never recovered: the cycle aborts and a P0 is filed.

Recovery is layered from cheapest to dearest:

1. **Host derivations, before any judge.** The host step that already performs the inbox claim (ADR-0100 F36, `core/host_effects.go`) also derives a deliverable the phase's own primary artifact fully determines, so the runner and the gate judge a complete deliverable and no rung or verdict refresh is needed. `triage-decision.json` from `triage-report.md` is the first. A derivation runs only when every field it needs parses from the primary; it declares its origin in the file (`"derived_from"`); and the registry (`phase-registry.json`), not Go, declares which owed file is derivable.
2. **Deterministic rungs in the ladder.** Relocate a misplaced artifact (salvage), re-bind or carry a verdict across a tree change that provably did not touch the change (ADR-0105), retry an operation that lost a race.
3. **The recovery agent.** An LLM helper that restructures a phase's deliverable around the logic the phase already produced: it reads the phase's own output and the host's logs and writes the violated deliverable in the contracted shape. It never edits code or tests, never decides a verdict, never authors a report that did not exist.
4. **The phase itself**, re-dispatched with the violation as a correction, as today.

### The evidence decides between assigning back and recovering

The deliverables that speak for themselves are the change itself: the code, its tests and its explanation document (ADR-0102). Reports are projections of that evidence into a contracted shape. A defect in a projection is form; a defect in the code, the tests or the explanation's reasoning is logic. So when an agent erred in a projection, the question is whether the evidence behind it is enough to regenerate it, and the kernel answers that question. No agent self-assessment is an input: every input is computed by the kernel (git, the host's own test runs, a parse of a document's shape) or owned by it (the inbox item's acceptance, the worktree base). A report the agent wrote is never evidence for its own repair.

- **Which phases qualify is registry config.** A phase takes part only when `phase-registry.json` gives it a `recovery` block naming its evidence kind (`change`, `tests` or `document`) and its decision-bearing sections. A phase without the block, every judgment phase and every control phase is never evaluated and never recovered; `remediationDenied` stays as the floor beneath that.
- **Decision-bearing fields are never recovery-authored**, and their absence is always `Missing`: verdict lines, `## AC-Materialization`, the deliverable-kind and cycle-size headers, `top_n`, `deferred` and `dropped`. V0 adopts a verdict only when the verdict field is byte-identical before and after the rung.
- `evidence.Sufficient(in)` returns `Present` (what it found, with paths), `Missing` (what the phase owes and is absent) and `Sufficient` when `Missing` is empty. Per evidence kind:
  - **change, code kind** (build): the phase's own change is non-empty, meaning the tree at this dispatch differs from the tree now (a continuation's diff against its base carries its predecessor's work); the build floor approved this tree hash and actually ran; this cycle's predicates under `acs/cycleN` exist with a complete inventory, none red and none skipped, on the current tracked tree; their bytes equal the snapshot taken at the end of TDD, so Build did not weaken them; the explanation document is present whenever material paths exist; the protected-surface floor is clean. Whether the predicates cover the acceptance stays the auditor's judgment. Paths outside the triage footprint are listed in `Present` as a fact for the auditor, never a condition: the footprint is agent-declared, and every honest build touches tests and the explanation document beyond it.
  - **change, document kind** (ADR-0099): `solutioncheck` reports nothing for every committed id, and the diff stays inside the solution root and the explanation document. The kind is read before the rung, and a rung that changes the kind signals fails.
  - **tests** (tdd): the diff against the base touches only test files and `acs/cycleN`, and the host's own run of those predicates on the base tree fails each on an assertion, not a compile error. E0 captures that run at the end of TDD; a `## RED Run Output` section may quote only it.
  - **document** (scout, triage, build-planner and any phase so declared): the decision-bearing sections parse; no tests run.
- **Sufficient: recover toward the original intention.** The host derivation or the recovery agent regenerates the projection from `Present`, the task contract's acceptance and the intent, and V0 re-earns the verdict.
- **Insufficient, or the deliverable is missing: assign it back.** The phase is re-dispatched on its preserved worktree, keeping the diff and every green run, with a correction that names each entry of `Missing` and never a format code. A missing or empty primary is always assigned back: the pipeline cannot tell a finished phase that forgot its report from one that stopped early, and for a contracted phase the report is where the owner declares the outcome. Exhausting an assign-back is the task's logic FAIL, not a pipeline defect. E2 changes only the correction directive: the gate's reason with its codes, the block count and the breaker, the `GATE_CONTRACT_REJECTED` signal and the interaction payload stay as they are, an evalgate remediation is never replaced, and resume composes the same directive.
- **Cost and placement.** `Sufficient` is computed only for an all-form rejection of a qualifying phase, after the whole violation set is routed (a logic or integrity member sends the set back as a whole). The build floor's result is reused by tree hash; only `acs/cycleN` is run, bound with the tracked-tree check; an audit seal is reused only when its evidence verifies for the current tree; document phases run nothing.

### Routing by violation code

The route lives in `deliverable`, beside the codes, so the gate and the ladder cannot disagree.

| Code | Route |
|---|---|
| `missing_effect` | host step (as today) |
| a derivable owed file: `missing_secondary`, `empty_secondary`, `malformed_secondary` of `triage-decision.json` | host derivation; a derivation that declines routes to re-dispatch, never to the agent (a decision document is the phase's judgment) |
| `stray_in_worktree` | salvage |
| `missing_section`; `empty_secondary` / `malformed_secondary` of a non-decision file | evidence sufficient: recovery agent; insufficient: assign back naming `Missing` |
| `missing_artifact`, `empty_artifact` | assign back: the owner is re-dispatched on its preserved worktree with `Present` as the correction; the diff and every green run are kept |
| a floor rejection without a code (the explanation floor, the build floor) | once floors carry stable codes (F0), routed with the whole violation set; until then, re-dispatch as today |
| `bad_verdict` | re-dispatch (the gate's deterministic verdict salvage already ran) |
| `missing_challenge_token` | re-dispatch (proof of read is never supplied by a helper) |
| `failure_context_missing`, `failure_class_unknown` | re-dispatch (the class is a judgment that drives retries) |
| `unbound_effect` | pipeline defect (a registry declaration nothing binds) |

### The recovery agent

- **Identity.** A helper built like `FailureAdvisor`: bridge-dispatched, persona-injected (`agents/evolve-deliverable-recovery.md`), profile `.evolve/profiles/deliverable-recovery.json` with `sandbox.enabled`, so the launch is wrapped or refused. On a host where the wrapper cannot run, the rung declines. It is a helper, not a registry phase: it never appears in a cycle's phase order.
- **Input.** The phase, the gate's violations, the phase's contract (its primary artifact, required sections, owed secondaries), the original intention (the task contract's acceptance and footprint, the intent document), and the evidence verdict's `Present` list: the phase's own output, its partial artifacts, the change's diff against its base (read-only) and the host-run test results. `Present` is the only set of sources the agent may cite.
- **Output.** The violated deliverable, and its own report `deliverable-recovery.json`: `{"repaired":[{"path","blocks":[{"source","offset","length"}]}],"unrecoverable":[{"path","why"}]}`. Every block the agent added names the evidence file and byte range it came from.
- **Placement.** A rung in the correction ladder after salvage and the (dormant) live-fix, before re-dispatch: the phase's own agent, when it is preserved, repairs with its own context and no fabrication risk; the recovery agent comes next; a fresh phase run is last.

### Guards

Recovery never launders. These hold on every rung, and a rung that cannot satisfy one declines.

- **The kernel fences the whole run.** Before the rung, the guard hashes every file in the workspace and fences the worktree (`treefence`). After it, only the violated deliverable paths and the agent's report may differ; host-written telemetry the dispatch itself appends is unfenced by name. Anything else changed is restored, the rung is an integrity violation, and the cycle aborts with a P0. Restoring and then re-dispatching onto a touched tree is not safe.
- **Evidence, not claims.** The decision to recover rests on `evidence.Sufficient`, whose every input is computed or owned by the kernel; a report the agent wrote is never evidence for its own repair.
- **Recovery never touches the change.** The code, the tests and the explanation document live in the fenced worktree; the audit's explanation review is a judgment and is excluded. A rung that changes a decision-bearing field or the kind signals fails.
- **Provenance for every added line.** The kernel checks each block the report names against the pre-rung snapshot of the named evidence file, byte for byte. A `## RED Run Output` section may quote only a host-captured log. A block without provenance, or with provenance that does not match, fails the rung.
- **The verdict is re-earned, not carried.** For contracted phases the runner reads the verdict from the deliverable; a missing section is a FAIL. After an approved rung the kernel re-classifies the deliverable through the phase's own classifier (V0). It adopts the new verdict only when the primary existed and was not empty before the rung and every added block has provenance; otherwise the verdict stays and the ladder falls through. This also closes today's gap where salvage's approval never refreshed the verdict.
- **Breaker-neutral checks; one judgment.** A rung's output is checked with the breaker-neutral `contractVerifier`, as salvage's is. `reviewDeliverable`, which counts toward the gate's demotion breaker, runs once, only when that check passes. A failed rung leaves the breaker count unchanged.
- **Judgment is out of reach.** No rung runs for a phase without a registry `recovery` block, nor for one in `remediationDenied` (audit, retrospective, debugger, adversarial-review, premise-challenge, plan-review). An honest rejection stays rejected (operating-policy §4).
- **Provenance is loud, before and after audit.** Every rung records a coded signal and an interaction outcome; the recovered paths ride `CycleResult.Remediations`; the audit prompt lists them beside the Task Contract, and recovered text carries a marker inside the file, so the auditor sees the repair.
- **One ladder.** Resume calls the same ladder as the live path; a resumed cycle reaches every rung the live cycle does.

Recovery is bounded by `workflow.recovery_rounds` (compiled default 1 per ladder; its own key, distinct from `recovery.phase_recovery` and `workflow.remediation_rounds`). Exhausted recovery falls through to re-dispatch and, after that, is recorded as a system-level pipeline defect. It still counts toward the consecutive-fail and zero-ship halts (ADR-0072); only the recorded reason changes.

## Components, steps and tests

Each component is one commit with its own tests, ordered by dependency; unwired components land before their wiring. The order follows cycles lost: the process rungs and the host derivations first, then the evidence decision wired into the re-dispatch (E0 → E1 → E2 → F0 → F6a), which pays off without any agent, then V0, and the agent after form rejections are measured again.

| # | Component | Step | Test (each mutation-checked) |
|---|---|---|---|
| **Policy** ||||
| D0 | operating-policy §0 (three classes, the evidence rule) + this ADR | docs | review only |
| **Process (P)** ||||
| P1 | ADR-0105 B1 (#652), the resume heal, then B3/B4 | per ADR-0105 | per ADR-0105 |
| P2 | a fleet lane's closeout dossier waits for the wave boundary (branch `fix/dossier-commits-at-wave-boundary`) | per its incident | per its incident |
| P3 | phase prompts state the agent's identity verifiably; phase panes run without prompt suggestions | prompt + driver config | the composed prompt names the session and sole-writer fact; the driver launch disables suggestions |
| P4 | a `rate_limit` auto-respond escalation exits with its own class, never unknown-prompt | bridge | the 1707 transcript classifies as quota, routes to the fallback, and is counted as quota |
| **Host derivation (H)** ||||
| H1 | one reader of the triage report: `triagedecision.Derive` (strict: `## top_n` stated, every present bucket readable, pinned ids committed; absent optional buckets are `[]`) and `Project` (lenient, ship's companion), one stamp `projected_by_orchestrator`; `triagecap` delegates to it | pure, unwired | derives 1707's report and the persona's metadata tails; declines a missing `## top_n`, prose in a bucket, a non-slug id, a missing pinned id; Project never declines |
| H2 | registry `outputs.derived_from`; the host step runs H1 after the effects, before the runner's judge and the gate, waiting out a write in flight | wiring | a triage run without the file is judged complete; a file the agent wrote, or is landing, is never touched; a read fault declines; a malformed report still rejects |
| H3 | the `## Explanation Documentation` declaration derived by the host (`DocumentPath`, the diff SHA) | wiring | a build report without the declaration is completed from kernel facts |
| **Evidence (E)** ||||
| E0 | host RED capture at the end of TDD: the host runs the cycle's predicates on the base tree and records each failing assertion | unwired | a red-on-assertion run is recorded; a compile failure is recorded as such, never as red |
| E1 | `evidence.Sufficient` over an injected evidence struct; the registry `recovery` block names each phase's kind and decision sections | pure, unwired | per kind: a non-empty own change with complete, green, unweakened predicates is sufficient; an empty change, a red or skipped predicate, a weakened predicate and a missing explanation are each named in `Missing`; a document cycle needs a clean `solutioncheck`; a phase without the block is never evaluated |
| E2 | the assign-back correction names `Missing`, never a format code; only the directive changes | pure, unwired | the directive lists every missing entry and no code; the gate's reason, the block count and the signals are byte-identical with and without E2; resume composes the same directive |
| F0 | stable codes for floor rejections (the explanation floor, the build floor); routing on the whole violation set | wiring | a code-less floor rejection routes with its set; a set with a logic member goes back whole |
| F6a | E1 and E2 wired into the re-dispatch, before any agent rung | wiring | an insufficient all-form failure re-dispatches with `Missing`; a sufficient one proceeds to the ladder |
| **Verdict and ladder (V, F)** ||||
| V0 | verdict refresh: re-classify after an approved rung; adopt only under the guard rule | wiring | salvage's approval now routes PASS; a rung on an empty primary keeps FAIL |
| F1 | `interaction.RungRecover` in `NextCorrection`, after live-fix, before re-dispatch, gated on `Repairable` | pure, unwired | order table; each rung only with budget; a judgment phase never reaches recover; exhausted → abort |
| F2 | `recoveryguard`: hash the workspace, fence the worktree, allow only the named paths, restore and report the rest; provenance check of a report's blocks | pure, unwired | a forged token, a rewritten sibling report, a planted verdict, a source edit are each restored and reported; an in-grant write passes; a block whose bytes differ from its source fails; host logs are unfenced |
| F3 | recovery-agent profile (`sandbox.enabled`, no network), persona, phase config | config, unwired | the profile requires the sandbox and no network; no grant reaches the repository beyond the run dir or any worktree |
| F3b | a `{cycle}` write-grant template in the sandbox resolver; the recovery profile grants only its own cycle's run dir | trust kernel | a sibling lane's run dir is not writable |
| F4 | `bridgeDeliverableRecoverer` (dispatch, prompt, strict report parse) | unwired | fake bridge: the prompt names the contract, violations and evidence; a malformed or missing report is a declined rung, never an error that blocks |
| F5 | `workflow.recovery_rounds`: policy.json → policy → config → orchestrator option | config plumbing | the compiled default is 1; the key is read; an absent block keeps the default |
| F6 | wire the ladder: execute recover, guard, breaker-neutral check, one review, V0; an integrity violation aborts with a P0; signal codes registered | wiring | fake runners: a report missing a section is repaired and approved without re-dispatch; a judgment phase never reaches the rung; a guard violation aborts the cycle and files the item; a failed rung leaves the breaker count unchanged |
| F6b | one ladder Strategy for the live path and resume | wiring | a resumed cycle reaches the recover rung |
| F7 | salvage executes at the default config: no cwd candidate in fleet mode, git-tracked candidates skipped; depends on V0 | wiring | a sibling lane's same-named report is never taken; a tracked file is never moved; the interaction ledger's would-act counts are cited in the PR |
| F8 | composition root wires the recoverer, with a wiring-proof test | wiring | `…Wired()` proof |
| **Ledger of outcomes (L)** ||||
| L1 | recovered paths and the evidence verdict in `CycleResult.Remediations`, the dossier, the audit prompt, and an in-file marker | wiring | a recovered cycle's dossier and audit prompt list the rung, the paths and the evidence present |
| L2 | recovery-rung exhaustion recorded as a system-level pipeline defect; breakers unchanged; an exhausted assign-back stays the task's logic FAIL | wiring | the consecutive-fail count still advances; the filed item names the exhausted rung; an exhausted assign-back files nothing |

## Consequences

- A cycle whose logic is right and whose report is malformed is repaired in one bounded, cheap dispatch instead of a full phase re-run; a derivable secondary never reaches a gate at all.
- The gate's standard does not move: it accepts exactly what it accepted before; recovery changes who writes the file, never what passes.
- An all-form rejection of a qualifying phase costs the host's run of this cycle's predicates (`acs/cycleN` only, bound to the tracked tree; the build floor's result is reused by tree hash; document phases run nothing), so the recover-or-assign-back decision is made on the same evidence audit uses; the cost is bounded by `recovery_rounds`.
- The recovery agent is a new LLM call on the failure path, bounded by `recovery_rounds`, and it replaces a whole-phase re-dispatch that costs more. It is built after H1/H2 land and form rejections are measured again, so it targets a class that still occurs.
- Salvage (F7) begins to execute in production once V0 refreshes its verdict and its fleet-mode candidates are fixed.
- Integrity blocks gain a name and a rule: never recovered. A recovery-guard violation is the first new member.
- Not in scope: recovering an audit FAIL on narrative fidelity (a build report that misdescribes the diff). The auditor's rejection is a judgment and stays one; if evidence later shows the class is large, a separate ADR decides it.

## Record

- Inventory of block points and cycles lost, 1550–1707: gathered 2026-09-26 for this ADR (G1–G30; summarised in Context).
- Architecture review round 2, 2026-09-26 (APPROVE-WITH-CHANGES, the evidence decision): a missing report is always assigned back; a green predicate suite is vacuous without this cycle's own, unweakened, red-on-base predicates; the footprint is agent-declared and not a condition; document-kind cycles have their own evidence; floor rejections carry no code; the triage projector already existed and H1 is its strict mode; E2 may change only the directive; the qualifying phases are registry config.
- Architecture review round 1, 2026-09-26 (APPROVE-WITH-CHANGES): a helper without a worktree runs unsandboxed unless its profile enables one; the phase verdict is read from the deliverable, so a verdict-equality guard is unsound; a run-dir grant reaches proof-of-read and the lane pin; re-derivation belongs before the judges; resume has a separate ladder; extra re-reviews demote the gate; exhaustion must not weaken the ADR-0072 breakers; integrity blocks need their own class; the auditor must see repairs; fleet rebase outranks the agent in cycles lost.
- [incident 2026-09-26, a passed audit sent back by ship process issues](../../incidents/2026-09-26-a-passed-audit-sent-back-by-ship-process-issues.md).
- Cycle 1707 (2026-09-26): the TDD agent read its own tmux session and prompt file as a second writer and refused the phase for an hour; a factual answer with a self-check (`tmux display-message -p '#S'`) resumed it at once.
