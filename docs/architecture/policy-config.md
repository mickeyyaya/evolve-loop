# User Policy Configuration (`.evolve/policy.json`)

The **policy** layer is the user-controlled rule set that bounds the autonomous
pipeline. It is the *top authority*: it constrains what the routing advisor may
do and pins how individual phases dispatch, above the per-agent profile defaults
and even above operator env overrides.

It lives in a single user-owned, version-controllable file: `.evolve/policy.json`.
The file is **optional** — absent means "no user rules" (the advisor and the
dispatch resolver use their built-in defaults). A present-but-malformed file is
a hard error (a typo'd rule fails loudly rather than silently disabling the
policy).

## Schema

```jsonc
{
  // Phases the routing advisor may NEVER drop from a cycle. Merged into the
  // orchestrator's mandatory set. (The non-configurable integrity floor —
  // ship ⇒ build ∧ audit — always applies on top, regardless of this list.)
  "mandatory_phases": ["scout", "build", "audit", "ship"],

  // Hard per-phase dispatch pins, keyed by PHASE name. Each pin may set "cli",
  // "model", or both. An empty field means "no pin for that dimension".
  "pins": {
    "audit": { "cli": "claude-tmux", "model": "claude-opus-4-8" }
  },

  // Context-fill telemetry: warn when a launch's prompt-side tokens occupy more
  // than this percentage of its driver family's effective context window.
  // Absent / empty / out-of-range (<=0 or >100) ⇒ 60.
  "context_fill": { "warn_threshold_pct": 60 },

  // KB recall bound + failure-lesson novelty gate. Absent / empty /
  // out-of-range ⇒ recall_k=5 (today's compiled bound), novelty_threshold=0.9.
  "research": { "recall_k": 5, "novelty_threshold": 0.9 }
}
```

## KB recall and lesson novelty (`research`)

Two knobs over the lessons corpus (`.evolve/instincts/lessons`), both resolved by
`policy.Policy.ResearchConfig()` (`go/internal/policy/research_config.go`).

| Key | Range | Default | Effect |
|---|---|---|---|
| `recall_k` | 1–50 | **5** | How many lessons one KB lookup returns — the top-k **prefix** of the existing deterministic ranking, not a resample. Resolved at the composition root (`go/cmd/evolve/cmd_cycle.go`, `kbRecallK` → `research.NewFileKBWithRecall`) and consumed by the advisor's recall memory (`Orchestrator.recallForPlan`). |
| `novelty_threshold` | (0, 1] | **0.9** | Similarity at or above which an incoming deterministic failure lesson counts as a near-duplicate of one already on disk and is skipped (`faillearn.WithNoveltyThreshold` → `go/internal/faillearn/novelty.go`). |

Out-of-range values fall back to the built-in rather than being honoured: `recall_k: 0`
would silently disable advisor recall memory and `novelty_threshold: 0` would suppress
every lesson write, and neither is an intent an operator can express by accident.

**The recall default is held at 5 on purpose.** It is the value the compiled
`research.maxResults` has always carried, so introducing the knob changes no
install's behaviour; lowering it narrows the advisor's failure recall, which is a
phase-integrity regression, not a token optimisation.

### Why the novelty gate sits in the writer

A lesson id is `cycle-N-<scope>-<slug>`, so the same observation recurring on a later
cycle lands under a *different* filename — `writeIfAbsent`'s exact-path dedupe cannot
see it, and a corpus that repeats one failure for twenty cycles crowds recall with
twenty copies of it. The gate therefore intercepts `faillearn.WriteArtifacts` itself
(the one Go lesson-write seam, reached from `cmd_loop_outcome.go`,
`core/failure_learning.go` and `core/reset.go`) rather than living in a helper the
write path never calls.

Similarity is Jaccard overlap over the *observation-bearing* fields only —
`pattern`, `description`, `defects` and `failureContext`. `id`, `source` and
`preventiveAction` all embed the cycle number, so including them would make every
recurrence look novel and the gate would never fire. Pure-digit tokens are dropped for
the same reason. Two hard rules bound the gate, both pinned by tests:

- a materially different failure is **never** suppressed (suppressing evidence is
  irreversible, so anything short of an almost token-identical match is written);
- corpus rot is **inert** — an unparseable neighbour is skipped, never treated as a
  reason to drop the incoming lesson, and never rewritten or deleted.

A suppressed lesson is not an error, and the failing cycle's own
`retrospective-report.md` is written either way.

## Context-fill telemetry (`context_fill`)

Every launch's token telemetry carries a derived **fill reading**: the prompt-side
tokens the resolver already recovered (`Input + CacheRead + CacheWrite` — generated
output does not sit in the prompt) divided by the driver family's effective context
window, on a 0–100 scale.

| Aspect | Behaviour |
|---|---|
| Effective window | claude family (`""`, `claude`, `claude-*`) → 200 000. Any family whose window has not been measured → 0. |
| Unmeasurable reading | An unconfigured window, or a launch no telemetry tier observed, yields the negative sentinel `tokenusage.FillPctUnmeasured` — never `0`, never `Inf`/`NaN`. "Unmeasured" and "0% full" must not read alike. |
| WARN | Emitted at the dispatch seam (`internal/bridge.Engine.recordTokenUsage`) **strictly above** the threshold, carrying the `CONTEXT-FILL` marker and naming the phase. The sentinel never warns. |
| Persistence | Each `llm-calls.ndjson` record carries `fill_pct`, so the reading is durable rather than only printed. |

The 200 000 claude window is deliberately conservative — below any advertised
maximum — because the operationally useful signal is proximity to *degradation*,
not to the hard ceiling. Windows for unmeasured families are left at 0 on purpose:
guessing one would publish a fabricated fill reading.

## Pin semantics (dispatch)

A pin is **absolute** — it overrides the entire normal resolution chain:

```
precedence (high → low):
  policy.pins[phase]          ← absolute (this file)
  EVOLVE_<AGENT>_CLI / _MODEL  (operator env)
  llm_config.json / profile    (defaults)
  built-in default
```

- `pin.cli` replaces the resolved primary CLI (dispatch log shows
  `source=policy.pin`). The profile's `cli_fallback` chain is still appended, so
  a pinned phase keeps CLI-failure resilience — empty `cli_fallback` in the
  profile if you want a strict single-CLI phase.
- `pin.model` replaces the resolved model verbatim, bypassing the
  env/profile/default chain **and** the `"auto"` → model-catalog expansion (a
  pinned exact model never triggers a catalog lookup).

### Candidate-chain construction (single authority)

The CLI candidate chain behind both dispatch entry points is built by ONE
function — `buildCandidates(primary, prof, excludeProfileCLI)` in
`go/internal/llmroute/dispatch.go`. Common behaviour: primary first, then
`profile.cli_fallback` whitespace-trimmed, empties dropped, first occurrence
wins, and the result always holds at least the primary (`Dispatch` fails loudly
on an empty chain).

`excludeProfileCLI` is the only difference between the two callers, and it is a
parameter rather than a second copy of the loop (cycle-1265 collapsed the former
`candidatesFrom`/`chainCandidates` pair):

| Caller | `excludeProfileCLI` | Why |
|---|---|---|
| `llmroute.Resolve` | `false` | a pinned or env-forced primary keeps the profile's chain intact, so the phase retains CLI-failure resilience (see the `pin.cli` bullet above) |
| `llmroute.ChainFor` | `true` | `prof.CLI` names the CLI the composition root deliberately swapped away from (e.g. the advisor routed around a benched family); re-appending it as a "fallback" would walk straight back into it |

### Guardrails

A pin is validated against the phase profile's guardrails at dispatch:

- `pin.cli`'s family must be within the profile's `allowed_clis` (unless
  `allowed_clis` is empty or `["all"]`).
- `pin.model`'s tier (classified from the model identifier — e.g.
  `claude-opus-4-8` → deep) must sit within the profile's `model_tier_envelope`.

An out-of-guardrail pin **hard-fails the phase loudly** rather than silently
breaching the trust-kernel constraints. (Model-tier validation is best-effort
for model identifiers the tier classifier can't rank; this hardens once the
live model catalog provides authoritative model→tier mapping.)

### Escape hatch

`EVOLVE_POLICY_BYPASS=1` skips policy entirely for a run (pins ignored, normal
resolution applies). Routine use defeats the purpose of a guardrail — reserve it
for emergencies.

## Enforcement points

| Rule | Consulted by | Mechanism |
|---|---|---|
| `mandatory_phases` | routing advisor | merged into the orchestrator mandatory set; `ClampPlanToFloor` keeps them in every cycle plan |
| `pins[phase]` | dispatch resolver (`internal/llmroute`) | absolute CLI/model override, validated via `policy.ValidatePin` |

Implementation: `go/internal/policy` (load + validate), consulted by
`go/internal/llmroute` (pin) and `go/internal/phases/runner` (load + bypass +
validate before dispatch).

`mandatory_phases` is applied uniformly via the shared `policy.MergeMandatory`
helper at **both** config-load sites — the loop's composition root
(`cmd_cycle.go`) and the per-phase `router.PolicyForProject` — so a
policy-mandatory phase is honored even by the self-skipping phases (triage,
tdd, build-planner) when they decide whether to run.
