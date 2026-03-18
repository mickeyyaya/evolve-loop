# Domain Adapters

The evolve-loop pipeline is structurally domain-agnostic: discover → build → audit → ship → learn applies to any iterative improvement workflow. However, four specific touch points currently assume a **coding** domain. This document names those touch points, defines the adapter interface for each, and maps them to alternative domains.

## The Four Domain-Specific Touch Points

| Touch Point | Current (Coding) | What It Does | Where Defined |
|-------------|------------------|--------------|---------------|
| **Build Isolation** | Git worktree | Isolates Builder changes from main state | SKILL.md, phases.md Phase 2 |
| **Ship Mechanism** | `git commit && git push` | Persists and distributes completed work | phases.md Phase 4 |
| **Eval Graders** | Bash/grep commands (exit 0 = pass) | Verifies task acceptance criteria | eval-runner.md |
| **Auto-Detection** | Language/framework/test-command detection | Identifies project context at init | SKILL.md step 3 |

Everything else — task selection, instinct extraction, bandit arms, benchmark scoring, operator health checks, memory consolidation — is already domain-agnostic.

## Adapter Interface

Each touch point requires an adapter that implements a standard interface:

### Build Isolation Adapter

```
interface BuildIsolation {
  create(context)   → isolatedWorkspace   // Create isolated copy of current state
  apply(workspace)  → changes             // Apply changes back to main state
  discard(workspace)                      // Clean up failed attempt
}
```

| Domain | Implementation |
|--------|---------------|
| Coding | Git worktree (current) |
| Writing | Copy document to temp dir, edit copy, diff and merge back |
| Research | Snapshot current findings file, work on copy, merge deltas |
| Design | Version artifact (Figma branch, file copy), merge on pass |

### Ship Mechanism Adapter

```
interface ShipMechanism {
  commit(changes, message)  → receipt   // Persist completed work
  push(receipt)             → url       // Distribute to shared location
  revert(receipt)                       // Undo a shipped change
}
```

| Domain | Implementation |
|--------|---------------|
| Coding | `git commit` + `git push` (current) |
| Writing | Save final document + publish/export |
| Research | Append findings to knowledge base + notify stakeholders |
| Design | Export assets + update design system |

### Eval Grader Adapter

```
interface EvalGrader {
  run(criteria, artifact)  → {pass: bool, score: 0-100, details: string}
}
```

| Domain | Implementation |
|--------|---------------|
| Coding | Bash commands with exit codes (current) |
| Writing | LLM rubric grader (clarity, completeness, tone) |
| Research | Groundedness check (claims supported?) + coverage check (questions answered?) |
| Design | Visual diff check + accessibility audit + brand compliance |

### Auto-Detection Adapter

```
interface DomainDetection {
  detect(projectRoot)  → {domain: string, signals: string[], confidence: float}
}
```

| Domain | Detection Signals |
|--------|------------------|
| Coding | `package.json`, `go.mod`, `*.py`, `.git`, test commands |
| Writing | `*.md`, `*.docx`, `*.txt` majority, no build commands, prose-heavy content |
| Research | `*.md` with citation patterns, `references/`, bibliography files |
| Design | `*.figma`, `*.sketch`, `*.svg` majority, design token files |

## Domain-Agnostic Core (unchanged)

These components work across all domains without modification:

- **Phase 1 DISCOVER** — Scout scans for improvement opportunities (universal)
- **Phase 5 LEARN** — Instinct extraction, consolidation, operator health (universal)
- **Bandit task selection** — Reward-based task type prioritization (universal)
- **Benchmark evaluation** — Multi-dimensional quality scoring (dimensions may vary per domain)
- **Strategy presets** — balanced/innovate/harden/repair intent mapping (universal)
- **State management** — state.json, ledger, notes, history archival (universal)
- **Stagnation detection** — Pattern recognition for repeated failures (universal)
- **Context management** — Stop-hook, handoff, session resumption (universal)

## Migration Path

Generalization should be **additive** — the coding domain remains the default and must not regress.

1. **Phase 1: Document** (this file) — Name the abstractions ✓
2. **Phase 2: Detect** — Add domain auto-detection at initialization
3. **Phase 3: Eval** — Add non-code eval grader patterns (rubric, groundedness)
4. **Phase 4: Adapt** — Make SHIP and BUILD isolation domain-configurable
5. **Phase 5: Benchmark** — Add domain-specific benchmark dimensions

Each phase is independently valuable. Phase 1-3 can ship without changing the coding pipeline at all.

## Anti-Patterns

- **Premature abstraction** — Don't build adapter framework before a second domain is actually used
- **Lowest common denominator** — Don't weaken coding evals to accommodate writing evals; keep both
- **Domain sprawl** — Support 2-3 domains well rather than 10 poorly
- **Regression by generalization** — Every change must pass existing coding-domain eval graders
