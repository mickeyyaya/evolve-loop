# Project Instructions (Claude Code)

> **Read [AGENTS.md](AGENTS.md) first.** It has the cross-CLI invariants and the 12 Core Agent Rules. This file is the Claude Code overlay (a digest).
>
> **The full runtime detail is in [docs/operations/runtime-reference.md](docs/operations/runtime-reference.md): the env-var table, the operator commands, the ship classes and the publish pipeline.** Read it before you change loop behavior, flags, gates or releases. Release notes: [CHANGELOG.md](CHANGELOG.md).
>
> **Operating policy (canonical, environment-independent): [docs/operations/operating-policy.md](docs/operations/operating-policy.md).** A pipeline issue gets a console-first fix at maximum reasoning. Routing is typed plumbing. Salvage comes before requeue. Wiring proofs are mandatory. The repo, not session memory, is the source of truth for these rules.

## Automation Loop Guardrails

- When you run the evolve loop or any batch/merge-train automation, evaluate the output after EVERY cycle. If 2 consecutive cycles produce zero ships (0 merged PRs), STOP the loop immediately. Find the root cause in the pipeline before you run another wave. The canonical zero-ship halt rule is an operator guardrail, not a compiled breaker: [docs/operations/operating-policy.md §4.1](docs/operations/operating-policy.md).
- Never let a batch run more than 2 unproductive waves "to see if it self-corrects".
- ADR-0072 reconciliation: a zero-ship streak is evidence of a SYSTEM fail, so HALT + P0 applies. This does not conflict with "never stop the queue". That rule governs task-level failures.

## Status Reporting

When you report on a batch or a pipeline run, always include these items:

- the cycles that ran;
- the ships or lanes that completed (for example, 2/2);
- the open PR numbers;
- the names of the failed CI jobs, with the failure class;
- the specific next action.

Do not report "running" or "in progress" as a status without these numbers.

## Session conventions

- **Confirm direction first**: multi-step or multi-cycle work needs a 3-bullet plan and an approval. These tasks are exempt: single-cycle bug fixes, tasks that specify a file path, and tasks from an approved plan.
- **Output discipline**: give summaries with `file:line` refs. Put findings of more than 300 lines in a markdown file, not in the chat.
- **Jobs that run for a long time**: after the launch, verify the health (exit codes, log tail). Make a checkpoint every cycle so that `--resume` works. Report failures immediately.
- **Pre-commit review fleet**: change → code-simplifier → (**architecture-reviewer ∥ code-reviewer**, parallel, both read-only) → address the findings → commit.
  - architecture-reviewer ([.claude/agents/architecture-reviewer.md](.claude/agents/architecture-reviewer.md)) blocks the commit: you must fix each CRITICAL finding (by the rubric of the profile) before the commit.
  - A pure-docs diff skips the architecture-reviewer, but not the commit gate. The commit gate still requires the simplify and review capabilities. One `code-review-simplify` diff-review pass covers both (ADR-0115).
  - Only a proven comment removal needs no reviewer.

## Autonomous execution (bypass mode)

Bypass = "don't ask the user", NOT "skip integrity checks". These rules are mandatory (the full text is in runtime-reference.md):

1. Continue all cycles without a pause. Never ask "should I continue?".
2. Run the FULL pipeline every cycle: real `scout-report.md` / `build-report.md` / `audit-report.md`.
3. The Go orchestrator state machine (`go/internal/core`) enforces the phase order at every transition.
   `evolve guard phase` is a live PreToolUse deny of in-process `Agent`/`Task` dispatch while a cycle is active.
   ADR-0075 rewired `evolve guard phase` (`go/internal/guards/phase.go`).
4. Never fabricate cycle numbers (CRITICAL violation).
5. Phase agents go through the native bridge (`evolve subagent run` / `evolve loop`). The in-process `Agent` is denied.
6. The OS sandbox wraps subprocesses (`EVOLVE_SANDBOX=1`). When the sandbox is nested, the EPERM fallback turns on automatically.
7. Run the eval-quality pre-flight on every eval (`evolve eval quality-check`).
8. The Adversarial Auditor is on by default (an Opus auditor against a Sonnet builder). `ADVERSARIAL_AUDIT=0` turns it off.

Maximum velocity, zero shortcuts. The runtime provisions worktrees natively: agents must NOT call `git worktree`. Follow the failure-adapter verdicts (PROCEED/RETRY/BLOCK) verbatim. `evolve ledger verify` checks the chain.

## Verification before claiming done

1. Before you declare a CLI unavailable, probe it: `evolve doctor probe <tool>`. List what you checked.
2. Read the actual exports before you import from a module or call it.
3. Run the tests and report the counts: `cd go && go test ./internal/<pkg>/... — N/N PASS, no regression`.

## CI Failures

- Classify each CI failure as a flake or a real defect before you retry it. Use `evolve ci classify <run-id|pr:N|sha:H> [--json] [--rerun]`. Exit 0 = retry-safe: every red is pre-existing or flake-evidence. Do not retry an unclassified red.
- File a regression issue for each failure class that occurs 2+ times. Link the issue in the PR description.
- Ship gates: if a gate reports RED, verify the logic of the gate before you assume that the code is broken. False-RED gates shipped in the past.

## Go Project Conventions

After you edit a Go file, run `gofmt -l .`, `go vet ./...` and `go test -count=1 ./...` (from the `go/` module root) before you open a PR.

**`-count=1` is required, not optional.** The Go test cache does not track file reads that escape the module root through `..`. Many tests read `.evolve/profiles/*.json` that way. Thus, after a config-only edit, a bare `go test ./...` serves a stale `ok (cached)` while the regression is live on disk (reproduced 2026-08-28). CI and `make test` already pass `-count=1`. The gap was local verification.

Add table-driven edge-case tests for each bug whose root cause was found during a pipeline stall.

## Shell conventions

- The target is bash 3.2.
- Banned: `declare -A`, `mapfile`, `${var^^}`, `sed -i ''`, `date -d`.
- Required: `set -uo pipefail` (not `set -e`), atomic writes through `mv "${f}.tmp.$$"`, and `git diff HEAD` for the tree-state SHA.
- `skills/<name>/` is canonical. `.agents/skills/` are symlinks.
- The full table, with the reasons and the portable alternatives, is in [runtime-reference.md](docs/operations/runtime-reference.md).

## /evo:loop task priority

Task priority is the computed inbox rank ([ADR-0121](docs/architecture/adr/0121-inbox-priority-is-a-computed-rank.md), `evolve inbox rank --explain`). The class order is in `.evolve/policy.json` `inbox_priority.class_order`.

## Critical runtime facts (full table → runtime-reference.md)

- The gates are default-ON as **compiled Go defaults** (`internal/policy` + `internal/config`). These defaults apply when the policy block is absent:
  - `eval_gate=enforce`;
  - `contract_gate=enforce`;
  - `repo_contract_gate=enforce`: the ship-time repo-contract scanner pack (`internal/phases/ship/repocontract.go`). Lane ships can no longer red main. The build handoff floor runs the same pack first, so a red goes back to the build. Both run the pack only in the module of evolve-loop itself (`internal/repocontract`);
  - EGPS `red_count==0` to ship;
  - the tdd phase is enabled.
- `.evolve/policy.json` CAN override these defaults through a `gates`/`workflow` block. But the checked-in file sets only floor/cli_health/cli_routing/catalog/acs/parallel_evaluate/fleet/disposition/failure_disposition/gc/inbox_priority. It does not contain the gate keys. Do not assume that policy.json is the source of the defaults. Details for some of these keys (see runtime-reference):
  - `gc`: the retention horizons, since 2026-09-29.
  - `inbox_priority`: the class order and the factor weights of the inbox rank, since 2026-10-06 (ADR-0121). It is strict-decoded. The plan's P2 (2026-10-06) wired it: every lane consumer reads `inboxrank.Order`.
  - `gc.mode: enforce`, since 2026-10-06. Thus the batch-start and batch-end GC of the loop applies its run-dir and worktree sweeps, and does not only report them. `evolve gc` at the boundary still owns the cache, temp and process sweeps. The cache has a 20 GB compiled-default cap, `gc.go_cache_max_gb`.
  - `cli_routing`: the CLI routing table, since 2026-10-07 (L2, [cli-routing-table-2026-10.md](docs/plans/cli-routing-table-2026-10.md)). It replaces the former `workflow.universal_fallback_exclude` and `pins` keys. `evolve cli-routing migrate` folds those keys into it.
- Since 2026-08-10: `failure_disposition.stage=enforce`. The escalation boundary is LIVE (it was shadow).
- `fleet.count=3`. This value was restored from the August codex-quota reduction. It was verified live on 2026-08-24. Over cycles 1530-1552 there were 76 codex dispatches / 44% share and zero quota halts. The gpt-5.6 sol/terra/luna tiers were healthy at the time.
  - From 2026-09-10 until 2026-10-07, deep/top ran the deep model of the codex family manifest (gpt-5.6-sol) at high effort. On 2026-10-07 the routing table took codex out of every chain.
  - gpt-5.6-sol is the value record of the CHANGELOG. The 2026-09-09 gpt-6-astra cutover was withdrawn the next day for token cost. Claude deep/top stay opus.
  - The resolution is policy pin > live catalog > manifest baseline. The manifest `codex-tmux.json` is the ONE tier table of the family.
- Since 2026-10-07 the `cli_routing` table decides every dispatch:
  - Every non-deep, non-top phase runs agy first (Gemini 3.8 Flash High at balanced). No tracked profile defaults to fast, and no tracked profile lets an envelope fall to fast.
  - Deep and top seats run agy-owned Claude (`agy-claude-tmux`), then Claude Code. `agy-claude-tmux` waits up to 10 s for a claude-family footer label before the prompt. If no label shows, it exits 87. Exit 87 is a walk trigger, so the seat falls through. The usage query that follows can bench an exhausted Claude group, and preflight only warns on it.
  - The claudeFamilyFloor seats (auditor/adversarial-review/tdd + the spec verifiers) run Claude Code only.
  - codex is reachable only under `--bypass-policy`.
  - `evolve cli-routing explain <agent>` prints the chain of any agent.
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
  - CLI routing is in `.evolve/policy.json` `cli_routing` (one table: `clis`, `default`, `work`, `tiers`, `agents`).
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
- Run `/clear` before a new evolve-loop batch. This isolates the session cost.

## References

- [docs/operations/workspace-layout.md](docs/operations/workspace-layout.md) — the hub layout: the .repo.git bare store, the console/runtime planes and the dev/ worktrees (since 2026-09-01)
- [docs/operations/runtime-reference.md](docs/operations/runtime-reference.md) — the env-var table, the operator commands, the ship classes, how to publish
- [docs/architecture/](docs/architecture/) — design docs; [control-flags.md](docs/architecture/control-flags.md) — all `EVOLVE_*` flags
- [CHANGELOG.md](CHANGELOG.md) · [release-notes/](docs/operations/release-notes/index.md)
