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
  "research": { "recall_k": 5, "novelty_threshold": 0.9 },

  // The ACS predicate lane (`evolve acs suite`, the audit). go_timeout_s is ONE
  // budget shared by every scope (absent / 0 ⇒ 60 s). predicate_env names the
  // operator-owned EVOLVE_ keys the suite copies from its environment into every
  // predicate's, past the scrub that drops the rest of the namespace. A lane
  // protocol key (ipcenv.ProtocolKeys) or a suite export (EVOLVE_PROJECT_ROOT,
  // the worktree-root key, CHANGED_PACKAGES) is refused with a warning in the
  // verdict; every other entry still forwards (ADR-0114).
  "acs": { "go_timeout_s": 600, "predicate_env": ["EVOLVE_FLAG_CAMPAIGN"] },

  // The inbox priority rank (ADR-0121). Decoded STRICTLY: an unknown key or an
  // invalid value fails the load. Absent ⇒ the compiled default shown here.
  "inbox_priority": {
    "class_order": ["correctness", "stability", "performance", "debuggability",
                    "feature", "maintainability", "hygiene", "security"],
    "factors": { "base": 0.45, "class": 0.2, "unblocks": 0.15,
                 "recurrence": 0.1, "age": 0.05, "goal": 0.05 },
    "age_halflife_days": 30,
    "unblocks_cap": 3,
    "recurrence_cap": 5,
    "active_campaigns": [],
    "preempt_margin": 0.05
  },

  // The convergence policy (ADR-0126). The convergence sub-block decodes
  // STRICTLY: an unknown key or an out-of-range number fails the load; an
  // unknown stage or bar word warns and resolves to its default. Absent ⇒ the
  // compiled default shown here. The other workflow keys stay lenient.
  "workflow": {
    "convergence": {
      "stage": "shadow",
      "max_fix_rounds": 3,
      "base_blocking_bar": "MEDIUM",
      "raised_blocking_bar": "HIGH",
      "concentration_threshold": 0.6,
      "concentration_window": 2,
      "concentration_min_findings": 5,
      "max_backward_edges": 3
    }
  }
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

## Inbox priority rank (`inbox_priority`)

The block configures `internal/inboxrank`, the one computed order over pending inbox items ([ADR-0121](adr/0121-inbox-priority-is-a-computed-rank.md); `evolve inbox rank` shows it). An item's score is `Σ factor × feature`, each feature in [0, 1].

| Key | Meaning | Default | Refused |
|---|---|---|---|
| `class_order` | the `priority_class` values, most urgent first; a class at position `i` of `n` scores `(n − i) / n`, one the order lacks scores 0. `evolve inbox add` refuses an item whose class it does not name. | correctness > stability > performance > debuggability > feature > maintainability > hygiene > security (operator correction, 2026-10-06) | empty, a blank or padded entry, a duplicate |
| `factors` | the weight of each feature: `base` (the item's weight), `class`, `unblocks`, `recurrence`, `age`, `goal`. A present block must name all six; 0 turns one off. | 0.45, 0.20, 0.15, 0.10, 0.05, 0.05 | a missing or unknown factor, a negative weight, all six 0 |
| `age_halflife_days` | the days after filing at which the age feature reaches 0.5 | 30 | 0 or less |
| `unblocks_cap` | the dependent count at which the unblocks feature saturates at 1 | 3 | below 1 |
| `recurrence_cap` | the recurrence count at which the recurrence feature saturates at 1 | 5 | below 1 |
| `active_campaigns` | the `campaign` values whose items get the goal feature | none | a blank, padded or duplicated entry |
| `preempt_margin` | the score margin by which a new item must beat the lowest uncommitted planned slot to take it at a wave boundary (read from the plan's P4; unused in P1) | 0.05 | negative |

The checked-in file names the class order and the factors explicitly, so the operator's weights live in config rather than in the compiled default. Since the plan's P2 (2026-10-06) the block orders the loop's work: the wave seed, the widen, the launch refill, the triage prompt's `inbox_batches` menu, `evolve inbox batches` and the dashboard's queue all read the rank it configures (loaded per evolve dir by `internal/inboxrank/rankinputs`), as well as deciding what `evolve inbox rank` shows and which classes `evolve inbox add` accepts. A malformed block makes those consumers rank with the compiled default and warn; `evolve inbox rank` and `evolve inbox add` refuse it. (Under P1, from the same morning, the block changed only the verb's output and the classes `add` accepts.)

## Convergence policy (`workflow.convergence`)

The block configures `internal/convergence`, the one rule every repeat-until-accepted loop uses to choose its next rung ([ADR-0126](adr/0126-every-iterative-loop-converges-or-escalates.md), [design](convergence-policy.md) §8; `evolve convergence decide` prints a decision). Absent means the compiled defaults below.

| Key | Meaning | Default | Refused or warned |
|---|---|---|---|
| `stage` | `shadow`: loops compute and signal the decision; `enforce`: loops act on it. The console follows the decision from the start. | `shadow` | an unknown word warns and resolves to `shadow` |
| `max_fix_rounds` | *N*: rung 1 for rounds 2 … *N*−1, rung 2 for the final round *N*, rung 3 after it; never a round *N*+1. A loop with its own budget (the audit-repair envelope, `TaskRetryCeiling`, `maxRecoveryDepth`) passes that instead. | 3 | below 1 fails the load |
| `base_blocking_bar` | the severity at or above which an OPEN finding blocks until the final round | `MEDIUM` | a word other than CRITICAL, HIGH or MEDIUM warns and resolves to `MEDIUM` |
| `raised_blocking_bar` | the bar from the final round on, for the loops that raise it (code-review, console lanes, the cycle) | `HIGH` | an unknown word warns and resolves to `HIGH`; a bar below the base warns and is held at the base |
| `concentration_threshold` | the share of a window's findings in one component that, on two consecutive windows, escalates that component | 0.6 | at or below 0, or above 1, fails the load |
| `concentration_window` | the judgments one concentration window spans | 2 | below 1 fails the load |
| `concentration_min_findings` | the findings (LOW or above) a window needs before its share is defined | 5 | below 1 fails the load |
| `max_backward_edges` | the cycle loop's *N*: the backward edges (audit → build, retro → tdd, ship → ship, …) one cycle may take before it stops with a continuation (design §6.1) | 3 | below 1 fails the load |

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
  a pinned phase keeps CLI-failure resilience. Since ADR-0104 (2026-09-14) every
  agent profile that names a `cli` MUST name a non-empty `cli_fallback`
  (`TestEveryAgentProfileHasAFallbackChain` guards the tracked tree; the five
  Claude-family-floor agents keep theirs in-family, `["claude-p"]`), and the
  universal tail (`workflow.universal_fallback`, the remaining available CLIs the
  profile's `allowed_clis` permits) is appended after it unconditionally — a
  strict single-CLI phase is `allowed_clis: ["<family>"]` plus a single-driver
  chain, not an empty one.
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
| `acs.predicate_env` | ACS suite (`internal/acssuite`: `forwardableKeys`, then `predicateEnv`) | each named key set in the suite's environment is copied into every predicate's; every other `EVOLVE_` key is stripped. Bounded in one place: a lane protocol key or one of the suite's own exports is refused and named in the verdict's `warnings` (at projection, not at `policy.Load`, so one bad entry neither fails every policy consumer nor drops the rest of the list). Read from the state root's policy only, so a cycle worktree cannot widen it; the checked-in list may name only keys a curated predicate reads |

Implementation: `go/internal/policy` (load + validate), consulted by
`go/internal/llmroute` (pin) and `go/internal/phases/runner` (load + bypass +
validate before dispatch).

`mandatory_phases` is applied uniformly via the shared `policy.MergeMandatory`
helper at **both** config-load sites — the loop's composition root
(`cmd_cycle.go`) and the per-phase `router.PolicyForProject` — so a
policy-mandatory phase is honored even by the self-skipping phases (triage,
tdd, build-planner) when they decide whether to run.
