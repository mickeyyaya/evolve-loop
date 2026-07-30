# Deliverable Contract + Self-Check (operator guide)

> Design: [ADR-0034](adr/0034-unified-deliverable-contract.md). Research:
> [knowledge-base/research/ai-harness-deliverable-contract-2026-06-03.md](../../knowledge-base/research/ai-harness-deliverable-contract-2026-06-03.md).

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
  `.evolve/contract-gate-breaker.json`; a clean cycle resets it (half-open).

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

**Known residual (queued):** partial-but-non-blank content — the file present with its sections but
its trailing verdict sentinel not yet appended (observed cycle-1198) — is NOT retried here. Closing
it at the source requires artifact-ready CROSS-POLL stability in the bridge detector
(`artifact-ready-crosspoll-debounce`, queued): an in-poll settle sleep cannot span the
multi-second gap between an agent's `Write` and its follow-up `Edit`, and the change alters the tick
contract for every artifact-completion fixture plus any short `ArtifactTimeoutS`, so it needs its own
cycle with a timeout-budget audit.

## Verdict sentinel (Strangler Fig)

Producers emit `<!-- evolve-verdict: {"phase":"audit","verdict":"PASS","schema_version":1} -->`.
The audit classifier and the verifier read the sentinel **first**, then fall back to the legacy
regex-on-prose, so older reports still classify. This removes the verdict-format-drift class.

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
