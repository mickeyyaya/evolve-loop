# Plan: each rule has one home and loads in the case where it applies

- **Status:** proposed, 2026-10-20. The operator decided the eight conflicts of §6 on 2026-10-20. The design set is in review; the migration (G5) waits for its approval.
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
| G1 | `evolve guidance build`: the L0 blocks, the path rules, the action files, the skill blocks | `go/internal/guidance`, `go/cmd/evolve` | lane |
| G2 | `evolve guidance check` and its CI step (§7 of the design) | `go/internal/guidance`, `.github/workflows` | console |
| G3a | The phase marker of D11, exported in the pane shell, with tests that the export is sent for every phase | `go/internal/bridge` (`driver_tmux_boot.go`, the launch config) | console |
| G3 | The action hook (new Go code that emits `additionalContext` on PreToolUse) and the console `SessionStart` hook. Both exit silently when the marker is set. | `.claude/settings.json`, `evolve guard` | console |
| G4 | The `Paths` selector of the overlay rules, and `docs-write` only for a scope with a document | `go/internal/policy/overlays.go` | console |
| G5 | The migration of §5, in batches by theme | `guidance/rules/`, the memory | console |
| G6 | The trigger evals of the task skills | `skills/*/evals` | lane |
| G7 | The with-and-without eval of L0, run once before the switch | `skills/` evals | console |

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
