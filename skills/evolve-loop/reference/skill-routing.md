# Skill Routing Policy

> Read this file when building the skill inventory (Phase 0), matching skills to tasks (Phase 1), or resolving skill conflicts during build/audit. Cross-references: [policies.md](policies.md), [task-selection.md](task-selection.md).

## Layer Precedence

Skills are organized in 4 layers. Higher layers take precedence when capabilities overlap.

| Priority | Layer | Examples | Rationale |
|----------|-------|----------|-----------|
| 1 (highest) | **Built-in** | `/inspirer`, `/evaluator`, `/code-review-simplify`, `/refactor` | Pipeline-native output formats, no Skill tool dispatch overhead |
| 2 | **ECC** (everything-claude-code) | `security-review`, `tdd`, `go-review`, `python-review` | Curated, tested, ~2-5K tokens per invocation |
| 3 | **Superpowers** | `brainstorming`, `writing-plans`, `systematic-debugging` | General-purpose process skills |
| 4 (lowest) | **Domain Reference** | `security-patterns-code-review`, `testing-patterns`, 100+ catalogs | Read-only guidance, not actionable tools |

**Empirical override:** If `skillEffectiveness.hitRate` data exists for both a built-in and external skill in the same domain, prefer whichever has `hitRate > 0.3` after 5+ invocations. This allows runtime evidence to override static precedence.

## Phase-Skill Eligibility Matrix

Which skills may be invoked at each pipeline phase. `--` means ineligible.

| Skill | Phase 0 (Calibrate) | Phase 0.5 (Research) | Phase 1 (Discover) | Phase 2 (Build) | Phase 3 (Audit) | Phase 5 (Learn) |
|-------|:---:|:---:|:---:|:---:|:---:|:---:|
| `/inspirer` | -- | ~20K QUICK | -- | -- | -- | -- |
| `/evaluator` | project scope | -- | -- | -- | ~15-35K | -- |
| `/code-review-simplify` | -- | -- | -- | ~5K pipeline | ~20-45K full | -- |
| `/refactor` | -- | -- | -- | task-scoped | -- | -- |
| ECC skills (`security-review`, `tdd`, lang reviewers) | -- | -- | -- | per Step 2.7 | supplementary | -- |
| Superpowers (`systematic-debugging`, `brainstorming`) | -- | -- | -- | per Step 2.7 | -- | -- |
| Domain Reference catalogs | -- | -- | Scout reads | Builder reads | -- | -- |

**Rules:**
- Phase 0.5: Only `/inspirer` — external `brainstorming` is redundant (inspirer produces Concept Cards with composite scores).
- Phase 1: Scout reads domain catalogs as reference but does NOT invoke skills via the Skill tool.
- Phase 2: Builder invokes recommended skills per Step 2.7. Built-in `/refactor` eligible when `task.type == "refactoring"`.
- Phase 3: Auditor may invoke `/evaluator` and `/code-review-simplify`. External review skills are supplementary only.
- Phase 5: No skill invocations. Effectiveness data is tracked, not generated.

## Task-Type Routing Table

For each task type, the Scout recommends primary and supplementary skills. Max 3 total per task.

| Task Type | Primary Skill(s) | Supplementary | Notes |
|-----------|-----------------|---------------|-------|
| **New feature** | Language reviewer (`go-review`, `python-review`, etc.) | `testing-patterns`, `architectural-patterns` | Skip review skills for S-complexity inline tasks |
| **Bug fix** | `superpowers:systematic-debugging` | Language reviewer | Debugging skill first, then review after fix |
| **Refactoring** | `/refactor` (built-in) | `detect-code-smells`, `refactoring-decision-matrix` | Built-in preferred — worktree isolation + parallel partitioning |
| **Security hardening** | `everything-claude-code:security-review` | `security-patterns-code-review`, `auth-authz-patterns` | Always match `security` category |
| **Performance optimization** | `performance-anti-patterns` | `caching-strategies`, `database-review-patterns` | Add DB skills if query-related |
| **Documentation** | `code-documentation-patterns` | `review-api-contract` (if API docs) | Skip for doc-only S-tasks |
| **Testing** | `everything-claude-code:tdd` | `testing-patterns`, language-specific test skill | TDD workflow first |
| **Agent/skill development** | `agent-orchestration-patterns` | `agent-memory-patterns`, `agent-self-evaluation-patterns` | Match when files in `agents/` or `skills/` |
| **Infrastructure/CI** | `cicd-pipeline-patterns` | `container-kubernetes-patterns`, `deployment-patterns` | Match when `.github/`, `Dockerfile`, CI configs |

**Always-include rule:** Language reviewer is included as supplementary if `projectContext.language` matches and is not already primary.

## Conflict Resolution

When built-in and external skills overlap, apply these resolutions.

| Domain | Built-in Winner | External Alternative | When to Use External Instead |
|--------|----------------|---------------------|------------------------------|
| Code review | `/code-review-simplify` | `code-review:code-review`, `pr-review-workflow` | Reviewing PRs outside the evolve-loop pipeline |
| Creative divergence | `/inspirer` | `superpowers:brainstorming` | User-initiated standalone brainstorming sessions |
| Evaluation | `/evaluator` | `everything-claude-code:eval-harness` | Eval infrastructure design tasks (not quality scoring) |
| Refactoring | `/refactor` | `refactor` (domain), `detect-code-smells` | External patterns as supplementary guidance only |
| Security | _(none built-in)_ | `everything-claude-code:security-review` | Always — no built-in overlap, external is primary |

## Token-Budget Depth Routing

Adjust skill invocation depth based on `scripts/context-budget.sh` exit status.

| Budget Status | Built-in Depth | External Invocations | Max Skills/Task |
|--------------|---------------|---------------------|:---:|
| **GREEN** (< 20%) | Full: evaluator `standard`, inspirer `STANDARD`, code-review-simplify full | Up to 3 (1 primary + 2 supplementary) | 3 |
| **YELLOW** (20-30%) | Reduced: evaluator `quick`, inspirer `QUICK`, code-review-simplify pipeline only | 1 primary only, skip supplementary | 1 |
| **RED** (> 30%) | Skip all except `/evaluator` when `forceFullAudit` (use `quick` depth) | Skip all external skills | 0 |

**`budgetPressure` mapping:** `low` → GREEN, `medium` → YELLOW, `high` → RED.

**Exception:** `/evaluator` invoked via `forceFullAudit == true` ignores budget constraints. Use `--depth quick` (~15K) under YELLOW/RED instead of `standard` (~35K).

## Effectiveness Integration

Routing decisions connect to `state.json.skillEffectiveness` tracking. Built-in and external skills use the same schema.

| Signal | Action |
|--------|--------|
| `hitRate >= 0.6` after 5+ invocations | Promote to primary in its category |
| `hitRate < 0.2` after 5+ invocations | Demote: exclude from primary recommendations |
| `hitRate` 0.2-0.6 | Keep as supplementary, do not promote |
| New skill (0 invocations) | Eligible as supplementary; promote after 3+ hits |
| Skill not invoked in 10+ cycles | Reset effectiveness data (stale) |

**Phase 5 feedback loop:** Sort skills within each category by `hitRate` descending when building the inventory next session. Demoted skills are never recommended as primary unless they match an explicit task signal (e.g., security task always gets `security-review` regardless of hitRate).

## Category Extensions

Add built-in skills to the existing routing categories in `state.json.skillInventory.categoryIndex`:

| Category | Add Built-in | Position |
|----------|-------------|----------|
| `code-review` | `/code-review-simplify` | First (highest precedence) |
| `refactoring` | `/refactor` | First (highest precedence) |

`/inspirer` and `/evaluator` are **not** categorized — they are triggered by strategy/state conditions (Phase 0.5 and Phase 3), not task-type matching. Their invocation is governed by:
- `/inspirer`: `strategy == "innovate"` OR `discoveryVelocity.rolling3 < 0.5`
- `/evaluator`: `strategy == "harden"` OR `forceFullAudit == true`
