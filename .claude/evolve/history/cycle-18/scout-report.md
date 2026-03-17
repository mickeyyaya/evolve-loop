# Cycle 18 Scout Report

## Discovery Summary
- Scan mode: incremental
- Files analyzed: 5 (skill-efficiency-research.md, state.json, evolve-scout.md, evolve-builder.md, evolve-auditor.md, evolve-operator.md, phases.md)
- Research: skipped (cooldown — all 3 queries dated 2026-03-14, TTL 12hr not expired)
- Instincts applied: 1 (inst-013: progressive-disclosure-over-inline)

## Key Findings

### Features — HIGH
- `state.json.planCache` is empty (`[]`) despite 42 completed tasks with extractable patterns. The plan cache mechanism was designed in cycle 9 but has never been populated. When populated, SKILL.md documents ~30-50% cost reduction on repeated task patterns.

### Code Quality — MEDIUM
- All four agent files contain a "Strategy Handling" section (scout: lines 33-41, builder: lines 22-30, auditor: lines 44-52, operator has none). These 4 blocks repeat the same 4 strategy descriptions that are already defined in SKILL.md's Strategy Presets table. Combined ~25-30 lines of duplicated content (~375-450 tokens) across 3 agent files. R1 from cycle 17 research directly targets this.

### Architecture — MEDIUM
- The process rewards rubric in phases.md (lines 213-223) tracks 5 dimensions (discover, build, audit, ship, learn) but has no `skillEfficiency` dimension. Without this dimension, meta-cycles have no signal to track prompt bloat over time. The cycle 17 measurement established a baseline (27,165 tokens) — now there needs to be a place to track whether that number improves.

## Research
Skipped — all three research queries still within 12-hour TTL (performed 2026-03-14).
Cycle 17 research findings in `.claude/evolve/workspace/skill-efficiency-research.md` provide sufficient grounding for all three tasks.

## Selected Tasks

### Task 1: Extract shared strategy definitions from agent prompts
- **Slug:** extract-strategy-from-agents
- **Type:** techdebt
- **Complexity:** M
- **Rationale:** R1 from cycle 17 research. Three agent files (scout, builder, auditor) each have a "Strategy Handling" section duplicating the same 4 strategy descriptions already in SKILL.md. Removing these and replacing with a 2-line reference saves ~25-30 lines (~375-450 tokens) total. Applies inst-013 (progressive-disclosure-over-inline). Scout (240 lines) is currently 60% over the 150-line research-recommended target — this is the highest-leverage reduction available.
- **Acceptance Criteria:**
  - [ ] Each of the 3 agent files (scout, builder, auditor) retains exactly 1 occurrence of "Strategy Handling" — a section header pointing to SKILL.md, not repeating definitions
  - [ ] Strategy behavior descriptions (balanced/innovate/harden/repair) are NOT present in agent files — only in SKILL.md
  - [ ] Each agent file is at least 7 lines shorter than before (minimum removal of the 4 strategy descriptions per file)
  - [ ] evolve-scout.md line count drops from 240 to <=235
  - [ ] evolve-builder.md line count drops from 152 to <=147
  - [ ] evolve-auditor.md line count drops from 148 to <=143
  - [ ] SKILL.md Strategy Presets table remains intact and complete
- **Files to modify:**
  - `agents/evolve-scout.md`
  - `agents/evolve-builder.md`
  - `agents/evolve-auditor.md`
- **Eval:** written to `evals/extract-strategy-from-agents.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -c "Strategy Handling" agents/evolve-scout.md | grep -q "^1$"` → expects exit 0
  - `grep -c "Strategy Handling" agents/evolve-builder.md | grep -q "^1$"` → expects exit 0
  - `grep -c "Strategy Handling" agents/evolve-auditor.md | grep -q "^1$"` → expects exit 0
  - `wc -l < agents/evolve-scout.md | awk '{if ($1 <= 235) exit 0; else exit 1}'` → expects exit 0
  - `wc -l < agents/evolve-builder.md | awk '{if ($1 <= 147) exit 0; else exit 1}'` → expects exit 0
  - `wc -l < agents/evolve-auditor.md | awk '{if ($1 <= 143) exit 0; else exit 1}'` → expects exit 0
  - `grep -q "SKILL.md" agents/evolve-scout.md` → expects exit 0
  - `grep -q "SKILL.md" agents/evolve-builder.md` → expects exit 0
  - `grep -q "SKILL.md" agents/evolve-auditor.md` → expects exit 0

### Task 2: Populate plan cache from historical task patterns
- **Slug:** populate-plan-cache-templates
- **Type:** feature
- **Complexity:** S
- **Rationale:** R4 from cycle 17 research. `state.json.planCache` is empty despite the mechanism being designed and documented in SKILL.md (cycle 9). Populating it with 3-4 generalized task templates extracted from the 42 completed tasks activates a built-in cost-reduction mechanism. Templates for recurring patterns (add-section-to-file, docs-update, version-bump) will give the Builder a reusable scaffold, reducing design time per task. Low-effort, high-reward activation of existing infrastructure.
- **Acceptance Criteria:**
  - [ ] `state.json.planCache` contains 3-4 entries (was empty)
  - [ ] Each entry has fields: `slug`, `pattern` (regex or keyword matcher), `template` (brief task plan), `useCount` (initialized to 0), `cycleAdded`
  - [ ] Templates cover at minimum these 3 patterns: `add-section-to-file`, `docs-update`, `version-bump`
  - [ ] Templates are concise (3-7 lines each) — no verbosity
  - [ ] state.json remains valid JSON after modification
- **Files to modify:**
  - `.claude/evolve/state.json`
- **Eval:** written to `evals/populate-plan-cache-templates.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `python3 -c "import json; s=json.load(open('.claude/evolve/state.json')); assert len(s.get('planCache', [])) >= 3; print('OK')"` → expects exit 0
  - `python3 -c "import json; s=json.load(open('.claude/evolve/state.json')); pc=s.get('planCache',[]); assert all('slug' in t and 'pattern' in t and 'template' in t for t in pc); print('OK')"` → expects exit 0

### Task 3: Add skillEfficiency dimension to process rewards rubric
- **Slug:** add-skill-efficiency-process-reward
- **Type:** feature
- **Complexity:** S
- **Rationale:** Cycle 17 established a baseline skillMetrics measurement (1,811 lines, 27,165 tokens). Without a corresponding process rewards dimension, meta-cycles have no signal to track whether efficiency work is producing gains over time. Adding `skillEfficiency` to the rubric in phases.md creates a measurable feedback loop: score 1.0 when total skill+agent tokens decrease, 0.5 when stable, 0.0 when increasing. This closes the loop on the goal "make skills more efficient."
- **Acceptance Criteria:**
  - [ ] phases.md process rewards rubric table gains a new `skillEfficiency` row
  - [ ] Row defines Score=1.0 (tokens decreased from baseline), Score=0.5 (tokens stable ±5%), Score=0.0 (tokens increased)
  - [ ] The `processRewards` JSON schema block in phases.md gains a `"skillEfficiency": <0.0-1.0>` field
  - [ ] Existing 5 dimensions (discover, build, audit, ship, learn) are unchanged
- **Files to modify:**
  - `skills/evolve-loop/phases.md`
- **Eval:** written to `evals/add-skill-efficiency-process-reward.md`
- **Eval Graders** (inline — Builder reads these directly):
  - `grep -q "skillEfficiency" skills/evolve-loop/phases.md` → expects exit 0
  - `grep -A3 "skillEfficiency" skills/evolve-loop/phases.md | grep -q "1\.0\|0\.5\|0\.0"` → expects exit 0

## Deferred
- **R2 (compress output templates):** S-complexity but lower ROI than R1-R4. Deferred to cycle 19 if goal continues.
- **R3 (extract shared conventions file):** M-complexity, requires careful cross-agent refactoring. Defer until R1 lands and line counts are re-measured.
- **R5 (150-line target per agent):** Depends on R1 landing first. Scout at 240 lines is the primary offender — R1 alone should bring it to ~230, which is partial progress toward the 150 target. Full R5 requires a deeper pass.
- **R6 (resume meta-cycle):** Meta-cycle is every 5 cycles. Cycle 20 is the next natural trigger (cycles 15 was skipped). Flag for cycle 20.
- **R7 (pass only relevant task to Builder):** Requires orchestrator change in phases.md (different file than agent prompts). Defer to keep blast radius low this cycle.
