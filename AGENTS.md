# AGENTS.md — Cross-CLI Canonical Instructions

> **Read this file first if you are an AI agent (Claude Code, Codex CLI, Gemini CLI, or generic) working in this repository.** It is the source-of-truth for cross-CLI invariants. CLI-specific runtime details live in companion files: [CLAUDE.md](CLAUDE.md), [GEMINI.md](GEMINI.md). All three reference back to this document.

## What evolve-loop is

A self-evolving development pipeline that orchestrates 4 specialized agents (Scout, Builder, Auditor, Orchestrator) through 6 lean phases per cycle (Calibrate → Intent → Scout → Build → Audit → Ship → Learn). Tier-1 kernel hooks enforce phase ordering, role-scoped write paths, atomic ship semantics, ledger SHA verification, and v8.37+ tamper-evident hash-chained recording.

> **Runtime note (Go-only):** The Go binary (`go/bin/evolve`) is the sole runtime entrypoint. The bash `legacy/scripts/` tree were removed in the Go-only consolidation — there is no bash fallback. Every operation below is a native `evolve <subcommand>` or a function in `go/internal/...`. For the history of the bash→Go port, see [docs/migration-from-bash.md](docs/migration-from-bash.md).

## Cross-CLI invariants (the universal rules)

These rules apply regardless of which CLI you are running under. They are STRUCTURAL — enforced by kernel hooks, not by prompt instructions.

### 1. Pipeline ordering is non-negotiable
Phases run Scout → Builder → Auditor → Ship/Record in that exact order. The phase-gate kernel hook (`evolve guard phase`) denies any subagent invocation that violates the sequence. The emergency operator override is the explicit `evolve guard phase --bypass` CLI flag; use is logged loudly and considered a CRITICAL violation.

### 2. Subagents are spawned through the native bridge, never via in-process tool calls
Every phase agent is spawned by the native runner (`evolve subagent run <agent> <cycle> <workspace>`, or the in-process `go/internal/bridge` launcher driven by `evolve loop` / `evolve cycle run`). This is enforced by the kernel hook. The in-process `Agent` (Claude Code) / `activate_skill` (Gemini) / equivalent (Codex) is **denied during a cycle**. Reason: in-process subagents bypass profile-scoped permissions and the tamper-evident ledger.

### 3. Commits go through `evolve ship`, never bare `git commit / git push`
The ship-gate kernel hook (`evolve guard ship`) denies bare git commit/push/gh release create. The only canonical entry point is the native `evolve ship`. It enforces audit verification, cycle binding (HEAD + tree_state_sha match), and version-aware self-SHA pinning. Operator escape: `--class manual` (interactive) or the explicit `evolve guard ship --bypass` emergency flag.

Interactive `--class manual` commits additionally require a fresh **commit-gate review attestation** (v13.0.0+): `.commit-gate/attestation.json` whose `tree_state_sha` matches the staged tree. Produce it with `/commit` (code-simplifier + code-reviewer + language reviewer + lint + targeted tests). `evolve ship --bypass-commit-gate` skips the check — routine use is a CLAUDE.md violation, identical in spirit to the ship-guard emergency bypass.

### 4. Builder writes only inside its worktree
Each cycle gets a per-cycle git worktree (provisioned natively by the orchestrator in `go/internal/core`, recorded in `cycle-state.json:active_worktree`). Builder's profile (`.evolve/profiles/builder.json`) restricts Edit/Write to the worktree path. The role-gate kernel hook (`evolve guard role`) denies edits outside that boundary, including interpreter-execution Bash-redirect leaks.

### 5. Audits are gated by the EGPS binary verdict
Auditor writes `audit-report.md`, and the ship gate is the binary `acs-verdict.json:red_count == 0` (EGPS v10.0.0+ — the scalar PASS/WARN/FAIL confidence level was removed). The verdict is computed from sandbox exit codes by the native audit phase (`go/internal/phases/audit` + `evolve acs suite`), never from a model's narrative. (Strict WARN→FAIL promotion is opt-in via `.evolve/policy.json` `workflow.strict_audit`; see [reference/env-vars](knowledge/reference/env-vars.md).)

### 6. The ledger is tamper-evident (v8.37.0+)
`.evolve/ledger.jsonl` records every subagent invocation with cycle binding, challenge token, artifact SHA, **prev_hash**, and **entry_seq**. Each new entry's prev_hash is the SHA256 of the previous entry's full JSON line. `.evolve/ledger.tip` records the latest entry's SHA atomically — truncation detection. Run `evolve ledger verify` (or `evolve guard chain`) to confirm history integrity. Modifying any historical entry breaks the chain at the next entry.

### 7. Failure adaptation is fluent-by-default (v8.28.0+)
Prior failures are recorded in `state.json:failedApproaches[]` with structured classifications (infrastructure-transient, code-audit-fail, code-audit-warn, etc.). The native failure-adapter (`go/internal/core`) returns deterministic decisions; the orchestrator follows them verbatim. Default mode is fluent (would-have-blocked rules emit awareness, not BLOCK). Strict mode (`.evolve/policy.json` → `workflow.strict_audit: true`) restores legacy block-on-recurring behavior.

### 8. Cost adaptation (v8.35.0+)
The auditor profile defaults to Opus, but the native diff-complexity check (`go/internal/core`) auto-downgrades to Sonnet for trivial diffs (≤3 files, ≤100 lines, no security paths). Saves ~$1.89/cycle on routine cycles. Operator override: `MODEL_TIER_HINT=opus` forces Opus regardless.

### 9. Knowledge Stewardship Rule (Day-One)

> **Knowledge Stewardship Rule (Day-One):** Every research finding, discovery, cycle learning, or tried-and-failed approach MUST be documented before the cycle ships. Place runtime references in `docs/research/`, archival dossiers in `knowledge-base/research/`. **Never delete; always archive.** When superseding a doc, MOVE it to `knowledge-base/research/archived-YYYY-MM-DD/` with a one-line note in the replacement pointing to the archive. Failing to document is a HIGH-severity audit defect.

Enforced by the doc-deletion guard (`evolve guard docdelete`, `go/internal/guards/docdelete.go`; PreToolUse kernel hook): blocks `rm`/`mv` targeting `docs/**` or `knowledge-base/**` unless the destination is the canonical archival path. Operator escape: set `workflow.allow_doc_delete=true` in `.evolve/policy.json` (logged; emergency only).

## 12 Core agent rules

Behavioral rules every agent must follow regardless of CLI. Where the kernel hooks above catch *structural* breaches, these catch *judgment* breaches. In bypass-permissions / autonomous mode, rule 4 ("stop and ask") is overridden — make the reasonable call and continue. All other rules apply unconditionally.

### Karpathy foundations (1–4): think before acting

1. **Think before coding.** Do not assume or over-engineer. State assumptions explicitly. Push back when a requested approach is wrong — propose the alternative.
2. **Push for simplicity.** If a simpler approach exists, propose it. Three similar lines beat a premature abstraction. Don't design for hypothetical future requirements.
3. **No silent changes.** When instructions admit multiple interpretations or the codebase offers multiple patterns, surface both and pick one explicitly with a one-line reason. Never guess silently.
4. **Surface ambiguity.** If something is unclear or context is missing, stop and ask. (Overridden in bypass-permissions mode: make the reasonable call and continue — the user will redirect.)

### Mnilax extensions (5–12): multi-step agent discipline

5. **Reserve judgment tasks for AI.** Use LLM cycles for qualitative work — summarizing, categorizing, drafting, designing. Deterministic work (retries, error codes, state transitions, hashing) goes in the Go kernel. evolve-loop already enforces this: the phase gate, ship, and failure-adapter (`go/internal/core`, `go/internal/phases/ship`) are deterministic Go; Scout/Builder/Auditor are LLM.
6. **Strict token budgeting.** Stay within your per-task and per-session token budget. When approaching it, summarize state to a markdown file and stop the phase rather than letting it auto-resume. (This is about your own working discipline, not a runtime cap: the product's token-budget *cost* gates were removed — the dollar-cost calculation was unreliable across LLM models — so the former dollar-cost budget flags (and `--budget-usd N`) are gone and cost is display-only telemetry.) See [docs/architecture/checkpoint-resume.md](docs/architecture/checkpoint-resume.md).
7. **Address conflicting patterns.** If two patterns conflict in scope (e.g., bash 3.2 vs bash 4 features; `skills/` canonical vs `.agents/skills/` canonical), do not mix them. Pick one, document the reason in the commit body, and tag the other for cleanup.
8. **Read first.** Before importing from or calling into a module, `Read` it and list its real exports. Do not invent function names from context. Builder agents shipping code against imagined APIs is the recurring failure mode this rule prevents.
9. **Write meaningful tests.** Tests verify *intent*, not surface behavior. EGPS predicates that pass with `echo PASS; exit 0` or `grep -q presence` are rejected by the eval-quality check (`evolve eval quality-check`, EGPS v10.0+). Same principle applies to Go unit tests: a passing test that doesn't probe the behavior change is a no-op.
10. **Use checkpoints.** For multi-phase work, summarize what was done and what remains after every phase. evolve-loop encodes this in the native cycle-state checkpoint + `phase report` artifacts. If you cannot clearly describe current status in 3 bullets, stop — you have lost the plot.
11. **Follow existing conventions.** Match the codebase even if you disagree. Specifics: Go is the runtime (`go/internal/...`); the few remaining shell helpers (test fixtures, the commit-gate runner) target bash 3.2 (no `declare -A`, no `mapfile`, no `${var^^}`) and use `printf > tmp && mv` for atomic writes; commits go through `evolve ship`; kernel guards are `evolve guard <name>` and split read roots (`PLUGIN_ROOT`) from write roots (`PROJECT_ROOT`).
12. **Fail loudly.** Do not silently skip steps, swallow errors, or report success when work was incomplete. A "migration complete" claim that secretly skipped 3 files is the failure mode this rule prevents. Format for completion claims: `cd go && go test ./internal/<pkg>/... — N/N PASS, no regression`.

## Per-CLI runtime details

This file covers the universal contract. CLI-specific runtime details live in companion files:

- **Claude Code**: see [CLAUDE.md](CLAUDE.md). Tier-1 production. Skills at `skills/<name>/SKILL.md` are the only invocation/slash-command surface (per ADR-0040; they carry `argument-hint`), with the plugin manifest at `.claude-plugin/plugin.json` declaring only `agents` and `skills` (no `commands[]` array). Kernel hooks fire as PreToolUse hooks per `.claude/settings.json`.

- **Codex CLI**: skills auto-discovered at `.agents/skills/<name>/SKILL.md` (this directory exists as symlinks to `skills/<name>/`). Codex reads this AGENTS.md as its canonical config. Driven by the native codex bridge driver (`go/internal/bridge/driver_codex*.go`): NATIVE when `codex` is on PATH and supports non-interactive prompts, HYBRID (delegates to `claude`) otherwise, DEGRADED as a last resort (pipeline still completes; reduced isolation). Capability tier visible via `evolve bridge probe`.

- **Gemini CLI**: skills auto-discovered at `.agents/skills/<name>/SKILL.md`. See [GEMINI.md](GEMINI.md) for Gemini-specific notes. `gemini` is a distinct CLI identity with its own adapter metadata (`adapters/gemini.sh`, `gemini.capabilities.json`); there is **no** dedicated `driver_gemini*.go` bridge driver. A Gemini *model* is also reachable natively through the Antigravity (agy) driver (`go/internal/bridge/driver_agy*.go`, documented in-code as "Gemini-backed"), but `gemini` and `agy`/`antigravity` are separate CLI identities — only `antigravity → agy` is name-resolved (see the Antigravity bullet below).

- **Antigravity CLI (agy)**: skills auto-discovered at `.agents/skills/<name>/SKILL.md`. Driven by the native agy bridge driver (`go/internal/bridge/driver_agy*.go`): NATIVE mode (`agy -p`) when the agy binary is on PATH; HYBRID when claude on PATH; DEGRADED otherwise. The cross-name resolver maps `antigravity → agy`. cost_blind:true in NATIVE mode (deferred billing tap). See [reference/agy-runtime.md](skills/loop/reference/agy-runtime.md). Capability tier: `evolve bridge probe`.

- **Generic / unsupported CLI**: see [skills/loop/reference/generic-runtime.md](skills/loop/reference/generic-runtime.md). Tool name translation tables at `skills/loop/reference/<platform>-tools.md`.

## Discovery contract for AI agents reading this file

If you are an AI agent activating in this repository:

1. **Identify your CLI**: Claude Code, Codex, Gemini, Antigravity (agy), or other.
2. **Read your CLI-specific overlay**: CLAUDE.md, GEMINI.md, or `docs/architecture/platform-compatibility.md`.
3. **Read this AGENTS.md** in full — the cross-CLI invariants apply to you.
4. **Discover available skills**: scan `.agents/skills/*/SKILL.md` (cross-CLI standard) or `skills/*/SKILL.md` (Claude Code primary).
5. **Discover available agents**: scan `agents/*.md`.

Skill files use YAML frontmatter (`name`, `description`) followed by markdown instructions. Skills include subdirectories (`scripts/`, `references/`, `assets/`) for resources used during execution.

## Trust boundary summary

The pipeline's safety properties stack into three tiers:
## Trust boundary summary

| Tier | Layer | What it catches |
|---|---|---|
| Tier 1 | Kernel hooks (phase-gate, role-gate, ship-gate, ledger SHA, cycle binding, hash chain) | Reward hacking, phase-skipping, integrity breach, tampering |
| Tier 2 | OS isolation (sandbox-exec on macOS, bwrap on Linux) | Compromised builder writing outside its sandbox |
| Tier 3 | Workflow defaults (intent capture, fan-out, mutation testing, adversarial audit) | Vague goals, sycophantic audits, tautological evals |

Tier 1 is non-negotiable and runs in privileged shell context. Tier 2 adapts to the environment. Tier 3 is operator-controlled per-run.

## Shared Constraints (v8.65.0+)

These constraints apply to ALL agents in the pipeline to ensure cost efficiency and structural integrity.

### 1. Tool Hygiene (P-NEW-9, P-NEW-21)
To prevent context saturation from accumulated tool results:
- **Summarize Reads**: After each `Read`, summarize the content in 2-3 lines; reference the summary in subsequent turns.
- **Discard Large Blobs**: After each `Bash` with large output, extract key lines and discard the full output.
- **Trajectory Compression**: When reading files >3000 tokens, extract 3–5 key facts and discard the full content from working context immediately.
- **No Speculative Loading**: Use `Glob`+`Grep` to locate points of interest before `Read`.

### 2. Banned Patterns
- **No Self-Reversion**: Do not revert your own changes unless explicitly instructed or if they cause an immediate environment failure.
- **No Bare Git**: Commits MUST go through `evolve ship`.
- **No Pipeline Bypass**: Never attempt to skip a phase or ignore a kernel-gate failure.
- **No Post-Report Turns**: Once the phase report (scout/build/audit/orchestrator) is written, STOP. Turn accumulation after report completion is a critical cost driver.

### 3. Flag → Parameter Conversion Standard (flag-reduction campaign)
When a cycle converts an `EVOLVE_*` env flag into a typed input parameter, the conversion is "done" ONLY when it meets the **[Flag → Parameter Conversion Standard](knowledge-base/research/flag-parameter-conversion-standard.md)**: (1) the resolution package is environment-agnostic (no `os.Getenv`/`LookupEnv`/`Environ` — enforced by `paramPackages` in `go/internal/policy/param_env_agnostic_test.go`, which the conversion MUST enroll the package in); (2) it ships an env-free, black-box, public-API test suite covering the full field × edge-case matrix; (3) every exported parameter API at 100% coverage with `apicover -enforce` exit 0. Tests drive behavior ONLY through input parameters — never `t.Setenv`. Reference template: `internal/quotareset` + `internal/policy` config accessors.

### 4. Minimalism (always-on)
Every coding change takes the laziest solution that actually works — the ladder (YAGNI → stdlib → native/`policy.json` config → already-present dependency → one line → minimum), no unrequested abstraction, deletion over addition, shortest working diff; mark a deliberate shortcut with a `minimal:` comment naming the ceiling + upgrade path. The cut is in scope, NEVER in safety: input validation, error handling, security, accessibility, explicit requests, and the pipeline gates (RED test / safety invariants / eval+contract gates / ship floor) are never simplified away. Full ruleset: **[skills/minimalism/SKILL.md](skills/minimalism/SKILL.md)** (adapted from ponytail, MIT).

## Where to file issues

- Security vulnerabilities: see [SECURITY.md](SECURITY.md)
- Code of conduct: see [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)
- Contributions: see [CONTRIBUTING.md](CONTRIBUTING.md)
- Pipeline issues: GitHub Issues at https://github.com/mickeyyaya/evolve-loop/issues
- Architecture / release protocol: [docs/guides/publishing-releases.md](docs/guides/publishing-releases.md), [docs/architecture/tri-layer.md](docs/architecture/tri-layer.md)
