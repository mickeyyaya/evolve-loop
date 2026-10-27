# Deliverable Contract + Self-Check (operator guide)

> Design: [ADR-0034](adr/0034-unified-deliverable-contract.md). Research:
> [docs/research/ai-harness-deliverable-contract-2026-06-03.md](../research/ai-harness-deliverable-contract-2026-06-03.md).

The deliverable contract makes every phase agent write its deliverable to the **exact contracted
path** in the **right shape**. It lets the agent **self-check** before it finishes. It gives the
harness a deterministic **backstop gate**. It closes the "misplaced/malformed deliverable" failure
class that dominated recent bug-fix churn.

## The contract (SSOT)

`go/internal/phasecontract` registers one `Contract` per deliverable protocol. Most agents have one
protocol. The router has three because one model/profile identity produces three different artifacts:

| protocol | artifact | kind | location |
|---|---|---|---|
| build | `build-report.md` | markdown | workspace |
| scout | `scout-report.md` | markdown | workspace |
| tdd | `test-report.md` | markdown | workspace |
| audit | `audit-report.md` | markdown | workspace |
| intent | `intent.md` | markdown | workspace |
| triage | `triage-report.md` | markdown | workspace |
| router plan (a.k.a. advisor) | `routing-plan.json` | JSON array | workspace |
| router replan | `routing-replan.json` | JSON array | workspace |
| router proposal | `routing-proposal.json` | JSON object | workspace |
| orchestrator | `cycle-state.json` | json (keys `cycle_id`,`phase`) | `.evolve/` |

A markdown contract requires its sections (from `phasecontract.<Phase>.Sections`) and a parseable
verdict. A JSON contract can require a top-level object or array and can require object keys. Object
keys use a **tolerant reader**: unknown future keys are ignored. The zero-value JSON shape retains
the legacy "any valid JSON value" behavior for user-defined contracts. `ArtifactName` is pinned to
the profile `output_artifact` basename by `TestArtifactNameMatchesProfileOutput` for single-output
profiles. The multi-protocol router instead has a profile-permission test that projects all three
artifact names from the registry.

`BridgeRequest.Agent` and `BridgeRequest.Contract` intentionally answer different questions.
`Agent` selects the router profile, model policy, skill overlays and telemetry label.

`Contract` selects the output protocol that the bridge injects into the prompt. An empty `Contract` defaults to `Agent` for
existing callers. An explicit unknown contract fails before model dispatch. A silent fallback is not allowed here.
If the bridge falls back silently, a typo is validated against the wrong file.

`PhaseAdvisor` explicitly selects `router`,
`router-replan` or `router-proposal` beside the corresponding artifact path. It never infers the
protocol from a filename.

### Conditional sections (2026-09-03)

Some sections are owed only in some cycles. `Contract.ExplanationSections` lists the sections that a phase
owes while the explanation-documentation contract of the cycle is active. Today this is the `## Explanation Documentation` section of the audit report
(declared once as `phasecontract.ExplanationDocumentation`). The
own gate of the audit derives its heading from the same declaration.

The contract version rides on
`phasecontract.Roots.ExplanationDocumentationVersion`, so every verifier asks the same question. The verifiers are:

- the host gate (from `ReviewInput`);
- the runner (from the request);
- the salvage re-check;
- the `evolve phase verify` of the agent (from `cycle-state.json`, derived from the workspace path when `--evolve-dir` is absent). An unreadable state is a stderr WARN, never a silent "inactive".

The match is the exact visible level-two heading (`reportdoc.HasSection`). A heading in a fence, a
comment, or at another level counts for neither the gate nor the self-check. A missing section
is reported under the same `missing_section` code as any other. It is therefore a correction re-dispatch
with the reason as the directive, not a terminal FAIL (cycles 1601/1603). The cycle is never asked for the section when it does not have the
contract.

The `## Explanation Documentation Review` section of the retro stays a gate-only
check for now. Retro has a registered location/well-formedness contract. It declares no
conditional explanation section and is not on the ship path.

### Reading the registry from Go (cycle-1145)

Two accessors project the registry so no consumer re-declares its vocabulary:

- **`phasecontract.ArtifactName(phase) string`** — the artifact-name SSOT. It returns `""` for an
  unregistered phase *and* for a `NoArtifact` phase (`ship`, whose result is a pushed commit).
  Callers that need a fallback branch on the empty string, as `core.backfillArtifactPath` does.
  `internal/evalgate`, `internal/topngate`, `internal/phases/scout`, `internal/router` and
  `internal/cyclesimulator` each carried their own `"scout-report.md"` copy until cycle-1145. A
  registry rename left all five reading a file that nobody wrote.

  **Never re-type an artifact filename in Go; call this.**
  `go/acs/cycle1145` predicate 001 enforces it (absence check over
  `go/internal`, with 003 as its anti-gaming twin).
- **`phasecontract.ArtifactFilename(phase) string`** (cycle-1147) — `ArtifactName` plus the
  `"<phase>-report.md"` convention fallback. It serves the callers that need *a* filename rather than the
  "is one registered?" signal. It exists because the fallback branch itself was the duplicated
  code. `core.backfillArtifactPath`, `core.recordGenericBinding`, `core/cyclerun_remediate`
  and `cycleclassify` each re-typed `phase + "-report.md"` beside their own lookup. A phase
  whose registry name *diverges* from the convention silently got the wrong path from three of
  them.
  - That was not hypothetical. The `tdd` gate writes **`test-report.md`**.
  - Graduated remediation therefore told the builder to "read the gate's report at `tdd-report.md`", a file
    that never exists.
  - This change fixed it. `TestRemediation_GateFailThenPassContinuesSpine` pins the fix. The test
    now asserts through the registry and explicitly rejects the conventional name.
  **Rule: `ArtifactName` when the empty string is meaningful, `ArtifactFilename` otherwise; never
  the literal.**
- **`phasecontract.Contracts()`** — the agent-name vocabulary. The dispatch
  allow-list of `internal/subagent` (`agentRoles`) is the **union** of two sets.
  The first set is every non-`NoArtifact` registry `AgentName`. The second set is the
  small hand-kept `nonRegistryRoles` slice (`inspirer`, `evaluator`, `plan-reviewer`, `memo`,
  `tester` — profile-backed roles that are not spine phases). A spine phase that you register is now
  dispatchable automatically. Before cycle-1145 someone hand-typed the list beside the registry, and
  `router` had already fallen out of it.
  - `NoArtifact` phases are excluded.
  - `ship` is registered but has no `.evolve/profiles/ship.json`.
  - It therefore stays rejected, and the role↔profile conformance invariant holds.

**The one sync point that cannot be automated:** `legacy/scripts/dispatch/subagent-run.sh` (cmd_run's
role case, ~line 631) mirrors `agentRoles` in bash. It is a separate runtime with no access to the Go
registry, so you **must add a new role there by hand**. Every other consumer derives it. The doc comment of `run.go` records this as well,
at the declaration that an editor actually touches.

## What the agent sees

The bridge injects a deterministic `## Deliverable Contract` block (rules < policy < contract <
body) and appends the **exact absolute path** as the last line:

```
DELIVERABLE PATH: /…/.evolve/runs/cycle-213/build-report.md
```

The invariant block stays in the cacheable prompt prefix. The per-cycle path lives in the footer
(cache-safe + recency-optimal). The block tells the agent to write there, to emit the verdict sentinel
when its contract requires one, and to run `evolve phase verify` before it finishes. Router prompts name
the selected protocol. A proposal runs `evolve phase verify router-proposal`, a replan runs
`evolve phase verify router-replan`, and a plan runs `evolve phase verify router`.

Immediately after that footer line, `phasecontract.RenderContractTail` appends the **machine** half of
the contract as one XML-tagged block, at the generation point:

```xml
<deliverable-contract phase="audit">
  <artifact-path>/…/.evolve/runs/cycle-1218/audit-report.md</artifact-path>
  <required-sections>
    <section>## Verdict</section>
  </required-sections>
  <verdict-sentinel verdicts="PASS|FAIL|WARN|SKIPPED"><!-- evolve-verdict: {…} --></verdict-sentinel>
  <self-check>evolve phase verify audit --workspace &lt;your workspace dir&gt;</self-check>
</deliverable-contract>
```

Why the same facts appear twice: Claude follows **turn-tail** instructions more reliably than
preamble ones, and XML-tagged sections parse unambiguously. For this reason the *correction* prompt
(identical requirements, tail placement) already got compliance that the prefix block did not.

The block projects every
string from `Contract.Sections` / `Contract.RequiredKeys` /
`RenderVerdictSentinel`. There is therefore **no second template** for the writer and the detector to drift
apart on. The sentinel is gated on `len(Verdicts)>0` exactly as the prefix block gates it (build/
scout/triage stay sentinel-free). A `NoArtifact` contract (`ship`) gets the footer alone.

## Self-check (agent-callable)

```
evolve phase verify <phase> --workspace <dir> [--worktree <dir>] [--evolve-dir <dir>] [--json]
```

Exit `0` well-formed · `1` confirmed violation (fix it) · `10` usage · `2` ambiguity (caller fails
open). Same `internal/deliverable.Verify` logic the host gate runs, so the agent's pre-finish
check and the harness's post-phase gate can never drift.

## Host gate

`.evolve/policy.json` `gates.contract_gate` (default **enforce**) mounts `deliverable.NewReviewer` at the orchestrator
`DeliverableReviewer` seam, chained after evalgate:

| stage | behavior |
|---|---|
| `off` | no gate (byte-identical to pre-feature) — kill-switch |
| `shadow` | verifier runs, every violation log-only |
| `enforce` (default) | a confirmed violation rejects the phase |

- **Fail-open on ambiguity** (unknown phase, unreadable dir). The loop never breaks on the
  inability of the gate to decide.
- **Fail-closed on a confirmed violation** (missing/misplaced/malformed deliverable) at enforce.
- **Circuit breaker:** trips on contract/quality violations (not process exit codes). After N
  consecutive blocks (`defaultBreakerThreshold = 3`) it demotes enforce→advisory and logs a
  `CIRCUIT OPEN` escalation, so a miscalibrated gate cannot halt the loop. State persists in
  `.evolve/contract-gate-breaker.json`. A clean cycle resets it (half-open).
  - The counter is **global** (not per-phase, per-cycle or per-lane).
  - A cycle that aborted mid-ladder can therefore leave it hot.
  - For this reason the escalation below keys off the reported count of the gate, not a local ordinal.
- **CLI escalation before the breaker** (`internal/core/contract_escalation.go`; full design:
  [contract-block-cli-escalation.md](contract-block-cli-escalation.md)). A contract block
  never triggers the `cli_fallback` chain of the profile (that chain fires only on infra exits
  `{80,81,85,87,124,127}`). A CLI that systematically mis-formats a deliverable therefore used to burn every
  correction and open the circuit. A format failure silently WEAKENED the gate (batch-19
  adversarial-review, batch-21 triage; both agy-tmux). The correction ladder now re-dispatches the
  **second** consecutive block on a different CLI **family**. The target is the first candidate in the resolved dispatch chain of the phase from another family,
  else the universal `claude-tmux` fallback.
  - The **first** block is never escalated: one malformed turn is a bad turn, not a CLI verdict.
  - The trigger is `ReviewResult.Blocks`, the own consecutive-block count of the gate. It is never a
    re-counted correction ordinal. The two desync when the global counter arrives hot or the salvage
    rung consumes a block without a re-dispatch.
  - The count alone is not the signature (cycle-1289). The block must also be the **same defect** as
    the one that triggered the previous correction. The comparison uses the own
    identity primitive of the blocker breaker, `normalizeReasonForFingerprint` (`internal/core/failure_digest.go`), and not
    a second hashing scheme.
    - Two blocks that report one defect in verbatim-different text
      (duration tokens, narrative verdicts) still escalate.
    - Two DIFFERING violations are two honest
      defects, not an incapable CLI, and do not escalate.
    - The rule is *prior reason known AND differing ⇒ suppress*.
    - When the breaker arrives HOT from an earlier cycle, there is no prior
      reason on this ladder. Escalation still gets its chance before the third strike.
  - `Blocks == 0` never escalates. The other gates chained at the same seam keep no block counter.
    These gates are evalgate, topngate, triagecap and the build floor.
    Their rejections are task-binding/capacity failures and not format-compliance failures.
    A different CLI is not the remedy.
  - The failed family comes from the CLI that **actually ran** (routing override > `EVOLVE_*_CLI` >
    profile, through `llmroute.Resolve`). It does not come from `profile.cli`, which computes the family from a
    CLI that never dispatched. Same-family siblings (`claude-tmux`/`claude-p`) are not escalations.
  - **No cross-family target ⇒ breaker-neutral salvage retry** (cycle-1300). When the whole
    dispatch chain of the phase is one CLI family, there is nowhere to escalate. The same trigger then
    enriches the correction with the output of the contract validator **verbatim** (a structured
    re-prompt, `composeContractSalvageRetry`).
    - It adds no dispatch and never touches `PhaseRequest.Driver`.
    - It leaves `ReviewResult.Blocks`, the own counter of the breaker, untouched. The circuit still
      opens on the third strike as the last resort.
    - It is disjoint from escalation by construction. A phase
      *with* a real cross-family target escalates and never also re-prompts.
    - Together the two rungs are the block-recovery ladder that runs before the breaker demotes the gate.
  - Candidates are validated with `policy.ValidatePin`, so the `allowed_clis` of a profile bounds the
    escalation exactly as it bounds an operator pin.
  - The escalation applies to `PhaseRequest.Driver`, an opaque `llmroute.Binding` (X19a), on **that re-dispatch only**. It is a soft overlay: the escalated
    CLI becomes chain primary, and the own chain of the profile stays behind it. It reverts when the ladder
    ends. The primary routing of the phase and the profile on disk are untouched, so the common PASS path is
    unchanged. Minted/user phases resolve `.evolve/profiles/<phase>.json`. The built-in
    `phaseAgentName` table covers only the 10 spine phases.
- **A demotion is no longer silent.** `ReviewResult.Demoted` marks the approval that the gate did not
  earn. `core.ChainReviewers` carries it through both exits (all-approve **and** the rejection of a later gate). The orchestrator does three things:
  - It emits a `WARN CONTRACT GATE DEMOTED` line. The line names the phase, the CLI, if escalation actually ran, and the last violation.
  - It appends a `contract_gate_demoted` ledger entry. The `Action` of the entry carries the same evidence.
  - It **stages** an autofile escalation intent in `.evolve/escalations/pending-actions.jsonl`.

  The orchestrator stages the intent and never writes it straight to `.evolve/inbox`. A
  mid-cycle inbox write races the `os.Rename` of `inboxmover.Claim`. `recurrence.ApplyBoundary`
  (per-iteration loop boundary) is the only sanctioned inbox writer. It applies intents only at
  `failure_disposition.stage=enforce`.

## Declared secondaries and effects (ADR-0100)

The registry (`docs/architecture/phase-registry.json`) has always declared the full output
list of a phase. Until ADR-0100 the gate judged only `outputs.files[0]`. Now, after the shape
checks of the primary, `VerifyWithStage` judges two more declared things — and nothing that is not declared:

| Declaration | Where | Check | Codes |
|---|---|---|---|
| `outputs.agent_owed` (secondaries the persona writes: `triage-decision.json`, `carryover-todos.json`) | `internal/deliverable/secondaries.go` | exists (same write-in-flight grace), non-empty, parses if `.json`/`.ndjson`. Markdown shape stays the business of the primary | `missing_secondary` · `empty_secondary` · `malformed_secondary` |
| `outputs.derived_from` (an agent-owed secondary the host derives from the primary: `triage-decision.json` from `triage-report.md`) | `internal/deliverable/host_effects.go` (`derive`), before the runner's judge and the gate, like the inbox claim | the host writes the file only when it is absent or empty. The read waits out a write in flight. The host writes only when the primary states the commitment. The host never touches a written file. The lane-pin check of the derived file therefore guards the fallback only, and the gate judges an agent-written decision as before. A decline is a WARN and leaves the absence for the gate (ADR-0106 H1/H2) | none of its own: the gate's secondary codes still judge the result |
| `outputs.harness_produced` (`acs-verdict.json`) | — | never demanded of an agent (a harness component writes it) | — |
| `effects` (lifecycle effects the persona performs outside its workspace; today `inbox-claim` on triage) | `internal/deliverable/effects.go` — `effectChecks`, a registry lookup that binds each declared name to one deterministic check | `inbox-claim`: every committed id (`committedset.Committed`) that is an inbox item must sit under THIS cycle's `processing/cycle-N/`. `inboxmover.Locate` finds it. Its layout is the layout of `inboxbatch/layout.go` (`ProcessingDir` / `ProcessingCycleDir` / `ProcessingCycleDirs` / `ParseProcessingCycle`), the same layout that the writer `inboxmover.Claim` uses. Scout/carryover ids with no inbox file owe nothing. An empty or unrecorded commitment owes nothing | `missing_effect` (item + exact `evolve inbox-mover claim` command in the message) · `unbound_effect` (a declared name that no check binds — a registry defect, pinned at commit time by `TestPhaseRegistry_EveryDeclaredEffectHasACheck`). Since F36 (2026-09-26) the `{check, perform}` pair of each effect lives in one registry. Before every review, the host performs `inbox-claim` from the own `ReviewInput` of the gate (`core.Orchestrator.performEffectsAndReview` → `deliverable.HostEffects` → `inboxmover.ClaimPending`, through the same claim floor). The check therefore verifies the own move of the host, and the command of the persona is advisory (ADR-0100 amendment 10). |

Every message is the correction directive that the agent is re-dispatched with. A persistent gap ends
the cycle `FAILED_EXPLAINED` and names the file or effect. A declared effect is judged against
per-cycle state, so `phasecontract.Roots` carries `Cycle`.
The host gate takes it from `ReviewInput.Cycle`. The runner takes it from the phase request (`verifyRootsFor`).
The self-check takes it from the persisted cycle state.

A verifier that omits it fails OPEN with an error and does not
decide blind. `Orchestrator.DeclaredDeliverablesGateWired()` is the composition-root proof.


## Write-in-flight grace (read robustness)

The final deliverable write of a phase agent is not atomic with respect to the verify call that follows
it. A verifier can observe ENOENT (create not yet visible) or a zero-length file for a deliverable
that IS being written. A single unretried read cannot distinguish that from "never written". Both
surface as a CONFIRMED violation — a false FAIL that fails CLOSED.

`readDeliverableWithGrace` (`internal/deliverable/deliverable.go`) therefore treats
absence/emptiness as **provisional** for a bounded window (`readGraceWindow = 500ms`, re-polled
every `readGracePoll = 20ms`):

- **Reads first, waits only on failure.** The common already-written case pays exactly one
  `os.ReadFile` and no sleep — pinned by `TestReadDeliverableWithGrace_PresentContentIsOneReadNoSleep`.
- **Never launders a real violation.** A genuinely missing or permanently empty deliverable still
  yields the same violation code once the window closes. The window delays the verdict, never
  changes it.
- **Fail-open on infra.** A non-absence read fault (EISDIR, permissions, IO) returns immediately as
  infra ambiguity. It never clears, so the code does not spend the budget on it, and it never
  reclassifies it as a violation.
- **Layering.** On the host-runner path this nests inside the existing 16x reconcile retry
  (`runner.go verifyReconcileDeliverable`). The confirmed-missing worst
  case of a genuinely-absent artifact is therefore ~11s. This is accepted: only a phase that produced nothing pays it, and it pays once. The real
  purpose of the layer is the retry-LESS callers (`evolve phase verify` self-check).

Deliberately **not** configurable: an I/O robustness constant, not a phase setting (`graceSleep` is
a test seam, not a dial). Coverage: `internal/deliverable/grace_test.go`.

**Closed at the source (`artifact-ready-crosspoll-debounce`).** This reader is deliberately NOT retried for partial-but-non-blank content.
In that case the file is present with its sections, but its trailing verdict sentinel is not yet appended (observed
cycle-1198). An in-poll settle sleep cannot span the
multi-second gap between a `Write` of an agent and its follow-up `Edit`.

The fix therefore lives upstream in
the bridge completion detector (`internal/bridge/completion.go`). It now carries a **cross-poll
stability window**. The same `(path, size, mtime)` key must be observed across `artifactStableTicks`
consecutive wait-loop ticks (~2s apart) before `ready` fires. `mtime` is in the key because a
size-only window is blind to an equal-length fix-up `Edit`. It is not configurable, for the same reason
that `readGraceWindow` is not.

### Relocation is gated by the window, with ONE stated exception

`artifactLocate` (read-only: where is the artifact?) is split from `artifactReady` (the mover that
canonicalizes a non-canonical fallback, per the cycle-108/141 tolerance) precisely so the window can
gate the destructive half. `relocateFile` falls back to **copy+remove** when rename fails
(cross-device, unwritable source directory). Consider a file that the agent is still appending to.
The fallback publishes a truncated snapshot of it at the canonical path and then deletes the original. So it runs
only on the tick when the window closes.

The exception is the ONE post-cancel final poll of the wait loop (`isFinalPoll`, and the `ctx.Err()` twin
for a detector whose context died mid-wait). It completes with `stable == 0`. This is deliberate.
Without the concession, a demand for a fresh window that it can never get launders every finished-at-the-buzzer session into
`ExitArtifactTimeout`. That is strictly worse than the truncated read this closed.

`renameOnlyRelocate` bounds that concession. Rename relinks the inode, so the open fd of an agent keeps appending
into the file at its new canonical path. No byte is lost. If the rename cannot be done, the poll
returns the error and the phase takes its artifact timeout.

`copy+remove` never runs under finality.
This is the finding of cycle-1256 audit D1. The earlier unqualified wording "the window gates relocation" in this file
and in `completion.go` was an audited false claim. State the exception or do not make the claim.

`artifactLocate` is also the chokepoint that decides which bytes become the committed deliverable.
It therefore runs `Lstat` and accepts only non-empty **regular** files. A planted symlink is never followed
into the artifact that `evolve ship` commits. The copy branch of `relocateFile` takes its temp file from
`os.CreateTemp` and not from a log-disclosed `<dst>.tmp.<pid>` (D3/D4).

Coverage: `internal/bridge/completion_debounce_test.go`,
`completion_relocate_stability_test.go`, `completion_finalpoll_relocate_test.go`. Durable regression
entry: `.evolve/evals/artifact-ready-crosspoll-debounce.md`.

## Verdict sentinel (Strangler Fig)

Producers emit `<!-- evolve-verdict: {"phase":"audit","verdict":"PASS","schema_version":1} -->`.
The audit classifier and the verifier read the sentinel **first**, then fall back to the legacy
regex-on-prose, so older reports still classify. This removes the verdict-format-drift class.

### Selection is tail-anchored (cycle-1298)

A report can contain **several** strings that match the sentinel shape. Only the last valid one is the
actual verdict of the producer. The earlier ones are prose: contract examples that the agent was shown, review
commentary that quotes a verdict it discusses, and scrollback echoes. `ParseVerdictSentinelFull`
(`go/internal/phasecontract/sentinel.go`) therefore walks candidates from the **END** of the document.
It returns the **last** candidate that meets three conditions:

- its leading complete JSON value decodes;
- it carries a non-empty verdict;
- it is not a placeholder echo.

Invalid candidates are **skipped, never fatal**. Documents with a single sentinel (the
overwhelmingly common case) are unaffected.

First-match selection was the original rule and it was wrong in two directions:

- a quoted decoy **outranked** the real verdict, so a report discussing a `PASS` shipped as `PASS`;
- worse, when the JSON of a decoy was **elided** for readability (`{"verdict":"FAIL",…}`), the unmarshal
  failed and the whole read returned not-found. This blanked a real verdict and did not only
  misread it.

In **cycle-1298** the second shape hit an adversarial-review report. Its prose quoted the sentinel
five times before it emitted a genuine `verdict=FAIL` at the tail. The gate read nothing and treated the
absent verdict as non-blocking. It effectively demoted itself from enforce to advisory on the one
report that tried to stop the cycle. That live artifact is checked in verbatim as the regression
fixture `go/internal/phasecontract/testdata/cycle1298-quoted-decoys.md`.

**If you write a new sentinel reader, read the shared parser instead.** `internal/deliverable` calls
`phasecontract.ParseVerdictSentinelFull`/`ParseVerdictSentinel` directly (`deliverable.go:257,327`).
There is one implementation and no second copy to drift. Coverage:
`go/internal/phasecontract/sentinel_tailanchor_test.go`. The test does three things:

- It runs the cycle-1298 fixture through the parser.
- It asserts that first-match selection gives the *wrong* answer on those same bytes, so the fixture cannot silently stop discriminating.
- It proves that the production reader `ReadFailureBlock` inherits the rule.

**Producer-side rule:** emit your verdict sentinel **once, at the tail** of the report. If you must
quote sentinel syntax in prose, quoting it earlier in the document is safe by construction.

### Trailing bytes inside the comment are tolerated (cycle-1478)

The parser reads the captured payload as its **leading complete JSON value** (`json.Decoder`), not as a
whole-string `Unmarshal`. In cycle-1478 an audit agent emitted a valid 2.8KB WARN sentinel followed
by **one stray `}`** before the `-->`. The whole-string parse rejected that byte. At contract-gate
enforce the prose fallback is off (ADR-0050 §3.10 Slice 5). The gate therefore reported "no parseable
verdict" three times against unchanged bytes. It burned two corrections plus a salvage retry and opened
the contract-gate circuit.

The orchestrator coherence floor then halted the batch as
verdict-incoherence, while prose, sentinel and `acs-verdict.json` all agreed on WARN.

The comment capture bounds the tolerance. A leading value that is not a verdict-bearing object is still
rejected. The vocabulary/placeholder/tail-anchor guards are unchanged. The live artifact is the
regression fixture `go/internal/phasecontract/testdata/cycle1478-trailing-brace.md`. Coverage is in
`sentinel_trailingdata_test.go` and `internal/deliverable/verdict_trailingdata_test.go` (the
enforce-stage gate-level pin). The correction-loop staleness half of that incident (re-validating
an artifact that the re-dispatched agent never rewrote) is tracked separately in inbox item
`contract-correction-stale-artifact-freshness`.

## Rollout / rollback

- Ship at the enforce default; run one `gates.contract_gate=shadow` policy cycle pre-merge to confirm no
  false-block.
- Rollback: set `gates.contract_gate` to `off` in `.evolve/policy.json`.
- Tune the breaker threshold in `internal/deliverable/reviewer.go` (`defaultBreakerThreshold`).

## Sandbox note

`evolve phase verify` only **reads** the artifact, so it is safe under `read_only_repo` profiles
(auditor). Restricted-Bash profiles (scout/auditor/triage/intent/router) carry an explicit
`Bash(evolve phase verify:*)` allow-entry so the in-loop self-check runs; builder/tdd have generic
`Bash`.
