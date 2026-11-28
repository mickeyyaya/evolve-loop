# AGENTS.md — Cross-CLI Canonical Instructions

> **Read this file first if you are an AI agent that works in this repository.** The agent can be Claude Code, Codex CLI, Antigravity/agy, Ollama, Gemini CLI or a generic agent. This file is the one source of the instructions for each CLI.
>
> `evolve skills generate` renders [CLAUDE.md](CLAUDE.md) and [GEMINI.md](GEMINI.md) from it. Each file has a short overlay for its CLI and a copy of the digest section of this file. Change this file, not a rendered copy. `evolve skills check` fails when a rendered copy differs from this file.
>
> **Operating policy (canonical):** [docs/operations/operating-policy.md](docs/operations/operating-policy.md) has the environment-independent process rules. Each rule has the incident evidence behind it. The rules cover:
>
> - the pipeline-integrity procedure (console-first, maximum reasoning);
> - the queue/routing authority;
> - the engineering standards;
> - the failure-handling contract;
> - the release policy.
>
> A clean environment gets the full policy from the repo alone. Session memory is only an advisory mirror.

<!-- instructions-digest BEGIN -->
## Digest (each CLI loads this part)

This digest has the facts that each agent must have when it starts. The full rules, the reasons and the evidence are in [AGENTS.md](AGENTS.md) and in the linked docs. Read them when your task touches their subject.

<!-- GENERATED:guidance-l0 BEGIN from guidance/rules by `evolve guidance build`; edit the registry, not this region -->
### Hard rules

- Top law: a self-healing, AI-driven loop that relentlessly builds the best-quality features, and fixes any issue with logic, research and the best practical option.
- Serve the top law, not a local metric. Finish inside each gate and limit, and report a conflict with the law.
- Never mutate or run a live kill guard. Inject the signal sender, so that a test cannot kill a real process.
- Never kill processes by a command pattern on the shared host. Kill only the process ids that you started.
- No LLM in the pipeline starts or directs another. The console acts for the operator, and runs a seat only through evolve agent run. (enforced by `go/internal/guards/phase.go`)
- Run the phases in the order Scout, Build, Audit, Ship. Never skip a phase, and never ignore a kernel-gate failure. (enforced by `go/internal/core/statemachine.go`)
- Commit and push only through evolve ship. The ship gate denies a bare git commit and a bare git push. (enforced by `go/internal/guards/ship.go`)
- The builder writes only inside the worktree that the runtime provisions. An agent never runs git worktree. (enforced by `go/internal/guards/role.go`)
- Ship only when the EGPS verdict has a red count of zero. Never edit the hash-chained ledger by hand.
- Never fabricate a cycle number, a test count or a verdict. Report only what a command printed.
- Do all development on a branch in a worktree, never on main. Main is the integration branch only.
- MANDATORY: code has no comments, except tool-read lines and the one package doc that docgo requires. A reason goes to the package note. (enforced by `go/internal/commitgate/comments_added.go`)
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
- Record each issue, alert or mistake through the issue-record skill, with its logs, results, root cause, fix and regression test.

The full rule set is in [guidance/index.md](guidance/index.md). Run `evolve guidance index --audience <you>` to list the rules of your case.
<!-- GENERATED:guidance-l0 END -->

### Operating rules

- **Status reports** give the cycles that ran, the ships (for example 2/2) and the open PR numbers. They also give the failed CI jobs with their class, and the next action.
- **Bypass mode** means "do not ask the user". It does not mean "skip integrity checks". Run the full pipeline every cycle, and follow the failure-adapter verdicts verbatim.
- **Pre-commit review fleet:** code-simplifier, then architecture-reviewer and code-reviewer in parallel, then fix the findings, then commit. `evolve agent run <name>` runs each reviewer on any family. The console runs each review and each build through `evolve agent run`, and attests a review run with `--reviewers run:<run label>`. Fix each CRITICAL finding of architecture-reviewer before the commit. A diff of only `.md` files gets the `docs-review` review, run with `evolve agent run docs-review`, and `--reviewers run:<run label>`; the code reviewers do not satisfy the gate. Only a proven comment removal needs no reviewer.
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
<!-- instructions-digest END -->

## What evolve-loop is

evolve-loop is a development pipeline that evolves itself. It orchestrates specialized phase agents (Intent, Scout, Triage, TDD, Builder, Auditor, and retro/memo). The agents go through a fixed spine in each cycle (Intent → Scout → Triage → [TDD] → Build → Audit → Ship → Learn). Since v22.27.0, a code cycle also runs the code-review phase between Build and Audit. It runs in shadow and is not a gate ([ADR-0124](docs/architecture/adr/0124-code-review-phase.md)). Tier-1 kernel hooks enforce these properties:

- the phase order;
- role-scoped write paths;
- atomic ship semantics;
- ledger SHA verification;
- tamper-evident, hash-chained records (v8.37+).

> **Runtime note (Go-only):** The Go binary (`go/bin/evolve`) is the only runtime entrypoint. The Go-only consolidation removed the bash `legacy/scripts/` tree. There is no bash fallback. Each operation below is a native `evolve <subcommand>` or a function in `go/internal/...`. For the history of the bash→Go port, see [docs/migration-from-bash.md](docs/migration-from-bash.md).

## Cross-CLI invariants (the universal rules)

These rules apply on every CLI. The OS sandbox, the worktree fence and the ship gates enforce them structurally for every family ([full detail](docs/operations/agents-full-rules.md#cross-cli-invariants-the-universal-rules)).

| # | Invariant | Kernel / gate |
|---|---|---|
| 1 | Phases run Scout → Builder → Auditor → Ship/Record in order. | `go/internal/core`, `evolve guard phase` |
| 2 | Phase agents start through `evolve subagent run` or the native bridge. | `evolve guard phase` |
| 3 | Commits go through `evolve ship`, never bare `git commit` or `git push`. | `evolve guard ship` |
| 4 | Builder writes only inside its cycle worktree. | `evolve guard role` |
| 5 | Audits gate on `acs-verdict.json:red_count == 0`. | `evolve acs suite` |
| 6 | `.evolve/ledger.jsonl` is hash-chained and tamper-evident. | `evolve ledger verify` |
| 7 | Failure adaptation is fluent by default (`workflow.strict_audit` for strict). | `go/internal/core` |
| 8 | Trivial diffs auto-downgrade Auditor from Opus to Sonnet. | `go/internal/subagent` |
| 9 | Document research and lessons under `docs/`; archive instead of delete. | `evolve guard docdelete` |
| 10 | Code carries no comments beyond tool-read markers and package doc. | `evolve commit-gate run` |
| 11 | Documents are written through the `docs-write` skill and pass STE lint. | `evolve docs ste-lint --strict` |
| 12 | Outside services are reached through a CLI, never MCP. | [cli-only-no-mcp](docs/conventions/cli-only-no-mcp.md) |
| 13 | The pipeline is the only controller; no LLM starts or directs another. | [ADR-0154](docs/architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) |

## 12 Core agent rules

Every agent follows the Karpathy foundations (1–4) and the Mnilax extensions (5–12). Read the full rules in [docs/operations/agents-full-rules.md](docs/operations/agents-full-rules.md#12-core-agent-rules).

## Per-CLI runtime details

The `cli_routing` table in `.evolve/policy.json` decides which CLI runs each phase. `evolve cli-routing explain <agent>` prints the chain of an agent ([ADR-0119](docs/architecture/adr/0119-one-routing-table-one-resolver.md)). `evolve bridge probe` shows the capability tier of each bridge manifest.

Each CLI reads one instruction file. The field `instruction_file` of its bridge manifest (`go/internal/bridge/manifests/<cli>.json`) names the file: a bare file name, or `inject`. For `inject`, the bridge puts the L0 rules and the pointer line at the start of the prompt. A test checks that each named file holds the L0 region. A CLI with its own file has a short overlay for that CLI and a rendered copy of the digest of this file.

The facts of each CLI are in one file for that CLI: [Claude Code](skills/loop/reference/claude-runtime.md), [Codex](skills/loop/reference/codex-runtime.md), [agy](skills/loop/reference/agy-runtime.md), [Gemini](skills/loop/reference/gemini-runtime.md) and [Ollama](skills/loop/reference/ollama-runtime.md). The tool names of each CLI are in `skills/loop/reference/<cli>-tools.md`. For another CLI, see [generic-runtime.md](skills/loop/reference/generic-runtime.md).

## Discovery contract for AI agents reading this file

If you are an AI agent that starts in this repository, do these steps:

1. **Identify your CLI**: Claude Code, Codex, Antigravity (agy), Ollama, Gemini, or other.
2. **Read the instruction file of your CLI** in full: CLAUDE.md, GEMINI.md, or AGENTS.md when your CLI has no file. Each file holds the digest of the cross-CLI rules, and they apply to you. Read AGENTS.md for the full rules.
3. **Read `docs/architecture/platform-compatibility.md`** when your CLI is not listed above.
4. **Discover available skills**: scan `.agents/skills/*/SKILL.md` (cross-CLI standard) or `skills/*/SKILL.md` (Claude Code primary).
5. **Discover available agents**: scan `agents/*.md`.

A skill file has YAML frontmatter (`name`, `description`), and then markdown instructions. Skills include subdirectories (`scripts/`, `references/`, `assets/`) for the resources that they use during execution.

## Trust boundary summary

The safety properties of the pipeline stack into three tiers:

| Tier | Layer | What it catches |
|---|---|---|
| Tier 1 | Kernel hooks (phase-gate, role-gate, ship-gate, ledger SHA, cycle binding, hash chain) | Reward hacking, phase-skipping, integrity breach, tampering |
| Tier 2 | OS isolation (sandbox-exec on macOS, bwrap on Linux) | A compromised builder that writes outside its sandbox |
| Tier 3 | Workflow defaults (intent capture, fan-out, mutation testing, adversarial audit) | Vague goals, sycophantic audits, tautological evals |

Tier 1 is non-negotiable. It runs in a privileged shell context. Tier 2 adapts to the environment. The operator controls Tier 3 for each run.

## Shared Constraints (v8.65.0+)

These constraints apply to ALL agents in the pipeline. They make sure of cost efficiency and structural integrity.

### 1. Tool Hygiene (P-NEW-9, P-NEW-21)
These rules prevent context saturation from accumulated tool results:
- **Summarize file reads**: After you read a file, summarize it in 2-3 lines. In later turns, refer to the summary.
- **Discard large blobs**: After a shell command with a large output, extract the key lines. Discard the full output.
- **Trajectory compression**: When you read a file of >3000 tokens, extract 3–5 key facts. Then discard the full content from the working context immediately.
- **No speculative loading**: Before you read a file, search by name and by text to find the points of interest.

### 2. Banned Patterns
- **No Self-Reversion**: Do not revert your own changes. Exceptions: an explicit instruction, or an immediate environment failure that the changes cause.
- **No Bare Git**: Commits MUST go through `evolve ship`.
- **No Pipeline Bypass**: Never try to skip a phase. Never ignore a kernel-gate failure.
- **No Post-Report Turns**: After you write the phase report (scout/build/audit/orchestrator), STOP. Turns that accumulate after the report is complete are a critical cost driver.

### 3. Flag → Parameter Conversion Standard (flag-reduction campaign)
This standard applies when a cycle converts an `EVOLVE_*` env flag into a typed input parameter. The conversion is "done" ONLY when it meets the **[Flag → Parameter Conversion Standard](docs/research/flag-parameter-conversion-standard.md)**:

1. The resolution package is environment-agnostic: no `os.Getenv`/`LookupEnv`/`Environ`.
   `paramPackages` in `go/internal/policy/param_env_agnostic_test.go` enforces this.
   The conversion MUST enroll the package in `paramPackages`.
2. It ships an env-free, black-box, public-API test suite that covers the full field × edge-case matrix.
3. Every exported parameter API is at 100% coverage, with `apicover -enforce` exit 0.

Tests drive behavior ONLY through input parameters. Never use `t.Setenv`. Reference template: `internal/quotareset` + `internal/policy` config accessors.

### 4. Minimalism (always-on)
Every code change takes the laziest solution that actually works:

- the ladder (YAGNI → stdlib → native/`policy.json` config → already-present dependency → one line → minimum);
- no unrequested abstraction;
- deletion over addition;
- the shortest diff that works.

Mark a deliberate shortcut with a `minimal:` comment that names the ceiling + upgrade path. The cut is in scope, NEVER in safety. Never simplify away these items:

- input validation;
- error handling;
- security;
- accessibility;
- explicit requests;
- the pipeline gates (RED test / safety invariants / eval+contract gates / ship floor).

Full ruleset: **[skills/minimalism/SKILL.md](skills/minimalism/SKILL.md)** (adapted from ponytail, MIT).

### 5. Code comments (always-on)
Code explains itself. A comment states only what the code cannot: a directive, a one-line contract on an export, or the *why* behind a non-obvious invariant. History never goes in code (cycle numbers, incidents, F-ids, dates). It goes to `docs/`. Full rule: **[docs/conventions/code-comments.md](docs/conventions/code-comments.md)**.

## Operating rules and runtime facts

These rules and facts apply on every CLI. Read the full operating rules and historical runtime facts in [docs/operations/agents-full-rules.md](docs/operations/agents-full-rules.md#operating-rules-and-runtime-facts), and the env-var table, operator commands, ship classes and publish pipeline in [docs/operations/runtime-reference.md](docs/operations/runtime-reference.md).

- Since 2026-10-07 the `cli_routing` table decides every dispatch:
  - The burn window of 2026-10-18 routed every phase to Claude Code and ended after the weekly reset. In the table today, `clis` is `[agy, agy-claude, claude]`, `default` is `[claude, agy, agy-claude]`, and `agents.builder` is `[agy, agy-claude, claude]`.
  - Non-deep, non-top seats follow `default` (`claude`, then `agy`, then `agy-claude`). On `agy`, `fast` pins Gemini 4 Hydrogen (Medium) and `balanced` pins Gemini 4 Hydrogen (High), which `builder` runs first. No tracked profile defaults to `fast`, and none lets an envelope fall to `fast`.
  - Since 2026-10-21 (ADR-0150), `tiers.deep` is `[claude, agy, agy-claude]` with `share_pct` 50. Thus `claude` takes half of the deep seats, paced by its week window. On `agy`, `deep` runs Gemini 4 Neon (High) and `top` runs Gemini 4 Neon (XHigh) (operator, 2026-11-21), and `agy-claude-tmux` runs Claude Opus 5.5. Since cycle 1911, a quota wall on one model group benches only that group, not all of `agy`. `tiers.top` is `[agy-claude, claude]`.
  - `agy-claude-tmux` waits up to 10 s for a claude-family footer label before the prompt. If no label shows, it exits 87. Exit 87 is a walk trigger, so the seat falls through. The usage query that follows can bench an exhausted Claude group, and preflight only warns on it.
  - The Claude floor seats (auditor, adversarial-review, tdd-engineer, code-reviewer and the spec verifiers) run Claude Code only. Here `code-reviewer` is the profile of the code-review phase, not the `diff-code-reviewer` profile of the review fleet. The floor is routing data, not code: `cli_routing.floors.claude` in `.evolve/policy.json`. With no `floors` key, the compiled default is the same list in `go/internal/policy/routing_defaults.json` (ADR-0154 I-6, X6 #1123). The floor profiles hold no `allowed_clis`; the floor is their only ceiling. Only `router` keeps its own ceiling.
  - codex is reachable only under `--bypass-policy`.
  - `evolve cli-routing explain <agent>` prints the chain of any agent.

### References

- [docs/operations/workspace-layout.md](docs/operations/workspace-layout.md) — the hub layout: the .repo.git bare store, the console/runtime planes and the dev/ worktrees (since 2026-09-01)
- [docs/operations/runtime-reference.md](docs/operations/runtime-reference.md) — the env-var table, the operator commands, the ship classes, how to publish
- [docs/architecture/](docs/architecture/) — design docs; [control-flags.md](docs/architecture/control-flags.md) — all `EVOLVE_*` flags
- [CHANGELOG.md](CHANGELOG.md) · [release-notes/](docs/operations/release-notes/index.md)

## Where to file issues

- Security vulnerabilities: see [SECURITY.md](SECURITY.md)
- Code of conduct: see [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- Contributions: see [CONTRIBUTING.md](CONTRIBUTING.md)
- Pipeline issues: GitHub Issues at https://github.com/mickeyyaya/evolve-loop/issues
- Architecture / release protocol: [docs/guides/publishing-releases.md](docs/guides/publishing-releases.md), [docs/architecture/tri-layer.md](docs/architecture/tri-layer.md)
