# AGENTS.md — Cross-CLI Canonical Instructions

> **Read this file first if you are an AI agent that works in this repository.** The agent can be Claude Code, Codex CLI, Antigravity/agy, Ollama, Gemini CLI or a generic agent. This file is the one source of the instructions for each CLI.
>
> `evolve skills generate` renders [CLAUDE.md](CLAUDE.md) and [GEMINI.md](GEMINI.md) from it. Each file has a short overlay for its CLI and a copy of this file from "What evolve-loop is" to the end. Change this file, not a rendered copy. `evolve skills check` fails when a rendered copy differs from this file.
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

## What evolve-loop is

evolve-loop is a development pipeline that evolves itself. It orchestrates specialized phase agents (Intent, Scout, Triage, TDD, Builder, Auditor, and retro/memo). The agents go through a fixed spine in each cycle (Intent → Scout → Triage → [TDD] → Build → Audit → Ship → Learn). Since v22.27.0, a code cycle also runs the code-review phase between Build and Audit. It runs in shadow and is not a gate ([ADR-0124](docs/architecture/adr/0124-code-review-phase.md)). Tier-1 kernel hooks enforce these properties:

- the phase order;
- role-scoped write paths;
- atomic ship semantics;
- ledger SHA verification;
- tamper-evident, hash-chained records (v8.37+).

> **Runtime note (Go-only):** The Go binary (`go/bin/evolve`) is the only runtime entrypoint. The Go-only consolidation removed the bash `legacy/scripts/` tree. There is no bash fallback. Each operation below is a native `evolve <subcommand>` or a function in `go/internal/...`. For the history of the bash→Go port, see [docs/migration-from-bash.md](docs/migration-from-bash.md).

## Cross-CLI invariants (the universal rules)

These rules apply on every CLI that you run under. They are STRUCTURAL: kernel hooks enforce them, not prompt instructions.

### 1. Pipeline ordering is non-negotiable
Phases run Scout → Builder → Auditor → Ship/Record, in that exact order. The Go orchestrator state machine (`go/internal/core`) enforces the phase ORDER. The phase-gate kernel hook (`evolve guard phase`) denies in-process subagent dispatch while a cycle is active. Per ADR-0075, this hook was rewired onto the `Agent|Task` matcher. The Go guard denies only `Agent` today (`go/internal/guards/phase.go:22`), so a `Task` call passes. X7 of ADR-0154 fixes it.

The emergency operator override is the explicit `evolve guard phase --bypass` CLI flag. With it, the guard allows the call and writes no special log line. Each use is a CRITICAL violation.

### 2. Subagents start through the native bridge, never through in-process tool calls
The native runner spawns every phase agent. The runner is `evolve subagent run <agent> <cycle> <workspace>`, or the in-process `go/internal/bridge` launcher that `evolve loop` / `evolve cycle run` drive. The kernel hook enforces this rule. During a cycle, the hook **denies** the in-process `Agent` (Claude Code) / `activate_skill` (Gemini) / equivalent (Codex). Reason: in-process subagents bypass the profile-scoped permissions and the tamper-evident ledger.

### 3. Commits go through `evolve ship`, never bare `git commit / git push`
The ship-gate kernel hook (`evolve guard ship`) denies a bare git commit/push/gh release create. The only canonical entry point is the native `evolve ship`. It enforces audit verification, cycle binding (HEAD + tree_state_sha match) and version-aware self-SHA pinning. Operator escape: `--class manual` (interactive) or the explicit `evolve guard ship --bypass` emergency flag.

Interactive `--class manual` commits also require a fresh **commit-gate review attestation** (v13.0.0+). The attestation is a `.commit-gate/attestation.json` whose `tree_state_sha` matches the staged tree. To make it, use `/commit` (code-simplifier + code-reviewer + language reviewer + lint + targeted tests). `evolve ship --bypass-commit-gate` skips the check. A routine use is a policy violation, identical in spirit to the emergency bypass of the ship guard.

### 4. Builder writes only inside its worktree
Each cycle gets its own git worktree. The orchestrator in `go/internal/core` provisions it natively and records it in `cycle-state.json:active_worktree`. The Builder profile (`.evolve/profiles/builder.json`) restricts Edit/Write to the worktree path. The role-gate kernel hook (`evolve guard role`) denies edits outside that boundary. This includes the leaks through interpreter-execution Bash redirects.

### 5. Audits are gated by the EGPS binary verdict
The Auditor writes `audit-report.md`. The ship gate is the binary `acs-verdict.json:red_count == 0` (EGPS v10.0.0+). EGPS v10.0.0 removed the scalar PASS/WARN/FAIL confidence level. The native audit phase (`go/internal/phases/audit` + `evolve acs suite`) computes the verdict from sandbox exit codes, never from the narrative of a model. (Strict WARN→FAIL promotion is opt-in through `.evolve/policy.json` `workflow.strict_audit`; see [reference/env-vars](knowledge/reference/env-vars.md).)

### 6. The ledger is tamper-evident (v8.37.0+)
`.evolve/ledger.jsonl` records every subagent invocation with the cycle binding, the challenge token, the artifact SHA, **prev_hash** and **entry_seq**. The prev_hash of each new entry is the SHA256 of the full JSON line of the previous entry. `.evolve/ledger.tip` records the SHA of the latest entry atomically, for truncation detection. To confirm the history integrity, run `evolve ledger verify` (or `evolve guard chain`). A change to any historical entry breaks the chain at the next entry.

### 7. Failure adaptation is fluent-by-default (v8.28.0+)
`state.json:failedApproaches[]` records the prior failures, with structured classifications (for example, infrastructure-transient, code-audit-fail, code-audit-warn). The native failure-adapter (`go/internal/core`) returns deterministic decisions. The orchestrator follows them verbatim. The default mode is fluent: a rule that finds a block condition emits awareness, not BLOCK. Strict mode (`.evolve/policy.json` → `workflow.strict_audit: true`) restores the legacy behavior: block when a failure occurs again.

### 8. Cost adaptation (v8.35.0+)
The auditor profile defaults to Opus. But for trivial diffs (≤3 files, ≤100 lines, no security paths), the native diff-complexity check (`go/internal/subagent`, modeltier) auto-downgrades to Sonnet. This saves about $1.89/cycle on routine cycles. Operator override: `MODEL_TIER_HINT=opus` forces Opus in all cases.

### 9. Knowledge Stewardship Rule (Day-One)

> **Knowledge Stewardship Rule (Day-One):** You MUST document every research finding, discovery, cycle lesson or tried-and-failed approach before the cycle ships. All documentation is under `docs/`, never in code comments ([code-comments convention](docs/conventions/code-comments.md)). Research notes go in `docs/research/`. (The former `knowledge-base/research/` tree moved there on 2026-08-05; see [docs/MOVED.md](docs/MOVED.md).) `knowledge-base/` is a runtime write surface only (`cycles/` dossiers), not documentation.
>
> **Never delete; always archive.** When a new doc supersedes an old doc, MOVE the old doc to `docs/private/research/archived-YYYY-MM-DD/`. Put a one-line note in the replacement that points to the archive. A failure to document is a HIGH-severity audit defect.

The doc-deletion guard enforces this rule (`evolve guard docdelete`, `go/internal/guards/docdelete.go`; a PreToolUse kernel hook). It blocks an `rm`/`mv` that removes committed content from `docs/**` or `knowledge-base/**`, unless the destination is under `docs/`. To archive, use `git mv` into `docs/private/research/archived-YYYY-MM-DD/`, so that the archived copy stays staged. `knowledge-base/` is deliberately NOT a valid destination.

The one exception: a Build can `git rm` the explanation document of the active cycle (`docs/explain/builds/cycle-<N>-<run>.md`) if that document was never committed. Operator escape: set `workflow.allow_doc_delete=true` in `.evolve/policy.json` (logged; emergency only).

### 10. Code carries no comments; it explains itself (2026-09-30)

> Every agent, phase and person writes code with no comments. Names, types, small functions and test names say what the code does. A design reason goes to the notes of the package under `docs/architecture/packages/`. Only the comments that a tool reads survive. [The code-comments convention](docs/conventions/code-comments.md) is the one list of them. When you remove a comment, the code must say what the comment said.

The commit gate (`evolve commit-gate run`) refuses a commit that adds a comment. Loop lanes: the build handoff floor counts the comments that a build adds (`comment_floor`). [The code-comments convention](docs/conventions/code-comments.md) records its rollout stage.

### 11. Documents are written through the `docs-write` skill (2026-10-12)

Before you write or change a document that a person reads, load the `docs-write` skill (`skills/docs-write/SKILL.md`). This covers research, plans, ADRs, architecture specs, conventions, guides and incident records. Each decision in the document cites its evidence and its rejected options. The new text passes `evolve docs ste-lint --strict` and the link check. The kernel preload of its compact form into each loop dispatch that writes a document is plan K2 ([ADR-0132](docs/architecture/adr/0132-documents-are-written-through-one-skill.md)).

### 12. Outside services are reached through a CLI, never MCP (2026-10-17)

No phase, lane or project config uses an MCP server. Reach each outside service through a CLI, and build an `evolve` verb or an adapter CLI when none exists ([the cli-only-no-mcp convention](docs/conventions/cli-only-no-mcp.md)).

### 13. The pipeline is the only controller (2026-10-25, proposed)

No LLM controls another LLM. The `evolve` binary starts each agent in an isolated cell, carries each message and routes each seat by data ([ADR-0154](docs/architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md), [spec](docs/architecture/llm-isolation.md)). The `evolve` CLI is the interface for other services ([cli-contract.md](docs/architecture/cli-contract.md)). The pipeline must read no control flag from the environment ([ADR-0156](docs/architecture/adr/0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md)). The task assignment is CLI-neutral, and only the bridge drivers hold CLI specifics (ADR-0154 I-17). The components X1 to X19 build these rules, and X1, X2 and X6 are on `main`.

## 12 Core agent rules

These are behavioral rules that every agent must follow, on every CLI. The kernel hooks above catch *structural* breaches. These rules catch *judgment* breaches. Bypass-permissions / autonomous mode overrides rule 4 ("stop and ask"): make the reasonable call and continue. All other rules apply unconditionally.

### Karpathy foundations (1–4): think before acting

1. **Think before coding.** Do not assume, and do not over-engineer. State the assumptions explicitly. If a requested approach is wrong, push back and propose the alternative.
2. **Push for simplicity.** If a simpler approach exists, propose it. Three similar lines beat a premature abstraction. Do not design for hypothetical future requirements.
3. **No silent changes.** When the instructions admit more than one interpretation, or the codebase offers more than one pattern, show both. Then pick one explicitly, with a one-line reason. Never guess silently.
4. **Surface ambiguity.** If something is unclear or you do not have the context, stop and ask. (Bypass-permissions mode overrides this rule: make the reasonable call and continue. The user will redirect.)

### Mnilax extensions (5–12): multi-step agent discipline

5. **Reserve judgment tasks for AI.** Use LLM cycles for qualitative work: summaries, categories, drafts, designs. Put deterministic work (retries, error codes, state transitions, hashes) in the Go kernel. evolve-loop already enforces this rule. The phase gate, ship and the failure-adapter (`go/internal/core`, `go/internal/phases/ship`) are deterministic Go. Scout/Builder/Auditor are LLM.
6. **Strict token budgeting.** Stay in your per-task and per-session token budget. When you come near the limit, summarize the state to a markdown file and stop the phase. Do not let the phase auto-resume. See [docs/architecture/checkpoint-resume.md](docs/architecture/checkpoint-resume.md).

   (This rule is about your own work discipline, not a runtime cap. The product removed its token-budget *cost* gates, because the dollar-cost calculation was unreliable across LLM models. Thus the former dollar-cost budget flags (and `--budget-usd N`) are gone, and cost is display-only telemetry.)
7. **Address conflicting patterns.** If two patterns conflict in scope, do not mix them. For example: bash 3.2 vs bash 4 features; `skills/` canonical vs `.agents/skills/` canonical. Pick one. Record the reason in the commit body. Tag the other for cleanup.
8. **Read first.** Before you import from a module or call into it, `Read` it and list its real exports. Do not invent function names from context. This rule prevents a failure mode that recurs: Builder agents ship code against imagined APIs.
9. **Write meaningful tests.** Tests verify *intent*, not surface behavior. The eval-quality check (`evolve eval quality-check`, EGPS v10.0+) rejects EGPS predicates that pass with `echo PASS; exit 0` or `grep -q presence`. The same principle applies to Go unit tests. A test that passes but does not probe the behavior change is a no-op.
10. **Use checkpoints.** For multi-phase work, after every phase, summarize what is done and what remains. evolve-loop encodes this rule in the native cycle-state checkpoint + `phase report` artifacts. If you cannot clearly describe the current status in 3 bullets, stop. You lost the plot.
11. **Follow existing conventions.** Match the codebase, even if you disagree. Specifics:
    - Go is the runtime (`go/internal/...`).
    - The few shell helpers that remain (test fixtures, the commit-gate runner) target bash 3.2 (no `declare -A`, no `mapfile`, no `${var^^}`). They use `printf > tmp && mv` for atomic writes.
    - Commits go through `evolve ship`.
    - Kernel guards are `evolve guard <name>`. They split read roots (`PLUGIN_ROOT`) from write roots (`PROJECT_ROOT`).
12. **Fail loudly.** Do not silently skip steps. Do not swallow errors. Do not report success when the work is incomplete. This rule prevents a failure mode: a "migration complete" claim that secretly skipped 3 files. The format for completion claims: `cd go && go test ./internal/<pkg>/... — N/N PASS, no regression`.

## Per-CLI runtime details

The `cli_routing` table in `.evolve/policy.json` decides which CLI runs each phase. `evolve cli-routing explain <agent>` prints the chain of an agent ([ADR-0119](docs/architecture/adr/0119-one-routing-table-one-resolver.md)).

Each CLI reads its own instruction file. `CLAUDE.md` and `GEMINI.md` each have a short overlay for the CLI and a rendered copy of AGENTS.md. A CLI with no file of its own reads AGENTS.md. The notes for each CLI:

- **Claude Code**: reads [CLAUDE.md](CLAUDE.md). Tier-1 production. The skills at `skills/<name>/SKILL.md` are the only invocation/slash-command surface (per ADR-0040). They carry `argument-hint`. The plugin manifest at `.claude-plugin/plugin.json` declares only `agents` and `skills` (no `commands[]` array). Kernel hooks fire as PreToolUse hooks, as `.claude/settings.json` specifies.

- **Codex CLI**: Codex discovers skills automatically at `.agents/skills/<name>/SKILL.md`. (This directory has symlinks to `skills/<name>/`.) Codex reads this AGENTS.md as its canonical config. The native codex bridge drivers (`go/internal/bridge/driver_codex*.go`) drive Codex. No mode delegates to `claude`.

  `evolve bridge probe` shows the capability tier of each manifest. The tier comes from `resolveTier` (`go/internal/bridge/probe.go:10-27`):
  - `none` when the manifest binary is not on PATH;
  - the manifest `default_tier` (`full` for `codex`, `hybrid` for `codex-tmux`) when each dependency of that tier is on PATH;
  - `degraded` when a dependency is absent, for example `tmux` for `codex-tmux`.

- **Gemini CLI**: Gemini discovers skills automatically at `.agents/skills/<name>/SKILL.md`. See [GEMINI.md](GEMINI.md) for Gemini-specific notes. `gemini` is a distinct CLI identity with its own adapter metadata (`adapters/gemini.capabilities.json`). There is **no** dedicated `driver_gemini*.go` bridge driver.

  A Gemini *model* is also reachable natively through the Antigravity (agy) driver (`go/internal/bridge/driver_agy*.go`; the code documents it as "Gemini-backed"). But `gemini` and `agy`/`antigravity` are separate CLI identities. Only `antigravity → agy` has a name resolution (see the Antigravity bullet below).

- **Antigravity CLI (agy)**: agy discovers skills automatically at `.agents/skills/<name>/SKILL.md`. The native agy bridge driver (`go/internal/bridge/driver_agy*.go`) drives it. Its capability tier follows the same `resolveTier` rule: `none` when `agy` is not on PATH, else the manifest `default_tier` (`full` for `agy`, `hybrid` for `agy-tmux`). `agy-tmux` is `degraded` when `tmux` is absent, and no mode delegates to `claude`. The cross-name resolver maps `antigravity → agy`. The legacy `adapters/agy.capabilities.json` sets `cost_blind: true` (deferred billing tap).

  The `agy-claude-tmux` driver routes to the Claude models that the agy binary serves (routing family `agy-claude`). Before the prompt, it verifies that agy booted a Claude model. If not, it exits 87.

  See [reference/agy-runtime.md](skills/loop/reference/agy-runtime.md). Capability tier: `evolve bridge probe`.

- **Ollama (local models)**: the native ollama tmux-REPL bridge driver (`go/internal/bridge/driver_ollamatmux.go`) drives it. The driver launches `ollama run <model>` in a tmux pane. It reuses the shared REPL completion-detection machinery. Ollama is a routing target for local models, with no skill/plugin surface of its own. Research record: [docs/research/ollama-control-surface-2026.md](docs/research/ollama-control-surface-2026.md).

- **Generic / unsupported CLI**: see [skills/loop/reference/generic-runtime.md](skills/loop/reference/generic-runtime.md). The tool name translation tables are at `skills/loop/reference/<platform>-tools.md`.

## Discovery contract for AI agents reading this file

If you are an AI agent that starts in this repository, do these steps:

1. **Identify your CLI**: Claude Code, Codex, Antigravity (agy), Ollama, Gemini, or other.
2. **Read the instruction file of your CLI** in full: CLAUDE.md, GEMINI.md, or AGENTS.md when your CLI has no file. Each file holds the cross-CLI invariants, and they apply to you.
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
- **Summarize Reads**: After each `Read`, summarize the content in 2-3 lines. In later turns, refer to the summary.
- **Discard Large Blobs**: After each `Bash` with a large output, extract the key lines. Discard the full output.
- **Trajectory Compression**: When you read a file of >3000 tokens, extract 3–5 key facts. Then discard the full content from the working context immediately.
- **No Speculative Loading**: Before a `Read`, use `Glob`+`Grep` to find the points of interest.

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

These rules and facts apply on every CLI. The full runtime detail is in [docs/operations/runtime-reference.md](docs/operations/runtime-reference.md): the env-var table, the operator commands, the ship classes and the publish pipeline. Read it before you change loop behavior, flags, gates or releases.

### Automation Loop Guardrails

- When you run the evolve loop or any batch/merge-train automation, evaluate the output after EVERY cycle. If 2 consecutive cycles produce zero ships (0 merged PRs), STOP the loop immediately. Find the root cause in the pipeline before you run another wave. The canonical zero-ship halt rule is an operator guardrail, not a compiled breaker: [docs/operations/operating-policy.md §4.1](docs/operations/operating-policy.md).
- Never let a batch run more than 2 unproductive waves "to see if it self-corrects".
- ADR-0072 reconciliation: a zero-ship streak is evidence of a SYSTEM fail, so HALT + P0 applies. This does not conflict with "never stop the queue". That rule governs task-level failures.

### Status Reporting

When you report on a batch or a pipeline run, always include these items:

- the cycles that ran;
- the ships or lanes that completed (for example, 2/2);
- the open PR numbers;
- the names of the failed CI jobs, with the failure class;
- the specific next action.

Do not report "running" or "in progress" as a status without these numbers.

### Session conventions

- **Confirm direction first**: multi-step or multi-cycle work needs a 3-bullet plan and an approval. These tasks are exempt: single-cycle bug fixes, tasks that specify a file path, and tasks from an approved plan.
- **Output discipline**: give summaries with `file:line` refs. Put findings of more than 300 lines in a markdown file, not in the chat.
- **Jobs that run for a long time**: after the launch, verify the health (exit codes, log tail). Make a checkpoint every cycle so that `--resume` works. Report failures immediately.
- **Pre-commit review fleet**: change → code-simplifier → (**architecture-reviewer ∥ code-reviewer**, parallel, both read-only) → address the findings → commit.
  - Each reviewer is a profile: `evolve agent run <name>` runs it on any family, with the diff as the input. The profile of each reviewer is `.evolve/profiles/diff-<name>.json`. The four reviewers are `architecture-reviewer`, `code-reviewer`, `code-simplifier` and `go-reviewer`. Their personas are `agents/evolve-diff-<name>.md`.
  - architecture-reviewer ([agents/evolve-diff-architecture-reviewer.md](agents/evolve-diff-architecture-reviewer.md)) blocks the commit: you must fix each CRITICAL finding (by the rubric of the profile) before the commit.
  - A pure-docs diff (every changed path ends in `.md`) gets a docs review, not a code review. Run the `docs-review` skill and pass `--reviewers docs-review`. The commit gate requires that capability, and the code reviewers do not satisfy it.
  - Only a proven comment removal needs no reviewer.

### Autonomous execution (bypass mode)

Bypass = "don't ask the user", NOT "skip integrity checks". These rules are mandatory (the full text is in runtime-reference.md):

1. Continue all cycles without a pause. Never ask "should I continue?".
2. Run the FULL pipeline every cycle: real `scout-report.md` / `build-report.md` / `audit-report.md`.
3. The Go orchestrator state machine (`go/internal/core`) enforces the phase order at every transition.
   `evolve guard phase` denies an in-process subagent dispatch while a cycle is active. Invariant 1 has the open gap, and X7 of [ADR-0154](docs/architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md) closes it (matrix row N11).
4. Never fabricate cycle numbers (CRITICAL violation).
5. Phase agents go through the native bridge (`evolve subagent run` / `evolve loop`). The in-process subagent tool is denied.
6. The OS sandbox wraps subprocesses (`EVOLVE_SANDBOX=1`). When the sandbox is nested, the EPERM fallback turns on automatically.
7. Run the eval-quality pre-flight on every eval (`evolve eval quality-check`).
8. The Adversarial Auditor is on by default. The auditor runs Claude Opus on Claude Code, and the builder runs agy first. `ADVERSARIAL_AUDIT=0` turns it off.

Maximum velocity, zero shortcuts. The runtime provisions worktrees natively: agents must NOT call `git worktree`. Follow the failure-adapter verdicts (PROCEED/RETRY/BLOCK) verbatim. `evolve ledger verify` checks the chain.

### Verification before claiming done

1. Before you declare a CLI unavailable, probe it: `evolve doctor probe <tool>`. List what you checked.
2. Read the actual exports before you import from a module or call it.
3. Run the tests and report the counts: `cd go && go test ./internal/<pkg>/... — N/N PASS, no regression`.

### CI Failures

- Classify each CI failure as a flake or a real defect before you retry it. Use `evolve ci classify <run-id|pr:N|sha:H> [--json] [--rerun]`. Exit 0 = retry-safe: every red is pre-existing or flake-evidence. Do not retry an unclassified red.
- File a regression issue for each failure class that occurs 2+ times. Link the issue in the PR description.
- Ship gates: if a gate reports RED, verify the logic of the gate before you assume that the code is broken. False-RED gates shipped in the past.

### Go Project Conventions

After you edit a Go file, run `gofmt -l .`, `go vet ./...` and `go test -count=1 ./...` (from the `go/` module root) before you open a PR.

**`-count=1` is required, not optional.** The Go test cache does not track file reads that escape the module root through `..`. Many tests read `.evolve/profiles/*.json` that way. Thus, after a config-only edit, a bare `go test ./...` serves a stale `ok (cached)` while the regression is live on disk (reproduced 2026-08-28). CI and `make test` already pass `-count=1`. The gap was local verification.

Add table-driven edge-case tests for each bug whose root cause was found during a pipeline stall.

### Shell conventions

- The target is bash 3.2.
- Banned: `declare -A`, `mapfile`, `${var^^}`, `sed -i ''`, `date -d`.
- Required: `set -uo pipefail` (not `set -e`), atomic writes through `mv "${f}.tmp.$$"`, and `git diff HEAD` for the tree-state SHA.
- `skills/<name>/` is canonical. `.agents/skills/` are symlinks.
- The full table, with the reasons and the portable alternatives, is in [runtime-reference.md](docs/operations/runtime-reference.md).

### /evo:loop task priority

Task priority is the computed inbox rank ([ADR-0121](docs/architecture/adr/0121-inbox-priority-is-a-computed-rank.md), `evolve inbox rank --explain`). The class order is in `.evolve/policy.json` `inbox_priority.class_order`.

### Critical runtime facts (full table → runtime-reference.md)

- The gates are default-ON as **compiled Go defaults** (`internal/policy` + `internal/config`). These defaults apply when the policy block is absent:
  - `eval_gate=enforce`;
  - `contract_gate=enforce`;
  - `repo_contract_gate=enforce`: the ship-time repo-contract scanner pack (`internal/phases/ship/repocontract.go`). Lane ships can no longer red main. The build handoff floor runs the same pack first, so a red goes back to the build. Both run the pack only in the module of evolve-loop itself (`internal/repocontract`);
  - EGPS `red_count==0` to ship;
  - the tdd phase is enabled.
- `.evolve/policy.json` CAN override these defaults through a `gates`/`workflow` block. But the checked-in file sets only floor/cli_health/cli_routing/catalog/acs/parallel_evaluate/fleet/failure_disposition/gc/inbox_priority/docs_floor/audit_cascade. It does not contain the gate keys. Do not assume that policy.json is the source of the defaults. Details for some of these keys (see runtime-reference):
  - `gc`: the retention horizons, since 2026-09-29.
  - `inbox_priority`: the class order and the factor weights of the inbox rank, since 2026-10-06 (ADR-0121). It is strict-decoded. The plan's P2 (2026-10-06) wired it: every lane consumer reads `inboxrank.Order`.
  - `docs_floor`: `ste_stage: enforce`, since 2026-10-21 (operator decision). A lane doc that fails the STE lint goes back to the build.
  - `audit_cascade`: `stage: shadow`, since 2026-10-21 (operator decision). Each audit records if a cheap-tier pass escalates. No dispatch and no verdict change.
  - `gc.mode: enforce`, since 2026-10-06. Thus the batch-start and batch-end GC of the loop applies its run-dir and worktree sweeps, and does not only report them. `evolve gc` at the boundary still owns the cache, temp and process sweeps. The cache has a 20 GB compiled-default cap, `gc.go_cache_max_gb`.
  - `cli_routing`: the CLI routing table, since 2026-10-07 (L2, [cli-routing-table-2026-10.md](docs/plans/cli-routing-table-2026-10.md)). It replaces the former `workflow.universal_fallback_exclude` and `pins` keys. `evolve cli-routing migrate` moves `pins`, `workflow.universal_fallback` and `router.cli/model` into it. It refuses a non-empty `workflow.universal_fallback_exclude`: drop those families from `clis`, then remove the key.
- Since 2026-08-10: `failure_disposition.stage=enforce`. The escalation boundary is LIVE (it was shadow).
- `fleet.count=4` and `fleet.min_lanes=4`, since the burn window ended on 2026-10-18 (ADR-0139 D3). The burn window ran on 2026-10-18 with `fleet.count=8` and `fleet.min_lanes=8`. It ended after the weekly Claude reset, and the operator set the width back to 4. The quota shrink counts a bench only when a chain has no healthy family (O1).
  - `fleet.orchestrator.stage=enforce`, since 2026-10-18. The stage stays `enforce` after the burn window. At `enforce`, admission by cell holds a todo whose cell is busy, and the rolling pool (O4) backfills a free slot at once.
  - From 2026-10-17 to 2026-10-18, before the burn window, `fleet.count` was 4 and `fleet.min_lanes` was 2 (ADR-0139 D3).
  - Before that, `fleet.count=3` was restored from the August codex-quota reduction. It was verified live on 2026-08-24. Over cycles 1530-1552 there were 76 codex dispatches / 44% share and zero quota halts.
  - From 2026-09-10 until 2026-10-07, deep/top ran the deep model of the codex family manifest (gpt-5.6-sol) at high effort. On 2026-10-07 the routing table took codex out of every chain.
  - gpt-5.6-sol is the value record of the CHANGELOG. The 2026-09-09 gpt-6-astra cutover was withdrawn the next day for token cost. Claude deep/top stay opus.
  - The resolution is policy pin > live catalog > manifest baseline. The manifest `codex-tmux.json` is the ONE tier table of the family.
- Since 2026-10-07 the `cli_routing` table decides every dispatch:
  - The burn window of 2026-10-18 routed every phase to Claude Code. It ended after the weekly reset. The table is back to the agy-first routing: `clis` and `default` are `[agy, agy-claude, claude]`, and there is no builder exception.
  - Every non-deep, non-top phase runs agy first (Gemini 3.8 Flash High at balanced). No tracked profile defaults to fast, and no tracked profile lets an envelope fall to fast.
  - Since 2026-10-17 deep seats run agy's Gemini 4 Argon (Medium) first, then agy-owned Claude (`agy-claude-tmux`, Claude Opus 5.5), then Claude Code. The operator asked to share the deep build with Argon. Since cycle 1911 an Argon quota wall benches only the Argon model group, not all of agy. Top seats stay agy-owned Claude, then Claude Code.
  - `agy-claude-tmux` waits up to 10 s for a claude-family footer label before the prompt. If no label shows, it exits 87. Exit 87 is a walk trigger, so the seat falls through. The usage query that follows can bench an exhausted Claude group, and preflight only warns on it.
  - The Claude floor seats (auditor, adversarial-review, tdd-engineer, code-reviewer and the spec verifiers) run Claude Code only. Here `code-reviewer` is the profile of the code-review phase, not the `diff-code-reviewer` profile of the review fleet. The floor is routing data, not code: `cli_routing.floors.claude` in `.evolve/policy.json`. With no `floors` key, the compiled default is the same list in `go/internal/policy/routing_defaults.json` (ADR-0154 I-6, X6 #1123). Seven profiles still pin `allowed_clis`; item `isolation-floor-profiles-allowed-clis-as-routing-data` moves the pins.
  - codex is reachable only under `--bypass-policy`.
  - `evolve cli-routing explain <agent>` prints the chain of any agent.
- Documents: load `docs-write` before you write any document that a person reads. Each decision cites its evidence and its rejected options ([ADR-0132](docs/architecture/adr/0132-documents-are-written-through-one-skill.md), AGENTS.md invariant 11).
- MCP: no phase, lane or project config uses an MCP server. Reach each outside service through a CLI, or build one (AGENTS.md invariant 12, [docs/conventions/cli-only-no-mcp.md](docs/conventions/cli-only-no-mcp.md)). Each Claude Code launch gets `--strict-mcp-config` from its manifest, and preflight warns about a configured MCP server.
- Comments: code has no comments. The code explains itself (AGENTS.md invariant 10, [docs/conventions/code-comments.md](docs/conventions/code-comments.md)). The commit gate refuses a commit that adds one. In loop lanes, the `comment_floor` of the build floor counts them (rollout stage: the convention).
- Boot self-heal: `boot.binary_refresh=auto` is a **compiled default** (`cmd_loop_boot_refresh.go`). If the build stamp of the current binary is behind HEAD with a go/ delta, the boot does a rebuild + re-exec. Use `off` only for deliberate pins of an old binary. Unknown words resolve to `auto`.
- Contract blocks: the second consecutive contract-gate block escalates the re-dispatch CLI (a soft overlay, `internal/core/contract_escalation.go`). Escalation comes before breaker demotion. Salvage rungs are breaker-neutral.
  - Exception: an optional evaluate phase past the ship floor that is not configured mandatory (code-review, ADR-0124) is breaker-exempt. Thus it gets no escalation and no salvage re-prompt. An exhausted ladder degrades a present-but-malformed report to SKIPPED+WARN. It aborts on an absent report.
  - When an identical-fingerprint halt occurs again, run `evolve loop --reset --fingerprint <fp>`. The command acks the fingerprint into `.evolve/resolved-fingerprints.json`.
- The default execution is the tmux-LLM drivers (for example, `claude-tmux`). Headless `claude -p` is opt-in only. The runtime detects the Claude OAuth from the macOS Keychain. There are five routing families:
  - claude;
  - codex;
  - agy (the Gemini models of Antigravity);
  - agy-claude (the Claude models that the agy binary serves, driver `agy-claude-tmux`);
  - ollama.
- Commits: the ship gate denies a bare `git commit` / `git push origin main`. Interactive commits: `/commit` → attestation → `evolve ship --class manual`. A routine use of `--bypass-commit-gate` is a violation. Cycle commits: `--class cycle` (full audit-binding). Releases: `evolve release X.Y.Z`. "publish" ≠ "push".
- For an unfinished cycle, use `evolve loop --resume` or `evolve cycle reset`. `evolve loop --force-fresh` is the last-resort escape hatch. It does NOT seal the history.
- Routing:
  - `EVOLVE_DYNAMIC_ROUTING=advisory` is the default since 2026-06-06, when retro steps 1-3 landed. `=off` is the static escape hatch.
  - The integrity floor is `ship ⇒ build ∧ audit ∧ (tdd unless trivial or deliverable_kind=document — ADR-0099)`.
  - CLI routing is in `.evolve/policy.json` `cli_routing` (one table: `clis`, `default`, `work`, `tiers`, `agents`, `floors`).
  - `pins` beside a table refuse every launch. Thus `evolve cli-routing init|set|unset|migrate` is the only writer of the table. It compiles before it writes. It refuses to write inside a phase or while a cycle holds a live lease.
  - `evolve setup recommend|apply` refuse on a declared project.
  - The policy bypass (`--bypass-policy`) is off by default.
  - Swarm: stage=shadow. This is a **compiled default**. `.evolve/policy.json` `swarm.stage` can override it (the checked-in file does not set it).
- Observer auto-spawn is on by default as a **compiled Go default** (`internal/policy`; stall 600s).
  - The observer judges a tmux phase on the pane-watch snapshot of the bridge (`<ws>/<agent>-pane-watch.json`: a normalized transcript hash, with the writer pid checked).
  - It judges a headless phase on stdout and on workspace growth.
  - A stall signals `LIVENESS_PHASE_STALLED` and kills nothing.
  - `evolve bridge sessions` lists every live pane, read-only.
  - A `.evolve/policy.json` `observer` block can override the default (the checked-in file does not set it).
- Operator verbs since v22.27.0 (details in runtime-reference):
  - `evolve checkpoint save|list|restore|prune` keeps the uncommitted work of a dev worktree as refs (ADR-0122).
  - `evolve cli update` installs a newer Claude Code or agy and smoke-tests it. The loop runs it at each wave boundary.
  - `evolve wave next|status|watch|note` (2026-10-08) runs and watches waves. `wave next` runs the boundary, composes the goal from facts and records the wave. Its `--json` prints one envelope on every exit ([wave-operations.md](docs/operations/wave-operations.md)). `boundary run` stays the low-level primitive.
  - `evolve convergence decide` gives the next step of a console lane's review chain (ADR-0126). The pipeline loops do not call it yet.
  - `workflow.convergence` is a compiled default at stage shadow.
  - `evolve agent run <profile>` runs one read-only phase agent with no cycle and prints its report; `evolve agent list` names the profiles ([ADR-0152](docs/architecture/adr/0152-one-phase-agent-runs-from-the-command-line-with-no-cycle-through-the-phase-runner.md)).
  - `evolve quality report --scope wave|tree|judges|replay` and `evolve quality compare` compute the quality indexes and publish them through the Signal Center ([ADR-0153](docs/architecture/adr/0153-quality-is-a-panel-of-separate-indexes-with-intervals-that-the-signal-center-publishes.md)). The scope `wave` reads PQ1, PQ4, CQ2 and CQ1, and `tree` reads CQ3, CQ4 and CQ5. `judges` and `replay` have no readers yet. The boundary runs `quality report --scope wave` after `sync-main`. The stage of each index is `quality.indexes.<id>.stage` (strict), and the checked-in policy does not set it.
  - `evolve signals tail --channel NAME` reads one event channel under `.evolve/events/ch/`, for example `quality.reading`.
- When only the deterministic audit gates force a FAIL over a PASS narrative, the cycle gets a bounded Build repair round (ADR-0093).
- Waves: the console starts each wave with `<plane>/go/bin/evolve wave next` and follows it with `evolve wave watch`. These read cycle state and signals, never the text log. Do not write goal scripts. The standing goal is the tracked `.evolve/wave-goal.md`; queue facts with `evolve wave note add`.
- Clear the session context before a new evolve-loop batch. This isolates the session cost.
- LLM isolation ([ADR-0154](docs/architecture/adr/0154-the-pipeline-is-the-only-controller-and-each-agent-is-an-isolated-replaceable-worker.md), [ADR-0155](docs/architecture/adr/0155-the-loop-heals-itself-inside-its-boundaries-an-agent-proposes-and-the-pipeline-disposes.md), [ADR-0156](docs/architecture/adr/0156-the-cli-is-the-service-interface-and-the-pipeline-reads-no-ambient-control-flags.md)): the pipeline is the only controller, and each LLM runs in an isolated cell. The state on 2026-10-25:
  - X1 is merged (#1121): the conformance matrix `isolation.Rows` and the `isolation.violation` channel. Each open gap is an expected-failure row.
  - X6 is merged (#1123): the Claude floor and the flow seat defaults are routing data.
  - X2 is merged (#1122): at `enforce` each LLM child gets an allowlisted environment. At the default `shadow` the child keeps today's environment, and the bridge only logs each name that `enforce` drops. The policy key `isolation.i2.stage` defaults to `shadow`, and the checked-in policy does not set it. The flip to `enforce` is an operator decision.
  - The CLI is the service interface for other services: [cli-contract.md](docs/architecture/cli-contract.md). `evolve loop status <word>` starts a loop today; X17a fixes it.
  - The pipeline must read no control flag from the environment (X18). [control-flags.md](docs/architecture/control-flags.md) lists each reader by class.
- Events ([event-notification-protocol-2026-10](docs/plans/event-notification-protocol-2026-10.md)): the channels, the filter, the wake, the reader, the publisher and the root wiring (E10) are on `main`. The `events` CLI verbs (E9) are not started, so `evolve wave watch` still polls every 5 s.

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
