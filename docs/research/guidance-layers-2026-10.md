# Research: how the rules reach the agent that needs them

- **Status:** complete, 2026-10-20.
- **Plan:** [guidance-layers-2026-10](../plans/guidance-layers-2026-10.md). **Decision:** [ADR-0145](../architecture/adr/0145-each-rule-has-one-home-and-loads-by-audience-path-action-or-failure.md). **Design:** [guidance-layers](../architecture/guidance-layers.md).
- **Method:**
  - a read-only inventory of every rule source of the project, the user rules and the memory files, on 2026-10-20;
  - a read of how the console and the phase agents load guidance (`go/internal/adapters/bridge/bridge.go`, `go/internal/policy/overlays.go`);
  - live web searches on 2026-10-20 (section 3).

## Table of contents

- [1. The request](#1-the-request)
- [2. Local evidence](#2-local-evidence)
- [3. Current practice](#3-current-practice)
- [4. Limits](#4-limits)

## Terms used throughout

- **Rule:** one instruction with one meaning, for example "a commit never adds a code comment".
- **Always-on text:** text that every request of an agent carries, such as `CLAUDE.md` and the memory index.
- **Overlay:** a skill that the loop puts into the prompt of a phase agent (`go/internal/policy/overlays.go`).
- **Console:** the Claude Code session of the operator. It merges PRs and runs the waves.
- **Phase agent:** the model that runs one phase of a cycle (scout, tdd, build, audit and the others).

## 1. The request

The operator asked on 2026-10-20: "could you translate the rule and policy into the different levels of guidance skills that should be loaded and loaded in different cases. Start with the design with ultrathink".

## 2. Local evidence

### Where the rules live

**F2.1** The rules sit in seven kinds of source:

| Source | Size | Reader |
|---|---|---|
| `~/.claude/CLAUDE.md` | 37 lines | console |
| `~/.claude/rules/` | 91 files; only `common/core.md` (10 lines) is always on, and `golang/*` loads on `**/*.go` | console |
| `runtime/CLAUDE.md` | 159 lines, 2526 words | console and claude-tmux phase agents |
| `runtime/AGENTS.md` | 220 lines: 12 invariants and the 12 core rules | every CLI |
| `docs/operations/operating-policy.md`, `runtime-reference.md`, `docs/conventions/*` | 269, 666 and 782 lines | read on demand |
| memory: `MEMORY.md` and 137 `feedback_*` files | index 110 lines | console only |
| overlays and personas: 33 skills, 104 `agents/*.md` files | — | phase agents |

**F2.2** The console carries about 316 lines of always-on text: 37 + 10 + 159 + 110. The 10 lines are `~/.claude/rules/common/core.md`. About 40 lines of the memory index are one-off notes, such as a monitor buffering trap and a zsh word-split trap.
*Implication:* the always-on text is above the budget that current practice gives (S6, S9).

**F2.3** The 12 core agent rules are written three times: in `~/.claude/CLAUDE.md`, in `~/.claude/AGENTS.md` and in `runtime/AGENTS.md`. Rules such as "no MCP" and "docs go through docs-write" are in memory, in `AGENTS.md` and in a convention at the same time.
*Implication:* one change to a rule must find all its copies, and the copies drift.

**F2.4** The inventory found eight conflicts:

| Rule | Conflicts with |
|---|---|
| "minimal comments" (memory) | zero comments (`AGENTS.md` invariant 10, the code-comments convention) |
| "save research findings to memory" | findings go under `docs/` (doc stewardship) |
| "run only targeted tests" | "the full floor before merge" |
| "parallel work only when read-only" | waves of 4 writer lanes |
| "never stop for cost" | core rule 6, strict token budgets |
| "recover from a rate limit with cron or /loop" | "never poll; wake on events" |
| "at most 4 background agents" | "fleet width" and the 8-lane burn window |
| "bypass mode: never ask" | "confirm direction first" |

*Implication:* when two rules conflict, the model can pick either (S6). Each conflict needs one owner and one answer.

**F2.5** Stale or superseded rules remain: `no_agy_fallback` and `ten_lane_concurrency_standing` are marked superseded, and about ten more refer to the retired `ship.sh` and `release.sh` scripts.

### What phase agents get

**F2.6** A claude-tmux phase agent starts with `--setting-sources project`. It reads `runtime/CLAUDE.md` and the project settings, but not the user files and not the memory. The bridge builds the prompt in this order: contract, policy prefix, rules, skill overlays, operator directives, correction (`go/internal/adapters/bridge/bridge.go:147`).
*Implication:* a phase agent reads console operations rules, such as the merge fleet and the status report format, that do not apply to it.

**F2.7** Rules that matter to phase agents live only in memory. They include "never mutate a live kill guard" (a mutant of the `pgid <= 1` guard ran `kill(-1, SIGKILL)`), "cover a guard through a seam, never delete it", "pin OS error text", "rerun the guard test after a rebase" and "read the failure dossier before you classify".
*Implication:* the tdd, build and triage agents cannot see the rules that their own past failures produced.

**F2.8** The overlay rules select by tier, phase, kind and `writes_source` (`go/internal/policy/overlays.go:62`). `docs-write` goes to every code-writing dispatch, also when the scope has no document. `fable` goes to every deep or top dispatch, whatever the phase.
*Implication:* the loop already loads guidance by case. It has no selector for the paths of the lane scope.

### What code already does

**F2.9** Code enforces many rules:
- no added comment (`go/internal/commitgate/comments_added.go:43`);
- the reviewer attestation (`commitgate/reviewers.go:5`);
- commits only through `evolve ship` (`guards/ship.go:21`, a PreToolUse hook);
- the worktree and the phase order (`guards/role.go:28`, `guards/phase.go:18`);
- STE prose (`internal/stelint`, CI);
- full coverage of changed code (cover-changed, apicover, cover-strict in CI).

*Implication:* for these rules the prose only has to say why and how to recover. The check does the rest.

**F2.10** The bridge sets only `EVOLVE_PROJECT_ROOT` and `EVOLVE_SANDBOX` in the launch environment. The pane shell receives `EVOLVE_PROJECT_ROOT` by a `SendKeys` export (`go/internal/bridge/driver_tmux_boot.go:130`). The project guards read no environment. A hook cannot tell a phase agent from the console.
*Implication:* a hook that must run only in the console has no discriminator today.

**F2.11** The guards `ship`, `phase`, `role` and `docdelete` only allow or deny. No Go code emits `hookSpecificOutput.additionalContext` on a PreToolUse hook.
*Implication:* an action hook that adds a checklist is new code.

**F2.12** A phase agent runs the project hooks, because `--setting-sources project` loads `.claude/settings.json`. The CLIs `codex` and `agy` read only `AGENTS.md`. Path rules, hooks and `SessionStart` reach Claude Code only.
*Implication:* on a non-claude CLI, the persona and the overlays are the only carriers of L1 and L2.

## 3. Current practice

The searches ran on 2026-10-20. "acc." marks a page with no date of its own.

### Skills and progressive load

**S1** An Agent Skill loads in three steps. The name and description are always in the prompt. The body loads when the model judges it relevant, and linked files load when needed. The open spec limits the description to 1024 characters and recommends a body under 5000 tokens.
Sources: [Anthropic](https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills), 2025-10-16; [spec](https://agentskills.io/specification), acc.

**S2** Claude Code gives the skill list 1% of the context window and drops the least-used descriptions first. A `paths:` field loads a skill only when matching files are in play. After compaction only the first 5000 tokens of each used skill come back ([Claude Code skills](https://code.claude.com/docs/en/skills), acc.).
*Implication:* the first lines of each skill carry its most important rules.

**S3** In evals on Next.js 16, a skill that the agent had to choose was never invoked in 56% of cases. An 8 KB index in the always-on file scored 100% ([Vercel](https://vercel.com/blog/agents-md-outperforms-skills-in-our-agent-evals), 2026-01-27; a vendor eval).
*Implication:* a rule that must be followed cannot depend on the model choosing to load a skill. It needs a forced load, and the always-on text needs a map of the deeper layers.

### Layers and budgets

**S4** Claude Code concatenates the `CLAUDE.md` files of the user and the project, and when two conflict the model "may pick one arbitrarily". The guidance is under 200 lines for each file, with procedures in skills and path rules in `.claude/rules/` with `paths:` ([memory docs](https://code.claude.com/docs/en/memory), acc.). A malformed `paths:` makes the rule always on.

**S5** Claude Code puts the cost of each feature in one table. `CLAUDE.md` costs its full text on every request, a skill costs its description, and a hook costs nothing unless it returns text. Its rule of thumb has three steps:

- a convention that goes wrong twice goes into `CLAUDE.md`;
- a procedure pasted three times becomes a skill;
- a rule that must hold every time becomes a hook.
Source: [features overview](https://code.claude.com/docs/en/features-overview), acc.

**S6** A hook can block an action with a reason (exit 2) and can add context to a tool call (`additionalContext`). A `SessionStart` hook with the `compact` matcher puts the core rules back after compaction ([hooks guide](https://code.claude.com/docs/en/hooks-guide), acc.).
*Implication:* a hook can load a procedure exactly when its action starts, for example before a merge.

**S7** Every surveyed tool has the same four tiers. They are an always-on root file, files scoped by a glob, files chosen by description, and files named by hand. Each tool caps the size: 500 lines for Cursor, 6000 and 12000 characters for Windsurf, and 32 KiB for Codex.
Sources, all acc.: [Cursor](https://cursor.com/docs/context/rules); [Copilot](https://docs.github.com/en/copilot/how-tos/configure-custom-instructions/add-repository-instructions); [Codex](https://learn.chatgpt.com/docs/agent-configuration/agents-md); [Windsurf](https://docs.devin.ai/desktop/cascade/memories).

### How load affects the rules

**S8** With many simultaneous instructions, the best models follow about 68% of 500. One strong model drops from 98% at 100 rules to 63% at 500. The errors are mostly omissions, and earlier instructions are followed better ([IFScale, arXiv 2507.11538](https://arxiv.org/abs/2507.11538), 2025).
*Implication:* the always-on text holds a few dozen rules at most, in order of cost.

**S9** Performance falls as the input grows, across 18 models ([Chroma](https://www.trychroma.com/research/context-rot), 2025-07-14). Context files that a model wrote lowered the success of coding agents and raised the cost by about 20%. The authors advise to "describe only minimal requirements" ([arXiv 2602.11988](https://arxiv.org/abs/2602.11988), 2026).

### Rules as code

**S10** Factory writes a standard with its reason in `AGENTS.md`, then encodes it as a lint with a message that states the fix. OpenAI reports that one large `AGENTS.md` failed, and uses a short map plus linters and structural tests.
Sources: [Factory](https://www.factory.ai/using-linters-to-direct-agents), 2025-09-05, read as a search excerpt; [a summary of "Harness engineering"](https://www.louisbouchard.ai/harness-engineering/), 2026, because the primary page was not readable.

**S11** A skill's trigger can be tested. The method uses 8 to 10 queries that must trigger it and 8 to 10 that must not. Each runs three times, with a held-out set.
Source: [skill-creator](https://raw.githubusercontent.com/anthropics/skills/main/skills/skill-creator/SKILL.md), acc.
*Implication:* "the skill loads in the right case" can be a test.

## 4. Limits

- The inventory counts are from one read on 2026-10-20. Some memory files touch two themes, so the theme counts are approximate.
- The phase agents' exact loaded text was inferred from the launch flags and the bridge code, not captured from a run.
- The Vercel result (S3) is one vendor eval. The Factory result is a vendor report, and the OpenAI page was read only through a summary.
- It is not confirmed that the installed Claude Code version emits PreToolUse `additionalContext` to the model (plan §3, D4).
