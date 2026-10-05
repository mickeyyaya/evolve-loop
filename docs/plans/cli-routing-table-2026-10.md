# CLI routing table — design and landing plan (2026-10-05)

> Status: approved by the operator 2026-10-05. Implementation tracked by ADR-0119 (lands with L1). This file is the design record; the ADR states the decision.


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

A source-scan guard, `TestOnlyCliRouteBuildsChains`, bans `llmroute.Resolve`, `ApplyUniversalFallback` and `resolvellm` role resolution outside `cliroute`. It follows the ADR-0104 precedent, so no future launch path can bypass the table.

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
10. **Delete the old keys:** `pins`, `workflow.universal_fallback*` and `router.cli/model`. Their presence becomes a load error that points to `migrate`.
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
