# X6b: the Claude floor has one home (design addendum, 2026-11)

> Spec of record: [llm-isolation.md](llm-isolation.md) (section on X6, "Status of X6"). Plan: [llm-isolation-2026-10.md](../plans/llm-isolation-2026-10.md) section 6.2. Decision record: [ADR-0154](adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md).
> Inbox item: `isolation-floor-profiles-allowed-clis-as-routing-data` (rank 61).
>
> **Terms used throughout:**
> - **Floor:** the list of agents that run only on one CLI family. It lives in `cli_routing.floors` in `.evolve/policy.json`.
> - **Floor agent:** one of `auditor`, `adversarial-review`, `tdd-engineer`, `spec-verifier`, `spec-verify`, `code-reviewer`.
> - **Ceiling:** the `allowed_clis` list of a profile. The router drops each CLI that is not in it.
> - **Agy-only table:** a `cli_routing` table whose `clis` list is `["agy"]`.
> - **Router ceiling:** the `allowed_clis` of `router.json`. It is not a floor copy (section 2, F2.2).

## 1. Request

The inbox item says: "An agy-only table with floors {} over the tracked profiles still refuses 7 agents: 'rule default leaves no allowed CLI'. The floor is thus held twice, once as routing data and once as profile data."

The console narrowed the scope for review round 3. This design drops the ceiling for the six floor agents only. The router ceiling, the legacy path and the setup rules stay as they are, and the router ceiling is an open operator question (Q1).

## 2. Problem and evidence

**F2.1** The policy holds the floor: `cli_routing.floors.claude` lists six agents (`.evolve/policy.json:107`).
*Implication:* the routing data already forces each floor agent onto Claude.

**F2.2** Six floor profiles also set `allowed_clis` to `["claude"]`: `auditor.json:201`, `code-reviewer.json:8`, `spec-verifier.json:5`, `spec-verify.json:5`, `adversarial-review.json:5` and `tdd-engineer.json:115`. The profile `router.json:95` sets `["agy-claude", "claude"]`. No floor lists the router.
*Implication:* six profiles hold a second copy of the floor. The router ceiling is the only home of a separate operator rule: the router runs Claude models only. It is not a copy.

**F2.3** `Table.allowed` keeps a CLI only when the floor permits it and the profile ceiling permits it (`go/internal/cliroute/table.go:88`, condition at `table.go:92`). The result is the intersection of the table `clis`, the ceiling and the floor.
*Implication:* with `clis` equal to `["agy"]` and `floors` equal to `{}`, the ceiling `["claude"]` leaves an empty set. `selectRule` then returns `RuleEmpty` (`table.go:142`, `rule_error.go:23`).

**F2.4** Other readers use the raw ceiling and do not use the table:
- `policy.ValidatePin` refuses a pin outside the ceiling (`go/internal/policy/core.go:149`). The legacy path calls it (`go/internal/cliroute/legacy.go:74`). Setup and the registrar call it too (`go/internal/setup/setup.go:226`, `go/internal/setup/apply.go:41`, `go/internal/phaseregistrar/registrar.go:96`).
- `setup.readProfileConstraints` reads the raw ceiling into the data that the setup presets choose from (`go/internal/setup/setup.go:274`).
- `Decision.Allowed` reports `prof.AllowedFamilies()` on the legacy path (`legacy.go:63`).
*Implication:* a plain deletion of the six ceilings opens a hole. `ValidatePin` can then accept a pin of `auditor` to `codex`, and a setup preset can assign `auditor` or `tdd-engineer` to a non-Claude family.

**F2.5** `TestClaudeFamilyFloor` reads the floor from `policy.DefaultFloors()`. It also reads the ceiling of each floor profile and fails when the ceiling admits the builder family (`go/internal/profiles/family_floor_test.go:19`, loop at lines 62 to 67).
*Implication:* acceptance 3 needs this second read to go. After the drop the loop runs zero times, so it is dead code.

**F2.6** The golden file `go/cmd/evolve/testdata/cli_routing_shipped_routes.golden.json` holds `rule`, `allowed` and `notes` for each agent. The reviewer of round 2 dropped only the six floor ceilings on a copy of the tracked tree. The golden stayed byte equal, and `cli-routing check` over the agy-only policy reported one error, `agent.router`.
*Implication:* narrowing to the six floor agents keeps AC2 literal, and the router row cannot change.

### Root cause

The floor has two homes. The floor says "this agent runs on Claude". The profile ceiling says the same. The table treats both as hard filters. Thus the profile copy can veto a table that the policy allows.

## 3. Approaches

| Id | Approach | Description |
|---|---|---|
| A | Drop the six floor ceilings, clamp from the floor | Remove `allowed_clis` from the six floor profiles. One policy helper returns a profile copy with the floor family as ceiling. `Compile` and `setup` call it. |
| B | Ceiling that floors can widen | Keep `allowed_clis`. When `floors` is `{}`, the table ignores the ceiling of an agent that the floor once held. |
| C | Generate the ceiling from the floor | A generator writes `allowed_clis` into the profiles from `floors`. |
| D | Drop the six ceilings, no clamp | Remove `allowed_clis`. Rely on `Table.allowed` only. |

Score each approach from 0 (fails) to 2 (meets).

| Criterion | A | B | C | D |
|---|---|---|---|---|
| AC1: agy-only, `floors` equal to `{}`, check passes for the six agents | 2 | 1 (needs memory of the old floor) | 0 (the generated file still names Claude) | 2 |
| AC2: every route and the golden keep their value | 2 | 2 | 2 | 2 |
| AC3: the profile guard reads the routing data only | 2 | 0 | 0 | 2 |
| One home for each rule | 2 | 0 | 0 | 2 |
| No feature flag, no environment value | 2 | 1 | 2 | 2 |
| Pin, legacy and setup paths keep the floor (F2.4) | 2 | 1 | 2 | 0 |
| Smallest change to the protected profiles | 2 (six files) | 2 | 1 | 2 |
| CI parity, Linux and Go 1.23 | 2 | 2 | 1 | 2 |
| **Total (of 16)** | **16** | **9** | **8** | **12** |

## 4. Decision

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Choose approach A. Remove `allowed_clis` from the six floor profiles only. Keep `router.json` and the router ceiling. | F2.1, F2.2, F2.3, F2.6 | B: it changes the `cliroute` contract for all profiles and keeps two homes. C: a generated copy still blocks an agy-only table (F2.3). D: it opens the pin and setup hole (F2.4). Router drop: the after-chain adds `agy-tmux` to the router chain, which breaks the operator rule (Q1). |
| D2 | Add `func (f Floors) Clamp(agent string, p profiles.Profile) profiles.Profile` to `go/internal/policy`. It returns a copy whose `AllowedCLIs` is the floor family when the agent is on a floor and the profile has no ceiling. Otherwise it returns the profile unchanged. | F2.4 | A new `ValidatePin` parameter: it changes the signature at four call sites. |
| D3 | `cliroute` snapshots, `setup.readProfileConstraints`, `setup.withPolicyPin` and `setup.apply` call `Clamp` with the floors of the loaded policy. The legacy path then reads the clamped snapshot, so its pin text stays `[claude]`. | F2.4 | Clamp inside `ValidatePin` only: setup chooses from the raw ceiling before it validates. |
| D4 | `TestClaudeFamilyFloor` drops the ceiling loop. A new test asserts that each floor profile holds no `allowed_clis`. Both read `policy.Load(<root>/.evolve/policy.json)` and use `.Floors()`, which falls back to the defaults. | F2.5 | Keep the loop: it is dead after the drop. Read `DefaultFloors()`: a floor agent that only `policy.json` adds escapes the guard. |
| D5 | A floor profile that still sets `allowed_clis` is not a `Compile` finding. The tracked-tree test of D4 guards the shipped data. `RuleEmpty` already reports a private ceiling that vetoes a widened floor. | F2.3 | An error finding: it breaks about 11 test files with fixtures (`go/internal/cliroute/helpers_test.go` `syntheticProfilesJSON` and others). It also breaks each user table with a private floor profile copy. |
| D6 | Rollout is enforce at once, after the evidence of section 8 shows zero route changes. | section 8 | Shadow: a data move has nothing to compare, because the golden is the oracle. |

### Consequences

- Good: an agy-only table with `floors` equal to `{}` compiles for the six floor agents over the tracked profiles.
- Good: a change of the floor in `.evolve/policy.json` takes effect without a profile edit.
- Good: the golden, the legacy goldens and the router row stay byte equal.
- Bad: an agy-only table still refuses the router (`agent.router`). This is the fail-closed result for a seat that must use Claude models only. AC1 holds for the six floor agents.
- Change the decision if: the operator frees the router (Q1), or a second family needs a floor with two members.

## 5. Files to change

| File | Change |
|---|---|
| `.evolve/profiles/auditor.json`, `adversarial-review.json`, `tdd-engineer.json`, `spec-verifier.json`, `spec-verify.json`, `code-reviewer.json` | Remove the `allowed_clis` key. Fix the `_note` text of `auditor.json` that says `allowed_clis=all`. |
| `go/internal/policy/routing_defaults.go` | Add `Floors.Clamp` (D2). |
| `go/internal/cliroute/compile.go` | Call `Clamp` when it builds the profile snapshots. Keep each function under 50 lines. |
| `go/internal/setup/setup.go`, `go/internal/setup/apply.go` | Call `Clamp` in `readProfileConstraints`, `withPolicyPin` and `apply` (D3). |
| `go/internal/profiles/family_floor_test.go` | Remove the ceiling loop (lines 62 to 67). Read the effective floor (D4). |
| `go/internal/profiles/floor_profiles_hold_no_ceiling_test.go` (new) | Test of D4. |
| `go/internal/policy/floors_clamp_test.go` (new) | Tests of D2. |
| `go/internal/cliroute/agy_only_realtree_test.go` (new) | The agy-only proof over the tracked tree. |
| `go/internal/setup/` test (new) | The setup proof of D3. |
| `docs/architecture/packages/internal-profiles.md`, `internal-setup.md`, `internal-cliroute.md` | State that a floor agent holds no ceiling, and name `Clamp`. Fix the `TestClaudeFamilyFloor` text at `internal-profiles.md:40`. |
| `docs/operations/runtime-reference.md` (lines 160 and 228), `AGENTS.md:395`, `skills/setup/SKILL.md:49` | Say that the floor comes from `cli_routing.floors`, not from `allowed_clis`. Run `evolve skills generate` if the AGENTS.md digest changes. |
| `docs/architecture/llm-isolation.md`, `docs/plans/llm-isolation-2026-10.md` (section 6.2) | Mark X6b built in "Status of X6". |

The six profiles are a protected surface. The change needs a full seat review and an adversarial review. The files that the change must keep byte equal: `go/cmd/evolve/testdata/cli_routing_shipped_routes.golden.json` and `go/internal/cliroute/testdata/legacy-plans.golden.jsonl` (test `TestResolve_AbsentTableMatchesLegacyGolden`). Run `evolve docs ste-lint --strict` on each edited document.

## 6. Tests

Red tests fail on `origin/main` and pass after the build. Record the red output for these only.

| Test | Package | What it proves | Acceptance |
|---|---|---|---|
| `TestCompile_AnAgyOnlyTableRoutesTheSixFloorAgents` | `go/internal/cliroute` | The policy is `cli_routing = {"clis":["agy"],"default":["agy"],"floors":{}}`, the other keys unchanged, in a temp root with the tracked `.evolve/profiles` and `docs/architecture/phase-registry.json`. No error finding names a floor agent. The router finding is the only error. | AC1 |
| `TestCLIRouting_CheckOnAnAgyOnlyTableReportsOnlyTheRouter` | `go/cmd/evolve` | `evolve cli-routing check` through `runCLIRouting` on that root reports exactly one error, `agent.router`. Today it reports 7. | AC1 |
| `TestFloorProfiles_HoldNoAllowedCLIs` | `go/internal/profiles` | Each name in the effective floor has no `allowed_clis` in the tracked tree. | AC3 |
| `TestResolve_ALegacyPinOutsideTheFloorIsRefusedWithoutAProfileCeiling` | `go/internal/cliroute` | With no ceiling, a no-table pin of `auditor` to `agy` fails with `cli "agy" not in allowed_clis [claude]`. | AC2 |
| `TestSetupApply_RefusesAFloorRolePinOutsideClaude` | `go/internal/setup` | `setup apply` over the tracked profiles with only codex and agy authed does not emit an `auditor` or `tdd-engineer` pin outside Claude. | AC2 |

Regression guards pass today and must stay green:

| Test | What it guards |
|---|---|
| `TestCLIRoutingShow_EveryShippedProfileKeepsItsResolvedRoute` | The shipped-routes golden stays byte equal, router row included. |
| `TestResolve_AbsentTableMatchesLegacyGolden` | The legacy goldens stay byte equal. |
| `TestRouter_EveryLaunchOfTheAdvisorStaysOnClaudeModels` (`deep_tier_never_gemini_pro_test.go:111`) | The router never routes to Gemini. |
| `TestClaudeFamilyFloor` | A Claude profile outside the floor still fails. |
| `TestCompile_AFloorAgentFollowsTheFloorWhenFloorsAreRestored` | With the shipped floors and `clis` equal to `["agy", "claude"]`, the six agents route to `claude` only. It passes today, because the floor term already restricts. |

## 7. Worked example

The policy sets `clis` to `["agy"]` and `floors` to `{}`. The auditor profile has no `allowed_clis`.

1. `Clamp` finds no floor entry for `auditor`. The snapshot keeps an empty ceiling.
2. `Table.allowed("auditor")` returns `["agy"]`.
3. The `default` rule keeps `agy-tmux`. The check passes for the auditor.

Now restore `floors` to `{"claude": ["auditor", ...]}` and `clis` to `["agy", "claude"]`.

1. `Clamp` writes `["claude"]` into the auditor snapshot.
2. `Table.allowed("auditor")` returns `["claude"]`. A pin to `agy` fails in `ValidatePin`.

## 8. Rollout and evidence

- Stage: enforce at once. No feature flag exists.
- Evidence before merge, all with zero route changes:
  - the shipped-routes golden diff is empty;
  - the `legacy-plans.golden.jsonl` diff is empty;
  - `evolve cli-routing explain auditor` and `explain tdd-engineer` print the same text before and after. Do this on the tracked tree, and on a no-table root with a non-Claude pin, which stays refused;
  - `evolve setup recommend` and `apply` output on the tracked profiles is the same before and after.
- Evidence after merge: the first wave log shows `cli=claude-tmux` for each floor agent.
- Gates: `make test test-integration test-e2e test-acs-durable apicover-enforce cover-strict` with `-count=1`, a Go 1.23 build, and the whole-tree ratchets.

## 9. Risks

| Id | Risk | Control |
|---|---|---|
| R1 | A new floor agent without a floor entry runs unclamped. | `TestFloorProfiles_HoldNoAllowedCLIs` plus `TestClaudeFamilyFloor`, which fails on a Claude profile outside the floor. |
| R2 | A caller of `ValidatePin` outside `cliroute` passes a raw profile. | D3 lists the setup callers. `phaseregistrar/registrar.go:96` validates a new phase, not a floor agent, so it needs no clamp. The setup test is the proof. |
| R3 | A profile outside the tracked tree sets `allowed_clis` for a floor agent. | `Clamp` keeps a private ceiling. `RuleEmpty` reports the veto with a clear message (D5). |

## 10. Open questions

- Q1 (open, for the operator): can the router ceiling go to routing data too?
  - Today the ceiling `["agy-claude", "claude"]` keeps the router on Claude models.
  - Without it, `Table.allowed(router)` becomes every CLI in `clis`. The after-chain stage then adds `agy-tmux` (`go/internal/cliroute/table.go:147`), a Gemini model at deep.
  - That breaks the rule of commit e3874ecd3 and `TestRouter_EveryLaunchOfTheAdvisorStaysOnClaudeModels`. It also changes the legacy pin and advisor rows (`legacy.go:63`, `legacy.go:133`).
  - A free router needs a routing-data home that limits the after-chain, for example a floor with two families for one agent. That changes `Floors.FamilyOf` and its consumers, not an `agents` entry.
  - Until the operator decides, the router keeps its ceiling.

## 11. Follow-up (not in this design)

- Free the router ceiling after Q1: a floor shape for one agent on two families, with the legacy goldens updated in the same change.

## 12. Limits

- The design does not change the floor itself, the family arrangement or any tier.
- An agy-only table still refuses the router.
- A test over the tracked tree cannot prove the behavior of a private profile.
