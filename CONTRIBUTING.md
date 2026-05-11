# Contributing to Evolve Loop

Thanks for your interest in contributing! This guide covers how to add agents, modify phases, or fix bugs.

## Getting Started

```bash
git clone https://github.com/mickeyyaya/evolve-loop.git
cd evolve-loop
./install.sh
```

## Contribution Types

### 1. Agents

Agent definitions live in `agents/` as Markdown files with YAML frontmatter. See `docs/writing-agents.md` for the full guide.

Current agents: Scout, Builder, Auditor, Operator.

### 2. Phases

Phase instructions live in `skills/evolve-loop/phases.md`. When modifying:
- Update the architecture diagram in `SKILL.md`
- Update `memory-protocol.md` if adding new workspace files
- Update the agent table in `SKILL.md`

### 3. Eval System

Eval definitions and the eval runner live in `skills/evolve-loop/eval-runner.md`.
- Eval definitions are created by the Scout agent at runtime
- The eval runner is orchestrator-executed (not a separate agent)
- Max 3 retry attempts before failure

### 4. Bug Fixes

- Reference the issue number in your PR
- Test with at least one `/evolve-loop --cycles 1` run on a sample project

### 5. Research notes & design references

evolve-loop maintains two content surfaces:

- `docs/research/` — actively cited by personas/skills/scripts; **loaded** into agent context during cycles
- `knowledge-base/research/` — developer-only reference; **NOT loaded** into agent context (kernel-blocked across all CLIs)

**Decision rule when filing a new research note:**

> Will any persona, skill, or script reference this doc?
> - **YES** → `docs/research/`
> - **NO**  → `knowledge-base/research/`

Cross-references count even if the doc isn't loaded into every cycle's context — what matters is whether any runtime artifact *could* read it. See [docs/architecture/knowledge-base.md](docs/architecture/knowledge-base.md) for the full convention.

**For agents writing research citations during cycles:** the stewardship rule (v9.1.x+) requires that every learned/applied/verified citation be persisted to `knowledge-base/research/` if not already present. Scout adds the entry; Builder cross-references it from build-report.md; Auditor verifies it exists.

## Pull Request Process

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-contribution`
3. Make your changes
4. Test by running `./install.sh` and executing `/evolve-loop 1` on a test project
5. Submit a PR with summary, phase impact, and test results

## Code of Conduct

Be respectful, constructive, and focused on making the project better. We follow the [Contributor Covenant](https://www.contributor-covenant.org/).
