# Contributing to Evolve Loop

Thanks for your interest in contributing! This guide covers how to add agents, modify phases, improve the eval system, or fix bugs.

## Getting Started

```bash
git clone https://github.com/danleemh/evolve-loop.git
cd evolve-loop
./install.sh
```

## Contribution Types

### 1. Agents

Agent definitions live in `agents/` as Markdown files with YAML frontmatter.

**Format:**

```markdown
---
model: sonnet  # or opus
---

# Agent Name

<agent instructions>

## Evolve Loop Integration

<workspace ownership, inputs, outputs, ledger format>
```

**Guidelines:**
- Each agent owns exactly one workspace file
- Include a `## Evolve Loop Integration` section with inputs, outputs, and ledger entry format
- For ECC wrapper agents, include `## ECC Source` with copy date for sync tracking
- Keep agents focused — one responsibility per agent

### 2. Phases

Phase instructions live in `skills/evolve-loop/phases.md`. When modifying phases:

- Maintain the sequential/parallel execution model
- Update the phase numbering in `SKILL.md` architecture diagram
- Update `memory-protocol.md` if adding new workspace files
- Update the agent table in `SKILL.md`

### 3. Eval System

Eval definitions and the eval runner live in `skills/evolve-loop/eval-runner.md`.

- Eval definitions are created by the Planner agent at runtime
- The eval runner is orchestrator-executed (not a separate agent)
- The hard gate retry protocol allows max 3 attempts

### 4. Bug Fixes

- Reference the issue number in your PR
- Include steps to reproduce if applicable
- Test with at least one `/evolve-loop 1` run on a sample project

## File Naming

- Lowercase with hyphens: `evolve-security.md`, `eval-runner.md`
- Agent files prefixed with `evolve-`: `evolve-<role>.md`

## Pull Request Process

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-contribution`
3. Make your changes
4. Test by running `./install.sh` and executing `/evolve-loop 1` on a test project
5. Submit a PR with:
   - Summary of changes
   - Which phase(s) are affected
   - Test results from a sample run

## PR Template

```markdown
## Summary
<1-3 bullet points>

## Phase Impact
- [ ] Phase 0: MONITOR-INIT
- [ ] Phase 1: DISCOVER
- [ ] Phase 2: PLAN
- [ ] Phase 3: DESIGN
- [ ] Phase 4: BUILD
- [ ] Phase 4.5: CHECKPOINT
- [ ] Phase 5: VERIFY
- [ ] Phase 5.5: EVAL
- [ ] Phase 6: SHIP
- [ ] Phase 7: LOOP+LEARN

## Test Plan
- [ ] Ran `/evolve-loop 1` on test project
- [ ] Verified affected workspace files are generated
- [ ] Verified ledger entries are correct
```

## Code of Conduct

Be respectful, constructive, and focused on making the project better. We follow the [Contributor Covenant](https://www.contributor-covenant.org/).
