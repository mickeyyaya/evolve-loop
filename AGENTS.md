# AGENTS.md — Cross-CLI Canonical Instructions

> **Read this file first if you are an AI agent that works in this repository.** The agent can be Claude Code, Codex CLI, Antigravity/agy, Ollama, Gemini CLI or a generic agent. This file is the source of truth for the cross-CLI invariants. The CLI-specific runtime details are in companion files: [CLAUDE.md](CLAUDE.md), [GEMINI.md](GEMINI.md). All three files refer back to this document.
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
Phases run Scout → Builder → Auditor → Ship/Record, in that exact order. The Go orchestrator state machine (`go/internal/core`) enforces the phase ORDER. The phase-gate kernel hook (`evolve guard phase`) denies in-process subagent dispatch while a cycle is active. Per ADR-0075, this hook was rewired onto the `Agent|Task` matcher.

The emergency operator override is the explicit `evolve guard phase --bypass` CLI flag. With it, the guard allows the call and writes no special log line. Each use is a CRITICAL violation.

### 2. Subagents start through the native bridge, never through in-process tool calls
The native runner spawns every phase agent. The runner is `evolve subagent run <agent> <cycle> <workspace>`, or the in-process `go/internal/bridge` launcher that `evolve loop` / `evolve cycle run` drive. The kernel hook enforces this rule. During a cycle, the hook **denies** the in-process `Agent` (Claude Code) / `activate_skill` (Gemini) / equivalent (Codex). Reason: in-process subagents bypass the profile-scoped permissions and the tamper-evident ledger.

### 3. Commits go through `evolve ship`, never bare `git commit / git push`
The ship-gate kernel hook (`evolve guard ship`) denies a bare git commit/push/gh release create. The only canonical entry point is the native `evolve ship`. It enforces audit verification, cycle binding (HEAD + tree_state_sha match) and version-aware self-SHA pinning. Operator escape: `--class manual` (interactive) or the explicit `evolve guard ship --bypass` emergency flag.

Interactive `--class manual` commits also require a fresh **commit-gate review attestation** (v13.0.0+). The attestation is a `.commit-gate/attestation.json` whose `tree_state_sha` matches the staged tree. To make it, use `/commit` (code-simplifier + code-reviewer + language reviewer + lint + targeted tests). `evolve ship --bypass-commit-gate` skips the check. A routine use is a CLAUDE.md violation, identical in spirit to the emergency bypass of the ship guard.

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

This file covers the universal contract. The CLI-specific runtime details are in companion files:

- **Claude Code**: see [CLAUDE.md](CLAUDE.md). Tier-1 production. The skills at `skills/<name>/SKILL.md` are the only invocation/slash-command surface (per ADR-0040). They carry `argument-hint`. The plugin manifest at `.claude-plugin/plugin.json` declares only `agents` and `skills` (no `commands[]` array). Kernel hooks fire as PreToolUse hooks, as `.claude/settings.json` specifies.

- **Codex CLI**: Codex discovers skills automatically at `.agents/skills/<name>/SKILL.md`. (This directory has symlinks to `skills/<name>/`.) Codex reads this AGENTS.md as its canonical config. The native codex bridge driver (`go/internal/bridge/driver_codex*.go`) drives Codex in one of three modes:
  - NATIVE when `codex` is on PATH and supports non-interactive prompts;
  - HYBRID (delegates to `claude`) in other cases;
  - DEGRADED as a last resort (the pipeline still completes, with reduced isolation).

  `evolve bridge probe` shows the capability tier.

- **Gemini CLI**: Gemini discovers skills automatically at `.agents/skills/<name>/SKILL.md`. See [GEMINI.md](GEMINI.md) for Gemini-specific notes. `gemini` is a distinct CLI identity with its own adapter metadata (`adapters/gemini.capabilities.json`). There is **no** dedicated `driver_gemini*.go` bridge driver.

  A Gemini *model* is also reachable natively through the Antigravity (agy) driver (`go/internal/bridge/driver_agy*.go`; the code documents it as "Gemini-backed"). But `gemini` and `agy`/`antigravity` are separate CLI identities. Only `antigravity → agy` has a name resolution (see the Antigravity bullet below).

- **Antigravity CLI (agy)**: agy discovers skills automatically at `.agents/skills/<name>/SKILL.md`. The native agy bridge driver (`go/internal/bridge/driver_agy*.go`) drives it. It uses NATIVE mode (`agy -p`) when the agy binary is on PATH, HYBRID when claude is on PATH, and DEGRADED in other cases. The cross-name resolver maps `antigravity → agy`. cost_blind:true in NATIVE mode (deferred billing tap).

  The `agy-claude-tmux` driver routes to the Claude models that the agy binary serves (routing family `agy-claude`). Before the prompt, it verifies that agy booted a Claude model. If not, it exits 87.

  See [reference/agy-runtime.md](skills/loop/reference/agy-runtime.md). Capability tier: `evolve bridge probe`.

- **Ollama (local models)**: the native ollama tmux-REPL bridge driver (`go/internal/bridge/driver_ollamatmux.go`) drives it. The driver launches `ollama run <model>` in a tmux pane. It reuses the shared REPL completion-detection machinery. Ollama is a routing target for local models, with no skill/plugin surface of its own. Research record: [docs/research/ollama-control-surface-2026.md](docs/research/ollama-control-surface-2026.md).

- **Generic / unsupported CLI**: see [skills/loop/reference/generic-runtime.md](skills/loop/reference/generic-runtime.md). The tool name translation tables are at `skills/loop/reference/<platform>-tools.md`.

## Discovery contract for AI agents reading this file

If you are an AI agent that starts in this repository, do these steps:

1. **Identify your CLI**: Claude Code, Codex, Antigravity (agy), Ollama, Gemini, or other.
2. **Read your CLI-specific overlay**: CLAUDE.md, GEMINI.md, or `docs/architecture/platform-compatibility.md`.
3. **Read this AGENTS.md** in full. The cross-CLI invariants apply to you.
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

## Where to file issues

- Security vulnerabilities: see [SECURITY.md](SECURITY.md)
- Code of conduct: see [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- Contributions: see [CONTRIBUTING.md](CONTRIBUTING.md)
- Pipeline issues: GitHub Issues at https://github.com/mickeyyaya/evolve-loop/issues
- Architecture / release protocol: [docs/guides/publishing-releases.md](docs/guides/publishing-releases.md), [docs/architecture/tri-layer.md](docs/architecture/tri-layer.md)
