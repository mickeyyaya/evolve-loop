# ADR-0039: Failure Floor + Advisor Failure-Path Vocabulary + Failure-Signal Contract

- Status: Accepted
- Date: 2026-06-07
- Extends: ADR-0024 (integrity floor), ADR-0033 (verdict single source), ADR-0034/0035 (deliverable contracts), ADR-0038 (phase plugins)

## Context

Learning from failure was best-effort: an LLM retrospective ran on FAIL/WARN verdicts *when
nothing else was broken*. Three campaign incidents showed the gaps were exactly where learning
mattered most:

1. **Cycle 243** — the retro bridge died mid-phase (orchestrator degradation branch): the cycle
   ended with NO retrospective artifact and NO lesson. A dead phase cannot self-report.
2. **Cycle 244** — `evolve cycle reset` sealed the run directory without extracting anything:
   operator resets learned nothing (the cycle-245 reset then also ate an in-progress fix).
3. **Loop fatals** — `cmd_loop` abnormal exits (batch cap, circuit breaker, verify-failed stop,
   integrity breach, resume failure) recorded no `failedApproaches` entry; the next session
   started blind.

Separately, the failure-learning *configuration* was scattered (env enable-chain
`EVOLVE_DISABLE_AUTO_RETROSPECTIVE`, registry `enable_var_inverted`, in-phase `req.Env` check)
and the advisor had no vocabulary to make failure-path decisions (retry vs end; full retro vs
memo) — the kernel decided alone, with no forensic trail of what an advisor would have chosen.

## Decision

### 1. Deterministic failure floor — the integrity-floor mirror

The integrity floor guarantees `ship ⇒ build ∧ audit` no matter what an advisor proposes. The
**failure floor** is its mirror on the failure branch: **every abnormal termination produces
learning artifacts no matter what is broken**, deterministically, with the LLM layer as
enrichment on top:

- `go/internal/faillearn` (leaf, stdlib-only): `FailureEvent` →
  `RenderRetrospectiveMarkdown`/`RenderLessonYAML` (golden-byte pinned).
- Orchestrator degradation branches call `writeDeterministicLearning` when the retro errors or
  returns a non-canonical verdict (closes cycle-243).
- `SealCycle` writes artifacts into the sealed archive AND records `failurelog.Record(OperatorReset)`
  BEFORE seal's state read — ordering is load-bearing and pinned (closes cycle-244).
- `cmd_loop.emitFatal` records `LoopFatal` with a `stop_reason=` summary at every abnormal-exit
  site (closes the loop-fatal gap; the exclusion list is documented in commit `532df7a`).
- `failurelog` gained `OperatorReset`/`LoopFatal` classifications, a `Summary` override, and a
  monotonic `lastCycleNumber` so cycle-0 records cannot regress the counter.

**SIGKILL of the orchestrator itself** is closed *downstream*, not by a signal handler: an
orphaned cycle forces reset/resume on the next invocation, and both paths now learn (floor at
seal/reset + resume). A supervisor synthesizes what a dead process cannot self-report.

### 2. Advisor failure-path vocabulary (above the floor, never instead of it)

`Proposal` += `LearningRichness ("full"|"memo")` and `RecoveryAction ("retry"|"end")`;
`retroDecision`/`applyFailureProposal` adopt advisor choices ONLY where the failure-adapter
permits — BLOCK is non-overridable, every clamp is recorded (`failure-proposal-clamped`), and a
memo choice can pick *which* learning phase runs but never *none*. Retry may insert
fault-localization / bug-reproduction (the `failureInsertPhases` kernel map) ahead of tdd.
Happy-path prompts stay byte-identical (prompt-prefix cache); the failure vocabulary renders only
at failure transitions.

**R5 (standing decision):** "retry on a fallback CLI" is already satisfied by the runner's chain
walk (`runner.go:398-438` — exit codes 80/81/124/127 advance CLIs inside ONE `Run` call). An
orchestrator-level CLI switch would invent API the runner already owns. Deterministic artifacts
ARE the fallback when every CLI fails.

**R7 (standing decision):** there are two `failedApproaches` appenders (orchestrator + cmd_loop).
Unification is deferred; `failedrecord_shape_test.go` pins shape parity (Recorded keys ⊆
FailedRecord) so they cannot drift apart silently.

### 3. One user surface: `policy.json:failure_floor` (Phase 4a)

```json
{ "failure_floor": { "always_learn": true, "audit_fail_routes_to": "retrospective" } }
```

- Closed vocabulary {`retrospective`, `memo`}; unknown values fall back to the default (the floor
  guarantees SOME learning phase routes).
- The composition root folds policy → `cfg.AuditFailRoutesTo`; router Rule 1 honors it AHEAD of
  the deprecated enable-chain. Empty ⇒ legacy behavior for one more release.
- `always_learn=false` downgrades only the DEFAULT route; an explicitly written
  `audit_fail_routes_to:"retrospective"` wins (explicit beats derived — `FailurePolicy()` launders
  defaults, so the fold checks the raw field).
- The deterministic floor (§1) is **non-configurable** — like the integrity floor, policy tunes
  only the LLM layer.

### 4. Rubric as a projection (Phase 4b — never-duplicate)

The advisor's decision rubric is rendered by ONE renderer (`writeRubricLines`) as a projection of
the structured routing data the kernel already walks: `insert_when` triggers (derived
`field op value → insert <phase>` lines), `conditional_mandatory` rules (ops negated into skip
exemptions), and `router.FailureInsertPhases()` (failure vocabulary). Registry
`routing.rubric_hint` carries ONLY judgment guidance with no structured counterpart, each line on
exactly one card. A threshold can never disagree between the walk and the prompt. The FORBIDDEN
ship-without-audit line stays in Go — kernel invariant, not phase data.

### 5. Defense-in-depth twins (Phase 4c deviation)

`router.EvaluatorFloorPhase` and policy's unexported `evaluatorFloorPhase` remain twins: the
reverse import would cycle (`router/policy.go` imports `policy`), and each layer independently
guaranteeing the evaluator is deliberate. The never-duplicate rule is satisfied by a tripwire —
`TestEvaluatorFloorPhase_SingleSource` — divergence is loud, not silent. This is the sanctioned
pattern for unavoidable twins.

### 6. Migration (Phase 5) and archaeology (Phase 4d)

- `EVOLVE_DISABLE_AUTO_RETROSPECTIVE` is deprecated: honored one more release, `config.Load`
  WARNs `deprecated-flag` with migration guidance whenever it is set; `failure_floor` wins when
  both are set (structurally true — the policy route bypasses `enableOf`; pinned by
  `TestAuditFail_RoutesPerFailurePolicyNotEnableVar`). Net flags: −1 next release.
- `.evolve/llm_config.json` (untracked runtime file, live tree) carries a `_deprecated` note: no
  runtime reader since Step 9 (`resolvellm` resolves from profiles only — see
  `TestResolve_IgnoresLLMConfig`); kept for archaeology.

### 7. Generalized failure-signal contract (Phase 6 design)

Failure signals are unified at CONVERGENCE (one FailureEvent/renderer/appender/floor) but were
heterogeneous at ORIGINATION (8 detection sites building events from thin summary strings). The
fix makes self-describable failures CONTRACTUAL while keeping crash-class failures
supervisor-synthesized (cycle 243 proved a dead phase can't self-report; the floor is the
contract's fallback):

- **Carrier**: the verdict sentinel (ADR-0033) extends to `schema_version: 2` with an optional
  `failure` block — `{"class", "defects": [...], "evidence_paths": [...]}` — one contracted
  artifact, one parser, v1-compatible forever (absent block legal for PASS and old artifacts).
- **Selection is tail-anchored** (cycle-1299): a report may contain several sentinel-shaped
  substrings — prose routinely QUOTES the syntax (contract examples, review commentary) — so
  `ParseVerdictSentinelFull` walks candidates from the END and returns the LAST one that
  unmarshals, carries a non-empty `verdict`, and is not a placeholder echo (cycle-603).
  Invalid candidates are SKIPPED, never fatal. Rationale: a producer emits its real verdict at
  the tail, after the prose. First-match selection let a quoted decoy win, and an elided decoy
  (`{"verdict":…}`) blanked the read entirely — cycle-1298's adversarial-review report carried
  five quoted decoys ahead of a well-formed `verdict=FAIL`, read as "no sentinel", fired
  `[bad_verdict]` ×3 and circuit-opened the contract gate enforce→advisory. Single-sentinel
  documents (the common case) are unaffected.
- **One parser, no exceptions** (cycle-1303): every consumer of the sentinel reads through
  `phasecontract.ParseVerdictSentinelFull`. The release gate was the last holdout — it hand-rolled
  a second scanner (`machineVerdictRE`/`markerVerdict`) that was tail-anchored but had no
  placeholder-echo guard, so a Deliverable-Contract example captured from scrollback stayed
  authoritative to `evolve release` long after cycle-603 closed that class everywhere else. It now
  delegates (`internal/releasepreflight/releasepreflight.go:588`); what stays local is the release
  gate's own POLICY — normalise the verdict, accept only PASS/WARN/FAIL, otherwise fall through to
  the prose scan. Rule: duplicate the mapping if you must, never the parse.
- **Contract conditionality**: contracts with `RequireFailureContext` make a missing/empty
  failure block on FAIL/WARN a Violation (`failure-context-missing`) → the existing
  correction-retry machinery re-dispatches with the exact reason.
- **Digest lifting**: `<phase>.failure_class` / `<phase>.defect_count` become objective signals
  (generic plane), so failure-phase insertion is DATA-driven on the walk; the Phase-3
  `failureInsertPhases` map remains ONLY for the retro-branch retry path (different mechanism).
- **faillearn consumption**: structured defects/evidence flow into lessons (supervisor synthesis
  stays the fallback); `cycleclassify` gains a Pass-0 sentinel read mapped via
  `failurelog.NormalizeLegacy` (unknown classes fall through to regex passes).

### 8. Self-healing ship repair ladder (Phase 7 — executor-side complement)

The retrospectives for cycles 230 and 243–248 converged on one verdict: *every ship
enforcement point is a cycle-killer; none has a correction tier* — audited-PASS work was
repeatedly hand-salvaged (stale TOFU pin ×2, merged-but-unpushed main ahead-1, colliders
looping audit↔ship to depth exhaustion, worktrees pruned with PASS work inside). The repair
ladder (`go/internal/phases/ship/repair.go`) is the executor-side complement of the failure
floor: ship finds the *legitimate* way to land the audited tree before surfacing an error.

Rules (operator-approved 2026-06-07):

- **Bounded**: each `ShipError` code gets at most ONE typed repair per Run
  (`opts.repairAttempted` once-guard); the orchestrator's `maxRecoveryDepth` bounds the outer
  loop independently. No unbounded recovery anywhere.
- **Provably safe**: every repair re-runs the violated invariant afterwards (the failed stage
  re-executes, or the closure re-verifies the tree binding). A repair that cannot prove safety
  declines and the original error stands.
- **Policy floor untouched**: never rebase, never force-push, never set bypass env vars, never
  delete content. Integrity class still defaults to BLOCK — only the provably-safe sub-cases
  below proceed.

| Code | Signature healed | Repair | Re-verify |
|---|---|---|---|
| `SELF_SHA_TAMPERED` | stale TOFU pin: running binary SHA == blob at `HEAD:<bin>` (legit rebuild/manual-ship of committed source; cycles 246-248) | re-pin `expected_ship_sha` | `verifySelfSHA` re-runs |
| `AUDIT_BINDING_HEAD_MOVED` | merged-but-unpushed, left by a ship from before the two-phase landing (8.1), which moved `main` before its push (cycle 246). The rung needs three facts. HEAD's tree satisfies the audit binding (`auditBindingSatisfied`). The audited base is an ancestor of HEAD. Origin is an ancestor of HEAD. | The rung builds a landing intent from these facts. It has no witness of its own, so only the ancestry checks apply: it calls `Landing.Admit`, not `Landing.Resume`. Then it runs the shared steps of 8.1, `pushLanding` and `settleLanding`. When origin already holds the commit, the rung settles it with no push. | the ancestry checks, then the post-push tree check and `ship-binding.json` |
| `GIT_FF_MERGE_DIVERGED` (collider variant) | untracked main-side colliders (cycle 230) | byte-identical → remove; differing → quarantine-move to `.evolve/quarantine/cycle-<N>/` + `manifest.json` (never deleted) | atomic-ship stage re-runs incl. collider pre-flight |
| `GIT_PUSH_REJECTED` | Ship classifies the stderr of the push: a policy refusal, a transport error or a 5xx, or another rejection (a race). | Policy: no retry. The code is `GIT_PUSH_POLICY_REFUSED`, a final precondition, and the router ends the cycle. Transport: the identical push again after the bounded backoff (`transportBackoff` in `landing/push.go`), in the step. Race: one fetch and one ff retry when origin is an ancestor of the pushed commit; a diverged origin becomes a precondition with `repair_outcome=needs-reaudit`. At the worktree site, a precondition unwinds the lane first, and a transient failure keeps the intent `prepared` for the resume (8.1). | post-push verification on the healed path |

Routing fix (`router/recovery.go`): ship-LOCAL preconditions a re-audit cannot re-establish
(`GIT_FF_MERGE_DIVERGED`, `COMMIT_PREFIX_GATE`, `GIT_DETACHED_HEAD`, `WORKTREE_RESOLVE`) now
route to the **debugger** phase, not audit — the in-Run ladder already declined by the time the
router sees the error, and re-auditing was the cycle-230 audit↔ship loop. `AUDIT_BINDING_*`
residues still re-audit; integrity still blocks. A policy refusal of the push (`GIT_PUSH_POLICY_REFUSED`) goes to the end of the cycle with a ship fail reason. A re-audit cannot change what origin refuses.

Worktree preservation (D10 fix): the orchestrator's exit cleanup skips pruning while a ship
failure is unresolved; the worktree is reclaimed when ship eventually succeeds or via
`evolve cycle reset`. Observability: every attempt logs `[ship] REPAIR:`, stamps
`RepairAttempted`/`RepairOutcome` on the run result, surfaces `ship.repair_attempted` /
`ship.repair_outcome` signals (v2 sentinel plane), and declined attempts annotate the
`ShipError` Debug map → `ship-error.json` → failure floor.

### 8.1 Two-phase landing with a write-ahead intent (amended 2026-10-07)

Cycle 1830 showed that the worktree landing was not a transaction across the push ([incident](../../incidents/2026-10-07-cycle-1830-a-failed-push-stranded-the-audited-commit.md)). Ship committed and fast-forwarded the shared `main`, and then a GitHub 500 failed the push. The retry ran the gates again, and they measured ship's own commit. The code home is `go/internal/phases/ship/landing` (ADR-0103 unit 07). The fix round of 2026-10-08 closed the gaps that the first review found.

**The intent.** `.evolve/landing/cycle-<N>.json` (`landing.IntentPath`) holds these fields:

- `cycle`, `run_id`, `audit_artifact_sha256` and `audited_tree`: the audit that the landing serves;
- `lane_tree`: the tree of the lane before ship's inbox consumption, for the unwind;
- `worktree_base_sha`: the base of the lane, for the unwind;
- `commit_sha` and `commit_tree`: the lane commit;
- `consumed_paths`: the inbox consumption pairs that the commit carries;
- `explanation_view_sha256`: the SHA256 of the sealed Build explanation (the host snapshot);
- `pre_main`, `branch` and `lane_branch`: the integration branch, its tip before the landing, and the lane branch;
- `status`: `prepared`, `complete`, `unwound` or `stale`.

Only ship writes the intent. The file is outside every run workspace, and `.evolve/landing/` is on the protected surface (`guards.ProtectedSurfaceManifest`). So the role guard denies a phase Edit or Write there and raises an alarm. The role guard sees only the Edit and Write tools. The OS sandbox is the second layer: no profile lets a phase write `.evolve/landing/` or the ship journal, so it stops Bash, the other write tools and a symlink. The resume also needs a journal entry and a PASS audit row, and the tree check refuses bytes that no audit saw. Ship writes the intent atomically (a temp file, then a rename). A resume never takes the branch from the intent: it uses the branch of the plane.

**The life of an intent.**

1. **Commit object.** Ship makes the lane commit with `git commit-tree`. This moves no ref.
2. **Catch-up.** Under `ship.lock`, ship fast-forwards `main` to `origin/main` when two conditions are true.
   - `main` is a strict ancestor of `origin/main`.
   - The ship journal holds each commit in `main..origin/main`.
3. **Check.** Ship checks that `main` is an ancestor of the commit (`Landing.CheckFastForward`).
4. **Divergence.** On a divergence, ship moves the lane ref to the commit and stops. It writes no intent and no journal entry.
5. **Record.** Ship journals `commit_sha` with its cycle. Then it writes the intent as `prepared`.
6. **Ref.** Only after the record does ship move the lane ref, with `git update-ref`.
7. **Record failure.** If the journal or the intent write fails, ship moves the ref, unwinds the lane and pushes nothing.
8. **Push.** Ship adopts the consumed paths through `adoptIntentConsumption` and checks `commit_tree` against the audit binding.
9. **Push target.** Then ship pushes `commit_sha:refs/heads/<branch>`.
10. **Settle.** After the push lands, ship fast-forwards the plane `main` (`settleLanding`). A failed fast-forward is a WARN.
11. **Complete.** Ship runs the post-push tree check, writes `ship-binding.json` and marks the intent `complete`.

The divergence rule keeps the fleet rebase (ADR-0105 B1) as it was: the rebase finds the lane commit on the lane. A failed fast-forward after the push (`SHIP_LANDING_MAIN_ADVANCE_FAILED`) does not stop later landings, because the next landing does the catch-up first.

**The resume.** `Landing.Resume` is the first stage of `ship.Run`. It runs before the repo-contract pack, the explanation gate and the audit binding.

1. **Roll forward.** A stop can come after the record and before the ref moves. Ship then moves the lane ref forward, if three conditions are true.
   - The lane tip is the parent of `commit_sha`.
   - The staged tree is `commit_tree`.
   - The journal holds `commit_sha`.
2. **Witness.** The host reads the run and the cycle from `cycle-state.json`. It also reads the lane tip, the journal and the newest audit.
3. **Checks.** A `prepared` intent resumes only when all of these checks pass:
   - the ship journal holds `commit_sha`;
   - the cycle and the run of the intent are the cycle and the run of the host;
   - the newest audit of the run is a PASS, and its bytes match its SHA256;
   - the newest audit names the same artifact and tree (no audit supersedes it);
   - the lane tip is `commit_sha`, and the sealed Build explanation has the same SHA256.
4. **Ancestry.** `Landing.Admit` checks that the commit holds `commit_tree`. Then it fetches origin and reads the ancestry.
   - When `commit_sha` is an ancestor of origin, origin holds the commit. Ship pushes nothing and settles the landing.
   - Otherwise origin and the plane `main` must be ancestors of `commit_sha`. Then ship resumes at the push.

No gate runs again. The content-addressed commit and the binding check before the push prove the bytes. The audit binding of a resume comes from the newest audit row (`bindResumedAudit`, in `audit.go`), never from the intent.

**Unwind or stale.** When an intent does not resume, ship decides from the lane, not from the intent.

- The lane tip is `commit_sha`, and the cycle and the run are the host's. Then ship resets the lane to the audited shape before any gate runs. The unwind is B1 `unwindShipCommit` (`core.UnwindToAuditedShape`), to `lane_tree` on `worktree_base_sha`. Ship marks the intent `unwound`.
- In all other cases, the lane does not hold this commit. Ship marks the intent `stale`, changes nothing on the lane, and runs the gates.

A failed write of `unwound` or `stale` is a `STATE_IO` error, not a WARN. A push failure that is not transient also unwinds the lane, so Audit never measures ship's own commit. The unwind goes to `lane_tree`, not to `audited_tree`. A carried lane (ADR-0105 B3) holds the change of the audit on a later base, and only `lane_tree` matches its base. If the unwind declines, the cycle stops with `GIT_LANDING_UNWIND_DECLINED` (integrity), and `main` does not move. The message gives the operator steps.

The plane `main` is never ahead of origin through ship's own action. A transient push failure goes back to ship, and the resume completes it.

**A lane with nothing to ship whose commits origin does not hold.** `prepareChanges` used to report "nothing to ship" when the lane was equal to local `main` while `main` was ahead of origin. So a ship passed without a push (the landing-lost WARN of cycle 1830). But at each boundary, local `main` is ahead of origin by design: the sync-main merge, the dossier closeouts and the inbox stamps. A lane with nothing to ship must not stop for those commits.

So ship refuses with `GIT_LANE_NOT_ON_ORIGIN` (integrity) only for a stranded landing. That is a commit in `origin/<branch>..<lane>` that the ship journal records as a `cycle` commit, whose landing intent is not `complete`. In all other cases, ship keeps the "nothing to ship" result. The reason:

- A push needs a binding that proves the lane's commits are the work of this audit. Only a `prepared` intent proves that, and the resume takes that path before `prepareChanges` runs.
- A stranded landing has no proof for this lane, and a push publishes commits with no proof. The integrity class stops the cycle with its work kept.
- The message gives the operator steps: fast-forward the plane `main` to the commit, run `evolve sync-main`, then run `evolve ship --push-only`.

**What is left.** Two binding sites still compare the audited tree with a held tree directly: the post-push idempotency check (`native.go`) and `verifyPostPushPredicateEvidence` (`audit.go`). A consuming or carried ship that is dispatched again after its intent is `complete` does not take the report-only path. Inbox `carried-ship-resume-binding-sites-skip-the-rule` stays open for these two sites.

A stop between ship's inbox consumption and the record is a window from before this change. The next ship then sees the consumption with no intent. Inbox `ship-consumption-before-the-landing-record-has-no-resume` keeps it open.

## Consequences

- No abnormal termination is silent: kill -9 a retro bridge, `evolve cycle reset`, or a loop
  fatal all leave a retrospective + lesson + failedApproaches entry (live-verified for reset).
- Failure-learning policy has exactly one user surface; the env flag retires next release.
- The advisor participates in failure routing with full forensics (routing-decision artifacts,
  clamps), but can never weaken the floor.
- Open follow-ups: persona cards (`agents/evolve-router.md`) still name failure-insert phases
  statically (queued for the Phase-6 personas pass); R7 appender unification deferred.

## Amendment (2026-09-15) — the failure block's class is vocabulary, validated at the gate

§7's failure block was verified for presence only; the `class` string went unvalidated straight into the retry envelope, where an unrecognised class declines the direct repair grant. Cycle 1684's audit wrote `superseded-predicate-contradiction`, the decline was silent, and the retry the retrospective later adjudicated rebuilt without the audit's findings. Now: the deliverables gate refuses an audit `class` outside `failurelog.KnownClassifications` (`failure_class_unknown`, the correction names the vocabulary), the audit contract block renders that vocabulary (`failurelog.VocabularyList`, one spelling for prompt and correction), and the decision is a coded signal (`ORCHESTRATOR_AUDIT_REPAIR_DECLINED` / `_GRANTED`). The two vocabularies — failurelog's 13 classes (what agents declare) and the failure-policy table's 7 categories (what the retry envelope reads) — meet in ONE join, `core.policyCategoryFor`; a known class with no repair row declines by name ("no retry policy row"), never as "unrecognised". The prompt exemplar draws its class from the vocabulary. The check is scoped to the audit's unconditional block; the PhaseIO self-report path's class feeds no decision today. Research F19 in `docs/research/verification-wave-findings-2026-09-14.md`.
