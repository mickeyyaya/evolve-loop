# Model Routing Policy

This document is the source of truth for **abstract tier** selection by pipeline
phase. Phase profiles in `.evolve/profiles/*.json` carry the executable default
(`model_tier_default`); this policy explains why those defaults differ by phase.

## Driver-agnostic by construction

Routing is expressed in **capability tiers**, never vendor model names. A profile
names an abstract tier — `fast`, `balanced`, or `deep`
(`modelcatalog.CanonicalTiers`) — and the runtime translates that tier to a
concrete, CLI-native model id at dispatch via `modelcatalog.Lookup(cli, tier)`.

**Invariant: Claude must be replaceable by codex, agy, ollama, or any driver,
using an equal-capability model.** No phase may hard-code a vendor model. A
vendor name in a profile (e.g. `"sonnet"`) reaches *past* the tier abstraction
and silently fails to resolve for any non-Claude CLI
(`Lookup("codex","sonnet")` misses), so it is forbidden in `model_tier_default`,
`model_tier_overrides`, and `model_tier_envelope`.

### Per-driver model mapping — owned by `modelcatalog` (single source of truth)

The tier → concrete-model table is **not** restated here; it lives in
`go/internal/modelcatalog` (live-refreshed per CLI). The doc shows only an
illustrative projection of what that SSOT holds:

| Tier | claude | codex | agy | ollama |
|---|---|---|---|---|
| `fast` | haiku-class | small-reasoning | gemini-flash-class | small (gemma/phi) |
| `balanced` | sonnet-class | gpt-5.4-class | gemini-pro-class | mid |
| `deep` | opus-class | gpt-5.5-class | gemini-deep-class | large (phi4) |

> Illustrative only — the authoritative, current ids are whatever `modelcatalog`
> resolves at dispatch. A driver is eligible for a phase iff `modelcatalog` has
> that phase's tier populated for that CLI.

### Substitutability acceptance test

Pick any spine phase, point its profile `cli` at codex/agy/ollama, keep the same
tier — the pipeline still resolves a model. Enforced by
`TestSpineProfilesAreDriverAgnostic` (rejects any spine profile using a vendor
model name) plus `modelcatalog` per-CLI lookup tests.

## Routing Principles

- Use deterministic Go for fixed checks, state transitions, hashing, and gates.
- Use `fast` for ranking, categorization, and low-variance reading tasks.
- Use `balanced` for code generation, test authoring, and specification work.
- Use `deep` for critical quality gates and high-consequence orchestration.
- Keep profile-specific overrides for unusually complex, security-sensitive, or
  retry-heavy work — overrides are tiers too, never vendor names.

## Spine Phase Defaults

| Phase | Profile | Default tier | Rationale |
|---|---|---|---|
| scout | `.evolve/profiles/scout.json` | `balanced` | Discovery and task design need broad reasoning; the envelope still allows `deep`/`fast` escalation per cycle maturity. |
| triage | `.evolve/profiles/triage.json` | `fast` | Triage ranks and categorizes already-discovered work — a low-variance classification task. |
| tdd-engineer | `.evolve/profiles/tdd-engineer.json` | `balanced` | Test writing needs capability and consistency; `deep` remains available through explicit overrides for adversarial separation. |
| builder | `.evolve/profiles/builder.json` | `balanced` | Code generation is capability-sensitive and should not drop to `fast` by default; `deep` via overrides for complex/retry work. |
| auditor | `.evolve/profiles/auditor.json` | `deep` | Audit is the final quality gate before ship; consistency at the gate dominates latency. |
| router | `.evolve/profiles/router.json` | `deep` | Routing decisions can alter phase order and must favor consistency over latency. |
| reflector | `.evolve/profiles/reflector.json` | `fast` | Retrospective extraction is structured summarization after ship and can run cheaply. |

## Migration Status

Driver-agnostic tier vocabulary is enforced for the **7 spine phases above**
(default + overrides + envelope), guarded by `TestSpineProfilesAreDriverAgnostic`.

The ~48 domain/optional phase profiles (`api-contract-design`, `security-scan`,
`market-sizing`, …) and their overrides **still carry vendor model names** and
are migrating to canonical tiers via the evolve loop (operator note
`driver-agnostic-model-routing`). When that migration completes, widen the guard
test's `spine` set to the full fleet so the invariant covers every profile.

## Research Basis

The cycle-337 research synthesis mapped current agent practice to this rule:
LLMs should decide where judgment is needed, while deterministic pipeline code
should execute fixed validation and state transitions. Agent consistency matters
most at quality gates, so expensive tiers belong at audit and routing boundaries,
not routine categorization.

Three concrete findings from the cycle-337 scout report:

- **Consistency-aware routing:** reserve high-consistency models for critical
  workflows where divergent action sequences are costly.
- **Deterministic workflow bias:** move recurring format, verdict, and gate
  checks into Go instead of spending model turns on fixed mechanics.
- **Workload-router-pool pattern:** classify recurring agent work into stable
  buckets, then route each bucket to the cheapest tier that preserves quality.

## Verification

```bash
cd go && go test -count=1 ./internal/profiles/... ./internal/resolvellm/... ./internal/modelcatalog/...
```

`TestSpineProfilesAreDriverAgnostic` fails if any spine profile reintroduces a
vendor model name. Policy changes should also keep cycle-specific ACS predicates
green when a cycle pins model routing behavior.
