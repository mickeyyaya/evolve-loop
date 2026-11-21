# Plan: the handoff judge walks the one seat walker through an independence filter

- **Status:** proposed, 2026-11-21 (review round 3).
- **Design:** [phase-handoff-contract](../architecture/phase-handoff-contract.md). **Parent plan:** [phase-handoff-contract-2026-10](phase-handoff-contract-2026-10.md). **Decision:** [ADR-0141](../architecture/adr/0141-every-phase-handoff-is-a-request-build-check-loop-with-three-exits.md). **Package notes:** [internal-handoff](../architecture/packages/internal-handoff.md) and [internal-bridgechain](../architecture/packages/internal-bridgechain.md).
- **Closes:** the inbox items `handoff-judge-one-seat-walker-ac-1b-console` and `handoff-h10-independent-judge`.

## Table of contents

- [1. Request](#1-request)
- [2. Terms used throughout](#2-terms-used-throughout)
- [3. Facts](#3-facts)
- [4. Root cause](#4-root-cause)
- [5. Candidate designs](#5-candidate-designs)
- [6. Decisions](#6-decisions)
- [7. Files to change or add](#7-files-to-change-or-add)
- [8. Red tests](#8-red-tests)
- [9. Rollout](#9-rollout)
- [10. Risks and limits](#10-risks-and-limits)
- [11. Open questions](#11-open-questions)
- [12. Follow-up items](#12-follow-up-items)

## 1. Request

The harness task of the operator: "Full speed to consume and finish all the features".

The item `handoff-judge-one-seat-walker-ac-1b-console` (rank 57) says: "A lane builder can never change the paths it needs (.evolve/policy.json, .evolve/profiles/handoff-judge.json, go/internal/core/handoff_judge.go, go/cmd/evolve/handoff_judge.go). The lane part ships without it, so the console owns this part."

The item `handoff-h10-independent-judge` (rank 1) says: "For each pair that declares a judge, a fidelity judge runs through the bridge on entries that passed the deterministic layer. Its seat is another family or a stronger tier than the ledger author."

The parent item `handoff-judge-one-seat-walker` (consumed 2026-11-18) demands "Design doc, architecture review, then build". This document is the design.

Acceptance of the two items, with the labels that this document uses:

| Label | Acceptance |
|---|---|
| A1 | A judge seat on the model of the ledger author is refused at stage enforce (red test first). |
| A2 | The judge result for each ask, with the judge identity, is in `handoff-<pair>.json` under `judge`. |
| A3 | A judge runs only on entries that passed the deterministic layer. |
| A4 | The judge, its profile `.evolve/profiles/handoff-judge.json` and `cli_routing.agents.handoff-judge` in `.evolve/policy.json` agree. |
| A5 | The two mutant-kill tests of the judge exist: the empty `Worktree` fallback and the multi-line ask text. |
| A6 | The judge walks `bridgechain.Walking`, so it gets the exit-85 bench and the fresh-session relaunch. |

## 2. Terms used throughout

- **Author:** the CLI, model and tier that wrote the ledger `handoff-<pair>.json`. The reviewer stamps it (`go/internal/core/handoff_persona.go`).
- **Judge:** a model seat that asks "does this evidence meet the meaning of this ask?".
- **Walker:** `bridgechain.Walking`. It walks the CLI and tier chain, benches a CLI after exit 85, and relaunches in a fresh session after a transient fault.
- **Filter:** a `bridgechain.CandidateSpec`, a function `func(cli, tier string) bool`. The walker skips each seat that the filter refuses.
- **Seat:** the CLI driver and tier that the walker launched.
- **Served model:** the model name that the dispatch path launches for a driver and a tier. `bridge.LoadManifest(cli).ModelTierMap[tier]` gives it, after the live catalog overlay.
- **Vendor set:** `policy.Catalog.AllowedFamilies[llmroute.Family(cli)]`, a list of vendor names.
- **Salvage branch:** `dev/cl-c1`. It holds an uncommitted build of 2600 lines of the same feature.

## 3. Facts

**F3.1** The judge does not exist on main. `go/internal/core/handoff_judge.go`, `go/cmd/evolve/handoff_judge.go` and `.evolve/profiles/handoff-judge.json` are absent. `.evolve/policy.json` has no `handoff-judge` key. The spec states the rule at `docs/architecture/phase-handoff-contract.md:169`, and marks the judge guard row `seam done, judge planned` at line 327.
*Implication:* the build is whole, not a repair.

**F3.2** The policy already carries the tier. `HandoffPairConfig.JudgeTier` is decoded and read by no code (`go/internal/policy/handoff.go:17-20`). The pair has one `stage` key (`handoff.go:18`, parsed at `:57`).
*Implication:* `judge_tier` is the one home of the judge tier. It also switches the judge on for a pair. The judge shares the stage of its pair (D8).

**F3.3** The reviewer runs the check and records it, and nothing else (`go/internal/core/handoff_reviewer.go:68-96`). `recordHandoffCheck` copies each key of the existing file, then sets `check` (`handoff_reviewer.go:107-128`). The author controls the file, so an author can write a `judge` key.
*Implication:* the reviewer must delete each `judge` key before it sets its own (D10).

**F3.4** The walker is complete. `Walking.LaunchSeat` (`go/internal/bridgechain/bridgechain.go:96-138`) resolves a plan, walks CLIs and tiers, benches on exit 85 (`:126-128`) and returns the `Seat` that launched. `launchWithTransientRetry` calls `core.LaunchWithFreshSession` (`:383`). A request with `ChainAttempt` set skips the walk (`:97-99`).
*Implication:* a caller that sets `ChainAttempt` opts out of the bench and of the fresh-session relaunch for the whole chain.

**F3.5** The seam for the filter exists. `bridgechain.CandidateSpec` and `WithCandidateFilter` are at `bridgechain.go:56` and `:69`. The filter applies in the walk (`:104-106`) and in the pinned path (`:88-94`).

A filter that refuses every seat launches nothing and returns `llmroute.ErrNoPermittedAttempt`. The tests are in `go/internal/bridgechain/candidate_filter_test.go`. The struct `Walking` holds `inner`, `resolve`, `bench`, `now`, `logf` and `admit` (`:47-54`), all unexported.
*Implication:* the independence rule needs no new filter code. A second walker with another resolver and filter, over the same inner bridge, needs one small export (D2).

**F3.6** A family-and-tier rule exists. `handoff.Independents(author)` returns an unnamed `func(cli, tier string) bool` that is assignable to a `CandidateSpec` (`go/internal/handoff/independents.go:12-24`).

It admits a seat of another family. It admits a same-family seat only at a higher tier. It refuses each seat when the author is empty or `unattributed`.

The test file `go/internal/handoff/independents_test.go` pins 19 cases.
*Implication:* the rule is the right base. It is not the whole rule. F3.12 shows the cases that it admits wrongly.

**F3.7** The cycle wires one walker. `wireOrchestratorDeps` calls `bridgechain.New` once (`go/cmd/evolve/cmd_cycle.go:568-571`). `TestWireOrchestratorDeps_EveryConsumerGetsTheChainWalkingBridge` (`go/cmd/evolve/cmd_cycle_bridgechain_test.go:10`, count check at line 23) counts exactly one call in that function and restricts each use of the raw bridge `br`. The reviewer chain is built in the same function (`cmd_cycle.go:646`).
*Implication:* the judge walker must not call `bridgechain.New` in `wireOrchestratorDeps`, and must not use `br`.

**F3.8** `*Walking` implements `core.Bridge`. A request with `ChainAttempt` set passes straight through it to the raw bridge (`bridgechain.go:97-99`, `:88-94`).
*Implication:* a judge walker that copies the inner bridge of `walked` walks once. It launches through the one inner bridge.

**F3.9** The salvage branch failed review in round 3 for one reason. Its runner sets `ChainAttempt: true` (`dev/cl-c1/go/cmd/evolve/handoff_judge.go:129`) and loops over seats that it picked itself (`dev/cl-c1/go/internal/core/handoff_judge.go:73-86`). It skips the bench and the fresh-session relaunch. It also adds a vendor table, a tier resolver and `IdentityFns` (the round-3 review record of the salvage branch).

Its round-1 finding F2 and round-2 finding C2 were confirmed CRITICAL. The family rule alone admitted a judge on the author model (`dev/cl-c1/DESIGN.md:98-107,199-217`). Its round-2 finding C1 was a catalog key defect, which F3.13 explains.
*Implication:* keep from the salvage branch the profile shape, the reviewer step, the `judge` record and the tests. Drop its seat loop, its vendor table and its persona file. Keep its findings: the family rule is not enough (F3.12), and the model must resolve through the dispatch path (F3.13).

**F3.10** The author tier is not always a tier name. `dispatchAuthor` sets `Tier: rec.RequestedModel` (`go/internal/core/handoff_persona.go:147`), and `Model` from the dispatch record, which can be `unknown`.

`policy.TierRank` accepts the tier names and their aliases (`go/internal/policy/core.go:165-175`). `Independents` ranks the author tier with it, and an unknown tier ranks 0.
*Implication:* the rule fails closed for an author tier that it does not know. Real authors carry a driver name and a tier name, for example `{cli: claude-tmux, tier: balanced}`. The author model must resolve from that pair, as the seat model does (D15).

**F3.11** `llmroute.Family` strips only the driver suffix (`go/internal/llmroute/llmroute.go:252-257`). `agy-claude` and `claude` are two families. Both serve a Claude model.
*Implication:* the family rule can admit a judge of the same vendor as the author.

**F3.12** The live catalog (`runtime/.evolve/model-catalog.json`) makes `Independents` admit a judge on the author model in three cases.

| Case | Author | Admitted seat | Why it is the same model |
|---|---|---|---|
| 1 | `claude-tmux` at deep | `claude` at top | `tier_models` maps both deep and top to `opus`. The row "same family at a stronger tier" (`go/internal/handoff/independents_test.go:23`) admits it. |
| 2 | `agy` at balanced | `agy` at top | `Gemini 4 Hydrogen (High)` and `Gemini 4 Hydrogen (XHigh)` are one model at two efforts. |
| 3 | `agy-claude` at deep | `claude` at deep | `Claude Opus 5.5 (Medium)` and `opus` are two families that serve one vendor. `agy-claude` is in the builder chain. |

The spec demands both rules: "another family, or a stronger tier" and "a seat on the model of the author is refused" (`docs/architecture/phase-handoff-contract.md:169`).
*Implication:* the build must refuse the author model, as well as apply the family-or-tier rule (D15).

**F3.13** The served model and the vendor have homes, and the keys differ.

- The walker hands the filter driver names, for example `claude-tmux`. `evolve cli-routing explain diff-architecture-reviewer` shows the chain `claude-tmux`, `agy-tmux`, `agy-claude-tmux`. `afterChainTail` maps each family to its driver (`go/internal/cliroute/table.go:167-172`).
- `modelcatalog.Catalog.Lookup(cli, tier)` (`go/internal/modelcatalog/catalog.go:50`) is keyed by base CLI (`agy`, `claude`). It ignores the `Source` of an entry. Dispatch reads only live entries (`catalog.go:44,81`) and otherwise the manifest `ModelTierMap`.
- The dispatch path is `bridge.LoadManifest(cli)` (`go/internal/bridge/manifest.go:127-137`). It applies the live catalog over the manifest (`go/internal/bridge/catalog_overlay.go:67-91`). `bridgechain.seatModel(cli, tier)` (`bridgechain.go:258-263`) reads it and falls back to the tier name. It is unexported.
- `clihealth.ModelGroup` (`go/internal/clihealth/clihealth.go:134`) strips the effort suffix.
- `policy.Catalog.AllowedFamilies` (`go/internal/policy/merge_catalog.go:87`) is the catalog-refresh filter (`go/internal/modelquery/query.go:118`). Its key is a family, its value a list, and it is nil when policy has no `catalog` block.
- `.evolve/model-catalog.json` is an untracked runtime file that refreshes every 24 hours. CI and a fresh checkout do not have it.

*Implication:* the model-identity rule needs no new vendor table, and no catalog read of its own. It must use the served model of the dispatch path (an exported `bridgechain.ServedModel`), strip the effort with `ModelGroup`, and read vendors under `llmroute.Family(cli)`. Tests must not need the runtime catalog file (D15, section 8).

**F3.14** `judge_tier` has no effect through the plain resolver. The runner sends it as `Model`, and `bridgechain.DefaultPlanResolver` passes it on as `DefaultModel` (`go/internal/bridgechain/bridgechain.go:153-160`, which also maps `ProjectRoot`, `Env`, `CallerCLI`, `NeedsTools` and `Cycle`). `llmroute.resolveModel` uses the overlay first, then the profile `model_tier_default`, and `DefaultModel` last (`go/internal/llmroute/llmroute.go:117-128`). Each profile must declare a non-empty `model_tier_default` (`go/internal/profiles/profile_model_routing_adversarial_test.go:10`). `LaunchSeat` then overwrites `attempt.Model` with the plan tier (`bridgechain.go:114-117`).

The routing stage `overlayTier` applies `Overlay{Tier}` when routing has no `model` key (`go/internal/cliroute/resolve.go:118-127`).
*Implication:* the judge needs a resolver that sends `judge_tier` as an overlay. It must sit beside `DefaultPlanResolver`, so that the request mapping has one home (D4).

**F3.15** The chain of an agent is not the whole plan. `after_chain` is on by default, and `Table.withAfterChain` appends each family of the allowed set as a universal fallback (`go/internal/cliroute/table.go:147-152`, `afterChainTail` at `:167-172`). The allowed set is `cli_routing.clis` filtered by the profile key `allowed_clis` (`table.go:88-96`). A profile with no `allowed_clis` lets `agy-claude` join the plan.
*Implication:* the profile must set `allowed_clis` (D5).

**F3.16** No production code in core reads `ReviewResult.Retry`. The correction loop runs on `!rr.Approve` and ends only through `ProvenBlocker` or the blocker prover (`go/internal/core/cyclerun_correction.go:41-47`, `go/internal/core/build_blocker_exit.go:120-127`). The blocked check uses `ProvenBlocker` (`handoff_reviewer.go:90-96`).
*Implication:* a rejection without `ProvenBlocker` redispatches the author up to `maxCorrections` times, and the judge runs again in each round (D8).

**F3.17** A second independence rule exists on main: `cliroute.Decision.IndependentOf` (`go/internal/cliroute/independent_judge.go:13`). It is family-only. It has no production caller. `docs/architecture/llm-isolation.md:1726` reserves it for the X16 T1 audit judge.
*Implication:* two homes hold one rule with two semantics (follow-up FU3).

**F3.18** The word "unattributed" has two homes: `core.handoffUnattributed` (`go/internal/core/handoff_persona.go:19`) and `handoff.unattributedAuthor` (`go/internal/handoff/independents.go:10`). If the stamp text changes, `Independents` reads the author as attributed and admits each seat of another family.
*Implication:* the rule fails open. One exported constant must replace both (D18).

**F3.19** The reviewer checks only two pairs: `triage-tdd` and `tdd-build` (`handoff_reviewer.go:41-44`). The spec table declares a judge for `triage-tdd` ("spec fidelity"), `build-audit` ("implementation fidelity") and `intent-scout` ("intent fidelity"). `tdd-build` and `scout-triage` have "none" (`phase-handoff-contract.md:93-97`).
*Implication:* `triage-tdd` is the one pair that has a spec judge and a reviewer check today (D13, D19).

**F3.20** The author can write in the run directory. The builder profile can write `.evolve/runs/cycle-*` (`.evolve/profiles/builder.json`, `write_subpaths`). An author can plant a judge file before the judge runs, and can put a `judge` key in the ledger.
*Implication:* the judge file needs the stale-file defence that every model seat already has (F3.22).

**F3.21** The whole-tree tests are registered by name. `TestPackages_HoldEveryTestThatReadsTheWholeTree` (`go/internal/repocontract/packages_test.go:16`) fails for each cmd/evolve test that reads the tree and is not in `treeReadingSelections` (`go/internal/repocontract/repocontract.go:54`). `dataFileReads` (`datafiles.go:66,76`) lists the data files that tests read. `TestByNameTests_StaysUnderTheCap` holds the cap of 775 names. `repocontract.go` and `datafiles.go` are on `guards.ProtectedSurfaceManifest` as `GateGuarantee` (`go/internal/guards/integrity_surface.go:245-246`).
*Implication:* the build must register its tests (section 7), and the console owns those two files.

**F3.22** Each model seat that decides something has one typed decision file. `phasecontract.FlowDecision` (`go/internal/phasecontract/flow_decision.go:67-73`) holds the artifact name, the shape and the required and allowed keys. The list `flowDecisions` is at `:77`.

`Clear` removes the file before the launch (`:205`). `Read` refuses a file older than the launch (`ErrFlowFileStale`, `NotBefore`) (`:182`). `Validate` refuses prose, trailing data, an unknown key and a missing key. `Resolve` emits `ISOLATION_I8_VIOLATION` (`:119`).

The plan judge uses it (`go/internal/core/phase_judge.go:62-79`) and launches with `ArtifactPath` and `CompletionArtifact`. The table of seats is `docs/architecture/llm-isolation.md` section 12.1.
*Implication:* the judge reply is a flow seat. A bespoke parser and a bespoke stale-file defence copies this home (D11, D20).

**F3.23** Spec section 6 says that the blocker proof is deterministic and holds in two cases: a protected path, or a conflict between asks. "An agent can name a blocker in its ledger. The exit decision accepts it only when the proof holds" (`docs/architecture/phase-handoff-contract.md:250-262`). `provenBlocker` reroutes the committed inbox item to the console with `BLOCKER_ROUTE` (`go/internal/core/build_blocker_exit.go:120-127`). `evolve` reports the host agy as benched (demoted) on 2026-11-21.
*Implication:* a judge seat that cannot launch is neither case. It must not return `ProvenBlocker`, or a transient seat fault turns lane work into console work (D8).

**F3.24** The usage record takes `Phase = req.Agent` (`go/internal/bridge/attempt_telemetry.go:110`). The author lookup `dispatchAuthor` reads the same records.
*Implication:* a judge record with `Agent: "handoff-judge"` must never be read as an author. One test pins it (section 8).

## 4. Root cause

The judge layer is spec text with no code. The one decision of the salvage branch that blocked it was to launch seats by its own loop. A second walker, beside `bridgechain.Walking`, is a second home for the rules of the chain: bench, step-down, relaunch. The fix is to use the one walker and to pass the independence rule to it as data, a filter. The filter is only as true as the model identity that it reads, so that identity comes from the dispatch path.

## 5. Candidate designs

**Approach A: a judge walker per call, derived from `walked`, with the filter.** The runner builds `walked.With(TierOverlayResolver(router, judgeTier), CandidateSpec(independent))` for each judgement, where `independent` is `handoff.Independents` (D15). It calls `LaunchSeat` with `Agent: "handoff-judge"`. It records the `Seat` that the walker returns.

**Approach B: the salvage branch as built.** A seat picker and a runner loop that sets `ChainAttempt`. It adds a vendor table and a tier resolver.

**Approach C: a fixed `cli` in the request, no walker.** The judge calls `walked.Launch` with a CLI that the reviewer chooses from the author (claude author, agy seat). No chain, no filter.

**Approach D: approach A, plus a vendor table in `llmroute`** so that the filter compares the model vendor, not the family.

Scores run from 0 (fails) to 2 (fits).

| Criterion | A | B | C | D |
|---|---|---|---|---|
| A1: a seat on the author model is refused (F3.12) | 2 (with the identity rule of D15) | 2 | 1 (pairing is fixed in code, not checked) | 2 |
| A6: bench and fresh session (F3.4) | 2 | 0 | 1 (`walked.Launch` walks the chain, but not the filter) | 2 |
| One home for each rule (F3.5, F3.9) | 2 | 0 (a second seat loop and a second identity rule) | 1 | 1 (a vendor table is a second belief) |
| One home for the tier (`judge_tier`, F3.2) | 2 | 1 | 2 | 2 |
| No feature flag, no value from the environment | 2 | 2 | 2 | 2 |
| Protected surface kept small | 1 (the 4 paths of the item, plus guard rows) | 0 (also `modelcatalog` and `cmd_cycle.go`) | 2 | 1 |
| CI parity on Linux and Go 1.23 (no new platform call, no new dependency) | 2 | 1 (larger test surface) | 2 | 2 |
| Size of the build | 2 (about 600 lines) | 0 (2600 lines) | 2 | 1 |
| Total | 15 | 6 | 13 | 12 |

Approach A wins. It is the design that the parent item names. It re-uses what main already tests: the filter, the walker and `Independents`.

Approach C loses on A6 and on A1: a hard pairing ignores the routing table. It does not step down when a CLI is out of quota.

Approach D adds a vendor table. The vendor already has a home (F3.13), so D adds only a second belief.

## 6. Decisions

| Id | Decision | Because (facts) | Rejected options and why |
|---|---|---|---|
| D1 | The judge launches only through a `bridgechain.Walking` that carries `WithCandidateFilter` with the independence rule of D15. | F3.4, F3.5, F3.12 | B: a second seat loop skips the bench (F3.9). C: no filter check. |
| D2 | `bridgechain` gets one export: `(*Walking).With(resolve PlanResolver, spec CandidateSpec) *Walking`. It copies `inner`, `bench`, `now` and `logf`, and sets the resolver and the filter. The runner derives the judge walker from the cycle walker `walked` for each judgement, because the author differs for each ledger. The runner parameter is typed `*bridgechain.Walking`, so a raw `br` does not compile. | F3.5, F3.7, F3.8 | A nested `bridgechain.New(walked, ...)`: it needs a copy of the option set, a second resolver and a reader's doubt about double walks. A second `bridgechain.New` in `wireOrchestratorDeps`: the wiring test forbids it. |
| D3 | The runner never sets `ChainAttempt`. | F3.4 | Setting it, as the salvage branch did, turns the walker off. |
| D4 | `handoff.<pair>.judge_tier` is the only home of the tier. `bridgechain` gets one more export, `TierOverlayResolver(router, tier) PlanResolver`, beside `DefaultPlanResolver`. It builds the same `cliroute.Request` and adds `Overlay: llmroute.Overlay{Tier: tier}`, so the `overlayTier` stage applies it. The profile `model_tier_default` exists only for the schema. `cli_routing.agents.handoff-judge` has a `cli` list only and no `model` key. | F3.2, F3.14 | Send the tier as `Model` through `DefaultPlanResolver`: the profile default wins and `judge_tier` has no effect. A resolver of the judge that copies the request mapping: a second home. A tier in routing: two homes for one value. |
| D5 | The chain of `handoff-judge` in `.evolve/policy.json` is `agy`, then `claude`. The profile sets `allowed_clis: ["agy", "claude"]`, and its `cli` and `cli_fallback` agree with the chain. A test resolves the plan through the real router and the real policy, and asserts that no `agy-claude` candidate is in it. The profile envelope has `min: balanced`, `default: deep` and `max: top`. The walker never steps below `balanced`. | A4, F3.9, F3.15 | A chain that lists two CLIs and a profile with no `allowed_clis`: `after_chain` adds `agy-claude`. A profile that names `codex-tmux` while routing names `agy` (the salvage profile): the two disagree. An envelope that admits `fast`: a judge seat on the weakest tier. |
| D6 | The judge record is `Judgement{Seat{CLI, Tier}, Verdicts, Refused, Error, Basis}` under the ledger key `judge`. The seat is the one that `LaunchSeat` returned. `Basis` holds the data that the filter used: the author model, the served model of the seat and the two vendor sets. The runner builds it from the same function calls that the filter makes. `Basis` also holds the tier ceiling of the plan, and the seat timeout comes from the profile or the bridge default. | A2, F3.4, F3.13 | The tier that was asked for: the walker can step down, so the asked tier can be false. A record with the seat only: the catalog refreshes every 24 hours, so a later recompute reads another catalog. |
| D7 | A walker error that wraps `llmroute.ErrNoPermittedAttempt` is `Refused`. Any other launch error, a non-zero exit, or a bad reply is `Error`. At most one of the two is set. | F3.5 | One field for both: the evidence needs the two rates apart. |
| D8 | The judge shares the stage of its pair. It reads `handoff.<pair>.stage`, so it moves to enforce with the check. At both stages the reviewer records `Refused` and `Error`, and the verdict stays. At enforce, only a disagreement acts: `Retry` with the ask ids and no `ProvenBlocker`. `maxCorrections` bounds the rounds. A seat fault is not a blocker (follow-up FU2). | A1, F3.16, F3.23 | `ProvenBlocker` on `Refused` and `Error`: it is neither blocker case of spec section 6. A transient seat fault then reroutes lane work. `Retry` on a refusal: core never reads `Retry`. A separate judge stage key: a second key for no gain. |
| D9 | The reviewer calls the judge only when the check is `pass`, the pair has a `judge_tier`, a runner is wired and one `met` ask exists. It sends only the `met` ask ids. | A3 | Judge all asks: a seat spent on an ask that failed the deterministic layer is waste. |
| D10 | `recordHandoffCheck` always deletes the `judge` key of the file, then sets it from this review when a judgement ran. | F3.3 | Trust the ledger: an author can forge a judge agreement. Delete only a key that "it did not write": the rule has no state to decide that. |
| D11 | The judge reply is a flow seat. `phasecontract.flowDecisions` gets `FlowSeatHandoffJudge = "handoff-judge"` with artifact `handoff-judge.json`, shape object, `Required` and `Allowed` both `["verdicts"]`. The runner reads it through `flow.Resolve` with `FlowCell{NotBefore: launch}`. The legacy function is `advisor.StdoutSpan(resp.Stdout, '{', '}')`, as in the plan judge (`go/internal/core/phase_judge.go:86`). The I8 stage then governs the fallback, as for each other seat. `handoff.CheckVerdicts` checks only the per-ask rules: a missing, an extra and a duplicate ask, and an unknown key of a verdict. | F3.22 | A bespoke strict parser: it copies `FlowDecision.Validate`. A per-seat "no fallback" rule: the I8 stage owns that rule. At shadow, a valid file and a false legacy function set `Disagree` (`go/internal/phasecontract/flow_decision.go:182-199`). `ISOLATION_I8_VIOLATION` then fires on each judgement. |
| D12 | The runner reads the tree from `Worktree`, and from `ProjectRoot` when `Worktree` is empty. The prompt shows the ask text on one line. | A5 | None. Each of the two lines is a surviving mutant of the salvage branch. |
| D13 | `judge_tier` must be `balanced`, `deep` or `top` when the policy loads (a name of `policy.TierNames()` without `fast`). It is also refused on a pair that has no judge in the spec or that the reviewer does not check. The set of judged pairs is one named list in `go/internal/policy/handoff.go`. A test pins it to the pairs of `handoffPairEndingAt` and to the spec table, and refuses `tdd-build`, `build-audit`, `scout-triage` and `intent-scout`. | F3.2, F3.19 | A late failure at the first judge call. A setting on a pair with no judge: a silent no-op. |
| D14 | The judge prompt is a Go template in `go/cmd/evolve/handoff_judge.go`. It carries the rules of the judge, as the plan judge does (`go/internal/core/phase_judge.go:90-107`). It has no persona file. The profile allows `Read`, `Grep`, `Glob`, read-only `git` and one write to `handoff-judge.json` in the run directory. It has no network. It sets `max_budget_usd: 1.0` and `max_turns: 20`, in the shape of `diff-architecture-reviewer.json`. | Isolation rules in `AGENTS.md` | A persona file: it adds a protected path, a skills-generate step and a loader for no gain. A judge that can edit: a judge must not change the evidence. |
| D15 | The whole rule lives in `go/internal/handoff`, as `Independents(author, served, vendors)`. Package main injects the data only: `bridgechain.ServedModel` and `policy.Catalog.AllowedFamilies`. The rule below the table says what it refuses. | A1, F3.12, F3.13 | The rule in package main: two homes for one rule, and `handoff` is not a leaf package (it imports `llmroute` and `policy`). `modelcatalog.Lookup` on a raw driver name: it knows no `-tmux` driver and refuses each seat. A vendor table in `llmroute` (D): a second belief. `Independents` alone: it admits the three cases of F3.12. |
| D16 | The cycle walker and the judge walker share their options by construction: `With` copies the bench, the clock and the log of `walked`. | F3.7 | A shared option helper in `go/cmd/evolve`: it is needed only when two walkers are built from scratch. |
| D17 | `cliroute.Decision.IndependentOf` stays in this change (follow-up FU3). | F3.17 | Retire it here: it is an unrelated item with no caller to move. |
| D18 | `handoff` exports `Unattributed`. `stampLedger` and `Independents` both use it. A test pins that a ledger stamped with it is refused. | F3.18 | Two string copies. |
| D19 | The first shadow pair is `triage-tdd`. | F3.19 | `tdd-build`: the spec declares no judge for it. Amending the spec table: the spec fits the reviewer. |
| D20 | The runner calls `flow.Clear` on `handoff-judge.json` before the launch. It reads the file only through `flow.Resolve` with `NotBefore` set to the launch time. The prompt comes from the ledger with `Judge` nil. The section 12.1 table of `llm-isolation.md` gets a row for the seat. The gate-input inventory gets an `advisory` row while every pair is at shadow. The read can only reject, and H12 reclassifies it for enforce. | F3.20, F3.22 | Read a file that the author may have planted. Class `verified`: the harness does not recompute the content (`gate-input-inventory.md:27`). Class `subject`: the file is a harness-launched verdict about the TDD output, not that output (`gate-input-inventory.md:28`). |
| D21 | `evolve handoff show --cycle N` prints the judge seat, the outcome and the `Basis`. The record alone lets a reader recompute the decision. | A2, D6 | A manual scan of ledger files. The aggregate verb and the signal codes are follow-up FU1. |
| D22 | The build adds entries to `guards.ProtectedSurfaceManifest`, each with a class and a reason: `.evolve/profiles/handoff-judge.json`, `go/internal/core/handoff_judge.go`, `go/cmd/evolve/handoff_judge.go`, `go/internal/handoff/independents.go`, `go/internal/handoff/judge.go`, `go/internal/policy/handoff.go` (`PolicyGuarantee`), `go/internal/core/handoff_reviewer.go`. It adds no entry for `go/cmd/evolve/cmd_cycle.go`. It adds one entry for each guard test file: `go/internal/handoff/independents_test.go`, `go/internal/core/handoff_judge_test.go` and `go/cmd/evolve/handoff_judge_test.go`. | F3.1, F3.21 | A lane-writable rule or guard test: a lane can weaken the judge, or delete the wiring proof together with the wiring, before enforce. Fence `go/cmd/evolve/cmd_cycle.go`: it is the hub of the cycle wiring (872 lines), and no other wiring in it is fenced. ADR-0157 (decisions 3 and 5) fences only the files that enforce a guarantee. It guards a gate call in a lane-editable file with a `gatepins` symbol pin or a protected guard test. A fence turns each lane diff into a `CONTROL_PLANE_VIOLATION` and a held proposal (ADR-0157 decision 8). Spec section 9 as the guard: a spec table is not a guard, and its row predates ADR-0157. |

`seatModel` becomes `ServedModel` plus the tier-name fallback, so the lookup has one copy. The guard of `cmd_cycle.go` is the protected `TestHandoffJudge_IsWiredIntoTheCycleReviewer`. A `gatepins` symbol pin on `wireHandoffJudge(`, in the shape of `sandboxPins`, is optional.

The signature of D15 is `Independents(author Author, served func(cli, tier string) (model string, ok bool), vendors map[string][]string) func(cli, tier string) bool`. `bridgechain.ServedModel` is the manifest-plus-overlay lookup with no tier-name fallback, so an unknown seat gives `ok=false`. A model name is `clihealth.ModelGroup(served(cli, tier))`. A vendor set is `vendors[llmroute.Family(cli)]`. Two seats are of one vendor when their sets intersect. The author resolves from `(author.CLI, author.Tier)`, never from `author.Model`.

A seat passes when the family-or-tier rule admits it and the identity rule does not refuse it. The identity rule refuses four seats:

- a seat when the author or the seat has no served model;
- a seat with an empty vendor set;
- a seat of the same vendor in another family;
- a seat of the same vendor with the same model name.

What changes our mind: shadow data shows a seat on the author model that D15 admitted. The console then fixes the catalog row or the vendor set in the policy, not a table in code.

## 7. Files to change or add

Protected paths are marked with `P`. The column "Guard" names what protects each `P` path.

| Path | Change | Guard |
|---|---|---|
| `.evolve/policy.json` `P` | Add `cli_routing.agents.handoff-judge` with `cli: ["agy", "claude"]` and no `model`. Add no `judge_tier` in the first PR (see section 9). | `ProtectedSurfaceManifest` (already listed) |
| `.evolve/profiles/handoff-judge.json` `P` | New read-only profile (D14, D5). Copy the shape of `diff-architecture-reviewer.json`, and name the copied profile in the build report. | The builder sandbox denies `.evolve/profiles/`. A new manifest entry (D22). |
| `go/internal/phasecontract/flow_decision.go` | `FlowSeatHandoffJudge` and its row in `flowDecisions` (D11). | Existing flow-seat tests (section 8). |
| `go/internal/bridgechain/bridgechain.go` | `(*Walking).With`, `TierOverlayResolver` and `ServedModel` (D2, D4, D15). | Not protected. |
| `go/internal/bridgechain/candidate_filter_test.go` | Update the calls at lines 252 and 266 to the new `Independents` signature with served-model stubs. `go/internal/handoff/independents_test.go` changes the same way, and its rows that the identity rule refuses (row 23, deep to top = opus) change their expected result. | The spec guard row at line 327 keeps the name `TestWalking_IndependentsFilterRefusesTheAuthorsOwnModelAndLaunchesNothing`. |
| `go/internal/handoff/judge.go` `P` | `Seat`, `Verdict`, `Judgement`, `Basis`, `Judgement.Agrees`, `Judgement.Disagreements`, `CheckVerdicts`. No import of `core` or `bridgechain`. | A manifest entry (D22). |
| `go/internal/handoff/types.go` | `Judge *Judgement` with the key `judge,omitempty` on `Ledger`. | |
| `go/internal/handoff/independents.go` `P` | The rule of D15, and the export `Unattributed` (D18). | A manifest entry (D22). |
| `go/internal/core/handoff_persona.go` | `stampLedger` uses `handoff.Unattributed` and drops `handoffUnattributed`. | |
| `go/internal/core/handoff_judge.go` `P` | `JudgeCall{Input, Pair, Author, Asks}`, `HandoffJudgeRunner func(ctx, JudgeCall) (JudgeResult, error)`, `JudgeResult{Seat, Verdicts, Basis}`, `WithHandoffJudge`, `JudgeWired`, `HandoffJudgeWired`, the `judge` step and `judgeRejection`. | A manifest entry (D22). |
| `go/internal/core/handoff_reviewer.go` `P` | Call `judge` after the check. Pass the result to `recordHandoffCheck`. Always delete the `judge` key first (D10). Return `Retry` for a disagreement at enforce (D8). | A manifest entry (D22). |
| `go/cmd/evolve/handoff_judge.go` `P` | The runner, `independentSeat` (the injection of the data sources), the prompt template and the clear-launch-resolve sequence. It builds the judge walker with `walked.With(...)`, so it holds no `bridgechain.New`. | A manifest entry (D22). |
| `go/cmd/evolve/cmd_cycle.go` | One call to wrap the reviewer: `wireHandoffJudge(reviewer, walked, cliRouter, ...)` at line 646. No new `bridgechain.New` in `wireOrchestratorDeps` and no use of `br`. | The file stays lane-editable (ADR-0157). The protected test `TestHandoffJudge_IsWiredIntoTheCycleReviewer` in `go/cmd/evolve/handoff_judge_test.go` guards the call. An optional `gatepins` symbol pin on `wireHandoffJudge(` adds a second guard (D22). |
| `go/cmd/evolve/cmd_handoff.go` | `evolve handoff show` prints `judge.seat`, the outcome and the `Basis` (D21). | |
| `go/internal/policy/handoff.go` `P` | The `judge_tier` checks and the judged-pair list (D13). | A manifest entry (D22). |
| `go/internal/guards/integrity_surface.go` `P` | The entries of D22. | The guard tests of the guards package. |
| `go/internal/repocontract/repocontract.go` and `datafiles.go` `P` | Register by name each new test that reads the tree. Add `dataFileReads` rows for `.evolve/policy.json` and `.evolve/profiles/`, so that the policy-only PR 2 runs the agreement test at ship. | `GateGuarantee` entries at `integrity_surface.go:245-246`. The console owns them. |
| `go/cmd/evolve/testdata/` | Fixtures committed from production rows (section 8): ledgers with the authors `claude-tmux/balanced`, `agy-tmux/balanced` and `agy-claude-tmux/balanced`, a catalog fixture and judge reply files. Update `cli_routing_shipped_routes.golden.json` for the new agent and its `allowed_clis`. | |
| `go/internal/cliroute/testdata/legacy-plans.golden.jsonl` | Update the golden for the new agent. The salvage branch changed 24 lines. | |
| `docs/architecture/phase-handoff-contract.md` | Name `handoff.Independents`, the `judge` key, the stage of the judge, and the record at both stages. Change the guard row at line 327 from `seam done, judge planned` to the real test names. Add the judge files to section 9. In the section 9 row, `handoff_reviewer.go` stays protected and `cmd_cycle.go` is lane-editable with a pinned or guarded call (ADR-0157). Amend line 169 to state the vendor rule: a seat of the author's vendor in another family is refused, where vendor is `catalog.allowed_families[Family(cli)]` (D15). | |
| `docs/plans/phase-handoff-contract-2026-10.md` | Line 179: the judge runs at the stage of its pair, not "only at stage enforce". | |
| `docs/architecture/llm-isolation.md` | Add the seat to the table of section 12.1. | |
| `docs/architecture/gate-input-inventory.md` | Add a row for `handoff-judge.json` with class `advisory` while every pair is at shadow (D20). | |
| `docs/architecture/model-routing-policy.md` | Add the section "Judge seat for the handoff check" (D4, D5). | |
| `docs/architecture/packages/internal-handoff.md` and `internal-bridgechain.md` | Link this plan. | |

The build does not add `modelcatalog` changes, a vendor table, `IdentityFns`, a seat picker, a persona file, a signal code or a new verb.

After the build, run `TestPackages_HoldEveryTestThatReadsTheWholeTree` and `TestByNameTests_StaysUnderTheCap` (cap 775).

## 8. Red tests

Each test fails on main for the right reason: the symbol or the behavior is absent. Two tests are regression pins and pass on main: `TestIndependents_UnattributedIsTheStampedValue` and `TestDispatchAuthor_IgnoresAHandoffJudgeRecord`. The first run must show the failure. Fixtures are copied from production rows and committed under `testdata`. No test reads `.evolve/model-catalog.json` of the runtime. The fixtures use driver names (`claude-tmux`, `agy-tmux`, `agy-claude-tmux`), as production does.

**`go/internal/handoff` (file `independents_test.go`)**

| Test | What it proves | Criterion |
|---|---|---|
| `TestIndependents_RefusesTheAuthorModelByDriverName` | Table cases with the three rows of F3.12, written with `-tmux` drivers and served-model stubs. Each seat on the author model, or of its vendor in another family, is refused. | A1 |
| `TestIndependents_AnAuthorThatDoesNotResolveRefusesEverySeat` | An author with no served model refuses each seat. | A1 |
| `TestIndependents_VendorSets` | A nil map, a missing key, a driver name and a list of more than one entry. A missing or empty set refuses. Two sets that intersect are one vendor. | A1 |
| `TestIndependents_UnattributedIsTheStampedValue` | A regression pin: the value that `core.stampLedger` writes is `handoff.Unattributed`, and the rule refuses it. D18 is enforced by the build, because the removed constant no longer compiles. | A1 |
| `TestLedger_JudgeRoundTripsUnderTheJudgeKey` and `TestJudgement_RefusedAndErrorAreDistinctKeys` | The record shape and D7. | A2 |
| `TestCheckVerdicts_RejectsMissingExtraAndDuplicateAskAndUnknownKey` | Each defect is an error and never a disagreement. | A2 |

**`go/internal/core` (file `handoff_judge_test.go`)**

| Test | What it proves | Criterion |
|---|---|---|
| `TestHandoffReviewer_AtEnforceAJudgeRefusalIsRecordedAndKeepsTheVerdict` | A runner that reports `Refused` at enforce records `judge.refused`, approves, and returns no `ProvenBlocker`. The same for `Error`. | A1 |
| `TestHandoffReviewer_AtShadowARefusalIsRecordedAndKeepsTheVerdict` | The same case at shadow. | A1 |
| `TestHandoffReviewer_JudgeResultAndSeatAreWrittenUnderJudge` | The file holds `judge.seat.cli`, `judge.seat.tier`, `judge.basis` and one verdict for each ask, and keeps `check` and the entries. | A2 |
| `TestHandoffReviewer_JudgeRunsOnlyAfterThePassingDeterministicLayer` | The outcomes `unmet`, `blocked` and `invalid` never call the runner. `pass` calls it once, with only the `met` ask ids. | A3 |
| `TestHandoffReviewer_APairWithNoJudgeTierNeverCallsTheRunner` | An empty `judge_tier` disables the judge. | A3 |
| `TestHandoffReviewer_JudgeFollowsThePairStage` | The same ledger gives a record at shadow and a `Retry` on a disagreement at enforce, from one `stage` key of the pair. | A1 |
| `TestHandoffReviewer_AnAuthorWrittenJudgeKeyIsNeverKept` | A `judge` key in the author file is deleted, with and without a runner. | A2 |
| `TestHandoffReviewer_AtEnforceADisagreementRetriesWithTheAskIds` | A disagreement returns `Retry` that names the ask ids, with no `ProvenBlocker`. | A2 |
| `TestHandoffReviewer_ACheckOnlyStubNeverGetsAJudgeKey` | A file with no author entry gets no judge call. | A3 |
| `TestDispatchAuthor_IgnoresAHandoffJudgeRecord` | A regression pin: `dispatchAuthor` already skips `rec.Phase != phase` (`go/internal/core/handoff_persona.go:138`). A usage record with `Phase: "handoff-judge"` is not read as the author (F3.24). | A2 |

**`go/cmd/evolve` (file `handoff_judge_test.go`).** These tests are runner wiring proofs. Each asserts the recorded `judge.seat` and does not restate a `bridgechain` test.

| Test | What it proves | Criterion |
|---|---|---|
| `TestHandoffJudge_EnforceNeverLaunchesTheAuthorModelSeat` | The test pins `judge_tier: balanced`. The author is `claude-tmux/balanced` and the chain is `agy`, `claude`. The scripted bridge makes `agy` exit 85. The test asserts 0 `claude` launches and the outcome `error` (walled). The refused case belongs to `TestHandoffJudge_NoIndependentSeatLaunchesNothingAndIsRefused`. | A1 |
| `TestHandoffJudge_ProductionAuthorsAreAllResolved` | The three production authors resolve through the real manifest fallback with no catalog file, and the filter admits a seat of another vendor for each. This pins the manifest fallback. | A1 |
| `TestHandoffJudge_ASeatThatIsADriverNameIsFiltered` | The seat `claude-tmux` is a driver name. The filter decides on its served model, not on the raw driver. | A1 |
| `TestHandoffJudge_NoIndependentSeatLaunchesNothingAndIsRefused` | An author with no independent seat in the chain gives `Refused`, and the scripted bridge sees 0 launches. | A1 |
| `TestHandoffJudge_Exit85BenchesTheSeatAndWalksToTheNextIndependentSeat` | With a scripted inner bridge, `agy` exits 85. The bench function runs once for `agy`. The judge walks to `claude`. `judge.seat.cli` is `claude`. | A6 |
| `TestHandoffJudge_TransientFaultRelaunchesInAFreshSession` | Cli-health is on and the other candidates are benched (`bridgechain.go:395-403`). A transient fault relaunches once with `FreshSession` set. `judge.seat` records the relaunched seat. | A6 |
| `TestHandoffJudge_JudgeTierReachesTheLaunchedSeat` | The profile default is deep and `judge_tier` is balanced. The scripted bridge must see `Model` balanced. | A4 |
| `TestHandoffJudge_TheWalkerNeverStepsBelowBalanced` | The envelope floor holds when the chain is out of quota. | A6 |
| `TestHandoffJudge_EmptyWorktreeFallsBackToTheProjectRoot` | With `Worktree` empty, the request is built on `ProjectRoot` (mutant 1). | A5 |
| `TestHandoffJudgePrompt_MultiLineAskTextIsOneLine` | An ask with a newline shows as one prompt line (mutant 2). | A5 |
| `TestHandoffJudgePrompt_CarriesTheRulesOfTheJudge` | The prompt holds the rules text and the artifact path. | A2 |
| `TestHandoffJudgeRunner_APlantedStaleFileIsNotReused` | A planted `handoff-judge.json` is cleared before launch. A file that is older than the launch is refused as stale and never read as the verdict. | A2 |
| `TestHandoffJudgePrompt_IsBuiltFromALedgerWithNoJudgeKey` | A forged `judge` key in the ledger is absent from the prompt. | A2 |
| `TestHandoffJudge_ProfileAndRoutingAgree` | The profile `cli`, `cli_fallback` and `allowed_clis` match `cli_routing.agents.handoff-judge`. The plan, resolved through the real router and the real policy, has no `agy-claude` candidate. The routing entry has no `model`. The test is registered in `repocontract` by name. | A4 |
| `TestHandoffJudge_IsWiredIntoTheCycleReviewer` | `Orchestrator.HandoffJudgeWired()` is true after `wireOrchestratorDeps`. It sits next to `TestHandoffReviewer_IsInTheCycleReviewerChain`. | A6 |
| `TestHandoffShow_PrintsTheJudgeSeatOutcomeAndBasis` | `evolve handoff show` prints them. | A2 |

**`go/internal/bridgechain`:** `TestWalking_WithCopiesTheInnerBenchAndLogAndSwapsResolverAndFilter`, `TestTierOverlayResolver_MapsTheRequestLikeDefaultPlanResolverAndAddsTheTier` and `TestServedModel_UnknownSeatIsNotOK`.

**`go/internal/phasecontract`:** the existing flow-seat table tests (`flow_decision_test.go`) must hold the new row. `TestFlowDecision_HandoffJudgeRefusesProseAnUnknownKeyAndAMissingVerdicts` covers it. A runner test at I8 stage shadow asserts that a valid file gives no `flow.shadow.disagree` event.

**`go/internal/policy`:** `TestHandoffPolicy_UnknownJudgeTierIsRejected` with `"Deep"` and `"fast"`, and `TestHandoffPolicy_JudgeTierOnAPairWithNoJudgeIsRejected` for `tdd-build`, `build-audit`, `scout-triage` and `intent-scout` (D13).

`TestWireOrchestratorDeps_EveryConsumerGetsTheChainWalkingBridge` stays green and unchanged. It is the proof for D2. The bench, fresh-session and filter behavior are proved in `go/internal/bridgechain`. The cmd tests above do not repeat those tests, so the testdup ratchet stays green. The `br` mutant does not compile, because the runner takes `*bridgechain.Walking`, so it needs no test.

Mutation check before review: make three mutants. Set `ChainAttempt` in the runner. Drop the `delete(doc, "judge")` line. Drop the identity rule from `Independents`. Each mutant must fail at least one test above.

## 9. Rollout

Stage: shadow. The reason: the judge is a model, and a model verdict is evidence, not proof (spec, "What a test cannot prove").

1. PR 1 (this design and its build): the code, the profile, the routing entry, the manifest entries and the docs. No pair has `judge_tier`, so the cost is 0 and no live cycle changes. The tests prove the path with a scripted bridge.
2. PR 2 (policy only, console): set `judge_tier: "deep"` for `triage-tdd` at stage shadow. `tdd-build` has no judge in the spec (D19), and the policy loader refuses a `judge_tier` there (D13). Add `build-audit` when the reviewer checks it.
3. Evidence for the flip to enforce: after two waves, each `handoff-triage-tdd.json` whose `check.outcome` is `pass` has a `judge` key. The judge `error` and `refused` counts are read from the `judge` records. The filter decision of each record recomputes from its `Basis` alone (D6). `evolve handoff show --cycle N` prints each record (D21). The aggregate verb over many cycles is follow-up FU1, and the flip item H12 needs it first. The flip is H12 of the parent plan, and this plan does not flip it.

Shadow never changes a verdict. The reviewer tests above pin that. At enforce, the judge changes a verdict only through a recorded disagreement (D8).

## 10. Risks and limits

| Id | Risk | Control |
|---|---|---|
| R1 | The catalog lacks a row for a seat or an author, a vendor set is empty, or two models map to one name. | The rule of D15 refuses the seat (fail closed). The red tests use fixtures from production rows, and one test uses no catalog file. The fix is a catalog or policy row. |
| R2 | A judge call costs a seat for each pair. | Only a pair with `judge_tier` and a passing check pays. The default cost is 0. The profile caps the budget and the turns (D14). |
| R3 | The judge reads text that the author wrote. The author can try prompt injection. | The profile is read-only and has no network. A verdict is evidence for the pass condition, not the only gate. Prompt hardening needs its own security review (open question Q3). |
| R4 | A step-down changes the seat tier. | D6 records the seat that launched, and the filter runs on each `(cli, tier)` try (F3.5). The envelope floor is `balanced` (D5). |
| R5 | CI parity: the tests use a scripted bridge and `t.TempDir`. | The design adds no process call, no platform path and no new dependency. Run the new tests with Go 1.23 (`GOTOOLCHAIN=go1.23.12`) on Linux. |
| R6 | At enforce, a judge seat fault lets a ledger pass with no judge opinion. | The judge is evidence, not proof (spec line 172). The record shows each fault. FU2 gives the typed exit. |

Limits: the design does not prove that a judge is right. It proves three things. The judge seat is another model than the author. The walker serves it. The record is true.

## 11. Open questions

- **Q1.** Does the operator want a judge seat fault to block at enforce now? That amends spec section 6 (F3.23), so it needs an operator decision before H12. This design says no.
- **Q2.** Does a reply fault of one seat deserve a retry on the next independent seat? This design says no (one `Error`, no second seat). The H4 loop cap bounds repeats.
- **Q3.** Prompt-injection hardening of the judge prompt. A separate security review.

## 12. Follow-up items

The acceptance does not need these parts. The console files one inbox item for each.

- **FU1. Aggregate evidence.** An `evolve handoff show --since <cycle>` flag with the judge refused, error and disagree rates, and three signal codes registered through `signalcenter.RegisterCode` in `go/internal/signalcenter` with `docs/architecture/signal-codes.md` regenerated by `evolve signals codes generate`. H12 needs it before the flip.
- **FU2. Typed exit for a judge seat fault at enforce.** A typed infrastructure outcome through the failure adapter, so that it neither charges the item nor routes it to the console (F3.23). It follows Q1.
- **FU3. Converge X16 on the filter.** The X16 T1 audit judge uses the `WithCandidateFilter` filter of D15, and `cliroute.Decision.IndependentOf` retires (F3.17).
