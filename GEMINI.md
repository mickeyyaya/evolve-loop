# Project Instructions (Gemini CLI)

> This file is rendered. The part between the GENERATED markers is a copy of the digest of [AGENTS.md](AGENTS.md), the one source of the instructions for each CLI. Read AGENTS.md for the full rules and the runtime facts. Change AGENTS.md, then run `evolve skills generate`. `evolve skills check` fails when this copy differs from AGENTS.md.
>
> **Operating policy (canonical, environment-independent): [docs/operations/operating-policy.md](docs/operations/operating-policy.md).** Release notes: [CHANGELOG.md](CHANGELOG.md).

## Gemini CLI overlay

- **Skill discovery.** Gemini CLI finds the skills at `.agents/skills/<name>/SKILL.md`, the cross-CLI open standard. Each entry is a symlink to the canonical `skills/<name>/` directory. Both paths give the same SKILL.md. The primary skill is `/evo:loop`.
- **Runtime.** The `evolve` binary is the only controller of a cycle. It starts each LLM CLI as an isolated worker, and no CLI starts or directs another CLI ([ADR-0154](docs/architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md)). The Gemini models run through the native Antigravity (agy) bridge drivers (`go/internal/bridge/driver_agy*.go`). The `cli_routing` table in `.evolve/policy.json` decides which phase runs on agy. `evolve cli-routing explain <agent>` prints the CLI chain of each phase.
- **The `gemini` binary** has no bridge driver. The bridge refuses the bare driver name `gemini` and names the agy drivers (`go/internal/bridge/driver.go`). The plugin manifest declares no Gemini tier.
- **Tool names.** The repository documents use the Claude Code tool names. The Gemini CLI names: `Read` is `ReadFile`, `Bash` is `RunShell`, `Edit` is `replace` and `Write` is `write_file`. `Grep` is `SearchCode`, `Glob` is `SearchFiles` and `Skill` is `activate_skill`. The full table is [skills/loop/reference/gemini-tools.md](skills/loop/reference/gemini-tools.md).
- **Subagents.** A phase agent starts through the native bridge (`evolve subagent run <agent> <cycle> <workspace>`), never through `activate_skill` as a subagent. A reviewer starts through `evolve agent run <name>`.

<!-- GENERATED:instructions BEGIN from AGENTS.md by `evolve skills generate`; edit the source, not this region -->
## Digest (each CLI loads this part)

This digest has the facts that each agent must have when it starts. The full rules, the reasons and the evidence are in [AGENTS.md](AGENTS.md) and in the linked docs. Read them when your task touches their subject.

### Hard rules

- **The pipeline is the only controller** (ADR-0154, proposed; X1 to X8 are on `main`). No LLM starts or directs another LLM. A phase agent starts through the native bridge (`evolve subagent run`, `evolve loop`). Inside its cell, a phase agent can start subagents of its own harness. Outside a cell during a cycle, `evolve guard phase` denies the start of a phase persona (`evolve-*`).
- **Phase order.** Scout → Build → Audit → Ship, in that order. The Go state machine (`go/internal/core`) enforces it. Never skip a phase, and never ignore a kernel-gate failure.
- **Commits go through `evolve ship`.** The ship gate denies a bare `git commit` or `git push`. An interactive commit runs `/commit`, then the attestation, then `evolve ship --class manual`. A routine `--bypass-commit-gate` is a violation. A release is `evolve release X.Y.Z`; "publish" is not "push".
- **Builder writes only inside its worktree.** The runtime provisions worktrees. An agent never runs `git worktree`.
- **Ship needs the EGPS verdict.** `acs-verdict.json:red_count == 0`. The ledger is hash-chained; `evolve ledger verify` checks it.
- **Never fabricate cycle numbers.** This is a CRITICAL violation.
- **Code has no comments.** Names, types and tests say what the code does. A design reason goes to `docs/architecture/packages/` ([code-comments](docs/conventions/code-comments.md)).
- **Documents go through `docs-write`.** Load `skills/docs-write/SKILL.md` before you write a document. The new text passes `evolve docs ste-lint --strict`.
- **No MCP.** Reach each outside service through a CLI ([cli-only-no-mcp](docs/conventions/cli-only-no-mcp.md)).
- **Document what you find.** Research, lessons and failed approaches go to `docs/`. Never delete a doc; archive it under `docs/private/research/archived-YYYY-MM-DD/`.

### Operating rules

- **Zero-ship halt.** Evaluate the output after every cycle. If 2 consecutive cycles ship nothing, STOP and find the root cause in the pipeline ([operating-policy §4.1](docs/operations/operating-policy.md)).
- **Status reports** give the cycles that ran, the ships (for example 2/2) and the open PR numbers. They also give the failed CI jobs with their class, and the next action.
- **Bypass mode** means "do not ask the user". It does not mean "skip integrity checks". Run the full pipeline every cycle, and follow the failure-adapter verdicts verbatim.
- **Pre-commit review fleet:** code-simplifier, then architecture-reviewer and code-reviewer in parallel, then fix the findings, then commit. `evolve agent run <name>` runs each reviewer on any family. Fix each CRITICAL finding of architecture-reviewer before the commit. A diff of only `.md` files gets the `docs-review` skill and `--reviewers docs-review`; the code reviewers do not satisfy the gate. Only a proven comment removal needs no reviewer.
- **Verify before you claim done.** Probe a CLI with `evolve doctor probe <tool>` before you call it unavailable. Read the real exports of a module before you call it. Report test counts: `N/N PASS, no regression`.
- **CI failures:** classify each red with `evolve ci classify` before a retry. When a gate reports RED, verify the logic of the gate before you assume that the code is broken.
- **Go:** run `gofmt -l .`, `go vet` and `go test -count=1` from `go/`. `-count=1` is required, because the test cache does not see the files that tests read through `..`.
- **Shell:** the target is bash 3.2. Use `set -uo pipefail` (not `set -e`) and atomic writes through `mv`. The banned constructs are in [runtime-reference.md](docs/operations/runtime-reference.md).
- **Task priority** is the computed inbox rank (`evolve inbox rank --explain`).
- **Confirm direction first:** multi-step work needs a 3-bullet plan and an approval. A single-cycle bug fix, a task that names a file path and a task from an approved plan are exempt.
- **Waves:** the console starts each wave with `evolve wave next` and follows it with `evolve wave watch`. Do not write goal scripts. Queue facts with `evolve wave note add` ([wave-operations.md](docs/operations/wave-operations.md)).

### Where the rest is

- The 12 core agent rules and the cross-CLI invariants: [AGENTS.md](AGENTS.md).
- The runtime facts (gates, routing, fleet, observer, operator verbs): [the AGENTS.md section](AGENTS.md#operating-rules-and-runtime-facts) and [runtime-reference.md](docs/operations/runtime-reference.md).
- The operating policy: [operating-policy.md](docs/operations/operating-policy.md). The isolation design: [llm-isolation.md](docs/architecture/llm-isolation.md).
- The routing of each phase: `evolve cli-routing explain <agent>`.
<!-- GENERATED:instructions END -->
