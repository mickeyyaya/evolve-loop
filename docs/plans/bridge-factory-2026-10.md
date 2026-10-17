# Plan: every query and action on an LLM CLI goes through one bridge factory and the manifest data

- **Status:** accepted, 2026-10-17 (the operator approved it).
- **Research:** [bridge-factory-2026-10](../research/bridge-factory-2026-10.md). **Design:** [agent-bridge-factory](../architecture/agent-bridge-factory.md).
- **Decision:** [ADR-0137](../architecture/adr/0137-one-bridge-factory-resolves-every-llm-cli-and-the-manifest-holds-its-facts.md). **Related plan:** [model-tier-policy-2026-10](model-tier-policy-2026-10.md), whose components Q1–Q5 depend on this plan. **Existing design:** [agent-bridge-dispatch-contract](../architecture/agent-bridge-dispatch-contract.md).

## Table of contents

- [1. Request](#1-request)
- [2. Terms](#2-terms)
- [3. Facts](#3-facts)
- [4. Goals and non-goals](#4-goals-and-non-goals)
- [5. Candidate designs](#5-candidate-designs)
- [6. Decisions](#6-decisions)
- [7. Components and order](#7-components-and-order)
- [8. Where each tier-policy component lives](#8-where-each-tier-policy-component-lives)
- [9. Verification](#9-verification)
- [10. Risks](#10-risks)

## 1. Request

On 2026-10-17 the operator said: "every query and action should go through agent bridge abstract layer for maximizing common layer as the factory model described in design pattern. Review this flow to see if we completely follow the clean code and design pattern guidances and best practice."

The flow under review is everything that queries or acts on an LLM CLI. It learns the models and the quota of a CLI, or it probes the CLI. It covers `/model`, `/usage`, the model list, the effort list, the probes and the dispatch.

## 2. Terms

- **Bridge:** the package `go/internal/bridge`. It starts an LLM CLI, talks to it and stops it.
- **Manifest:** the JSON file of one CLI driver in `go/internal/bridge/manifests/`. It holds the facts of that CLI as data.
- **Factory:** one function that takes a family name and returns the objects that a caller needs for that CLI. Its first form is a Simple Factory: it returns the driver name, the manifest and the controller. It grows into an Abstract Factory when it also returns the model lister and the version reader of the family. A caller receives these objects by injection. A caller never calls the registry itself.
- **Service Locator:** a global lookup that any code can call. The current registry is one. It hides a dependency, so a test cannot replace it.
- **Argon pool:** the quota pool of the Gemini 4 Argon model, separate from the other Gemini models (see [ADR-0136](../architecture/adr/0136-each-model-tier-walks-separate-quota-pools-and-agy-runs-argon-medium-at-deep-and-top.md)).
- **Bypass:** a call site that starts or talks to an LLM CLI without the bridge, or that builds a driver name by hand.

## 3. Facts

An architecture audit read the code on 2026-10-17 (main, `57531f436`). Each fact names its file and line.

| Id | Fact | Source (research id) |
|---|---|---|
| B1 | The `/usage` path goes through the bridge. `ControllerFactory.For(family).Do(EventUsage)` reads the manifest's `controls.usage` data. | [research B1](../research/bridge-factory-2026-10.md#2-the-call-sites) |
| B2 | The `/model` picker goes through the bridge (`bridge.CaptureModelPicker`). Its parse step is Go code keyed on the CLI name. | [research B2](../research/bridge-factory-2026-10.md#2-the-call-sites) |
| B3 | The model lists bypass the bridge. `agy models` and `ollama list` run through a second exec path, `modelquery.defaultRunner`. | [research B3](../research/bridge-factory-2026-10.md#2-the-call-sites) |
| B4 | The effort list bypasses the bridge. It runs `<cli> --help` and parses the effort flag in Go. | [research B4](../research/bridge-factory-2026-10.md#2-the-call-sites) |
| B5 | `bridge doctor --deep` builds its own argv for each CLI in a switch. It duplicates the drivers. Its agy probe passes no `--model`. | [research B5](../research/bridge-factory-2026-10.md#2-the-call-sites) |
| B6 | `--version` runs in two places: the bridge doctor and the loop preflight. | [research B6](../research/bridge-factory-2026-10.md#2-the-call-sites) |
| B7 | A controller factory exists (`ControllerFactory.For`), but no factory resolves the driver name and the manifest. The bridge has a global registry (`drivers`, filled by `init()` in each driver file), which is a Service Locator. Its `Driver` interface has `Name` and `Launch` only. | [research B7](../research/bridge-factory-2026-10.md#3-the-factory-and-the-registry) |
| B8 | 19 production sites build a driver name with `family + "-tmux"`: 10 in `cmd/evolve`, 6 in `bridge`, 2 in `llmroute` and 1 in `looppreflight`. | [research B8](../research/bridge-factory-2026-10.md#3-the-factory-and-the-registry) |
| B9 | The map from family to driver exists twice, with one function derived from the second map. `bareDriverMap` has no `agy-claude` and no `ollama`, so `DriverFor("ollama")` returns `ollama`, not `ollama-tmux`. | [research B9](../research/bridge-factory-2026-10.md#3-the-factory-and-the-registry) |
| B10 | Some facts of one CLI exist twice: as a Go literal in the tmux driver and as manifest data. An example is the agy prompt marker `"? for shortcuts"`. | [research B10](../research/bridge-factory-2026-10.md#3-the-factory-and-the-registry) |
| B11 | Several branches outside the bridge switch on a CLI name, for example `cli == "claude"` mapped to `claude-p`, and the liveness detectors. | [research B11](../research/bridge-factory-2026-10.md#4-facts-of-a-cli-outside-the-manifest) |
| B12 | Three functions mix duties or pass 50 lines. `Engine.LaunchArgs` is about 170 lines. `cmd_models_live.go` holds the catalog stage, the bridge adapters and the refresh wiring in package `main`. | [research B12](../research/bridge-factory-2026-10.md#5-clean-code-findings) |

**What is already right.** The usage path (B1) is the model to copy. Its CLI facts are manifest data, and it reaches the CLI through one controller. `usageprobe.Prober` and `modelquery.Refresh` take injected interfaces, so a test can replace the CLI.

## 4. Goals and non-goals

**Goals**

- **G1:** one factory resolves every family. No caller builds a driver name by hand.
- **G2:** every start of, and every talk with, an LLM CLI goes through the bridge. No other package runs an LLM binary.
- **G3:** the facts of a CLI are manifest data. A new CLI needs a manifest, not new Go branches.
- **G4:** a guard test makes each rule permanent.

**Non-goals**

- A new driver, or a change to what a dispatch does.
- The Go 1.27 migration of the drivers, or a new tmux layer.

## 5. Candidate designs

### A. Add the factory and move the facts into the manifest, one step at a time

Add `bridge.Resolve(set, family)`, a Simple Factory over an injected `DriverSet`. It returns the driver name, the manifest and the controller, all from the manifest set. It grows into an Abstract Factory as F3 and F4 add the lister and the version reader. Replace the 19 concatenation sites with it.

Then move each bypass into the bridge: the model list, the effort list, the deep probe and the version check. Each moved fact becomes manifest data. A guard test bans the old shapes after each step.

- **Advantages:** each step is small and has its own test. The usage path (B1) is a proven model to copy. The dispatch path does not change until its own step.
- **Weaknesses:** the old registry stays for a while beside the factory.

### B. Replace the registry and unify the driver interface at once

Replace the `init()` registry with an injected `DriverSet`. Unify the launch driver and the recipe driver into one interface with launch, capture, control and teardown. Move all CLI facts into the manifest in the same change.

- **Advantages:** it gives the cleanest end state, with one interface and no globals.
- **Weaknesses:** it touches every driver and every dispatch at once. A defect then stops all cycles, and the review is very large.

### C. Keep the code, and add guards only

Add guard tests that ban `exec` of an LLM binary outside the bridge, and `+"-tmux"` outside the factory. Leave the current call sites with an allow-list.

- **Advantages:** it is fast, and it stops new bypasses at once.
- **Weaknesses:** the current bypasses stay, so the hidden Argon spend of B5 stays too. An allow-list tends to grow.

### Scored comparison

Scores are 1 (worst) to 5 (best).

| Criterion (weight) | A. Factory, step by step | B. Big change | C. Guards only |
|---|:-:|:-:|:-:|
| Meets G1–G3 (×3) | 5 | 5 | 1 |
| Risk to running cycles (×3) | 4 | 1 | 5 |
| Effort (×2) | 3 | 1 | 5 |
| Stops new bypasses, G4 (×2) | 5 | 4 | 5 |
| **Weighted total (out of 50)** | **43** | **28** | **38** |

**The winner is A.** It is the only design that removes the current bypasses at a low risk. It takes the guard idea from C: each step adds a guard, so a removed bypass cannot come back. B stays as the end state, after A has moved the facts into the manifest. The interface change of B is then smaller.

## 6. Decisions

| Id | Decision | Facts |
|---|---|---|
| D1 | `bridge.Resolve(set DriverSet, family string) (Resolved, error)` is the one factory for a family. `Resolved` holds the driver name, the manifest and the controller. It reads only the manifest set. Code outside the bridge receives narrow interfaces, not the manifest. | B7–B9 |
| D2 | Only the bridge starts an LLM binary. Every other package uses a bridge interface. | B3–B6 |
| D3 | The model list, the effort list and the picker parse become manifest data (`controls.models`, `controls.effort`, `controls.model.parse`). | B2–B4, B10 |
| D4 | The deep probe runs through the driver of its family, so it gets the manifest's model and argv. | B5 |
| D5 | One bridge function runs `--version` for every caller. | B6 |
| D6 | Each step that removes a bypass adds a guard test that bans the shape it removes. | G4 |
| D7 | A function that passes 50 lines splits by duty in the step that touches it. | B12 |

## 7. Components and order

| Id | Component | Red test first | Guard it adds |
|---|---|---|---|
| F1 | D1: `bridge.Resolve` over a `DriverSet` built in `main`, and the 19 concatenation sites move to it, the 6 inside the bridge included | every manifest resolves, `agy-claude` and `ollama` included | no `+"-tmux"` outside `Resolve` |
| F2 | D4: the deep probe through the driver, by a `Probe` capability on an optional interface, as `CLIPreflight` does today. This delivers tier-policy Q4 for the probe. Q4's own test checks the dispatch launch | the agy probe argv has the manifest's `fast` model | no LLM argv literal in `doctor.go` |
| F3 | D2 and D3: a bridge adapter reads `controls.models` and implements the `Lister` interface that `modelquery` already defines. `modelquery` does not import `bridge`. agy first, then ollama | a fixture of the real `agy models` output gives the model list | no `sysexec` of an LLM binary outside the bridge |
| F4 | D5: one `bridge.Version` | both callers get the same version from one fake runner | one `--version` call site |
| F5 | D3: the effort list becomes a bridge capture of `--help` with its parse spec in the manifest. The picker parse also becomes manifest data. The CLI-name branches of B11 move with them | fixtures of the agy and claude screens | no `pickerParsers` map keyed on a CLI name |
| F6 | D7: `liveRefresh` and its adapters move out of package `main` into `internal/modelrefresh`. `LaunchArgs` splits by duty | the moved code keeps its tests green | no function over 50 lines in `bridge/launch.go` |

**The order is F1, F2, F3, F4, F5, F6.**

1. F1 comes first, because every later step uses the factory.
2. F2 comes next, because the probe stops its use of the Argon pool.
3. F3 closes the largest bypass.
4. F4 and F5 follow the F3 pattern and move the last facts into the manifest.
5. F6 comes last, because tier-policy Q5 needs the refresh code outside package `main`.

## 8. Where each tier-policy component lives

The tier-policy plan adds components that query a CLI. Each one must use the bridge, and this table says how.

| Component | Layer | How it stays inside the rule |
|---|---|---|
| P1 Argon (Medium) at `top` | manifest `model_tier_map` | data only |
| P2 catalog tier pin | `internal/policy` and `modelquery.liveTiers` | pure catalog logic. It checks a pin against the models that the lister already returned. It calls no CLI. |
| Q1 usage reader for agy 1.4.0 | manifest `controls.usage.windows`, and `internal/quotastate` | data and parse only. The read is the existing bridge controller (B1). |
| Q2 pool-scoped bench | manifest `scopes` with `per_model`, `internal/usageprobe`, `internal/clihealth` | no CLI call. It reads the windows that the bridge returned. |
| Q3 headroom order | `internal/cliroute`, with a new `HeadroomSource` interface that `cliroute` defines | `cliroute` imports no bridge and runs no probe. An adapter over the stored usage observations implements the interface. |
| Q4 valid model for every agy launch | `internal/bridge` | delivered by F2 |
| Q5 `evolve models capacity` | a new `internal/modelcapacity` package, with a new `CatalogReader` and a new `UsageReader` interface. The verb lives in `cmd/evolve`, and `wave next` calls the package | the package has no `exec`, no tmux and no CLI-name literal. A repo-contract test pins this. |

## 9. Verification

| Goal | Measure | Target |
|---|---|---|
| G1 | `grep -E '\+ ?"-tmux"'` over non-test Go files outside the bridge | 0 sites (13 today outside the bridge, 19 in all, B8) |
| G2 | a guard test over the AST: `sysexec` or `exec` of `claude`, `agy`, `codex` or `ollama` outside the bridge | 0 sites |
| G3 | per-CLI `switch` branches outside the bridge on the query flow (B11) | 0 branches |
| G4 | each guard test fails on a fixture that has the banned shape | red on the fixture, green on the tree |

## 10. Risks

| Risk | Control |
|---|---|
| A manifest parse spec reads a real screen in a different way from the Go parser. | Each move keeps the Go parser until a fixture of the real screen passes. Then the Go parser goes. |
| The factory returns a driver that a caller did not expect. | F1's test resolves every manifest. Each moved site keeps its own tests. |
| A guard bans a legitimate case, such as the bridge's own exec layer. | The guard allows the `bridge` package and its tmux layer, and nothing else. |
| The factory reads the global `init()` registry, which is hidden global state. | `Resolve` takes a `DriverSet`. The global registry only feeds the default `DriverSet` in `main`, and a later step removes it. |
