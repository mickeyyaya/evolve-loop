# ADR-0034: Unified Deliverable Contract + Self-Check Harness

- Status: Accepted
- Date: 2026-06-03
- Supersedes/extends: ADR-0033 (structured verdict single-source-of-truth)

## Context

The autonomous loop kept producing **inaccurate or misplaced deliverables** because of LLM
randomness. Empirically, ~18 of the last ~30 `fix(...)` commits (≈60%) were in this one failure
class:

- **Write-location / cwd confusion** (10): `recoverBuildLeak` ×3, `cb604d6` per-persona
  write-location contracts, `cf2eeff`/`991daa1`/`e258614` cwd fixes, `d58769c` backfill paths,
  `f96537c`/`43bf333` builder-output preservation.
- **Verdict/report-format drift** (4): `87ad308`/`64b2d95`/`7a4d557`/`36ea0af`.
- **Missing deliverable / no self-check** (4): `2790533` scout materialization, `2a57c4d` builder
  pre-handoff self-check, `e93c46c`, `0853934`.

**Root cause:** the agent's prompt never carried the **exact output path**. `runner.go:246`
computed it and passed it only into `BridgeRequest.ArtifactPath` — a flag the *bridge* uses to
poll, not an instruction the *agent* reads. The agent had to infer workspace + filename + join,
then actually Write there. Each step leaked randomness; the fail-open harness silently degraded a
misplaced deliverable into a WARN/backfill instead of a loud "fix it."

Two prior commits were *this design done piecemeal* — `cb604d6` (write-location contracts on 2 of
6 personas) and `2a57c4d` (builder runs `evolve acs suite` before claiming PASS). We were paying
the cost one reactive commit at a time.

## Decision

One **SSOT deliverable contract** per agent, with a **shared verifier** used both as an
agent-callable self-check and a host-side gate. Five layers:

1. **Contract data** (`internal/phasecontract`): a per-agent `Contract` (artifact path, `Kind`
   markdown|json, required sections/verdicts/keys, write-target) covering the 6 phase agents +
   the routing brain (`router`, a.k.a. advisor; `routing-plan.json`) + the orchestrator
   (`cycle-state.json`). A drift-detector test pins `ArtifactName` to the profile
   `output_artifact` basename (caught + fixed the real `triage` drift). SSOT + Strategy patterns.
2. **Prompt injection** (`internal/adapters/bridge`): a deterministic `## Deliverable Contract`
   block injected at the rules/policy prefix seam, with the **exact path in a footer** (last line).
   Cache-safe (invariant block in the prefix; volatile path in the suffix) AND recency-optimal.
3. **Self-check executable** `evolve phase verify <phase>`: deterministic well-formedness checks
   (file exists at the contracted path, sections/verdict present, JSON keys present via a tolerant
   reader, not stray in the worktree). Shared `internal/deliverable` package so the agent
   self-check and the host gate run byte-identical logic.
4. **Host gate + circuit breaker** (`internal/deliverable.NewReviewer`): mounted at the
   orchestrator's `DeliverableReviewer` seam, chained after evalgate via `core.ChainReviewers`.
   `EVOLVE_CONTRACT_GATE` (RolloutStages, **default enforce**). Fail-open on ambiguity,
   fail-closed on confirmed violation. A circuit breaker trips on **contract/quality violations**
   (not exit codes) and demotes enforce→advisory after N consecutive blocks.
5. **Verdict sentinel** (`<!-- evolve-verdict: {...} -->`): classifiers read it first, then fall
   back to legacy regex-on-prose (Strangler Fig). Removes the verdict-drift class.

### Key decisions

- **Default enforce** (not staged shadow→enforce). Fault-proof immediately; the circuit breaker +
  `off` kill-switch are the safety, not a fail-open default. Operators run one `shadow` cycle
  pre-merge to confirm no false-block.
- **Validation, not guardrail.** The contract checks *well-formedness only*; semantic correctness
  stays the auditor's LLM-judged job (anti-Goodhart; "validation = well-formed, guardrail =
  allowed — both needed", digitalapplied/orq 2026).
- **Advisor = `router`.** The routing brain dispatches with `Agent="router"`; the contract is
  keyed by that wire identity with `advisor` as a human-facing alias. This forces the brain to
  materialize `routing-plan.json` (the advisor-brain audit found 0 ever produced).

## Consequences

- The 18-commit reactive-fix class is closed by one mechanism; future drift fails CI (drift
  detector) instead of silently mis-grading at cycle time.
- Small prompt-size cost (<200-token block); cache-safe by construction.
- The host gate is the backstop even when a CLI ignores the prompt instruction (defense in depth:
  prompt → agent self-check → host gate).
- Rollback: `EVOLVE_CONTRACT_GATE=off` (byte-identical to pre-feature).

See `docs/architecture/deliverable-contract.md` (operator guide) and
`knowledge-base/research/ai-harness-deliverable-contract-2026-06-03.md` (external research).
