# Model-level benches: bench (family, model), not only the family (design note, 2026-10-06)

> Status: **proposed; not built.** Filed as a follow-up of the usage-window reader ([internal-usageprobe](../architecture/packages/internal-usageprobe.md)). Inbox item: `model-level-benches-from-per-model-usage-windows`.

## The gap

Every `/usage` screen the probe reads now becomes typed `UsageWindow`s through the CLI's manifest (`controls.usage.windows`). Each window has a scope, a kind, a percentage used and a reset. A window's scope maps to what a bench would act on:

| Window | Scope | Bench target today |
|---|---|---|
| claude `Current session`, `Current week (all models)` | the account | the `claude` family, until the reset |
| agy `GEMINI MODELS`, `CLAUDE AND GPT MODELS` | a quota pool | the `agy` and `agy-claude` families |
| claude `Current week (Fable)` (one per model claude meters on its own) | **one model** | **none.** The probe logs a `WARN` and records the window in `.evolve/usage-windows.json` |

The clihealth bench is keyed by routing family (`llmroute.Family`), so it can only take a whole CLI out of a chain. A drained Fable week leaves Claude Code's other models usable. Benching `claude` for it would push every claude phase, including the auditor floor, to fallbacks with no need. Not benching means a phase dispatched to the drained model fails at run time, and the reactive wall classifier then benches the whole family anyway.

## Proposal

Bench at **(family, model)** granularity and demote only the tiers that resolve to that model.

1. **A model-scoped entry in the bench store.** `clihealth.Entry` gains `Model`, and the store keys such an entry by `family + "\x00" + model`, a separator no family name contains. `Active()` keeps returning family benches unchanged. A new `ActiveModels()` returns the model benches. The canary skips model entries until step 4. No routing effect yet.
2. **The usage probe writes them.** A per-model window (`UsageWindow.Model != ""`) that is exhausted becomes `BenchModelUntil(family, model, reason, evidence, reset)`, in place of today's WARN. The WARN stays as the log line.
3. **The router demotes tiers, not CLIs.** The resolver already turns `(cli, tier)` into a concrete model: the policy pin, then the live catalog, then the manifest's `model_tier_map`.
   - A model bench marks each `(cli, tier)` candidate whose resolved model **matches** the benched model as benched, and leaves the CLI's other tiers in place.
   - A match needs a declared name map, because a screen names a model by its display word (`Fable`, `Opus`) while the tier map holds ids (`Claude Opus 5.5 (High)`, `claude-opus-5-5`). The window spec gets `model_match`, a manifest-declared regex per scope word, for example `{"Opus": "(?i)opus"}`. An unmatched word benches nothing and keeps the WARN, which is fail-open.
   - **Example.** Claude Code's Opus week drains. The claude deep and top tiers resolve to Opus, so they are demoted in each chain. The next candidate for deep is `agy-claude-tmux` deep (`Claude Opus 5.5 (High)`), the same model on agy's separate quota pool. claude's balanced (Sonnet) phases are untouched.
4. **The canary clears a model bench with a model probe.** It runs `doctor live <family>-tmux --model <model>` (the `--model` flag exists since 2026-10-06). A clean probe clears the model bench. A wall re-benches it.
5. **Operator surfaces.** `evolve clihealth list` shows model benches as `family/model`. `evolve clihealth usage` already shows the per-model windows. Preflight's `cli-health` check lists them.

**Engineering bar.** Each step is its own component and commit with its own red-first tests, unwired before it is wired. Steps 1–2 change nothing in routing; step 3 is the behaviour change.

## Tests the steps name

- `TestStore_AModelBenchIsNotAFamilyBench` (1)
- `TestProber_AnExhaustedPerModelWindowBenchesThatModelOnly` (2)
- `TestApplyModelBench_DemotesOnlyTheTiersThatResolveToTheBenchedModel` and `TestApplyModelBench_AnUnmatchedScopeWordBenchesNothing` (3)
- `TestCanary_AnExpiredModelBenchIsProbedWithItsModel` (4)

## Open questions

- **Opus on the screen.** claude's per-model rows are named by the screen (`Fable` today). Which models get their own row varies with the plan. The map must stay data, never a Go list.
- **Which pool is binding.** agy's Claude group is one pool for Opus, Sonnet and GPT-OSS, so its scope is a family, not a model. Only a screen that meters one model on its own produces a model bench.
- **Reset trust.** A model bench should keep the 24 h cap the family bench has (`clihealth` `resetHintCap`).
