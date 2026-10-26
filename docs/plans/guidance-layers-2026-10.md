# Plan: each rule has one home and loads in the case where it applies

- **Status:** proposed, 2026-10-20. The operator decided the eight conflicts of §6 on 2026-10-20. The design set is in review; the migration (G5) waits for its approval. §9 adds the runtime index on 2026-10-26.
- **Research:** [guidance-layers-2026-10](../research/guidance-layers-2026-10.md). **Decision:** [ADR-0145](../architecture/adr/0145-each-rule-has-one-home-and-loads-by-audience-path-action-or-failure.md). **Design:** [guidance-layers](../architecture/guidance-layers.md).
- **Related:** [ADR-0132](../architecture/adr/0132-documents-are-written-through-one-skill.md) (documents through one skill), the skill-naming convention.

## Table of contents

- [1. The problem](#1-the-problem)
- [2. Candidate designs](#2-candidate-designs)
- [3. Decisions](#3-decisions)
- [4. Components and order](#4-components-and-order)
- [5. The migration](#5-the-migration)
- [6. Conflicts for the operator](#6-conflicts-for-the-operator)
- [7. Verification](#7-verification)
- [8. Limits](#8-limits)
- [9. The runtime index (2026-10-26)](#9-the-runtime-index-2026-10-26)

## 1. The problem

The rules of the project are in seven kinds of source (research F2.1). Three facts make them fail:

- The console carries about 316 lines of always-on text, above the budget of current practice (F2.2, S4, S8).
- A phase agent reads console rules that do not apply to it. It cannot read the memory rules that its own past failures produced (F2.6, F2.7).
- Rules are copied, conflict and go stale: three copies of the core rules, eight conflicts and about twelve stale rules (F2.3 to F2.5).

## 2. Candidate designs

Each criterion scores 1 (poor) to 5 (good).

| Design | A must-hold rule loads in its case | Context cost | Copies cannot drift | Reaches phase agents | Change size | Total |
|---|---|---|---|---|---|---|
| A. Prune the present files by hand | 2 | 3 | 1 | 1 | 5 | 12 |
| B. Hand-written layers: skills, path rules and hooks | 4 | 4 | 2 | 3 | 3 | 16 |
| C. One rule registry; a generator writes every layer; a check in CI | 5 | 5 | 5 | 4 | 2 | **21** |
| D. Every rule as a hook or a gate | 5 | 5 | 4 | 4 | 1 | 19 |
| E. One compressed always-on index of all rules | 3 | 2 | 3 | 4 | 4 | 16 |

C wins. It scores 4 for reach, not 5. A non-claude CLI reads only `AGENTS.md`. On that CLI, C reaches the phase agent through the persona and the overlays only (F2.12).

- A and B keep copies that a person must keep in step (F2.3).
- D is the right end for each rule that a check can decide. Most rules need judgment, so C keeps D as its L4 layer.
- E works for broad knowledge (S3), but hundreds of rules in one file exceed the budget where rule-following holds (S8).

## 3. Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D1 | Five layers: L0 invariants, L1 role, L2 situation, L3 reference, L4 enforcement. | S1, S5, S7 | Two layers (always and on demand): no place for the role and the case. |
| D2 | L0 is at most 60 lines and 30 rules, ordered by the cost of a break. It ends with a map of the L2 skills. | S3, S4, S8 | No budget: F2.2. |
| D3 | A must-hold rule loads through a forced path: an overlay, a path rule, an action hook or failure text. A skill that the model chooses carries only guidance that can be missed. | S3 (one vendor eval, so the G7 eval re-tests it) | Skills chosen by description for every rule: 56% were never loaded in S3. |
| D4 | A new action trigger: a PreToolUse hook adds the checklist of an action when its command starts. This is new code: no guard emits `additionalContext` today (F2.11). | S5, S6, F2.11 | Keep procedures in `CLAUDE.md`: the full cost on every request. |
| D5 | The overlay rules of the loop get a `Paths` selector, matched to the lane scope. `docs-write` loads only when the scope has a document. | F2.8 | Load every overlay for every code dispatch: the waste of F2.8. |
| D6 | `CLAUDE.md` holds only L0 and the map. The console role loads through a `SessionStart` hook that exits silently when the phase marker of D11 is set. | F2.6, F2.10 | Keep console rules in `CLAUDE.md`: phase agents read them. |
| D7 | Each rule is one file in `guidance/rules/`. `evolve guidance build` writes every layer, and `evolve guidance check` keeps them true in CI. | F2.3, S10 | Hand-kept layers: they drift. |
| D8 | Memory keeps personal facts and project status. A project rule moves into the registry. | F2.7, operating policy ("the repo is the source of truth") | Keep rules in memory: phase agents cannot read it. |
| D9 | Each task skill has a trigger eval: 8 queries that must load it and 8 that must not. | S11 | Trust the description: S3. |
| D10 | A rule that a check enforces keeps one line and names the check. The check's error text names the fix and the skill. | S10, F2.9 | Full prose beside a check: two copies of one rule. |
| D11 | The bridge exports a marker in the pane shell of every phase agent: `EVOLVE_PHASE_AGENT=1` and `EVOLVE_PHASE=<phase name>`. It uses the path that exports `EVOLVE_PROJECT_ROOT` (`primePaneShell`). The console `SessionStart` hook and every action hook exit silently when the marker is set. | F2.10, F2.12 | Detect a phase agent from the cwd or the settings: no stable signal, and the guards read no environment. A second settings file for phases: it forks the hook set. |

### Consequences

- Good: the console carries about 60 lines of always-on rules in place of about 316.
- Good: a phase agent gets the rules of its phase and its paths, including the safety rules that today live only in memory.
- Good: a rule changes in one file, and the check catches a copy, a conflict or a broken glob.
- Bad: the generator and the check are new code that must be kept.
- Bad: the migration of about 300 rules takes review time, and a rule can be put in the wrong layer.
- Risk: the registry needs one owner. The console operator owns it, and every rule change is a reviewed PR.
- Risk: the migration can break a safety rule in transit. The order is safety and integrity first, then commit, ship and boundary, then the rest (§5).

### What changes our mind

- A with-and-without eval of the new L0 shows lower success than today's files. Then L0 keeps more rules, within the budget of S8.
- The trigger evals show that an action hook fires in the wrong cases. Then that action uses a skill named in the L0 map.

## 4. Components and order

| # | Component | Home | Route |
|---|---|---|---|
| G0 | The registry format, `guidance/rules/`, and a reader with tests | `go/internal/guidance` (new) | lane |
| G1 | `evolve guidance build`: the L0 block and the runtime index first (§9). The path rules, the action files and the skill blocks follow as G1b. | `go/internal/guidance`, `go/cmd/evolve` | console |
| G2 | `evolve guidance check` and its CI step (§7 of the design), and the verb `evolve guidance index` (§9) | `go/internal/guidance`, `go/cmd/evolve`, `.github/workflows` | console |
| G3a | The phase marker of D11, exported in the pane shell, with tests that the export is sent for every phase | `go/internal/bridge` (`driver_tmux_boot.go`, the launch config) | console |
| G3 | The action hook (new Go code that emits `additionalContext` on PreToolUse) and the console `SessionStart` hook. Both exit silently when the marker is set. | `.claude/settings.json`, `evolve guard` | console |
| G4 | The `Paths` selector of the overlay rules, and `docs-write` only for a scope with a document | `go/internal/policy/overlays.go` | console |
| G5 | The migration of §5, in batches by theme | `guidance/rules/`, the memory | console |
| G6 | The trigger evals of the task skills | `skills/*/evals` | lane |
| G7 | The with-and-without eval of L0, run once before the switch | `skills/` evals | console |
| G8 | The context index of a cell reads `guidance/index.json` as its knowledge layer (§9.4) | the context-index design, after X5 (#1133) | the context-index builder |
| G9 | The instruction loader of each CLI in the driver layer, and the injection for a CLI with no file (§9.4) | `go/internal/bridge/manifests`, `go/internal/adapters/bridge` | console |
| G10 | A byte budget for `AGENTS.md`, under the Codex cap, after the G5 batches (§9.4) | `go/internal/guidance` | console |

G0 and G1 come first, so the migration has a target. G3a comes before G3. G5 runs after the review of this set.

Open item: confirm that the installed Claude Code version passes PreToolUse `additionalContext` to the model. G3 starts only after the confirmation.

## 5. The migration

The migration sorts each rule of the present sources into one of four outcomes:

| Outcome | Test | Example |
|---|---|---|
| Promote | a project rule that matters to any agent | "never mutate a live kill guard" goes to L0 with `audience: all` |
| Keep in memory | a personal fact or a project status | the hub layout, an in-flight campaign |
| Merge | the same rule in two or more sources | the 12 core rules become one set of registry files |
| Retire | superseded, or its code is gone | `no_agy_fallback`, the `ship.sh` rules |

The batches follow the themes of the inventory, in this order. Each batch is one PR:

1. safety and integrity;
2. commit and ship, and the boundary;
3. the rest: docs, review, CI, testing, comments, wave operations, halts, routing, research, session and config.

A reviewer checks each rule's layer and audience.

## 6. Conflicts for the operator

Each conflict of research F2.4 has one answer. The operator accepted all eight as proposed on 2026-10-20.

| Conflict | Decision (operator, 2026-10-20) |
|---|---|
| "minimal comments" against zero comments | zero comments; retire "minimal" |
| research findings in memory against `docs/` | `docs/`; memory keeps only a pointer |
| targeted tests against the full floor | both, by stage: targeted package tests during a live wave and inside a lane, and the full floor once before each merge |
| parallel work only when read-only against waves of writer lanes | retire "parallel only when read-only"; it predates the wave orchestrator |
| never stop for cost against strict token budgets | never stop a cycle mid-phase for cost; when a budget is reached, stop at the next wave boundary and report |
| cron or /loop for a rate limit against "never poll" | one wake-up at a known quota reset, not a poll |
| at most 4 background agents against the fleet width | two scopes: at most 4 console subagents; the loop lanes follow `fleet.count` |
| bypass mode against "confirm direction first" | confirm only a new multi-step direction; never ask inside an approved plan |

## 7. Verification

- **The registry:** a rule file with a bad field, a duplicate statement or a broken glob fails `evolve guidance check`.
- **The generator:** the build is byte-stable. A second run changes nothing, and a hand edit of a generated block fails the check.
- **The budgets:** L0 above 60 lines fails the check.
- **The triggers:**
  - an action hook adds its checklist on a matching command and on no other command;
  - a path rule loads on a matching file;
  - an overlay with `Paths` loads only for a lane scope that matches.
- **The audiences:** a phase agent's prompt has no console role text, and it has the L0 safety rules.
- **The marker:** a test asserts that the pane-shell export of `EVOLVE_PHASE_AGENT` and `EVOLVE_PHASE` is sent for every phase. With the marker set, the `SessionStart` hook and each action hook print nothing and exit 0.
- **The evals:** each task skill passes its trigger eval. L0 passes the with-and-without eval of G7.

## 8. Limits

- A forced load costs context in its case. A case with many triggers at once can still pass the budget of S8.
- The trigger evals test the model of the day. A new model can trigger in a different way, so the evals run again after a model change.
- The registry does not decide what a rule says. A wrong rule is still wrong in one place.
- On a non-claude CLI (`codex`, `agy`), path rules, hooks and `SessionStart` do not load. L1 and L2 arrive only through the persona and the overlays (F2.12).
- The registry has one owner: the console operator. Without a reviewed PR for each change, the registry drifts like the files it replaces.

## 9. The runtime index (2026-10-26)

The operator asked on 2026-10-26: "Extract the common rules, policies from claude / gemini / codex / ollama / etc... and build a structured system level indexes MD file to be loaded dynamically in the run time."

This section extends the registry of D7 with one more generated output: an index of all active rules. Research §5 (S12 to S18, F5.1 to F5.3) holds the facts.

### 9.1 The extraction inventory

The inventory read each project rule source on `origin/main` at `f6b8a7ab0` and the X10 branch of #1136, on 2026-10-26. The user files in `~/.claude` are personal and stay outside the generator (design §2).

| Source | Size | Reader | Common to each family | Family-specific, and why | Copies and conflicts |
|---|---|---|---|---|---|
| `AGENTS.md` (main) | 224 lines, 23,976 B | each CLI | the 13 invariants, the 12 core rules, the banned patterns | "Per-CLI runtime details" (the skill paths, drivers and tiers of each CLI). "Tool Hygiene" names the Claude tools `Read`, `Bash`, `Glob` and `Grep`. | invariants 1 to 6 are also in `GEMINI.md`, and invariants 1 to 5 in the X10 digest |
| `AGENTS.md` (X10, #1136) | 434 lines, 50,571 B | each CLI | the digest region (4,914 B, it ends at byte 6,033) and the full rules | the same two sections | over the Codex cap of 32 KiB from line 303 (F5.2); "Shared Constraints §5" allows a one-line comment on an export, against invariant 10 (zero comments) |
| `CLAUDE.md` (main) | 172 lines, 21,513 B | Claude Code, claude-tmux phase agents | the guardrails, the status format, the CI and Go rules | the kernel hooks of `.claude/settings.json`, `/clear` | X10 moves its rules to `AGENTS.md` and renders the digest; the X10 `CLAUDE.md` is 6,551 B in total |
| `GEMINI.md` (main) | 61 lines, 3,447 B | Gemini CLI, agy (legacy) | 6 invariants copied from `AGENTS.md` | the tool name table: a fact of one family | it names "9 cross-CLI invariants"; `AGENTS.md` has 13 (stale) |
| `.claude/agents/*.md` | 4 reviewers | Claude Code | the review rules | the frontmatter (tool list, model) | X10 renders each body from `agents/evolve-diff-<name>.md` |
| `.claude/rules/` | absent | — | — | — | — |
| `agents/*.md` personas | 106 files; 78 use MUST or NEVER | phase agents of each family | the phase procedure (L1) | the tool syntax of one family in 2 files (`Bash(`, "Agent tool") | — |
| `skills/*/SKILL.md` | 34 skills | console; phase agents through overlays | the task procedures (L2) | none found | `minimalism` and `architecture-review` have their own `Rules` blocks |
| policy overlays | 6 compiled rules (`go/internal/policy/overlays.go:58-63`) and `.evolve/policy.json` | phase agents | the selection by tier, phase, kind | the field `CLIs` selects a family (`overlays.go:33`) | — |
| `docs/operations/runtime-reference.md` | 743 lines, 318,112 B | on demand | the runtime facts (L3) | the CLI rows of the routing tables | many facts also in `CLAUDE.md` "Critical runtime facts" |
| `docs/operations/operating-policy.md` | 282 lines | on demand | the process rules with evidence (L3) | none | the digest restates the zero-ship halt |
| memory `feedback_*.md` | 143 files | the console only | see the next table | see the next table | see the next table |

The 143 memory feedback files sort into these outcomes. The list of each file and its outcome is the first step of each G5 batch. Batch 1 is part of the G0 to G2 item. Batch 3 also confirms that the 3 routing files are in `cli_routing` and then retires them. The 9 personal files need no action.

| Outcome | Files | Examples |
|---|---|---|
| L0, batch 1 (this change) | 20 | `never_mutate_live_kill_guards`, `no_broad_pkill_shared_host`, `every_llm_query_through_agent_bridge`, `no_mcp_cli_only` |
| L2 commit and ship, batch 2 | 20 | `gate_exit_code_through_pipe`, `attest_only_reviewers_that_ran`, `git_add_explicit_paths` |
| L1 and L2 boundary and waves, batch 2 | 20 | `no_pr_merges_mid_wave`, `merge_only_all_green_and_loop_pid`, `pool_live_from_signals_not_status` |
| L2 `test-write`, batch 3 | 11 | `coverage_never_by_deleting_guards`, `ci_os_error_text_pins` |
| L2 `review-converge`, batch 3 | 7 | `review_convergence_rule`, `reviewer_agents_no_git_mutation` |
| L1 and L2 design, docs, config and session, batch 3 | 30 | `phase_settings_from_config_not_code`, `always_run_online_research` |
| L2 `agent-brief`, host traps, batch 3 | 2 | `zsh_no_word_split_worktree`, `perl_delimiter_alternation_trap` |
| Family-specific: the driver layer (G9) | 8 | `parallel_bash_cd_shares_one_shell` and `monitor_pipeline_line_buffering` (Claude Code tools), `skill_tool_system_level` (claude flags), `commit_hook_dash_n_false_positive` (a Claude Code hook) |
| Routing data, not a rule | 3 | `test_lanes_on_sonnet`, `comment_removal_on_haiku`, `advisor_opus_deep_reasoning`: they belong in `cli_routing` |
| Personal or status: keep in memory | 9 | `hold_at_93pct_weekly_usage`, `claude_dir_14_day_retention` |
| Retire | 13 | `no_agy_fallback`, `release_script`, `ship_bypass_var`, `self_explaining_code_minimal_comments`, `skip_reviewers_for_docs` |

**F9.1** The inventory found a ninth conflict. `skip_reviewers_for_docs` says that a docs-only commit bypasses the reviewer requirement. `docs_change_gets_docs_review` says that a docs-only commit gets the `docs-review` skill, and the gate capability `docs-review` exists (#1010). The newer rule wins, so the older one retires.

**F9.2** Duplicates collapse into one rule each. Examples:

- `gate_and_commit_chained` and `gate_exit_code_through_pipe` become `commit-gate-unpiped` (design §9);
- `subagent_dispatch_via_agent_bridge`, `every_llm_query_through_agent_bridge`, invariant 2 and invariant 13 become `pipeline-only-controller`;
- `no_feature_flags_use_design_patterns` and `no_feature_flag_sprawl` become `no-feature-flags`;
- `session_handoff_at_40pct` and `refresh_context_at_40pct` stay in memory as one personal note.

### 9.2 Candidate designs

The criteria score 1 (poor) to 5 (good).

| Design | Every family gets the common rules | Context cost | One home | Reachable at any time by any agent | Change size | Total |
|---|---|---|---|---|---|---|
| A. A hand-written `RULES.md` that each CLI file imports | 3: Ollama reads no file; Codex truncates at 32 KiB | 2: an import loads in full (S12) | 2: a person keeps it in step with the registry | 3 | 5 | 15 |
| B. A generated index of the registry. L0 is always on, and the bodies load on demand. A CLI verb filters the index. The driver layer injects L0 for a family with no file. | 5 | 5 | 5 | 5: a file and a verb | 3 | **23** |
| C. A per-family copy of all rules, rendered into each CLI file | 4 | 1: about 50 KB for each session (X10 rejected the same copy) | 4 | 3 | 3 | 15 |
| D. A per-CLI native rules directory (`.claude/rules/`, `.agents/rules/`), rendered from the registry, and no index | 3: no file for Codex or Ollama | 4 | 4 | 2: only the native loader decides | 2 | 15 |

B scores 3 for change size: it needs a reader, a generator, a check and a verb, as C needs a generator and a check. The score hides G1b, the native renders that B adds later. B wins. A and C load the full text each session, and Codex drops all text after 32 KiB (F5.2). D uses the best native loader of each CLI, but two families have none, and an agent cannot list the rules. B keeps the good part of D: G1b can still render the native path rules from the same registry.

### 9.3 Decisions

| Id | Decision | Because | Rejected options and why |
|---|---|---|---|
| D12 | `evolve guidance build` writes `guidance/index.md` and its machine twin `guidance/index.json` from the registry. The index has one row for each active rule, grouped by layer. | S1, S3, S18 | A hand-written index: it drifts (F2.3). |
| D13 | Each always-on file carries only L0 and one pointer line to `guidance/index.md`. The L0 block is a generated region inside the digest of `AGENTS.md`, so `evolve skills generate` (X10) copies it into `CLAUDE.md` and `GEMINI.md`. The region replaces the hand-written "Hard rules" of the digest. The other digest facts stay hand-written beside it until the G5 batches move them to L1 and L2. | S12, S15, F5.2 | A second generator for `CLAUDE.md`: X10 already owns that render. |
| D14 | `evolve guidance index [--audience X] [--phase P] [--cli F] [--json]` prints the filtered index. It is an `always` verb of agent class `A`: it reads only, works while a wave runs, and prints one envelope with `--json` (ADR-0156 I-15). | ADR-0154 I-4, ADR-0156 I-15 | Only a file: a cell cannot filter it by its own audience. |
| D15 | A rule names no family fact. The optional field `families` selects the model families that need a rule. It matches the `model_family` of a manifest, a wider unit than the CLI name that `OverlayRule.CLIs` matches today. The tool names, file names and flags of a family stay in the driver layer. That layer is the CLI overlay of its file and its bridge manifest. | operator rule, F9 inventory | Family sections in `AGENTS.md`: Codex and Ollama read the facts of another family. |
| D16 | A family with no instruction file gets L0 and the pointer through the `## Rules` prefix of the bridge (`injectRulesPrefix`). Each manifest names its file or `inject` (G9). | S16, F5.3 | A Modelfile `SYSTEM` for each model: it is a second home, and each new model needs a new file. |
| D17 | Each entry of `guidance/index.json` has the shared fields `id`, `layer`, `kind`, `summary`, `bytes`, `path` and `audience`. Thus one schema of the context index holds a rule and a data artifact. The context-index design owns the wiring into the cell (G8). Until then, a phase agent reads `guidance/index.md` or runs the verb. | ADR-0154 I-4; the operator, 2026-10-26: data access is indexed and layered too | Wire the rules into the cell here: a second index beside the context index splits one need into two loaders. |
| D18 | The check fails on a bad registry file, a stale output, a repeated statement or L0 over budget. It also fails on two active rules in conflict and on an absent `enforced_by` path. The build and the verb fail closed on a bad registry file. | design §7 | Skip a bad file: a missing safety rule is silent. |
| D19 | The first batch migrates the L0 set of §9.5. Batches 2 and 3 follow §5. | §5 | Migrate all 143 at once: one review cannot judge the layer of each rule. |

**Consequences.** Good: each family gets the same 20 invariants, and any agent can list the rest with one verb. Bad: an agent must choose to open a body, so a rule that must hold still needs a forced loader (D3). Risk: the index row of a rule is its statement only. A statement that is too short can hide its scope, so the reviewer of each batch reads each statement against its body.

**What changes our mind.** The G7 eval shows lower success with L0 and the index than with today's files. Then L0 keeps more rules, within the budget of S8.

### 9.4 Components

G1 and G2 of §4 build the index, the L0 region and the verb. The new components:

- **G1b:** the path rules, the action files and the skill blocks of design §4, when the first L2 rule exists.
- **G8:** the context index of a cell extends `inputs/index.json` of X5 (#1133). It reads `guidance/index.json` as its knowledge layer, filtered by the audience, the phase and the family of the seat. The context-index builder owns this component (design §10.6).
- **G9:** each bridge manifest names its instruction file (`AGENTS.md`, `CLAUDE.md`, `GEMINI.md`) or `inject`. The bridge injects L0 and the pointer for `inject`. The family facts of `AGENTS.md` ("Per-CLI runtime details", "Tool Hygiene") and the 8 family-specific memory rules move to the CLI overlays and the manifests.
- **G10:** after the G5 batches move the full rules into the registry, `AGENTS.md` gets a byte budget under 32,768 bytes, and the check enforces it.

### 9.5 The first L0 batch

These 20 rules apply to every agent of every family. They are ordered by the cost of a break.

| Id | Source | Enforced by |
|---|---|---|
| `kill-guard-never-mutated` | memory `never_mutate_live_kill_guards` | — |
| `no-broad-pkill` | memory `no_broad_pkill_shared_host` | — |
| `pipeline-only-controller` | `AGENTS.md` invariants 2 and 13; two memory files (F9.2) | `go/internal/guards/phase.go` |
| `phase-order` | `AGENTS.md` invariant 1 | `go/internal/core/statemachine.go` |
| `commit-through-ship` | `AGENTS.md` invariant 3 | `go/internal/guards/ship.go` |
| `builder-worktree-only` | `AGENTS.md` invariant 4 | `go/internal/guards/role.go` |
| `ship-needs-egps-verdict` | `AGENTS.md` invariants 5 and 6 | — |
| `never-fabricate-results` | the X10 digest | — |
| `dev-on-branches` | memory `dev_on_branches_never_main` | — |
| `code-zero-comments` | `AGENTS.md` invariant 10 | `go/internal/commitgate/comments_added.go` |
| `docs-through-docs-write` | `AGENTS.md` invariant 11; three memory files | `go/internal/stelint` |
| `cli-only-no-mcp` | `AGENTS.md` invariant 12; memory `no_mcp_cli_only` | — |
| `findings-go-to-docs` | `AGENTS.md` invariant 9; two memory files | — |
| `no-ambient-control` | ADR-0156 I-16; memory `cli_params_over_env_flags` | — |
| `no-feature-flags` | two memory files (F9.2) | — |
| `zero-ship-halt` | the X10 digest; memory `zero_ship_halt_and_gate_red_protocol` | — |
| `verify-before-done` | the X10 digest; memory `probe_before_unavailable` | — |
| `read-before-call` | core rule 8; memory `read_before_implement`, `verify_named_proofs_exist` | — |
| `root-cause-no-workaround` | memory `no_workaround_root_cause_redesign` | — |
| `one-home-no-duplicates` | memory `never_duplicate_centralize_via_design_patterns` | — |

The commit rules (`commit-gate-unpiped` and the staging rules) are L2 of `commit-ship` (design §6), so they wait for batch 2.
