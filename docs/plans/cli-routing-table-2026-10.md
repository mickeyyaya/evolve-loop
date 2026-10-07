# CLI routing table — design and landing plan (2026-10-05)

> Status: approved by the operator 2026-10-05. L1 landed 2026-10-06; L2 built 2026-10-07 (see "L2 landing notes"); L3 open. Implementation tracked by ADR-0119. This file is the design record; the ADR states the decision.


## Context

The operator subscribes only to **Claude Code and agy**, with no codex. Today:
- 75 of the 101 profiles name `codex-tmux` as primary.
- Each of those dispatches boots codex, hits the wall and benches it for 24 hours, then lands on Claude.
- Claude is the quota-constrained family: the 2026-09-02 directive is pinned by `profiles/family_floor_test.go`, and Claude's weekly limit stopped wave 58.

The operator asked for an **easy configured option**, set in one place, that says which CLIs do which work and which fallback policy to follow.

The deep review found three main things:
- **None of today's controls can express that.**
- **Today's controls disagree with each other.** The table below shows the gaps.
- **Many launch paths bypass the controls.** Two exploration agents traced eleven, and they are listed under "One resolver" below.

| Today's control | What's wrong |
|---|---|
| `policy.json` `pins` | **Keys don't match.** Setup writes them by role (`setup/apply.go:36`) and reads them back the same way (`setup.go:203`), but the runner reads them by **phase** (`runner/routing.go:39`). So `pins.builder` never applies.<br>**A pin is a lock.** It switches off the probe, bench and fallback tail, and it also drops the advisor's tier raise.<br>**Many launches ignore pins** (see the list below). |
| Profile `cli` / `cli_fallback` / `allowed_clis` | **Not operator-owned.** These are protected files, and switching to "agy, then Claude" would take 75 edits.<br>**One profile is already inconsistent.** `tester` has `cli=codex-tmux` but `allowed_clis=[claude]`. |
| `workflow.universal_fallback(_exclude)`, `router.cli/model` | **Partial.** They cover only the last-resort tail and the advisor. |
| `EVOLVE_*_CLI`, `--cli` | **Unguarded.** Nothing checks them against the floor, so `--cli auditor=codex-tmux` runs. |
| Loop preflight (`looppreflight/drivers.go:26 distinctDrivers`) | **Halts on codex whatever the config says.** It builds its driver list from profiles only. |
| `setup detect` / `setup recommend` | **Wrong on a claude+agy host.**<br>`detect` reports codex as SUBSCRIPTION because a stale auth file exists.<br>On a claude+agy host, `chooseCrossFamilyPair` (`setup/recommend.go:210`) quietly picks Claude for both builder and auditor. |

The outcome should be:
- **One block** in `.evolve/policy.json` says which CLIs exist, which CLI does each kind of work, and what happens when one fails.
- **One resolver** applies that block on **every** launch path.
- **The integrity floor still holds:** the Claude-family floor, and builder ≠ auditor.
- **`evolve cli-routing`** shows and edits it.

**Why not the minimal option.** Design A repairs `pins` and adds a `"*"` wildcard. It would take about 2 days, but it has two problems:
- It keeps four overlapping sources (pins, the tail keys, `router.*`, env) and eight separate chain builders that share only a lookup function. That is the class defect the inbox item `one-cli-resolver-for-phases` already records.
- It can't say "this kind of work goes to this CLI" without listing profile names.

Its discoveries are included below.

## The config (`.evolve/policy.json`)

The operator decided the starting table on 2026-10-05:
- **Every non-floor phase runs agy first,** with Claude as the fallback. This includes the builder, at its balanced tier, on **Gemini 3.8 Flash (High)**.
- **Deep and top tiers run on Claude Code Opus only.** This holds until Gemini 4 Pro is released. Gemini 3.1 Pro is never used.
- **The floor agents stay on Claude.**

```json
"cli_routing": {
  "clis":    ["agy", "claude"],
  "default": ["agy", "claude"],
  "tiers":   { "deep": ["claude"], "top": ["claude"] },
  "after_chain": "other_clis"
}
```

What that table does in practice:
- **The builder needs no override.** Its default tier is balanced, so it runs on agy (Gemini 3.8 Flash High).
- **Builder escalations go to Opus.** The builder's own tier overrides (`m_complex_5plus_files`, `ultrathink_strategy`, `audit_retry_2plus` → deep) and any advisor tier raise move that dispatch to Claude Opus.
- **Capacity cost, stated so it isn't a surprise.** 28 profiles default to deep, and all of them move to Claude Opus.
  - The spine ones run every cycle: intent, plan-reviewer, retrospective and the router. The router runs on agy (Gemini 3.1 Pro) today.
  - The others run when invoked: debugger, failure-advisor, failure-adjudicator and swarm-planner, plus 17 analytic phases.
  - agy keeps the balanced and fast work: scout, triage, build-planner, build, memo and most scans.
  - `show` prints the split. If Claude's weekly limit binds, the operator has one lever per agent, for example `set agents.router agy --model balanced`. The plan does not pull it by default.
- **Switching to Gemini later is config only.** When Gemini 4 Pro ships, run `evolve models refresh` (agy's deep and top pick it up), then `evolve cli-routing set tiers.deep agy,claude` and the same for `top`. No code change is needed.

| Key | Meaning |
|---|---|
| `clis` | The CLIs this operator has. A family not listed is never dispatched or preflighted by any path. |
| `default` | The chain for anything no other rule covers. |
| `work.<role>` | Optional. The chain for every phase of one Role: `plan`, `build`, `evaluate` or `control`. The Role comes from `phasespec.Role`, through the spec's `role` field or `RoleOrDefault()` (`internal/phasespec/phasespec.go:145`). The operator never lists profile names, and a new phase classifies itself. |
| `agents.<name>` | Per-agent override: an array, or `{cli, model}` when it also sets a tier. A phase name (`audit`) normalizes to its agent (`auditor`) through `PhaseSpec.AgentName()`, so the key bug can't recur. |
| `tiers.<tier>` | Optional ceiling, keyed by `fast`, `balanced`, `deep` or `top`. A dispatch running at that tier may use only the listed CLIs, whichever rule chose its chain. |
| `after_chain` | What happens after the chain runs out:<br>`other_clis` (the default) tries the remaining `clis` in their listed order;<br>`stop` ends the walk.<br>This replaces `workflow.universal_fallback(_exclude)`. |

- **Names:** chain entries are family names (`claude`, `agy`, `codex`, `ollama`), mapped through `llmroute.defaultDriverForFamily` (`llmroute.go:107`). A driver name such as `claude-p` is also accepted.
- **Rule precedence:** `agents` > `work` > `default` > (with no `default`) the profile's own `cli` + `cli_fallback`, filtered by `clis`.
- **The floor needs no config.** The floor agents (auditor, adversarial-review, tdd-engineer, spec-verifier, spec-verify) are clamped to Claude automatically, so `"default": ["agy","claude"]` already leaves them on Claude.

### Semantics

- **Allowed set:** `A(agent) = clis ∩ profile.allowed_clis ∩ ({claude} if agent is on the floor)`.
  - Chains from `default` and `work` are filtered by `A` silently, and `show` prints the filtering as a note.
  - An explicit `agents.<name>` entry that falls outside `A` is an error, because the operator asked for something that can't run.
  - A chain that is empty after filtering is an error.
- **An assignment is a chain, not a lock.** The probe, the 24-hour bench and `after_chain` still apply, so a walled agy falls through to Claude.
- **The tier ceiling filters each (CLI, tier) attempt.** It applies inside the existing walk over `Candidates` × `TierChain` (`llmroute.go:54–66`), and it covers the `after_chain` tail too.
  - The tier is the one `resolveModel` and `TierChain` produce, after profile overrides, the advisor's tier raise and `agents.<n>.model`.
  - With `tiers.deep = [claude]`, the (agy, deep) attempt is never made.
  - If Claude is walled during a deep dispatch, the walk steps down the existing tier chain. At balanced it may then run on agy (3.8 Flash High).
  - If nothing is left, the walk ends as capacity (a quota-pause, under the T2 rule), not as a failure.
- **Advisor overlay, per dimension.**
  - An assignment fixes the CLI chain, which outranks the advisor's CLI choice, just as a pin does today.
  - The advisor may still raise the tier, unless the assignment sets `model`.
- **Env and `--cli`.** They stay as per-run overrides, but a value outside `A` is an error. `evolve cycle` and `evolve loop` exit 2 before dispatch, which closes the `--cli auditor=…` bypass.
- **Absent block.** A *legacy projection* of today's sources (profile cli/fallback, phase-keyed pins as locks, `universal_fallback(_exclude)`, `router.cli/model`) is pinned byte-identical by a golden test.
- **Two sources are refused.** If the block is present and `pins`, `workflow.universal_fallback*` or `router.cli/model` are also set, the result is a load error. It names the key and points to `evolve cli-routing migrate`.

### Integrity checks (`Compile` reports every finding, not only the first)

- **Strict decode:** unknown keys inside the block are rejected through `DisallowUnknownFields`, following the precedent in `internal/campaign/campaign.go`.
- **Known names only:**
  - families and drivers: known (`llmroute.KnownDriver`, `chain_shared.go:55`) and tool-capable (`bridge.HasToolUse` behind a seam);
  - roles: one of the four;
  - tiers: one of `fast`, `balanced`, `deep` or `top`, with entries that are a subset of `clis`;
  - a floor agent's tier must keep Claude reachable, so a tier ceiling that excludes Claude at the auditor's deep tier is an error;
  - `agents` keys: a tracked profile or a phase in the merged catalog.
- **Floor:** `claude` must be in `clis`. The floor list moves out of `family_floor_test.go` into exported `profiles.ClaudeFamilyFloor`, so production code and the test share one list.
- **Cross-family:** for each `cross_family_with` pair (builder↔auditor, tdd-engineer↔builder), the two resolved primaries must be different families. This is an error when `clis` has two or more families, and a WARN for a Claude-only install. A fallback that shares a family is a WARN in `show`. `CrossFamilyWith` gets added to `profiles.Profile`.
- **Where the checks run:** in `evolve cycle`/`loop` start (exit 2), in the preflight `cli-routing` check, in `cli-routing check|set`, and in `setup apply`.

## One resolver: new package `go/internal/cliroute`

```go
func Compile(p policy.Policy, cat Catalog, profs ProfileSource) (Table, []Finding)
func New(t Table, h Host) *Router
func (r *Router) Resolve(req Request) (Decision, error)
```

- **`Host` seams:** `LookPath`, `Discover`, `Bench`, `ToolCapable` and `Logf`. `cliroute` imports only policy, profiles, phasespec, llmroute, clihealth and envchain, never core or bridge, so no import cycle.
- **`Decision`:** `Plan`, `Rule` (e.g. `work:build`, `agents:builder`, `legacy:profile`), `Allowed`, `Trace`, and `Allows(cli)`.
- **Stages:** rule → guarded env → overlay (CLI, then tier) → probe → bench → `after_chain` → `A` filter → tier ceiling → floor check. Each stage is a function of under 50 lines.
- **Reuse:** `cliroute` composes `llmroute.Resolve` (`llmroute.go:54`), `ApplySoftOverlay`, `ApplyUniversalFallback` (`llmroute.go:165`) and `AllowedDiscovered` (`chain_shared.go:15`), and doesn't reimplement them.
- **One Router per process,** built in `wireOrchestratorDeps` (`cmd/evolve/cmd_cycle.go`).

Every launch path calls `Resolve`:

| Launch path | Change |
|---|---|
| Runner `resolveDispatchPlan` (`phases/runner/routing.go:26`) | Calls `Resolve`; its own pin, probe, bench and tail blocks are deleted. |
| `bridgechain.DefaultPlanResolver` (`bridgechain.go:147`) | Covers retro, failure-advisor, judge, adjudicator and swarm. `CallerCLI` (today's `leadWith`) applies only when no rule covers the agent. `PlanResolver` now also returns an error, so a bad rule fails instead of silently falling back (14 references in 3 files). |
| `resolvellm.Resolve` | Covers `evolve subagent run`, `resolve-llm`, `failureAdvisorOpts` and `setup detect`. It goes through `Resolve`, so these honour the table. |
| `retro.resolveCLI` | Deleted; `retro.Config.Router` is injected instead, so the skill overlays and logs name the CLI that actually runs. |
| Advisor router (`resolveRouterDispatch*` in `cmd_cycle.go`, `advisor/launch.go:57`) | `Resolve{Agent:"router"}`. The benched swap walks the resolved chain instead of a hard-coded `claude-tmux`. |
| Contract escalation (`core/contract_escalation.go:118,139`) | Picks the next different-family candidate that `Decision.Allows`. |
| `profileForModelRouting` (`core/cyclerun.go:878`) | Uses the `<phase>.json` lookup, which closes the clamp gap that covered only 10 phases. |
| Loop preflight (`looppreflight/drivers.go:26`) | Takes `distinctDrivers` from the resolved chains, so it boots only `agy-tmux` and `claude-tmux`. Adds a `cli-routing` halt check. |
| `setup recommend/apply` (`setup/recommend.go:210`, `apply.go:36`) | `apply` writes `cli_routing.agents`; `chooseCrossFamilyPair` reads `clis` and reports when no pair exists. |
| Model classifier (`cmd/evolve/cmd_models_live.go`: `liveRefresh` → `tierClassifier` → `pickClassifierCLI` → `bridgePromptDispatcher`; added in L1b from inbox `model-classifier-resolves-its-cli-through-the-routing-table`) | `Resolve{Agent: "model-classifier", Launch: classifier}`; its families, in chain order, filtered to the ready CLIs. The legacy projection is today's codex > claude > agy; `classifierCLIPreference` is deleted. |

A source-scan guard, `TestOnlyCliRouteBuildsChains`, bans every chain builder outside `cliroute`; the ban set as built (the five `llmroute` builders, `resolvellm.Resolve`, the dot-import refusal and, until L2 removed it, the one allowlist entry) is listed in [ADR-0119](../architecture/adr/0119-one-routing-table-one-resolver.md) Decision 2. It follows the ADR-0104 precedent, so no future launch path can bypass the table.

## Operator verbs: `evolve cli-routing`

`evolve routing` already exists and explains a recorded decision, so the new verbs get a new noun.

| Verb | What it does |
|---|---|
| `show [--static] [--json]` | Prints, for each agent: the rule, the chain in walk order, the model at each tier (e.g. `balanced: agy Gemini 3.8 Flash (High)`, `deep: claude opus`), and notes for FLOOR, ceiling filtering, cross-family and host health. Agents that share a chain are grouped. |
| `check` | Exit codes: 0 when clean, 1 when there are findings, 2 on usage error. |
| `explain <agent>` | Explains how one agent's chain was resolved. |
| `init --clis agy,claude` | Writes a block that compiles: `clis` plus `default` in that order. |
| `set default agy,claude`<br>`set work.plan claude,agy`<br>`set tiers.deep claude`<br>`set agents.router agy --model fast`<br>`set clis agy,claude`<br>`set after_chain stop` | Edits one entry. |
| `unset <key>` | Removes one entry. |
| `migrate [--dry-run]` | Moves `pins`, `universal_fallback*` and `router.cli/model` into the block. |

- **Writes:** every write validates the merged file first, then uses the setup lossless atomic writer, moved to `policy.PatchBlock`.
- **Refusals:** a write is refused inside a phase (dispatch depth > 0) or while a cycle lease is held.
- **After a write:** it prints `ship with: evolve ship --class manual (at a wave boundary)`.
- **`setup detect`:** it reads `clis`, so an undeclared family is no longer reported as SUBSCRIPTION.

## Landing sequence

Each item below is one component, with its own commit and red-first tests. Wiring comes after the unwired pieces. Each landing goes in at a wave boundary through a console train.

**L1: table, resolver and every launch path (checked-in policy unchanged; behaviour byte-identical)**
1. **Pin today's behaviour.** Generate the legacy golden first, from current code: all 101 profiles × {no env, `EVOLVE_CLI`, per-agent env, phase pin, advisor overlay, benched family, missing binary}.
2. **Policy schema:** the strict `cli_routing` decode, exported `profiles.ClaudeFamilyFloor`, and `CrossFamilyWith`.
3. **`cliroute` itself:** `Compile` with its findings, the legacy projection, `Resolve`, and `Router`. Still not wired to anything.
4. **Wire every launch path in the table above,** together with the source-scan guard, so the `clis` filter holds everywhere at once. Compile once at the composition root.
5. **Read-only verbs:** `cli-routing show|check|explain`; ADR-0119 "One routing table, one resolver" (amends ADR-0029 and ADR-0104 §2–3); `docs/architecture/packages/internal-cliroute.md`.

**L2: write verbs and the operator's table**
6. **Write verbs:** `set|unset|init|migrate`, plus the guard on env and `--cli`. The intentional changes go in the release notes: a disallowed env primary now fails, and a CLI-only assignment keeps the advisor's tier raise.
7. **Profile fixes (protected, `--class manual`):**
   - `builder.json` `allowed_clis` adds `agy`. The operator decided on 2026-10-05 that the builder builds on agy at balanced and escalates to deep on Claude Opus.
   - `tester.json` gets its ceiling and primary made consistent.
   - Cross-family: the builder's primary (agy) differs from the auditor's (claude). Deep builder escalations share Claude with the auditor; `show` reports that as an accepted WARN, not an error.
7b. **agy models move to Gemini 3.8 Flash.** fast becomes `Gemini 3.8 Flash (Low)` and balanced becomes `Gemini 3.8 Flash (High)`; `agy models` lists both today.
   - The live catalog `.evolve/model-catalog.json` is stale on 3.7, because `catalog.refresh_stage` is `shadow`. Refresh it with `evolve models refresh`, then check it with `evolve models list`.
   - The offline baseline `model_tier_map` in `go/internal/bridge/manifests/agy-tmux.json` moves to 3.8 in the same commit, together with the CHANGELOG value record.
   - agy's deep and top entries stay on Gemini 3.1 Pro (High), which is unreachable while `tiers.deep/top = [claude]`.
   - Claude's catalog is already deep/top = opus.
8. **The operator's table lands:** `evolve cli-routing init --clis agy,claude` plus `set …`, shipped with `--class manual`. `TestTheCheckedInPolicyPutsAgyInTheLastResortTail` becomes `TestTheCheckedInTableCompiles` and `TestTheCheckedInTableRoutesNoLaunchToCodex`.
9. **Close superseded inbox items:** `operator-declares-cli-substitutes` (this design replaces it) and `one-cli-resolver-for-phases`. Retire the paused `dev/cl-clisubst` lane and its worktree.

**L3: retire the legacy keys**
10. **Delete the old keys:** `pins`, `workflow.universal_fallback*`, `router.cli/model` and `router.plan_model/propose_model`. Their presence becomes a load error that points to `migrate`. (The per-decision models joined this list in the L1b fix round: `decisionModel` lays them over the advisor's decision in both modes; L1b refuses them beside a declared block, and `migrate` must move them into `agents.router` or a decision-aware rule.)
11. **Alias:** `resolve-llm` becomes an alias of `cli-routing show --json`.
12. **Docs:**
    - `docs/operations/runtime-reference.md`: one precedence table replaces the list of 24 mechanisms;
    - `policy-config.md`, `internal-llmroute.md` and `internal-policy.md`;
    - the CLAUDE.md routing bullets and the `skills/evo:setup` text.

## Critical files

- **Modified:**
  - Routing: `go/internal/phases/runner/routing.go`, `go/internal/bridgechain/bridgechain.go`, `go/internal/llmroute/llmroute.go` (mechanics only), `go/internal/resolvellm/`.
  - Policy and profiles: `go/internal/policy/{policy.go,core.go,workflow.go}`, `go/internal/profiles/`.
  - Commands: `go/cmd/evolve/cmd_cycle.go` and `registry.go`.
  - Launchers: `go/internal/core/{contract_escalation.go,cyclerun.go}`, `go/internal/phases/retro/retro.go`, `go/internal/advisor/launch.go`.
  - Preflight and setup: `go/internal/looppreflight/drivers.go`, `go/internal/setup/{setup.go,apply.go,recommend.go}`.
  - Config (L2): `.evolve/policy.json`, `.evolve/profiles/{builder,tester}.json`, `.evolve/model-catalog.json` (through `evolve models refresh`), `go/internal/bridge/manifests/agy-tmux.json` (`model_tier_map` fast/balanced).
- **New:**
  - `go/internal/cliroute/` (`compile.go`, `resolve.go`, `legacy.go`, `findings.go` and tests);
  - `go/cmd/evolve/cmd_cli_routing.go`;
  - `docs/architecture/adr/0119-*.md`;
  - `docs/architecture/packages/internal-cliroute.md`.
- **Reused:** `phasespec.PhaseSpec.RoleOrDefault()` and `AgentName()`, the phase catalog loader that `wireOrchestratorDeps` already uses, `policy.BaseCLI` and `TierRank`, the `setup/apply.go` `parseExistingPolicy`/`encodePolicy` lossless writer, and the `clihealth` bench.

## Verification

- **Red first:**
  - `TestApply_WritesAgentKeyedRulesTheRunnerReads` (red today, because `pins.builder` never applies);
  - `TestResolve_EnvOverrideOutsideAllowedFails`;
  - `TestDistinctDrivers_FollowsTheResolvedChains`;
  - `TestEveryProfileChainIsWithinAllowedCLIs` (red on `tester`).
- **`cliroute` tests:**
  - `Compile` refuses: a floor agent leaving Claude, builder and auditor collapsing into one family, an unknown agent, a role or family outside the known sets, an unknown key, and two sources at once. It also reports every finding, not only the first.
  - `Resolve`: the absent table matches the legacy golden; `work.<role>` applies by phase Role; `agents` beats `work` beats `default`; an assignment keeps probe, bench and `after_chain`; `stop` ends the walk; the advisor can raise the tier but can't change the CLI.
  - Tier ceiling:
    - `TestResolve_DeepAndTopNeverRunOnAgy`: this includes the `after_chain` tail and an advisor raise to deep.
    - `TestResolve_BuilderBalancedOnAgyEscalatesToClaudeAtDeep`: this uses the real `builder.json` overrides.
    - `TestResolve_WalledClaudeAtDeepStepsDownNotToAgyDeep`.
    - `TestCompile_TierCeilingExcludingClaudeAtAFloorTierIsAFinding`.
- **Call-site tests:**
  - `TestDefaultPlanResolver_DeclaredTableOutranksCallerCLI`
  - `TestRetro_DispatchesTheResolverPrimary`
  - `TestContractEscalation_PicksFromTheTable`
  - `TestRouterDispatch_ComesFromTheTable`
  - `TestFailureAdvisor_ComesFromTheTable`
  - `TestSubagentRun_HonoursTheTable`
  - `TestPreflight_CliRoutingHaltsOnAFinding`
- **Existing routing tests stay green in L1:**
  - `TestResolve_Pin*`, `TestRun_PolicyPin_*`, `model_routing_overlay_test`, `universal_fallback_*`
  - `bridgechain_test`, `contract_escalation_target_test`, `cmd_router_dispatch_test`, `setup/apply_test`
  - `family_floor_test`, `deep_tier_family_arrangement_test`, `fallback_chain_test`, `routing_order_realtree_test`
- **Full floor at every landing:** `make test test-integration test-e2e test-acs-durable apicover-enforce cover-strict` (with `-count=1`), plus CI-parity gofmt.
- **Live checks after L2:**
  - `evolve cli-routing show` lists no `codex` on any chain.
  - `evolve cli-routing check` exits 0.
  - `evolve setup detect` no longer claims a codex subscription.
  - Preflight boots only agy and Claude.
  - In the next wave's log, every `[runner] … cli=` line shows `agy-tmux` or `claude-tmux`, with `source=default`, `work:<role>` or `agents:<name>`, and the floor agents show `claude-tmux`.
  - The builder dispatches as `agy-tmux` with `Gemini 3.8 Flash (High)`.
  - Every deep or top dispatch shows `claude-tmux` with opus.
  - `grep "Gemini 3.1 Pro"` over the wave's dispatch lines finds nothing.
  - The builder's ship rate on agy is watched over the first two waves. If it regresses, roll back with one command: `evolve cli-routing set agents.builder claude,agy`.
  - Claude's share of dispatches drops, compared with wave 60's.

## L1a landing notes (2026-10-05)

L1a (items 1–3) is built, tested and unwired. The package notes are [internal-cliroute.md](../architecture/packages/internal-cliroute.md). Where the code forced a departure from the text above, the smallest one was taken:

| Plan text | As built | Why |
|---|---|---|
| `New(t, h) *Router` | `New(t, h) (*Router, error)`, refusing a table with an error finding; the findings are stored on the `Table` | Review round 1 (N5): a table with a defect must never route. |
| `ToolCapable` is a `Host` seam | `Compile(p, cat, profs, cliroute.WithToolCapable(fn))` | The tool-capability check is a `Compile` finding, and `Compile` takes no `Host`. The three-argument call still compiles; L1b passes `bridge.HasToolUse`. |
| `cliroute` imports policy, profiles, phasespec, llmroute, clihealth, envchain | adds `phasecontract`; does not need `clihealth` | `PhaseSpec.AgentName()` returns `evolve-build`, `evolve-audit` and `evolve-tdd` for the built-ins, because the registry leaves `agent` empty for them, so `agents.audit` could not reach `auditor`. The built-in contracts name the agent. `phasecontract` imports only `phasespec` and `failurelog`. The bench arrives as a seam, so `clihealth` is not imported. |
| `Catalog` | `Get(name)` and `Names()` | `Compile` checks every agent under each role its phases have, which needs the phase list. `phasespec.Catalog` already has both. |
| "all 101 profiles", "75 of the 101" | 92 tracked profiles, 73 codex-primary | The tracked tree on 2026-10-05. |
| Golden variants: 7 | 11: the 7, plus a model-only pin, a caller CLI, the checked-in `universal_fallback_exclude: []` and `--bypass-policy` | Each exercises a branch of today's resolvers that the 7 do not (a pin that locks the model but not the CLI; `bridgechain`'s `leadWith`; the tail with agy admitted; bypass ignoring the pins but keeping the tail). 92 profiles × 11 × 2 resolvers = 2024 records. |
| Stages end with "floor check" | the floor is part of the allowed set `A` | `A` already clamps a floor agent to `{claude}` before and after the tail, so a separate check could never fire. |
| `Decision.Rule` examples | `agents:<agent>`, `work:<role>`, `default`, `profile`, `legacy:pin`, `legacy:env`, `legacy:caller`, `legacy:default` or `legacy:profile`; in declared mode `Plan.PrimarySource` is the rule | The runner's existing `source=` log field then shows the rule, as the live checks after L2 expect. |
| Legacy projection includes `router.cli/model` | not part of `Resolve` | Today they feed only the advisor's own dispatch (`resolveRouterDispatch*` in `cmd_cycle.go`), which picks a CLI and a model but builds no plan. L1b decides how the advisor path calls `Resolve{Agent: "router"}`; the two-sources finding already covers the keys. |

Found while pinning the golden: today an advisor overlay or an `EVOLVE_<AGENT>_CLI` value can put a CLI outside `allowed_clis` first on a chain (the builder gets `agy-tmux`), and the bridge-chain resolver ignores pins and the advisor overlay. The legacy projection keeps both; the declared mode refuses or ignores them. And the operator's table compiled against today's profiles reports `cross_family_with.auditor+builder` as an error until item 7 widens `builder.json`.

### Review round 1 (2026-10-05)

The architecture review returned FIX_THEN_MERGE, and the Go review passed with notes. Every item was fixed red-first, and each fix is pinned by a test that fails on a mutant of it:

- **C1 — the walk.** A fully excluded walk no longer reports `Walled`. The consumer half is L1b decision 5.
- **W1 — one rule predicate.** `Table.selectRule` returns the selection or a typed `RuleError` (`RuleLeak`, `RuleEmpty`), shared by `Compile` and `Resolve`.
- **W2 — the role of a phase-less launch.** Recorded as L1b decision 8.
- **W3 — bypass.** Decision 1 is revised, and a `bypass` golden variant is added. Its 184 records were added; the 1840 existing records stayed byte-identical.
- **W4 — the profile snapshot.** Recorded as L1b decision 9.
- **W5 — a floor agent's override tier.** The override tier of a floor agent is checked against the ceiling.
- **Go review.** The ceiling check uses `Plan.Model` when `Tiers` is empty, as the walk does, which is stricter than the suggested early return: a deep model on agy is still refused.
- **Nits:**
  - no flag argument, and at most four parameters per function;
  - one allows-family predicate (L1b decision 10), with the `after_chain` tail built from the allowed set;
  - one tier vocabulary, `policy.TierNames()`;
  - a deep-cloned ceiling;
  - `New` refuses error findings;
  - a `null` block or agent rule is refused;
  - `profiles.IsClaudeFamilyFloor`.

Reviewer mutants re-run on the fixed code: M9 (floor override tiers dropped) is killed, and so is the C1 pair (a nothing-permitted walk marked walled; an observed wall forgotten).

### Review round 2 (2026-10-05)

The re-review confirmed all eleven round-1 findings fixed: the golden byte-identical and its seven mutants killed. It raised one WARNING of the W2 class: under `{clis: [agy, claude], default: [agy], tiers: {deep: [claude], top: [claude]}, after_chain: stop}`, `Compile` reported nothing for `intent`, yet `Resolve` refused it at the tier ceiling. Fixed red-first:

- **One ceiling predicate.** `Table.ceilingRefusal` returns a typed `CeilingError` (rule, tiers tried, chain, ceiling) for `Resolve`. `Compile` runs it on each selection's static plan: the selected chain plus the `after_chain` tail unless `stop` (one `Table.withAfterChain` for both), along the tier chain of `agents.<agent>.model` when set, otherwise of the default tier and each `model_tier_overrides` tier, each built by `llmroute.ApplySoftOverlay` as `Resolve` builds it. It is keyed `agent.<agent>.tier_ceiling`.
- **The floor tier check reads the same tier list.** `agents.<agent>.model` now fixes it, as at dispatch.
- **The property test covers every refusal type and matches only keys.** The probe table is one of its four tables.
- **Mutants run and killed:**
  - `Compile` skips the ceiling check (killed by the property test, the probe test and the override test);
  - `Compile` checks only the default tier (killed by the floor-override and ceiling-override tests);
  - `reportsAgent` loosened back to message `Contains` (killed by `TestReportsAgent_MatchesKeysNeverMessages`).
- **Unchanged.** The operator's table still compiles with only the accepted warnings against the widened-builder fixture, and the golden is unchanged (`ace6dca0…`).

## L1b decisions (console, 2026-10-05)

L1a's lane raised seven questions for the wiring step (L1b, item 4). The answers below were decided before L1b starts.

| # | Question | Decision | Why |
|---|---|---|---|
| 1 | `--bypass-policy`: today the runner skips pins under `req.BypassPolicy`, and `Request` has no such field | The `Router` holds two compiled tables, built once at the composition root:<br>• `declared`, from the loaded policy;<br>• `bypass`, from `Compile(policy.Policy{Workflow: loaded.Workflow}, …)`: the legacy projection of the loaded policy without its pins or `cli_routing` block, keeping its `workflow` (revised in review round 1, 2026-10-05).<br>`Request.BypassPolicy` picks the table, so no stage ever branches on the flag. | Bypass keeps today's meaning: it ignores the pins but not the universal tail, because the composition root reads `workflow.universal_fallback*` whatever the flag says. The `bypass` golden variant records that today (pins ignored, agy kept in the tail under `universal_fallback_exclude: []`), and `cliroute` reproduces it through the table above. Ignoring operator policy never means ignoring the profile ceilings. Whether the floor guard on env values also binds under bypass is an L2 decision (item 6). |
| 2 | Is policy loaded once per process or on every dispatch? | Once per process. `evolve cycle`, `evolve loop` and `evolve subagent run` each compile at start. A malformed `policy.json` or any error finding exits 2 before the first dispatch, replacing today's per-phase FAIL. ADR-0119 states this. | `policy.json` is protected and changes only at wave boundaries through `ship --class manual`, and every wave is a fresh `evolve loop` process. Failing at start beats failing mid-cycle, and the routing is identical for every phase of a run. |
| 3 | How does the advisor's router path (`resolveRouterDispatch*`) map onto `Resolve`? | It calls `Resolve{Agent: "router"}`.<br>• **Legacy projection:** in L1b it takes on `router.cli` (primary) and `router.model` (model) as the router agent's rule. New golden variants cover these keys, set and unset.<br>• **Benched swap:** instead of the hard-coded `claude-tmux`, it takes the first unbenched candidate of `Decision.Plan.Candidates`. For today's router chain (`agy-tmux`, then `claude-tmux`), that gives the same result as now. | One resolver for every launch. The two-sources finding already refuses `router.*` alongside a declared table. |
| 4 | Should the registry carry `agent` for build, audit and tdd? | Not in this change. `cliroute` keeps the `phasecontract` mapping. An inbox item is filed for the duplicate phase→agent sources: the registry's `agent` field and the built-in contracts. | That is a registry SSOT change touching `phaseinventory`, and it is outside the routing scope. |
| 5 | How does the runner handle a walk that is fully excluded? | **The walk (done in L1a):** `DispatchTiered` sets `Walled` only when a wall is known, which means a launch in the walk met a quota wall. A permitted benched family is never skipped: the bench demotes, never removes, so its wall is observed by a launch. A walk the ceiling empties launches nothing and returns `ErrNoPermittedAttempt` with `Walled` false.<br>**The consumers (L1b):** a walled result means capacity: the existing WallKeeper and quota-pause path, under the T2 rule. `ErrNoPermittedAttempt` without `Walled` means `Compile` missed a finding. That is a system failure: FAIL loudly with the rule and the ceiling in the reason. Today neither walker reads `walk.Err` on an empty walk: `bridgechain.Walking.Launch` (`bridgechain.go:137`, `keeper.Surface(walk, last, lastErr)` with no launch behind `last`) and the runner's dispatch (`phases/runner/dispatch.go:79`) both read the zero response as success. The reviewers' probe got 0 launches, exit 0 and a nil error. L1b makes both surface `walk.Err` when `Attempts` is empty, with red-first tests. | Capacity is not a defect, and an unreachable chain is. |
| 6 | Which discovery list does `Host.Discover` get? | The raw doctor list; `cliroute` applies the exclusion itself. L1b deletes the `llmroute.ExcludeFamilies` call from `cmd/evolve/universal_fallback_tail.go:20`, so the exclusion has one home. | One owner for the exclusion rule. |
| 7 | When is the golden regenerated in L2? | In the same commit as the `builder.json` and `tester.json` edits, with `-update`. The commit message lists which plans moved. | The diff is the review artifact. |
| 8 | What role does a launch with no phase get? (review round 1, W2) | Settled in L1a: in declared mode, an empty `Request.Phase` takes the agent's role when every phase the agent serves has one role (tester: evaluate; retrospective: `retro` and `retrospective`, both control); otherwise it has no role, and `work` does not apply. `Compile` checks every selection `Resolve` can make: each role of the agent's phases, plus the no-role selection whenever that selection is reachable. The legacy projection keeps today's meaning: no phase means the bridge chain's semantics, with no pin. | Before this, the tester resolved with no `Compile` finding on the no-phase path, and the retrospective got two different chains depending on the caller. |
| 9 | When are profiles read? (review round 1, W4) | Once, at `Compile`. `Resolve` never re-reads them. In declared mode a listed profile that fails to load is an error, never a nil profile. A nil profile would widen the allowed set to every CLI. The legacy projection keeps the nil, for byte-identity. **L1b consequence:** a profile minted mid-cycle (`phaseregistrar`) is invisible to a table compiled at start. The catalog publisher that re-binds the contract resolver after a mint (`WithCatalogPublisher`) must also recompile the table and swap the `Router`. | One snapshot makes a run's routing consistent, and it makes the `Compile` checks cover exactly what `Resolve` will see. |
| 10 | Where else is "the profile allows this family" decided? (review round 1, N2) | `(*profiles.Profile).AllowsFamily` and `AllowedFamilies` are the one predicate, and `cliroute` uses them. Three older homes remain as migration targets:<br>• `llmroute.AllowedDiscovered` (`chain_shared.go:15`), the legacy universal tail, which L3 retires with the legacy keys;<br>• `policy.ValidatePin` (`policy/core.go:149`), the legacy pin check, also L3;<br>• `setup`'s `allowedBaseSet` (`setup/recommend.go:236`), which moves to `AllowsFamily` when L2 rewires `setup recommend/apply`. | Each copy is a place where the floor or a profile ceiling could drift. |

## L1b landing notes (2026-10-06)

L1b (items 4 and 5) wires every launch path in the "One resolver" table through `cliroute` and adds the read-only verbs and [ADR-0119](../architecture/adr/0119-one-routing-table-one-resolver.md). The checked-in `policy.json` is unchanged, so every launch still resolves through the legacy projection. The package notes ([internal-cliroute.md](../architecture/packages/internal-cliroute.md), "Wiring") list each call site.

**The goldens.** Both were generated from the pre-L1b code before any routing change, then replayed through the wired paths:

- `legacy-plans.golden.jsonl` gains the `router_keys` variant (a policy that sets `router.cli`, `router.model` and `router.plan_model`): 184 records proving the runner and the bridge chain ignore the advisor's keys. The 2024 earlier records are byte-identical (`ace6dca0…` → `80630cbd…`, 2208 records). The generator now injects a `Router` into the runner and calls `DefaultPlanResolver(router)`.
- `legacy-advisor.golden.jsonl` (new, `902579a8…`, 96 records: 12 variants × 4 bench states × the plan and propose decisions) pins the advisor's `{cli, model, healthy}` through `resolveRouterDispatchHealthy`.

**Where the code forced a choice the text left open:**

| Plan text | As built | Why |
|---|---|---|
| "`resolvellm.Resolve` … goes through `Resolve`" | `(*Router).ResolveRole(role, resolvellm.Options)`: legacy = `resolvellm.Resolve` exactly, declared = `Resolve{Agent: role}`'s primary, model and rule | Those launches take one CLI and a tier, not a chain, and `resolvellm` searches its own roots (plugin, project, git). Calling `Resolve` in legacy mode would have added env, probe and bench to four paths that never had them. |
| Advisor: "`Resolve{Agent: "router"}`; the legacy projection takes `router.cli`/`router.model`" | `Request.Launch = LaunchAdvisor` selects a legacy advisor projection: profile `cli` (else `claude-tmux`), then `router.cli`; profile tier (else `DefaultModel`, `opus`), then `router.model`; the chain is the profile chain followed by `claude-tmux`, today's swap target. No env, no probe. A declared table ignores `Launch`. | Today's advisor reads neither env nor PATH, so the phase-less projection (env, probe, tail) would not have been byte-identical. Ending the legacy chain with `claude-tmux` makes "the first unbenched candidate" (decision 3) equal today's swap for every router profile without a `cli_fallback`. |
| Advisor walk (`advisor/launch.go:57`) | unchanged: `ChainFor(identity.CLI, profile)` | With the tracked `router.json` (no `cli_fallback`) the walk is the chosen CLI alone in both modes; walking the resolved chain would give the advisor fallbacks it lacks today. Open for L2. |
| Model classifier: "the routing table's chain for agent `model-classifier`, keeping today's order as the legacy projection" | `Request.Launch = LaunchClassifier`, agent `cliroute.ClassifierAgent`; legacy = `codex-tmux, claude-tmux, agy-tmux`; `classifierCLIPreference` is deleted | `model-classifier` is not a profile, so it resolves by the `default` rule in a declared table. |
| `--bypass-policy` (decision 1) | `Build(Setup)` compiles both tables; `Request.BypassPolicy` picks one; the runner reads skill overlays from `Router.Policy()`, or none under bypass, as before | Policy is loaded once per process; bypass never branched inside a stage. |
| Decision 5, the consumers | one rule, `bridgechain.Unlaunched` (sentinel `ErrUnlaunched`), used by both walkers; the runner's `Run` returns a FAIL with the error (`unlaunchedFailure`), as it does for a routing refusal | The two walkers already shared `WallKeeper`; the empty walk is its sibling. |
| Decision 9 | `catalogPublisher(br, router)` calls `Router.Recompile(catalog)` after re-binding the contract resolver; a refused recompile WARNs and keeps the start table | A mint changes the profile list; the swap is atomic. |
| Runner without the root's router | `Options.Router`, else `runner.DefaultRouter` (set by `wireOrchestratorDeps`), else a per-launch legacy router over the request's `policy.json` and `cliroute.SingleProfile{prep.profile}` | The same set-once package default `DefaultUniversalFallback` used; the fallback keeps today's routing for tests and non-root callers. The retro and contract escalation do the same with an empty policy. |
| Contract escalation | candidates from the decision for (agent, phase) (probe, bench and tail included), allowance by `Decision.Allows`; the universal `claude-tmux` fallback is appended only when `Decision.Legacy()` | A declared `after_chain: stop` must not be overridden by a hard-coded fallback. |
| Loop preflight | `Options.Routing` (compiled by `defaultLoopPreflight` with no discovery host): drivers are the union of each agent's phase-less resolved chain; check `cli-routing` halts on a table that does not compile | Without discovery the legacy set equals today's profile chains, plus `claude-tmux` for a profile with no `cli` and the CLI a set `EVOLVE_CLI` names. |
| A refused route on a one-CLI launch | `cliroute.ErrRefused` wraps every declared refusal (a legacy pin error keeps the runner's own text); `evolve subagent run` maps it to `subagentrun.ErrRouteRefused` and fails, and `validate-profile` returns it | Their old resolver error fell back to the profile's `cli`; a declared table must not be bypassed that way. |
| `check` exit codes | 0 when the table compiles (warnings printed), 1 on an error finding or a policy that does not load, 2 on usage | The operator's table keeps an accepted warning (builder and auditor share claude as a fallback); "`check` exits 0" after L2 needs warnings to pass. |

**Intended legacy differences** (none is in the golden; each is recorded in ADR-0119):

- `resolve-llm`, `evolve subagent run` and `validate-profile` compile the table at start and exit 2 on a refusal.
- The legacy probe and tail reorders are logged by `cliroute` (`[cliroute] agent=… capability probe reordered the chain`), replacing the runner's own lines; the root's bench lines carry `[cliroute]`.
- Contract escalation can now reach a CLI of the universal tail before the hard-coded `claude-tmux`.
- The retro launches, and names in its overlays, the CLI the bridge chain will walk first (probe and bench applied), and it honours `EVOLVE_RETROSPECTIVE_CLI` like every other agent.
- `profileForModelRouting` reads a minted or user phase's own `<phase>.json` (the shared `phaseProfile` lookup), so `router.ClampPlanModelRouting` now clamps those phases to their profile's envelope; before, only the ten built-in phases had a profile there.
- `evolve models` live refresh (and the cycle-start refresh that calls `liveRefresh`) fails when the routing table refuses to route the model classifier, instead of classifying in the old codex > claude > agy order.

**Kept inside the size ratchet** by extracting `wireCycleRun`, `printLoopDryRun`, `printSubagentRunHelp`, `parseValidateProfileFlags`, `rootRouter`, `resolvedCLI`, `terminalTier` and `onceRouting`. The allowances of the functions the change shrank are tightened in `offenders.json` (`wireOrchestratorDeps` 302 → 273, the retro's `Run` 159 → 131, `ValidateProfile` 137 → 128, `runCycleRun` 62 → 57, `runLoopBatch` and `runSubagentRun` 77 → 75, `dispatchPhaseAttempts` 78 → 77), and `resolveDispatchPlan` and `runSubagentValidateProfile` leave the list (now under 50 lines).

**Open questions for L2:**

1. Should the advisor walk the resolved chain (`advisor/launch.go:57`) instead of `ChainFor(identity.CLI, profile)`?
2. In a declared table with no `default`, a profile-less launch (the model classifier, an unknown agent) has no chain and is refused. Should it fall back to `clis` in order?
3. `runner.DefaultRouter` is process-global like the seams it replaced; in `cmd/evolve` tests one wiring test's router outlives the test. Thread the router through each phase package's `Config` instead?
4. The env and `--cli` guard (item 6), and whether it binds under `--bypass-policy` (decision 1 left this to L2).
5. `setup detect` still reports a family outside `clis` as SUBSCRIPTION, and `setup apply` still writes pins; both are item 6. Only `setup detect` gets the root's router; `recommend`, `apply` and the `models` verbs' detect still compile their own legacy router.
6. `cli-routing show` resolves each agent at its own phase only; an agent serving phases of several roles shows one of them.
7. The preflight resolves each agent without a phase, so a `pins.<phase>.cli` adds no driver to the boot list (the pre-L1b profile walk ignored pins too).
8. Go review (2026-10-06, no CRITICAL or HIGH): a `New`-built router answers a bypass request from its one table (latent: every `New` caller compiles the bypass policy itself); `contractDispatchCLI` indexes `Candidates[0]` on the resolver's non-empty-chain invariant. Each is recorded here rather than changed in L1b.
9. Minted profiles under `--evolve-dir`: `phaseregistrar.Registrar.ProfilesDir` is `<evolveDir>/profiles`, but `Router.Recompile` re-lists `routingProfilesDir(projectRoot)` = `<project>/.evolve/profiles`. When `--evolve-dir` is not `<project>/.evolve`, a minted profile lands where the recompile never looks, so the minted phase routes with no profile (the runner, which reads `<project>/.evolve/profiles` too, would not find it either, as before L1b). Mint into the routing profiles directory, or pass the registrar's directory to the router's `Setup`.

### L1b fix round (2026-10-06, architecture review FIX_THEN_MERGE)

The review's 11 mutants were all killed; it raised four findings and a handful of nits, each fixed red first (the red runs are kept with the lane's scratch evidence):

- **W1, a missing `.evolve/profiles` exited 2 with no block.** The L1a `profiles` finding was an error in both modes, but before L1b the runner and the bridge chain routed a project with no profiles directory as profile-less. With no block, a profiles directory that does not exist is now a warning and an empty snapshot; it stays an error in declared mode, and any other list failure stays an error. Red first: `TestCompile_ALegacyTableTreatsAMissingProfilesDirectoryAsEmpty`, `TestWireOrchestratorDeps_AProjectWithNoProfilesDirectoryRoutesAsBefore`. The wiring tests that L1b had made create the directory are back to their pre-L1b form, so they now prove byte-identity for a fresh project.
- **W2, the golden did not pin the production wiring.** `TestProductionRouter_ReplaysTheLegacyGolden` (`cmd/evolve`) replays all 2208 records through `buildCLIRouter` + `routingHost`, with a fake doctor report that includes a family the compiled default bans (agy), a toolless driver (ollama-tmux), a headless driver and a blocked one. A mutant that benches on `time.Now` and one that lets a toolless driver into the tail both fail it. Two divergences it exposed are fixed: the profile source is back to the runner's `<project>/.evolve/profiles` (L1b had moved it to `<evolveDir>/profiles`), and the root's bench reads an injected clock (`routingHost(log, now)`).
- **W3, one table from two profile sets.** `routingProfilesDir(projectRoot)` is the one answer to "which profiles route this project", and `buildCLIRouter(projectRoot, …)` takes no other: `cycle run`, `loop`, the preflight, `subagent run`/`validate-profile`, `resolve-llm`, `cli-routing`, setup detect and the classifier all compile it. `TestOneProfilesDirectory_ThePreflightAndTheCycleCompileTheSameTable` was red on the pre-fix code (the cycle compiled `--evolve-dir`'s profiles and the preflight the plugin root's).
- **W4, the guard and the preflight fallback.** The guard bans `llmroute.ChainFor`, `ExcludeFamilies` and `AllowedDiscovered` too, refuses a dot-import of `llmroute` or `resolvellm`, and admits one named, file-pinned exception, `internal/core/advisor/launch.go`'s `llmroute.ChainFor` (L2 question 1), which `TestChainBuilderAllowlist_EveryEntryIsStillNeeded` drops once it is unused. The preflight's silent profile-walk fallback is deleted: a table that does not compile gives no drivers (the `cli-routing` check halts the same run), and with no seam the profile seams compile a legacy table, so the drivers always come from the resolver (`RoutedDrivers`).
- **Contract escalation (Go review).** `contractDispatchCLI` used to fall back silently when the route was refused; both escalation lookups now share one loud line, `[orchestrator] phase <p>: contract escalation has no route: <err>` (`TestContractEscalation_ARefusedRouteIsLoudOnBothPaths`).
- **Nits.** `runner/cli_chain.go` is folded into `dispatch.go`. `ValidateProfile`'s stale doc comment is removed; its order now lives in [internal-subagent.md](../architecture/packages/internal-subagent.md), and `evolve comments history` finds no history to record. `bridgechain.Unlaunched` names `ErrNoPermittedAttempt` when the walk carries no error (`TestUnlaunched_AnEmptyWalkWithNoErrorStillNamesTheCause`). `Recompile` serializes its compiles, so the call made last stores last (`TestRouter_ConcurrentRecompilesKeepTheLastCallersCatalog`, red without the lock). The tracked `go/evolve` binary, which a bare `go build ./cmd/evolve` in `go/` had rewritten during the lane, is restored to `HEAD` and unstaged; builds go through `make -C go build`.

### L2 prerequisites (ranked)

1. **W7 / Q3: a production root that forgets the router silently ignores the operator's table.** `runner.DefaultRouter` is a process global, and the retro (`retro.go`, `resolveCLI`) and contract escalation (`contract_escalation.go`, `cliRouter`) compile a per-launch legacy router when none is injected. Require the router in the production constructors (each phase package's `Config`, `retro.Config`, `core.WithCLIRouter`) and keep the per-launch fallback for tests only, or refuse it outright.
2. **W5 / Q4: `--bypass-policy` reaches only the runner.** `core.BridgeRequest` has no bypass field, so the bridge chain (retro, failure advisor, judge, adjudicator, swarm), contract escalation and the advisor always resolve the declared table. Carry the flag on every launch request. The env and `--cli` guard (item 6) lands with it, including whether the guard binds under bypass.
3. **Q1: the advisor walks `ChainFor(identity.CLI, profile)`, not the table's chain** (`core/advisor/launch.go`, the guard's one allowlist entry).
4. **Q2: a table with no `default` refuses the model classifier and any unknown agent at dispatch time.** Prefer a `Compile` error finding whenever a profile-less launch class is unreachable, so `check` and the start gate catch it.
5. **The per-decision router models bypass the tier ceiling.** `decisionModel` (`cmd_cycle.go`) lays `router.plan_model` / `router.propose_model` over `Decision.Plan.Model` after the table decided it, in both modes, so a declared `tiers.<tier>` ceiling could be stepped around for the advisor. L1b refuses the two keys beside a declared block (`checkRouterDecisionModels`, pinned by `TestCompile_ThePerDecisionRouterModelsAreASecondSourceBesideTheBlock`); L2's `migrate` moves them into the table, and the advisor's per-decision tier must go through `Resolve` (an overlay tier the ceiling checks), never after it.
6. **W6: `failureAdvisorCLI` (`cmd_cycle.go`) swallows `ErrRefused`** and falls back to the compiled default silently; it must surface the refusal loudly, as contract escalation now does.
7. **Q5–Q9, low:** setup detect's SUBSCRIPTION report and `apply`'s pins, and the `recommend`/`apply`/`models` detect routers (Q5); `show` resolves one phase per agent (Q6); the preflight ignores phase pins (Q7); a `New`-built router has one table for bypass, and `contractDispatchCLI` indexes `Candidates[0]` on the resolver's non-empty-chain invariant (Q8); minted profiles under `--evolve-dir` land outside the routing profiles directory (Q9).

### L1b final fix round (2026-10-06, re-review: architecture FIX_THEN_MERGE with W1–W4 closed, Go PASS)

- **The surviving mutant M5.** Turning contract escalation's `if d.Legacy()` into `if true` survived: no test covered a declared table whose chain ends before claude. `TestContractEscalation_ADeclaredStopNeverEscalatesToTheUniversalFallback` (a builder on `agy-tmux` allowed `[claude, agy]`, `{clis: [agy, claude], default: [agy], after_chain: stop}`) passes on the code and fails on the mutant, run through a `go test -overlay`.
- **A second model-routing truth.** `router.plan_model` and `router.propose_model` are now error findings beside a declared block (`checkRouterDecisionModels`, red first), because the advisor root laid them over the table's decision. The item is ranked in the L2 prerequisites and joins L3 item 10.
- **Smaller fixes.** `classifierPreference` drops its unused `evolveDir`; `cli-routing show` lists agents from `routingProfilesDir`; the production golden replay goes through `loadCLIRouter` (the catalog half included; the bytes did not change); `preflightRouting` wraps both early errors with context.
- **Docs.** ADR-0119 lists the full ban set and the two further intended legacy differences above; the preflight page's `cli-routing` row describes the no-seam table; the open L2 questions are renumbered, with a ninth on minted profiles under `--evolve-dir`.
- **Last three leftovers (delta re-review).** The loop root and the preflight's no-seam default now share one helper, `looppreflight.CompileRouting`, so both tolerate a missing profiles directory the same way; before, the no-seam path listed the profiles a second time and halted where the root warned (red first: `TestPreflight_NoSeamAndAMissingProfilesDirectoryWarnsAndNeverHalts`). `routingCatalog` warns on stderr when the builtin registry fails to load, as `wireOrchestratorDeps` does. The plan's ban set points to ADR-0119 Decision 2, the Go-review note no longer claims a preflight fallback that W4 deleted, and ADR-0119 records the `[cliroute]` log prefix, the retro's `EVOLVE_RETROSPECTIVE_CLI` and `validate-profile`'s start-up refusal among its intended differences.

## L2 landing notes (2026-10-07)

L2 (items 6 to 9 and the ranked prerequisites 1 to 6) is built in one console lane and staged as a patch series that lands together, the table last. The checked-in `.evolve/policy.json` now declares the operator's table, written by the new verbs:

```json
"cli_routing": {
  "clis":    ["agy", "agy-claude", "claude"],
  "default": ["agy", "agy-claude", "claude"],
  "tiers":   { "deep": ["agy-claude", "claude"], "top": ["agy-claude", "claude"] }
}
```

**Status of each L2 item.**

| Item | Status | As built |
|---|---|---|
| 6, write verbs | done | `evolve cli-routing init/set/unset/migrate`; every write compiles the merged file first, then writes through `policy.PatchBlocks` (setup's writer, moved), and prints the ship hint; refused inside a phase and under a live cycle lease |
| 6, env and `--cli` guard | done | start-time `overrideRefusals` in the composition root: exit 2 before any dispatch; binds under `--bypass-policy` for the floor and the profile's `allowed_clis` |
| 7, profile fixes | done, floor half deferred | `builder.json` and `tester.json` allow `[agy, agy-claude, claude, codex]`; the floor profiles do **not** gain `agy-claude` (decision 1 below) |
| 7b, agy models on 3.8 Flash | already done | the agy manifest and the plane's live catalog both map balanced to `Gemini 3.8 Flash (High)`; no write needed |
| 8, the operator's table | done | written with `init` and `set`; pinned by `TestTheCheckedInTableCompiles` and `TestTheCheckedInTableRoutesNoLaunchToCodex` |
| 9, superseded inbox items | done in part | `one-cli-resolver-for-phases` consumed; `operator-declares-cli-substitutes` exists only untracked in the paused `dev/cl-clisubst` worktree, which the console retires |

**Status of the ranked prerequisites.**

| # | Status | As built |
|---|---|---|
| 1, W7 | done, "refuse" branch | `cliroute.RefuseLaunchRouter` and `cliroute.NewLegacyLaunchRouter`: the runner's, the retro's and contract escalation's per-launch routers refuse a declared table; `evolve campaign` installs the root router (`installRootRouter`). The process-global `runner.DefaultRouter` stays: threading the router through ten phase packages' `Config`s was out of proportion to the risk once the fallback refuses |
| 2, W5/Q4 | done | the root router carries the run's bypass (`cliroute.Setup.Bypass`), so every launch path that resolves through it, not only the runner, ignores the table under `--bypass-policy` |
| 3, Q1 | done | `cliroute.Decision.WalkAt(tier, benched)`: the root hands the advisor the table's resolved chain, filtered by the ceiling at the advisor's tier, the first unbenched CLI first; the advisor leaf walks it (`advisor.WithRoute`); `llmroute.ChainFor` and the advisor's profile loader are gone, and the guard's one allowlist entry with them |
| 4, Q2 | done as a warning | `cli_routing.default` unset is a `Compile` **warning**, not an error (decision 4 below) |
| 5, per-decision router models | done | already refused beside a declared block (L1b); `migrate` folds them into `agents.router.model` when they name one tier and refuses when they differ |
| 6, W6 | done | `failureAdvisorCLI` writes `[cycle] WARN failure advisor has no route (agent failure-advisor)` instead of falling back silently |

**Decision log (L2).**

1. **The floor agents stay on Claude Code.** The brief asked for the floor via agy-claude, then claude. `cliroute`'s floor admits only the `claude` family (`Table.allowed`), so agy-claude cannot reach a floor seat until L1c judges the floor by model family, and the six prerequisites above (C5's launch-time model check first) are unmet. Adding `agy-claude` to the floor profiles now would be inert and refused by the guard.
2. **The agy-claude guard is split by seat, not removed.** A floor seat still needs `launch_model_verification` (C5). A non-floor seat (the router, the deep and top ceilings, the fallbacks) needs the booted model to be visible: the resolved `agy-claude-tmux` manifest's `model_label_regex` (#778). That is the argument the router landing (2026-10-06) already accepted for a non-floor seat. `TestAgyClaudeRouting_TheFloorStillAdmitsOnlyClaudeCode` fails the day the floor admits another family, which forces the guard to widen with L1c.
3. **`default` is `[agy, agy-claude, claude]`, not `[agy, claude]`.** A tier ceiling filters a chain but never reorders it. With `[agy, claude]` plus the `after_chain` tail, deep work would run Claude Code before agy-owned Claude, contrary to the operator's 2026-10-06 rule that the brief's `tiers` line restates.
4. **Q2 is a warning.** Only `default` reaches a profile-less launch (`agents.model-classifier` is not an accepted key), so an error would make the documented profile rule (agents > work > default > profile) unusable in every declared table.
5. **The fast-tier phases run at balanced.** The ten fast-default profiles (`behavior-baseline`, `changelog-sync`, `close-checklist`, `dependency-map`, `doc-sync`, `flake-rerun-scan`, `locale-format-check`, `runbook-draft`, `scope-baseline`, `telemetry-coverage-check`) default to balanced; `telemetry-coverage-check`'s envelope min rises to balanced; the other nine take the universal floor (min balanced). agy's `fast` tier is not remapped, so one tier name keeps meaning one model.
6. **The builder and the tester allow agy and agy-claude.** Item 7 named agy; agy-claude is added too, so the builder's deep escalation (`m_complex_5plus_files`, `ultrathink_strategy`, `audit_retry_2plus`) runs Claude through agy first. The tester mirrors the builder, as its note says, which also makes its codex primary consistent. Cross-family: the builder starts on agy and the auditor on claude; both fall back onto claude, an accepted warning.
7. **Live evidence for prerequisite 3.** In cycles 1817 and 1818 the router got exit 85 on agy-claude and still produced LLM routing plans: the old `ChainFor` walk reached `claude-tmux` through `router.json`'s `cli_fallback`. The gap was that the advisor walked the profile's chain, not the table's: no `after_chain` tail and no ceiling. When Claude Code was also benched (wave 70, auth expiry) the router had no third CLI. The walk is now the table's.
8. **The golden.** `legacy-plans.golden.jsonl` was regenerated in the same change as the profile edits: all 2208 records kept; only the twelve edited profiles' records moved (fast to balanced; the builder's and the tester's wider allowed set).

### L2 fix round (2026-10-07, review FIX_THEN_MERGE: 0 CRITICAL, 4 HIGH, 5 MEDIUM, 4 LOW)

The review confirmed the directive (64 non-deep, non-floor agents resolve agy first; no deep or top dispatch reaches Gemini; no route reaches codex outside `--bypass-policy`), `WalkAt`, bypass, the write verbs and the golden. Every finding is fixed; none is declined.

| Finding | As fixed |
|---|---|
| H1, three phase verbs refuse a declared project | `phasecmd.RouterInstaller` (`func(projectRoot string) error`): `NewRunPhase(floor, install)` and `NewRunServePhase(install)` install the root router before the factory runs, and `runCompose` calls `installRootRouter` after its dry-run; production wires `rootRouterInstaller` (`cli_routing_root.go`). A refused install exits 2. One test per verb under a declared table (`phase_verbs_root_router_test.go`) |
| H2, `setup apply` is a second routing writer | first fixed as a single-writer route (presets written as `agents.<role>` rules through the table writer); the delta review found that version wrong (below), and the final round replaced it with a refusal: on a declared table `setup recommend` and `setup apply` exit 1 and point at `cli-routing show`/`set`, and pins never land beside a table |
| H3, env precedence restated | `llmroute.EnvPrimary(agent, env) (cli, source)` is the one rule (`EVOLVE_<AGENT>_CLI` over `EVOLVE_CLI`); `overrideRefusals` reads it and names the remedy for the key that won. The reviewer's probe is adopted and kills M10 |
| H4, `model_label_regex` is passive | the launch-time model check (C5's agy-claude slice, below) |
| M1, the floor tripwire had no positive control | `floorChainLeaks(router, root, agents)`, pinned by a synthetic table where the router leaks and the auditor does not |
| M2, requests built from `Plan.Candidates[0]` | `Decision.Walk()` (the walk at the decision's own tier, nothing benched): the retro, contract escalation (dispatch and escalation candidates) and `Router.ResolveRole` name what the ceiling permits |
| M3, `wireOrchestratorDeps` grew | `wireRouterAdvisor(routerAdvisorWiring)` owns dispatch, persona and `NewPhaseAdvisor`; the ratchet allowance fell 273 → 257 |
| M4, over three parameters | parameter objects: `routingWrite`, `routingSet`, `migration`, `routerSite`, `routerAdvisorWiring` |
| M5, flag argument in a test helper | `realTreeSeatGaps(t)` returns `(floorGaps, otherGaps)` |
| LOWs | the moved comments are gone; `resolveRouterDispatch`/`resolveRouterDispatchFor` are deleted and their tests drive `resolveRouterDispatchHealthy`; `routingProjectRoot` makes an unreadable working directory loud; `migrate` refuses router tiers that differ, comparing the effective tier; the override refusal names its remedy |
| Envelopes | the thirteen tracked profiles still at `min: fast` (the twelve the review named plus `cache-strategy-scan`) rise to balanced; `TestNoTrackedProfileDefaultsToTheFastTier` refuses any fast bound, with no exception; `scan_tier_envelope_test.go` pins `{balanced, balanced}` |
| Single sources | CLAUDE.md, `builder.json`'s `_note`, the policy `description`, the setup skill |

**Composition roots and their router** (re-enumerated from every `registry.For` and runner construction):

- `evolve cycle run` and `evolve loop`: `wireOrchestratorDeps` builds the root router once and sets `runner.DefaultRouter`, the retro's `Config.Router`, `core.WithCLIRouter` and the bridgechain resolver; phases the registrar mints read `DefaultRouter`.
- `evolve campaign`: `installRootRouter`.
- `evolve phase` (and plan-and-execute, which runs through `runPhase`) and `evolve serve-phase`: the `phasecmd.RouterInstaller` hook.
- `evolve compose`: `installRootRouter` after the dry-run.
- `evolve cycle run --simulate`: stub runners, no dispatch.
- `evolve subagent run`/`validate-profile`, `resolve-llm`, `setup detect`, the models classifier and preflight: each compiles its own root router (L1b).
- `looppreflight`'s `factoryKnown`: a name check, no dispatch.

**The launch-time model check (H4; C5's agy-claude slice).**

- **Rule:** a manifest declares `launch_model_verification: {label_wait_s: N}`; `agy-claude-tmux.json` declares 10. The loader refuses a rule with no `model_family`, no `model_label_regex` or a non-positive wait.
- **Check:** `runTmuxREPL`, after the boot and before the prompt, polls the pane once a second for up to `label_wait_s` and reads the label with `model_label_regex`; `modelquery.FamilyOf(label)` must equal `model_family`. A pass logs `launch model verified`.
- **Mismatch or no label:** the session is killed (the launch's own cleanup), the launch exits **87** (`ExitModelMismatch`, class `model_mismatch`, a plain failure), and `BRIDGE_DISPATCH_MODEL_MISMATCH` carries `expected`, `observed` (a family, or `unreadable`), `label`, `cli` and `phase`. 87 joins the default walk triggers, so the walk moves to Claude Code. The exit benches nothing by itself (only 85 benches and only 80 records a boot strike); since #789 the usage query on every quota-explainable exit can bench an exhausted family (the final round below).
- **Why a new exit:** 80 records boot strikes that bench, 85 benches as quota and 81 is the artifact timeout; a model mismatch is none of them.
- **Signal name:** the registry requires the owning module's prefix, so the brief's `DISPATCH_MODEL_MISMATCH` is `BRIDGE_DISPATCH_MODEL_MISMATCH`; the exit's own `BRIDGE_EXIT_MODEL_MISMATCH` fires as for every exit.
- **One rule for both seats (supersedes L2 decision 2).** The guard no longer splits by seat: a floor seat and a non-floor seat both need `bridge.VerifiesLaunchModel`, true only when the target's driver runs a rule the loader accepts. `model_label_regex` is an input of that rule, not a safeguard by itself. The real-tree guard turns red when the rule is dropped from the manifest.
- **Goldens:** the legacy golden moved 298 records for the envelopes (tier ladders and envelope text of the thirteen profiles) and 2183 for trigger 87 (`triggers` only).
- **Mutation:** the fix round's 43 mutants were all killed (H1 6, H2 10, H3 2, H4 14 with one by hand on the embedded manifest, M1 1, M2 7, LOW 3); a mutant whose overlay did not compile was rewritten and re-run, never counted.

### L2 final round (2026-10-07, delta re-review FIX_THEN_MERGE)

The delta review closed H1, H3, H4 and every MEDIUM of the fix round, read all 13 real agy-claude panes correctly, and confirmed 87 is in every chain, never benches and never retries. Main had moved (train #792: #788 carry evidence, #789 usage evidence on every walk, #791 the inbox rank's P2), so the change was re-applied onto it in a fresh worktree; the conflicts (CHANGELOG, CLAUDE.md line 65, the llmroute notes, the size-ratchet list) were merged by meaning, and a numstat audit matched the lane file for file.

| Finding | As fixed |
|---|---|
| HIGH-1, `setup apply` on a declared table compared presets with profile defaults the table no longer runs, printed "wrote" over a byte-identical file, and deleted an operator's `agents` rule while naming the wrong route | **refusal, not a table-relative preset.** `detect` reports `routing_table_declared`; `setup.Apply` returns `setup.ErrRoutingTableDeclared` and `setup recommend` prints it, both exit 1, nothing is written. The table-relative version was judged disproportionate: the recommender is profile-relative (the profile's CLI, the cross-family builder/auditor pair, the envelope bias) and assigns one CLI per role, while the table routes ordered chains through tier ceilings, the Claude floor and the tail. Making presets table-relative needs a second copy of the router's resolution inside `internal/setup` (ADR-0119 allows one resolver) or freezes each role's chain into an `agents` rule, and the "max" preset would pick agy at deep, which the operator's deep ceiling refuses. Pinned end to end by `cmd_setup_table_test.go` (the operator's `agents.scout` rule survives). |
| HIGH-2, mutant H2e survived (a removal notice for an absent rule) | moot: the notice and the whole table-writer path (`applyPresetRoutes`, `applyPresetToTable`, `presetTableEdit`) are deleted with the refusal |
| HIGH-3, mutant H4i survived (`family: "claude"` hard-coded) | `TestLaunchModelCheckFor_ExpectsTheResolvedTargetsOwnFamilyAndWait`: an override declaring `model_family: gemini` and a 3 s wait |
| MEDIUM-1, the check could fail over a healthy boot (it decided on the first readable label and read only the last line) | it waits for the dispatched family and calls a mismatch only at the deadline; it reads the bottom six non-blank lines (`panestream.ModelLabels`); it ticks the auto-responder on each frame, and a frame the responder acted on is never evidence; the agy label regex reads extra footer segments and the `(High)` style. The review's panes are fixtures (`go/internal/bridge/testdata/agy-model-check/`, sanitized) |
| MEDIUM-2, boot smoke skipped the check; 87 was unnamed | the check runs before a boot smoke returns; `bootRCName`, `doctor boot` and `doctor live` name 87 |
| MEDIUM-3 (console decision: keep the usage query on 87) | the signal and doc texts now say the mismatch benches nothing by itself while the usage-evidence decorator's query can bench an exhausted family; pinned by `usageevidence` `TestBridge_AModelMismatchOnADrainedClaudeGroupBenchesAgyClaude` (and a healthy screen benches nothing) |
| LOW: a named session | killed explicitly on a mismatch (cleanup would preserve it for resume) |
| LOW: a pane that dies right after boot | exits 81 with a session-recoverable `dead_shell` observation, so the engine's fresh-session retry still applies |
| LOW: the canary | keeps a bench whose probe returns 87 |
| LOW: `cli_fallback_on_exit` without 87 | `Compile` warns, in the legacy projection and under a declared table |

**ACS predicates (the A2 retention rule).** Two tests on main hard-coded the five-code trigger set. `go/internal/llmroute/trigger_aliasing_test.go` now compares with `DefaultTriggers()` taken before any edit. `go/acs/cycle1821`'s predicates 001 to 003, and `go/acs/cycle1580`'s 002 (which froze the exit-code list), asserted the old sets as their acceptance, so they were archived verbatim, never edited, to `docs/private/research/archived-2026-10-07/superseded-predicates/` with README rows naming what pins each contract now (including the new `cliroute` `TestRouter_EveryDecisionOwnsItsTriggers`). The lane's earlier in-place edit of `cycle1580`'s frozen map was reverted in favour of the archive.

**Mutation.** The round's 28 mutants were all killed: 26 for the delta (two survived the first run, the responder-tick and acted-on-frame mutants, until the dialog tests asserted the responder's distinctive answer; one was run by hand on the embedded manifest regex; two came with the tests that name `ModelLabels` and `llmroute.ExitModelMismatch`) and the two aliasing mutants behind the archived predicates' replacements.

### L2 fix round 6 (2026-10-07, delta check FIX_THEN_MERGE)

The delta check confirmed every finding it was asked to confirm and named the items below. Each was written red first or pinned by a test that kills its mutant.

| Item | As fixed | Test |
|---|---|---|
| H1, preflight halted the batch on 87 | the `bridge-boot` check WARNS on a model mismatch and keeps the batch; any other boot failure still halts, with the mismatch beside it (`bootTally.result`) | `TestRun_BridgeBoot_AModelMismatchWarnsWithItsUsageLineAndKeepsTheBatch`, `TestRun_BridgeBoot_EveryBootFailureHaltsAloneAndBesideAMismatch` (fix round 7's table) |
| H2, unpinned exit and retry branches | pins only (the code was right) | `TestLaunchModelVerification_AResponderLoopGuardTripDuringTheWaitIsABootFailureNotAMismatch` (M5), `…APaneThatDiesDuringTheWaitKeepsTheFreshSessionRetry` now for a named and an unnamed launch (M6), `…ALiveSessionWhosePaneFellBackToTheShellIsADeadShell` (M14), `…TheFooterIsReadUpToSixLinesAboveTheBottom` (M3: 3 and 6 lines up verify, 7 fails over) |
| M1, the check could verify a Gemini boot | **the footer-prefix line decides** (as fixed in round 7; round 6's bottom-most label under the input box let a Claude toast below a Gemini footer verify): the label on the footer-prefix line decides; any other label never counts; no prefix line means unreadable, which means 87. `panestream.FooterModelLabel` reads the bottom-most line that starts with the label regex's named `footer` group, and `modelquery.FamiliesIn` makes a label naming two families no evidence (`ambiguous`). This is the deep/top integrity guarantee | `…OnlyTheLabelOnTheFooterPrefixLineDecides` (round 7), `…AFooterNamingTwoFamiliesIsReportedAmbiguous`, `TestFooterModelLabel_ReadsOnlyTheBottomMostFooterPrefixLine`, `TestFamiliesIn_NamesEveryFamilyALabelMentionsInTableOrder`; every real-pane fixture stays green |
| M2, the setup refusal failed open on a policy that does not load | `Apply` refuses whenever the JSON it patches has a `cli_routing` key, and `Detect` reports the key from the raw file (the dead `polErr == nil` guard is gone) | `TestApply_ATablePolicyThatFailsToLoadIsStillRefused` (the review's probe), `TestApply_TheFileItPatchesDecidesTheRefusalNotTheReport`, the no-file case of `TestDetect_ReportsWhetherTheRoutingTableIsDeclared` |
| M3, unpinned diagnostics | pins | `…TheMismatchReportsTheLastFooterRead` (M2), `TestDoctorLive_AWrongModelPrintsTheUsageVerdict` (M9), a one-code `[85]` list in `TestCompile_WarnsWhenAProfilesTriggerListOmitsTheModelMismatchExit` (M10), human-mode recommend in `TestRunSetupRecommend_ADeclaredProjectOffersNoPresets` (M15) |
| L1, a cancelled wait reported 87 and a mismatch signal | it exits 81 with the `cause=context_cancelled` marker and no signal, as the completion wait does when its context is cancelled (the boot loop, by contrast, ends with 80 at its boot deadline) | `…ACancelledWaitExitsAsACancelledLaunchWithoutAMismatch` |
| L2, the refusal's stated reason | reworded in internal-setup.md and setup-onboarding.md: the verbs are the table's only writer, a preset's one-CLI-per-role answer is not a rule the table can hold, and the "max" bias would pick agy at deep | docs |
| L3, human-mode detect said nothing of the table | one line naming the table and `evolve cli-routing show` | `TestRunSetupDetect_TheHumanReportNamesTheDeclaredTable`, `…OfAPinsProjectNamesNoTable` |
| L4, the skill skipped the latest-model probe on a table project | it skips steps 4 to 7 only and asks the adopt-latest question on its own | skill text |
| L5, the canary's order between 87 and a wall | a benchable wall re-benches with a strike first; a bare 87 keeps the bench | `TestCanaryAWallBesideAModelMismatchRebenchesWithAStrike` |
| L6, the archive README on cycle1821 | all four live predicates run; 007 is point-in-time and fails in this lane until the merge | README |
| L7, the deleted table writer's residue | `presetRoutesFor` and its Set/Cleared split folded back into `Apply` with main's order and wording; the review's preservation probe reproduces main's output and error on all 35 no-table cases; the skill header says `apply` writes pins | `TestApply_*` |

**Equivalent mutants.** M4 (comparing the raw label with the family name instead of the classified family) is equivalent: every footer agy renders for a Claude model carries the word "Claude", and none for Gemini does, so the two agree on every label the regex reads; they would part only on a bare `Opus` label, which agy does not show. M11 (dropping `polErr == nil &&`) was equivalent because a failed `policy.Load` returns a zero policy whose `CLIRouting` is nil; the guard is gone with the raw-key rule.

### L2 fix round 7 (2026-10-07, delta check FIX_THEN_MERGE, the last round)

The round-6 delta check confirmed H1, M2, L1, L5 and L7 and found one MEDIUM left: a Claude-only toast below a Gemini footer still verified a Gemini boot (its probes P1, P1b, P1c; P1c needs one transient frame). Each item below was written red first or pinned by a test that kills its mutant.

| Item | As fixed | Test |
|---|---|---|
| MEDIUM-1, a toast below the footer decided | **the label on the footer-prefix line decides; any other label never counts; no prefix line means unreadable, which means 87.** The agy base's `model_label_regex` names its prefix group `(?P<footer>esc to cancel\|\? for shortcuts)`; `panestream.FooterModelLabel` lifts that group out of the regex (`regexp/syntax`), finds the bottom-most line of the six-line tail that starts with it and reads the label on that line alone; a label naming two families is `ambiguous`. The loader refuses a verified target whose label regex names no `footer` group. This also closes LOW-1 (other box glyphs, no box, Claude text in the output) and the false failures P8 and P10 | `TestLaunchModelVerification_OnlyTheLabelOnTheFooterPrefixLineDecides` (all 21 probes with the review's Want, the five earlier footer rows and three stale-footer rows), `TestFooterModelLabel_ReadsOnlyTheBottomMostFooterPrefixLine`, `TestFooterModelLabel_APatternWithoutBothNamedGroupsReadsNoLabel`, the no-footer case of `TestParseManifest_RefusesALaunchModelRuleItCannotRun` |
| P4, P4b, P5, P5b stay unreadable (87) | a footer split across two lines (P4, P4b) leaves the prefix line without a whole label, and reading a label off another line would reopen the toast hole; a `•` separator (P5) and no-break-space padding (P5b) appear in no captured agy pane, so the regex does not guess at them. All four fail over to Claude Code, the safe direction | the P4, P4b, P5 and P5b rows |
| LOW-2, preflight on other boot failures | pin: 80, 81, 127, 10 and -1 each halt, alone and beside an 87 | `TestRun_BridgeBoot_EveryBootFailureHaltsAloneAndBesideAMismatch` (replaces `…AMismatchBesideABootTimeoutStillHalts`) |
| LOW-3, the signal's label | pin: a footer whose label went blank before the deadline keeps the last labelled footer (`label=Gemini 3.8 Flash · low`, `observed=gemini`) | `TestLaunchModelVerification_TheMismatchKeepsTheLastLabelledFooter` |
| LOW-4, docs | `bootTally.result` named; the cancel wording corrected (the model check exits 81 as the completion wait does; the boot loop ends with 80 at its deadline); internal-bridge-panestream.md lists four readers, each once, and the pane watch's own two | docs |
| INFO, a case-variant routing key | `declaresRoutingTable` compares keys with `strings.EqualFold`, because `policy.Parse` matches JSON keys without regard to case, so `"CLI_Routing"` is a live table | `TestApply_TheFileItPatchesDecidesTheRefusalNotTheReport` (both spellings, with the parse premise asserted), the other-case row of `TestDetect_ReportsWhetherTheRoutingTableIsDeclared` |

### L2 landing: review convergence and known limits (2026-10-07)

- **No eighth round.** The operator said "It just retry for too many times", so the console stopped here.
  - The cause was not the model: the lane and its reviewers all ran on claude-opus-5-5, and every round's fixes were correct.
  - The cause was the target. About 18 of the 27 findings in rounds 5–7 came from one heuristic: the launch model check reads agy's TUI footer text, which can take endless shapes.
  - A second cause was the review protocol. Each delta check had to add new adversarial probes, so it was certain to find another pane shape.
  - Round 7 got a narrow verification instead: the round-6 probes behave as wanted, or fail safe, and the floor is green.
- **Known limits of the model check, all in the safe direction.** A footer split over two lines, a `•` separator, or no-break-space padding read as unreadable, which gives exit 87 and failover to Claude Code. A Gemini boot can verify as Claude only if agy draws a footer-prefix line whose own label names Claude.
- **Follow-up, filed with this landing:** `agy-boot-model-check-reads-a-structured-source`. Read the booted model from a structured source, such as the `/model` response through `modelquery`'s capture or agy's own state, instead of the ambient footer.
- **Superseded draft, never filed:** `operator-declares-cli-substitutes` (2026-10-05, a `cli_substitutes {codex: agy}` key). The operator's routing table is the one home for that intent, so the draft was retired with the `dev/cl-clisubst` worktree.

## Provider-aware targets (2026-10-06)

### The directive

On 2026-10-06 the operator said: "agy could also use claude opus 5.5 and sonnet 5.5, prioritize using agy owned claude models first then using claude code models after".

- **What agy serves.** agy 1.2.17's `agy models` lists `Claude Opus 5.5 (Low|Medium|High)` and `Claude Sonnet 5.5 (Low|Medium|High)` beside the Gemini models. Both bill the Google AI Pro subscription, not Claude Code's weekly limit.
- **The rule.** Wherever a Claude *model* is used (the Claude-family floor agents, the deep and top tiers, Claude as a fallback), it runs through agy first. Claude Code is used only after that.

### What the provider-target component landed (bridge, catalog, probe)

The CLI routing table needs a name for "Claude through agy" before it can route to it. This component adds that name. Its one routing change is the router profile, made at landing (see *The router runs agy-owned Claude* below); no other profile, and no pin, tail or table, names it.

| Piece | What it is | Where |
|---|---|---|
| Target `agy-claude-tmux` | the agy binary and driver with its own tier map: fast `Claude Sonnet 5.5 (Low)`, balanced `Claude Sonnet 5.5 (High)`, deep and top `Claude Opus 5.5 (High)`. agy offers no Haiku-class Claude. The manifest is a JSON Merge Patch over `agy-tmux` (`"base"`). | [internal-bridge.md](../architecture/packages/internal-bridge.md#provider-aware-targets-manifest_basego-model_familygo-agy-claude-tmux) |
| Model family | every manifest's `model_family` (claude, gemini, gpt, local); `bridge.ModelFamily(driver or family)` is the one accessor | same |
| Routing family `agy-claude` | `llmroute.Family("agy-claude-tmux")` is `agy-claude`, as `profiles.BaseCLI` already said; `llmroute.Binary` is `agy`. `DefaultDriverForFamily("agy-claude")` is the target, so a table can name `agy-claude`. | [internal-llmroute.md](../architecture/packages/internal-llmroute.md) |
| Bench | keyed by routing family, so an agy Claude quota wall benches `agy-claude` only, and an agy Gemini wall benches `agy` only. Antigravity pools quota per model group (Gemini; Claude and GPT), each with a weekly and a 5-hour limit (live, 2026-10-06, `evolve bridge control agy usage`), so the split matches the provider. Pinned both ways by `TestBenchOnEscalation_AQuotaWallOnOneAgyTargetLeavesTheOtherFirst` | same |
| Catalog entry `agy-claude` | one `agy models` run feeds `agy` (filtered to gemini) and `agy-claude` (filtered to claude, `catalog.allowed_families` in the checked-in policy) | [model-discovery-and-catalog.md](../architecture/model-discovery-and-catalog.md#provider-aware-entries-agy-claude-2026-10-06) |
| `evolve doctor live <driver> --model <model>` | the operator's smoke boot of one model through the interface | [runtime-reference.md](../operations/runtime-reference.md) |

**Kept out of routing on purpose.**

- The legacy universal tail is keyed by binary, so the doctor's `agy-claude-tmux` row never joins a chain. 86 tracked profiles have no `allowed_clis`, or `all`, and would otherwise have gained it ahead of agy-tmux.
- Preflight's `distinctDrivers` reads profile chains, so since the router moved it boots `agy-claude-tmux` for the router's chain.

**Live evidence, 2026-10-06.**

- `evolve doctor live agy-claude-tmux --model "Claude Opus 5.5 (High)"` and `--model "Claude Sonnet 5.5 (High)"` both answered LIVE OK.
- `evolve bridge launch --cli=agy-claude-tmux --model=deep` and `--model=balanced` showed `Claude Opus 5.5 (High)` and `Claude Sonnet 5.5 (High)` in agy's banner. The agents wrote those names to their artifacts.
- The first deep attempt hit an upstream `Eligibility check failed: UNAVAILABLE (code 503)`. While that outage lasted agy refused the model (`… is not recognized … Ignoring the flag`), and the attempt timed out. The retry passed.

### L1c: route by model family (specified here; built after L1b)

**Integrity by model family.**

- `cliroute`'s Claude-family floor and the `cross_family_with` check (builder ≠ auditor) judge `bridge.ModelFamily` of each candidate, not its CLI family. The floor admits `agy-claude` and `claude`. A builder on `agy` (gemini) and an auditor on `agy-claude` (claude) are different families.
- `cliroute` must not import `bridge`, so the family arrives the way tool capability does: a `Compile` option (`WithModelFamily(bridge.ModelFamily)`), wired at the composition root by L1b's successor.
- The contract-escalation "different family" pick (`core/contract_escalation.go`) moves to model family in the same step.

**The operator's table.**

```json
"cli_routing": {
  "clis":    ["agy", "agy-claude", "claude"],
  "default": ["agy", "agy-claude", "claude"],
  "tiers":   { "deep": ["agy-claude", "claude"], "top": ["agy-claude", "claude"] },
  "after_chain": "other_clis"
}
```

- A floor agent's allowed set becomes `[agy-claude, claude]`: Claude through agy first, then Claude Code.
- Deep and top dispatches run Claude Opus on agy first.
- **L2:** the floor profiles' `allowed_clis` (auditor, adversarial-review, tdd-engineer, spec-verifier, spec-verify) gain `agy-claude`, and `builder.json` gains `agy` as already planned.

**Prerequisites before a floor agent routes to agy-claude.** Each was found by this component, three of them live:

1. **Confirm the booted model.** agy 1.2.17 boots its default model for a `--model` it does not recognize. `--model "Claude Opus 9.9 (High)"` ran `Gemini 3.8 Flash (High)` and the launch reported success. A boot-time check must refuse a launch whose banner names another model than the one dispatched. It needs a manifest banner regex, because the typed launch line also contains the model name. Without it, a stale tier map, a renamed model, or an upstream 503 can put a Gemini model in a Claude-floor seat silently. The new `model_not_recognized` escalate rule covers only a run whose pane keeps the warning, and at boot agy shows it only briefly.
   - **Cross-link:** this prerequisite is C5 (launch-time model verification) in [model-currency-2026-10.md](model-currency-2026-10.md), which lands with PR #774.
   - **Built (L2 fix round, 2026-10-07).** `agy-claude-tmux.json` declares the typed rule `launch_model_verification: {label_wait_s: 10}`. After the REPL boots and before the prompt goes out, the bridge reads the label on the footer-prefix line through the base's `model_label_regex` (its named `footer` group is the prefix; any other label never counts) and compares its family (`modelquery.FamiliesIn`, where two families are no evidence) with the manifest's `model_family`; a wrong family, or no readable label within the wait, exits 87 so the walk moves on (see the L2 fix-round notes above). No second banner parser was built.
   - **Guard:** `TestAgyClaudeRouting_AFloorSeatNeedsTheLaunchModelCheck` and `TestAgyClaudeRouting_ANonFloorSeatNeedsTheLaunchModelCheck` (`go/cmd/evolve`, `agy_claude_routing_verification_test.go`) fail when any route names `agy-claude` while `bridge.VerifiesLaunchModel("agy-claude-tmux")` is false: the target's own driver must run a rule the loader accepts, not merely declare a key. The routes it reads are a `profiles.ClaudeFamilyFloor` profile's `cli`, `allowed_clis` or `cli_fallback`; any `.evolve/policy.json` pin; and the typed `policy.CLIRouting`: `clis`, `default`, every `work.<role>`, every `agents.<agent>` chain and every `tiers.<tier>`. The manifest is read through the loader's own merge-patch resolution, so a chained base fails as the loader fails, a `null` that removes the base's rule counts as no rule, and an empty `{}` or a rule with no label regex is a load error. `TestAgyClaudeRouting_TheGuardSeesEachRouteL2CanAdd` and `TestAgyClaudeRouting_OnlyARunnableRuleOnTheResolvedTargetCounts` keep the guard from going vacuous; `bridge.ManifestObject` is gone with the untyped check.
   - **Still open for a floor seat:** the floor admits only the `claude` CLI family until L1c judges it by model family, so the floor guard passes because nothing routes a floor seat to agy-claude.
2. **A usage probe per target.** The pre-wave usage probe reads agy's `/usage` once per binary and benches `agy`. With two agy families it must classify each family's cap separately, or one family's wall benches the other.
3. **The catalog filter from the model family.** The checked-in policy filters `agy-claude` to `claude`, and a test pins every filter to `ModelFamily`. A deployment without that policy entry would classify all agy models for `agy-claude`. The refresh should default each key's filter to its target's model family.
4. **The live pick's effort.** The first live classification chose `Claude Sonnet 5.5 (Medium)` for balanced and `Claude Opus 5.5 (Medium)` for deep. If deep must be `(High)`, say so with `agents.<name>.model` or a catalog `tier_fallbacks` entry when the catalog moves to `enforce`.
5. **Profiles stay byte-identical until L2.** Adding `agy-claude` to `clis` before the floor profiles allow it leaves those agents on `claude`, and `show` prints the filtering.
6. **One bare-name → driver projection.** `bridge.bareDriverMap`, which `bridge.DriverFor` reads, has no `agy-claude` entry, so `DriverFor("agy-claude")` returns the name unchanged and the caller's `LookupDriver` miss fails loudly: `subagent/bridgeadapter.go:21` and `:82`, `consensusdispatch/consensusdispatch.go:348`, `phaseregistrar/registrar.go:128`. Nothing names the bare family today. L1c folds `bareDriverMap` into `llmroute.DefaultDriverForFamily`, so routing and dispatch share one projection (deferred from review round 2, 2026-10-06).

### Integration with the boundary updater (PR #774, merged)

- #774 adds `default_env: {"AGY_CLI_DISABLE_AUTO_UPDATE": "1"}` and `update_argv: ["agy", "update"]` to `agy-tmux.json`.
- `agy-claude-tmux` inherits the env through `base`, so its launches carry the off switch with no edit. `bridge.ProcessEnv("agy")` reads `agy-tmux` and covers the agy processes that are not launches.
- The target's `update_argv: null` removes the inherited updater. #774's updater walks `InteractiveFamilies()`, deduped by binary, so agy updates once.
- #774's `TestUpdateArgv_EveryOtherManifestDeclaresNoUpdater` stays green with either landing order.
- The only expected conflict is textual: both change `agy-tmux.json`, near the top for #774 and at `transient_regex`, `model_family` and `interactive_prompts` here.
- **Landing note (review round 1, 2026-10-06).** #774 merges at the next boundary, before this component. This component lands rebased onto main with #774, and the rebase adds the assertion that the merged `agy-claude-tmux` manifest declares no `update_argv` (its updater is agy-tmux's). Until then the key is inert: this base's `Manifest` has no `update_argv` field, and the loader ignores it.
- **Done at landing (train #780 on main, 2026-10-06).** #774's `TestUpdateArgv_EveryOtherManifestDeclaresNoUpdater` already is that assertion: it walks every manifest through `LoadManifest`, which resolves the merge patch, so it reads the merged `agy-claude-tmux`. Removing the target's `"update_argv": null` turns it red (`agy-claude-tmux declares update_argv ["agy" "update"]`), so no second test was added. The landing also resolved four conflicts with #777 (L1b) and #778 (agy liveness): `agyTmuxLaunch` takes the target name, so `agy-claude-tmux` launches with agy-tmux's `>` input-line marker and pane watcher; the auto-responder takes #778's injected profile (`cliPaneProfile`), which goes through `paneProfileFor` and so keys by binary; and the #778 manifest keys (`busy_line_regex`, `token_line_regex`, `model_label_regex`) reach the target through `base`.

### The router runs agy-owned Claude (landing, 2026-10-06)

- **The rule.** The operator's rule is that deep and top work runs a Claude model, agy-owned first, Claude Code after, and never Gemini 3.1 Pro. `.evolve/profiles/router.json` ran `cli: agy-tmux` at `model_tier_default: deep`, which resolves to `Gemini 3.1 Pro (High)` every cycle. `evolve bridge sessions` showed it live in cycle 1807.
- **The change.** `router.json` runs `cli: agy-claude-tmux`, so its deep tier is `Claude Opus 5.5 (High)`. `cli_fallback` stays `["claude-tmux"]`, and the tier stays deep. Its `allowed_clis` is `["agy-claude", "claude"]` (was `["all"]`), so the resolved chain is `[agy-claude-tmux, claude-tmux]` for both the phase launch and the advisor launch: no codex, which has no subscription and can only fail, and no Gemini tail.
- **Pinned by:**
  - `TestDeepAndTopProfiles_NoChainEntryResolvesTheirTierToAGeminiProModel` (cmd/evolve): no deep or top profile's own chain (`cli` plus `cli_fallback`) resolves its tier to a Gemini Pro model, and no profile's full resolved chain does either. The full chain is resolved through `cliroute.Build` over the checked-in policy, with every binary installed, so it includes the universal tail. Its only exceptions are the profiles listed in `deepTierGeminiProTailAwaitingL2`, and a listed profile that stops reaching Gemini Pro fails the test, so the list only shrinks. It was red twice: first on the router's own chain (agy-tmux), then on its tail (agy-tmux after codex);
  - `TestRouter_EveryLaunchOfTheAdvisorStaysOnClaudeModels` (cmd/evolve): both the router phase and the advisor launch resolve to exactly `[agy-claude-tmux, claude-tmux]`;
  - `TestRouterRunsAgyOwnedClaudeFirstThenClaudeCodeAtDeep` (profiles; it was `TestRouterStaysOnAgyWithClaudeFallback`);
  - the router exception in `TestDeepTierFamilyArrangement`.
- **Goldens regenerated.** Only router records changed, verified record by record. The move changed 19 plan records and 88 advisor records. The `allowed_clis` stopgap changed 22 more plan records and no advisor record. The files keep 2208 and 120 records. In the first regeneration:
  - `legacy-plans.golden.jsonl`: 19 router records change, and the file keeps 2208 records. The checked-in configuration's chain (`checked_in_tail`) is `[agy-claude-tmux, claude-tmux, codex-tmux, agy-tmux]`. In the `advisor` variant, an `agy` overlay now promotes `agy-tmux` ahead of the router's own entry, because `agy-claude-tmux` is not of the `agy` family.
  - `legacy-advisor.golden.jsonl`: 88 router records change, and a fifth bench state, `agy_claude_target_benched`, adds 24 (120 records). An `agy-claude` wall moves the advisor to `claude-tmux`, and an `agy` or `claude` wall leaves it on `agy-claude-tmux`. The older state named `agy_claude_benched` benches the `agy` and `claude` families, not `agy-claude`.
- **The guard does not refuse it, by design** (superseded 2026-10-07: the L2 fix round built the launch-time check and the guard now requires it for every seat; see "L2 fix round"). `TestAgyClaudeRouting_WaitsForALaunchTimeModelVerificationRule` walked the Claude-floor profiles and the policy-level routes (pins and `cli_routing`), which can reach a floor agent. The router is not a floor seat: a silent substitution there runs agy's default Gemini Flash in the advisor, which costs quality but not the builder ≠ auditor integrity the guard protects. It is also visible: #778's pane watcher records the model label agy shows. So the guard's scope is unchanged.
- **Stopgap applied: `allowed_clis`.** The move alone left the router's chain ending with the legacy universal tail's `agy-tmux` at deep (Gemini 3.1 Pro), after codex. The router's `allowed_clis: ["agy-claude", "claude"]` removes both, following two standing operator rules: Gemini 3.1 Pro is never used for deep or top work, and codex has no subscription. One more effect: a phase pin that names `agy` for the router is now refused (`cli "agy" not in allowed_clis`), which the golden's `pin` variant records.
- **Explicit routes still reach agy-tmux** (golden variants, unchanged by design): an operator's `EVOLVE_ROUTER_CLI=agy-tmux` (`env_agent`) and an advisor overlay that names `agy` for the router phase (`advisor`; a soft overlay prepends its family's default driver). Both are explicit choices, not the silent tail. Whether `allowed_clis` binds them is the L2 question already listed (env values under the floor guard).
- **22 other deep profiles reach Gemini 3.1 Pro only through the universal tail**, after codex and Claude Code: `architecture-design`, `caching-strategy-design`, `compat-surface-check`, `data-integrity-check`, `data-model-design`, `debugger`, `failure-adjudicator`, `failure-advisor`, `idempotency-check`, `intent`, `merge-to-main-gate`, `migration-safety-check`, `observability-design`, `plan-reviewer`, `preliminary-study`, `premise-challenge`, `prompt-regression-eval`, `resilience-design`, `retrospective`, `rollout-plan`, `swarm-planner`, `type-safety-audit`. Each runs `codex-tmux` then `claude-tmux`, and the tail appends `agy-tmux` at deep. They are left unchanged: L2's `tiers.deep` ceiling (`["agy-claude", "claude"]`, above) closes them all at once, and the test's `deepTierGeminiProTailAwaitingL2` list is the ratchet until then. The router was the one that ran every cycle.

### Review round 1 (2026-10-06)

| # | Finding | Fix | Pinned by |
|---|---|---|---|
| 1 | CRITICAL: the bridge keyed the pane profile by the driver-name stem, so `agy-claude-tmux` got `DefaultDetector` and the `? for shortcuts` footer instead of agy's `*AgyDetector` and `>` boundary | `driverBinary(driver)` (the manifest's `binary`; the `-tmux` stem only with no manifest) keys `paneProfileFor`, the auto-responder's busy read and the wall corroborator's probe recipe. A grep of `go/internal/bridge` found no other stem derivation that picks per-binary behaviour. | `TestPaneProfileFor_AgyClaudeTmuxReadsTheAgyBinarysPaneWithTheAgyDetector`, `TestAutoResponderTick_ReadsBusyFromTheProfileOfTheBinaryATargetDrives`, `TestDefaultWallCorroborator_ProbesTheRecipeOfTheBinaryATargetDrives` |
| 2 | HIGH: `ApplySoftOverlay` read a family by name prefix, so an `agy` overlay promoted `agy-claude-tmux` | the family rung matches `Family(entry) == ov.CLI`, and a name is a family selector when it is not a `KnownDriver` | `TestApplySoftOverlay_AnOverlayPromotesTheChainEntryOfItsRoutingFamily` |
| 3 | HIGH: nothing verifies the model agy boots | the guard above, until C5 lands. Round 2: the first guard decoded only `cli_routing.tiers` and found no route in `default`, `work`, `agents` or `clis`, and it re-implemented base resolution (it followed chained bases and counted a `null` deletion or `{}` as a rule). It moved to `go/cmd/evolve`, walks the typed `policy.CLIRouting`, and reads the manifest through `bridge.ManifestObject` | `TestAgyClaudeRouting_WaitsForALaunchTimeModelVerificationRule`, `TestAgyClaudeRouting_TheGuardSeesEachRouteL2CanAdd`, `TestAgyClaudeRouting_OnlyANonEmptyRuleOnTheResolvedTargetCounts`, `TestManifestObject_IsTheMergedManifestTheLoaderParses` |
| 4 | HIGH: `llmroute.cliBinaryFor` and the manifests' `binary` could drift | a parity test at the composition root, both directions, over `bridge.DriverNames()` and the new `llmroute.Drivers()` | `TestDriverBinaryParity_*` (cmd/evolve) |
| 5 | MEDIUM: setup's `familyDriverManifest` duplicated `llmroute.DefaultDriverForFamily` | setup uses `DefaultDriverForFamily`, and `capManifest` reads the family's binary. Side effect: `setup detect` now reports ollama's real tier map, where the old switch reported the bare tier words | `TestTierModelsFor_EveryFamilyReadsItsInteractiveTargetsTierMap` |
| 6 | MEDIUM: `ParseInterspersed` read a flag value of `--` as the terminator | it replays the parsed span and stops on a `--` only at a flag position, so `--model -- pos1 --json` keeps `--json` a flag while `a -- x --json` passes the remainder raw | `TestParseInterspersed_ValueFlagsKeepTheirValueWhereverTheyStand` |
| 7 | MEDIUM: two interspersed-flag idioms (`ReorderArgs`, `ParseInterspersed`) and two hand-rolled copies | filed, not migrated here: inbox `migrate-reorderargs-callers-to-parseinterspersed` | — |
| 8 | `update_argv: null` looked dead | kept; the note now says the target opts out of the agy binary's updater, which only agy-tmux owns; see the landing note above | — |
| 9 | LOW: `TestFamilyMapsDriverToBinary` tested the routing family | renamed `TestFamilyMapsDriverToRoutingFamily` | — |
| 10 | LOW: `manifest_base.go` discards one `json.Unmarshal` error | kept on purpose: `parseManifest` reports the malformed target | `TestManifestBase_AMalformedTargetIsStillReportedAsInvalidJSON` |
