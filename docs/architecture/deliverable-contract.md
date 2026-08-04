# Deliverable Contract + Self-Check (operator guide)

> Design: [ADR-0034](adr/0034-unified-deliverable-contract.md). Research:
> [docs/research/ai-harness-deliverable-contract-2026-06-03.md](../research/ai-harness-deliverable-contract-2026-06-03.md).

The deliverable contract makes every phase agent write its deliverable to the **exact contracted
path** in the **right shape**, lets the agent **self-check** before finishing, and gives the
harness a deterministic **backstop gate**. It closes the "misplaced/malformed deliverable" failure
class that dominated recent bug-fix churn.

## The contract (SSOT)

`go/internal/phasecontract` registers one `Contract` per agent:

| agent | artifact | kind | location |
|---|---|---|---|
| build | `build-report.md` | markdown | workspace |
| scout | `scout-report.md` | markdown | workspace |
| tdd | `test-report.md` | markdown | workspace |
| audit | `audit-report.md` | markdown | workspace |
| intent | `intent.md` | markdown | workspace |
| triage | `triage-report.md` | markdown | workspace |
| router (a.k.a. advisor) | `routing-plan.json` | json (key `plan`) | workspace |
| orchestrator | `cycle-state.json` | json (keys `cycle_id`,`phase`) | `.evolve/` |

A markdown contract requires its sections (from `phasecontract.<Phase>.Sections`) and a parseable
verdict; a JSON contract requires valid JSON with the listed top-level keys (a **tolerant reader**
— unknown/future keys are ignored). `ArtifactName` is pinned to the profile `output_artifact`
basename by `TestArtifactNameMatchesProfileOutput` (drift detector).

### Reading the registry from Go (cycle-1145)

Two accessors project the registry so no consumer re-declares its vocabulary:

- **`phasecontract.ArtifactName(phase) string`** — the artifact-name SSOT. Returns `""` for an
  unregistered phase *and* for a `NoArtifact` phase (`ship`, whose result is a pushed commit);
  callers needing a fallback branch on the empty string, as `core.backfillArtifactPath` does.
  `internal/evalgate`, `internal/topngate`, `internal/phases/scout`, `internal/router` and
  `internal/cyclesimulator` each carried their own `"scout-report.md"` copy until cycle-1145 — a
  registry rename left all five reading a file nobody wrote. **Never re-type an artifact filename
  in Go; call this.** Enforced by `go/acs/cycle1145` predicate 001 (absence check over
  `go/internal`, with 003 as its anti-gaming twin).
- **`phasecontract.ArtifactFilename(phase) string`** (cycle-1147) — `ArtifactName` plus the
  `"<phase>-report.md"` convention fallback, for the callers that need *a* filename rather than the
  "is one registered?" signal. It exists because the fallback branch itself was the duplicated
  thing: `core.backfillArtifactPath`, `core.recordGenericBinding`, `core/cyclerun_remediate`
  and `cycleclassify` each re-typed `phase + "-report.md"` beside their own lookup, so a phase
  whose registry name *diverges* from the convention silently got the wrong path from three of
  them. That was not hypothetical — the `tdd` gate writes **`test-report.md`**, so graduated
  remediation had been telling the builder to "read the gate's report at `tdd-report.md`", a file
  that never exists (fixed here; pinned by `TestRemediation_GateFailThenPassContinuesSpine`, which
  now asserts through the registry and explicitly rejects the conventional name).
  **Rule: `ArtifactName` when the empty string is meaningful, `ArtifactFilename` otherwise; never
  the literal.**
- **`phasecontract.Contracts()`** — the agent-name vocabulary. `internal/subagent`'s dispatch
  allow-list (`agentRoles`) is the **union** of every non-`NoArtifact` registry `AgentName` and the
  small hand-kept `nonRegistryRoles` slice (`inspirer`, `evaluator`, `plan-reviewer`, `memo`,
  `tester` — profile-backed roles that are not spine phases). Registering a spine phase now makes
  it dispatchable automatically; before cycle-1145 the list was hand-typed beside the registry and
  `router` had already fallen out of it. `NoArtifact` phases are excluded so `ship` — registered but
  with no `.evolve/profiles/ship.json` — stays rejected and the role↔profile conformance invariant
  holds.

**The one sync point that cannot be automated:** `legacy/scripts/dispatch/subagent-run.sh` (cmd_run's
role case, ~line 631) mirrors `agentRoles` in bash. It is a separate runtime with no access to the Go
registry, so a **new role must be added there by hand**. Every other consumer derives. This is
recorded in `run.go`'s doc comment as well, at the declaration an editor actually touches.

## What the agent sees

The bridge injects a deterministic `## Deliverable Contract` block (rules < policy < contract <
body) and appends the **exact absolute path** as the last line:

```
DELIVERABLE PATH: /…/.evolve/runs/cycle-213/build-report.md
```

The invariant block stays in the cacheable prompt prefix; the per-cycle path lives in the footer
(cache-safe + recency-optimal). The block tells the agent to write there, emit the verdict
sentinel, and run `evolve phase verify` before finishing.

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
preamble ones, and XML-tagged sections parse unambiguously — which is why the *correction* prompt
(identical requirements, tail placement) already got compliance the prefix block did not. Every
string in the block is projected from `Contract.Sections` / `Contract.RequiredKeys` /
`RenderVerdictSentinel`, so there is **no second template** for the writer and the detector to drift
apart on. The sentinel is gated on `len(Verdicts)>0` exactly as the prefix block gates it (build/
scout/triage stay sentinel-free), and a `NoArtifact` contract (`ship`) gets the footer alone.

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

- **Fail-open on ambiguity** (unknown phase, unreadable dir) — never bricks the loop on the gate's
  own inability to decide.
- **Fail-closed on a confirmed violation** (missing/misplaced/malformed deliverable) at enforce.
- **Circuit breaker:** trips on contract/quality violations (not process exit codes). After N
  consecutive blocks (`defaultBreakerThreshold = 3`) it demotes enforce→advisory and logs a
  `CIRCUIT OPEN` escalation, so a miscalibrated gate cannot halt the loop. State persists in
  `.evolve/contract-gate-breaker.json`; a clean cycle resets it (half-open). The counter is
  **global** (not per-phase, per-cycle or per-lane), so a cycle that aborted mid-ladder can leave it
  hot — which is why the escalation below keys off the gate's reported count, not a local ordinal.
- **CLI escalation before the breaker** (`internal/core/contract_escalation.go`): a contract block
  never triggers the profile's `cli_fallback` chain (that fires only on infra exits
  `{80,81,85,124,127}`), so a CLI that systematically mis-formats a deliverable used to burn every
  correction and open the circuit — a format failure silently WEAKENING the gate (batch-19
  adversarial-review, batch-21 triage; both agy-tmux). The correction ladder now re-dispatches the
  **second** consecutive block on a different CLI **family**: the first candidate in the phase's
  resolved dispatch chain from another family, else the universal `claude-tmux` fallback.
  - The **first** block is never escalated: one malformed turn is a bad turn, not a CLI verdict.
  - The trigger is `ReviewResult.Blocks` — the gate's own consecutive-block count — never a
    re-counted correction ordinal (the two desync when the global counter arrives hot or the salvage
    rung consumes a block without a re-dispatch).
  - The count alone is not the signature (cycle-1289): the block must also be the **same defect** as
    the one that triggered the previous correction, compared through the blocker breaker's own
    identity primitive `normalizeReasonForFingerprint` (`internal/core/failure_digest.go`) rather
    than a second hashing scheme — so two blocks reporting one defect in verbatim-different text
    (duration tokens, narrative verdicts) still escalate. Two DIFFERING violations are two honest
    defects, not an incapable CLI, and do not escalate. The rule is *prior reason known AND
    differing ⇒ suppress*: when the breaker arrives HOT from an earlier cycle there is no prior
    reason on this ladder, and escalation still gets its shot before the third strike.
  - `Blocks == 0` never escalates. The other gates chained at the same seam (evalgate, topngate,
    triagecap, the build floor) keep no block counter; their rejections are task-binding/capacity
    failures, not format-compliance failures, so a different CLI is not the remedy.
  - The failed family comes from the CLI that **actually ran** (routing override > `EVOLVE_*_CLI` >
    profile, via `llmroute.Resolve`), not from `profile.cli` — which would compute the family from a
    CLI that never dispatched. Same-family siblings (`claude-tmux`/`claude-p`) are not escalations.
  - Candidates are validated with `policy.ValidatePin`, so a profile's `allowed_clis` bounds the
    escalation exactly as it bounds an operator pin.
  - Applied to `PhaseRequest.ModelRoutingCLI` on **that re-dispatch only** (a soft overlay: escalated
    CLI becomes chain primary, the profile's own chain stays behind it) and reverted when the ladder
    ends. The phase's primary routing and the profile on disk are untouched — the common PASS path is
    unchanged. Minted/user phases resolve `.evolve/profiles/<phase>.json` (the built-in
    `phaseAgentName` table covers only the 10 spine phases).
- **A demotion is no longer silent.** `ReviewResult.Demoted` marks the approval the gate did not
  earn; `core.ChainReviewers` carries it through both exits (all-approve **and** a later gate's
  rejection). The orchestrator emits a `WARN CONTRACT GATE DEMOTED` line naming the phase, the CLI,
  whether escalation actually ran, and the last violation; appends a `contract_gate_demoted` ledger
  entry whose `Action` carries the same evidence; and **stages** an autofile escalation intent in
  `.evolve/escalations/pending-actions.jsonl`. Staged, never written straight to `.evolve/inbox`: a
  mid-cycle inbox write races `inboxmover.Claim`'s `os.Rename`; `recurrence.ApplyBoundary`
  (per-iteration loop boundary) is the only sanctioned inbox writer — and it applies intents only at
  `failure_disposition.stage=enforce`.

## Write-in-flight grace (read robustness)

A phase agent's final deliverable write is not atomic with respect to the verify call that follows
it: a verifier can observe ENOENT (create not yet visible) or a zero-length file for a deliverable
that IS being written. A single unretried read cannot distinguish that from "never written", and
both surface as a CONFIRMED violation — a false FAIL that fails CLOSED.

`readDeliverableWithGrace` (`internal/deliverable/deliverable.go`) therefore treats
absence/emptiness as **provisional** for a bounded window (`readGraceWindow = 500ms`, re-polled
every `readGracePoll = 20ms`):

- **Reads first, waits only on failure.** The common already-written case pays exactly one
  `os.ReadFile` and no sleep — pinned by `TestReadDeliverableWithGrace_PresentContentIsOneReadNoSleep`.
- **Never launders a real violation.** A genuinely missing or permanently empty deliverable still
  yields the same violation code once the window closes. The window delays the verdict, never
  changes it.
- **Fail-open on infra.** A non-absence read fault (EISDIR, permissions, IO) returns immediately as
  infra ambiguity — it will never clear, so the budget is not spent on it and it is never
  reclassified as a violation.
- **Layering.** On the host-runner path this nests inside the existing 16x reconcile retry
  (`runner.go verifyReconcileDeliverable`), so a genuinely-absent artifact's confirmed-missing worst
  case is ~11s — accepted: paid once, only by a phase that produced nothing. The layer's real
  purpose is the retry-LESS callers (`evolve phase verify` self-check).

Deliberately **not** configurable: an I/O robustness constant, not a phase setting (`graceSleep` is
a test seam, not a dial). Coverage: `internal/deliverable/grace_test.go`.

**Closed at the source (`artifact-ready-crosspoll-debounce`).** Partial-but-non-blank content — the
file present with its sections but its trailing verdict sentinel not yet appended (observed
cycle-1198) — is deliberately NOT retried by this reader. An in-poll settle sleep cannot span the
multi-second gap between an agent's `Write` and its follow-up `Edit`, so the fix lives upstream in
the bridge completion detector (`internal/bridge/completion.go`), which now carries a **cross-poll
stability window**: the same `(path, size, mtime)` key must be observed across `artifactStableTicks`
consecutive wait-loop ticks (~2s apart) before `ready` fires. `mtime` is in the key because a
size-only window is blind to an equal-length fix-up `Edit`. Not configurable, for the same reason
`readGraceWindow` is not.

### Relocation is gated by the window, with ONE stated exception

`artifactLocate` (read-only: where is the artifact?) is split from `artifactReady` (the mover that
canonicalizes a non-canonical fallback, per the cycle-108/141 tolerance) precisely so the window can
gate the destructive half. `relocateFile` falls back to **copy+remove** when rename fails
(cross-device, unwritable source directory); run against a file the agent is still appending to,
that publishes a truncated snapshot at the canonical path and then deletes the original. So it runs
only on the tick the window closes.

The exception is the wait loop's ONE post-cancel final poll (`isFinalPoll`, and the `ctx.Err()` twin
for a detector whose context died mid-wait). It completes with `stable == 0` — deliberately, because
demanding a fresh window it can never get would launder every finished-at-the-buzzer session into
`ExitArtifactTimeout`, strictly worse than the truncated read this closed. That concession is
bounded to `renameOnlyRelocate`: rename relinks the inode, so an agent's open fd keeps appending
into the file at its new canonical path and no byte is lost; if the rename cannot be done, the poll
returns the error and the phase takes its artifact timeout. `copy+remove` never runs under finality
(cycle-1256 audit D1 — the earlier unqualified "the window gates relocation" wording in this file
and in `completion.go` was an audited false claim; state the exception or do not make the claim).

`artifactLocate` is also the chokepoint that decides which bytes become the committed deliverable,
so it `Lstat`s and accepts only non-empty **regular** files — a planted symlink is never followed
into the artifact `evolve ship` commits — and `relocateFile`'s copy branch takes its temp file from
`os.CreateTemp` rather than a log-disclosed `<dst>.tmp.<pid>` (D3/D4).

Coverage: `internal/bridge/completion_debounce_test.go`,
`completion_relocate_stability_test.go`, `completion_finalpoll_relocate_test.go`. Durable regression
entry: `.evolve/evals/artifact-ready-crosspoll-debounce.md`.

## Verdict sentinel (Strangler Fig)

Producers emit `<!-- evolve-verdict: {"phase":"audit","verdict":"PASS","schema_version":1} -->`.
The audit classifier and the verifier read the sentinel **first**, then fall back to the legacy
regex-on-prose, so older reports still classify. This removes the verdict-format-drift class.

### Selection is tail-anchored (cycle-1298)

A report may contain **several** strings matching the sentinel shape. Only the last valid one is the
producer's actual verdict; the earlier ones are prose — contract examples the agent was shown, review
commentary quoting a verdict it is discussing, scrollback echoes. `ParseVerdictSentinelFull`
(`go/internal/phasecontract/sentinel.go`) therefore walks candidates from the **END** of the document
and returns the **last** one that unmarshals, carries a non-empty verdict, and is not a placeholder
echo. Invalid candidates are **skipped, never fatal**. Documents with a single sentinel — the
overwhelmingly common case — are unaffected.

First-match selection was the original rule and it was wrong in two directions:

- a quoted decoy **outranked** the real verdict, so a report discussing a `PASS` shipped as `PASS`;
- worse, when a decoy's JSON was **elided** for readability (`{"verdict":"FAIL",…}`), the unmarshal
  failed and the whole read returned not-found — blanking a real verdict rather than merely
  misreading it.

In **cycle-1298** the second shape hit an adversarial-review report whose prose quoted the sentinel
five times before emitting a genuine `verdict=FAIL` at the tail. The gate read nothing, treated the
absent verdict as non-blocking, and effectively demoted itself from enforce to advisory on the one
report that was trying to stop the cycle. That live artifact is checked in verbatim as the regression
fixture `go/internal/phasecontract/testdata/cycle1298-quoted-decoys.md`.

**If you write a new sentinel reader, read the shared parser instead.** `internal/deliverable` calls
`phasecontract.ParseVerdictSentinelFull`/`ParseVerdictSentinel` directly (`deliverable.go:257,327`) —
one implementation, no second copy to drift. Coverage:
`go/internal/phasecontract/sentinel_tailanchor_test.go`, which runs the cycle-1298 fixture through
the parser, asserts first-match selection gives the *wrong* answer on those same bytes (so the
fixture cannot silently stop discriminating), and proves the production reader `ReadFailureBlock`
inherits the rule.

**Producer-side rule:** emit your verdict sentinel **once, at the tail** of the report. If you must
quote sentinel syntax in prose, quoting it earlier in the document is safe by construction.

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
