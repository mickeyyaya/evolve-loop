# Agent run: one phase agent from the command line, with no cycle (research, 2026-10)

> The plan that uses this research: [agent-run-standalone-2026-10](../plans/agent-run-standalone-2026-10.md). The design: [agent-run](../architecture/agent-run.md). The decision record: [ADR-0152](../architecture/adr/0152-one-phase-agent-runs-from-the-command-line-with-no-cycle-through-the-phase-runner.md).

Date: 2026-10-21. Method: a read of the phase command, the spec runner, the bridge adapter and the ledger, and a live web search on 2026-10-21. Each local claim names its file. "Synthesis" marks our own inference.

## Table of contents

1. [Local evidence](#1-local-evidence)
2. [Current practice](#2-current-practice)
3. [Adopt or reject](#3-adopt-or-reject)
4. [Refinements to adopt](#4-refinements-to-adopt)

## 1. Local evidence

**F1.1** `evolve phase <name>` reads a `core.PhaseRequest` JSON on stdin, or derives it with `--cycle N` (`go/internal/cli/phasecmd/phase.go`, `phaseRequest`). It prints only the `core.PhaseResponse` envelope on stdout (`phase.go:77`).
*Implication:* a caller gets the verdict and the artifacts directory, but not the report text. A program must read a file to get the answer.

**F1.2** `RequestForCycle` refuses a cycle that a live run holds (`go/internal/cli/phasecmd/phase_cycle.go:47`). It reads the cycle state and requires the worktree of that cycle (`phase_cycle.go:30`).
*Implication:* the `--cycle` path needs a real, finished cycle. A caller with only a diff has no cycle to name, and an invented cycle number is forbidden (CLAUDE.md, "Never fabricate cycle numbers").

**F1.3** `ResolveRunner` builds a spec phase as `specrunner.New` with `bridge.NewDefault(root, nil)` (`go/internal/cli/phasecmd/spec_phase.go:17`, `:28`). The spec runner is the shared `runner.BaseRunner`: it resolves the CLI chain through the routing table, walks the fallbacks and judges the artifact (`go/internal/phases/runner/runner.go`, `Run`).
*Implication:* one call already gives the persona, the `cli_routing` chain of the profile, the bridge and the verdict. A new command can reuse it. The bridge is fixed inside the call, so a test cannot inject a fake bridge today.

**F1.4** The spec runner puts into the prompt only the context keys that the spec lists in `prompt_context` (`go/internal/phases/specrunner/specrunner.go:35`). An evaluate phase gets `goal` by default (`go/internal/phasespec/phasespec.go:145`, `ApplyArchetypeDefaults`). Plan phases get no default key.
*Implication:* no key carries free input to every spec phase. A standalone input needs its own key that the spec runner always renders.

**F1.5** Each spec phase names its inputs as cycle paths, for example `adversarial-review` reads `.evolve/runs/cycle-{cycle}/build-report.md` (`.evolve/phases/adversarial-review/phase.json`).
*Implication:* the persona expects cycle artifacts. The standalone prompt must tell the agent that the given input replaces them.

**F1.6** The bridge maps the profile write grant `.evolve/runs/cycle-*` to the workspace of the request, not to the project run directories (`go/internal/bridge/sandbox_paths.go:165`).
*Implication:* a workspace outside `.evolve/runs` still gets the write grant of the profile. The sandboxed agent can write its report there.

**F1.7** The routing request sets `NeedsTools` only when the request has a worktree (`go/internal/phases/runner/routing.go:95`). The sandbox grants Git metadata writes to a linked worktree (`go/internal/bridge/sandbox_paths.go:58`).
*Implication:* a read-only run needs no worktree. A worktree on the plane tree gives the agent Git writes there, so a read-only run must not set one.

**F1.8** The ledger has a manual-entry form: `"cycle": 0` plus a `cycle_label` (`docs/operations/runtime-reference.md:195`). `ledger.New(evolveDir).Append` hash-chains each line under a file lock (`go/internal/adapters/ledger/ledger.go:77`). The cycle path uses kind `agent_subprocess` (`go/internal/subagent/subagent.go:198`).
*Implication:* a standalone run can record its dispatch with no cycle number. A separate kind keeps it out of the cycle dossier reader, which keys on `agent_subprocess`.

**F1.9** `subagent.ReadDispatchDepth` reads the dispatch depth of the process. The boundary writers refuse a depth above 0 as "inside a phase" (`go/cmd/evolve/boundary_write.go:14`).
*Implication:* the command can detect a call from a phase agent and refuse a nested dispatch.

**F1.10** Two spec phases write source: `bug-reproduction` and `test-amplification` (`writes_source: true` in `.evolve/phases/*/phase.json`). The built-in phases build, tdd and ship write source through the worktree rule (`go/internal/core/worktree_fence.go:28`).
*Implication:* a read-only default can exclude them with the existing `core.PhaseWritesSource` predicate.

## 2. Current practice

**P2.1** Claude Code headless mode runs one prompt with `-p` and prints the final result. `--output-format json` gives one object with the result text, the duration, the turns, the session id and `total_cost_usd` ([Claude Code headless docs](https://code.claude.com/docs/en/headless.md), [docs.claude.com](https://docs.claude.com/en/docs/claude-code/headless)).

**P2.2** `codex exec` reads the prompt from stdin when the prompt argument is absent or `-`. It streams progress to stderr and prints only the final message to stdout. `--json` switches stdout to JSON lines. The default sandbox is read-only, and edits need `--sandbox workspace-write` ([Codex non-interactive mode](https://developers.openai.com/codex/noninteractive)).

**P2.3** Gemini CLI headless mode prints one JSON object. It has a `response` string, a `stats` object for tokens and an optional `error` object ([Gemini CLI headless reference](https://geminicli.com/docs/cli/headless/)). A third-party mirror lists exit 0 for success, 1 for an error and 42 for an input error ([mintlify mirror](https://www.mintlify.com/google-gemini/gemini-cli/advanced/headless)). The official page does not show the table.

**P2.4** The OpenAI Agents SDK runs one agent with `Runner.run_sync(agent, input)` and returns `final_output`, plus a `max_turns` cap ([openai-agents on PyPI](https://pypi.org/project/openai-agents/), [running agents](https://openai.github.io/openai-agents-python/ja/running_agents/)).

**P2.5** LangGraph exposes each node of a compiled graph through `graph.nodes`, and `.invoke()` on one node runs it without the whole graph. The docs note that this path skips the checkpointer ([LangGraph testing](https://docs.langchain.com/oss/python/langgraph/test.md), [LangChain forum](https://forum.langchain.com/t/best-practices-for-testing-langgraph-nodes-separately/1396)).

**P2.6** The command-line guidelines put the primary output and any machine-readable output on stdout, and diagnostics on stderr. Exit 0 means success, and distinct non-zero codes let a script tell failure modes apart. A `--json` flag selects machine output ([cli-output-conventions](https://tessl.io/registry/testland/cli-output-conventions), [unix-cli skill](https://playbooks.com/skills/pproenca/dot-skills/unix-cli)).

## 3. Adopt or reject

| Practice | Decision | Reason |
|---|---|---|
| Final answer only on stdout, progress on stderr (P2.1, P2.2, P2.6) | Adopt | A pipe gets the report and nothing else. The runner already logs on stderr. |
| `--json` gives one envelope with the text, the cost and the tokens (P2.1, P2.3) | Adopt | A program parses one object. One object, not JSON lines: the runner has no event stream to expose. |
| Prompt or input from stdin, `-` for stdin (P2.2) | Adopt | `git diff \| evolve agent run adversarial-review` is the main use. |
| Read-only sandbox by default, writes need an explicit opt-in (P2.2) | Adopt, and go further in v1 | A source-write profile is refused in v1, because no worktree path is safe from a live loop yet (F1.7, F1.10). |
| Distinct exit code for an input error (P2.3) | Adopt | Our codes follow `evolve phase`: 10 usage, 11 input. |
| Invoke one node and skip the checkpointer (P2.5) | Adopt | The run skips cycle state, the lease and the run directory, as the LangGraph node skips the checkpointer. |
| JSON lines event stream (P2.2) | Reject for v1 | The phase runner returns one response. A stream needs a new port. |

## 4. Refinements to adopt

- **R1** Reuse the spec-runner constructor of `ResolveRunner` with an injectable bridge, so tests run a fake bridge and production keeps `bridge.NewDefault` (F1.3).
- **R2** Add one context key for the standalone input. Render it in every spec prompt, with a note that it replaces the cycle artifacts (F1.4, F1.5).
- **R3** Put the workspace under `.evolve/ephemeral/agent-runs/<label>`, outside `.evolve/runs`, and set no worktree (F1.6, F1.7).
- **R4** Record one ledger entry with cycle 0, a `cycle_label` and kind `agent_run` (F1.8).
- **R5** Refuse a call from inside a phase, a built-in phase and a source-write profile (F1.9, F1.10).
