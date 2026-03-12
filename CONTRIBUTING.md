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
- Test with at least one `/evolve-loop 1` run on a sample project

## Pull Request Process

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-contribution`
3. Make your changes
4. Test by running `./install.sh` and executing `/evolve-loop 1` on a test project
5. Submit a PR with summary, phase impact, and test results

## Code of Conduct

Be respectful, constructive, and focused on making the project better. We follow the [Contributor Covenant](https://www.contributor-covenant.org/).
