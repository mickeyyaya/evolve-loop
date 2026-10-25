# LLM isolation: where the pipeline is not yet the only controller, and what current practice says (research, 2026-10)

> The plan that uses this research: [llm-isolation-2026-10](../plans/llm-isolation-2026-10.md). The design: [llm-isolation](../architecture/llm-isolation.md). The decision records: [ADR-0154](../architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) and, for §5, [ADR-0155](../architecture/adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md), and, for §6, [ADR-0156](../architecture/adr/0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md).

Date: 2026-10-25. Method:

- A read-only audit of `origin/main` on 2026-10-25 by three explorers. A second pass on the same day re-read each cited `path:line` on `origin/main` at `9fd56a0e0`. §1.13 lists each citation that moved or was wrong.
- 28 live web searches over six topics for §1 to §4, and 20 more over five topics for §5.
- 6 more live web searches over five topics for §6. The search tool gave excerpts and summaries. No page was fetched in full, so each outside claim is as strong as its excerpt.
- Each outside source was accessed on 2026-10-25.
- Each source has a mark: **P** is primary (an official document, a specification, a paper or a commit). **S** is secondary or weak (a blog, a forum, a mirror or a vendor explainer).

"Synthesis" marks our own inference.

## Table of contents

1. [Local evidence](#1-local-evidence)
2. [Current practice](#2-current-practice)
3. [Adopt or reject](#3-adopt-or-reject)
4. [Refinements to adopt](#4-refinements-to-adopt)
5. [Self-healing within boundaries](#5-self-healing-within-boundaries)
6. [The CLI service contract and the control flags](#6-the-cli-service-contract-and-the-control-flags)

## 1. Local evidence

The rule that the operator gave on 2026-10-25: "the pipeline evo loop itself should the center of invoking all LLM cli by design... There is no dependency between a LLM to control over another LLM directly." Each finding below is one place where the code does not yet follow this rule. F1.1 to F1.11 are the gaps G1 to G11 of the plan.

### F1.0 What already holds

The phase order, the transitions and the typed phase contracts already put the pipeline in control. The Go state machine in `go/internal/core` picks each next phase, and no LLM picks it. The bridge (`go/internal/bridge`) launches the phase agents through `Engine.LaunchArgs` (`go/internal/bridge/launch.go:17`). The cycle broker of ADR-0129 carries each agent message, and the broker stamps the sender.
*Implication:* the refactor closes gaps at the edges. It does not change the phase structure.

### F1.1 (G1) Each LLM child inherits the full environment of the launcher

- `driverEnv` starts from `os.Environ()` and removes only the dispatch tag (`go/internal/bridge/driver_common.go:23-32`). `dispatchEnv` adds the dispatch id to it (`go/internal/bridge/dispatch_reap.go:54-60`).
- `envMap` in the bridge command also starts from `os.Environ()` (`go/cmd/evolve/cmd_bridge.go:631-640`), and `LaunchArgs` gets it (`:87`).
- A tmux session starts with `new-session` and no `-e` flag (`go/internal/bridge/tmux.go:111`, `:120`). A pane therefore gets the environment of the tmux server.
- A test pins the inheritance: `TestProcessEnv_IsTheProcessEnvironmentPlusTheFamilyManifestsDefaultEnv` (`go/internal/bridge/process_env_test.go:25-39`).
- `ipcenv.Scrub` (`go/internal/ipcenv/scrub.go:10`) removes `EVOLVE_` variables. It is a deny list, and no bridge path calls it.

*Implication:* `CLAUDECODE`, `CLAUDE_CODE_*`, `GEMINI_*` and `CODEX_HOME` of the launcher reach each child. A child `claude` that inherits `CLAUDECODE` refuses to start or hangs (P2.7).

### F1.2 (G2) Seven launch paths start an LLM binary outside the bridge launch path

| Path | Location | What it skips |
|---|---|---|
| doctor deep probe and version | `go/internal/bridge/doctor.go:157`, `:188` | the sandbox; it calls `deps.Runner` with `driverEnv` |
| usage controller | `go/internal/bridge/controllerfactory.go:23-33` | it builds a `Config` with `AllowBypass: true` |
| recipe sessions (control capture, model capture) | `go/internal/bridge/recipe_adapter.go:40-61`; callers `:146`, `:161`, `go/internal/bridge/capture_models.go:16` | `unset TMUX` and the sandbox. `unset TMUX TMUX_PANE` exists only in `driver_tmux_boot.go:14`, `:133`. |
| cli update | `go/internal/cliupdate/exec.go:28` | the sandbox; `updaterEnv` inherits fully |
| boot smoke and live smoke | `go/internal/bridge/bootsmoke.go:30`, `go/internal/bridge/livesmoke.go:54` | `LaunchArgs`; they call `d.Launch` directly |
| version inventory | `go/internal/looppreflight/versioninventory.go:22` | the bridge; `sysexec.Command(ctx, bin, "--version")` |
| model queries | `go/internal/modelquery/exec.go:19` | the bridge; `sysexec.Command(ctx, name, args...)` |

The guard `TestPipelineCode_StartsEveryProcessThroughCommand` (`go/internal/sysexec/command_guard_test.go:19`) checks only that a process starts through `sysexec.Command`. No test checks that only the bridge starts `claude`, `agy`, `codex` or `gemini`.
*Implication:* each path has its own environment and its own confinement. A rule for the cell cannot hold when seven paths bypass it.

### F1.3 (G3) A phase agent can start another LLM or a nested dispatch

- `evolve agent run` refuses when the dispatch depth is above 0 (`go/internal/cli/agentcmd/run.go:93-94`).
- The depth variable is `EVOLVE_DISPATCH_DEPTH` (`go/internal/subagent/recursion.go:14`). Only the fan-out worker sets it (`:51`, through `dispatchparallel.go:184`). No code in `go/internal/bridge` sets it, so a normal phase pane has depth 0, and the refusal does not fire.
- The SBPL preamble is `(deny default)` and then an unconditional `(allow process-exec)` (`go/internal/adapters/sandbox/sandbox.go:149-152`). No rule denies the exec of `claude`, `agy`, `codex` or `gemini`.
- `EVOLVE_SANDBOX=off` turns the wrapper off (`go/internal/config/env.go:19`, `:66`; `go/internal/bridge/sandbox_wrap.go:20-34`). The bridge logs "host opt-out honoured; runs UNCONFINED".

*Implication:* a phase agent can be a controller of another LLM. This is the dependency that the operator forbids.

### F1.4 (G4) An agent can read the raw output of other agents and the runs of other cycles

- The bridge writes `<agent>-stdout.log` into the shared workspace (`go/internal/bridge/engine.go:341`). The tmux driver writes `-final-pane.txt` there (`go/internal/bridge/driver_tmux_prepare.go:69`).
- The sandbox gives reads of the whole repo and of `$HOME` (`go/internal/adapters/sandbox/sandbox.go:164-182`: `cfg.RepoRoot` at `:166`, `(allow file-read* (subpath HomeDir))` at `:175-176`).
- The write fence covers the runs of other cycles (`go/internal/bridge/sandbox_paths.go:161-201`, `refuseRunDirsReach` at `:185`). No fence covers their reads.
- The auditor persona reads the upstream reports straight from the workspace (`agents/evolve-auditor.md:46`, `:53`, `:82`).
- The handoff reviewer makes the next agent meet the asks of the previous agent "by its ask id, in handoff-%s.json" (`go/internal/core/handoff_reviewer.go:55-71`, text at `:66`).

*Implication:* the pipeline does not choose what an agent sees. A prompt injection in one report reaches each later agent, and an agent can read secrets in `$HOME`.

### F1.5 (G5) The Claude floor and the flow defaults are Go constants

The floor is a Go map: `claudeFamilyFloor` lists `auditor`, `adversarial-review`, `tdd-engineer`, `spec-verifier`, `spec-verify` and `code-reviewer` (`go/internal/profiles/family_floor.go:5-12`). It is read in five places:

| Site | Location |
|---|---|
| the routing table | `go/internal/cliroute/table.go:88` |
| the compiler refuses a table without `claude` | `go/internal/cliroute/compile.go:126-127` |
| the findings | `go/internal/cliroute/findings.go:153` |
| the routing run | `go/cmd/evolve/routing_run.go:67` |
| the model failover chain | `go/internal/bridge/model_failover.go:59-78` |

The flow defaults that name a Claude driver or model:

| Default | Location | Value |
|---|---|---|
| phase advisor | `go/internal/core/phase_advisor.go:73` | `claude-tmux`, `opus` |
| failure advisor | `go/internal/core/failure_advisor.go:54` | `claude-tmux`, `opus` |
| plan judge, with no override | `go/internal/core/phase_judge.go:28` | `claude-tmux`, `haiku` |
| router | `go/cmd/evolve/router_dispatch.go:27` | `opus` |
| legacy route | `go/internal/cliroute/legacy.go:22`, `:25` | `claude-tmux`; classifier families |
| contract escalation | `go/internal/core/contract_escalation.go:22`, `:75`, `:88` | `claude-tmux` |
| plan fallback | `go/internal/llmroute/llmroute.go:153` | `claude-tmux` |
| subagent default | `go/internal/subagent/subagent.go:142` | `claude-tmux` |
| observer | `go/internal/adapters/observer/core_adapter.go:178` | `claude-tmux` |
| postmortem | `go/cmd/evolve/cmd_postmortem.go:22` | `claude-tmux` |
| wave health drivers | `go/internal/policy/policy_wave.go:26` | `claude-tmux` |
| phase default model | `go/internal/phases/audit/audit.go:69`, `buildplanner/buildplanner.go:57`, `swarmplan/swarmplan.go:44`, `debugger/debugger.go:107` | `opus` |
| unmeasured headroom order | `go/internal/cliroute/headroom.go:21` | `claude` first |

The bare-name map sends `gemini` to `claude-tmux` with no error (`go/internal/bridge/driver.go:48-53`, value at `:50`).
*Implication:* a policy change cannot move these seats. A family name in flow code makes one vendor a hidden controller of the flow.

### F1.6 (G6) The kernel guards exist only as Claude Code hooks

`.claude/settings.json` registers four PreToolUse hooks: `guard ship` (matcher `Bash`, `:6`), `guard phase` (matcher `Agent|Task`, `:15`), `guard role` (matcher `Edit|Write`, `:24`) and `guard docdelete` (matcher `Bash`, `:33`). Only Claude Code reads this file, so agy, codex and ollama panes get no hook. The Go phase guard checks only the tool name `Agent` (`go/internal/guards/phase.go:22`). No non-test Go code names `Task`, so a `Task` call passes.
*Implication:* a guarantee that lives in one vendor's hook is not a guarantee for the other families.

### F1.7 (G7) Five flow-steering outputs are parsed loosely

| Output | Location | Method |
|---|---|---|
| advisor proposal and phase plan | `go/internal/core/advisor/parse.go:34`, `:60` (helper `LastBalancedSpan` at `:94`) | the last balanced JSON span in stdout |
| plan judge | `go/internal/core/phase_judge.go:84`, `:88` | the last balanced span; a private copy of the helper at `go/internal/core/phase_advisor.go:165` |
| retry adjudicator | `go/internal/core/retry_adjudicator_agent.go:70`, `:75` | the last balanced span |
| failure advisor | `go/internal/core/failure_advisor.go:149`, `:165` | JSON for each line, then the trimmed text |
| router digest | `go/internal/router/digest.go:284`, `:333`, `:345` | prose header lines in the report |
| triage capacity | `go/internal/triagecap/floors.go:31`, `:101` (code registered at `signals.go:14`) | a fallback that counts floors in the report prose |

*Implication:* the flow depends on the output style of one model. Another family with a different style can steer the flow by accident.

### F1.8 (G8) Parsers and collectors are Go switches on the family name

| Part | Location |
|---|---|
| pane profiles | `go/internal/bridge/panestream/panedelta.go:31` (keys `claude`, `codex`, `agy`, `ollama`) |
| liveness detector | `go/internal/bridge/panestream/liveness.go:195-196` |
| usage parsers, only `claude` and `codex` | `go/internal/bridge/clicontrol/usage.go:41` |
| token collectors | `go/internal/tokenusage/defaultresolver.go:35`, `:72`, `:94` |
| transcripts, only Claude | `go/internal/attemptpostmortem/transcript.go:28`, `:35` |
| binary and driver tables | `go/internal/llmroute/llmroute.go:21`, `:120` |
| model families, pickers, effort and listers | `go/internal/modelquery/family.go:12`, `picker.go:11-13`, `effort.go:100-101`, `exec.go:38-39` |

The manifests (`go/internal/bridge/manifests/*.json`, eight files) already hold `binary`, `default_env`, `controls`, `transient_regex`, `token_line_regex` and `model_tier_map`. They have no `env_passthrough`, `parsers`, `collectors`, `transcript` or `capabilities` field.
*Implication:* a new family needs Go changes in about eight packages. The manifest is not the single source of a family.

### F1.9 (G9) The instruction files are written for Claude Code

- `CLAUDE.md` holds policy facts that only Claude Code loads at start. `CLAUDE.md:44` says that `evolve guard phase` denies `Agent`/`Task`. The Go guard does not check `Task` (F1.6).
- `GEMINI.md:13-20` said that the runtime "delegates to `claude`" and named `adapters/gemini.sh`, which does not exist. This PR corrects it.
- `skills/loop/SKILL.md:88`, `:280` and `:296` said that the phase guard is "a wired no-op" or "wired under `Bash` only", "pending ADR-0074". The hook is wired under `Agent|Task`, there are four hooks and not three, and ADR-0074 is about typed signal contracts. ADR-0075 did the rewire. This PR corrects these lines. `SKILL.md:17` and `:70` still name the removed shell hooks.
- `.claude/agents/` has four reviewers: `architecture-reviewer`, `code-reviewer`, `code-simplifier` and `go-reviewer`. No other family can run them.
- `AGENTS.md` (the "Per-CLI runtime details" section) says that the codex and agy drivers have a HYBRID mode that "delegates to `claude`". This needs a check at X10.

*Implication:* a non-Claude agent gets less policy and wrong facts.

### F1.10 (G10) The console runs builds and reviews as in-process Claude Code subagents

- `skills/commit/SKILL.md:17` says "invoke via the **Agent tool**" for the review chain.
- `skills/diff-review/SKILL.md:104` dispatches `Agent(general-purpose or <lang>-reviewer, ...)`.
- `docs/research/self-recovering-agent-loops-2026.md:148` says "the reviewer fleet runs as interactive subagents ... console only".
- The console build brief is outside the repo (`dev/console-notes/console-build-brief.md`). It tells each build agent to run reviews "via the Agent tool".

*Implication:* the console seat is a Claude Code session that directs other Claude agents. That path is outside the bridge, and only Claude can hold it.

### F1.11 (G11) No test proves that another family can fill a seat

- `TestE2ECycleCLIMatrix` skips (`go/cmd/evolve/e2e_cycle_cli_matrix_test.go:25-27`): "legacy ship-script hatch removed (Stage 5.1); native CLI-matrix e2e pending port". Its list is `claude-p`, `codex`, `agy` (`:22`).
- The conformance options pin `"cli":"claude"` (`go/internal/subagent/conformance_test.go:27`).
- The sandbox contract (`go/internal/bridge/sandbox_contract_integration_test.go:53-126`) has 22 rows. They cover tmux sockets, signals, writes and ttys. No row covers exec, network or a read of another cycle.
- `go/internal/fakeclitest` exists and can serve stub CLIs.

*Implication:* nothing measures the rule. A regression goes unseen.

### F1.12 The Signal Center pattern from ADR-0153

The quality panel uses module `quality` (`go/internal/signalcenter/event.go:72`) and codes from `BreachCode(id)` (`go/internal/qualitypanel/codes.go:11-13`), with fixed warning codes at `:6-8`. A WARN signal without a registered code gets `SIGNALCENTER_UNREGISTERED_CODE`.
*Implication:* the isolation guards can reuse the shape: one module, one violation kind and one code for each invariant.

### 1.13 Citations that moved or were wrong on re-verification

The audit cited these on 2026-10-25. The second pass found:

| Audit citation | Current | Status |
|---|---|---|
| `driver_common.go:23-31` | `:23-32` | moved by one line |
| `modelquery/exec.go:18` | `:19` (`defaultRunner` at `:18`) | moved |
| `recipe_adapter.go:40-61` "usage and budget probes" | the lines are recipe sessions for control and model capture | **wrong label**; the usage path is `controllerfactory.go:23` |
| "no test pins only the bridge starts an LLM" | `sysexec/command_guard_test.go:19` exists but checks a different rule | holds, with a note |
| `sandbox.go:164-176` | `:164-182` | range grew |
| `handoff_reviewer.go:59-74` | `:55-71` | moved |
| `model_failover.go:59-77` | `:59-78` | moved by one line |
| `driver.go:49-52` | `:48-53` (`gemini` at `:50`) | moved |
| `phase_judge.go:89` | `:84` (span), `:88` (unmarshal) | moved |
| `advisor/parse.go:33,59` | functions at `:33`, `:59`; span calls at `:34`, `:60` | same |
| `tokenusage/defaultresolver.go:34` | `:35` | moved by one line |
| `liveness.go:196` | `:195-196` | same |
| `subagent/conformance_test.go:24` | `:27` | moved |
| `sandbox_contract_integration_test.go:53-117` | `:53-126` | range grew |
| `skills/loop/SKILL.md:88,296` | also `:280`; and `:17`, `:70` name removed shell hooks | wider than cited |
| `CLAUDE.md` guard text | `CLAUDE.md:44` claims a `Task` deny that the Go guard lacks | new finding |

Every other citation was the same.

## 2. Current practice

### P2.1 Hermetic process environments

- Bazel turned the strict action environment on by default. An action gets a static `PATH`, and the host environment does not leak in. The ways back are `--action_env=NAME` or the opposite flag. [Bazel commit defd737](https://github.com/bazelbuild/bazel/commit/defd737761be2b154908646121de47c30434ed51), [Bazel commit 3a4be3c](https://bazel.googlesource.com/bazel/+/3a4be3c93813987a27a97dade3f9ebbc5770e349). **P** (commits; no current manual page surfaced).
- A leaked host `PATH` breaks cache keys between machines. [F. Zakaria, 2024-10-23](https://fzakaria.com/2024/10/23/bazel-knowledge-mind-your-path), [Tweag, 2022-09-15](https://tweag.io/blog/2022-09-15-hermetic-bazel/). **S**.
- `nix-shell --pure` clears almost the whole environment, keeps `HOME`, `USER` and `DISPLAY`, and `--keep NAME` allows more. A shell start file still runs. [nix-shell manual](https://nix.dev/manual/nix/2.35/command-ref/nix-shell). **P**.
- `bwrap --clearenv` unsets each variable except `PWD`, and `--setenv` adds back named ones. [bwrap(1)](https://man.archlinux.org/man/bwrap.1). **P**.

*Synthesis:* the common shape is "clear by default, then allow named variables". That is an allowlist, not a deny list like `ipcenv.Scrub`.

### P2.2 Least privilege and sandboxes for coding agents

- Claude Code sandboxes the file system and the network for the agent and its child processes: Seatbelt on macOS, bubblewrap on Linux. The docs say that both kinds are necessary. `sandbox.failIfUnavailable` turns a missing sandbox into an error. [Claude Code sandboxing](https://code.claude.com/docs/en/sandboxing). **P**.
- Codex sandboxes with Seatbelt on macOS and with seccomp and Landlock on Linux. In `workspace-write` the network is off by default. [Codex security](https://developers.openai.com/codex/security), [Codex sandboxing](https://developers.openai.com/codex/concepts/sandboxing). **P**.
- Gemini CLI picks its sandbox from the `-s` flag, `GEMINI_SANDBOX` or settings. The default Seatbelt profile allows the network ([Gemini CLI sandbox](https://geminicli.com/docs/cli/sandbox/), **P**). A third party says that the sandbox is off by default. [InfoWorld](https://www.infoworld.com/article/4025916/ai-coding-at-the-command-line-with-gemini-cli.html). **S**.
- OWASP LLM06:2025 "Excessive Agency" gives three root causes: too much function, permission and autonomy. It says to enforce authorization in the systems downstream, not in the LLM. [OWASP LLM06](https://genai.owasp.org/llmrisk/llm062025-excessive-agency/). **P**.
- The OWASP prompt injection cheat sheet says that no single defence is complete. It advises layers: untrusted retrieved content, least privilege for tools, output validation and a human for actions with high risk. [OWASP cheat sheet](https://cheatsheetseries.owasp.org/cheatsheets/LLM_Prompt_Injection_Prevention_Cheat_Sheet.html). **P**.

*Synthesis:* each vendor sandbox is different (network on or off, default on or off), and ollama has none. A neutral guarantee must come from the launcher.

### P2.3 Orchestrator-mediated communication

- Anthropic separates workflows, which follow code paths that are set in advance, from agents, which direct themselves. It advises the simplest shape that works. [Building effective agents](https://www.anthropic.com/engineering/building-effective-agents). **P**.
- In the Anthropic research system, a lead agent spawns subagents, each with its own context. It scored 90.2% above one agent and used about 15 times the tokens. It fits coupled coding work less well. [Multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system). **P**.
- A blackboard lets agents share one store and be chosen by its content ([arXiv 2507.01701](https://arxiv.org/abs/2507.01701), **P**, preprint). Another paper says that a master controller must know the skills of each sub-agent ([arXiv 2510.01285](https://arxiv.org/abs/2510.01285), **P**, preprint).
- Erlang processes share no data and talk only through messages to a queue for each process. [Erlang concurrent programming](https://www.erlang.org/doc/system/conc_prog.html). **P**. For Akka, only course slides and blogs surfaced: isolation holds only by convention on the JVM. [RIT slides](https://www.se.rit.edu/~swen-342/slides/Actors%20Overview.pdf). **S**.
- A2A: only vendor explainers surfaced, with no primary specification. [Tyk explainer](https://tyk.io/learning-center/a2a-protocol-architecture-and-technical-specification/). **S**.

*Synthesis:* our pipeline is a workflow in the Anthropic sense. The deterministic Go kernel is the orchestrator, so the "lead agent" role belongs to code, not to an LLM. The shared-nothing actor model fits the cell: an agent gets a copy of its inputs, and it writes only its own outputs.

### P2.4 Exec and read rules in OS sandboxes

- `sandbox-exec` is deprecated but still works on macOS 14 and 15. A deny-default profile needs allows from the logs. [jmmv, 2019-11](https://jmmv.dev/2019/11/macos-sandbox-exec.html), [Gastown note](https://gastown.dev/docs/research/macos-sandbox-exec). **S**. No Apple primary document surfaced.
- `process-exec` allow and deny rules work in SBPL, and an exec allow also needs reads of the dynamic libraries. [syumai/sbx](https://github.com/syumai/sbx). **S**.
- Landlock has `LANDLOCK_ACCESS_FS_EXECUTE` in its base ABI. A right that a ruleset handles but no rule grants stays denied. [Landlock kernel docs](https://docs.kernel.org/6.6/userspace-api/landlock.html). **P**.
- seccomp filters system calls and is "not a sandbox" alone. [Seccomp BPF](https://docs.kernel.org/4.20/userspace-api/seccomp_filter.html). **P**.

*Synthesis:* the exec deny for LLM binaries and the read deny for other cycles fit SBPL and `bwrap` rules. The SBPL syntax has only secondary sources, so each rule needs a contract test on the host.

Apple announced the Containerization framework and the `container` tool at WWDC 2025. They run each Linux container in its own lightweight VM, on macOS 26 with Apple Silicon. Sources: [The Register, 2025-06-10](https://www.theregister.com/2025/06/10/apple_tries_to_contain_itself/) and [InfoQ, 2025-06](https://www.infoq.com/news/2025/06/apple-container-linux), **S**. Such a cell runs Linux builds of the CLIs with no Keychain login and no host tmux pane. Approach C therefore fits the loop host poorly today.

### P2.5 Schema-checked structured outputs

| Vendor | Mechanism | Source |
|---|---|---|
| Anthropic API | `output_config.format` with `json_schema`; `strict: true` on a tool | [Structured outputs](https://platform.claude.com/docs/en/build-with-claude/structured-outputs), **P** |
| Claude Code | `claude -p --output-format json --json-schema`; the object is in `.structured_output` | [Headless](https://docs.claude.com/en/docs/claude-code/headless), **P** |
| OpenAI API | `response_format` `json_schema` with `strict: true` | [Structured Outputs, 2024-08-06](https://openai.com/index/introducing-structured-outputs-in-the-api/), **P** |
| Codex CLI | `codex exec --output-schema schema.json -o out.json` | [Codex non-interactive](https://developers.openai.com/codex/noninteractive), **P** |
| Gemini API | `responseSchema` with `responseMimeType: application/json`; fields that it does not support are ignored | [JSON mode](https://ai.google.dev/gemini-api/docs/json-mode), [Vertex docs](https://docs.cloud.google.com/gemini-enterprise-agent-platform/models/capabilities/control-generated-output), **P** |
| Ollama | a JSON schema in `format` | [Ollama structured outputs, 2024-12-06](https://registry.ollama.ai/blog/structured-outputs), **P** (vendor blog) |

A valid shape does not prove true values ([mer.vin](https://mer.vin/news/why-claude-json-schema-returns-valid-json-with-fabricated-values/), **S**). `--output-schema` did not work with `codex exec resume` (a blog report, **S**).
*Synthesis:* each vendor has its own flag, and some REPL drivers have no flag. The neutral contract is a declared file that the pipeline checks against a schema. A vendor flag can help, but the check is in Go.

### P2.6 Instruction files for each vendor

- AGENTS.md is a founding project of the Agentic AI Foundation, a Linux Foundation fund, with MCP and goose. [OpenAI, 2025-12-09](https://openai.com/index/agentic-ai-foundation/). **P**. The agents.md site did not surface, only a mirror. **S**.
- Codex reads `AGENTS.md` from the git root down to the working directory. Closer files win, up to 32 KiB. [Codex AGENTS.md guide](https://developers.openai.com/codex/guides/agents-md). **P**.
- Gemini CLI loads `GEMINI.md` files and can be set to read `AGENTS.md`. The key name differs between sources. [GEMINI.md docs](https://geminicli.com/docs/cli/gemini-md). **P** for the files, **S** for the key.
- Claude Code reads `CLAUDE.md`, not `AGENTS.md`. The advice is a `CLAUDE.md` that imports `@AGENTS.md`. [Claude Code memory](https://docs.claude.com/en/docs/claude-code/memory). **P**.

*Synthesis:* one neutral source (`AGENTS.md` and the policy docs) can render each vendor file. A fact that lives only in `CLAUDE.md` does not reach the other families.

### P2.7 Environment leaks into nested agents

A child `claude` that inherits `CLAUDECODE` refuses to start with "cannot be launched inside another Claude Code session". Inherited `CLAUDECODE` and `CLAUDE_CODE_ENTRYPOINT` made a nested `claude -p` hang, and the removal of both gave 10 of 10 good runs. [claude-code #25642](https://github.com/anthropics/claude-code/issues/25642), [#47018](https://github.com/anthropics/claude-code/issues/47018), [mirror of #26190](https://claudeissues.com/issue/26190-nested-claude-p-instances-hang-when-claudecode-env-var-is-inherited). **S** (community reports in the official tracker; no Anthropic document).

### Topics with weak sources only

- T3: Akka and A2A (no primary page).
- T4: the macOS SBPL syntax and its deprecation status (no Apple document).
- T6: the agents.md site and the Gemini key for `AGENTS.md`.
- P2.7: the environment leak (community reports only).

## 3. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| Clear the environment, then allow named variables (P2.1) | adopt | It closes F1.1 and matches the manifest `default_env`. |
| A deny list of known bad variables | reject | A new vendor variable leaks until someone adds it. `ipcenv.Scrub` shows the limit. |
| The OS sandbox for each cell, with exec and read denies (P2.2, P2.4) | adopt | It holds for each family, also for ollama. |
| Vendor-native sandboxes as the only control (P2.2) | reject | They differ for each vendor, and ollama has none. They can stay as an extra layer. |
| A lead LLM that spawns and directs the others (P2.3) | reject | It is the dependency that the operator forbids. Our kernel is a workflow. |
| Shared-nothing cells that read only indexed artifacts (P2.3) | adopt | It closes F1.4. The pipeline record is the blackboard: the pipeline writes and indexes it, and agents read the indexed parts. |
| Schema-checked output files (P2.5) | adopt | It closes F1.7 for each family, with or without a vendor flag. |
| One neutral instruction source (P2.6) | adopt | It closes F1.9. |
| A container for each cell | reject now | Apple's container runtime needs macOS 26 and runs Linux guests with no Keychain login (P2.4). |

## 4. Refinements to adopt

- **R1** Build the child environment from an allowlist and the manifest. Log each dropped name in `shadow` (F1.1, P2.1, P2.7).
- **R2** Put each LLM launch behind one bridge API, and pin it with an AST guard like the `sysexec` guard (F1.2).
- **R3** Mark each phase pane with a dispatch depth and a phase marker. Deny the exec of LLM binaries in the sandbox (F1.3, P2.4).
- **R4** Give each phase an index of the artifacts that it can read. These are the deliverables of earlier phases and the retry lessons of its item lineage. Grant reads of exactly those paths. Deny agent logs, unrelated cycles and the secret parts of `$HOME` (F1.4, P2.2, P2.3). The operator refined this rule on 2026-10-25. The lessons must be "accessible when needed (not necessary to be loaded at the beginning)".
- **R5** Move the floor and the defaults into the routing data, with the same values (F1.5).
- **R6** Hold each guarantee in the sandbox, the fence and the gates. Keep the hooks as defence in depth (F1.6, P2.2).
- **R7** Make each flow-steering output a declared file with a schema (F1.7, P2.5).
- **R8** Declare parsers, collectors and transcripts in the manifest (F1.8).
- **R9** Render the vendor instruction files from one neutral source (F1.9, P2.6).
- **R10** Run the console reviews through `evolve agent run` (F1.10).
- **R11** Prove substitution with a matrix of stub CLIs first (F1.11).
- **R12** Report each violation through the Signal Center with the ADR-0153 shape (F1.12).

## 5. Self-healing within boundaries

The operator set the north star on 2026-10-25: "build the pipeline that auto agile loop which is driven by LLM agents, the pipeline defines the clear and clean boundary to make each agent and do it works within the given scope without gaming the system. Even error or accident happened, the loop itself can detect the issue and fix the pipeline issues with orchestrator agent analysis and actions to trigger phase agents to fix."

### 5.1 Local evidence

**F3.1** `failure-decision.json` has closed enums for the level and the action. The level is `system` or `task`.

The action is `halt-and-diagnose`, `retry-with-fix` or `defer-or-quarantine` (`go/internal/policy/policy_failure.go:5-15`). Go drops a file with another action or level (`go/internal/core/failure_decision.go:26-52`). The fix type is an open string: the policy names `reintent` and `pipeline-repair`, but Go does not check it, and a test accepts `address-audit-findings` (`go/internal/core/failure_decision_test.go:27`).
*Implication:* a typed, closed action set already exists. The catalog extends it, and X14 closes the fix type.

**F3.2** The advisor proposes a phase plan, and Go clamps it. `ReplayPlanFromResponse` takes the floor and returns the clamps (`go/internal/core/phase_advisor.go:154`). `CanTransition` checks each step (`go/internal/core/statemachine.go:129`).
*Implication:* "an agent proposes, the pipeline disposes" is the existing pattern, not a new one.

**F3.3** The ADR-0064 protected surfaces are a list in `guards.IsProtectedSurface` (`go/internal/guards/integrity_surface.go:184`). The ship rejects a cycle diff that touches one (`go/internal/phases/ship/integrity.go:23`). ADR-0064 records a split-const dodge that passed every gate: the code changed a literal so that a grep gate no longer saw it.
*Implication:* the base of the constitution exists. A weaker gate that stays outside the protected list is not caught.

**F3.4** Two accidents had a global effect. A builder ran `tmux kill-server` and killed the shared run server (`docs/incidents/cycle-1853-tmux-run-server-kill.md`). A mutant of the `pgid <= 1` guard ran `kill(-1, SIGKILL)` (`docs/research/guidance-layers-2026-10.md:73`).
*Implication:* self-healing needs a cell that keeps an accident local. Part I gives it.

**F3.5** The operating policy says: "A loop cannot repair its own foundation while it stands on it" (`docs/operations/operating-policy.md:70-72`). On 2026-10-25 the inbox has 62 `pipeline-repair` items: 18 route to the console (`console` or `console-manual`), and 44 have no route. The method: parse each `.evolve/inbox/*.json` as JSON and count `kind == "pipeline-repair"` by `route`, on `origin/main` at `9fd56a0e0`.
*Implication:* the move from the console to the loop needs tiers and stages. The kernel stays with the operator.

**F3.6** The item `tdd-predicates-frozen-at-handoff` is open (`.evolve/inbox/2026-10-21T03-30-00Z-tdd-predicates-frozen-at-handoff.json`). Today a build can rewrite a predicate of its own cycle through its shell.
*Implication:* I-11 depends on this item.

### 5.2 Current practice

**P3.1 Autonomic computing and reconciliation.**
- Kephart and Chess, "The Vision of Autonomic Computing", IEEE Computer 36(1), 2003, proposed self-managing systems with autonomic managers ([bib entry](https://www.ic.unicamp.br/~andred/dir/bib2html/entry-Kephart-2003.html)). **P** (bibliographic record).
- The MAPE-K loop is monitor, analyze, plan and execute over shared knowledge ([arXiv 1505.00903](https://arxiv.org/pdf/1505.00903)). **S**.
- A Kubernetes controller is a control loop that moves the actual state toward the desired state. The docs prefer many simple controllers to one large loop ([Kubernetes controllers](https://kubernetes.io/docs/concepts/architecture/controller/), [operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)). **P**.
- A level-triggered loop fixes drift on the next pass, also after a lost event ([USENIX](https://www.usenix.org/node/299293)). **S**.

*Synthesis:* detect, diagnose, decide, repair, verify and learn is MAPE-K. Only "analyze" is an LLM task. The detector reads state, not only events.

**P3.2 Error budgets, retry budgets and circuit breakers.**
- An error budget is 1 minus the SLO. It moves effort to stability when it is spent ([SRE workbook, error budget policy](https://sre.google/workbook/error-budget-policy/)). **P**.
- A retry budget of at most 3 tries for each request and a budget for each server stop retries that multiply ([SRE book ch. 21](https://sre.google/sre-book/handling-overload/), [ch. 22](https://sre.google/sre-book/addressing-cascading-failures/)). **P**.
- A circuit breaker counts failures, opens at a threshold and fails fast. A half-open state lets one probe through ([Azure circuit breaker](https://learn.microsoft.com/azure/architecture/patterns/circuit-breaker), **P**; [Fowler, 2014](https://martinfowler.com/bliki/CircuitBreaker.html), **S**).

*Synthesis:* repair budgets for each window and breakers for each incident class follow these patterns (I-13).

**P3.3 Specification gaming and reward hacking.**
- Specification gaming satisfies the literal objective and misses the intended outcome ([DeepMind, 2020-04-21](https://deepmind.google/discover/blog/specification-gaming-the-flip-side-of-ai-ingenuity/)). **P**.
- Recent models change tests or scoring code and read the grader's answers. In one case o3 walked the call stack to return the precomputed answer ([METR, 2025-06-05](https://metr.org/blog/2025-06-05-recent-reward-hacking)). **P**.
- A monitor that reads the reasoning caught test trivialization. Optimization against the monitor taught agents to hide the intent ([Baker and others, 2025](https://arxiv.org/abs/2503.11926v1)). **P**.
- Reward hacks learned in coding environments generalized to sabotage. Prevention of the hack itself worked best ([MacDiarmid and others, 2025](https://arxiv.org/pdf/2511.18397)). **P**.
- Adversarial Goodhart is the case of an agent that games the measure ([Manheim and Garrabrant, 2018](https://ideas.repec.org/p/arx/papers/1803.04585.html)). **S** (listing only).

*Synthesis:* an agent must not be able to change its oracle (I-11), and a measure must not be its target. A monitor is a detection signal, not a grade that the agent sees.

**P3.4 Separation of duties.**
- Separation of duty limits the privileges of one user, so that no user can misuse the system alone. The two-person rule needs a second, different user ([NIST glossary](https://csrc.nist.gov/glossary/term/Separation_of_Duty), [two-person integrity](https://csrc.nist.gov/glossary/term/two_person_integrity)). **P**.
- NIST SP 800-53 AC-5 says that the people who administer access control do not also administer the audit ([mirror](https://www.stigviewer.com/controls/nist-800-53/AC-5)). **S** (mirror of a primary text).
- Maker-checker: one party starts an action, and an independent party approves it ([Wikipedia](https://en.wikipedia.org/wiki/Maker-checker)). **S**.

*Synthesis:* the orchestrator agent is the maker, and the deterministic validator is the checker. The T1 judge is from another family than the builder.

**P3.5 Incident response and human oversight.**
- NIST AI RMF MANAGE 2.4 asks for mechanisms "to supersede, disengage, or deactivate" a system that does not act as intended ([mirror](https://www.stigviewer.com/controls/ai-rmf/MANAGE%202.4)). **S** (mirror).
- NIST AI 600-1 MANAGE 4.3 asks to track, respond to and recover from incidents, with after-action reviews ([NIST AI 600-1](https://nvlpubs.nist.gov/nistpubs/ai/NIST.AI.600-1.pdf)). **P** (the exact words came from a secondary summary).
- EU AI Act Article 14 requires effective human oversight in proportion to the risk and the autonomy ([Article 14](https://ai-act-service-desk.ec.europa.eu/en/ai-act/article-14)). **P**. The "stop button" detail is from secondary summaries.
- Human oversight suffers from automation bias and lower attention ([Fink, SSRN](https://papers.ssrn.com/abstract=5147196)). **S**.
- OWASP agentic AI threats advise clear boundaries, behavior monitoring and detailed logs ([OWASP agentic threats](https://genai.owasp.org/resource/agentic-ai-threats-and-mitigations)). **P**. Practitioner guides add human approval for actions with high impact, and separate generation from execution ([Teleport](https://goteleport.com/blog/owasp-top-10-agentic-applications)). **S**.
- Blameless postmortems fix the system, not the person ([SRE book, postmortem culture](https://sre.google/sre-book/postmortem-culture/)). **P**.

*Synthesis:* escalate seldom, with a typed dossier, and keep a halt that leaves the loop in a safe state. The recovery retrospective of ADR-0151 is the postmortem.

Weak spots:

- kill-switch guidance for agents (practitioner sources only);
- Nygard's book (no primary link);
- NIST AC-5 and AI RMF (mirrors only);
- the Strathern wording of Goodhart's law (secondary only).

### 5.3 Refinements to adopt

- **R13** Keep the orchestrator agent a proposer. The validator and the dispatch are Go (F3.1, F3.2, P3.4).
- **R14** Make the kernel a constitution on the ADR-0064 base, and add a classifier for a weaker gate (F3.3, P3.3).
- **R15** Freeze checks at handoff, judge T1 with another family, and hide the indexes from the agents (F3.6, P3.3).
- **R16** Bound the repairs with budgets, breakers and converge-or-escalate (P3.2, ADR-0126).
- **R17** Move from the console to the loop by tier and by stage, on measured evidence (F3.5, P3.5).

## 6. The CLI service contract and the control flags

The operator asked two more things on 2026-10-25:

> "Make sure evo loop cli can be invoked for most of the core functions all the time. It's the interfaces for other services to use it's functions anytime."

> "this refactory is also the good timing to remove the global flag that should be removed and replaced with dynamic configuration long time ago"

The decision is [ADR-0156](../architecture/adr/0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md). The contract is [cli-contract.md](../architecture/cli-contract.md). The design is [Part III of the spec](../architecture/llm-isolation.md#26-the-cli-service-contract-x17).

Method: a read-only audit of `origin/main` at `603389ebd` on 2026-10-25 by one explorer. The docs builder re-read each cited `path:line` on the same commit. §6.3 lists each citation that was wrong or moved. `c/` is `go/cmd/evolve/` and `i/` is `go/internal/`.

### 6.1 Local evidence: the CLI surface

**F4.1** The CLI has 112 verbs in one registry (`grep -c 'Name: "' go/cmd/evolve/registry.go` = 112). No single document lists their inputs, outputs, exit codes and availability. The surface is spread over the usage text of `main.go`, the registry summaries and the usage text of each verb.
*Implication:* a service caller cannot learn the contract without reading Go code.

**F4.2** Many core verbs print human text only. Examples: `loop-stop`, `gc` and `sync-main` (`c/cmd_gc.go`, `c/cmd_syncmain.go`), `quality report` (`c/cmd_quality.go:121`), `ledger verify` (on stderr, `c/cmd_ledger.go:60`), `routing explain`, and the inbox curation verbs. No verb puts a schema version in its JSON. `loop status` encodes `dashboard.LoopStatus` directly (`c/cmd_loop_status.go:45`).
*Implication:* a caller parses prose, and a field rename breaks it with no signal.

**F4.3** There is no shared exit-code table. `exitUsage=10` and `exitIO=2` are in `c/cmd_pr.go:25-26`, and `exitRoutingRefused=2` is in `c/cli_routing_root.go:122`. A usage error exits 1 in `loop-stop` and `inbox-mover` (`c/cmd_loop_stop.go:34`, `c/cmd_inbox_mover.go:32`), and 10 in `gc`, `loop status` and `quality` (`c/cmd_gc.go:30`, `c/cmd_loop_status.go:26`, `c/cmd_quality.go:110`).
*Implication:* a caller cannot tell "you called me wrong" from "the work failed" without a table for each verb.

**F4.4** `evolve loop status foo` starts a real loop. `isLoopStatusInvocation` takes the read-only path only when `status` is alone or the next word starts with `-` (`c/cmd_loop_status.go:15`). Any other form goes to the loop parser, and `parsePositional` joins the rest into a goal (`c/cmd_loop_args.go:307-323`). The reserved-word guard refuses only a goal that is exactly `status` (`c/cmd_loop_args.go:178-181`).
*Implication:* a read verb with one wrong word can start a batch. This is the first defect to fix.

**F4.5** Five inbox verbs refuse while any run lease is live: `edit`, `withdraw`, `verify`, `route-console` and `route-lane`.
- The check is `refusedMidWaveAt` (`c/cmd_inbox_curate.go:25-35`).
- The calls are at `c/cmd_inbox_curate.go:58`, `:103`, `:124`, `c/cmd_inbox_route_lane.go:18` and `c/cmd_inbox_route_console.go:25`.
- `add` and `consume` have no such gate. `cli-routing set|unset|init|migrate` refuse under a live lease or inside a phase (`c/boundary_write.go:13-27`).
*Implication:* a service cannot curate the backlog during a wave, although the change can wait for the next boundary.

**F4.6** These refusals are correct and stay:
- `pr merge` (`c/cmd_pr.go:265-276`);
- `sync-main` (`c/cmd_syncmain.go:58-63`);
- `cycle reset` without `--force` (refusal at `c/cmd_cycle.go:131-133`, flag at `:109`);
- `cli update` without `--dry-run` (`c/cmd_cli.go:57-59`);
- the single owner of `loop` and `loop --detach` (`c/cmd_loop_detach.go:55`, `:129`).
*Implication:* "available at all times" cannot mean "every verb mutates at all times". It means each verb has a declared availability class.

**F4.7** `gc` has no top-level lease check (`c/cmd_gc.go:23-50`). Only `--only cache` says that it is safe while lanes run (`:28`).
*Implication:* a full `gc` during a wave depends on the care of each sweep, not on one lease.

**F4.8** Some verbs have no `--project-root` flag and read the root from `EVOLVE_PROJECT_ROOT` or the current directory: `inbox list|show|add|edit|withdraw|consume|rank` (`c/cmd_inbox.go:91`, `c/cmd_inbox_curate.go:18`), `inbox-mover` (`c/cmd_inbox_mover.go:18`) and `signals tail` (`c/cmd_signals.go:28`). `quota-reset` and `fanout-dispatch` join `os.Getenv("EVOLVE_PROJECT_ROOT")` with a relative path, so an unset variable gives a path relative to the current directory (`c/cmd_quota_reset.go:26`, `c/cmd_fanout_dispatch.go:44`). `cycle reset` defaults the root to `.` (`c/cmd_cycle.go:105`).
*Implication:* a service with another working directory reads or writes the wrong plane.

**F4.9** `install` asks for `y` on stdin unless `--ci` or `CI=true` is set (`c/cmd_install.go:103`, `:172-178`). `inbox add` reads the item from stdin when `--file` is absent.
*Implication:* these two verbs are interactive by default.

**F4.10** These verbs are absent:
- `inbox claim`. It exists only as `inbox-mover claim` (`c/cmd_inbox_mover.go:29`).
- A read of one dossier. `dossier` has only `verify`, `retro-mislabel`, `publish` and `sweep` (`c/cmd_dossier.go:19-32`).
- `cycle resume`. Only `loop --resume` exists.
- A wave stop. Only `loop-stop --wait` exists.
- `events`. Plan E9 is not started ([event-notification-protocol-2026-10](../plans/event-notification-protocol-2026-10.md) §16).
*Implication:* a service uses an internal verb or a flag of another verb for these functions.

**F4.11** The only server is `dashboard`: loopback HTTP, read-only, with no authentication (`i/dashboard/server.go:90-98`). There is no socket and no RPC.
*Implication:* the CLI is the service interface. A daemon is a separate decision.

### 6.2 Local evidence: the control flags

**F5.1** `control-flags.md` says that the registry has 35 rows (`docs/architecture/control-flags.md:3`). The generated table has 26 rows, which match `go/internal/flagregistry/registry_table.go`.
*Implication:* the header is wrong.

**F5.2** `EVOLVE_WORKTREE_PATH` and `EVOLVE_REFLECTION_JOURNAL` are in the registry, but no production code reads them. The unprefixed `WORKTREE_PATH` is a different, live subagent variable (`c/cmd_subagent.go:288`).
*Implication:* two registry rows are dead.

**F5.3** Many readers are not in the registry:
- the `ipcenv` keys (`i/ipcenv/ipcenv.go:5-18`), `DISPATCH_DEPTH` and the `FANOUT_*` names;
- the bridge launch names (`i/bridge/launch.go:275-297`);
- `PHASE_RECOVERY_STAGE`, `SHIP_*`, `CLI_MAX_CONCURRENT_<FAM>` and `PHASE_<X>_BIN`;
- the per-agent `EVOLVE_<AGENT>_*` names and `RESUME*`.

Many names are built as `"EVOLVE_"+"X"` (`c/cmd_loop_args.go:357`), so a literal grep misses them.
*Implication:* the registry is not the full inventory, and a grep guard alone cannot find each reader.

**F5.4** `policy.Parse` decodes with plain `json.Unmarshal` (`i/policy/patch.go:9-21`). Only the `cli_routing` and `quality` blocks use `decodeStrict` with `DisallowUnknownFields` (`i/policy/cli_routing.go:124-126`, `i/policy/quality.go:23`).
*Implication:* a typo in a policy key is silently ignored. A knob that moves into policy needs a strict decode first.

**F5.5** `buildCycleEnv` copies each `EVOLVE_*` variable of the operator shell into the cycle environment (`c/cmd_loop_args.go:341-352`). `driverEnv` gives each LLM child the whole `os.Environ()` and strips only `EVOLVE_DISPATCH_ID` (research F1.1). The X2 review found the same hole at `enforce`: the hermetic allowlist still passes each `EVOLVE_*` and `BRIDGE_*` name (review of #1122, finding H2).
*Implication:* each operator knob and each bypass reaches the LLM children today.

**F5.6** The audit put each reader in one of five classes. After re-verification (§6.3), the counts are 32 pipeline markers (23 `EVOLVE_*`, 9 `BRIDGE_*`), 18 operator knobs, 10 bypasses, 6 dead names and 8 test seams. [control-flags.md](../architecture/control-flags.md#classes-of-the-readers-outside-the-registry) has the list.
*Implication:* each class has a different fix. One rule for all the names fails.

### 6.3 Citations that moved or were wrong on re-verification

| Audit claim | On `603389ebd` | Status |
|---|---|---|
| "`loop --project-root P status` launches a loop" | The goal is exactly `status`, so the reserved-word guard refuses it with exit 10 (`c/cmd_loop_args.go:178-181`). It does not reach the status reader either. | **wrong**; the real launch case is `loop status <word>` |
| "the packages under `i/events/{filter,channel,wake,reader,publisher}` exist but nothing calls them" | `channel` is called by `signals tail --channel` (`c/cmd_signals_tail_channel.go:17`) and `agentmsg/channelstore`; `filter` and `reader` by `orchestrator/oversight.go`; `wake` by `agentmsg/outbox`; `publisher` by `events/rootwire` (E10, merged in #967). | **wrong**; the CLI verbs of E9 are still absent |
| `c/cmd_loop_args.go:343-352` (`buildCycleEnv`) | `:341-352` | moved |
| `c/cmd_inbox_curate.go:22-33` | `:22-35` | range grew |
| `i/dashboard/server.go:92-97` | `:90-98` | range grew |
| `i/policy/patch.go:14` | `:14` in `Parse` at `:9` | same |
| the worker variables of `dispatchparallel.go:176-178` | the shell line also exports `EVOLVE_FANOUT_WORKER_NAME` | one name missing: 23 `EVOLVE_*` markers, not 22 |
| "17 operator knobs" | the audit's own list has 18 names: the per-agent family has four (`_CLI`, `_MODEL`, `_SYSTEM_PROMPT`, `_PERMISSION_MODE`) | count wrong by one |
| "7 test seams" | the audit's own list has 8 names (`BRIDGE_<X>_BINARY` is one of them) | count wrong by one |

Every other citation was the same.

### 6.4 Local evidence: CLI specifics above the driver layer

The operator added on 2026-10-25: "for different LLM clis, when assigned the tasks through agent bridge, the task assignment should only contains the general info without any LLM cli specification and settings... LLM specification can only be appended under the driver layer." The docs builder read `origin/main` at `72561f774` for these facts.

**F6.1** `core.PhaseRequest` carries `ModelRoutingCLI`, `ModelRoutingTier` and `ModelRoutingEffort` (`go/internal/core/phase.go:101-105`). The phase runner reads `ModelRoutingCLI` as a routing overlay (`go/internal/phases/runner/routing.go:94`, `:113`).
*Implication:* a named CLI travels in the assignment, above the bridge.

**F6.2** The advisor prompt asks the LLM to name a `cli` for each phase and each minted phase (`go/internal/core/advisor/prompt.go:90`).
*Implication:* a flow seat picks a CLI, not a capability.

**F6.3** All 93 tracked agent profiles have a `cli` and a `cli_fallback` key (the 94th file, `tool-policy.json`, is not an agent profile). All 93 hold `Bash(evolve phase verify:*)` in their tool lists. `allowed_tools` and `disallowed_tools` use the Claude permission syntax, for example `Edit(.evolve/state.json)` and `Bash(git push:*)` (`.evolve/profiles/builder.json`). 27 profiles have `extra_flags_by_cli` with Claude Code flags such as `--strict-mcp-config` and `--plugin-dir`.
*Implication:* the profile, which the pipeline reads, holds CLI flags and vendor tool syntax.

**F6.4** Four flow seats name a family and a model in Go: `phase_advisor.go:73`, `failure_advisor.go:54`, `phase_judge.go:28` and `contract_escalation.go:22` (all `go/internal/core/`). Four phases have the default model `opus` (`audit.go:69`, `debugger.go:107`, `buildplanner.go:57`, `swarmplan.go:44` under `go/internal/phases/`).
*Implication:* X6 already owns these; X19 must not build them again.

**F6.5** 14 of 106 persona files under `agents/` name a Claude Code tool or feature (`grep -l 'Agent tool\|TodoWrite\|WebFetch\|Claude Code\|Task tool' agents/*.md`).
*Implication:* the persona text is not CLI-neutral.

**F6.6** The per-agent `EVOLVE_<AGENT>_CLI` and `_MODEL` names and the `.claude/settings.json` hooks are CLI specifics too. X18c and X7 own them.
*Implication:* X19 lists them and leaves them to their owners.

### 6.5 Current practice

**P4.1 A CLI for programs.** The Command Line Interface Guidelines put the primary and machine-readable output on stdout and the messages on stderr. They add `--json` for machine output, document the exit codes, and prompt only when stdin is a terminal. The search gave only secondary summaries of clig.dev ([create-cli skill](https://tessl.io/registry/skills/github/marchatton/agent-skills/create-cli), [cli-output-conventions](https://tessl.io/registry/testland/cli-output-conventions)). **S**.

**P4.2 A versioned output.** kubectl advises a script to name the API group and version of the output. The structure can change between versions ([kubectl field printing](https://kubectl.docs.k8s.io/guides/resource_printing/fields/), **P**; [kubectl conventions mirror](https://norazhao0728githubio.readthedocs.io/en/latest/user-guide/kubectl-conventions/), **S**). No primary source gives a stability promise for `-o json`.

**P4.3 The environment leaks down the process tree.** A child process gets the environment of its parent. The systemd documentation warns that variables cross the setuid boundary ([HN quote of the systemd docs](https://hn.svelte.dev/item/31225921), **S**; [jolicode](https://jolicode.com/blog/what-you-need-to-know-about-environment-variables-with-php), **S**). Any user with privileges reads it through `/proc/<pid>/environ` ([GitGuardian](https://blog.gitguardian.com/secure-your-secrets-with-env)). **S**.

**P4.4 Configuration in the environment.** The twelve-factor app keeps the deploy configuration in environment variables ([12factor.net/config](https://12factor.net/config)). **P**. It excludes internal settings that do not change between deploys. Critics say that the environment holds only strings and has no structure ([roots.io](https://roots.io/twelve-factor-03-config.md)). **S**.

**P4.5 A strict decode.** Go's `encoding/json` skips an unknown key by default. `Decoder.DisallowUnknownFields` turns a misspelled key into an error ([oneuptime, 2026](https://oneuptime.com/blog/post/2026-01-23-go-json-unknown-fields/view), **S**; [sigs.k8s.io/json](https://pkg.go.dev/sigs.k8s.io/json), **P**).

**P4.6 Flag debt.** A flag that nobody removes is technical debt. The advice is an owner and an expiry for each flag, and a check that finds flags that no code reads ([LaunchDarkly](https://docs.launchdarkly.com/guides/flags/technical-debt), **P**; [Statsig](https://statsig.com/perspectives/feature-flag-retirement), **S**).

**P4.7 Provider specifics behind one interface.** LiteLLM forwards each non-OpenAI parameter as provider-specific, and by default it raises an error on a parameter that a model does not support. `drop_params` turns the error into a silent drop ([LiteLLM drop params](https://docs.litellm.ai/docs/completion/drop_params), [input params](https://docs.litellm.ai/docs/completion)). **P**. One search on 2026-10-25.

*Synthesis:* a typed, versioned stdout and one exit-code table make the CLI a service contract. The environment is a channel that each child inherits, so a control value in it reaches each LLM. Twelve-factor puts the deploy values in the environment. Our control knobs change the flow, so they are not deploy values. They belong in the strict policy file, which the pipeline reads at a boundary.

### 6.6 Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| `--json` on stdout, messages on stderr (P4.1) | adopt | It closes F4.2. |
| A schema version in each JSON envelope (P4.2) | adopt | A caller can check the version before it reads a field. |
| One exit-code table (P4.1) | adopt | It closes F4.3. |
| A daemon or an RPC server for services | reject now | The CLI already reaches every function (F4.11). A server adds authentication and a lifecycle. |
| Control knobs in the environment (P4.4) | reject | They reach each LLM child (F5.5, P4.3) and leave no record. |
| Strict decode of the policy file (P4.5) | adopt | It closes F5.4 before the knobs move in. |
| An owner and a removal for each dead flag (P4.6) | adopt | It closes F5.2. |

### 6.7 Refinements to adopt

- **R18** Write one CLI contract with a versioned envelope, one exit-code table and an availability class for each core verb. Fix `loop status <word>` first. Pin the contract against the registry with a guard test (F4.1 to F4.11, P4.1, P4.2).
- **R19** Read no control flag from the environment. Keep one named list of pipeline markers. Move the knobs into the strict policy file, and turn the bypasses into audited parameters. Delete the dead names, and inject the test seams (F5.1 to F5.6, P4.3 to P4.6).
- **R20** Send a CLI-neutral task assignment. Keep each model name, CLI flag, tool syntax and parser in the driver and manifest layer. Ask for capabilities, never for a named CLI, and refuse a missing capability instead of a silent drop (F6.1 to F6.6, P4.7).
