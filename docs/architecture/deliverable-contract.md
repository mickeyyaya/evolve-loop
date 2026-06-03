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

## What the agent sees

The bridge injects a deterministic `## Deliverable Contract` block (rules < policy < contract <
body) and appends the **exact absolute path** as the last line:

```
DELIVERABLE PATH: /…/.evolve/runs/cycle-213/build-report.md
```

The invariant block stays in the cacheable prompt prefix; the per-cycle path lives in the footer
(cache-safe + recency-optimal). The block tells the agent to write there, emit the verdict
sentinel, and run `evolve phase verify` before finishing.

## Self-check (agent-callable)

```
evolve phase verify <phase> --workspace <dir> [--worktree <dir>] [--evolve-dir <dir>] [--json]
```

Exit `0` well-formed · `1` confirmed violation (fix it) · `10` usage · `2` ambiguity (caller fails
open). Same `internal/deliverable.Verify` logic the host gate runs, so the agent's pre-finish
check and the harness's post-phase gate can never drift.

## Host gate

`EVOLVE_CONTRACT_GATE` (default **enforce**) mounts `deliverable.NewReviewer` at the orchestrator
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

## Verdict sentinel (Strangler Fig)

Producers emit `<!-- evolve-verdict: {"phase":"audit","verdict":"PASS","schema_version":1} -->`.
The audit classifier and the verifier read the sentinel **first**, then fall back to the legacy
regex-on-prose, so older reports still classify. This removes the verdict-format-drift class.

## Rollout / rollback

- Ship at the enforce default; run one `EVOLVE_CONTRACT_GATE=shadow` cycle pre-merge to confirm no
  false-block.
- Rollback: `EVOLVE_CONTRACT_GATE=off`.
- Tune the breaker threshold in `internal/deliverable/reviewer.go` (`defaultBreakerThreshold`).

## Sandbox note

`evolve phase verify` only **reads** the artifact, so it is safe under `read_only_repo` profiles
(auditor). Restricted-Bash profiles (scout/auditor/triage/intent/router) carry an explicit
`Bash(evolve phase verify:*)` allow-entry so the in-loop self-check runs; builder/tdd have generic
`Bash`.
