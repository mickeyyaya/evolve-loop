# Project Instructions (Claude Code)

> This file is rendered. The part between the GENERATED markers is a copy of the digest of [AGENTS.md](AGENTS.md), the one source of the instructions for each CLI. Read AGENTS.md for the full rules and the runtime facts. Change AGENTS.md, then run `evolve skills generate`. `evolve skills check` fails when this copy differs from AGENTS.md.
>
> **Operating policy (canonical, environment-independent): [docs/operations/operating-policy.md](docs/operations/operating-policy.md).** A pipeline issue gets a console-first fix at maximum reasoning. Routing is typed plumbing. Salvage comes before requeue. Wiring proofs are mandatory. The repo, not session memory, is the source of truth for these rules.
>
> Release notes: [CHANGELOG.md](CHANGELOG.md).

## Claude Code overlay

- The kernel guards fire as PreToolUse hooks, as `.claude/settings.json` specifies. `evolve guard phase` matches `Agent|Task`. During a cycle it denies the start of a phase persona (`evolve-*`) outside a phase cell. Inside a cell, each call passes (AGENTS.md invariant 1). The hooks are defence in depth; the sandbox and the gates hold each guarantee for every family.
- The four reviewers of the pre-commit review fleet are also Claude Code subagents in `.claude/agents/`. `evolve skills generate` renders the body of each subagent from its neutral persona `agents/evolve-diff-<name>.md`. Only the frontmatter (the tool list and the model) is Claude Code data. Change the persona, not the rendered body.
- The repository documents use the Claude Code tool names (`Read`, `Grep`, `Glob`, `Bash`, `Edit`, `Write`).
- Run `/clear` before a new evolve-loop batch.

<!-- GENERATED:instructions BEGIN from AGENTS.md by `evolve skills generate`; edit the source, not this region -->
## Digest (each CLI loads this part)

This digest has the facts that each agent must have when it starts. The full rules, the reasons and the evidence are in [AGENTS.md](AGENTS.md) and in the linked docs. Read them when your task touches their subject.

<!-- GENERATED:guidance-l0 BEGIN from guidance/rules by `evolve guidance build`; edit the registry, not this region -->
### Hard rules

- Never mutate or run a live kill guard. Inject the signal sender, so that a test cannot kill a real process.
- Never kill processes by a command pattern on the shared host. Kill only the process ids that you started.
- No LLM starts or directs another LLM. A phase agent starts only through the native bridge of the pipeline. (enforced by `go/internal/guards/phase.go`)
- Run the phases in the order Scout, Build, Audit, Ship. Never skip a phase, and never ignore a kernel-gate failure. (enforced by `go/internal/core/statemachine.go`)
- Commit and push only through evolve ship. The ship gate denies a bare git commit and a bare git push. (enforced by `go/internal/guards/ship.go`)
- The builder writes only inside the worktree that the runtime provisions. An agent never runs git worktree. (enforced by `go/internal/guards/role.go`)
- Ship only when the EGPS verdict has a red count of zero. Never edit the hash-chained ledger by hand.
- Never fabricate a cycle number, a test count or a verdict. Report only what a command printed.
- Do all development on a branch in a worktree, never on main. Main is the integration branch only.
- MANDATORY: code has no comments and no Go doc comments. Never write, ask for or test a comment. A reason goes to the package note. (enforced by `go/internal/commitgate/comments_added.go`)
- Write each document through the docs-write skill. The new text passes evolve docs ste-lint --strict. (enforced by `go/internal/stelint`)
- Reach each outside service through a CLI, never through MCP. Build a CLI when a service has none.
- Put research, lessons and failed approaches under docs/. Never delete a document; archive it.
- Read no control value from the environment. An operator setting lives in policy.json, and a bypass is a CLI parameter.
- Add no feature flag. Select each behavior through configuration data and a design pattern.
- Stop after two consecutive cycles that ship nothing, and find the root cause in the pipeline.
- Before you claim done, run the check and read its output. Report test counts as N/N PASS.
- Read a module and list its real exports before you call or import it. Never invent an API.
- Fix the root cause through a redesign. Never apply a workaround that hides a failure.
- Never duplicate logic, settings or prose. Keep each in one home, and generate each copy from it.

The full rule set is in [guidance/index.md](guidance/index.md). Run `evolve guidance index --audience <you>` to list the rules of your case.
<!-- GENERATED:guidance-l0 END -->

### Operating rules

- **Status reports** give the cycles that ran, the ships (for example 2/2) and the open PR numbers. They also give the failed CI jobs with their class, and the next action.
- **Bypass mode** means "do not ask the user". It does not mean "skip integrity checks". Run the full pipeline every cycle, and follow the failure-adapter verdicts verbatim.
- **Pre-commit review fleet:** code-simplifier, then architecture-reviewer and code-reviewer in parallel, then fix the findings, then commit. `evolve agent run <name>` runs each reviewer on any family. The console runs each review and each build through `evolve agent run`, and attests a review run with `--reviewers run:<run label>`. Fix each CRITICAL finding of architecture-reviewer before the commit. A diff of only `.md` files gets the `docs-review` skill and `--reviewers docs-review`; the code reviewers do not satisfy the gate. Only a proven comment removal needs no reviewer.
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
