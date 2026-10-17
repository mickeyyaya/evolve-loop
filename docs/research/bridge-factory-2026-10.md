# The agent bridge as the one common layer: an audit of every call that queries or acts on an LLM CLI (research, 2026-10)

> The plan that uses this research: [bridge-factory-2026-10](../plans/bridge-factory-2026-10.md). The design: [agent-bridge-factory](../architecture/agent-bridge-factory.md). The decision record: [ADR-0137](../architecture/adr/0137-one-bridge-factory-resolves-every-llm-cli-and-the-manifest-holds-its-facts.md).

Date: 2026-10-17. Method: a read of the Go module at `57531f436` (main), from each call site that starts or talks to `claude`, `agy`, `codex` or `ollama`. Two reviews checked the line numbers and the counts. Each claim names its file and line. "Synthesis" marks our own inference.

## Table of contents

- [1. The request](#1-the-request)
- [2. The call sites](#2-the-call-sites)
- [3. The factory and the registry](#3-the-factory-and-the-registry)
- [4. Facts of a CLI outside the manifest](#4-facts-of-a-cli-outside-the-manifest)
- [5. Clean-code findings](#5-clean-code-findings)
- [6. Current practice](#6-current-practice)
- [7. Adopt or reject](#7-adopt-or-reject)
- [8. Refinements to adopt](#8-refinements-to-adopt)

## 1. The request

On 2026-10-17 the operator said: "every query and action should go through agent bridge abstract layer for maximizing common layer as the factory model described in design pattern. Review this flow to see if we completely follow the clean code and design pattern guidances and best practice."

## 2. The call sites

**B1** The `/usage` path goes through the bridge. `ControllerFactory.For(family).Do(EventUsage)` reads the manifest's `controls.usage` data (`cmd/evolve/cli_usage_probe.go:42-45`, `cmd/evolve/cmd_clihealth_usage.go:34-41`, `internal/bridge/usage_window_reader.go:10`).
*Implication:* this path is the model for the others. Its CLI facts are data, and one controller reaches the CLI.

| File and line | What it does | Through the bridge? |
|---|---|---|
| `internal/modelquery/recipe.go:25`, `cmd/evolve/cmd_models_live.go:120-129` | captures the `/model` picker through `bridge.CaptureModelPicker` | yes |
| `cmd/evolve/cmd_models_live.go:137-172` | sends the classifier prompt through `bridge.NewEngine(...).Launch` | yes, but line 158 maps `claude` to `claude-p` by name |
| `internal/bridge/clicontrol_adapter.go:39-56`, `internal/bridge/controllerfactory.go:23` | builds a controller: `LoadManifest(family + "-tmux")` | yes, but the name is built by hand |
| `cmd/evolve/cli_health_canary.go:18-43` | runs `LiveSmokeTest(family + "-tmux")` | yes, but the name is built by hand |
| `internal/cli/opscmd/doctor_live.go:23-36`, `:192` | `evolve doctor live`, and usage evidence after a failed probe | yes |
| `internal/modelquery/agy.go:21` | runs `agy models` | **no** |
| `internal/modelquery/ollama.go:19` | runs `ollama list` | **no** |
| `internal/modelquery/effort.go:29` | runs `<cli> --help` and parses the effort flag | **no** |
| `internal/modelquery/exec.go:18-28` | `defaultRunner`: `sysexec.Command` plus a package global `processEnv` | **no**, a second exec path |
| `internal/bridge/doctor.go:137-160` | `bridge doctor --deep`: a switch of argv for each CLI | partly: the bridge runner, but not the driver |
| `internal/bridge/doctor.go:188`, `internal/looppreflight/versioninventory.go:19-29` | `<bin> --version`, twice | the first yes, the second **no** |
| `internal/cliupdate/exec.go:19-36` | runs the manifest's `update_argv` | partly: the argv is data, the exec is outside the bridge |

**B2** The `/model` picker goes through the bridge. `RecipeLister.List` calls the `ModelCapturer` interface (`internal/modelquery/recipe.go:25`). `bridgeModelCapturer` calls `bridge.CaptureModelPicker` (`cmd/evolve/cmd_models_live.go:120-129`). Its parse step is Go code keyed on the CLI name (`internal/modelquery/picker.go:10-75`).

**B3** The model lists bypass the bridge (the `agy.go` and `ollama.go` rows above).

**B4** The effort list bypasses the bridge (the `effort.go` row).

**B5** The deep probe builds its own argv: `claude -p … --model haiku`, `codex exec`, and `agy -p "Reply only: PROBE_OK"` with no `--model` (`internal/bridge/doctor.go:137-160`).
*Implication (synthesis):* the agy probe runs agy's saved default model. On 2026-10-17 that default was gone, so agy used Gemini 4 Argon ([model-tier-policy research](model-tier-policy-2026-10.md), P3.3). Thus each deep probe used quota from the Argon pool.

**B6** `--version` runs in two places: the bridge doctor (`doctor.go:188`) and the loop preflight (`looppreflight/versioninventory.go:19-29`).

## 3. The factory and the registry

**B7** A controller factory exists (`ControllerFactory.For`, `bridge/controllerfactory.go:25`). No factory resolves the driver name and the manifest. The bridge keeps a package global `drivers` (`bridge/driver.go:22`). Each driver file fills it in `init()`: for example `driver_agy.go:60`, `driver_agytmux.go:48-51` and `driver_claudetmux.go:59`.

The `Driver` interface has `Name` and `Launch` only (`bridge/driver.go:10-14`).

*Implication:* the registry is a Service Locator. Any code can call it, it hides a dependency, and a test cannot replace it.

**B8** 19 production sites build a driver name with `family + "-tmux"`. There are 10 in `cmd/evolve`, 6 in `internal/bridge`, 2 in `internal/llmroute` and 1 in `internal/looppreflight`. The count comes from `grep -E '\+ ?"-tmux"'` over the non-test Go files on 2026-10-17.

**B9** The map from family to driver exists twice: `bareDriverMap` (`bridge/driver.go:48-53`) and `cliBinaryFor` (`llmroute/llmroute.go:21-30`). One function is derived from the second map (`llmroute.go:116-121`). `bareDriverMap` has no `agy-claude` and no `ollama`, so `DriverFor("ollama")` returns `ollama`, not `ollama-tmux`.
*Implication:* three copies of one fact give three chances to differ, and one copy already differs.

**B10** Some facts of one CLI exist twice. The agy prompt marker `"? for shortcuts"` is a Go literal (`bridge/driver_agytmux.go:37`) and manifest data (`agy-tmux.json:26`). The exit keys of the tmux drivers are Go literals too (`driver_agytmux.go:42`, `driver_codextmux.go:59`, `driver_claudetmux.go:53`, `driver_ollamatmux.go:61`).

## 4. Facts of a CLI outside the manifest

**B11** Several branches outside the bridge switch on a CLI name:

| File and line | The branch |
|---|---|
| `internal/modelquery/exec.go:37-41` | a lister for each of `ollama`, `agy` and `agy-claude` |
| `internal/modelquery/picker.go:10-75` | a picker parser for each of `codex`, `agy` and `claude`, with the agy screen markers as Go constants |
| `internal/modelquery/effort.go:98-103` | an effort lister for `claude` and `agy` |
| `cmd/evolve/cmd_models_live.go:158` | `cli == "claude"` mapped to `claude-p` |
| `internal/bridge/panestream/liveness.go:195-205` | a liveness detector for each of `claude`, `ollama` and `agy` |
| `internal/bridge/model_dispatch.go:167-169`, `bridge/recipe_adapter.go:119` | special cases for `codex` |

**What is already data.** The usage command, the exhausted text and the window parse (`controls.usage`), `update_argv`, `default_env`, `model_tier_map`, `interactive_prompts`, `model_freshness` and `transient_regex` are manifest fields.

## 5. Clean-code findings

**B12** Three places mix duties or pass 50 lines:

- `Engine.LaunchArgs` (`bridge/launch.go:16-185`) is about 170 lines. It parses flags, loads the profile, resolves the sandbox, handles a dry run, looks up the driver, runs the preflight and launches.
- `cmd/evolve/cmd_models_live.go` holds the catalog stage, the bridge adapters, the refresh wiring and the diagnostic salvage in package `main`. So `wave next` cannot reuse the refresh without an import of `main`.
- `usageprobe.Prober` reads, judges, records and benches on one type.

**What is already right.** `usageprobe.Prober` and `modelquery.Refresh` take injected interfaces (`Lister`, `Classifier`), so a test can replace the CLI. `clicontrol.ErrUnsupported` is a clean sentinel.

## 6. Current practice

**P6.1** (online, 2026-10-17) The Factory Method pattern lets a subclass choose the class to create. The Abstract Factory pattern gives one interface to create a family of related objects ([Refactoring Guru, Abstract Factory](https://refactoring.guru/design-patterns/abstract-factory), [Factory Method](https://refactoring.guru/design-patterns/factory-method)).
*Implication:* a function that takes a family name is an Abstract Factory when it returns four products of that family. Those products are the driver, the controller, the lister and the version reader. Its first form, with fewer products, is a Simple Factory.

**P6.2** A Service Locator hides the dependencies of a class. Dependency Injection makes them explicit, so a test can replace them ([Fowler, Inversion of Control Containers and the Dependency Injection pattern](https://martinfowler.com/articles/injection.html)).
*Implication:* the `init()` registry (B7) is a Service Locator. The fix is an injected `DriverSet`.

**P6.3** In Go, the package that uses an interface defines it. "Accept interfaces, return structs" ([Go Code Review Comments, Interfaces](https://go.dev/wiki/CodeReviewComments#interfaces)).
*Implication:* `modelquery` keeps its `Lister` interface, and the bridge implements it. `modelquery` does not import the bridge.

**P6.4** Current Go guidance (2026) keeps `init()` simple. It advises explicit set-up that returns errors. It warns against work in `init()` that makes tests harder ([OneUptime, init functions in Go](https://oneuptime.com/blog/post/2026-01-23-go-init-functions/markdown)).

Guides for dependency injection in Go pass dependencies through constructors. They reject package-level service locators ([JetBrains, DI in Go](https://jetbrains.com.cn/guide/go/tutorials/dependency_injection_part_one), [Relia Software, DI in Go](https://reliasoftware.com/blog/practical-guide-to-dependency-injection-in-go-hands-on-implementation)). The `database/sql` driver registry is a long-standing exception for libraries. It does not wire the services of an application.
*Implication:* the bridge registry wires an application, so the guidance applies. An injected `DriverSet` follows it.

**P6.5** A study of the GoF patterns maps command names to constructor functions in a table. It names the cost: each new product extends the factory interface ([Schmidt, GoF patterns lecture](https://www.cs.wm.edu/~dcschmidt/cs251/C++/lectures/PDFs/GoF-patterns-8c.pdf)). A manifest-driven CLI keeps the manifest as the reviewed record between the defaults and the flags (a mirror of a dev.to post, [neura.market](https://neura.market/directories/midjourney/blog/devto-3622961)).
*Implication:* a table of constructors fed by the manifest is the known shape. The cost of a wider factory interface is the reason to grow it one product at a time.

## 7. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| One factory for a family (P6.1) | Adopt, as a Simple Factory that grows into an Abstract Factory | B7–B9 |
| An injected `DriverSet` in place of the global registry (P6.2) | Adopt, after the factory | B7 |
| Interfaces that the consumer defines (P6.3) | Adopt | B3, B4: the lister moves into the bridge without a reverse import |
| Move all CLI facts into the manifest in one change | Reject | It touches every driver and every dispatch at once. |
| Guard tests only, with an allow-list | Reject as the whole plan, adopt as a part of each step | It stops new bypasses but leaves the current ones, B5 included. |

## 8. Refinements to adopt

- **R1** (B7–B9, P6.1, P6.2): `bridge.Resolve(set, family)` over an injected `DriverSet` replaces the 19 hand-built names and the two maps.
- **R2** (B5, P6.1): the deep probe runs through the driver of its family, by an optional `Probe` capability.
- **R3** (B3, B4, B11, P6.3): the model list, the effort list and the picker parse become manifest data. Bridge adapters read them and implement the interfaces of the consumer.
- **R4** (B6): one `bridge.Version` serves every caller.
- **R5** (B12): `liveRefresh` moves out of package `main`, and `LaunchArgs` splits by duty.
- **R6** (all): each step adds a guard test that bans the shape it removes.
