# Intent Phase Architecture (v8.19.0)

> The pre-Scout phase that fills Microsoft's User-intent layer in evolve-loop. Opt-in via `EVOLVE_REQUIRE_INTENT=1`. Autonomy-preserving by design.

## Why this phase exists

evolve-loop's trust kernel makes cycles correct, audited, and tamper-evident — but doesn't ensure they're pointed at the right thing. The 2026-04 "giant useless circle force-graph" incident burned 25 cycles because the orchestrator inherited a vague goal verbatim. Five 2026 sources (`arxiv 2409.00557`, Karpathy, Microsoft governance, *Prompt Fidelity*, *Intent Architect*) converge on the diagnosis: autonomous agents drift when user intent is left implicit.

evolve-loop already encodes 3 of Microsoft's 4 intent layers:

| Layer | Owner in evolve-loop |
|---|---|
| Organizational | Trust kernel (sandbox, ledger SHA, ship-gate) |
| Developer | `.evolve/profiles/*.json` (per-role permission allowlists) |
| Role | scout/builder/auditor/orchestrator personas (single perspective each) |
| **User** | **NEW: evolve-intent persona** |

## The schema

`<workspace>/intent.md` with YAML frontmatter:

```yaml
---
awn_class: IMKI | IMR | IwE | IBTC | CLEAR
goal: <restated goal>
non_goals: [...]
constraints: [...]
interfaces: [...]
acceptance_checks:
  - check: "..."
    how_verified: "programmatic|manual|review"
assumptions: [...]
challenged_premises:
  - premise: "..."
    challenge: "..."
    proposed_alternative: "..."
risk_level: low|medium|high|critical
---
```

The 8 non-classifier fields are the convergent set across 5 independent sources. The `awn_class` is the Ask-when-Needed classifier from arxiv 2409.00557 (single class per their model output).

## The autonomy invariant

This phase MUST NOT block on human approval. The intent persona produces intent.md; the kernel verifies structure (`gate_intent_to_research`); the cycle continues. There is no `accept-intent` operator command; there is no mid-cycle pause. The user's recovery path is inter-cycle (re-run `/intent` to replace, then re-run `/loop`), not intra-cycle.

The earlier draft of this design included an `accept-intent` checkpoint. It was removed before implementation because checkpoints break autonomous `/loop` runs — and autonomy is the core value of evolve-loop.

## The phase pipeline (intent-enabled vs default)

```
Default (intent_required=false):
  calibrate → research → discover → tdd → build → audit → ship → learn

Intent-enabled (intent_required=true):
  calibrate → INTENT → research → discover → tdd → build → audit → ship → learn
```

Every existing phase is unchanged. Intent slots in between calibrate and research with two new gates:

| Gate | When | Verifies |
|---|---|---|
| `gate_calibrate_to_intent` | calibrate exit | `cycle-state.intent_required==true` (no-op otherwise) |
| `gate_intent_to_research` | intent exit | intent.md exists, has YAML frontmatter, `challenged_premises >= 1`, `awn_class != IBTC`, SHA matches ledger entry |

## Init-time binding (mid-stream-flip safety)

`cycle-state.intent_required` is recorded at cycle init from `EVOLVE_REQUIRE_INTENT` env. After init, env changes do NOT affect in-flight cycles. This protects against an operator setting/unsetting the flag mid-run and confusing the gates. To turn intent on or off, set the env var and start a new cycle.

## IBTC short-circuit

Per the AwN paper, 15.3% of real-world queries are Instructions Beyond Tool Capabilities. When the intent persona classifies a goal as IBTC, it emits `awn_class: IBTC` in intent.md and `gate_intent_to_research` short-circuits the cycle with a scope-rejection message. This saves Scout's budget for cycles that can actually proceed.

## ≥1 challenged_premise rule

`gate_intent_to_research` enforces that `challenged_premises` contains at least one entry. This is the kernel-level expression of Karpathy's #1 failure mode prevention: every goal contains assumptions; even "clear" goals need at least one to be surfaced and questioned. Source: arxiv 2303.08769 (Edward Chang on Socratic prompting), Princeton SocraticAI, MARS framework.

## Re-run and ledger-binding

Re-running `/intent` replaces the prior `intent.md`. The kernel reads the latest ledger entry of `kind=agent_subprocess, role=intent` for the cycle. This means:

- No special "revise" flag — re-running is the same as first running
- The latest SHA is what the kernel verifies — old SHAs are ignored
- Re-spawn within phase=intent is allowed by the existing precondition rule (L194-199 of phase-gate-precondition.sh)

## Default-off compatibility

Without `EVOLVE_REQUIRE_INTENT=1` at init, `cycle-state.intent_required` is `false` and:

- `gate_calibrate_to_intent` is a no-op pass
- `phase-gate-precondition.sh`'s scout-blocked-without-intent block does not fire (it gates on `intent_required==true`)
- Phase loop in orchestrator skips the intent step

26 existing dispatcher tests + 19 phase-gate-precondition tests + 16 cycle-state tests + 7 ship-integration tests must continue to pass. They do.

## Future directions (not in v0.1)

- **Multi-lens intent review** (CEO + User + Critic fan-out) — v0.2 if validated
- **Auto-promotion of `EVOLVE_REQUIRE_INTENT` from opt-in to default-on** — wait for ≥5 successful cycles using opt-in
- **Distribution AwN classifier** (`awn_classification: {imki: 0.6, imr: 0.1, ...}`) — gives downstream agents uncertainty info; chose single class for v0.1 simplicity
- **Intent quality scoring → lessons feedback** — track intents that turned out to be wrong (Scout discovered something different) and weight challenged_premise patterns by historical accuracy
- **`team-context.md` integration** — let intent persona post a "User layer" section to the team context bus so all downstream agents read it cleanly

## References

- `agents/evolve-intent.md` — persona
- `skills/evolve-intent/SKILL.md` — workflow
- `.claude-plugin/commands/intent.md` — slash command
- `.evolve/profiles/intent.json` — permission profile
- `scripts/phase-gate.sh` — `gate_calibrate_to_intent`, `gate_intent_to_research`
- `scripts/guards/phase-gate-precondition.sh` — scout-blocked-without-intent block
- `.evolve/research/intent-capture-patterns.md` — full research grounding (5 sources)
- `arxiv 2409.00557` — Ask-when-Needed framework (4 ambiguity types, AwN classifier)
- `Microsoft Agent Governance` — 4-layer intent model (organizational/role/developer/user precedence)
