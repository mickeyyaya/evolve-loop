# Logic-first delivery — design document

- **Status:** living document, kept current with every landing. Last updated 2026-09-26 22:20.
- **Decision record:** [ADR-0106](adr/0106-logic-first-delivery.md). **Policy:** [operating-policy §0](../operations/operating-policy.md).
- **Landings:** design PR #653 (`docs/logic-first-delivery`); code train PR #654 (`feat/recovery-rungs`, six commits); ADR-0105 B1 in PR #652.
- **Owner of the request:** the operator. **Priority:** P0; everything else parks.

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
| D0 | operating-policy §0 + ADR-0106 + this document | shipped, #653 | `docs/` |
| P1 | ADR-0105 B1 unwind-rebase-pend; then the resume heal, B3, B4 | B2 merged (#649); B1 shipped, #652 (CI green, merges at a wave boundary) | `core/ship_recovery*.go` |
| P2 | a fleet lane's closeout dossier waits for the wave boundary | built, parked (`fix/dossier-commits-at-wave-boundary`; architect N1–N5 applied) | `dossier/publish_pending.go`, `cmd_loop_dossiers.go` |
| P3 | phase prompts state the agent's identity verifiably; phase panes run without prompt suggestions | designed | bridge prompt composition, driver launch flags |
| P4 | an exit-85 escalation names its pattern in `cause_code` and the cause line, anchored to the line start | shipped `4aabd0fa`, #654 | `bridge/launchoutcome/cause.go` |

### 7.2 Host derivations (H)

| Id | Component | Status | Where |
|---|---|---|---|
| H1 | one triage-report reader: `Derive` (strict) and `Project` (lenient); `triagecap` delegates; one stamp `projected_by_orchestrator`; protected surface | shipped `476f821e`, #654 | `internal/triagedecision` |
| H2 | registry `outputs.derived_from`; the host derives an absent or empty declared secondary after the effects, before the judges, waiting out a write in flight | shipped `cfecc540`, #654 | `deliverable/host_effects.go`, `phasespec`, `phasecontract`, `phase-registry.json` |
| H3 | the `## Explanation Documentation` declaration derived by the host | designed | `deliverable/host_effects.go`, `explanationdocs` |

### 7.3 Evidence (E) and floors (F0, F6a)

| Id | Component | Status | Where |
|---|---|---|---|
| E0 | host RED capture at the end of TDD | designed | `phases/tdd`, a host runner of `acs/cycleN` on the base tree |
| E1 | `evidence.Sufficient` over an injected evidence struct; registry `recovery` block | designed | `internal/evidence` (new leaf) |
| E2 | the assign-back correction names `Missing`; only the directive changes | designed | `core/retry_backoff.go` (`composeCorrection`), `core/resume.go` |
| F0 | stable codes for floor rejections; routing on the whole violation set | designed | `explanationdocs`, `core/build_floor_reviewer.go`, `deliverable` |
| F6a | E1 and E2 wired into the re-dispatch, before any agent rung | designed | `core/cyclerun_correction.go` |

### 7.4 Verdict and ladder (V, F)

| Id | Component | Status | Where |
|---|---|---|---|
| V0 | verdict refresh after an approved rung, adopted only under the guard rule | designed | `core/cyclerun_correction.go`, the phase classifiers |
| F1 | `interaction.RungRecover` after live-fix, gated on `Repairable` | shipped `107c0f77`, #654 | `internal/interaction/correction.go` |
| F2 | `recoveryguard`: whole-workspace fence + treefence; `Scope{Worktree, Workspace, Allowed, Unfenced, UnfencedStems}`; protected surface | shipped `4a998ac3`, #654 | `internal/recoveryguard` |
| F2b | provenance check: every block the agent's report names is byte-equal to its source in the pre-rung snapshot | designed | `internal/recoveryguard` |
| F3 | recovery-agent profile (codex family by default with claude as fallback, per the balanced-tier floor; sandbox on, read-only repo, run-dir grant, no network declared) and persona | shipped `ea58774b`, #654; routing corrected by the follow-up commit | `.evolve/profiles/deliverable-recovery.json`, `agents/evolve-deliverable-recovery.md` |
| F3b | a `{cycle}` write-grant template so a profile grants only its own cycle's run dir | designed | `bridge/sandbox_paths.go` |
| F4 | `bridgeDeliverableRecoverer`: dispatch, prompt, strict report parse | designed | `core/` beside `failure_advisor.go` |
| F5 | `workflow.recovery_rounds`: policy.json → policy → config → orchestrator option | designed | `internal/policy`, `internal/config`, `core/orchestrator.go` |
| F6 | wire the ladder: recover, guard, breaker-neutral check, one review, V0; an integrity violation aborts with a P0; signal codes | designed | `core/cyclerun_correction.go` |
| F6b | one ladder Strategy for the live path and resume | designed | `core/resume.go` |
| F7 | salvage executes at the default config (no cwd candidate in fleet mode; tracked candidates skipped); depends on V0 | designed | `core/correction_ladder.go` |
| F8 | composition root wires the recoverer, with a wiring-proof test | designed | `cmd/evolve/cmd_cycle.go` |

### 7.5 Ledger of outcomes (L)

| Id | Component | Status | Where |
|---|---|---|---|
| L1 | recovered paths and the evidence verdict in `CycleResult.Remediations`, the dossier, the audit prompt, and an in-file marker | designed | `core/`, `dossier/`, the audit prompt |
| L2 | recovery-rung exhaustion recorded as a system-level pipeline defect; breakers unchanged; an exhausted assign-back stays the task's logic FAIL | designed | `core/blocker_breaker.go`, the retro paths |

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

// P4 — internal/bridge/launchoutcome (shipped)
// exit 85: cause_code = the escalation pattern (rate_limit, model_unsupported, …) or unknown_prompt

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

No `missing_section` or `bad_verdict` rejection is recorded in the range, so the recovery agent (F4/F6) lands last, after H1/H2 and the evidence decision are measured again.

Cost: an all-form rejection of a qualifying phase costs the host's run of this cycle's predicates (`acs/cycleN` only, bound to the tracked tree; the build floor's result is reused by tree hash; an audit seal is reused only when its evidence verifies for the current tree; document phases run nothing), bounded by `workflow.recovery_rounds`. The recovery agent is one bounded LLM call that replaces a whole-phase re-dispatch.

## 11. Rollout

Landing order and the checkpoint each must pass before the next starts.

| Step | Lands | Checkpoint |
|---|---|---|
| 1 | D0 (#653), P1 (#652), the code train (#654) | merged at a wave boundary in one burst; the plane synced; the full floor green on each |
| 2 | soak one wave | a triage lane that omits `triage-decision.json` proceeds without `GATE_CONTRACT_REJECTED [missing_secondary]`; a codex `rate_limit` escalation shows `cause_code=rate_limit`; the first fleet-rebase recovery logs "unwound its ship commit" |
| 3 | P2 (dossier at the boundary), P3 (identity prompt, no suggestions) | a FAIL sibling's closeout no longer moves `main` under a passed lane; no agent refusal on identity |
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

## 13. Review log

| Date | Review | Verdict | What changed |
|---|---|---|---|
| 2026-09-26 | architect, round 1 (design) | APPROVE-WITH-CHANGES | sandbox claim corrected; V0 verdict refresh; whole-workspace fence; breaker-neutral checks; H1/H2 into the host step; F6b shared ladder; L2 relabel only; integrity class; auditor visibility; landing order by cycles lost |
| 2026-09-26 | architect, round 2 (the evidence decision) | APPROVE-WITH-CHANGES | a missing report is always assigned back; per-kind evidence with this cycle's own unweakened red-on-base predicates; footprint a fact, not a condition; document-kind evidence; floor codes (F0); H1 as the strict mode of the existing projector; E2 changes only the directive; registry allowlist |
| 2026-09-26 | docs review | WARNING → APPROVE | policy renumbered as §0; §4 names control phases; re-check after both revisions approved |
| 2026-09-26 | go-reviewer (train) | APPROVE-WITH-MINOR | four minors applied |
| 2026-09-26 | code-reviewer (train) | WARNING | the derivation's write-in-flight grace (MAJOR) applied; the duplicate projector absorbed |
| 2026-09-26 | security-reviewer (train) | BLOCK → APPROVE-WITH-MINOR | unfenced boundary; allowed-path type check; no network declared; anchored markers; three gaps filed |

## 14. Document history

| Date | Change |
|---|---|
| 2026-09-26 | Created after the design landed in ADR-0106 and the first six components shipped (#654). |
| 2026-09-26 | Review (APPROVE-WITH-MINOR): the exit-85 census corrected (the eight unknown-prompt exits were four `rate_limit` and four `model_unsupported`); the audit-seal reuse clause restored; F2b cross-referenced from the ADR. |
| 2026-09-26 | F3 routed to the codex family with claude as fallback: the balanced-tier floor (`TestClaudeFamilyFloor`) reserves claude for judgment phases with a justification, and the recovery agent is an analytic helper. |
