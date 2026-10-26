# Model Routing Policy

This document is the source of truth for **abstract tier** selection by pipeline
phase. Phase profiles in `.evolve/profiles/*.json` carry the executable default
(`model_tier_default`); this policy explains why those defaults differ by phase.

## Driver-agnostic by construction

Routing uses **capability tiers**, never vendor model names. A profile
names an abstract tier — `fast`, `balanced`, or `deep`
(`modelcatalog.CanonicalTiers`). At dispatch, the runtime translates that tier to a
concrete, CLI-native model id with `modelcatalog.Lookup(cli, tier)`.

**Invariant: Claude must be replaceable by codex, agy, ollama, or any driver,
using an equal-capability model.** Phases must not hard-code a vendor model.

A vendor name in a profile (for example, `"sonnet"`) reaches *past* the tier abstraction
and silently fails to resolve for any non-Claude CLI
(`Lookup("codex","sonnet")` misses). The policy forbids it in `model_tier_default`,
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
tier — the pipeline still resolves a model. Enforced by:

- `TestAllProfilesSubstitutabilityAtParity` — the definitive all-profiles parity
  guard. It iterates all profiles with `loader.List()`. It checks `model_tier_default`,
  every `model_tier_overrides` value, and every non-empty `model_tier_envelope`
  field (min/default/max) against each swappable bridge manifest. It uses real
  manifests (not a synthetic fixture). It uses `t.Errorf`, so any missing tier is a
  hard failure, not a silent skip.
- `TestAllProfileDefaultTiersResolveForSwappableDrivers` — companion check that
  all profiles' default tiers resolve across all swappable driver manifests
  (codex, codex-tmux, agy, agy-tmux, ollama-tmux).
- `TestSpineProfilesAreDriverAgnostic` — rejects any spine profile using a vendor
  model name.

**`allowed_clis` dispatch exceptions:** Three profiles carry intentional
`allowed_clis` restrictions that limit runtime driver selection: `builder`
(claude + codex), `tdd-engineer` (claude only), and `tester` (claude only).
These encode cross-family invariants (TDD ≠ builder floor, adversarial-audit
floor) — not tier-vocabulary violations. The tier vocabulary
(`fast`/`balanced`/`deep`) remains fully driver-agnostic for all profiles;
`allowed_clis` constrains *dispatch eligibility*, not tier resolution.

## Effort for each tier

The reasoning effort is a second dial beside the tier. It lives in the same central file as the CLI chains: `.evolve/policy.json` `cli_routing`.

- `cli_routing.tiers.<tier>.effort` sets the effort of a tier.
- `cli_routing.agents.<agent>.effort` sets the effort of one agent at every tier.
- With no entry, the compiled default applies: fast `low`, balanced, deep and top `medium`.

The operator directive of 2026-10-08 is: "Based on the opus 5.5, the effort should be configured starting with medium effort". A higher level comes only after a measured gain. A profile carries no effort: the fields `effort_level` and `effort_overrides` are retired. Write the effort with `evolve cli-routing set ... --effort <level>`. See [model-routing.md](../reference/model-routing.md#effort-for-each-tier).

## Family order by quota headroom

The plan [wave-latency-2026-10](../plans/wave-latency-2026-10.md) (D4, W4) adds a rule that sorts the chain of one agent. It is for the builder seat. The research found that an agy build takes about 2.4 times the model time of a claude build (F3.3). It also found that the Claude week window was 14% used on 2026-10-19 (F3.5). Thus claude first is faster while the window has headroom, and agy first is safer when the window is low.

The rule is policy data on the agent rule:

| Key | Values | Default | Effect |
|---|---|---|---|
| `cli_routing.agents.<agent>.order` | `headroom` | absent: the policy order | Sorts the `cli` chain of the rule at each dispatch. |
| `cli_routing.agents.<agent>.headroom_pct` | 1 to 100 | 60 | A family whose week window is at or above this percent leaves the head of the chain. |

With `order: headroom`, the router does these steps for each dispatch of the agent:

1. A family whose week window is at or above `headroom_pct` goes to the tail. It stays in the chain as a fallback.
2. The other families go in the order of their lane time to a PASS over their last 20 builds (W0). A family without a measure goes after the measured ones.
3. With no measure, the order is claude, agy, agy-claude.
4. A quota exit keeps the present behavior: the cli-health bench puts the family in its cooldown.
5. When the usage read fails, the policy order stays. The router logs the reason.

The week use comes from the usage windows that the pre-wave usage probe records. The measure comes from `.evolve/lane-time.json`, which the loop writes once at each wave boundary. `evolve cli-routing explain builder` prints both as `note:` lines. The design and its limits are in [internal-cliroute.md](packages/internal-cliroute.md#the-headroom-order-w4-2026-10-19).

With `order: headroom`, the builder can start on claude, the family of the auditor and the tdd engineer. `Compile` reports this as a `cross_family_with` warning. The checked-in `.evolve/policy.json` does not set `order` yet. The operator applies D5 (W3) and then this rule at a wave boundary.

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
| tdd-engineer | `.evolve/profiles/tdd-engineer.json` | `balanced`, with effort `high` in `cli_routing` `agents.tdd-engineer` since 2026-10-18 | The tdd phase writes the acceptance tests that grade the builder. In cycles 1946 and 1950, both phases ran Sonnet 5.5 at medium effort, so the tests and the code had the same blind spots. Since the burn window ended, the builder runs agy first (Gemini 3.8 Flash). Thus Sonnet in the tdd seat is another family than the builder. The operator set tdd to Sonnet at high effort on 2026-10-18. When the builder falls back to Claude Code, both seats are Sonnet again. |
| builder | `.evolve/profiles/builder.json` | `balanced`, pinned in `cli_routing` `agents.builder` since 2026-10-18 | The operator asked to run the build on Gemini 3.8 Flash again. In cycle 1951, the router raised the build to `deep`, where agy serves Gemini 4 Argon. The Argon week window was 100% used, so each build try exited 85 and fell back to Claude Code. The pin keeps the build at `balanced`: agy first, on Gemini 3.8 Flash (High). A `deep` build now needs an explicit change of this pin. Since 2026-10-19 (W4), `agents.builder` can also take `order: headroom`, which the checked-in table does not set yet ([Family order by quota headroom](#family-order-by-quota-headroom)). |
| auditor | `.evolve/profiles/auditor.json` | `deep` | Audit is the final quality gate before ship; consistency at the gate dominates latency. |
| router | `.evolve/profiles/router.json` | `deep` | Routing decisions can alter phase order and must favor consistency over latency. |
| reflector | `.evolve/profiles/reflector.json` | `fast` | Retrospective extraction is structured summarization after ship and can run cheaply. |

## Migration Status

Driver-agnostic tier vocabulary is enforced for the **7 spine phases above** by
`TestSpineProfilesAreDriverAgnostic` and for the full `.evolve/profiles/*.json`
fleet by `TestAllProfilesAreDriverAgnostic`. Both guards cover defaults,
overrides, and tier envelopes.

The domain/optional phase profile migration is complete: profile defaults and
overrides now use only `fast`, `balanced`, and `deep`. The per-driver concrete
model remains owned by `modelcatalog`, keeping profiles driver-agnostic at
parity across Claude, Codex, agy, and other supported bridges.

## Research Basis

The cycle-337 research synthesis mapped current agent practice to this rule:
It is recommended that LLMs decide where judgment is needed. It is also recommended that deterministic pipeline code
executes fixed validation and state transitions. Agent consistency matters
most at quality gates. Expensive tiers belong at audit and routing boundaries,
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
vendor model name. It is recommended that policy changes also keep cycle-specific ACS predicates
green when a cycle pins model routing behavior.
