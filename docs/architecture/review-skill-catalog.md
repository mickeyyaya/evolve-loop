> **Audience**: Operators configuring `EVOLVE_BUILDER_REVIEW_SKILLS`; Builders invoking the self-review loop.
> **Status**: Normative — reflects cycle-20 assessment of all available skills for `EVOLVE_BUILDER_SELF_REVIEW=1` integration.

# Review Skill Catalog

## Table of Contents

1. [Skill-Loop Contract Requirements](#1-skill-loop-contract-requirements)
2. [Skill Compatibility Table](#2-skill-compatibility-table)
3. [Primary Skill: code-review-simplify](#3-primary-skill-code-review-simplify)
4. [Assessed Candidates](#4-assessed-candidates)
5. [Operator Extension Guide](#5-operator-extension-guide)
6. [Future Wrapper Candidates](#6-future-wrapper-candidates)

---

## 1. Skill-Loop Contract Requirements

For a skill to be usable in `EVOLVE_BUILDER_REVIEW_SKILLS`, it must satisfy all three:

| Requirement | Why required |
|-------------|-------------|
| **Emits numeric composite score (0.0–1.0)** | Builder's convergence loop compares score against `EVOLVE_BUILDER_REVIEW_THRESHOLD` (default 0.85) to decide continue/stop |
| **Reads its own diff** | Builder invokes the skill without pre-loading context; skill must call `git diff HEAD` (pre-commit context) or `git diff HEAD~1` (standalone) autonomously |
| **Single-writer safe** | Must not spawn parallel worktrees or subagents — Builder is the only writer per the single-writer invariant (`parallel_eligible: false` in builder profile) |

**Incompatible patterns:**
- Action-based skills that apply changes directly without emitting a structured score (e.g., `simplify`)
- PR-centric skills that read GitHub PR diffs rather than local git diff (e.g., `review`)
- Skills that spawn parallel worktrees per independent group (e.g., `refactor`)

---

## 2. Skill Compatibility Table

| Skill | Source | Composite score? | Reads diff itself? | Single-writer safe? | Verdict |
|-------|--------|-----------------|-------------------|---------------------|---------|
| `code-review-simplify` | evolve-loop built-in | ✓ 0.0–1.0 (4 dimensions) | ✓ (adaptive HEAD/HEAD~1, post cycle-20 fix) | ✓ | **COMPATIBLE — primary default** |
| `simplify` | system (Claude Code) | ✗ No numeric score | ✗ Action-based, applies fixes directly | ✓ | **INCOMPATIBLE — no parseable score** |
| `review` | system (Claude Code) | ✗ No numeric score | ✗ PR-centric (`gh pr view`) | ✓ | **INCOMPATIBLE — PR context, not local diff** |
| `refactor` | evolve-loop built-in | ✗ No numeric score | ✓ | ✗ Spawns parallel worktrees | **INCOMPATIBLE — violates single-writer** |
| `security-review` | ECC installed | ✗ Checklist only | ✗ | ✓ | **INCOMPATIBLE without wrapper — future candidate** |
| Custom skills | Operator-defined | Variable | Variable | Variable | See §5 |

---

## 3. Primary Skill: code-review-simplify

**Why unified over split:** A single `code-review-simplify` pass covers all 4 review dimensions (correctness, security, performance, maintainability) in one LLM context load. Splitting into two skills (e.g., `code-review` + `code-simplify`) would double the token cost for the same signal:

| Approach | Token cost per review | Dimensions | Convergence loops |
|----------|-----------------------|------------|-------------------|
| Single `code-review-simplify` | ~20–45K | 4 (all) | 1 loop covers all |
| `code-review` + `code-simplify` (hypothetical) | ~40–90K | same 4 split | 2 loops, 2 scores to parse |

**Composite score formula:**
```
composite = 0.35×correctness + 0.25×security + 0.15×performance + 0.25×maintainability
```

**Verdict thresholds:**
| Composite | Verdict | Builder action |
|-----------|---------|----------------|
| ≥ 0.85 | Converged | Stop loop, proceed to build-report |
| 0.6–0.84 | Iterate | Apply suggestions, re-invoke (up to `EVOLVE_BUILDER_REVIEW_MAX_ITERS`) |
| < 0.6 | Fail | Record FAIL verdict, note in build-report, hand off to Auditor for judgment |

**Adaptive depth routing:** Diff size auto-selects tier:
- < 50 lines / 1–3 files → Lightweight (~10–15K tokens)
- 50–200 lines / 3–10 files → Standard (~20–35K tokens)
- > 200 lines / 10+ files, or security-sensitive files → Full (~40–80K tokens)

**Security-sensitive escalation:** Files matching `agents/`, `skills/*/SKILL.md`, `*auth*`, `*eval*` auto-escalate to Full tier. The SKILL.md edit in cycle-20 (Task A) triggered Full tier review on its own diff — expected behavior.

**Cycle-20 git ref fix:** Pre-cycle-20, Step 1 hardcoded `git diff HEAD~1`, causing the skill to review the previous cycle's commit rather than Builder's in-flight worktree changes. Post-fix, Step 1 auto-detects context:

```bash
DIFF_STAT=$(git diff HEAD --stat 2>/dev/null || echo "")
if [ -n "$DIFF_STAT" ]; then
  REF="HEAD"   # pre-commit: review uncommitted changes
else
  REF="HEAD~1" # standalone: review last committed change
fi
```

The shell script backing layer (`scripts/utility/code-review-simplify.sh`) already supported `REF="${1:-HEAD~1}"` override — the SKILL.md AI layer now uses the same adaptive logic.

---

## 4. Assessed Candidates

### `simplify` (system skill)

**Description:** "Review changed code for reuse, quality, and efficiency, then fix any issues found."

**Assessment:** Action-based — applies simplification edits directly to source files without emitting a numeric composite score. The skill-loop requires a parseable score (0.0–1.0) to drive convergence decisions. `simplify` produces natural-language findings and applies fixes, but has no structured score output compatible with the loop's threshold comparison. **Incompatible with current skill-loop mechanism.**

**Could it be made compatible?** Only with a thin wrapper that: (a) invokes `simplify` in a dry-run / report-only mode, and (b) maps its findings to a numeric score. The wrapper would need to be created as a new evolve-loop built-in skill. Deferred to a future cycle.

### `review` (system skill)

**Description:** "Review a pull request."

**Assessment:** PR-centric — reads GitHub PR context via `gh pr view`, not local `git diff HEAD`. Requires an open PR, which does not exist during Builder's in-flight worktree phase. **Incompatible — wrong invocation context.**

### `refactor` (evolve-loop built-in)

**Description:** Orchestrates full refactoring pipeline with parallel worktree isolation per independent refactoring group.

**Assessment:** Spawns parallel worktrees via subagents. This directly violates the single-writer invariant (`parallel_eligible: false` in builder profile). **Structurally incompatible.** The `refactor` skill is designed for human-initiated slash command use, not Builder self-review loops.

### `security-review` (ECC installed)

**Assessment:** Produces a checklist-style security review (findings listed by category) with no numeric composite score. Could be a high-value extension if a thin wrapper maps checklist severity counts to a composite: `composite = 1.0 - (critical×0.5 + high×0.2 + medium×0.05)`. Deferred to a future cycle — wrapper creation is out of scope for cycle-20 per intent.md constraints.

---

## 5. Operator Extension Guide

### Adding a compatible skill

To add a skill to the Builder self-review loop:

1. **Verify the skill contract:**
   - Invokes via `Skill("<skill-name>")` from inside a Builder Claude session
   - Reads `git diff HEAD` (or detects context adaptively)
   - Emits a line matching: `Composite Score: 0.XX` (or `"composite": 0.XX` in JSON)
   - Does not spawn worktrees or subagents

2. **Register in `EVOLVE_BUILDER_REVIEW_SKILLS`:**
   ```bash
   export EVOLVE_BUILDER_REVIEW_SKILLS="code-review-simplify,your-skill-name"
   ```
   The list is comma-separated. Builder invokes each skill in sequence, parses all composite scores, and computes an aggregate.

3. **Test the integration:**
   - Run a cycle with `EVOLVE_BUILDER_SELF_REVIEW=1 EVOLVE_BUILDER_REVIEW_SKILLS="code-review-simplify,your-skill-name"`
   - Verify `build-report.md` contains `## Self-Review` with per-skill scores: `code-review-simplify=0.XX,your-skill-name=0.XX`
   - Verify convergence verdict is one of: `converged`, `iter-cap-hit`, `error`

### Env-var reference

| Env var | Default | Effect |
|---------|---------|--------|
| `EVOLVE_BUILDER_SELF_REVIEW` | `0` | Master switch — set `1` to enable skill-loop |
| `EVOLVE_BUILDER_REVIEW_SKILLS` | `code-review-simplify` | Comma-separated skill names to invoke |
| `EVOLVE_BUILDER_REVIEW_MAX_ITERS` | `3` | Max convergence iterations before `iter-cap-hit` |
| `EVOLVE_BUILDER_REVIEW_THRESHOLD` | `0.85` | Composite score above which loop converges |

---

## 6. Future Wrapper Candidates

| Skill | Wrapper approach | Value | Priority |
|-------|-----------------|-------|----------|
| `security-review` | Map checklist severity counts → composite: `1.0 - (critical×0.5 + high×0.2 + medium×0.05)` | High — catches auth/injection issues before Auditor | P-C21+ |
| `simplify` (system) | Dry-run mode + extract finding count as score proxy | Medium — overlaps with `code-review-simplify` maintainability dimension | Low — unified skill already covers this |
