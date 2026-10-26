# User Policy Configuration (`.evolve/policy.json`)

The **policy** layer is the user-controlled rule set that bounds the autonomous
pipeline. It is the *top authority*. It constrains what the routing advisor is allowed to
do and pins how individual phases dispatch. It ranks above the per-agent profile defaults
and even above operator env overrides.

It lives in a single user-owned, version-controllable file: `.evolve/policy.json`.
The file is **optional** — absent means "no user rules" (the advisor and the
dispatch resolver use their built-in defaults). A present-but-malformed file is
a hard error (a typo'd rule fails loudly rather than silently disabling the
policy).

The whole file decodes strictly (X18a, [llm-isolation.md](llm-isolation.md) §27).
A key that is not in the schema below fails the load, at any level. The
error names the file, the full key path and the fix, for example
`policy: parse .evolve/policy.json: unknown policy key "fleet.cuont": remove it or check its spelling`.
Three top-level metadata keys are allowed and have no effect: `$schema`,
`description` and `version`. The same words below the top level are unknown keys.

To fix a refused file, read the key path in the error. Then correct its
spelling against the schema below, or remove the key. An older release
ignored such a key, so the key had no effect before.

The loop refuses to start on a refused file, because the `cli-routing` preflight
check halts. A best-effort reader that runs mid-wave prints one
`[policy] WARN` line (`policy.LoadOrDefaults`). It then uses the compiled
defaults, but each safety floor fails closed to `enforce`: `docs_floor.stage`,
`docs_floor.ste_stage`, `comment_floor.stage` and `cover_changed.stage`.

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

  // The inbox curator (inbox-curator.md). Decoded STRICTLY. Absent => the
  // compiled default shown here: the shadow stage and the default kinds.
  "inbox": {
    "curator": {
      "stage": "shadow",
      "overlap_refuse": 0.5,
      "overlap_warn": 0.3,
      "consume_min_share": 0.5,
      "kinds": ["bug", "docs", "feature", "fix", "hardening", "maintenance",
                "pipeline-integrity", "pipeline-repair", "refactor", "research",
                "security", "sweep", "task", "techdebt", "test"]
    }
  },

  // The convergence policy (ADR-0126). The convergence sub-block decodes
  // STRICTLY: an unknown key or an out-of-range number fails the load; an
  // unknown stage or bar word warns and resolves to its default. Absent ⇒ the
  // compiled default shown here. An unknown key in any block fails the load.
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

Out-of-range values fall back to the built-in.
`recall_k: 0` silently disables advisor recall memory if the loader accepts it. `novelty_threshold: 0` suppresses
every lesson write if the loader accepts it. An operator cannot express either intent by accident.

**The recall default is held at 5 on purpose.** It is the value that the compiled
`research.maxResults` has always carried, so the new knob changes no
install's behaviour. A lower value narrows the failure recall of the advisor. That is a
phase-integrity regression, not a token optimisation.

### Why the novelty gate sits in the writer

A lesson id is `cycle-N-<scope>-<slug>`. So the same observation that recurs on a later
cycle lands under a *different* filename. The exact-path dedupe of `writeIfAbsent` cannot
see it. A corpus that repeats one failure for twenty cycles crowds recall with
twenty copies of it.

The gate therefore intercepts `faillearn.WriteArtifacts` itself
(the one Go lesson-write seam, reached from `cmd_loop_outcome.go`,
`core/failure_learning.go` and `core/reset.go`). It does not live in a helper that the
write path never calls.

Similarity is Jaccard overlap over the *observation-bearing* fields only —
`pattern`, `description`, `defects` and `failureContext`. `id`, `source` and
`preventiveAction` all embed the cycle number. If the gate included them, every
recurrence looks novel and the gate never fires. Pure-digit tokens are dropped for
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
| `preempt_margin` | the score margin by which a new item must beat the lowest uncommitted planned slot to take it at a wave boundary. (Read from the plan's P4; unused in P1.) | 0.05 | negative |

The checked-in file names the class order and the factors explicitly, so the operator's weights live in config rather than in the compiled default. Since the plan's P2 (2026-10-06) the block orders the work of the loop. These consumers all read the rank that the block configures (loaded per evolve dir by `internal/inboxrank/rankinputs`):

- the wave seed;
- the widen;
- the launch refill;
- the `inbox_batches` menu of the triage prompt;
- `evolve inbox batches`;
- the queue of the dashboard.

The block also decides what `evolve inbox rank` shows and which classes `evolve inbox add` accepts. A malformed block makes those consumers rank with the compiled default and warn. `evolve inbox rank` and `evolve inbox add` refuse it. (Under P1, from the same morning, the block changed only the output of the verb and the classes that `add` accepts.)

## Inbox curator (`inbox.curator`)

The block configures `internal/inboxcurator`, the deterministic checks of the inbox ([inbox-curator.md](inbox-curator.md); plan P3 in [the inbox prioritization plan](../plans/inbox-prioritization-2026-10.md)). `evolve inbox add` reads the thresholds and the kinds.

| Key | Meaning | Default | Refused |
|---|---|---|---|
| `stage` | `shadow` publishes the proposals of the curator only. `enforce` also applies each proposal whose deterministic check passes. The stage does not govern `evolve inbox add`: its intake always refuses. | `shadow` | a value other than `shadow` or `enforce` |
| `overlap_refuse` | the similarity at or above which `evolve inbox add` refuses an item that shares a fix surface with an open item | 0.5 | 0 or less, or above 1 |
| `overlap_warn` | the similarity at or above which `evolve inbox add` warns about such an item | 0.3, or `overlap_refuse` when that is lower | below 0, or above `overlap_refuse` |
| `consume_min_share` | the share of the named tests of an item that must exist before the curator proposes to consume it | 0.5 | 0 or less, or above 1 |
| `kinds` | the `kind` values that `evolve inbox add` accepts | the 15 kinds above | empty, a blank or padded entry, a duplicate |

The similarity is the token Jaccard of the title and the acceptance of two items. The live inbox of 2026-10-27 had 2,710 pairs of open items with a shared fix surface. The highest similarity was 0.52, and 13 pairs were between 0.3 and 0.5. These numbers set the defaults.

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

## Documentation floors (`docs_floor`)

The block sets the stages of the two documentation floors of the build handoff. The docs floor only WARNs. The STE floor fails the handoff at `ste_stage: enforce`. The loader decodes the block strictly. An unknown key, or a `stage` or `ste_stage` word that is not `0`, `off`, `shadow` or `enforce`, fails the load of the whole file. Thus a typo key in `docs_floor` stops every consumer of the policy, not only the two floors.

| Key | Meaning | Default |
|---|---|---|
| `stage` | the documentation floor for an architecture change without a document ([ADR-0077](adr/0077-docs-floor-for-architecture-changes.md)). `off` or `0` turns it off. `shadow` and `enforce` turn it on. | `enforce` |
| `ste_stage` | the STE lint of the changed documents ([internal-stelint](packages/internal-stelint.md), `evolve docs ste-lint`). `off` or `0` prints nothing. `shadow` prints the `[ste-lint] WARN:` line. `enforce` makes a finding on an added line a floor failure, which sends the document back to the build. The checked-in policy sets `enforce` (operator decision of 2026-10-21). | `shadow` |

## Changed-function coverage (`cover_changed`)

The block sets the stage of the changed-function coverage check of the build handoff ([plan](../plans/cover-changed-gate-2026-10.md), [internal-covchanged](packages/internal-covchanged.md)). The check runs the tests of the changed packages with coverage. Then it refuses each changed function below 100%. A test failure in that run refuses at `enforce`, and a plumbing fault only warns. A word that is not `off`, `shadow` or `enforce` turns the check off with a WARN.

| Key | Meaning | Default |
|---|---|---|
| `stage` | `off` skips the check. `shadow` prints the `[build-floor] WARN cover-changed (shadow)` line. `enforce` rejects the handoff. | `enforce` |

## Recovery retrospectives (`retrospectives`)

The block sets the stage of the retrospective documents that the cycle closeout writes ([spec](recovery-retrospective.md), [plan](../plans/recovery-retrospective-2026-10.md)). The loader decodes the block strictly. A word that is not `off`, `shadow` or `enforce` turns the writer off with a WARN. The writer is not a safety floor, so a refused policy does not move it to `enforce`.

| Key | Meaning | Default |
|---|---|---|
| `stage` | `off` writes no document. `shadow` writes the documents into the cycle workspace. `enforce` does the same until plan C4 to C6 add the publish and the todo retirement. The checked-in policy sets `enforce` (operator decision of 2026-10-27). | `shadow` |

## Context-fill telemetry (`context_fill`)

The token telemetry of every launch carries a derived **fill reading**. It is the prompt-side
tokens that the resolver already recovered (`Input + CacheRead + CacheWrite`), divided by the effective context
window of the driver family, on a 0–100 scale. Generated
output does not sit in the prompt.

| Aspect | Behaviour |
|---|---|
| Effective window | claude family (`""`, `claude`, `claude-*`) → 200 000. Any family whose window has not been measured → 0. |
| Unmeasurable reading | An unconfigured window, or a launch no telemetry tier observed, yields the negative sentinel `tokenusage.FillPctUnmeasured` — never `0`, never `Inf`/`NaN`. "Unmeasured" and "0% full" must not read alike. |
| WARN | Emitted at the dispatch seam (`internal/bridge.Engine.recordTokenUsage`) **strictly above** the threshold, carrying the `CONTEXT-FILL` marker and naming the phase. The sentinel never warns. |
| Persistence | Each `llm-calls.ndjson` record carries `fill_pct`, so the reading is durable rather than only printed. |

The 200 000 claude window is deliberately conservative, below any advertised
maximum. The operationally useful signal is proximity to *degradation*,
not to the hard ceiling. Windows for unmeasured families are left at 0 on purpose.
A guess of one is not published, because it is a fabricated fill reading.

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
  a pinned phase keeps CLI-failure resilience.
  - Since ADR-0104 (2026-09-14) every
    agent profile that names a `cli` MUST name a non-empty `cli_fallback`.
    `TestEveryAgentProfileHasAFallbackChain` guards the tracked tree. The five
    Claude-family-floor agents keep theirs in-family, `["claude-p"]`.
  - The
    universal tail (`workflow.universal_fallback`, the remaining available CLIs that the
    `allowed_clis` of the profile permits) is appended after it unconditionally.
  - A
    strict single-CLI phase is `allowed_clis: ["<family>"]` plus a single-driver
    chain, not an empty one.
- `pin.model` replaces the resolved model verbatim, bypassing the
  env/profile/default chain **and** the `"auto"` → model-catalog expansion (a
  pinned exact model never triggers a catalog lookup).

### Candidate-chain construction (single authority)

The CLI candidate chain behind both dispatch entry points is built by ONE
function — `buildCandidates(primary, prof, excludeProfileCLI)` in
`go/internal/llmroute/dispatch.go`. Common behaviour:

- The primary is first.
- Then `profile.cli_fallback` follows, whitespace-trimmed, with empties dropped. The first occurrence wins.
- The result always holds at least the primary (`Dispatch` fails loudly
  on an empty chain).

`excludeProfileCLI` is the only difference between the two callers. It is a
parameter, not a second copy of the loop (cycle-1265 collapsed the former
`candidatesFrom`/`chainCandidates` pair):

| Caller | `excludeProfileCLI` | Why |
|---|---|---|
| `llmroute.Resolve` | `false` | a pinned or env-forced primary keeps the profile's chain intact, so the phase retains CLI-failure resilience (see the `pin.cli` bullet above) |
| `llmroute.ChainFor` | `true` | `prof.CLI` names the CLI the composition root deliberately swapped away from (for example, the advisor routed around a benched family). If the chain re-appends it as a "fallback", the route walks straight back into it |

### Guardrails

A pin is validated against the phase profile's guardrails at dispatch:

- `pin.cli`'s family must be within the profile's `allowed_clis` (unless
  `allowed_clis` is empty or `["all"]`).
- `pin.model`'s tier (classified from the model identifier — for example
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
| `pins[phase]` | dispatch resolver (`internal/llmroute`) | absolute CLI/model override, validated with `policy.ValidatePin` |
| `acs.predicate_env` | ACS suite (`internal/acssuite`: `forwardableKeys`, then `predicateEnv`) | each named key set in the suite's environment is copied into the environment of every predicate. Every other `EVOLVE_` key is stripped. Bounded in one place: a lane protocol key or one of the suite's own exports is refused and named in the `warnings` of the verdict. The refusal happens at projection, not at `policy.Load`, so one bad entry does not fail every policy consumer or drop the rest of the list. The suite reads the policy of the state root only, so a cycle worktree cannot widen it. The checked-in list is allowed to name only keys that a curated predicate reads |

Implementation: `go/internal/policy` (load + validate), consulted by
`go/internal/llmroute` (pin) and `go/internal/phases/runner` (load + bypass +
validate before dispatch).

`mandatory_phases` is applied uniformly with the shared `policy.MergeMandatory`
helper at **both** config-load sites: the composition root of the loop
(`cmd_cycle.go`) and the per-phase `router.PolicyForProject`. So even the self-skipping phases (triage,
tdd, build-planner) honor a policy-mandatory phase when they decide if to run.
